"""在 MP 容器内运行的固定探针客户端；不输出或落盘 API 凭据。"""

import json
import os
import sys
import urllib.error
import urllib.request

from dotenv import dotenv_values


def main():
    """仅允许固定探针及补丁助手动作，凭据通过容器 loopback 请求头使用。"""
    routes = {
        "installed": ("GET", "/plugin/installed"),
        "patch-register-local": ("POST", "/system/env"),
        "patch-source": ("GET", "/plugin/source/RelayNestPatch/options"),
        "patch-install": ("GET", "/plugin/install/RelayNestPatch"),
        "patch-enable": ("POST", "/plugin/instance/RelayNestPatch/enabled"),
        "patch-load": ("POST", "/plugin/reload/RelayNestPatch"),
        "patch-form": ("GET", "/plugin/form/RelayNestPatch"),
        "patch-status": ("GET", "/plugin/RelayNestPatch/patch/status"),
        "patch-apply": ("POST", "/plugin/RelayNestPatch/patch/apply"),
        "enable": ("POST", "/plugin/instance/RelayNestProbe/enabled"),
        "disable": ("POST", "/plugin/instance/RelayNestProbe/enabled"),
        "load": ("POST", "/plugin/reload/RelayNestProbe"),
        "status": ("GET", "/plugin/RelayNestProbe/status"),
        "probe": ("POST", "/plugin/RelayNestProbe/probe"),
        "text-preflight": ("GET", "/plugin/RelayNestProbe/text_preflight"),
        "text-probe": ("POST", "/plugin/RelayNestProbe/text_probe"),
        "transport-probe": ("POST", "/plugin/RelayNestProbe/transport_probe"),
        "callback-probe": ("POST", "/plugin/RelayNestProbe/callback_probe"),
        "candidate-probe": ("POST", "/plugin/RelayNestProbe/candidate_probe"),
        "unload": ("DELETE", "/plugin/RelayNestProbe"),
    }
    if len(sys.argv) != 2 or sys.argv[1] not in routes:
        raise SystemExit("参数：" + " / ".join(routes))
    action = sys.argv[1]
    method, path = routes[action]
    values = dotenv_values("/config/app.env")
    token = os.environ.get("API_TOKEN") or values.get("API_TOKEN")
    if not token:
        raise SystemExit("缺少 API_TOKEN，未生成或修改任何凭据")
    port = int(os.environ.get("PORT") or values.get("PORT") or "3001")
    data = json.dumps({"enabled": action in {"enable", "patch-enable"}}).encode() if action in ("enable", "disable", "patch-enable") else None
    if action == "patch-register-local":
        # 仅追加本助手的固定持久仓库，不覆盖用户已有仓库，不写其它环境配置。
        old_paths = os.environ.get("PLUGIN_LOCAL_REPO_PATHS") or values.get("PLUGIN_LOCAL_REPO_PATHS") or ""
        paths = [item.strip() for item in old_paths.split(",") if item.strip()]
        if "/config/relaynest-local-plugins" not in paths:
            paths.append("/config/relaynest-local-plugins")
        data = json.dumps({"PLUGIN_LOCAL_REPO_PATHS": ",".join(paths)}).encode()
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
