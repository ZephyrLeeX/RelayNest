"""在 MP 容器内运行的固定探针客户端；不输出或落盘 API 凭据。"""

import json
import os
import sys
import urllib.error
import urllib.request

from dotenv import dotenv_values


def main():
    """仅允许操作 RelayNestProbe，凭据通过容器 loopback 请求头使用。"""
    routes = {
        "enable": ("POST", "/plugin/instance/RelayNestProbe/enabled"),
        "disable": ("POST", "/plugin/instance/RelayNestProbe/enabled"),
        "load": ("POST", "/plugin/reload/RelayNestProbe"),
        "status": ("GET", "/plugin/RelayNestProbe/status"),
        "probe": ("POST", "/plugin/RelayNestProbe/probe"),
        "unload": ("DELETE", "/plugin/RelayNestProbe"),
    }
    if len(sys.argv) != 2 or sys.argv[1] not in routes:
        raise SystemExit("参数：enable / load / status / probe / disable / unload")
    action = sys.argv[1]
    method, path = routes[action]
    values = dotenv_values("/config/app.env")
    token = os.environ.get("API_TOKEN") or values.get("API_TOKEN")
    if not token:
        raise SystemExit("缺少 API_TOKEN，未生成或修改任何凭据")
    port = int(os.environ.get("PORT") or values.get("PORT") or "3001")
    data = json.dumps({"enabled": action == "enable"}).encode() if action in ("enable", "disable") else None
    request = urllib.request.Request(
        f"http://127.0.0.1:{port}/api/v1{path}", method=method, data=data,
        headers={"X-API-KEY": token, "Content-Type": "application/json"},
    )
    try:
        with urllib.request.urlopen(request, timeout=30) as response:
            result = json.load(response)
            print(json.dumps({"action": action, "http_status": response.status, "result": result}, ensure_ascii=False))
            if isinstance(result, dict) and result.get("success") is False:
                raise SystemExit(2)
    except urllib.error.HTTPError as error:
        print(json.dumps({"action": action, "http_status": error.code}))
        raise SystemExit(1) from None


if __name__ == "__main__":
    main()
