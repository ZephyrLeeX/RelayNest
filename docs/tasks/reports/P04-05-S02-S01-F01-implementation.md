# P04-05-S02-S01-F01 受控管理员写事务实施报告

- 角色：修复；交付实现，待不同 agent 对最终组合执行 A02；不自行关闭 A01-F01，不标父 S02 完成。
- 精确开发基线：`2d80e85e7a9850d33e26716931cfe95b7f7e348e`，已含原实现 `45ce3c9c4ef97edad6142d32b9bd999edb8ff3c6`、原 A01、规划及规划独审。
- 工作树：`/home/lijx/.codex/worktrees/device-session-finish/RelayNest`；分支：`codex/device-session-finish`。
- A02 应审原实现与本修复组合：`e2ea912c5bdedf1d57f2cb8a9a58bbc75a65ae15..最终修复提交`，最终 SHA 在提交后交接，不在本报告预写自身 SHA。
- 实际修改：`server/internal/auth/device_sessions.go`、`device_sessions_test.go` 和本报告；`session.go/session_test.go` 无需改动。原未跟踪 checkpoint 未修改、未暂存。

## 接口与事务语义

新增 `SessionStore.WithAuthorizedWrite(ctx context.Context, actor AdminRequest, fn func(*sql.Tx) error) error`。调用者只提供 actor 和受信业务回调，不能提供外部 Tx、Store、owner 标志或事务工厂。入口固定使用 `s.store.Write` 创建事务，顺序为私有入口鉴权 → 业务/changes 回调 → 私有末尾鉴权及续期 → Store.Write 提交。nil/零值 SessionStore、nil 回调均安全拒绝，不调用业务。

公开 `AuthorizeTx/RenewTx` 已删除，改为私有 `authorizeTx/renewTx`，没有弃用包装或公开别名。所有调用都位于本文件的受控入口、列表或精确撤销的自身 Store.Write 闭包。`authorizeStateTx` 和 `session.go` 的私有底层检查保持私有；原有 actor.store/issuer/issued、公开 Session 防篡改、当前 generation/revoked/idle/absolute 检查保留，已由其他请求推进的 idle 允许正常消费，续期不倒退且不越 absolute。

回调业务/CAS/SQL 错误原样返回；鉴权失败仍为 ErrInvalidSession，鉴权存储、创建事务、尾部续期和提交错误沿既有 ErrStorage 语义。任何失败均由 Store.Write 回滚，只有入口返回 nil 才能公布成功结果、明文凭据或通知。取消测试发生在业务与 changes 写入后、尾检前，最终持久数据全部回滚。

回调遵循现有 Store.Write 的受信 Go 代码约束：同步使用所给 Tx，关闭 Rows，不自行 Commit/Rollback、不保留或另开事务。本修复排除误接克隆外库 Tx 的公开入口，不声称隔离恶意 Go、业务 SQL 或 unsafe；没有证据表示远程请求可提供 Tx。现有 changes.Append 继续直接使用该 Tx，无 storage/shared/API/SQL/依赖变更。

列表与撤销只把旧公开方法调用改为私有 helper。精确自撤继续使用本次 revoked=1 的私有专用检查、跳过续期、提交前同一末尾时刻双方检查；没有套用一般业务闭包。一般闭包中将 actor 撤销会尾检失败，相关写入一起回滚。精确 ID hook 仍只在撤销成功提交后调用。

## 旧 F01 的独立实证

在本轮修改源码前，HEAD 为上述精确开发基线时，使用 A01 报告内的 `TestAuditForeignTxAfterOriginRevocation` 独立重现。把当时完整 device_sessions_test.go 复制到 `/tmp/relay-f01.*`，追加报告中该测试，经 Go overlay 运行；没有修改测试源码或仓库基线：

```sh
cd server
GOTOOLCHAIN=local /home/lijx/.local/share/mise/installs/go/1.27.3/bin/go test -overlay "$tmp_f01/overlay.json" -count=1 -run TestAuditForeignTxAfterOriginRevocation -v ./internal/auth
```

实际退出0，auth 0.009s。两个真实文件 SQLite fixture、真实 RequestSecurity 签发 actor，复制会话 ID/秘密摘要/元数据/期限到第二库，原库 generation 改为2，外库仍为1。原库 Write 调旧 AuthorizeTx 返回 ErrInvalidSession；同一个原 SessionStore/actor 接克隆外库 Tx 时旧 AuthorizeTx 成功，写入真实 changes 后旧 RenewTx 成功提交。测试断言外库 changes count=1 且 idle 增长，并明确记录原库拒绝/外库接受对照。该通过表示**缺陷成功复现**，不是安全验收通过。

## 新入口回归与六矩阵映射

所有新增/迁移测试使用真实 SQLite，断言最终持久状态；没有 mock Tx、sleep、弱化旧断言或只用 race 代替竞争验证。

