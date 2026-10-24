# P04-06-S02 Bot 三管理路由实施报告

- 角色：developer；结果：implemented，待不同 agent 对最终提交独审，再由调度精确集成；本报告不将 S02 或父卡置 done。
- 工作树 `/home/lijx/.codex/worktrees/bot-admin-http/RelayNest`；分支 `codex/bot-admin-http`。
- 精确基线 `78bbaa5bbbe62c3149853974920047c39330c130`。本报告与实现同一提交，最终实现 SHA 由提交后交接记录，不预写自身 SHA。
- 范围仅 Bot GET/POST `/api/v1/admin/bots`、PATCH `/api/v1/admin/bots/{id}` 的仓储适配、handler、router、main 组装及对应测试。不改 auth/storage/origin/API/SQL/registry；设备会话 S03 的共享组装留待后卡。

## 消费的固定前置

| 前置 | 本基线已包含的精确证据 |
| --- | --- |
| Bot S01 | 原实现 `56fa387e91378a85f0fbaf6a34d51c228a57f196`，main 实现 `bdeaffe846ce90707a4eee3456a8f821ea4d3a30`；源码与原实现相同，独审组合 `1fa1d6c10b964613513844656931f5d210e399db`；见 [S01-I01](P04-06-S01-I01.md) |
| 完整 P04-02 | 集成证据 `e831cf9d9810206f96b21ea16f2dabe0b3df3afe` |
| 完整 P04-03 | 集成证据 `a4df025ae42e29ec45a825a025ade77aef41d96a`；见 [P04-03-I01](P04-03-I01.md) |
| P04-05-S01 | 来源最终实现 `f7e28a29f03f7c3f24e55a32a5a0c84041d8f428`，独审 `44aecdada6df1353ef66c2b55b12d025537dc8d4`，集成证据 `95ca132bd4444fc3c8aa85f9de6573ef76cf4426`；见 [S01-I01](P04-05-S01-I01.md) |
| P04-05-S02-S01 / F01 | 原实现 `45ce3c9c4ef97edad6142d32b9bd999edb8ff3c6`、最终受控事务修复 `c0735c77218bc5281949c9bfd9c1d347807dc3f1`；main 修复 `e8565d78c5a543d4fef79bd6db5edb04a44761e9`；A02 PASS 组合 `31127d824064e53bb10d83ee69dcd138806b3049`，I01 `f698152fcd04d23c37c32399221c3b2d50a9db00`，见 [A02](P04-05-S02-S01-A02.md)、[I01](P04-05-S02-S01-I01.md) |
| 本卡补充规划 | 来源实施 `78cb562535c23727b47066ccdcb7129943d3e359`、独审 `a16202c91f70bbf8d0adfb1aae23f288e8bdd6da`，见 [PLAN01-I01](P04-06-S02-PLAN01-I01.md) |

registry 只由调度维护；本工作树的静态 registry 并非本次派发授权或前置结论的替代品。

## 实现与真实接口

`httpapi.BotAccess` 仅承载绑定原管理员 actor 的同步闭包，避免已有 auth → httpapi 依赖形成循环。main 以生产 `RequestSecurity.Authorize` 取得真实 `AdminRequest`；GET 成功读后调用同一 security 的 `RenewRead`，确认复核成功后才输出页面。POST/PATCH 的 `bots.WriteFunc` 则直接调用 `SessionStore.WithAuthorizedWrite(ctx, actor, fn)`，由 auth 自持同一 Store 的 Write，并使用其给定 Tx 执行业务。

原仓储 `Create/Patch` 保留，新增最小 `CreateWithWrite/PatchWithWrite` 共用原事务体。目标存在性、CAS、Bot/凭据/changes、原 actor 精确身份复核及成功续期全部在同一受控事务；无嵌套写事务或复制认证 SQL。任何错误（包括 COMMIT 失败）都返回空创建结果，不发布明文。公开写回调是受信服务端适配约定，不是对恶意 Go 回调的隔离沙箱。

列表按稳定 ID 有界分页，默认 50、最大 100，游标为不含秘密的 opaque 投影；列表与 PATCH 只返回公开凭据 ID/代次及元数据。请求使用 JSON、拒绝未知字段/null/错误类型/超限；PATCH expected_version 沿用 S01 CAS。创建成功仅返回顶层 bot/api_token/webhook_key。响应 no-store，错误使用公共稳定 code，数据库与 GET/PATCH/errors/logs 不暴露本次明文。

## 实际验证环境与命令

2026-10-24，Linux x86_64，`go version go1.27.3 linux/amd64`。Go 工具目录 `/home/lijx/.local/share/mise/installs/go/1.27.3/bin` 加入 PATH，均 `GOTOOLCHAIN=local`，cwd 为本工作树 `server`。Web embed 的 dist 已存在且可编译；本次恢复没有重新执行 Web 安装/构建，也不将此前未核对的构建命令记作新检查。

| 实际执行命令 | 结果 |
| --- | --- |
| `go test -count=1 ./internal/bots ./internal/auth ./internal/httpapi ./cmd/relaynest ./internal/storage` | PASS，五包全部非缓存通过 |
| `go test -race -count=1 ./internal/bots ./internal/auth ./internal/httpapi ./cmd/relaynest ./internal/storage` | PASS，五包通过 |
| `go vet ./internal/bots ./internal/auth ./internal/httpapi ./cmd/relaynest ./internal/storage` | PASS，退出 0 |
| `go test -count=1 ./cmd/relaynest` | 最终双通道 actor 续期测试补强后重跑 PASS |
| `go test -race -count=1 ./cmd/relaynest` | 最终补强后重跑 PASS，18.588s |
| `go vet ./cmd/relaynest` | 最终补强后重跑 PASS，退出 0 |
| `gofmt -l`（本次五个 Go 路径） | PASS，无输出 |
| `git diff --check` | PASS，无输出 |

