"""Linux 真实 PTY 回归；仅使用标准库与合成口令，参数为已构建 CLI。"""
import argparse
from concurrent.futures import ThreadPoolExecutor
import fcntl
import os
import pty
import select
import signal
import sqlite3
import subprocess
import tempfile
import termios
import time
from pathlib import Path

SECRET = b" SYNTHETIC_PTY_SECRET "


def tty_case(binary, db, operation, action, expect_broken=False):
    master, slave = pty.openpty()
    original = termios.tcgetattr(slave)
    flags = fcntl.fcntl(slave, fcntl.F_GETFL)
    args = [binary, "admin", operation, "--db", str(db)]
    if operation == "init":
        args += ["--username", "Admin"]

    def controlling_terminal():
        os.setsid()
        fcntl.ioctl(slave, termios.TIOCSCTTY, 0)

    proc = subprocess.Popen(args, stdin=slave, stdout=slave, stderr=slave,
                            preexec_fn=controlling_terminal)
    output = bytearray()
    try:
        deadline = time.monotonic() + 4
        while termios.tcgetattr(slave)[3] & termios.ECHO:
            assert time.monotonic() < deadline, "hidden prompt timeout"
            assert proc.poll() is None, "exited before hidden prompt"
            time.sleep(.005)
        # 输入尚未完成的口令也不能回显；信号/EOF 后检查输出。
        if action in ("sigint", "sigterm", "ctrl-c", "double", "normal"):
            os.write(master, SECRET)
        if action == "normal":
            os.write(master, b"\n")
        elif action == "eof":
            os.write(master, b"\x04")
        elif action == "ctrl-c":
            os.write(master, b"\x03")
        else:
            proc.send_signal(signal.SIGTERM if action == "sigterm" else signal.SIGINT)
            if action == "double":
                proc.send_signal(signal.SIGINT)
        result = proc.wait(timeout=4)
        while select.select([master], [], [], 0)[0]:
            output.extend(os.read(master, 4096))
        restored = termios.tcgetattr(slave) == original
        restored_flags = fcntl.fcntl(slave, fcntl.F_GETFL) == flags
        assert SECRET not in output, "password leaked"
        if expect_broken:
            assert result == -signal.SIGINT and not restored
            print("PASS old control: exit=-2, termios remains changed")
            return
        assert restored, (operation, action, "termios not restored")
        assert restored_flags, "file status flags not restored"
        assert result == (0 if action == "normal" else 1), (operation, action, result)
        if action != "normal":
            assert b"admin: password input failed" in output, "missing safe error"
    finally:
        if proc.poll() is None:
            proc.kill()
            proc.wait(timeout=4)
        termios.tcsetattr(slave, termios.TCSANOW, original)
        fcntl.fcntl(slave, fcntl.F_SETFL, flags)
        os.close(master)
        os.close(slave)


def administrator(db):
    with sqlite3.connect(db) as conn:
        return conn.execute("SELECT * FROM administrator").fetchall()


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("binary")
    parser.add_argument("--expect-broken", action="store_true")
    opts = parser.parse_args()
    with tempfile.TemporaryDirectory() as directory:
        root = Path(directory)
        if opts.expect_broken:
            tty_case(opts.binary, root / "old.db", "init", "sigint", True)
            assert not (root / "old.db").exists()
            return
        db = root / "admin.db"
        tty_case(opts.binary, db, "init", "normal")
        before = administrator(db)
        image = db.read_bytes()
        for action in ("sigint", "sigterm", "ctrl-c", "double", "eof"):
            fresh = root / (action + ".db")
            tty_case(opts.binary, fresh, "init", action)
            assert not list(root.glob(action + ".db*")), "cancelled init created DB"
            tty_case(opts.binary, db, "reset-password", action)
            assert db.read_bytes() == image, "cancelled reset changed DB bytes"
            assert administrator(db) == before, "cancelled reset changed administrator"
        # 重复取消覆盖调度时序；含真实控制终端 Ctrl-C 和直接信号两种。
        for i in range(15):
            fresh = root / ("repeat" + str(i) + ".db")
            tty_case(opts.binary, fresh, "init", "ctrl-c" if i % 2 else "sigint")
            assert not fresh.exists()
        tty_case(opts.binary, db, "reset-password", "normal")
        after = administrator(db)
        assert after[0][0:2] == before[0][0:2]
        assert after[0][3] == before[0][3] + 1
        # 独立进程共享数据库：唯一初始化与重置代次原子递增。
        shared = root / "process.db"
        def invoke(operation):
            args = [opts.binary, "admin", operation, "--db", str(shared)]
            if operation == "init":
                args += ["--username", "Admin"]
            result = subprocess.run(args, input=SECRET + b"\n", capture_output=True, timeout=8)
            assert SECRET not in result.stdout + result.stderr
            return result.returncode
        with ThreadPoolExecutor(max_workers=2) as pool:
            assert sorted(pool.map(invoke, ["init", "init"])) == [0, 1]
            assert list(pool.map(invoke, ["reset-password", "reset-password"])) == [0, 0]
        rows = administrator(shared)
        assert len(rows) == 1 and rows[0][3] == 3
        print("PASS independent CLI processes: competing init + two atomic resets")
        print("PASS real PTY: normal init/reset, SIGINT/SIGTERM/Ctrl-C/double/EOF,")
        print("full termios + fd flags restored, secrets hidden, cancellation leaves DB unchanged (25 cancellations)")


if __name__ == "__main__":
    main()