| 矩阵 | 最终测试与证据 |
| --- | --- |
| ① 安装持久化与隔离 | 原 `TestDeviceSessionsPersistenceAndExactRevocation` 原样保留：文件库重开、同安装 S1/S2/另一安装/Web、会话与凭据新身份，精确撤销不影响其他设备/重登/Bot。 |
| ② 分页/UTC/无秘密 | 原 `TestDeviceSessionPagination` 原样保留：103行、默认/上限/非法参数/边界，当前代次、撤销和期限过滤、排序及公开元数据；源码仍只选择公开列，UTC，无秘密输出。 |
| ③ 精确撤销与旧身份 | 原持久化精确撤销测试及 `TestDeviceSessionRevokeSerializesResetAndRelogin` 原样保留；旧 token/真实请求被拒绝、device_id 不回退、新登录不复用旧 ID。 |
| ④ 私有绑定及原子性 | 原 ExistingTxBinding/ExistingTxAtomicity 迁移为 `TestDeviceSessionAuthorizedWriteBinding/Atomicity`，直接调用最终 WithAuthorizedWrite；保留公开伪造、元数据/秘密篡改、nil/错误 issuer、跨 SessionStore、业务 CHECK、changes CHECK、回调错误、延迟外键 commit 错误、撤销/代次/期限尾检及 absolute 上限断言，补零值 actor、nil/零值 Store、nil回调、续期 SQL 触发器失败、业务写后取消、absolute尾部到期、其他请求已推进idle及错误身份/映射。 |
| ⑤ 双方有效性、竞争、自撤末尾 | 原 `TestDeviceSessionRevocationCommitBoundaries`、`QueuedInvalidation`、`RevokeSerializesResetAndRelogin` 原样保留14种提交边界、排队撤销/reset、精确自撤最终同刻检查及重新登录隔离；新 `AuthorizedWriteQueuedInvalidation` 用通道固定撤销/generation/到期先提交，断言回调不执行、业务/changes/idle不变；新 `AuthorizedWriteSerializesReset` 固定受控业务先持写事务，reset只能串行在后，业务+真实changes.Append+renew共同提交，随后旧actor拒绝。 |
| ⑥ logout/hook | 原 `TestDeviceSessionPostCommitHookAndLogout`、`HookConcurrentReplacement`、`LogoutRollbackDoesNotNotify` 原样保留；成功后可见、精确 ID、每次成功一次、失败无通知、自撤不续期、并发替换无 race。 |

新增 `TestDeviceSessionAuthorizedWriteOwnsSourceStore/reset=true` 是正式 F01 回归：两文件库复制同一会话并用外库真实 guard 验证克隆行有效；原库重置 generation 后调用原 SessionStore 受控入口拒绝，回调执行次数0。另一 SessionStore 收到原 actor 同样拒绝。两库业务/changes count=0，原库和外库 idle 均不变。不存在传入外库 Tx 的参数。

同测试 `reset=false` 证明成功路径拿到源库事务：回调写真实业务表与 `changes.Append`，只有源库 count=1、idle推进，外库业务/changes count=0、idle不变。不是源库二次查询搭配外库业务写入。

原“续期后回调错误”不能在最终入口中由外部插入，已按规划以末尾续期 SQL 失败与**续期后延迟外键 commit 失败**证明等价持久回滚；原 commit-error 的业务/changes/idle 全回滚断言完整保留。一般闭包内 revoked/generation 变更及期限到达也证明尾检不可省略，撤销或代次临时修改本身均随业务回滚。

## 实际执行的检查

环境：Linux amd64，Go `/home/lijx/.local/share/mise/installs/go/1.27.3/bin/go`，实际 `go version go1.27.3 linux/amd64`，所有测试/vet 使用 `GOTOOLCHAIN=local`；真实文件 SQLite 本地夹具。

| 命令 | 实际结果 |
| --- | --- |
| 修改两文件 gofmt -w，最终 gofmt -l | 已格式化，最终无输出。 |
| `go test -count=1 -run 'TestDeviceSessionAuthorizedWrite' ./internal/auth` | 新入口局部通过；补竞争后0.056s，最终差异由下列完整三包检查覆盖。 |
| `go test -count=1 ./internal/auth ./internal/storage ./migrations` | 退出0；auth 10.995s，storage 0.547s；migrations `[no test files]`。 |
| `go test -race -count=1 ./internal/auth ./internal/storage ./migrations` | 退出0；auth 31.281s，storage 2.312s；migrations `[no test files]`，无race诊断。 |
| `go vet ./internal/auth ./internal/storage ./migrations` | 退出0，无输出。 |
| `git diff --check` | 退出0；提交前再次检查。 |
| auth定向搜索 `AuthorizeTx\|RenewTx` | 无结果；定向核对私有调用点全部在自身 Store.Write 中。bots包的同名 Bot身份授权独立存在且未改，不是SessionStore旁路。 |

未运行全项目、手机、真实 WS/provider E2E，均为 **NOT_RUN**；没有宣称真实连接关闭或provider映射解绑。P09 持久化失效和发送/回调前复核义务仍保留。

## 下游与未决

S03、BotS02 只能消费最终独审并集成的接口；Bot Create/Patch 后续应把原业务与 changes 事务体直接交 WithAuthorizedWrite，不另开外层 Write，不传裸 Tx，不嵌套事务，CAS/凭据只在成功后发布；GET 继续原 RenewRead。此次没有修改 Bot、HTTP、router、main 或 RequestSecurity。

无新增需用户选择的需求或架构变更。遗留门槛：由不同 agent 对完整组合 SHA 执行 A02，复核 F01 与六矩阵，再精确集成；旧 A01/规划 PASS 不替代新代码审计。该 finding 的关闭仅由后续独审决定。提交只暂存本卡两源码及本报告，不包含旧 checkpoint，不 push/部署、不改 registry。
