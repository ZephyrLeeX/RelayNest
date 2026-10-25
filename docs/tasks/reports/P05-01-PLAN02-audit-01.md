# P05-01-PLAN02-A01 独立规划审计

```yaml
task_id: P05-01-PLAN02-A01
subject_id: P05-01-PLAN02
role: auditor
reviewed_base_sha: f72a11c147269b4c81f0e319998292a3228a83f9
reviewed_implementation_sha: 8bcf4c6188717f21a976a81c4f6696d474b7a7c9
verdict: PASS
findings: []
report: docs/tasks/reports/P05-01-PLAN02-audit-01.md
```

## 审计范围与结论

审计者与原规划者不同。工作树 `/home/lijx/Projects/RelayNest-recovery-20261024/message-store-plan`，分支 `codex/message-store-plan-20261025`，开始时 clean、HEAD 与被审 SHA 一致。实际差异仅父卡追加、S02/S03 两张新卡及 PLAN02 实施报告；代码、SQL、API、registry 均未改变。本审计只新增本报告，不修改被审卡或台账，不将父任务置 done。

未发现需要修复的编号问题。两卡保留父卡全部原验收，划分有限且有可执行的检查边界：

- S02 仅新增消息/最小队列 Schema 与 messages 模型；storage_test.go 明确只允许迁移夹具和回归适配，无 storage 生产代码授权。实例正式任务为 P08-04-D01-S01，必须最终 A/I done；0005 已独占，0006 未预占，最终集成后串行核号。不得借消息升级初始化、修复或旋转实例身份。
- S03 明确在同一 Store.Write 内复核授权、分配 bot_seq、写 revision=1 消息、Append、回填真实 created_change_seq 和最小 pending；事务 primitive 可供 P05-02 复用。故障要求覆盖 SQL/Append、提交失败、重开、独立 Store 并发和真实 session 目标，不能拿空 pending 替代原子性验收。
- Bot 签名与现有 authorization.go 一致：Authenticate(ctx, botID, kind, secret)、AuthorizeTx(ctx, tx, identity, botID, bots.Send)。身份不可自行构造、事务内复核代次/启用/能力、Webhook 不扩权；管理员只提供受信同事务基础，后续正式入口必须复用 WithAuthorizedWrite 和 G1 校验。
- 最小 delivery 的不可变 MessageDeliveryEvent 快照符合现有契约；push pending/通知只保存目标元数据，不含正文、附件 URL 或凭据。二者不能混为同一种载荷限制；changes 继续复用公共 Metadata/Append，不建立私有日志实现。
- 父卡保留 transport-only 与 RelayShelf 边界。P05-02 正式创建/业务幂等、P05-03 引用/历史/共享已读、P06 调度、P07 推送接线、P09 真实 provider、G1/HTTP/管理员接线均后置，不因本拆分宣告通过；S01 已有基础只复用，不重复实施或重跑其整套阶段验收。

采用调度提供的权威状态：P04-07 存储 lifecycle（非公开 HTTP）、P03-08、P05-01-S01 已独审集成 done；本树旧 registry 不作为阻塞理由。S03 对实例任务通过 S02 传递依赖，无子卡强依赖未完成父卡；PLAN02 必须先完成自身 A/I 并由调度登记。

## 实际检查与证据

- `git diff --name-status <base> <implementation>`：恰好四份任务文档；逐项检查完整差异，未见越界修改。
- Perl 字节前缀检查：父卡基线全文为新卡文件前缀，PASS；四文档相对 Markdown 链接逐个存在，PASS。
- `git diff --check <base> <implementation>`：退出 0；审计报告提交前另执行工作区 diff 检查。
- 定点核对 PRD §1 及消息/投递边界、TECHNICAL_DESIGN §1/2.1/5.1/9.1、OpenAPI MessageDeliveryEvent、实际 Bot 授权、Store.Write、changes.Append、SessionStore.WithAuthorizedWrite 和存储迁移夹具；与卡中职责及事务接口一致。
- 读取 P05-01-S01-I01 公共接口交接及 PLAN01 摘要；读取精确 `7e18fd5d97e74203e8f9cc5f5598e9cc4d2af18b:docs/tasks/reports/P08-04-D01-C01-I01.md`，其中正式合同证据为 40 operations / 118 schemas / 2455 cases，集成组合 f72a11c。未将历史 39/115 计数或合同通过解释为 runtime 完成。
- 每卡输入≤12k、总体≤窗口60%、留≥25%及 checkpoint 约束明确；实现主要文件建议有限。S03 测试矩阵较密集，实施者须按卡及时 checkpoint，不得因此省略父卡原验收。

## 限制与交接

这是规划审计，无源码/迁移变更，Go、race、SQLite 业务和 API 校验均 not_run；卡内命令属于未来实施义务，未声称本轮通过。未审核尚未最终集成的实例存储实现或未来消息实现；真实编号、精确依赖 SHA 和组合验证仍须派发/集成角色确认。初次探查用了不存在的 `api/openapi.yaml` 与 S01 长格式报告名，随后定位实际 `server/api/openapi.yaml`、`P05-01-S01-I01.md` 完成核对；无据此形成通过结论。

无 push、新聊天、额外 agent 或业务全测；报告独立本地提交，真实报告 SHA 在提交后交接。
