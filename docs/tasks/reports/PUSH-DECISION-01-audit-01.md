# PUSH-DECISION-01 独立文档审计（A01）

日期：2026-10-22；角色：独立审计；结论：**PASS**。本结论仅覆盖用户决定的文档落盘，不代表 OPPO 外部门槛、实现或真机验收通过。

```yaml
task_id: PUSH-DECISION-01-A01
subject_id: PUSH-DECISION-01
reviewed_base_sha: 428014852a00275c27362e9caa5e748e1e8338f1
reviewed_implementation_sha: 489adb8855c4af8f717e7da3fc9e21418fe3d34c
verdict: PASS
findings: []
report: docs/tasks/reports/PUSH-DECISION-01-audit-01.md
```

## 审计对象与依据

工作树 `/tmp/relaynest-run2/decisions`，分支 `codex/decisions-run2`；开始时 HEAD 为 `ee85f4346fdae61969d294e2cee4e38ee1d1e7a3`，工作区干净。该 HEAD 中后续 Android 修复规划属于另一任务，本次只审查上述基线至实现的精确 10 文件差异，不把后续规划纳入结论。

读取根 AGENTS.md、CONTRIBUTING、WORKER_PROTOCOL、PUSH-DECISION-01 实施报告、PRD 通知相关约束、T9.1～9.3，以及基线中的 PUSH-PLAN-01 集成报告。未提供独立同名任务卡，采用总调度派发的单卡范围、允许路径及可信用户决定作为审计输入，不据缺失卡片扩大职责。

审查文件：`docs/TECHNICAL_DESIGN.md`；七张任务卡 P02-05、P03-07、P09-02、P09-03、P09-04、P09-06、P09-07；`docs/tasks/reports/push-channel-adapter-plan.md`；`docs/tasks/reports/PUSH-DECISION-01-implementation.md`。

## 核查结果

- **决定一致**：T9.3、实施报告和原提案当前注记准确记录固定标题「RelayNest 有新消息」、固定正文「打开 RelayNest 同步查看」，继续禁止业务正文、URL、凭据。显式可选 OPPO、超时不跨 FCM 补响、切换仅未来新事件，以及外发残余、配置收敛前漏提醒和离线 DND/已读/退出撤回限制均有落盘，与派发传达的可信批准一致。
- **范围未扩大**：未自动激活 vendor，未授权自动择优或跨通道回退；Hub 职责仍限传输、存储和路由。FCM 继续按 T9.2 原验收；发送前策略过滤、模式 CAS、代次隔离、迟到结果和双 worker 验证要求保留。更新仅取消已经批准的用户待决，没有用批准替代技术实现或平台资格。
- **外部门槛真实**：资格、测试/正式权限、SDK/API 版本、签名、分类/额度/错误码和真机矩阵仍未验证；P02-05 保留 G-OPPO blocked/缺环境 not_run 要求。未发现声称厂商资格、展示或实测已经通过；未把 HTTP 受理或桩结果视为真机成功。
- **历史与后续职责清晰**：原提案旧注记标记为历史，并由当前注记覆盖过去待决语境；历史报告未被改写。基线已有 PUSH-PLAN-01 精确集成报告，支持 T9.3 的“已集成”表述。子卡、registry 和 P03-08 依赖范围建议均明确需要另行登记，不构成本卡实施授权。
- **变更边界**：精确差异仅 10 份文档，无 PRD、registry、源码、接口或依赖清单修改；人工检查未发现秘密值、业务正文或原始敏感日志。实施报告没有预写自身提交 SHA。

## 实际检查与限制

环境：工作树 zsh、Git、Perl（`/usr/bin/perl`）。

1. `git diff --stat`、`git diff --name-only` 及完整固定范围 diff：通过；10 文件、76 行新增、24 行删除，与实施交接范围一致。
2. `git diff --check 428014852a00275c27362e9caa5e748e1e8338f1 489adb8855c4af8f717e7da3fc9e21418fe3d34c`：通过。
3. Perl 从实现 SHA 读取全部 10 文件，枚举 Markdown 本地链接并用 `git cat-file -e` 检查实现树中的目标：78 个链接全部存在；同时断言恰为 10 文件、均在 docs 下且不含 PRD/registry，通过。未验证锚点或外链可达性。
4. 初次尝试 Python 链接检查时，`python3` 不存在，未运行成功；随后以 Perl 完成上述检查，不将失败尝试记为通过。
5. 开始时 `git status --short` 为空；本次仅新增本审计报告，提交前再次检查 diff/status。

纯文档审计未运行 Go/Android 构建、SDK 下载、平台登录/注册、密钥创建、真实推送或真机矩阵，也未重新查证现行外部接口。本报告只核查文档是否忠实保留相应未验证状态，不能作为 G-OPPO、G-FCM 或后续功能验收的替代证据。未进行集成、部署、发布或远端推送；总任务状态由调度处理。
