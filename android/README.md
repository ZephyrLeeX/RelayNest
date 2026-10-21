# Android 最小原生工程

正式工程入口为本目录，只纳入 `:app`。`poc/fcm` 使用自己的 settings/wrapper，未纳入正式构建；本应用没有 Firebase、厂商推送、网络客户端、登录、数据库或业务操作。

## 工具链与构建

| 项目 | 固定值 |
| --- | --- |
| JDK | Temurin 21.0.12+101.0.LTS（mise 标识，实际 java 报告 21.0.12.1） |
| Gradle | 9.8.2，wrapper 带官方 SHA-256 |
| AGP | 9.4.1，沿用已有 PoC 工具链 |
| Kotlin / Compose 编译插件 | 2.2.10，与 AGP POM 内置 KGP 一致 |
| Compose BOM | 2025.08.00，固定稳定版本 |
| Activity Compose | 1.10.1 |
| SDK / Build Tools | compile/target 36、min 29 / 36.0.0 |
| Java / Kotlin 字节码 | 17 |
| applicationId / namespace | `top.jianxinli.relaynest` |

使用 `mise` 安装本目录锁定工具（按本机配置完成 trust），或通过 `JAVA_HOME` 指定同版本 JDK；使用 wrapper，无需安装另一套 Gradle。先准备 SDK `platforms;android-36`、`build-tools;36.0.0`，通过 `ANDROID_HOME` 或被忽略的 `local.properties` 指向 SDK。

在本目录执行：

```sh
./gradlew --version
./gradlew :app:lintDebug :app:assembleDebug --console=plain
```

格式检查采用 ktlint CLI 1.7.1（独立开发工具，不是应用依赖），遵循本目录 `.editorconfig`，只检查正式工程：

```sh
java -jar /path/to/ktlint-cli-1.7.1-all.jar '*.gradle.kts' 'app/**/*.kt' 'app/**/*.gradle.kts'
```

`app/gradle.lockfile` 记录已解析的 debug 编译、运行及 Lint 配置，普通构建读取锁；增改依赖后使用 `./gradlew :app:lintDebug :app:assembleDebug --write-locks` 有意更新并审查差异。release 未在本卡构建或锁定，后续发布任务需补充其依赖锁定及验证。

debug 产物为 `app/build/outputs/apk/debug/app-debug.apk`，不入库。基础 UI 主题暂放 `app` 内的 `core/ui` 包；后续有实际复用需求再提取模块，不预建空 network/sync 模块。主题跟随系统深浅色，内容使用 safe drawing inset 避免 Android 16 边到边布局遮挡。

目标为一加 Ace 3 Pro / Android 16 / ColorOS 16。SDK 配置覆盖目标 API，不代表真机兼容已通过；需连接设备后验证安装、冷启动、深浅色及系统栏布局。正式 app 与旧 PoC 包名相同但签名可能不同，安装前需确认现有 PoC 的保留与迁移要求，不自动卸载或覆盖设备应用。

## 签名准备约定

debug 使用 AGP 默认开发签名，仅供本机开发，不作为正式发布身份。AGP 可能在用户工具私有目录自动建立 debug keystore；不要读取、上传或提交该文件。正式版本必须使用维护者持有并安全备份的长期签名密钥，同一包名的升级需满足 Android 的签名连续性要求。debug 与正式签名不同，不能假定两者可直接覆盖安装。

默认不启用正式签名，也不读取四项外部签名配置；debug 无配置仍可构建。默认 release 没有签名配置，不得将 unsigned APK 当作可发布包。只有显式请求才接入外部 release signingConfig，缺失配置直接失败，绝不回退 debug 签名。本工程不生成、保管或提交正式 keystore/口令。

现有加载器只从以下环境变量读取配置；必须由秘密管理器或受保护作业预先注入，四项非空白才有效：

| 环境变量 | 安全来源 |
| --- | --- |
| `RELAYNEST_ANDROID_KEYSTORE_PATH` | 仓库外、仅构建用户可读的密钥文件路径 |
| `RELAYNEST_ANDROID_KEYSTORE_PASSWORD` | 本机秘密管理器或 CI 加密 Secret |
| `RELAYNEST_ANDROID_KEY_ALIAS` | 本机秘密管理器或 CI 加密 Secret |
| `RELAYNEST_ANDROID_KEY_PASSWORD` | 本机秘密管理器或 CI 加密 Secret |

