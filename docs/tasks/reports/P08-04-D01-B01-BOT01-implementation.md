# P08-04-D01-B01-BOT01 内部 Bot API 身份发现实施

- 角色：developer；结果：implemented，等待另一实施者之外的 agent 独审与精确集成，不标父任务或附件 gate 通过。
- base：`a346e033da54f7f302fcf2819c90d68737a9d5a7`；分支：`codex/bot-api-identity-20261025`。
- worktree：`/home/lijx/Projects/RelayNest-recovery-20261024/bot-api-identity`。
- 实现 SHA 在本报告提交后随交接提供，不在提交内预写自身 SHA。
- 输入：BOT01 卡、SCOPE01 报告/A01/I01、C01-I01 与 TECHNICAL_DESIGN §3.3 已批准的两附件 Bot-first 局部契约。恢复时已有未跟踪 `api_identity.go` 草稿，先核对后续写；本卡只新增该文件、同包测试与本报告。

## 实现与交接语义

`Repository.DiscoverAPIIdentity(ctx, secret) (Identity, APIMatch, error)` 不需要 botID，只在现有只读事务中以唯一 credential_id 且 kind=api_token 定位、常量时间比较完整 token 的 SHA-256 摘要，成功后绑定现有不可由外包调用者构造的 Bot ID/kind/代次/能力快照。没有读取 HTTP、实例头、附件或管理员会话，没有改原 Authenticate、Webhook 或 AuthorizeTx 政策，没有新增 SQL、索引、依赖或公开 HTTP 接口。

- `APINoMatch, nil`：当前内部解析确定不能匹配、查询无行或摘要不符；返回零 Identity。只有这个结果允许未来调用方进入 ID→Admin。解析的分隔符/长度仅适配现有私有发行格式，不能被当成稳定 wire 分流协议，更不构成认证。
- `APIMatched, nil`：摘要核验成功，读事务提交后释放 Identity；业务仍须在同一写事务调用 AuthorizeTx，并单独检查附件资源域。
- `APIMatched, ErrForbidden`：摘要匹配但 Bot 停用，返回零 Identity；不能 fallback Admin。
- 未确定匹配的依赖/存储/取消/超时：`APIUnavailable, ErrAPIIdentityUnavailable`。摘要匹配后的读事务故障仍保留 `APIMatched` 并返回同一固定错误；任何此错误都必须终止、由未来 HTTP 调用方映射 503 INSTANCE_UNAVAILABLE / “实例暂不可用” / retryable=true。零值 APIMatch 是失败关闭状态。

只在 `match == APIMatched && err == nil` 时使用 Identity。后续 AuthorizeTx 的401/403或资源层404及依赖故障均不能重新解释成 NoMatch。错误不包装原始数据库错误，不包含 token/credential_id；本卡没有日志输出。AuthorizeTx 的 Download 仅沿原 receive 能力检查，不能据此声称已实现 send 下载或附件资源权限。

## 实际验证

环境实际核对：Go `go1.27.3 linux/amd64`，完整可执行路径 `/home/lijx/.local/share/mise/installs/go/1.27.3/bin/go`，`GOTOOLCHAIN=local`；Node `v22.23.2`、npm `10.9.8`，PATH 仅子进程加入 `/home/lijx/.local/share/mise/installs/node/22.23.2/bin`。没有升级依赖；`npm ci` 按现有 lock 安装，构建产物不入提交。

以下在本工作树实际执行并最终通过：

