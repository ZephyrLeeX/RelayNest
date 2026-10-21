# P00-04 Android 与 Firebase 环境登记

日期：2026-10-21。本报告由 P2 FCM 线会话登记；registry 状态由总调度维护，此处不复制。

## 已登记信息（用户提供）

| 项 | 值 | 来源 |
| --- | --- | --- |
| 目标真机 | 一加 Ace 3 Pro（`PJX110`，device `OP5D06L1`，国行 CN01 变体） | adb 实测 |
| 软件版本 | `PJX110_16.0.5.1001(CN01)`，ColorOS `V16.0.0` | adb `getprop` 实测，与用户口述一致 |
| 系统 | Android 16（API 36） | adb 实测 |
| Firebase 测试项目 | RelayNest（用户已创建；实际 project_id 为 `raleynest`，疑似创建时手误少一个 y，纯标识符不影响功能） | 控制台配置文件复核 |
| Android 包名 | `top.jianxinli.relaynest` | 用户指定 |
| `google-services.json` | 已下载并验证：单一 client、包名匹配、mobilesdk_app_id 存在、api_key 存在但未读取内容；oauth_client 为空（FCM 不需要 SHA-1） | 2026-10-21 实测 |

未擅自排除其他用户设备：当前仅登记此一台真机，其余设备以用户后续补充为准。

## 开发机现状（2026-10-21 实测）

已具备：

- `adb` 37.0.0p1（Arch android-tools）、`git`、`curl`、`python3`（mise）。
- 出口访问 `https://fcm.googleapis.com` 返回 HTTP 404（连通正常，发送桩走 443 即可）；`firebase.google.com` HTTP 200。
- `mtalk.google.com:5228` 从开发机直连超时；手机侧 FCM 长连接可达性属真机验证项，不在开发机下结论。

工具链（2026-10-21 用户指示改由 mise 管理，已装齐）：

- 项目根 `mise.toml`：`java = "temurin-21"`（实际 21.0.12 LTS，mise 核心后端）、`android-sdk = "23.0"`（vfox:jdx 插件 = cmdline-tools 23.0）、`python = "3.13"`（实际 3.13.15，发送桩用）。`mise.toml` 暂不入库，待 P02-01/P03 工程脚手架一并提交。
- `sdkmanager` 已装：`build-tools;36.0.0`、`platform-tools` 37.0.1、`platforms;android-36`。许可已接受。
- `ANDROID_HOME` 由插件指向 `~/.local/share/mise/installs/android-sdk/23.0`。
- Python 侧已装 `google-auth` 2.62.0 + `requests`（发送桩 OAuth 用）。

服务账号凭据验证（2026-10-21 实测）：

- 私钥文件 `~/.config/relaynest-dev/raleynest-firebase-adminsdk-fbsvc-f33ac2eba3.json`，`type=service_account`、`project_id=raleynest`、`client_email=firebase-adminsdk-fbsvc@raleynest.iam.gserviceaccount.com`。
- 以 `https://www.googleapis.com/auth/firebase.messaging` scope 成功换取访问令牌（凭据有效 True，UTC 过期时间正常顺延 1 小时）。未输出令牌内容，未发送任何消息。

## 待用户完成清单

1. ~~Firebase 控制台：添加 Android 应用并下载 `google-services.json`~~ **已完成**。文件已暂存 `~/.config/relaynest-dev/google-services.json`（SHA-256 `7b01d84…a454fb`），原下载件留在 `~/Downloads`。
2. ~~Firebase 控制台：生成服务账号私钥~~ **已完成并验证**（见上节；文件名含指纹段 `f33ac2eba3`，内容不入报告）。
3. ~~真机连接~~ **已完成（无线 adb，用户暂无数据线）**：mDNS 发现 `192.168.1.110:41519`，连接并授权成功（电脑 192.168.1.111 同网段），设备识别为 `PJX110`。
4. ~~真机 GMS 确认~~ **已完成**：`com.google.android.gms` versionName 26.41.30 已装且未停用，`com.android.vending`（Play 商店）存在，无被停用的谷歌包。ColorOS 后台限制基线留待 P02-03 按场景实测。
5. ~~minSdk 确认~~ **已确认（2026-10-21 用户拍板）：minSdk 29（Android 10）**，PoC 与产品一致采用。

## 秘密与证据约定

- `google-services.json` 属客户端配置，构建期需要；将放 `android/poc/fcm/app/`，并在 `android/poc/fcm/.gitignore` 中排除，报告中仅记 SHA-256 指纹。
- 服务账号私钥仅存 `~/.config/relaynest-dev/`，运行时读取，不打印内容。
- 报告不包含任何凭据明文。

## 结论

清单全部闭合（Firebase 客户端配置、服务账号凭据、无线 adb 连接、GMS 复核、minSdk 29）。开发机工具链（mise：java 21 / cmdline-tools 23 / build-tools 36 / platform-tools 37 / android-36 / python 3.13 + google-auth）就绪，P02-01 的外部门槛满足。本报告为 worker 交付；独立审计、registry 状态更新与集成由总调度线完成。

遗留：手机侧 FCM 长连接可达性与 ColorOS 后台行为属 P02-01～03 实测项，本卡不下结论；无线 adb 端口会变化，重连时需重新发现（mDNS 可用，`adb mdns services`）。
