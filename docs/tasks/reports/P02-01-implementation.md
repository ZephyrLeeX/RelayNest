# P02-01 最小 Android FCM 注册 PoC 实测

日期：2026-10-21。基线 SHA：`061c0b8`。实现 SHA 见本地提交。设备与 Firebase 环境见 [P00-04 报告](P00-04-android-firebase-env.md)。

## 结论

**已在目标真机（一加 Ace 3 Pro / Android 16 / ColorOS V16.0.0）完成 FCM 注册并接收 data-only 提示，端到端延迟约 0.1 秒。** 发送桩经开发机（Hub 侧网络代表）调用 FCM HTTP v1 返回 HTTP 200，设备端 `onMessageReceived` 收到与服务端完全一致的消息 ID 和完整 data 载荷。载荷只含 `type/deployment_id/watermark`（T9.1 形态），不含正文、URL 或凭据；token 全文仅存应用私有文件，日志、界面与报告只出现脱敏形态。

**重要负面发现：手机在未开代理的家庭网络下 FCM 注册失败**（`firebaseinstallations.googleapis.com` 不可达，SDK 约 160 秒后放弃且不再自动重试）；手机侧开启代理后 6 秒完成注册。对产品的影响：**推送通道依赖 Google 可达网络，WorkManager 补偿（P02-02）是必需能力而非优化项**。

本卡不覆盖：后台/锁屏/Doze 时序（P02-02）、Token 轮换与异常矩阵（P02-03）、通知展示（P09）。

## 范围与实现

新增 `android/poc/fcm/`（隔离 Gradle 工程，可整体移除）：

- `settings.gradle.kts` / 根与模块 `build.gradle.kts` / `gradle.properties` / wrapper（Gradle 9.8.2）。
- 版本锁定（2026-10-21 取自 Google Maven / Maven Central 元数据最新稳定版）：AGP **9.4.1**（内置 Kotlin，未 apply kotlin-android）、google-services 插件 **4.5.0**、firebase-messaging **25.1.3**；compileSdk/targetSdk 36、minSdk 29（用户确认）。
- `app/src/main/java/top/jianxinli/relaynest/`：`MainActivity`（框架 TextView，无 UI 框架依赖）、`FcmPoCService`（data-only 接收，记录消息 ID/优先级/时间戳/键值，不弹通知）、`PoCLog`（进程内日志 + 脱敏工具 + logcat 镜像 tag=RelayNestFCM）、`TokenStore`（token 全文仅落 `files/fcm_token.txt`）。
- `tools/send_data_message.py`：FCM HTTP v1 发送桩，服务账号私钥从 `~/.config/relaynest-dev/` 读取，默认 normal 优先级，输出不含 token 全文。
- `evidence/registration-and-receipt.png`：设备界面截图（注册成功 + 两次接收记录）。
- 仓库根 `mise.toml` 随本卡入库（java/gradle/android-sdk/python 工具链声明，P00-04 已预告）。

`app/google-services.json` 由 `.gitignore` 排除，不入库（SHA-256 见 P00-04 报告）。

## 环境与操作

- 构建：`./gradlew :app:assembleDebug` 首次全量 37 任务成功，增量约 15 秒；无告警错误。
- 安装：`adb install -r`（无线 adb，192.168.1.110，mDNS 发现，无需配对码）。
- 发送：`mise exec -- python3 tools/send_data_message.py --token <全文> --watermark 101`（token 经 `adb shell run-as top.jianxinli.relaynest cat files/fcm_token.txt` 读取，仅在本地 shell 变量中传递）。

## 证据

| 时间（设备） | 事件 |
| --- | --- |
| 15:28:40.600 | App 启动，开始 FCM 注册（手机代理已开） |
| 15:28:46.758 | token 获取成功：`eNXTP-4t…(len=142)…RgdY` |
| 15:30:52.998→.092 | 消息 1：发送 `0:1792567853005066%cec58810f9fd7ecd`，HTTP 200；94ms 后 `onMessageReceived`，ID 一致，`prio=2/2`，`data=[type=sync_hint, watermark=101, deployment_id=poc-deployment-1]` |
| 15:31:33.648→.765 | 消息 2：`0:1792567893656109%cec58810f9fd7ecd`，HTTP 200；117ms 后收到，watermark=102，ID 一致 |

对照失败记录（未开代理）：15:20:19 启动 → 15:23:00 `token 获取失败: IOException: FCM Registration failed!`（FirebaseInstallations 网络调用失败，"Won't retry the operation"）。同网络下手机 ping `firebaseinstallations.googleapis.com`（解析为 172.217.117.4）100% 丢包；`mtalk.google.com` 与 `142.250.x` 网段可达（0.47ms 应答，路由器透明代理按 IP 段拦截）。

## 验收核对

- [x] 目标真机收到提示：两条 data 消息均到达，ID 与载荷完整。
- [x] Hub 测试网络可访问 FCM：发送桩两次 HTTP 200。
- [x] 不含正文/秘密：data-only 载荷、脱敏日志、token 私有存储、私钥不入库。
- [x] 受影响检查：本卡无独立单元测试（PoC 无库代码），验证即上述真机构建+安装+收发实测；未运行与本项目无关的其他套件。
- [x] 仅提交本任务变更（见 diff 摘要）；独立审计与集成完成后才可 `done`。

## 遗留问题

1. 未代理网络注册失败的量化与降级验证留给 P02-02/03（含 Doze、锁屏、force-stop）。
2. logcat 主缓冲会刷掉早期行，P02-02 需要时序证据时应先 `adb logcat -c` 再分段抓取，或让 PoC 落盘。
3. 网络环境记录：开发机隧道（wg0 + singtun0）切换时 IPv6 路径抖动曾致发送桩 SSL EOF，IPv4 稳定；Hub 正式部署（fnOS）的 Google 出口方案需在 P9/P12 复核。
4. AGP 9 迁移资料中 `android { compilerOptions {} }` 写法有误，实际应为顶层 `kotlin {}` 或省略（jvmTarget 默认跟随 `targetCompatibility`），已在工程中按官方迁移文档处理。
