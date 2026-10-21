# P03-03 集成报告（I01）

日期：2026-10-22；角色 integrator。结论 PASS，原主体/F01/A02 精确组合无冲突、无代码漂移。

目标 main，集成前 `c95f49e88c1daaa65c03d3bf28e95d0c4fa4c877`。

| 对象 | 本次原 SHA | main 目标 SHA |
| --- | --- | --- |
| 原 Android 实现映射 | `6351d6cb85585897a3f02742cb0c24c69c7da99e` | `9c3f488c0434606758aaa8e4e0a568ee5d901ef5` |
| 原 BLOCKED 审计映射 | `a092d476cfc483f6de31e663b044dbdfc8d09dd0` | `6d7c717ca480bb50f6c38f826a9732d528769e3c` |
| F01 最终修复 | `e002339fb085a6f5f8ac349c6b077d5751b3708f` | `a64a6296a86d33aece2773c3b246138cd4c6fd68` |
| A02 独立 PASS | `8239b4d609703f917ba81c71d07f0b5eff9b80e1` | `cd361071ab655476859e3f2dba9dcdc1671a60a0` |

仅依序 cherry-pick 四提交，没有重复原始 8fe8/dec758 提交。本报告提交前 HEAD 为 `cd361071ab655476859e3f2dba9dcdc1671a60a0`；最终报告 SHA 提交后交接。

## 实际组合验证

- 阅读 F01 和 A02 报告，确认被审最终组合为 e002339、A02 PASS 关闭 F01、E01 仅延期。Python 对四个提交各路径原/目标 blob 逐字节核对全部一致，最终 android 整树与 A02 受审 e002339 的 diff 为空。
- 在主目录 android 内，设 JAVA_HOME 为已有 Temurin 21、ANDROID_HOME 为已有 android-sdk/23.0，清除四项继承签名配置后执行 `./gradlew :app:lintDebug :app:assembleDebug --offline --no-configuration-cache --console=plain`：PASS，3 秒、46 tasks executed；Lint 0 errors / 4 warnings，无降级规则。既有 Gradle 弃用及原生库 strip 提示保留。
- 同环境 `bash tools/check-signing-config.sh`：15 用例全部 PASS，包括实际不可读引用两例，无跳过。只用脚本自建临时目录/空文件/非秘密标记；没有读取已有密钥、生成密钥或执行成功 release 签名。
- `bash -n android/tools/check-signing-config.sh`：PASS。Debug APK SHA-256 为 `b62fb153ee80acbdda444c8e45816b3a54d391fafeba5ca0123350e0656dabca`，与独立 A02 相同。
- `git -c core.whitespace=cr-at-eol diff --check <集成前> HEAD`：PASS；另排除已有 wrapper CRLF 文件的默认 diff --check 也通过。原 wrapper 字节未改。
- registry、`.directory`、ORCHESTRATOR-CHECKPOINT 哈希与本卡开始一致，除本任务路径外 index entries 完全不变，未 stage 用户文件或构建产物。

## 限制及移交

F01 的配置消费者与本地诊断通过，初始化可按 A02 结论收口；历史 A01 BLOCKED 原文保持。真实 keystore 解码、口令/别名有效性、成功 release 签名、release 依赖锁定、CI Secret 注入均 NOT_RUN。

E01 真机安装、冷启动、深浅色及系统栏 inset 仍 NOT_RUN。按用户决定留最终验收，移交 P09/P11/P12；届时需维护者授权设备连接，并先决定同包名旧 PoC 保留/覆盖，再记录最终 APK 与设备证据独立复核。此次未探测设备、安装或覆盖应用，不以 Debug 构建/同哈希冒充真机通过。

未修改共同契约、台账或其他工作树，未推送、发布或部署。正式任务状态由总调度登记。
