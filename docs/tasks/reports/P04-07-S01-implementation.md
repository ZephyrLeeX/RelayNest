# P04-07-S01 Bot 凭据轮换与状态事务扩展点实现

- 角色：developer；结果：implemented，等待其他 agent 固定 SHA 独审，不自行标记 done。
- 基线：`1c1988c88ead48e26cfb8cc16d0306a4ab501728`；工作树 `/home/lijx/Projects/RelayNest-recovery-20261024/bot-lifecycle-store`；分支 `codex/bot-lifecycle-store-20261024`。开始时 HEAD/分支核对正确且工作树干净，独占原生非 managed 树。
- 已读根 AGENTS、CONTRIBUTING、WORKER_PROTOCOL、S01/父卡、PLAN01/A01/I01、P04-06-S02-I01、架构 §1、PRD3.1、T4.3、两 reset 冻结 OpenAPI（只读）及 bots 局部实现。规划来源 d42093 / 审 b1cd09 / 集成组合 751e878 已在基线中。
- 本实现提交 SHA 在提交后交接，不在本文预写自身 SHA。变更仅 bots 四个文件及本报告；无 HTTP/router/main、auth/storage、API、SQL、依赖、迁移、registry 修改。

## 实际接口与事务边界

沿用 `WriteFunc func(context.Context, func(*sql.Tx) error) error` 与 `Store.Write`，没有新身份/header/会话签名。

- `RotateAPIToken(ctx, id)` / `RotateWebhookKey(ctx, id)`：本地仓储便利入口。
- `RotateAPITokenWithWrite(ctx, id, write, hook)` 返回 `(IssuedAPIToken, changes.Sequence, error)`。
- `RotateWebhookKeyWithWrite(ctx, id, write)` 返回 `(IssuedWebhookKey, changes.Sequence, error)`，不接收 API 专用 hook。
- `PatchWithLifecycle(ctx, id, req, write, hook)` 返回原 `(Bot, changes.Sequence, error)`。现有 Patch/PatchWithWrite 委托此入口、hook=nil，行为兼容；后续完整装配显式传 hook。
- `LifecycleFunc func(context.Context, *sql.Tx, LifecycleChange) error`；事件为 `Action`（patch / rotate_api_token）、公开 `Before/After Bot`，无摘要/明文。Patch 事件覆盖 enabled、能力和其他 Patch，适配层依前后快照判断队列动作；API 事件含旧/新 API 代次和 Bot ID。

轮换在给定写事务读取目标 → 拒绝 Bot version / 所选 generation 已达 int64 上限 → 现有 issue 安全随机 → 所选凭据 ID/generation/digest 的 CAS → Bot version CAS → hook → typed bot.upsert → 外层会话尾检/续期 → COMMIT。只有 WriteFunc 返回 nil 后返回新明文/序号；任何错误返回零结果。另一凭据完全不写。版本/代次耗尽沿现 ErrConflict；changes AUTOINCREMENT 耗尽沿现 Append/SQLite 错误整笔回滚，不回绕。最后一次 MaxInt64−1 → MaxInt64 合法，下一次拒绝。

hook 与 WriteFunc 都是受信 Go 适配边界，必须同步仅使用给定 Tx 并传播错误；禁止嵌套写事务、网络等待、自行 commit/rollback 或保留 Tx。回调在元数据修改后、changes 追加前运行，失败或后续追加/续期/提交失败会连同回调写入回滚。本卡没有预造 P06-07 租约 schema/函数签名，便利入口和旧 HTTP Patch 不能代表完整队列级联已装配。

## 原卡逐项证据

全部测试使用临时文件真实 SQLite（storage.Open 和现有迁移），没有用 SQL/mock 成功冒充外部系统。

