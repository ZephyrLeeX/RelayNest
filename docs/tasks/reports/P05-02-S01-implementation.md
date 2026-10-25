# P05-02-S01 幂等存储与同事务创建实施

```yaml
task_id: P05-02-S01
role: developer
outcome: implemented
base_sha: 8a70a31c6195ef02a5fc3dc43dc64db8be0dd330
review_base_sha: 137799bc9f3c94b279b94a0d18b01480bb4903bd
implementation_sha: f9c7b7a823051f5bd1f9882310d0e460ff96dd98
commit_range: 8a70a31c6195ef02a5fc3dc43dc64db8be0dd330..f9c7b7a823051f5bd1f9882310d0e460ff96dd98
worktree: /home/lijx/Projects/RelayNest-recovery-20261024/idempotent-create
branch: codex/idempotent-create-20261025
migration: server/migrations/0007_idempotency.sql
scope_integration_sha: a128b20449993b8d854faa4f498a2ca4f7dcd607
```

源码先独立提交，上述为真实已完成提交；本报告后续独立提交，其自身SHA在交接记录，不预写。实施状态等待另人独审与精确集成，不标父P05-02或S01 done。

## 变更与依据

已读AGENTS、CONTRIBUTING、WORKER_PROTOCOL、S01及PLAN01/A01/I01、ALLOC01，定点核PRD §3.3、T §1/2.4/3.1、OpenAPI IdempotencyKey/CreateMessageRequest/Message/ClientMessageReceipt及真实repository/pending。迁移号由ALLOC01唯一分配，未重分配或修改旧SQL/checksum。补充SCOPE01由调度通知最终I生效后才改storage测试；开发树保留原基线，未pick范围文档或改registry。

- 主要实现三个文件：`internal/idempotency/store.go`、`internal/messages/create.go`、唯一新SQL；另有两份对应测试和获准storage测试。未修改repository/pending、auth、HTTP/router/main/API、instance_test或依赖。
- 唯一键为稳定身份+operation+小写UUIDv4。管理员唯一身份，Bot为BotID；API与Webhook操作隔离，不按会话、Token或凭据代次分区。摘要覆盖目标Bot及全部载荷字段，规范化空数组/省略/null，不trim正文，不改变数组顺序，不含Authorization或内部推送目标。
- Bot包装器使用真实Store.Write；Tx primitive先AuthorizeTx复核，再查键/摘要，复用CreateApplicationTx。管理员仅受信Tx primitive，要求外层SessionStore.WithAuthorizedWrite/G1拥有事务，并在重放前复核目标启用/receive能力；未增加管理员公开创建包装器。
- JSON保存原Created快照，包括原message/序号/revision、delivery_id及响应所需元数据，不读取编辑后的消息冒充原结果。重放不新分配UUID、序号、changes、delivery或push job。提交失败Bot包装器返回零值；Tx返回值始终是未提交候选，外层必须传播错误、提交后才能发布。
- 同键异摘要为idempotency.ErrConflict（HTTP后续映射409 IDEMPOTENCY_CONFLICT）；墓碑为ErrExpired（410 RESOURCE_EXPIRED）；正文超64KiB为messages.ErrTooLarge（413）。UTF-8/format/空文本校验沿有限文本路径；非空附件/引用/按钮明确ErrUnsupported，空字段仍参与规范化摘要。结构400及正式HTTP错误DTO由后续接线验证。
- 活记录没有7天TTL。TombstoneTx仅允许消息及关联投递都不存在时同Tx清除result_json，保留键/摘要/资源ID/过期时间，未来清理须至少保留墓碑7天。本卡不启动清理worker。
- 既有永久Bot删除会级联消息/投递。依据PRD永久删除与T §3.1清理后无正文墓碑要求，0007的BEFORE DELETE bots触发器在级联前用message_id或delivery_id定位创建快照，同原事务清正文、转墓碑；删除语句/COMMIT失败时一起回滚。其他Bot不受影响。删除后的旧凭据仍先被拒绝；受信墓碑查重返回ErrExpired，不能借墓碑绕过授权重建。

## 实际检查

环境：Linux amd64，本地Go1.27.3完整路径 `/home/lijx/.local/share/mise/installs/go/1.27.3/bin/go`；测试实际SQLite3.53.4。Node22.23.2/npm10.9.8由 `/home/lijx/.local/share/mise/installs/node/22.23.2/bin`加入PATH。下表Go命令均使用该完整Go路径，从server执行。

