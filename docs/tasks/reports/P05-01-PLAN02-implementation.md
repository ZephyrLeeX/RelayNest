# P05-01-PLAN02 消息事务有限拆卡报告

## 交付与边界

- 角色 planning；结果 implemented（仅规划交付，等待独立审计/集成，不表示业务验收完成）。
- 基线 `f72a11c147269b4c81f0e319998292a3228a83f9`；树 `/home/lijx/Projects/RelayNest-recovery-20261024/message-store-plan`；分支 `codex/message-store-plan-20261025`。开始时工作树 clean。
- 仅追加 [父卡](../cards/P05-01.md)，新增 [S02](../cards/P05-01-S02.md)、[S03](../cards/P05-01-S03.md) 和本报告；原父卡所有文字、前置及验收完整保留。未改源码、SQL、API 或 registry；无 push、额外 agent 或业务实现。
- 最终 SHA 提交后交接，报告不预写自身 SHA。本次授权是 P3～P15 范围内有限拆分，未改变需求/架构/权限。

## 定点证据与有限上下文

已读 AGENTS/CONTRIBUTING/WORKER_PROTOCOL、父卡、PLAN01 与 common changes S01 实施/集成报告；技术设计仅 §1/2/4/5.1/5.2/9.1、架构 §1、阶段计划 P5、OpenAPI 局部 Message/MessageDeliveryEvent。P05-02/03 只核对职责避免侵入。

| 定点事实 | 拆卡结论 |
| --- | --- |
| 当前 SQL 为 0001～0004；无 messages/deliveries/push_jobs 表及 messages 包；0004_bots 已有 bot_seq | S02 只新增模型/迁移，不重做 Bot 或全局变化基础。 |
| storage.Write 统一事务、错误回滚；迁移校验连续编号/name/checksum；storage_test.go 固定夹具版本 | S03 复用一次 Write；S02 仅增补单测试文件夹具/迁移回归权限，不开放 storage 生产代码。 |
| bots/authorization.go 实际 Authenticate 需要 botID/kind/secret，AuthorizeTx 复核 identity/代次/能力 | 不使用不存在的 literal BotAuthenticate；S03 明列业务事务复核与排队期间撤销测试。 |
| changes.Append 已独审集成，Metadata/Sequence/错误及事务所有权有精确报告 | S03 直接复用，不复制计数/Append，不用 Bot 元数据版本替代内容序号。 |
| 现有 sessions 有 Android/device/撤销/过期/代次；devices 包仅 HTTP，未有推送注册存储 | 最小 push pending 用真实 session 绑定及显式有效目标验证；不得凭空假定完整设备注册/偏好系统已实现。 |
| auth 已有 SessionStore.WithAuthorizedWrite；P05-02 承接正式管理员/Bot 创建与业务幂等 | S03 留最小受信同事务 primitive，无绕过 G1 的公开管理员写入路径；HTTP/G1、完整输入和生产目标枚举继续由后续验收。 |

规划主要实现上下文限制为 Bot 授权/模型、storage.Write/迁移与测试的定点片段，加已集成 changes 报告；未整仓读取。每卡建议 3～5 个主要局部文件，输入≤12k、总体≤窗口60%、至少25%交付，接近上限 checkpoint；这两卡之后应直接派发可满足依赖的实施，不无限 PLAN 套 PLAN。

## 实例、编号与调度建议

- 总调度实际 dirty registry 为权威，本树旧快照不决定状态：P04-07（含存储 lifecycle S01/S02，非 HTTP）、P03-08、P05-01-S01 已独审集成 done；本报告不改台账、不替 root 宣告新任务 done。
- 正式实例存储任务是 **P08-04-D01-S01**，不是 ALLOC01。S02 强依赖其最终 A/I done；`0005_instance_identity.sql` 已由实例集成角色独占分配。规划基线上的“0005 未占”是历史事实，不能覆盖最新分配。S02 不预留 0006，等待 S01 真正入 main 后，由集成角色串行核对下个空号。
- 精确读取 `7e18fd5d97e74203e8f9cc5f5598e9cc4d2af18b:docs/tasks/reports/P08-04-D01-C01-I01.md`：C01 正式契约已是 40 operations / 118 schemas；当前规划基线正是其精确组合 f72a11c。历史 39/115 门槛不代替新契约；本轮没有重跑 API 校验，引用的是既有证据。
- 新卡建议 `kind=implementation,parent_id=P05-01`。S02 dependencies：PLAN02、P03-08、P05-01-S01、P04-07、P08-04-D01-S01；S03 dependencies：PLAN02、S02、P03-08、P05-01-S01、P04-07。依赖皆要求最终独审/集成 done，无子卡反依赖父卡。PLAN02 自身审计/集成完成后再由 root 登记/释放；S02 等实例存储收口，S03 等 S02。
- A/I 继续使用 subject_id 与精确 SHA，触发 review / review+最终审计 PASS；不把 subject done 作强依赖。父卡收口核对三子卡与组合，不丢原验收。

## 原验收与后续门槛

S02 负责 Schema/约束、旧库升级及实例初始化不被后续迁移重新触发。S03 对失败回滚（包括提交失败）、独立 Store 并发及重开单调、消息/计数/delivery/push/changes 同事务逐项给出可复现验证，使用真实目标写入 pending，不能拿空目标或模型存在替代验收。delivery/push 最小写入仍在 messages 包，不越权相应模块；正式 P06 状态机和 P07 发送保持原职责。

幂等、完整引用/附件/编辑、HTTP 成功回执、Android G1 与实例认证接线、生产注册目标枚举、provider 和手机/NAS 组合验证均未完成；由相应既有任务继续验收。S03 的受信底层 primitive 不宣称可直接承接不受信 HTTP。若实际必需接口仍不够，用具体证据报告边界缺口，不能擅自扩大授权。

## 检查与限制

- 文档检查实际执行：父卡原文前缀逐字核对、四文档相对 Markdown 链接存在、依赖无父子环/无 ALLOC01 依赖、写入路径仅四份任务文档、`git diff --check`；提交前再次核对 stage。
- 业务构建/Go/SQLite/API 测试 not_run：本轮仅规划文档，无源码/SQL/API 变更。卡内所列检查是未来实施要求，不是本轮已通过结果。
- 环境 PATH 无 python/python3；两次文档生成调用未执行，随后用已安装 Python 3.13.15 的绝对路径成功写入；检查同用该运行时。无隐藏失败或业务验证结论。
- 遗留：等待本规划独审集成和调度登记；S02 的真实迁移编号/派发基线须待实例存储最终集成后确定，当前不抢号。其余无需要用户再决策的范围内事项。
