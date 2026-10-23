# P04-05-S01 公共管理员请求安全入口实施

- 角色 developer；基线 `3f39182da4cd67522a015a2ec675a3ad43878944`，分支 `codex/admin-request-security`，独占树 `/home/lijx/.codex/worktrees/admin-request-security/RelayNest`，开始干净。
- 前置沿基线保留：完整 P04-02/P04-03、P04-05-PLAN01 与 P04-04 最终登录防护；没有回退到旧认证基线。只改 http.go、新 request_security.go 和对应新测试及本报告，既有 http_test.go 未改。
- 依据卡片、父卡、PLAN01、WORKER_PROTOCOL、CONTRIBUTING、T3.2/T7.1、冻结 API 认证/设备安全 keys，以及 P04-03/P04-04 最终交接。没有新增安全政策、接口路由、依赖或迁移。

## 实际公共接口

`auth.NewRequestSecurity(*auth.SessionStore, *httpapi.OriginPolicy) *auth.RequestSecurity`；消费者注入启动时同一组 session/origin 对象。

- `Authorize(http.ResponseWriter, *http.Request, auth.RequestMode) (auth.AdminRequest, bool)`：路由明确传 `RequestRead` 或 `RequestWrite`；false 已写错误，必须立即 return。只检查，不续期、不撤销、不发 Cookie；所有响应 no-store。模式不是自动依据请求方法推断，写路由必须传 RequestWrite；未知模式按内部不可用 503 拒绝。
- `AdminRequest.Session auth.Session`：身份元数据及不可复用 Session.ID；`Source httpapi.RequestSource`：可信来源解析及外部 Secure 信息。凭据仅私有保存，无原值 getter，JSON 不输出；禁止记录/编码整个身份对象。
- `RenewRead(context.Context, auth.AdminRequest) (auth.Session, error)`：普通成功读取后重新原子鉴权并续期，调用方必须先处理错误再输出响应。原 auth/session 使用它，CSRF bootstrap 沿私有凭据生成。读处理器不能只 Inspect 后永久不续期。此方法单独提交，禁止拿它代替业务写事务复核。
- `AdminRequest.ClearCookie(http.ResponseWriter)`：业务成功自撤后调用；仅 Web 清 Cookie，继承已验证来源的 Secure、HttpOnly、SameSite=Lax、Path=/api/v1、MaxAge=-1。失败不得调用。
- `auth.WriteSessionError(http.ResponseWriter, error)`：ErrInvalidSession/ErrNotInitialized →401 AUTHENTICATION_FAILED，其余→503 SERVICE_UNAVAILABLE；复用原 Error 结构/request_id/retryable，不清 Cookie。来源/CSRF失败403 FORBIDDEN；凭据歧义/通道不符401。

credential 与原 current 的检查提取为唯一实现；session/logout 消费公共入口，Login 仍按原 ValidateLogin/Resolve 和 Create 的登录生命周期运行，错误映射共用 WriteSessionError。P04-04 的摘要槽、IP预算、日志与 strict JSON 逻辑未改。Android 固定 Bearer、Web 固定 Cookie；重复/混合拒绝，Web写同源 Origin/Referer + 单值会话 CSRF；Inspect 期间不续期，旧代次/过期/撤销/真实 Bot 凭据无管理员权限。

## 提交边界与后续依赖

入口成功只是检查时的快照。业务写入必须在自身同一个 `Store.Write`/`sql.Tx` 内按精确 Session.ID 重新读取 actor，核对 revoked=0、idle/absolute期限、administrator.auth_generation 与会话代次一致，再复核 target 归属/有效性、执行变更和成功续期；提交后才执行失效钩子/清 Cookie。禁止把 Session.ID 当 device_id、只信缓存 Session 或把先前 Inspect/RenewRead 当最终授权。

当前 `SessionStore.Inspect/Authenticate/Revoke` 都自行调用私有 access→Store.Write，无公开接收现有 Tx 的复核方法。S02 的允许范围含 session.go/device_sessions，应真实补足事务内 actor/target 复核及成功续期（含竞争/重置/撤销测试），不复用本入口快照无条件写入。本卡未实现或承诺此能力；S02/S03 待本提交独审集成后按实际接口接入。

Bot `repository.go:44 Create(ctx, req)`、`:144 Patch(ctx,id,req)` 没有 actor/Tx 参数，各自 Store.Write；内部仅复核 Bot 的目标/版本，不能在事务内确认管理员会话仍有效。故只把 Authorize 接到这些仓储调用前，仍存在重置/撤销与写入竞争。建议 Bot P04-06-S02 等 **P04-05-S02 实际 Tx 复核接口独审集成**，同时由调度确认一个最小 Bot 仓储补足范围，允许在其写事务内调用该接口，再派 HTTP 消费；仅增加 S01 依赖不足以 ready。不能假定 S02 自动修改 Bot 仓储，或在 HTTP 外层再套一个独立事务就获得原子保证。本卡保持 bots/session/storage/SQL/API 只读。

## 验证与限制

环境 Linux/amd64，Go1.27.3 绝对路径 `/home/lijx/.local/share/mise/installs/go/1.27.3/bin/go`，GOTOOLCHAIN=local；Node22.23.2/npm10.9.8。新树缺 embed，按现锁 `npm ci --prefix web`、`npm run build --prefix web` PASS，产物未提交。

- server 下 `go test -count=1 ./internal/auth ./internal/httpapi ./cmd/relaynest` PASS（8.672s / 0.112s / 1.145s）。包含原 HTTP生命周期、strict JSON F01、来源/代理/安全Cookie、CSRF/失败不续期、重置排序、P04-04 登录槽/限流/日志回归。
- server 下 `go test -race -count=1 ./internal/auth ./internal/httpapi ./cmd/relaynest`：最终 PASS（28.092s / 1.099s / 7.729s），包含真实 Bot 凭据覆盖；首次完整检查也 PASS。
- server 下 `go vet ./internal/auth ./internal/httpapi ./cmd/relaynest` PASS；修改 Go 文件 gofmt、`git diff --check` PASS。
- 新公共入口直接使用真实 SessionStore/SQLite：Web/Android读写成功、只检查无续期、成功读显式续期、无公开凭据字段、Secure清Cookie；重复/混合/错误通道、同源/CSRF、真实 Bot API token/webhook key拒绝；idle/absolute/撤销/管理员代次变化后拒绝，且旧 AdminRequest 无法续期。拒绝均核对会话状态不变、no-store、无Cookie、统一错误结构。
- 首轮新增测试失败为夹具问题：直接构造 Header 的 CSRF键未规范化、缩短 absolute 时违反 idle<=absolute 约束。改为 Header.Add 与同时设置合法期限后通过，未放宽生产策略或既有断言。曾尝试无 python 的编辑命令及错误工作目录 gofmt，均未改文件，后以锁定 Node 和正确路径完成。
- 真实 NAS、手机、WS、正式 AC：NOT_RUN；无设备/Bot HTTP 路由或事务组合验收。本卡只交 implemented，须另人固定 SHA 独审和精确集成，不能据此标父卡 done。
