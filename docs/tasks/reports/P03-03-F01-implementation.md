# P03-03-F01 正式签名安全注入与脱敏诊断修复实施报告

## 范围与基线

- 日期：2026-10-22；角色：fixer；唯一目标发现：`P03-03-A01-F01`。
- 基线：`a092d476cfc483f6de31e663b044dbdfc8d09dd0`；工作树：`/tmp/relaynest-run2/android-fix`；分支：`codex/android-fix-run2`。开始时干净。
- 基线已组合原实现 `8fe8d36bf7a85edf8f95f20e1f02f9b72930106a`（映射 `6351d6cb85585897a3f02742cb0c24c69c7da99e`）和原审计 `dec758e55d7611e330f0981691ce985c9b9f80d0`（映射至本基线），修复规划已由调度独立审计并集成。
- 结果：最小修复及本地检查完成，交独立复审；本报告不自行关闭发现、不改写历史 BLOCKED、不修改卡片或台账。最终提交 SHA 在提交后交接，不预写自身 SHA。

## 变更

仅修改 `android/app/build.gradle.kts`、`android/README.md`，新增 `android/tools/check-signing-config.sh` 和本报告。既有忽略规则足够，未修改 `.gitignore`。

`-PrelaynestReleaseSigning=true` 为显式消费者入口：在配置阶段检查四个既定环境字段非空白及引用存在、普通文件、可读，再接入 release 的 `externalRelease` signingConfig。缺项或无效引用直接报固定诊断，无 debug 签名回退。省略该开关时不读取这些环境变量，release 保持 unsigned；debug 无正式配置继续构建。

`:app:checkReleaseSigningConfig` 只检查完整性与文件元数据，不读取 keystore 内容。缺项仅列变量名，无效引用不打印路径；诊断成功也明确不验证口令、别名或签名。为避免含口令的签名模型进入配置缓存，显式签名请求拒绝配置缓存，README 命令使用 `--no-configuration-cache`。未新增依赖、升级工具链、修改业务或添加 CI 发布工作流。

README 同步实际命令、无配置行为、仓库外私有保管和独立安全备份要求，以及受保护 CI 私有临时目录加载、退出清理、PR/fork 隔离和日志约束。正式保管方案仍由维护者确认，本卡未执行该方案。

## 环境及实际检查

WSL2 Linux amd64；沿用 Temurin `21.0.12.1+1-LTS`、Gradle `9.8.2`、AGP `9.4.1`、应用 Kotlin/Compose 编译插件 `2.2.10`。Gradle 内嵌 Kotlin `2.4.10` 不是应用编译器版本。没有更新 wrapper、依赖或锁文件。

以下构建命令均在工作树 `android/` 执行，Gradle/脚本前缀为：

```sh
JAVA_HOME=/home/lijx/.local/share/mise/installs/java/temurin-21
ANDROID_HOME=/home/lijx/.local/share/mise/installs/android-sdk/23.0
```

| 命令/检查 | 实际结果 |
| --- | --- |
| `./gradlew --version --offline --console=plain`、JDK 绝对路径 `java -version` | PASS，退出 0，版本如上 |
| 清除四项 `RELAYNEST_ANDROID_*` 签名环境后 `./gradlew :app:lintDebug :app:assembleDebug --offline --console=plain --no-configuration-cache` | PASS，退出 0；16 秒，46 tasks executed |
| `/home/lijx/.local/share/mise/installs/java/temurin-21/bin/java -jar /tmp/relaynest-ktlint-1.7.1.jar '*.gradle.kts' 'app/**/*.kt' 'app/**/*.gradle.kts'` | PASS，退出 0、无输出；使用已有外部 CLI，无新增 app 依赖 |
| `bash tools/check-signing-config.sh` | PASS，最终完整执行 15 个用例，下表列明；只输出用例名和结果 |
| `bash -n tools/check-signing-config.sh` | PASS，退出 0 |
| `git diff --check`、提交前逐项审阅差异和路径 | PASS；无秘密、密钥、构建产物、PoC、台账或其它范围变更 |

