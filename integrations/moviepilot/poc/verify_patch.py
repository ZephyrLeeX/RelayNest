"""补丁助手回归：只改临时夹具目录，不改运行中的 MP 文件。"""

import argparse
import importlib.util
import json
from pathlib import Path
import tempfile
import sys
import shutil
import subprocess
from types import ModuleType, SimpleNamespace
import unittest
from unittest.mock import patch

MODULE_PATH = Path(__file__).parent / "relaynestpatch/core_patch.py"
spec = importlib.util.spec_from_file_location("relaynest_core_patch_under_test", MODULE_PATH)
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)


class PatchTests(unittest.TestCase):
    def setUp(self):
        self.folder = tempfile.TemporaryDirectory(prefix="relaynest-patch-test-")
        self.addCleanup(self.folder.cleanup)
        self.root = Path(self.folder.name)
        (self.root / "app/schemas").mkdir(parents=True)
        (self.root / "version.py").write_text("APP_VERSION = 'v3.1.5'\n")
        self.originals = {
            "app/schemas/types.py": b'from enum import Enum\nclass NotificationChannel(Enum):\n    QQ = "QQ"\n',
            "app/schemas/notification.py": b'class ChannelCapabilityManager:\n    _capabilities: Dict[NotificationChannel, ChannelCapabilities] = {\n    }\n',
        }
        targets = {}
        for name, source in self.originals.items():
            (self.root / name).write_bytes(source)
            _, anchor, insertion = module.TARGETS[name]
            targets[name] = (module.digest(source), anchor, insertion)
        # 只在本地单元夹具中替换指纹表，生产API不暴露任何自定义profile输入。
        context = patch.dict(module.TARGETS, targets, clear=True)
        context.start()
        self.addCleanup(context.stop)
        self.manager = module.CorePatch(self.root, self.root / "data")

    def assert_original(self):
        for name, content in self.originals.items():
            self.assertEqual((self.root / name).read_bytes(), content)

    def test_plugin_ui_and_admin_dependencies(self):
        names = ["fastapi", "app", "app.api", "app.api.dependencies", "app.api.dependencies.auth",
                 "app.api.principal", "app.sdk", "app.sdk.plugin",
                 "app.schemas", "app.schemas.types", "app.schemas.notification"]
        stubs = {name: ModuleType(name) for name in names}
        admin = object()
        stubs["fastapi"].Depends = lambda dependency: SimpleNamespace(dependency=dependency)
        stubs["app.api.dependencies.auth"].get_current_active_superuser = admin
        stubs["app.api.principal"].ApiPrincipal = object
        stubs["app.sdk.plugin"]._PluginBase = object
        package_name = "relaynestpatch_ui_test"
        package = importlib.util.spec_from_file_location(package_name, MODULE_PATH.parent / "__init__.py",
                    submodule_search_locations=[str(MODULE_PATH.parent)])
        loaded = importlib.util.module_from_spec(package)
        try:
            with patch.dict(sys.modules, stubs):
                sys.modules[package_name] = loaded
                package.loader.exec_module(loaded)
                plugin = loaded.RelayNestPatch()
                plugin._patch = self.manager
                routes = plugin.get_api()
                self.assertEqual(len(routes), 3)
                for route in routes:
                    self.assertEqual(route["auth"], "bear")
                    self.assertIs(route["endpoint"].__func__.__defaults__[0].dependency, admin)
                forms, defaults = plugin.get_form()
                self.assertEqual(len([item for item in forms if item["component"] == "VBtn"]), 3)
                handlers = [item["props"]["onclick"] for item in forms if item["component"] == "VBtn"]
                if shutil.which("node"):
                    subprocess.run(["node", "-e", "for (const h of JSON.parse(process.argv[1])) new Function('return (' + h + ')');",
                                    json.dumps(handlers)], check=True)
                self.assertFalse((self.root / "data").exists())
                # UI方法声明与生成配置不能应用补丁；显式API调用才写临时夹具。
                response = plugin.patch_apply()
                self.assertTrue(response["success"] and response["data"]["restart_required"])
                self.assertTrue(plugin.patch_restore()["success"])
                self.assert_original()
        finally:
            for name in list(sys.modules):
                if name.startswith(package_name):
                    sys.modules.pop(name, None)

    def test_read_only_inspect(self):
        self.assertEqual(self.manager.inspect()["disk_state"], "original")
        self.assertFalse((self.root / "data").exists())

    def test_apply_idempotent_restore(self):
        self.assertTrue(self.manager.change("apply")["success"])
        self.assertEqual(self.manager.inspect()["disk_state"], "patched")
        inode = (self.root / "app/schemas/types.py").stat().st_ino
        self.assertTrue(self.manager.change("apply")["success"])
        self.assertEqual((self.root / "app/schemas/types.py").stat().st_ino, inode)
        self.assertTrue(self.manager.change("restore")["success"])
        self.assert_original()

    def test_symlink_version_refused(self):
        path = self.root / "version.py"
        target = self.root / "actual_version.py"
        path.rename(target)
        path.symlink_to(target)
        self.assertFalse(self.manager.inspect()["supported"])
        self.assertFalse(self.manager.change("apply")["success"])
        self.assert_original()

    def test_unknown_version(self):
        (self.root / "version.py").write_text("APP_VERSION = 'v3.1.6'\n")
        self.assertEqual(self.manager.change("apply")["reason"], "unsupported_version")
        self.assert_original()

    def test_same_version_modified_source(self):
        path = self.root / "app/schemas/types.py"
        changed = path.read_bytes() + b"# another change\n"
        path.write_bytes(changed)
        self.assertEqual(self.manager.change("apply")["reason"], "source_fingerprint_mismatch")
        self.assertEqual(path.read_bytes(), changed)
        self.assertEqual((self.root / "app/schemas/notification.py").read_bytes(), self.originals["app/schemas/notification.py"])

    def test_backup_conflict(self):
        self.assertTrue(self.manager.change("apply")["success"])
        backup = self.manager.data / "types.py.original"
        backup.write_bytes(b"changed")
        self.assertEqual(self.manager.change("restore")["reason"], "backup_conflict")
        self.assertEqual(self.manager.inspect()["disk_state"], "patched")

    def test_restore_refuses_user_changes(self):
        self.assertTrue(self.manager.change("apply")["success"])
        path = self.root / "app/schemas/types.py"
        changed = path.read_bytes() + b"# user change\n"
        path.write_bytes(changed)
        self.assertFalse(self.manager.change("restore")["success"])
        self.assertEqual(path.read_bytes(), changed)

    def test_partial_failure_recovery(self):
        real_write = module.atomic_write
        def fail_second(path, data, mode):
            if path == self.root / "app/schemas/notification.py":
                raise OSError("fixed simulated failure")
            return real_write(path, data, mode)
        with patch.object(module, "atomic_write", fail_second):
            self.assertFalse(self.manager.change("apply")["success"])
        self.assertEqual(self.manager.inspect()["disk_state"], "partial")
        self.assertEqual(self.manager.change("apply")["reason"], "partial_patch_restore_first")
        self.assertTrue(self.manager.change("restore")["success"])
        self.assert_original()

    def test_symlink_refused(self):
        path = self.root / "app/schemas/types.py"
        outside = self.root / "outside.py"
        path.rename(outside)
        path.symlink_to(outside)
        self.assertEqual(self.manager.change("apply")["reason"], "symlink_refused")
        self.assertEqual(outside.read_bytes(), self.originals["app/schemas/types.py"])

    def test_mode_preserved_and_invalid_operation(self):
        path = self.root / "app/schemas/types.py"
        path.chmod(0o640)
        self.assertEqual(self.manager.change("shell")["reason"], "invalid_operation")
        self.assertTrue(self.manager.change("apply")["success"])
        self.assertEqual(path.stat().st_mode & 0o777, 0o640)