| S01 验收 | 本地证据与结论 |
| --- | --- |
| 两种轮换各自独立、A/B Bot、另一凭据不受影响 | PASS：TestRotationIsolationAndDisclosure 对两种 kind 分别比较全部 Bot 凭据 ID/generation/digest 快照，只允许所选一行改变；另一凭据旧 Identity 可继续授权，B 两凭据仍可认证且公开 Bot 不变；跨 B 授权拒绝。新凭据按原能力授权，Webhook Claim 拒绝。 |
| 新值只成功响应一次、失败无可用秘密 | PASS：同测试验证普通 Bot JSON 无秘密/摘要、typed changes 只有既定元数据；再次轮换返回不同值，前值失效。RollbackMatrix、真实会话组合与 deferred FK COMMIT 故障均要求凭据/结果/seq 为零。没有取回既有明文的读取接口。 |
| 已 Authenticate 的旧身份在 reset/停用提交后最终业务 Tx 拒绝 | PASS：TestLifecycleCommitOrdering 以两个 Store 连接同一 SQLite、显式 channel 写锁屏障，分别调用真实 API/Webhook 轮换及 Patch 停用；后序 AuthorizeTx 返回 Unauthorized/Forbidden，business_marker 无写且无额外 changes。 |
| 先合法提交业务可成功 | PASS：同测试 business-first 屏障固定相反顺序，业务标记及 changes 成功保留，后 reset 成功；不声称追回已提交/已返回的数据。 |
| CAS、int64 上限、随机/扩展点/changes/续期/提交失败原子性 | PASS：TestLifecycleRollbackMatrix 两种 kind 共25分支：missing、Bot version、所选 generation、changes seq 上限、实例局部 issue 故障、凭据/Bot UPDATE 故障、RAISE(IGNORE) 强制两级 CAS 未命中、changes trigger、API hook 写后失败、尾部 write 失败、deferred FK COMMIT 失败；逐一比较 Bot、两凭据及 B 凭据摘要、changes 数量/序号全部不变。TestLifecycleCounterBoundaryAndInputs 验证三种计数最后合法增量/后续拒绝及空 id/nil write。现有 TestConcurrentCASAndSequences 继续通过。 |
| 启停扩展点失败同样回滚 | PASS：TestPatchLifecycleAtomicityAndRestore 覆盖 hook/changes/尾部续期失败、过期 CAS、version/changes 耗尽，hook 的 queue_marker 和元数据全回滚；TestEnableHookFailureRollsBack 单独验证恢复启用 hook 失败。 |
| 停用阻收发、恢复原能力 | PASS：PatchLifecycleAtomicityAndRestore 及既有 TestAuthorizationMatrix 覆盖旧 API/Webhook 身份停用拒绝，恢复保留凭据和原 send/receive 能力，未新增能力；启用/停用 hook 前后快照正确。 |
| Webhook 不触发 API 扩展点 | PASS：Webhook 方法没有该 hook 参数；IsolationAndDisclosure 和真实会话组合要求 API hook 零调用，另一 API 身份、摘要/代次不变。 |
| 真实调用方会话续期与业务同事务 | PASS：外部 package bots_test 的 TestLifecycleAuthorizedSessionTransaction 使用真实 auth.RequestSecurity 签发 Android actor → SessionStore.WithAuthorizedWrite；API/Webhook/Patch 各 success/changes/renew/commit/revoked 五分支。仅缩短持久 idle 截止时间观察续期，无 sleep；成功只续 actor，其他 session 不变。renew trigger 与 deferred COMMIT 错误回滚 Bot、两凭据、queue_marker、changes、会话；入口撤销后不执行业务。 |
| 检查、范围、提交与独立审计 | 本节实际检查通过；只明确 stage 四个 bots 文件及本报告，本地独立 Conventional Commit 后交接 SHA；独审/集成由其他 agent 执行。 |

实例私有 issueCredential 默认绑定现有 issue，测试只注入该实例的错误，不修改全局 crypto/rand。元数据公开结构/鉴权实现未改。并发证据是固定事务顺序的屏障，race 检查作为额外数据竞争验证。

## 实际检查

2026-10-24，Linux amd64，Go `/home/lijx/.local/share/mise/installs/go/1.27.3/bin`，`go version go1.27.3 linux/amd64`、`GOTOOLCHAIN=local`。原树没有 Go embed 的 webui/dist，因此在 web 使用 Node22.23.2、npm10.9.8 执行 npm ci / npm run build（含 typecheck）；产物与 node_modules 均 ignored，没有提交依赖或构建产物。

| 命令（Go 在 server，npm 在 web） | 结果 |
| --- | --- |
| npm ci；npm run build | PASS；16 packages，0 vulnerabilities，typecheck / Vite build 成功 |
| go test -count=1 ./internal/bots ./internal/changes ./internal/storage ./internal/auth ./internal/httpapi ./cmd/relaynest | PASS，六包非缓存运行 |
| go test -race -count=1 同六包 | PASS |
| go vet 同六包 | PASS |
| 最后补充边界/恢复失败测试后：go test -count=1 ./internal/bots ./internal/changes ./internal/storage | PASS |
| 最后补充测试后：go test -race -count=1 ./internal/bots；go vet ./internal/bots | PASS |
| 四个变更 Go 文件 gofmt -l；git diff --check（含 staged） | PASS |

第一次尝试写测试时误在 server 工作目录使用了带 server/ 前缀路径，shell 报路径不存在，该次未写入测试，随后的 bots PASS 只覆盖当时已有内容；修正路径后已实际创建并执行上述完整矩阵，未将第一次执行当作新增测试证据。

## 遗留及后续输入

- 不宣称真实 HTTP reset、取消 Long Polling、撤销/重排租约、消息/失败记录完整保留或永久删除/完整 cascade PASS；本卡仅本地存储扩展点。真实 reset 成功含这些冻结语义，S03 必须等 P06-07、设备 P04-05-S03 最终 A/I，再装配并实测，不提前挂空级联成功端点。
- S02 可沿本实现 Bot version/CAS、typed changes 与受信同 Tx 模式实现确认删除基础；没有迁移分配。
- HTTP reset 的耗尽公开错误映射留 S03 契约门槛，未新增 code/body/header。
- G1 C01 未冻结的身份/附件契约不作为依赖，未添加 instance header；未触碰该独立树。
- 手机/NAS/OPPO、外部系统、完整消息租约矩阵及现网维护未运行，均不阻塞本卡本地存储证据；没有 push、部署、新聊天或额外 agent。
