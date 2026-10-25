# P05-01-S03 消息与最小待办原子仓储实施

- 角色 developer；结果 implemented，待独立审计/集成，不标记父 P05-01 done。
- 基线 `4f4cfd0ea4af1b9c71919af5dd9cba0d365d4111`；工作树 `/home/lijx/Projects/RelayNest-recovery-20261024/message-store`，分支 `codex/message-store-20261025`。实现 SHA 提交后交接，不在报告中预写自身 SHA。
- 恢复时仅有本卡未跟踪的 repository.go、pending.go、repository_test.go；逐份研究并继续现有草稿，本轮重新实际执行全部必跑检查。没有以旧 worker 的未完成检查作为通过证据。
- 范围仅 messages 三个新增 Go 文件和本报告；无 SQL、storage/auth/bots/changes 生产代码、API、registry、delivery/push 或依赖修改，无推送/部署/额外 agent。

## 实现与接口

已核对根约定、贡献/worker 协议、父卡与 S03、PLAN02 实施/I、S02 实施/A/I、S01 实施/I、Bot 最终 A/I，以及 T §1、2.1/2.2、4、5.1、9.1 和冻结 MessageDeliveryEvent。使用 S02 已集成 `0006_messages.sql`，集成实现 `0bed7d5213dc4badd250e306d5a78a99c980a5b7`，其独审组合 `23b9e08e472a5e39f8fdcb17b998be6b57898cdf`、集成报告即当前基线。S01 公共基础集成实现 `6de2970ab1c18e97463b789cd39c2c68e96356d2`；Bot S02 最终组合 `e6de9e5e039a85e57ec8020ee03d7f24c6f59563`；PLAN02 集成组合 `ae65b4de7a97c0a64f7ae72f21b2f2a5295aa928`。

`messages.New(*storage.Store)` 返回 Repository。`CreateApplication(ctx, bots.Identity, TextInput) (Created,error)` 唯一包装 Store.Write，失败返回零 Created。身份必须由真实 bots.Authenticate 获得，同一写事务内以 bots.AuthorizeTx(..., bots.Send) 复核归属、启用、能力与凭据代次。

`CreateApplicationTx(ctx,*sql.Tx,bots.Identity,TextInput)` 与 `CreateAdministratorTx(ctx,*sql.Tx,TextInput)` 均返回 `(Created,error)`，供后续受信业务复用同一事务。前者仍复核 Bot 身份，后者固定 administrator sender 并检查启用/接收能力；后者不认证管理员，正式调用必须位于 auth.SessionStore.WithAuthorizedWrite 回调内并完成最终 G1，不能直接作为 HTTP 入口。nil tx 返回 ErrInvalid；所有错误必须向外传播并回滚，Created 仅为候选，外层提交成功前禁止发布。

创建路径完成 bot_seq 显式溢出与受影响行检查、revision=1 消息插入、真实 changes.Append(MessageUpsert) 及 created_change_seq 回填；管理员消息另写稳定随机 event_id 的完整 message.created 不可变 JSON 快照、初始 pending delivery 及 DeliveryUpsert。消息水位始终指向创建事件，Bot 元数据 version 不变。delivery 初始 state_version/attempt/retry_generation 为 SQL 默认 1/0/0。应用消息不产生自身应用投递。

TextInput 仅支持非空有效 UTF-8 文本（≤65536 字节）、plain/markdown；非空附件或非 nil 引用显式 ErrUnsupported，非法字段 ErrInvalid。空附件/按钮及 null 引用写入完整快照。PushTargets 最多100个，不重复 session；同事务查询真实 Android session 的 device 绑定、撤销、两种期限与管理员代次，否则 ErrPushTarget。push 仅存 session/device/实际全局水位与扫描字段，无正文、URL、凭据。随机读取错误原样包裹返回；Bot int64 耗尽 ErrSequenceExhausted，异常受影响行 ErrAffectedRows，其余数据库/Bot/changes 错误保留。

## 实际验证

2026-10-25，Linux amd64，`go version go1.27.3 linux/amd64`；Go 绝对路径 `/home/lijx/.local/share/mise/installs/go/1.27.3/bin/go`，所有命令设置 `GOTOOLCHAIN=local`，cwd 为本树 server。真实 SQLite 版本 `3.53.4`（fixture EngineVersion 实测）；使用临时真实文件库、多连接及重开，不用 mock/内存数据库。恢复时本树已有 ignored Web 构建产物，本轮未重新运行或冒称 Web build 通过。