新增组合测试使用真实 TLS HTTP、生产 router、RequestSecurity/SessionStore、SQLite 和真实事务；没有以认证桩冒充真实接口。fixture 仅在明确的请求检查后/GET 读取后插入真实 SQLite 状态变更以稳定复现竞争；`TestBotHTTPProductionAssembly` 切回无此插入的实际生产组装，并通过真实登录取得 Web/Android 会话。

## 子卡七项验收映射

| 卡内验收 | 结果与证据 |
| --- | --- |
| 1 三路由真实双通道成功、拒绝无凭据/过期/撤销/两类 Bot 凭据 | PASS：`TestBotHTTPProductionAssembly` 真实 Web/Android 登录后分别创建/列表/PATCH；`TestBotHTTPAuthorizationAndCredentialMatrix` 对四能力组合分别创建 Bot，API Token/Webhook Key × 两通道 × 三路由均 401，逐次比较真实库快照；两通道撤销、空闲/绝对过期均三路由 401；错误无秘密 |
| 2 CSRF/来源与代理正负例 | PASS：`TestBotHTTPWebSecurity` 真实 Cookie POST/PATCH 缺失/错误 CSRF、缺失来源、跨 Origin、Referer 回退、Origin 优先、端口/scheme，可信/不可信 X-Forwarded-*；真实有效 Cookie 写成功；Android Bearer 按既定规则不要求 Web CSRF |
| 3 两 Bot 隔离、receive-only 拒发与 HTTP 管理拒绝 | PASS（现有授权与本三路由范围）：S01 `TestAuthorizationMatrix`、`TestIdentityCapabilitiesAreBound` 与 HTTP credential matrix 组合；未实现的消息/Webhook HTTP 业务验收 NOT_RUN，404 仅证明未知路由不落管理 handler |
| 4 明文、脱敏、CAS/字段/no-store | PASS：`TestBotHTTPContractAndPagination` 真实 DB 全字段断言无创建明文、顶层三字段、分页/输入/错误码/CAS；`TestBotHTTPProductionAssembly` 检查 GET/PATCH 不泄漏且捕获实际 slog；S01 `TestPersistenceAndBoundary` 公开投影无摘要，成功创建以外无法取回明文 |
| 5 事务竞争和失败全回滚、成功仅续期 actor | PASS：`TestBotHTTPActorRecheckAndRollback` 三路由请求后撤销/空闲/绝对到期/管理员代次/会话代次，真实库快照不变；写入 business/创建 credentials/changes/续期/业务后末尾授权/延迟外键 COMMIT 失败全部回滚。PATCH 不新增 credentials，不冒称覆盖该无关分支。GET 读后复核失败不输出成功。`TestBotHTTPSuccessRenewsOnlyActor` 双通道各三路由成功仅续期 actor；保留并实际重跑 S01 提交前停用/能力收回/旧代次/跨 Bot 测试 |
| 6 父五项映射及受影响检查 | 下表逐项；本地验证 PASS，不代替不同角色 A/I |
| 7 单独提交/准确交接/独审集成 | 本实现与报告单独本地提交，提交 SHA 交接；独审与集成待调度派发，不自行标 done |

## 父 P04-06 五项原验收映射

最终待审组合为本基线（含上述前置源码）加本卡单一实现提交；准确提交范围在交接提供。

| 父卡原验收 | 当前证据与边界 |
| --- | --- |
| 跨 Bot/跨路由越权拒绝 | S01 `TestAuthorizationMatrix` 的真实身份/跨目标授权 + 本卡真实三管理路由两类 Bot 凭据拒绝。未来消息/claim/ACK HTTP 路由未实现，NOT_RUN，不把未知路由 404 充作该业务通过 |
| 明文仅一次 | S01 持久化摘要与公开投影 + 本卡创建成功响应/GET/PATCH/error/log/真实 DB 组合；失败事务不返回结果 |
| receive-only 不能发送 | S01 真实授权矩阵及身份绑定回归通过；HTTP 管理拒绝通过；未来发送业务 HTTP 按后卡实际实现另验 |
| 受影响检查命令/环境/结果 | 上述普通/race/vet/gofmt/diff；无未报告的外部系统/手机/正式 AC 验证 |
| 仅本任务提交、精确 SHA/diff/遗留、不同角色 A/I | 当前五个 Go 路径与本报告；基线已固定，最终单提交 SHA 在交接给出。独审/精确集成尚未执行，由调度维护状态 |

当前本卡授权范围内实现及本地组合已覆盖；父卡最终 done 必须等待不同角色的 A/I 和调度结合明确边界判断。本报告不越权提前勾父卡。

## 后续接口与限制

- P05/P06 继续消费 S01 `Authenticate` 签发独立 Bot Identity 与 `AuthorizeTx` 的提交前复核；API Token/Webhook Key 分离，Webhook Key 仅发且仍受 send/停用控制，能力新增必须重新认证取得相应身份快照。
- 未来业务不能以管理会话认证替代 Bot 凭据校验，也不能借本次未知路由 404 声称消息/附件/claim/ACK/renew 已经实现或验收。
- 未实现 DELETE、凭据重置、设备管理、消息业务；无 push、部署、外部消息、新聊天或子 agent。
- 已知未解决实现问题：无。本次独立审计/集成尚待执行；设备 S03 未来组装须保留本次认证与 Bot 两种 handler 能力。
