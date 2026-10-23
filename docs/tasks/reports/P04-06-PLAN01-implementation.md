# P04-06-PLAN01 Bot 存储与管理 HTTP 拆分规划

## 交付与基线

- 角色：规划；结果：implemented（仅规划已交付，不代表实现或验收通过）。
- 基线：`9ca05a0dfe7973a2fb8d077f3e9646085c2451f6`。
- 工作树：`/home/lijx/.codex/worktrees/bot-plan/RelayNest`；分支：`codex/bot-plan`。
- 只修改 P04-06 父卡、新 S01/S02 和本报告；无源码/API/设计/registry/其他卡或旧报告写入，无推送、无额外 agent。
- 规则来源：AGENTS、CONTRIBUTING、WORKER_PROTOCOL；PRD §3.1、架构 §1 C-01～05；技术设计 §3.2、§5.1/5.4、§6.1/6.2、§7.1/7.2；API 按下表键定位。文档/源码静态核查不冒充运行验证。

## 已核事实与拆分依据

| 定点来源 | 事实 | 规划结论 |
| --- | --- | --- |
| 原 P04-06 | 前置 P04-02；范围仅 bots/migrations；创建/查询/编辑、独立凭据、send/receive | 原前置/工作/五项验收保留，不能整卡宣布 ready |
| T7.1/7.2、PRD §3.1 | 管理员会话和 Bot 凭据独立；停用覆盖授权；receive 不授创建/编辑/跨 Bot | S01 无 HTTP 可先行；S02 保留管理员前置 |
| T7.1、P04-03 父卡 | Web 写操作需同源/CSRF、外部 scheme/host/port/代理判断；helper 不含完整会话安全 | S02 强依赖完整 P04-03，不能以 P04-03-S01 代替 |
| API Bot/CreatedBotResponse | 创建三键为 bot/api_token/webhook_key；普通 Bot.credentials 仅 CredentialMetadata；Webhook 仅发送，不能互换 | S01 摘要/身份，S02 仅成功创建响应返回明文 |
| API SyncChange/SyncBotUpsert/SyncBotDelete | 实际类型是 bot.upsert、bot.deleted；分别关联 Bot 与 id/version/deleted_at | 禁止使用预检简称 bot.updated；删除不在本卡，后卡保留墓碑 |
| API Version/Sequence；T2.1 | Version 为 JSON integer≥1；Sequence 为 SQLite 非负 int64 规范十进制字符串 | 不套用全局 settings 专用上限/类型；seq/version/epoch 各守语义 |
| 0001_initial.sql:2～23 | system_state 生成 epoch；changes 已有全局 AUTOINCREMENT，无 Bot 外键以保存 tombstone | 不重建表、另起全局序列或加级联删除；Bot/版本/changes 同事务 |
| storage.go:108～127 | Store.Write 接受调用方短事务回调，错误回滚、成功提交 | 可复用，不改 storage 实现/锁/依赖 |
| server/internal 源码符号检查 | 基线未有公共 changelog append/类型序列化接口 | 独立共同前置必须先冻结实现，不能让 S01 私建旁路 |
| storage_test.go:154、302～374 | 固定真实版本 3/读取 0001～0003，人工 0004_upgrade；新增迁移会撞 | S01 最小夹具维护真实集合/动态下一编号，保留失败/锁/checksum 强度 |
| httpapi/router.go、cmd/relaynest/main.go | 基线只挂 health/static；serve 实际调用 httpapi.New(h) | S02 必须明确 router/cmd 最小接线路径，不能限于 bots/migration 又虚报 HTTP |

## 职责和写入归属

| 单元 | 责任与路径 | 排除/证据归属 |
| --- | --- | --- |
| S01 | bots 仓储/摘要/独立身份与事务校验；新串行 Bot 迁移；storage_test.go 最小夹具维护 | 无 HTTP/管理员认证；独立 send/receive/停用及事务故障证据 |
| S02 | bots 必要适配；httpapi 三管理路由 handler/router/测试；cmd main.go/main_test.go 必要组装 | 复用完整管理员安全；真实 HTTP 越权/CSRF/明文响应与父验收证据 |
| P05-01-S01（拟定前置 ID） | 全局变更日志事务追加基础；具体 scope/interface 由 P05-01-PLAN01 独立规划 | 当前四文档不实现、不冻结函数、不写公共模块；调度负责登记 |
| P04-03-S01 独立来源 helper | 调度告知在独立树仅 origin.go/测试，不涉及 SQL/auth/migrations | S01 无重叠；S02 等完整 P04-03，接入真实集成 SHA |

S02 路由严格是 GET/POST `/api/v1/admin/bots` 和 PATCH `/api/v1/admin/bots/{id}`；无 DELETE、重置、清空、消息/投递/附件、其他 P06 管理功能。无真实轮询时遵循 Bot.connection 的 unknown/null，不捏造状态。

## 共同事务缺口与最小先行建议

必要共同点包括：接受调用方已有事务的追加、既有 changes 列/事件类型与实体版本映射、全局 int64 change_seq、epoch 读取/持久性、公共 Sequence 序列化，以及同事务失败全部回滚。单纯 Store.Write 只给事务，不提供这些共同消费约定。

