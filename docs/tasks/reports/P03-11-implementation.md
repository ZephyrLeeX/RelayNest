# P03-11 Android 检查 CI 实施报告

日期：2026-10-22。角色：developer。基线：`491fc873657006f3f037b57175b35783a2ea388f`；工作树：`/tmp/relaynest-run3/android-ci`；分支：`codex/android-ci-run3`。实现提交 SHA 在提交后交接，不在本文件预写自身 SHA。

## 变更与边界

新增 `.github/workflows/android-check.yml`，对 PR 和 push 的 `android/**`、本 workflow 与根 `mise.toml` 变更触发。只运行正式 `android/:app` 的 ktlint 1.7.1、`lintDebug` 和 `assembleDebug`，沿用 JDK 21、wrapper Gradle 9.8.2、AGP 9.4.1、API 36、Build Tools 36.0.0、minSdk 29；未改源码、依赖锁或其他模块。

- 使用 `ubuntu-24.04`，由 GitHub 官方 setup-java 安装 Temurin JDK 21，使用 runner 已有 Android 命令行工具显式安装指定的两个 SDK 包，不执行工具链升级。
- 权限仅 `contents: read`，checkout 不持久化凭据；不使用 `pull_request_target`，不引用 Secret。四项正式签名配置在作业中设空，不启用 release 属性，不读取 Google/OPPO 测试配置，不签发或上传 APK。
- ktlint 从官方发布下载，验证官方 release API 记录的 SHA-256 后执行。格式失败后仍提供编译/Lint 结果，任一步失败均保持作业失败；Gradle 普通堆栈和 Lint 报告可见，报告上传保留七天，报告缺失会告警。
- 缓存只覆盖 Gradle `modules-2` 和 wrapper distributions；key 绑定 OS/架构/JDK 21、wrapper 属性、版本目录、所有 Android 依赖锁/Gradle 脚本、Gradle 属性及 mise 配置。不设置宽泛 restore-key；仅成功 push 保存，PR/fork 仅恢复，不缓存整个用户目录、debug keystore、配置缓存或构建产物。构建后检查锁和工具版本文件未改写。
- 本任务不需要范围外 helper；没有新增 helper 或外部依赖。
- 编译命令显式选择进程内 Kotlin 编译，不依赖跨构建残留的 Kotlin daemon；不改变 Kotlin 版本或字节码目标。

## 官方依据核查

以下版本 tag 在本轮通过 `git ls-remote` 核实对应不可变提交，并读取该 SHA 的官方 README；workflow 使用完整 SHA：

| Action | 版本 | SHA |
| --- | --- | --- |
| actions/checkout | v4.3.1 | `34e114876b0b11c390a56381ad16ebd13914f8d5` |
| actions/setup-java | v4.8.0 | `c1e323688fd81a25caa38c78aa6df2d33d3e20d9` |
| actions/cache 的 restore/save | v4.3.0 | `0057852bfaa89a56745cba8c7296529d2fc39830` |
| actions/upload-artifact | v4.6.2 | `ea165f8d65b6e75b540449e92b4886f43607fa02` |