首轮 DSL 编译发现 `java` 扩展遮蔽及弃用 API 检查错误，已改为 `File` 导入和 Gradle `BuildFeatures` API；首轮格式检查发现换行问题，已用固定 ktlint 格式化并复验通过。上表只把修正后实际通过的构建与检查列 PASS。Lint 为 **0 errors / 4 warnings**：OldTargetApi、compileSdk 新版本提示、DataExtractionRules、MissingApplicationIcon；未关闭警告。仍有既有 Gradle 弃用及 debug 原生库 strip 工具提示，不据此升级工具链。

## F01 负面及脱敏证据

脚本用 `env -u` 清除继承的四项配置，仅创建自己的临时目录、空文件、不可读文件和非秘密标记。未读取现有正式密钥、口令或用户秘密文件；脚本结束清理临时目录。Gradle 原始日志只在临时目录捕获，检查后删除，不提交原始输入或日志。

| 实际用例 | 显式请求 `:app:assembleRelease -PrelaynestReleaseSigning=true` | 诊断 `:app:checkReleaseSigningConfig` |
| --- | --- | --- |
| 无配置 | PASS：非零，列全四个缺失字段 | PASS：非零，列全四个缺失字段 |
| 仅部分字段 | PASS：非零，缺项诊断 | PASS：非零，缺项诊断 |
| 字段为空白 | PASS：非零，指明缺少字段名 | PASS：非零，指明缺少字段名 |
| 引用不存在 | PASS：非零，固定无效引用诊断 | PASS：非零，固定无效引用诊断 |
| 引用为目录 | PASS：非零，固定无效引用诊断 | PASS：非零，固定无效引用诊断 |
| 引用不可读（自建文件 chmod 000） | PASS：当前非 root 用户实际不可读，非零 | PASS：非零 |
| 自建可读空普通文件 | 未执行实际签名 | PASS：退出 0，明确未验证口令、别名或签名 |

以上 12 个负例均检查捕获日志不含测试标记、完整临时路径；口令及别名都带该标记，因此同时覆盖值、口令和别名泄漏检查。显式请求负例还检查没有任何 `> Task` 执行行，确认在配置阶段停止。

另外 2 个用例：使用空文件并显式开启消费者，仅执行 `:app:checkReleaseSigningConfig -PrelaynestReleaseSigning=true`，退出 0，证明 signingConfig 可配置但没有执行签名；无配置正式签名请求带 `--configuration-cache`，明确要求 `--no-configuration-cache` 并非零退出。共 15 个用例全部 PASS。**空文件通过仅为元数据诊断成功，不是正式签名成功。**

## 验收、限制与交接

F01 的配置消费者、显式入口、失败分支、无配置 debug、DSL 格式、脱敏回归和 README 一致性已有本地证据，交不同 agent 对原主体加修复组合精确 SHA 独立复审。未执行真实 keystore 解码、口令/别名有效性验证、成功 release 签名、release 依赖锁定、CI Secret 注入或分发，均为 **NOT_RUN**。完整性通过后的 AGP 真签名工具失败日志不在本地假数据回归的验证范围；后续获授权环境须私有保管并脱敏分享。

`P03-03-A01-E01` 仍为 **NOT_RUN**。按用户“真机运行留到最后验收的时候”，本卡未探测/连接设备、安装、卸载或运行应用；移交 P09 通知/点击与 P11/P12 正式验收相关范围。届时由维护者提供当前 ADB 路径及已授权连接，安装前确认同包名旧 PoC 保留/覆盖安排，再验证正式最终 APK 安装、冷启动、深浅色和系统栏 inset，保留设备/APK/命令/观察证据并独立复核。初始化收口不表示 E01 已通过，调度须保留最终验收责任。

未执行推送、发布、部署、平台注册或密钥保管方案。没有需要本卡新增的产品/架构决定。
