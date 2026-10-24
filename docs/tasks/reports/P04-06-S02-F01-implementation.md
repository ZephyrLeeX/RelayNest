# P04-06-S02-F01 精确 JSON 键修复实施

- 角色：修复 worker；唯一对象 [A01-01 / P2](P04-06-S02-A01.md)。派发基线 `39298292641cd3ac2a388b8ec3e1e20e1718783b`，工作树 `/home/lijx/.codex/worktrees/bot-admin-http/RelayNest`、分支 `codex/bot-admin-http`，开始干净独占。仅修改 `server/internal/httpapi/bots.go`、`server/cmd/relaynest/main_test.go` 和本报告；未改 registry、API、SQL、auth/storage、仓储或生产接线。
- 原实现 `80a87509f0687afbb7f6a23c945b96363da480a8`、原开发基线 `78bbaa5bbbe62c3149853974920047c39330c130`；修复基线含原实现及 A01、修复规划/规划 A01。规划主树集成实现 `42501a9e8bef53946a2c361421631eb2fbf0fc7d`、I01 `5cd145bc7b854e160c59599d93c7fa3fc35202cd`、最终 snapshot `d0ba3acc7449d09177710cb38ba4d5714df12f63`，不据此声称原源码已进 main。最终修复 SHA 在提交后交接，不在报告内预写自身 SHA。
- 前置精确消费表完整沿用 [原实施报告](P04-06-S02-implementation.md#消费的固定前置)；未替换或削弱任何前置。特别保留受控事务修复来源 `c0735c77218bc5281949c9bfd9c1d347807dc3f1`、main 修复 `e8565d78c5a543d4fef79bd6db5edb04a44761e9`、A02 PASS 组合 `31127d824064e53bb10d83ee69dcd138806b3049`、I01 `f698152fcd04d23c37c32399221c3b2d50a9db00`。

## 修复与真实回归

`DisallowUnknownFields` 仍作为原类型解码的一部分；在其前增加仅此 handler 使用的精确属性核对。按 create/patch 冻结允许键检查顶层，并逐个出现的 capabilities 对象检查 send/receive；属性名经过 JSON 转义解码后精确比较，未知键统一 400 / INVALID_REQUEST、no-store。不能只检查最后值的 map，否则重复 capabilities 的早先未声明键仍可能被覆盖。没有拒绝合法重复字段或新增 duplicate JSON policy；精确重复字段沿用原 encoding/json 覆盖/合并行为。

保持 256 KiB 限制、Content-Type、单对象及尾部 EOF、原 null/类型/必填/业务检查；认证仍先于解码，PATCH ID 验证仍先于解码。未改变 HTTP/Go 公共接口、错误契约、事务或凭据协议，无消费者迁移。POST/PATCH 的 WithAuthorizedWrite、actor 复核/CAS/失败不续期/COMMIT 全回滚、创建明文仅提交成功后释放、GET 复核后输出均未改动。

- `TestBotHTTPExactJSONKeys`：96 个真实 TLS/router/RequestSecurity/SessionStore/SQLite 拒绝子例，覆盖两个通道 × POST/PATCH，顶层各允许键及嵌套 send/receive 的首字母/全大写变体（含 Name、NAME、Enabled、EXPECTED_VERSION、Send、Receive），分别替代和附加精确键；各层普通未知键也在完整合法字段旁测试。断言 400、稳定错误码、仅 error 响应/no-store/无已知凭据泄漏，比较 bots、bot_credentials、changes、sessions、administrator 全库快照。事先缩短 actor idle 期限，使错误续期必然可被快照捕获。每例拒绝后立即发送原精确合法请求，POST 得到凭据/正确字段，PATCH 仍以 expected_version=1 得到 version=2 和正确变更。Web 实际 Cookie/CSRF/Origin，Android 实际 Bearer，未使用认证桩。
- `TestBotHTTPExactJSONKeyOccurrences`：POST/PATCH 各拒绝早先重复 capabilities 中的 Send、转义后的顶层 Name 和嵌套 Send，共 6 个拒绝例，均比较五表快照；随后合法重复字段和转义后的精确合法键成功，保留原解码语义。原全部测试/断言原样保留。

## 实际验证

2026-10-24，Linux x86_64；Go `/home/lijx/.local/share/mise/installs/go/1.27.3/bin/go`，`go version go1.27.3 linux/amd64`；Go 命令均将该目录加入 PATH、设置 `GOTOOLCHAIN=local`，cwd 为本树 `server`。已有 Web embed 可编译，本轮未执行 Node/npm 安装或 Web 构建。

| 实际命令 | 结果 |
| --- | --- |
| 修改前 `go test -overlay=/tmp/relaynest-a01-overlay/overlay.json -count=1 -run '^TestAuditBotAliases$' ./cmd/relaynest` | 退出 1，独立旧 overlay 三反例：POST 顶层/嵌套各 201、PATCH 200，均 database_changed=true；准确重现原 finding |
| 最终代码 `go test -overlay=/tmp/relaynest-a01-overlay/overlay.json -count=1 -run '^TestAuditBot(ExactJSONFields\|Aliases)$' ./cmd/relaynest` | PASS，0.667s；沿用原 A01 临时 overlay，未修改其反例/原断言 |
| `go test -count=1 ./internal/bots ./internal/auth ./internal/httpapi ./cmd/relaynest ./internal/storage` | PASS，依次 0.065s / 12.494s / 0.186s / 15.862s / 0.558s |
| `go test -race -count=1 ./internal/bots ./internal/auth ./internal/httpapi ./cmd/relaynest ./internal/storage` | PASS，依次 1.725s / 38.190s / 1.106s / 51.108s / 2.458s |
| `go vet ./internal/bots ./internal/auth ./internal/httpapi ./cmd/relaynest ./internal/storage` | PASS，退出 0 |
| 根目录 `gofmt -l server/internal/httpapi/bots.go server/cmd/relaynest/main_test.go`、`git diff --check` | PASS，无输出；改动 Go 文件已 gofmt |

初次新增测试编译曾因误把 IssuedAPIToken/IssuedWebhookKey 结构用作 string 而失败；已修为实际 Token/Key 字段，以上最终完整普通检查与 vet 均在修正及逐属性检查完成后执行，未隐瞒初次失败。早期定向键矩阵 PASS 后又增加逐属性检查，最终结果以上述完整五包命令为准。

## S02 七矩阵与父五项保留

| S02 原验收 | 本最终待审组合的证据与限制 |
| --- | --- |
| 1 三路由双通道成功/非法与 Bot 凭据拒绝 | 原 ProductionAssembly、AuthorizationAndCredentialMatrix 全部保留并随五包重跑；新增精确键后合法双通道 POST/PATCH 成功 |
| 2 Web CSRF/来源/代理/合法 Cookie 写 | 原 WebSecurity 保留重跑；新增回归走真实 Cookie/CSRF/Origin，未改检查顺序 |
| 3 跨 Bot/receive-only 与管理拒绝 | S01 AuthorizationMatrix、IdentityCapabilitiesAreBound 与 HTTP 凭据组合保留重跑；未来消息 HTTP NOT_RUN |
| 4 明文/脱敏/CAS/字段/no-store | 原 ContractAndPagination/ProductionAssembly/PersistenceAndBoundary 保留；本次逐层精确键和五表无副作用回归补 A01-01 证据 |
| 5 actor 竞争/全回滚/仅 actor 续期 | 原 ActorRecheckAndRollback、SuccessRenewsOnlyActor 和 S01 提交前复核测试保留重跑；本卡再以缩短 idle+全库快照验证拒绝不续期 |
| 6 父五项与受影响检查 | 下表逐项映射；以上记录实际命令，最终 A02/I 尚待执行 |
| 7 独立提交/不同 agent A02/精确 I | 本卡单独本地修复提交；调度随后固定 `78bbaa5..最终修复SHA` 交不同 agent A02，PASS 后精确 I，再由父级核全部条件 |

| 父 P04-06 原验收 | 本次保留证据与边界 |
| --- | --- |
| 跨 Bot/跨路由越权拒绝 | 原 S01 真授权 + 三管理路由凭据矩阵继续通过；未来消息/claim/ACK HTTP 未实现，未知路由 404 只证明不落管理 handler |
| 明文仅一次 | 原成功创建/GET/PATCH/error/log/DB 与失败回滚回归保留；新增拒绝不写库、不返回凭据 |
| receive-only 不能发送 | 原 S01 真授权及 Bot HTTP 管理拒绝保留；未来实际发送 HTTP 由后卡另验 |
| 受影响检查及准确报告 | 实际 Go/普通/race/vet/gofmt/diff 以上表为准，不以旧测试结果替代最终组合 |
| 精确提交/证据/遗留及不同角色 A/I | 本基线+单修复提交为待审组合，交接 SHA；不自行关闭 finding、不标父卡 done，须不同 agent 最终 A02 PASS 和 I |

后续 P05/P06 仍消费 S01 Authenticate/AuthorizeTx，保持独立身份、API Token/Webhook Key 分离、停用覆盖与新增能力重新认证。P04-07 保留父卡门槛；本修复不引入消息/Webhook/claim/ACK HTTP。手机/真机、外部系统、未来业务 HTTP、正式 AC、发布/部署均 NOT_RUN。未 push、未创建聊天、未派子 agent；工作树提交后交还调度，最终 finding 状态及原实现集成由 A02/I 决定。
