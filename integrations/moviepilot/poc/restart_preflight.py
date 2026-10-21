"""在Docker宿主机重启前只读检查绑定源；不读取容器环境、不执行启停。"""

import json
from pathlib import Path
import subprocess


def check_bind_sources(mounts):
    rows = []
    for mount in mounts:
        if mount.get("Type") != "bind":
            continue
        source = mount["Source"]
        try:
            Path(source).stat()
            available, error = True, None
        except OSError as failure:
            available, error = False, failure.errno
        rows.append({"source": source, "target": mount["Destination"], "available": available, "errno": error})
    return rows


def main():
    try:
        container = json.loads(subprocess.check_output(
            ["docker", "inspect", "moviepilot-v2"], stderr=subprocess.PIPE))[0]
    except (subprocess.CalledProcessError, OSError, ValueError):
        print(json.dumps({"bind_sources_available": False, "reason": "docker_inspect_failed"}))
        raise SystemExit(1)
    rows = check_bind_sources(container["Mounts"])
    available = bool(rows) and all(row["available"] for row in rows)
    print(json.dumps({"container": "moviepilot-v2", "state": container["State"]["Status"],
                      "bind_sources_available": available, "binds": rows}, ensure_ascii=False))
    if not available:
        raise SystemExit(1)


if __name__ == "__main__":
    main()