在本目录执行以下入口（命令不携带任何秘密值）：

```sh
# 仅完整性诊断，不读取 keystore 内容、不接入签名、不构建 release。
./gradlew :app:checkReleaseSigningConfig --no-configuration-cache --console=plain

# 仅供后续获授权的正式签名环境；本次未执行真实签名。
./gradlew :app:assembleRelease -PrelaynestReleaseSigning=true --no-configuration-cache --console=plain
```

诊断和显式签名请求都检查四项完整性及文件引用存在、为普通文件且可读。缺少/空白字段只列环境变量名；无效引用只输出固定原因，不输出值、路径、别名或文件内容。签名请求在配置阶段检查，失败不进入 release 签名任务。诊断成功仅表示配置齐备及文件元数据有效，不验证 keystore 格式、口令、别名或签名；空文件也可以通过此诊断，不能据此声称可签名。引用应使用仓库外绝对路径。

`relaynestReleaseSigning` 只有精确值 `true` 启用；省略它不会因环境变量齐备就自动签名。签名请求拒绝配置缓存，防止含口令的签名模型被持久化。不要对签名作业开启 Gradle debug 日志、构建扫描或输出环境变量；不要运行可能显示证书/别名/路径的诊断任务。完整性检查后的 AGP 实际签名失败属于后续授权环境验证范围，原始工具日志应私有保管并脱敏后再分享。release 依赖锁定和真实签名有效性仍需后续发布任务验证。

本地回归可运行 `bash tools/check-signing-config.sh`（需要 Bash、rg 和已有离线 Gradle/SDK 缓存）。脚本屏蔽继承的签名环境，只用自身临时空文件和非秘密标记测试缺项、空白字段、无效/不可读引用、空文件诊断及日志脱敏，结束删除临时目录；不生成密钥，不执行成功签名。

维护者在发布前确认正式保管方案：长期密钥置于仓库外私有目录，限制构建用户读取；独立安全备份并验证恢复流程，口令交秘密管理器保管。本卡不执行密钥生成、备份或真实签名。CI 只在受保护、明确授权的发布作业从加密 Secret/文件存储恢复到临时私有目录（例如目录 0700、文件 0600），注入上述环境变量；用退出清理机制覆盖成功、失败与取消，删除临时文件并清除环境。不要向 PR/fork 作业暴露秘密，不缓存/上传临时目录、配置缓存或原始签名日志；不要把秘密放在命令行参数、仓库 Gradle 属性、日志或产物。只上传经校验的 APK/AAB。本卡未添加 CI 工作流或发布操作。

未注册 OPPO 开放平台，未核实其包名/证书摘要等厂商约束，不将这些约束写成事实。推送方案及厂商注册由独立规划和后续任务负责。

## 版本与签名依据

核对日期：2026-10-21。普通稳定依赖选择用于最小 Compose 工程，不升级已锁工具链。

- [AGP 9.4 兼容表](https://developer.android.com/build/releases/agp-9-4-0-release-notes)：Gradle 最低 9.6.0、Build Tools 36.0.0、JDK 最低 17。
- [AGP 9.4.1 官方 POM](https://dl.google.com/dl/android/maven2/com/android/tools/build/gradle/9.4.1/gradle-9.4.1.pom)：KGP 2.2.10。
- [内置 Kotlin](https://developer.android.com/build/migrate-to-built-in-kotlin)：不再应用 Kotlin Android 插件，JVM target 默认跟随 compileOptions。
- [Compose 编译插件配置](https://developer.android.com/develop/ui/compose/setup-compose-dependencies-and-compiler)：编译插件跟随 Kotlin 版本。
- [Compose 2025 年 8 月稳定版](https://android-developers.googleblog.com/2025/08/whats-new-in-jetpack-compose-august-25-release.html)：BOM 2025.08.00。
- [Activity 版本记录](https://developer.android.com/jetpack/androidx/releases/activity#1.10.1)：Activity Compose 1.10.1。
- [Android 应用签名](https://developer.android.com/studio/publish/app-signing)：开发与发布签名、更新签名身份。