| 目录 | 命令 | 结果 |
| --- | --- | --- |
| web | `PATH=/home/lijx/.local/share/mise/installs/node/22.23.2/bin:$PATH npm ci` | PASS，16 packages added |
| web | `PATH=/home/lijx/.local/share/mise/installs/node/22.23.2/bin:$PATH npm run build` | PASS，tsc + Vite，供 Go embed 使用 |
| 根 | `GOTOOLCHAIN=local /home/lijx/.local/share/mise/installs/go/1.27.3/bin/gofmt -w server/internal/bots/api_identity.go server/internal/bots/api_identity_test.go` | PASS |
| server | `GOTOOLCHAIN=local /home/lijx/.local/share/mise/installs/go/1.27.3/bin/go test ./internal/bots -count=1` | PASS |
| server | `GOTOOLCHAIN=local /home/lijx/.local/share/mise/installs/go/1.27.3/bin/go test -race ./internal/bots -count=1` | PASS，包含原生命周期/Webhook/授权测试及新增并发轮换 |
| server | `GOTOOLCHAIN=local /home/lijx/.local/share/mise/installs/go/1.27.3/bin/go vet ./...` | PASS |
| server | `GOTOOLCHAIN=local /home/lijx/.local/share/mise/installs/go/1.27.3/bin/go test ./... -count=1` | PASS，cmd/relaynest、auth、bots、changes、devices、health、httpapi、storage、webui |
| 根 | `git diff --check`、显式 stage 后 `git diff --cached --check` | PASS |

首次 bots 测试因缺少 webui/dist 在 setup 阶段失败；构建 Web 后解决。新增测试首次编译把 IssuedAPIToken 的 Credential 字段误作匿名嵌入，修正字段访问后通过。最终再次执行 race 与 vet，覆盖最终源码/测试；未用跳过断言掩盖失败。

验收对应真实临时文件 SQLite，不用查询桩：

- `TestDiscoverAPIIdentityIsolation`：两个 Bot 独立 ID/代次/能力绑定；与显式 Authenticate 结果相同；跨 Bot 事务拒绝；未知 ID、同 ID 错摘要、畸形值、Webhook 为 NoMatch；API 轮换前后发现及旧 Identity 代次失效。形状正确仍必须核摘要。
- `TestDiscoverAPIIdentityMatchedRemainsTerminal`：匹配后停用仍 Matched/ErrForbidden；匹配后的停用、能力撤销、轮换、删除凭据、代次变化、Bot 删除及依赖/取消均在真实 Store.Write 中拒绝，受保护写入未执行。没有在本测试伪造 Admin fallback 或 HTTP 404。
- `TestDiscoverAPIIdentityUnavailable`：关闭存储、缺凭据表、实例身份缺失、已取消/已过期上下文返回固定不可用错误和零身份，不作为 NoMatch，不泄露原始错误或凭据。
- `TestDiscoverAPIIdentityWaitingDeadline`：真实读连接池仅1连接，另一个事务以 channel 占用连接；发现等待时超时失败关闭，释放连接后恢复正常，无 sleep 或查询替身。
- `TestDiscoverAPIIdentityConcurrentRotation`：8个 goroutine 发现/事务复核与 API 轮换并发，观察结果只可为匹配或确定无行；匹配快照只绑定原 Bot/代次，轮换后旧凭据及旧 Identity 均失效，race 通过。

## 限制与后续门禁

NOT_RUN：本卡没有真实附件 HTTP 路由，故未验证两操作的 header/ID/Admin 顺序、401/403/404/503 外壳与 no-store、send/receive 附件资源域、文件打开顺序或实例 ID/auth_generation/凭据代次的完整业务事务组合；这些必须由未来真实附件接入验证。未通过注入模拟“摘要匹配后读事务提交故障”，该防回落路径由代码检查覆盖，依赖等待/取消与匹配后的写事务故障有真实 SQLite 测试。没有执行 API validator、Android/phone/NAS、WS/provider、签名/Keystore 或产品阶段验收；本次改动不涉及这些接口或平台。

本基线不含 main 后续0006消息 schema/M01，最终集成应在精确 main 组合再跑相应检查；本卡不挑拣共享 schema、不扩展附件或仓储源码。未改 registry、未 push/部署、新聊天或额外子 agent；无待用户决定的契约冲突。
