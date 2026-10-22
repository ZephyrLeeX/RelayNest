# P03-11 集成 I01

日期：2026-10-22；角色：integrator；结论：PASS。目标分支 `main`，工作树 `/home/lijx/Projects/RelayNest`。本次精确集成 Android debug CI 与独立审计报告，未修改 workflow 或应用代码。

## 精确提交与范围

| 项目 | 原 SHA | main SHA |
| --- | --- | --- |
| 原实现基线 | `491fc873657006f3f037b57175b35783a2ea388f` | — |
| 集成前基线 | — | `6f9cba2191378dacc93a55075c29386be777df3e` |
| CI 实现 | `8469c2f3caa5ff403ca61bce3116babfe633bf90` | `b774bcaeb9f2b918fc780a153c05029203796e1d` |
| 独立审计 PASS | `31a451964020b7688b65531a27ebcce0b8800c31` | `381aaff707de7390765a760bf005ef6e8098fcbb` |

来源 `/tmp/relaynest-run3/android-ci` 开始时干净。两次 cherry-pick 无冲突。workflow 与原实现 blob 同为 `3a09a13b54e718387e4f02dbcaf5d3563aeafea4`，实施/审计报告逐文件 diff 为空；除这三文件外与集成前基线内容一致。Android 整树、依赖锁、版本目录、wrapper 均未修改，没有代码漂移，无需重新审计。独立证据：[P03-11-audit-01.md](P03-11-audit-01.md)。本报告独立提交，自身 SHA 提交后交接。

## 当前 main 实际检查

环境：Linux amd64；已有 Temurin JDK `21.0.12.1+1-LTS`，SDK `/home/lijx/.local/share/mise/installs/android-sdk/23.0`，实查 API 36 与 Build Tools 36.0.0 已存在；使用现有 wrapper Gradle 9.8.2（发行包 SHA-256 固定）、AGP 9.4.1、Kotlin 2.2.10、minSdk 29。没有安装/升级 SDK 或 Gradle。四项 `RELAYNEST_ANDROID_KEYSTORE_PATH/KEYSTORE_PASSWORD/KEY_ALIAS/KEY_PASSWORD` 均显式置空，只构建 debug，不启用 release 属性。

| 命令/检查 | 真实结果 |
| --- | --- |
| `/tmp/relaynest-ci-tools/actionlint .github/workflows/android-check.yml` | PASS，退出0；沿用 actionlint 1.7.7，没有执行 shellcheck |
| Python 读取 workflow，提取全部 run 后各用 `/bin/bash -n` | PASS，4/4；使用已有哈希锁 Python 环境，未改源码 |
| 现有 ktlint 发布文件 SHA-256 核对及 Android 目录执行 `java -jar /tmp/relaynest-audit-ktlint-1.7.1 '*.gradle.kts' 'app/**/*.kt' 'app/**/*.gradle.kts'` | PASS，退出0；文件摘要 `b596a05f09364e1997b644152683af12fef58944f3acfabdf78071f5a490eb98` 与固定工作流一致 |
| Android 目录执行 workflow 对应命令 `./gradlew :app:lintDebug :app:assembleDebug -Pkotlin.compiler.execution.strategy=in-process --no-daemon --no-configuration-cache --console=plain --stacktrace` | PASS，退出0，`BUILD SUCCESSFUL in 17s`；46 tasks，5 executed / 41 up-to-date；本地组合复验，未声称无缓存全量编译 |
| 解析本次 `android/app/build/reports/lint-results-debug.xml` | PASS，0 Error/Fatal、7 Warning：OldTargetApi、3项GradleDependency、NewerVersionAvailable、DataExtractionRules、MissingApplicationIcon；未抑制或弱化检查 |
| 锁文件/版本目录/wrapper `git diff --exit-code`、精确范围/blob检查、`git diff --check` | PASS；无版本或锁内容改写，未提交 APK/报告/缓存等生成物 |

Gradle 仍提示部分特性将不兼容 Gradle 10，本次固定工具链构建通过。完整本地 Gradle 日志保留在 `/tmp/relaynest-run3/p0311-integration-gradle.log`，不入库。首次额外 Bash 提取从 Android 子目录读取仓库根相对路径，因路径错误失败；随后在仓库根执行四段提取和语法检查通过，未改实现或把失败记为通过。

## 验收范围与未测项

工作流配置不引用秘密、固定工具与 Action SHA，只恢复指定 Gradle 依赖/wrapper 缓存，key绑定锁和工具版本；没有宽泛恢复键，仅成功push保存；普通失败传播保留，只上传限定 Lint 报告。上述配置与固定审计版本相同。本次亲自运行 debug/Lint/格式检查并确认锁未改写，证明当前 main 本地组合可运行；不证明远端缓存/作业行为。

NOT_RUN：GitHub hosted runner SDK安装、真实PR/fork、失败作业状态、缓存命中/保存和artifact上传；设备、Google/OPPO、正式签名、发布与部署也未运行。不得据本地 PASS 宣称远端 CI 或正式验收通过。

registry 原有用户及授权调度元数据变化保留，不stage、不覆盖；调度期间补真实agent_id同样不提交。`.directory` 与已提交 ORCHESTRATOR-CHECKPOINT.md 的 SHA-256 均保持不变。本次仅提交指定实现/审计与本报告，不修改卡或其他模块，不派额外agent，不推送/发布/部署。后续用 create_worktree 在本报告提交后的最终 main 精确 ref 准备 P03-10 web-ci 持久树，实际路径/分支在交接给出，仅准备不实现。
