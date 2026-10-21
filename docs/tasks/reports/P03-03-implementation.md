# P03-03 Android 最小原生工程实施报告

## 范围与状态

- 角色：developer；基线：`fefad6f1ced17aa7a2dd328f8683d1ae8397f1fe`。
- 工作目录：`/tmp/relaynest-run2/android`，分支：`codex/android-run2`。
- 实施日期：2026-10-21；实现 SHA 在提交后交接，不预写自身 SHA。
- 代码实现可交独立审计；最小 debug APK 与相关检查通过。目标真机兼容验收为 **NOT_RUN / blocked**，不将本卡或整个 Android 验收标为 done。
- 仅新增 `android/` 正式入口和本报告。`android/poc/fcm` 未修改，未纳入 settings。未修改技术设计、台账、其他任务、后端或 RelayShelf。

## 实现

建立 `:app` Compose 空应用，唯一界面为占位文本。必要 core 仅为应用内 `core/ui/RelayNestTheme.kt`，跟随系统深浅色，并应用 safe drawing inset；没有网络、FCM、厂商推送、登录、Room、同步或业务命令/MCP。

沿用 AGP 9.4.1 / Gradle 9.8.2 / JDK 21；本目录 mise 将已安装 JDK 精确固定到 `temurin-21.0.12+101.0.LTS`。官方 AGP POM 核对 KGP 2.2.10，Compose 编译插件同为 2.2.10，不应用 Kotlin Android 插件。固定稳定 Compose BOM 2025.08.00、Activity Compose 1.10.1，compile/target 36、min 29、Build Tools 36.0.0，包名 `top.jianxinli.relaynest`。wrapper 复用 PoC 已有脚本/JAR，另添加官方 Gradle 发行包 SHA-256；没有升级或迁移 PoC。

版本 catalog 固定直接依赖，Gradle 生成 `app/gradle.lockfile` 锁定本卡实际解析的 debug 相关传递依赖。`settings-gradle.lockfile` 是 Gradle 自动生成的空 catalog 配置锁。release 未构建，未声称其配置已锁定。

`android/README.md` 已记录 debug 与正式签名差异、同包名升级的签名连续性、仓库外文件/环境变量加载和受保护 CI Secret 注入约定。当前 release 无签名配置；本卡没有正式密钥、口令或厂商注册操作。AGP 默认 debug 签名仅在本机工具私有目录使用；不读取/导出/提交密钥。OPPO 包名/证书约束尚未核实，未作为事实。

## 环境与实际验证

WSL2 Linux x86_64；JDK 实际输出 Eclipse Adoptium `21.0.12.1+1-LTS`；Gradle 输出 `9.8.2`。Gradle `--version` 中的 Kotlin 2.4.10 是 Gradle 自身嵌入版本，不能冒充应用编译器；应用 KGP 固定 2.2.10。

所有下表 Gradle 命令在 `android/` 执行，实际环境前缀为：

```sh
JAVA_HOME=/home/lijx/.local/share/mise/installs/java/temurin-21
ANDROID_HOME=/home/lijx/.local/share/mise/installs/android-sdk/23.0
```

两值在执行时以命令前缀注入。初始 PATH 无 mise/Java/SDK，使用绝对路径及 wrapper。已有 JDK 可用；wrapper 下载既定 Gradle。运行以下 SDK 准备命令成功，未使用 `yes` 或人工代接受提示：

```sh
JAVA_HOME=/home/lijx/.local/share/mise/installs/java/temurin-21 /home/lijx/.local/share/mise/installs/android-sdk/23.0/cmdline-tools/23.0/bin/sdkmanager 'platforms;android-36' 'build-tools;36.0.0' 'platform-tools' </dev/null
```

工具输出转发到新版 Android CLI 后，三项下载解压成功，退出码 0。当前环境可构建，不沿用旧环境报告假称 SDK 已齐备。

