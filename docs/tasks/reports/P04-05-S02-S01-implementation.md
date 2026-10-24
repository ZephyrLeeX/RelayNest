# P04-05-S02-S01 实施交接

状态：`implemented`，待不同 agent 固定实现 SHA 独立审计及精确集成；本报告不是安全审计结论，不标父 S02 done，不作为 S03 已冻结契约。

基线：`e2ea912c5bdedf1d57f2cb8a9a58bbc75a65ae15`。工作树 `/home/lijx/.codex/worktrees/device-session-finish/RelayNest`，分支 `codex/device-session-finish`。实施提交 SHA 在提交后的交接消息记录，报告不预写自身 SHA。接管既有 `session.go`、`device_sessions.go` 草稿及前 worker 未提交测试，沿现稿完成验证；原 `device-session-store` 工作树未接触。继承的旧 `P04-05-S02-checkpoint.md` 只读且不提交，完整源码与测试不依赖该文件。

## 变更与边界

仅修改 `server/internal/auth/session.go`，新增 `server/internal/auth/device_sessions.go`、`device_sessions_test.go` 及本报告；复用原 `session_test.go` 的真实 SQLite 夹具但未修改它。沿现有 sessions 列存储安装元数据，不新增安装表/迁移/依赖。共享内部事务鉴权；列表按当前数据库有效会话分页；按不可复用 Session.id 精确撤销；token logout 和目标撤销成功提交后走同一精确 ID hook。自撤也在事务末尾复核身份、期限及代次；非自撤同时续期 actor，并用同一末尾时刻复核双方，失败整体回滚。

实际 Go 接口（本卡候选，尚未独审冻结）：

```go
func (s *SessionStore) AuthorizeTx(context.Context, *sql.Tx, AdminRequest) (Session, error)
func (s *SessionStore) RenewTx(context.Context, *sql.Tx, AdminRequest) (Session, error)
func (s *SessionStore) ListDeviceSessions(context.Context, AdminRequest, string, int) (DeviceSessionPage, error)
func (s *SessionStore) RevokeDeviceSession(context.Context, AdminRequest, string) error
func (s *SessionStore) SetRevocationHook(func(string))
```

`DeviceSessionPage` 含 `Items []Session`、`NextCursor *string`、`HasMore bool`。它是内部 Go 元数据，HTTP 层仍须按冻结 API 映射 JSON。列表 cursor 为 canonical base64url 编码的版本与 UUID，按 Session.id 升序，无跨页快照承诺。内部 limit=0 代表未传而取50，>100截到100，负数/非法 cursor 为 `ErrInvalidSessionPage`；HTTP 明传0等格式语义由后续 S03 按 API minimum=1 处理。尾页 cursor=nil，成功空页 Items 非nil；时间统一 UTC，不含凭据。

`AuthorizeTx` 只复核、不续期/提交，要求真实 RequestSecurity 的私有 store/issuer/issued 绑定与原身份相符，允许此前有效请求已推进 idle 时间。`RenewTx` 在业务与 changes 之后、同一写事务提交前再次复核并续期，绝不越过 absolute。所有错误必须传播出 `Store.Write`，不能忽略。未知/已失效目标为 `ErrSessionNotFound`，actor 无效为 `ErrInvalidSession`，数据库故障沿 `ErrStorage`；未改变 RequestSecurity、API、storage、origin、路由或 main。

hook 只在 `Store.Write` 返回提交成功后同步调用一次，传精确 Session.id；失败/回滚不调用。替换 hook 使用互斥锁，调用在锁外，回调须自身并发安全。回调可以看到已提交数据库状态并再次设置 hook；hook 并非可靠消息队列，也不承诺和其他并发提交的回调顺序。

**待独立审计的接口限制**：裸 `*sql.Tx` 无公开 owner DB 查询。可信 Go 调用者必须传入构造该 SessionStore 的同一个 `storage.Store.Write` 回调 Tx；本实现能拒绝跨 SessionStore actor，但不能强识别内容完全相同的外库克隆 Tx 或错误同库接线。本报告披露不构成用户安全批准、豁免或安全 PASS；独立审计必须判断是否满足原要求，若不满足须保留阻断交调度，不能凭本次测试关闭该问题。

## 真实 SQLite 验收证据

全部使用文件 SQLite、真实 Store.Write/Read 和 RequestSecurity.Authorize（Web Cookie+CSRF+HTTPS 同源或 Android Bearer），未 mock Tx。测试名位于新增 `device_sessions_test.go`。

