"""固定指纹的文件补丁管理器；不热重载 Python 模块，不执行重启命令。"""

import ast
from contextlib import contextmanager
import fcntl
import hashlib
import json
import os
from pathlib import Path
import stat
import tempfile

PATCH_ID = "relaynest-callback-v1"
SUPPORTED_VERSION = "v3.1.5"
# 指纹匹配比字符串替换更早执行；未知版本/第三方修改不能套用旧补丁。
TARGETS = {
    "app/schemas/types.py": (
        "47c5a9f4a10d9350ac61ee64e31871880ba8571326b8df31237d4a748f543419",
        b'    QQ = "QQ"\n',
        b'    RelayNest = "RelayNest"  # relaynest-callback-v1\n',
    ),
    "app/schemas/notification.py": (
        "a83d9e1123fea976e30dad3bd258e63813157d1d2373ac9074ed55335983b31d",
        b'    _capabilities: Dict[NotificationChannel, ChannelCapabilities] = {\n',
        b'        NotificationChannel.RelayNest: ChannelCapabilities(\n'
        b'            channel=NotificationChannel.RelayNest,\n'
        b'            capabilities={ChannelCapability.INLINE_BUTTONS, ChannelCapability.CALLBACK_QUERIES},\n'
        b'            fallback_enabled=True,\n'
        b'        ),  # relaynest-callback-v1\n',
    ),
}


class PatchError(Exception):
    """稳定错误码，不将原始异常、文件正文或配置返回页面。"""


def digest(data):
    return hashlib.sha256(data).hexdigest()


def atomic_write(path, data, mode):
    """同目录原子替换单文件并刷盘；两文件整体不是原子事务。"""
    if path.is_symlink():
        raise PatchError("symlink_refused")
    fd, temporary = tempfile.mkstemp(prefix=".relaynest-patch-", dir=path.parent)
    try:
        with os.fdopen(fd, "wb") as stream:
            os.fchmod(stream.fileno(), mode)
            stream.write(data)
            stream.flush()
            os.fsync(stream.fileno())
        os.replace(temporary, path)
        directory = os.open(path.parent, os.O_RDONLY | os.O_DIRECTORY)
        try:
            os.fsync(directory)
        finally:
            os.close(directory)
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)


class CorePatch:
    def __init__(self, root, data_path):
        # API不接受这两个路径；运行插件只使用/app和宿主分配的数据目录。
        self.root = Path(root).resolve()
        self.data = Path(data_path) / PATCH_ID / SUPPORTED_VERSION

    def _target(self, relative):
        path = self.root / relative
        if path.is_symlink() or path.resolve() != path:
            raise PatchError("symlink_refused")
        return path

    def _version(self):
        try:
            tree = ast.parse(self._target("version.py").read_text())
            for node in tree.body:
                if isinstance(node, ast.Assign) and any(
                        isinstance(target, ast.Name) and target.id == "APP_VERSION" for target in node.targets):
                    return ast.literal_eval(node.value)
        except (PatchError, OSError, ValueError, SyntaxError):
            return None
        return None

    def _snapshot(self):
        version = self._version()
        if version != SUPPORTED_VERSION:
            raise PatchError("unsupported_version")
        result = {}
        for relative, (expected, anchor, insertion) in TARGETS.items():
            path = self._target(relative)
            content = path.read_bytes()
            if digest(content) == expected:
                original, state = content, "original"
            elif content.count(insertion) == 1 and digest(content.replace(insertion, b"", 1)) == expected:
                original, state = content.replace(insertion, b"", 1), "patched"
            else:
                raise PatchError("source_fingerprint_mismatch")
            if original.count(anchor) != 1:
                raise PatchError("patch_anchor_mismatch")
            patched = original.replace(anchor, anchor + insertion, 1)
            compile(patched, relative, "exec")
            result[relative] = {"path": path, "current": content, "original": original,
                                "patched": patched, "state": state, "mode": stat.S_IMODE(path.stat().st_mode)}
        return result

    def inspect(self):
        """只读检查；磁盘状态不能当作当前进程已经生效。"""
        try:
            rows = self._snapshot()
            states = {row["state"] for row in rows.values()}
            state = next(iter(states)) if len(states) == 1 else "partial"
            return {"supported": True, "version": SUPPORTED_VERSION, "disk_state": state,
                    "files": {name: row["state"] for name, row in rows.items()}}
        except (PatchError, OSError, SyntaxError) as error:
            return {"supported": False, "version": self._version(), "disk_state": "blocked",
                    "reason": str(error) if isinstance(error, PatchError) else "source_unavailable"}

    @contextmanager
    def _lock(self):
        self.data.mkdir(parents=True, exist_ok=True)
        if self.data.is_symlink() or (self.data / "patch.lock").is_symlink():
            raise PatchError("symlink_refused")
        # 跨API请求及插件实例互斥；数据目录必须使用同一插件本体目录。
        with (self.data / "patch.lock").open("a") as stream:
            fcntl.flock(stream, fcntl.LOCK_EX)
            try:
                yield
            finally:
                fcntl.flock(stream, fcntl.LOCK_UN)

    def _manifest(self, operation, rows):
        data = {"patch_id": PATCH_ID, "version": SUPPORTED_VERSION, "operation": operation,
                "files": {name: {"original": digest(row["original"]), "patched": digest(row["patched"])}
                          for name, row in rows.items()}}
        atomic_write(self.data / "manifest.json", json.dumps(data, indent=2).encode(), 0o600)

    def _backups(self, rows):
        for name, row in rows.items():
            backup = self.data / (Path(name).name + ".original")
            if backup.is_symlink():
                raise PatchError("symlink_refused")
            if backup.exists():
                if backup.read_bytes() != row["original"]:
                    raise PatchError("backup_conflict")
            else:
                atomic_write(backup, row["original"], 0o600)

    def change(self, operation):
        """仅显式点击调用；失败后保留备份/清单，禁止未知源码覆盖与热重载。"""
        if operation not in {"apply", "restore"}:
            return {"success": False, "reason": "invalid_operation"}
        try:
            with self._lock():
                rows = self._snapshot()
                states = {row["state"] for row in rows.values()}
                if operation == "apply" and len(states) != 1:
                    raise PatchError("partial_patch_restore_first")
                # 先保存所有原文件及清单，再开始改核心；数据位于持久插件目录。
                self._backups(rows)
                self._manifest("prepared_" + operation, rows)
                for name, row in rows.items():
                    path = self._target(name)
                    if path.read_bytes() != row["current"]:
                        raise PatchError("source_changed_during_operation")
                    desired = row["patched"] if operation == "apply" else row["original"]
                    if row["current"] != desired:
                        atomic_write(path, desired, row["mode"])
                self._manifest("completed_" + operation, rows)
            return {"success": True, "operation": operation, "restart_required": True,
                    "status": self.inspect()}
        except (PatchError, OSError, SyntaxError) as error:
            return {"success": False, "reason": str(error) if isinstance(error, PatchError) else "io_or_compile_error",
                    "status": self.inspect(), "message": "操作未完成，请检查磁盘状态并恢复，确认前不要重启。"}