def real_source_check(source_root):
    """在固定源码副本上检查真实指纹，原研究目录始终只读。"""
    with tempfile.TemporaryDirectory(prefix="relaynest-real-patch-test-") as folder:
        root = Path(folder)
        for name in module.TARGETS:
            path = root / name
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_bytes((source_root / name).read_bytes())
        (root / "version.py").write_text("APP_VERSION = 'v3.1.5'\n")
        manager = module.CorePatch(root, root / "data")
        assert manager.inspect()["disk_state"] == "original"
        assert manager.change("apply")["success"]
        assert manager.inspect()["disk_state"] == "patched"
        assert manager.change("restore")["success"]
        for name in module.TARGETS:
            assert (root / name).read_bytes() == (source_root / name).read_bytes()
    print("PASS: 固定v3.1.5源码副本真实指纹、应用/重复检查/恢复；未修改运行实例")


def container_check_source():
    """生成可经stdin运行的副本验证脚本；不依赖远端预存探针文件。"""
    body = r'''
"""容器独立进程，只修改临时源码副本并在退出时清理。"""
import importlib.util
import json
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import json
with tempfile.TemporaryDirectory(prefix="relaynest-callback-copy-") as folder:
    work = Path(folder)
    manager_file = work / "core_patch.py"
    manager_file.write_text(MANAGER_SOURCE)
    spec = importlib.util.spec_from_file_location("copy_patch_manager", manager_file)
    manager_module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(manager_module)
    root = work / "copy"
    (root / "app/schemas").mkdir(parents=True)
    for name in ("app/__init__.py", "app/schemas/__init__.py"):
        (root / name).write_text("")
    for name in ("version.py", "app/schemas/types.py", "app/schemas/notification.py", "app/schemas/message.py", "app/schemas/common.py"):
        shutil.copyfile(Path("/app") / name, root / name)
    manager = manager_module.CorePatch(root, work / "state")
    assert manager.change("apply")["success"]
    check = """
from app.schemas.types import NotificationChannel
from app.schemas.notification import ChannelCapabilityManager as C
from app.schemas.message import IncomingMessage, Message, MessageResponse
channel = NotificationChannel.RelayNest
assert C.supports_buttons(channel) and C.supports_callbacks(channel)
assert not C.supports_editing(channel)
for model in (IncomingMessage, Message, MessageResponse):
    assert model(channel='RelayNest', source='RelayNest-P1Probe:A').channel is channel
print('PASS: patched_copy_models_and_capabilities')
"""
    subprocess.run([sys.executable, "-B", "-c", check], cwd=root, check=True)
    assert manager.change("restore")["success"]
    for name in manager_module.TARGETS:
        assert (root / name).read_bytes() == (Path("/app") / name).read_bytes()
print(json.dumps({"kind":"container_independent_temporary_source_copy", "success":True, "real_core_modified":False, "temporary_copy_removed":not Path(folder).exists()}))
'''
    return "MANAGER_SOURCE = " + repr(MODULE_PATH.read_text()) + "\n" + body


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--source-root", type=Path)
    parser.add_argument("--emit-container-check", action="store_true")
    args = parser.parse_args()
    if args.emit_container_check:
        print(container_check_source(), end="")
        raise SystemExit(0)
    result = unittest.TextTestRunner(verbosity=2).run(unittest.defaultTestLoader.loadTestsFromTestCase(PatchTests))
    if not result.wasSuccessful():
        raise SystemExit(1)
    if args.source_root:
        real_source_check(args.source_root)