| 卡验收 | 测试及实际断言 |
| --- | --- |
| 1 安装持久化/隔离 | `TestDeviceSessionsPersistenceAndExactRevocation`：同安装 S1/S2、另一安装 B、Web 共存；关闭重开数据库后完整元数据一致；新会话新 ID/凭据，撤 S1 后 S2/B/Web/重登会话及 Bot 凭据仍有效；device_id 不回退。 |
| 2 有界分页 | `TestDeviceSessionPagination`：103个有效行，默认50/1/最大100/超最大/int最大/负数/非法游标；完整跨页顺序与数量、尾页/空尾页；真实 revoked/generation/idle/absolute 等于 now 的筛除；UTC/完整元数据/敏感字段排除。 |
| 3 精确撤销/旧凭据 | 持久化测试：旧 token Inspect、Authenticate、真实后续 RequestSecurity 均拒绝；重登新 ID，旧 ID 重试失败，不影响同安装和其他安装；Bot 不能成为管理员。`TestDeviceSessionRevokeSerializesResetAndRelogin`：撤销持锁、重置排队、重登新代次后迟到旧 ID 不能命中新会话。 |
| 4 existing Tx 私有绑定 | `TestDeviceSessionExistingTxBinding`：真实签发 actor；公开伪造、篡改 Session/secret、nil/异 issuer、跨 store（另一真实库复制相同 ID/凭据）均拒绝；AuthorizeTx/RenewTx/列表/撤销无持久副作用；nil Tx 拒绝。不能由此推导可识别外库 Tx。 |
| 4 原子业务/changes/续期 | `TestDeviceSessionExistingTxAtomicity`：成功共同提交；业务 CHECK、真实 changes CHECK、回调错误、续期后错误、延迟外键提交错误均全回滚；事务中撤销/代次变动/到期让末尾复核失败，业务/changes/续期及身份变动共同回滚；成功续期受 absolute 上限约束。 |
| 5 提交前失效/竞争 | `TestDeviceSessionRevocationCommitBoundaries`：actor入口/尾部到期、自撤尾部及最终复核到期、自撤写后代次变动、目标 UPDATE 时及续期期间到期、双方撤销/代次失效、未知、UPDATE/commit错误，持久撤销/续期/hook均未逃逸回滚。时钟精确步进，不用sleep。 |
| 5 写入串行化 | `TestDeviceSessionQueuedInvalidation`：另一 Write 已持唯一写连接并用通道控制释放，actor撤销/重置、目标撤销/到期先提交，排队撤销随后失败，不误撤其他会话且不续期/通知。`TestDeviceSessionRevokeSerializesResetAndRelogin` 用事务内时钟屏障验证反向顺序。结果证明本 Store 写连接串行化边界，不宣称任意跨进程时序。 |
| 6 hook/logout | `TestDeviceSessionPostCommitHookAndLogout`：回调独立读连接可见真实提交及续期；精确 ID/每次一次；logout重试无重复；自撤成功一次且不续期。`TestDeviceSessionLogoutRollbackDoesNotNotify`：UPDATE/commit失败无通知、撤销回滚。边界测试同样核对失败通知为0。 |
| 6 并发 hook | `TestDeviceSessionHookConcurrentReplacement`：通道同时放行hook替换和24次真实logout，调用总数精确24；race检测覆盖共享hook。 |

## 实际检查

环境：Linux amd64，Go `go1.27.3`，绝对路径 `/home/lijx/.local/share/mise/installs/go/1.27.3/bin/go`，`GOTOOLCHAIN=local`。本次未重跑 Node/embed 构建；所测包不要求完整服务 embed 构建。

| 命令（Go 三项均在 server/ 下） | 结果 |
| --- | --- |
| `gofmt -w server/internal/auth/session.go server/internal/auth/device_sessions.go server/internal/auth/device_sessions_test.go`（绝对工具路径） | 退出0；随后格式检查无输出。 |
| `go test ./internal/auth ./internal/storage ./migrations` | 退出0，auth/storage cached，migrations 无测试文件。 |
| `go test -count=1 ./internal/auth ./internal/storage ./migrations` | 退出0，auth 11.016s、storage 0.542s，migrations 无测试文件。 |
| `go test -race ./internal/auth ./internal/storage ./migrations` | 退出0，auth 32.257s、storage 2.212s，migrations 无测试文件，无 race 报告。 |
| `go vet ./internal/auth ./internal/storage ./migrations` | 退出0，无诊断。 |
| `git diff --check`、`git diff --cached --check`、逐文件差异/暂存清单核对 | 退出0；仅本卡三个 Go 文件及报告进入提交，继承 checkpoint 未暂存。 |

首次普通三包检查命中 Go 缓存，已追加 `-count=1` 强制实际执行；不冒认前 worker 未留存的检查结果。

## 未运行与后续

真实 WS/provider E2E、真实关闭连接、原子解绑厂商映射均 `NOT_RUN`，当前无对应运行时；P09 仍须持久化映射失效，并在发送/回调提交前复核不可复用 Session 身份与有效性。本卡不实现该功能，也不以 hook 测试代替。未运行全仓库构建/全量测试，范围为受影响 auth/storage/migrations 三包。须另人固定本次实现 SHA 独审、必要修复及精确集成后，再向 S03 交接口；不自行审计、不标父任务 done、不 push/部署。