已向调度提交证据；调度确定交由独立 `P05-01-PLAN01` 规划拟定的 `P05-01-S01`，从原仓储职责抽取与管理员会话/Bot 无关的最小基础，原 P05 父前置/验收保留。此处仅引用拟定 ID，不创建不存在的链接/卡或占位接口。共同规划须独审集成，实际基础实现再独审集成；S01 用其真实实现与精确 SHA，方可 ready。若规划证明 ID/边界需调整，由调度同步当前卡，不静默扩大 S01。

P05/P06 后续消费者以该真实实现审计集成 SHA 交接，不依据本规划伪签名；Bot 业务写入与版本/对应 changes 必须同一短事务，不能等同步功能阶段补写。T6 的当前投影/墓碑读取由后续同步任务实现；本卡只保证写历史完整及删除不会 cascade 丢历史的约束。

## 并行、串行和 registry 建议

- P04-02/P04-03 会话 HTTP 与 Bot S01 职责独立；但 S01 需 P03-07、本规划、共同 P05-01-S01 已审计集成后才派发。
- 所有 migration 编号与公共 changelog 契约均串行；基线只有 0001～0003，0004 未分配，派发前按最新 main 分配。共同前置若无需迁移也不得预留编号。
- S02 等 S01、P04-02、完整 P04-03；父卡等 P04-02、S01、S02 收口。S01 不依赖父卡 done；归属关系不作为依赖。

建议由调度在本规划独审集成后写 registry，当前文档不写台账：

| ID | 建议 dependencies / gate |
| --- | --- |
| P04-06-PLAN01 | 本轮授权规划；待独立 A/I 精确 SHA |
| P04-06-S01 | P03-07、P04-06-PLAN01、P05-01-S01；共同规划/实际实现独审集成 SHA 与迁移串行分配门槛，缺一 blocked |
| P04-06-S02 | P04-06-S01、P04-02、P04-03、P04-06-PLAN01 |
| P04-06 | 保留 P04-02；增加 S01、S02；全部原验收最终组合验证 |
| 各单元 A01 | kind=audit、subject_id=被审单元；仅其 review 与精确实现 SHA 触发，非 subject done 强依赖 |
| 各单元 I01 | kind=integration、subject_id=被集成单元；固定实现 SHA+最终审计 PASS 及报告；非 subject done 强依赖 |

共同 P05-01-PLAN01/S01 的具体 DAG 由其规划交付；不得通过原 P05 父卡依赖回指未完成 P04-06 造成环，调度登记时必须复核。集成若改变业务代码需要重审。父卡 done 仅由调度在最终组合验收后登记。

## 原验收逐项保持与映射

以下原文字未改，父卡继续保留：

| 原验收 | S01 证据 | S02/父级证据 |
| --- | --- | --- |
| 跨 Bot/跨路由越权拒绝。 | 独立 Bot 身份、凭据种类/代次/能力与停用 | 真实管理三路由+会话安全拒绝；未实现未来业务路由列后卡约束，404 不冒充业务验收 |
| 明文仅一次。 | 独立摘要持久化与查询不返明文 | 创建三键、GET/PATCH/错误/日志无明文或摘要 |
| receive-only 不能发送。 | 真实独立 send 检查与提交前重新鉴权 | HTTP 管理拒绝与独立授权组合；未来消息 HTTP 不虚报 |
| 执行受影响检查，逐项报告命令/环境/结果；无条件运行时保留 `blocked`，不将必需验收记为通过。 | 相关 Go/迁移/故障检查 | HTTP 安全/响应组合及父级最终相关检查 |
| 提交仅本任务变更，交付基线SHA/实现SHA、diff摘要、证据路径和遗留问题；独立审计与集成完成后才可 `done`。 | 独立 SHA、共同依赖消费 SHA、A/I | 独立 SHA、会话/安全消费 SHA、父最终组合 A/I |

父级必需证据不足仍 blocked，不因规划或 S01 单测完成自动通过；产品 AC-02 的重置和 AC-01 删除等由原后续任务负责，未并入当前范围。

## 验证与遗留

- 本轮仅规划：执行文档链接存在性、原验收逐字保留、写入白名单、diff whitespace 和上述 DAG 静态核查；链接、五项原验收逐字保持及 git diff --check 均通过；四路径白名单与所提依赖边手工核对通过（共同前置实际 DAG 尚待另行规划，不宣称已验证其未知依赖）。不运行业务 Go 测试，因为无源码改动；不声明任何运行验收通过。
- 当前已知实现阻塞：共同 P05-01-S01 未实现审计集成；S02 等管理员和完整 Web 安全。迁移编号未分配，不虚构 SHA。
- 没有新增产品/权限/外部接口决定；共同基础职责由调度另行规划。若新接口/架构确需改变，先提交证据与方案由用户确认。
- 本报告不预写自身提交 SHA；提交后通过交接提供精确本地 SHA，独立审计/集成后才正式登记新 DAG。