| 实际命令 | 结果与证据摘要 |
| --- | --- |
| `./gradlew --version` | PASS，退出 0，Gradle 9.8.2 / Launcher JVM 21.0.12.1 |
| `/home/lijx/.local/share/mise/installs/java/temurin-21/bin/java -jar /tmp/relaynest-ktlint-1.7.1.jar '*.gradle.kts' 'app/**/*.kt' 'app/**/*.gradle.kts'` | PASS，退出 0；初次发现 Compose PascalCase 命名，按官方 Compose 约定配置 annotated-function 规则后复验通过 |
| `./gradlew :app:lintDebug :app:assembleDebug --console=plain` | PASS，退出 0，首次真实编译，46 tasks executed，2m38s |
| `./gradlew :app:lintDebug :app:assembleDebug --write-locks --console=plain --warning-mode=all` | PASS，退出 0，持久化依赖锁，8 executed / 38 up-to-date，4s |
| `./gradlew :app:lintDebug :app:assembleDebug --offline --console=plain` | PASS，退出 0，使用已生成锁离线复验，11 executed / 35 up-to-date，7s |
| `/home/lijx/.local/share/mise/installs/android-sdk/23.0/build-tools/36.0.0/aapt2 dump badging app/build/outputs/apk/debug/app-debug.apk` | PASS，包名符合，min 29 / target 36，debuggable，原生库含 arm64-v8a；未声明 INTERNET 或通知权限 |
| `timeout 10s /home/lijx/.local/share/mise/installs/android-sdk/23.0/platform-tools/adb devices -l` | 环境探测超时，退出 124，无设备信息；不是安装/运行通过。首次无 timeout 探测也无输出，已结束该探测进程 |
| `git diff --check`、提交前暂存差异及路径检查 | PASS；仅本卡允许路径，无 PoC 变更、密钥或构建产物入库 |

ktlint 1.7.1 CLI 从 Maven Central 下载至 `/tmp`，不是 app 依赖，也未入库。工具 SHA-256：`5f3de4ee4014137b05c72ed68eb46fe686aeb11fbdef27faf77f55d51c1b2536`。

Lint 结果：0 errors / 7 warnings，保留原有检查、不关闭警告：目标 SDK 非最新、compile SDK/Compose BOM/Activity/Kotlin 编译插件存在新版本共 5 条；另有空工程无 launcher icon 与未配置 Android 12 dataExtractionRules 两条。版本警告不据此升级已锁工具链。当前没有持久业务数据；后续引入会话/缓存前必须落实备份排除并复核设备转移规则，图标随正式 UI 补充。

工具链另报告 `Configuration.setVisible(boolean)` 将在未来 Gradle 移除；本卡源码未调用该 API。`libandroidx.graphics.path.so` 在无 NDK strip 工具环境下原样打入 debug APK；没有把该警告写成发布质量通过。

构建产物（仅本地、不提交）：`android/app/build/outputs/apk/debug/app-debug.apk`，25,225,950 字节，SHA-256 `b62fb153ee80acbdda444c8e45816b3a54d391fafeba5ca0123350e0656dabca`。完整 Lint 本地报告：`android/app/build/reports/lint-results-debug.{txt,xml,html,sarif}`，不提交生成文件。

## 验收与交接

| 卡片验收项 | 状态 |
| --- | --- |
| 兼容目标一加 Ace 3 Pro / Android 16 / ColorOS 16 | NOT_RUN / blocked；仅 SDK/ABI 配置与 APK 元数据吻合，未连接真机，不替代运行验收 |
| 最小 debug APK 编译 | PASS |
| 无业务实现，PoC 排除 | PASS，settings 仅含 `:app`；源码无业务或推送接入 |
| 格式/静态检查及命令证据 | PASS，ktlint 与 Lint 实际执行；警告如上 |
| 本卡单独提交与审计 | 交接提交后由另一 agent 针对精确 SHA 审计；尚未审计/集成 |

NOT_RUN：真机安装、冷启动、深浅色、系统栏/导航 inset 视觉检查；正式签名/密钥加载器/CI 注入/分发；release 构建；厂商平台注册与推送；业务/仪器测试。没有为纯占位 UI 编写镜像实现的单元测试，也没有将编译中的 `preDebugAndroidTestBuild SKIPPED` 计为设备测试。

最小后续输入：恢复可靠 adb 设备连接并由维护者提供目标机；在覆盖安装前确认旧 PoC 的保留/卸载安排（同包名可能不同签名），本卡未执行安装或卸载。独立审计可先验证固定提交和构建证据，真实兼容门槛保持 blocked；后续发布任务按 README 约定实现正式密钥安全加载和受保护 CI。