| 实际命令 | 结果 |
| --- | --- |
| `go test -count=1 ./internal/messages ./internal/storage ./internal/changes ./internal/bots` | PASS，0.373s / 0.650s / 0.049s / 0.369s |
| `go test -race -count=1 ./internal/messages` | PASS，6.752s |
| `go vet ./internal/messages ./internal/storage ./internal/changes ./internal/bots` | PASS，退出0 |
| `go test -count=1 -run 'TestMessageSQLAndCommitFailuresRollback/commit\|TestMessageIndependentStoreConcurrentMonotonic\|TestMessageQueuedInvalidationAndContext' -v ./internal/messages` | PASS，0.134s；COMMIT 实测输出见下 |
| mise `gofmt -l` 三个新增 Go 文件 | PASS，无输出 |
| `git diff --check`、提交前显式 stage 后 `git diff --cached --check` | PASS；提交范围核对仅本卡四路径 |

验收证据均在 repository_test.go，可按下列测试名称重现：

- `TestMessageSQLAndCommitFailuresRollback`：计数后、消息后、Append 后、回填后、delivery Append/入队后、push 入队后真实 SQL trigger 故障，含唯一约束冲突与受影响行异常；各次对比他连接和重开后的消息/临时 NULL/队列/changes/Bot 计数/sqlite_sequence 全量计数状态，无孤立行。commit 子用例的回调已成功且有候选，随后延迟外键 COMMIT 失败，实际输出 `callback succeeded; COMMIT rejected deferred FK: constraint failed: FOREIGN KEY constraint failed (787)`。应用包装器在同样真实提交错误时返回零结果。
- `TestMessageRandomInputAndSequenceFailures`、`TestMessageSequenceUpperBoundsAndCallerRollback`：随机源在消息/delivery/event/push UUID 分配阶段耗尽、有限输入拒绝、nil tx、Bot MaxInt64 最后可提交值及耗尽、真实 SQLite AUTOINCREMENT 达 MaxInt64 后 Append 失败、外层后续业务错误全回滚；成功消息不改 Bot version。
- `TestAtomicMessagesPendingAndSnapshot`：管理员消息+delivery+显式有效 session push 同一事务，他连接提交前不可见，提交后齐全；核对 revision=1、创建与排队真实 changes 对应关系、初始 retry_generation=0、完整12键 Message 快照、event_id 与 push 水位；修改当前正文不改变快照。应用消息+同一有效目标仅入 push；Bot 删除级联消息/delivery而保留全局提示，session 删除清理 push。
- `TestMessageBotAuthorizationRecheck`、`TestMessageQueuedInvalidationAndContext`：API/Webhook A/B 串用、receive-only、停用、真实轮换/过时代次、Bot 删除、零身份、认证后能力变化；Webhook 只使用 Send。写事务阻塞期间先停用/降权/代次更新/删除再放行创建，必须拒绝且无新增残留；超时上下文及独立 Store 的真实文件写锁失败均不返回候选。
- `TestMessagePushTargetValidity`：撤销、idle/absolute 过期、管理员代次改变、Web 会话、缺失 session、device 错绑、空/重复目标均失败并原子回滚。
- `TestMessageIndependentStoreConcurrentMonotonic`：32条同 Bot/多 Bot、同文件独立 Store 并发，明确只重试 SQLITE_BUSY；每 Bot 提交序号唯一递增且隔离，全局序号唯一递增，每条都具有真实 push；重开状态一致，删除正文/日志后再重开创建不回绕。无需承诺回滚候选不复用或无空洞。

## 遗留门槛

范围内无已知未解决项。NOT_RUN：HTTP/G1、完整幂等/附件/引用正式业务、P06 调度/租约/重试/ACK、P07 目标生产枚举/偏好与 provider、API 全合同、全仓构建、真实手机/NAS/外部系统。它们属于后续卡，不由受信 primitive 或 SQLite 测试证明。父 P05-01 仍须 S03 独立审计、精确集成与最终组合核查，由总调度维护状态。