依据：[checkout](https://github.com/actions/checkout/tree/34e114876b0b11c390a56381ad16ebd13914f8d5)、[setup-java](https://github.com/actions/setup-java/tree/c1e323688fd81a25caa38c78aa6df2d33d3e20d9)、[cache](https://github.com/actions/cache/tree/0057852bfaa89a56745cba8c7296529d2fc39830)、[upload-artifact](https://github.com/actions/upload-artifact/tree/ea165f8d65b6e75b540449e92b4886f43607fa02)。另核对 cache restore/save 的 `action.yml` 输入输出，包括 `cache-primary-key`。

SDK 依据：[Ubuntu 24.04 镜像说明](https://github.com/actions/runner-images/blob/main/images/ubuntu/Ubuntu2404-Readme.md)和[官方 Android 安装脚本](https://github.com/actions/runner-images/blob/main/images/ubuntu/scripts/build/install-android-sdk.sh)：runner 提供 `ANDROID_HOME`、`cmdline-tools/latest/bin/sdkmanager` 及已接受的 Android licenses。镜像会更新；本地阅读文档不代表已在 hosted runner 验证。

格式工具依据：[ktlint 1.7.1 官方发布](https://github.com/ktlint/ktlint/releases/tag/1.7.1)及[官方 release API](https://api.github.com/repos/ktlint/ktlint/releases/tags/1.7.1)。`ktlint` asset 的 digest 为 `sha256:b596a05f09364e1997b644152683af12fef58944f3acfabdf78071f5a490eb98`，实际下载后 `sha256sum` 一致。

## 实际检查

环境：Linux/amd64，复用 `/home/lijx/.local/bin/mise` 的现有安装，JDK 实际输出 Temurin `21.0.12.1+1-LTS`；SDK `/home/lijx/.local/share/mise/installs/android-sdk/23.0` 确有 API 36 和 Build Tools 36.0.0。wrapper 使用现有 Gradle 9.8.2；未安装替代 Gradle、重建或升级 SDK。外部工具下载只用于本地验证，不进入仓库。

| 检查 | 真实结果 |
| --- | --- |
| `/tmp/relaynest-ci-tools/actionlint .github/workflows/android-check.yml`，actionlint v1.7.7（Go 1.26.0 在临时目录安装） | PASS，覆盖 YAML/workflow 表达式/action 输入；没有安装 shellcheck，不声称执行 shellcheck |
| 提取 workflow 两段多行 `run`，逐段 `/bin/bash -n` | PASS |
| Android 目录运行 ktlint 1.7.1 executable，参数 `'*.gradle.kts' 'app/**/*.kt' 'app/**/*.gradle.kts'` | PASS，另用 `java -jar` 同工具同参数也通过 |
| 初次清空继承的四项签名环境，指定上述 `JAVA_HOME`/`ANDROID_HOME`，运行 `./gradlew :app:lintDebug :app:assembleDebug --no-daemon --no-configuration-cache --console=plain --stacktrace` | 未完成：约三分钟仍等待旧 Kotlin daemon；`jcmd <本次Gradle进程> Thread.print` 确认停在 `ClientUtils.walkDaemons` → loopback `Socket.connect`。仅终止本次 wrapper/Gradle 两个 PID，退出 143；未删共享 daemon 文件或终止其他构建，不把这次记通过 |
| 相同环境，以 workflow 最终命令 `./gradlew :app:lintDebug :app:assembleDebug -Pkotlin.compiler.execution.strategy=in-process --no-daemon --no-configuration-cache --console=plain --stacktrace` 重跑 | PASS，退出 0，`BUILD SUCCESSFUL in 22s`，46 tasks（16 executed / 30 up-to-date）。产生 debug APK 和 HTML/SARIF/TXT/XML Lint 报告，均留在忽略目录，未提交或上传 |
| `git diff --exit-code -- android/app/gradle.lockfile android/settings-gradle.lockfile android/gradle/libs.versions.toml android/gradle/wrapper/gradle-wrapper.properties` | PASS，无锁或工具版本变化 |
| `git diff --check` | PASS |
| GitHub hosted runner、远端 PR/fork、缓存命中/保存、artifact 上传 | NOT_RUN；无远端运行授权，本地检查不冒充远端 CI 通过 |
| 真机、Google/OPPO、正式签名/发布 | NOT_RUN，不属于本卡执行范围，真机留最终正式验收 |

Lint XML 实际为 0 errors / 7 warnings：`OldTargetApi`、三项 `GradleDependency`、`NewerVersionAvailable`、`DataExtractionRules`、`MissingApplicationIcon`。未弱化检查或加 baseline/抑制；本卡不升级锁定版本或改应用源码。构建还报告 Gradle 10 兼容性弃用提示；首次构建提示 `libandroidx.graphics.path.so` 无法 strip、原样打包，均未造成 debug 构建失败。通过只覆盖当前固定工具链。

## 验收与交接

不依赖签名秘密、错误可见、缓存 key 绑定锁版本三项均有实现与本地静态证据。远端行为仅完成配置审查，运行验收仍为 NOT_RUN；不得据本报告声称 GitHub CI 实跑通过。待独立审计和精确集成，本任务不自行记 done。

提交仅含 workflow 与本报告。后续审计需核对精确实现 SHA、路径/权限、固定 action SHA、失败传播、PR/fork 缓存边界及本地检查真实性；后续在获授权的首次远端运行确认 hosted runner SDK、缓存和 Lint 报告上传。未推送远端，未触发部署或发布。