| 命令 | 结果 |
| --- | --- |
| web内 `npm ci --ignore-scripts`、`npm run build` | PASS，typecheck及Vite产出ignored webui/dist，未提交生成物/依赖 |
| `go test -count=1 ./internal/idempotency ./internal/messages ./internal/storage ./internal/bots ./internal/auth` | PASS；最终五包均通过 |
| `go test -race -count=1 ./internal/idempotency ./internal/messages ./internal/storage ./internal/bots ./internal/auth` | PASS；最终五包均通过 |
| `go test -count=1 ./internal/storage`、`go test -race -count=1 ./internal/storage` | PASS；按SCOPE01额外单独执行 |
| `go vet ./...` | PASS |
| 最后仅增强重开库测试关闭所有旧Store后，`go test -race -count=1 ./internal/messages`、`go vet ./...` | PASS；最终源码/测试无后续修改 |
| gofmt所有新增Go与storage测试；`git diff --check`及暂存区核查 | PASS |
| 正式新Bot/Android创建HTTP、G1组合、完整附件/按钮/引用/Webhook HTTP | NOT_RUN；本卡未实现/验收HTTP，不代替S02/S03及后续业务门槛 |
| `go test ./...` | NOT_RUN；按卡运行相关五包及全服务端vet |

初跑五包暴露缺少ignored embed dist及历史fixture保留0007导致断号，分别由真实Webbuild和SCOPE01允许的测试适配解决；未隐藏失败、删除断言或修改旧迁移。新增墓碑测试曾发现列名应为deliveries.delivery_id，已修正并完成上述最终复跑。

## 验收证据

| 验收 | 结果与定位（测试函数名） |
| --- | --- |
| 同键并发仅一资源，全部相关计数一次增加 | PASS：TestIdempotentConcurrentSameKey，两Store/12并发，核消息/changes/序号/push/记录计数 |
| 同键不同内容、目标及各有效字段冲突，空值规范化/数组顺序 | PASS：TestIdempotentRecoveryAndOriginalSnapshot、TestCreateDigestFieldsAndNormalization |
| 回复丢失/关闭所有连接/重开库后恢复原message及管理员回执，编辑后原快照稳定 | PASS：TestIdempotentRecoveryAndOriginalSnapshot，两发送身份路径 |
| reset新有效凭据恢复同Bot身份、旧代次拒绝、能力撤销后拒绝重放 | PASS：TestIdempotentAuthorizationAndScopes，使用真实RotateAPIToken/Authenticate |
| 跨Bot、管理员、API/Webhook操作不串键；管理员路径目标入摘要 | PASS：TestIdempotentAuthorizationAndScopes |
| 任一步/COMMIT失败全回滚，无成功包装器候选，CAS/overflow | PASS：TestIdempotentFaultsAreAtomic，9类触发器/真实deferred FK COMMIT故障，核资源/计数/记录不增；另核MaxInt64 |
| 文本UTF8/format/64KiB边界，不支持字段拒绝 | PASS：TestIdempotentValidationAndTombstone |
| 墓碑410不重建、无正文；存活消息或投递保护不按7天删除 | PASS：TestIdempotentValidationAndTombstone、TestProtectedAdministratorReceiptCannotExpire；后者先删消息保留投递 |
| 永久Bot删除成功/语句失败/COMMIT失败及其他Bot隔离 | PASS：TestPermanentBotDeleteTombstonesAtomically；失败后原两身份回执恢复，成功两墓碑正文NULL |
| 全新迁移、真实v6升级、旧checksum拒绝 | PASS：TestRealVersionSixUpgrade以真实1–6 SQL及checksum构库再Open升级7；消息测试全新Open |
| 历史v3/v5升级/失败锁/台账/重复Open非回归 | PASS：完整storage包及race；旧数据快照只比较原5版，另逐项核完整最新ledger/name/checksum及0007空表 |

## 交接接口与限制

S02消费 `Repository.CreateIdempotentApplication(ctx, identity, key, CreateRequest, targets)`，只在err==nil时构造201。需要既有写事务时用其Tx版本。S03仅在真实WithAuthorizedWrite及B01绑定拥有的同一事务内调用 `CreateIdempotentAdministratorTx(ctx, tx, botID, key, request, targets)`，尾部续期/COMMIT失败必须清空候选；本卡受信管理员测试驱动不冒充正式会话/G1验收。

CreateRequest字段与冻结载荷一致；传输层仍须严格JSON、拒绝未知字段/非法null和大小超限，按既有DTO输出十进制字符串序号，不直接JSON输出内部Created/Message。仅通过上述相关包不代表父P05-02完成。没有尚未解决的S01实现阻塞；独审、精确I、正式HTTP与未来清理仍为明确后续门槛。无push、部署、维护、新聊天、子agent或外部业务操作。
