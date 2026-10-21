#!/usr/bin/env python3
"""P02-01 FCM data-only 消息发送桩。

用法（仓库根目录，mise 环境已装 google-auth/requests）：
  python3 android/poc/fcm/tools/send_data_message.py --token <设备token全文> [--priority high]

约束：
- 服务账号私钥从 ~/.config/relaynest-dev/ 读取，绝不入库、不打印。
- 只发送 data 载荷（type/deployment_id/watermark），不含正文、URL 或凭据，
  与技术设计 T9.1 的推送形态一致。
- 默认 normal 优先级；--priority high 仅用于 P02-02 的对照实验。
"""
import argparse
import glob
import os
import sys

import requests
from google.auth.transport.requests import Request
from google.oauth2 import service_account

PROJECT_ID = "raleynest"
SCOPES = ["https://www.googleapis.com/auth/firebase.messaging"]
FCM_ENDPOINT = f"https://fcm.googleapis.com/v1/projects/{PROJECT_ID}/messages:send"
KEY_GLOB = os.path.expanduser("~/.config/relaynest-dev/raleynest-firebase-adminsdk*.json")


def load_credentials():
    # 私钥只在本机读取并换取短期访问令牌，不输出任何凭据内容
    keys = sorted(glob.glob(KEY_GLOB))
    if not keys:
        sys.exit("未找到服务账号私钥，请检查 ~/.config/relaynest-dev/")
    creds = service_account.Credentials.from_service_account_file(keys[0], scopes=SCOPES)
    creds.refresh(Request())
    return creds


def main():
    parser = argparse.ArgumentParser(description="P02-01 FCM data-only 发送桩")
    parser.add_argument("--token", required=True, help="目标设备注册 token（全文）")
    parser.add_argument("--priority", choices=["normal", "high"], default="normal",
                        help="消息优先级，默认 normal（T9.1 约束）")
    parser.add_argument("--deployment-id", default="poc-deployment-1",
                        help="不透明部署标识（默认 poc-deployment-1）")
    parser.add_argument("--watermark", default="42", help="目标水位（字符串，默认 42）")
    args = parser.parse_args()

    creds = load_credentials()
    # data-only：仅含不透明部署 ID、目标水位和类型
    body = {
        "message": {
            "token": args.token,
            "data": {
                "type": "sync_hint",
                "deployment_id": args.deployment_id,
                "watermark": args.watermark,
            },
        }
    }
    if args.priority == "high":
        # Android transport 优先级：high 走高优先级通道，仅在对照实验中使用
        body["message"]["android"] = {"priority": "high"}

    resp = requests.post(
        FCM_ENDPOINT,
        json=body,
        headers={"Authorization": f"Bearer {creds.token}"},
        timeout=20,
    )
    # 输出只含脱敏 token 与响应摘要；成功响应的 name 形如 projects/.../messages/0:...
    masked = args.token[:8] + "…" + args.token[-4:]
    print(f"target={masked} priority={args.priority}")
    print(f"HTTP {resp.status_code}")
    print(resp.text[:500])
    sys.exit(0 if resp.ok else 1)


if __name__ == "__main__":
    main()
