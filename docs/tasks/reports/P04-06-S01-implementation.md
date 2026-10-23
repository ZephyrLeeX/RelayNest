# P04-06-S01 实施报告

- 角色：developer；结果：implemented，等待独立审计和集成；不标记 done。
- 基线：`5e5ae142f4988aff9c89eaedfbf3d109838306a2`；工作树 `/home/lijx/.codex/worktrees/bot-store/RelayNest`，分支 `codex/bot-store`。
- 范围：`server/internal/bots/{models,repository,authorization}.go` 及 `bots_test.go`；唯一新迁移 `0004_bots.sql`；storage_test.go 迁移夹具；本报告。无 storage 实现、HTTP/auth/API/依赖/旧迁移修改。
- 迁移分配：调度在本卡派发时串行正式分配 0004，基线真实集合仅 0001～0003，本工作树唯一 writer。
- 公共 changes 实际实现 `9041072b5a86610d4e08440732f06b9492918011`；A01 PASS `3314661e6343e36acfa702538f9d7fe35ab7e64f`；main 集成 `b245192604c09445ed21156b1832fcd9e9a1c785`；I01 `c8e782d264226e9204e61494e311ede640ca3bc1`。Bot 规划实现 `1dc49229cbcffd83da8bf0c142d4201c9db43d77`，A01 PASS `3ac208f153b0984a3ab55ec688dd26d5a8949c50`，集成 `a6c47166d671c81731cc9e789b8ea5f4882ef716`。

## 实际接口及边界

`bots.New(*storage.Store) *Repository`：

- `Create(ctx, CreateRequest) (Created, changes.Sequence, error)`：名字非空、UUID v4 Bot ID、version=1；两套独立随机凭据均为公开随机标识 + 点号 + 256 bit 随机秘密，只存 SHA-256 摘要。仅创建成功返回严格 `bot/api_token/webhook_key` 三键。`Created` JSON 模型和 `Bot.credentials` 分离，普通查询无秘密或摘要。
- `Get(ctx,id) (Bot,error)`、`List(ctx) ([]Bot,error)`：公开元数据。List 当前返回一致事务内全部公开投影；HTTP 分页接入属于后续路由卡，不能直接宣称冻结的分页路由已实现。无轮询时 connection 固定 unknown/null。
- `Patch(ctx,id,PatchRequest) (Bot,changes.Sequence,error)`：ExpectedVersion 为 int64 正整数；Name/Enabled/Capabilities 指针表示部分修改；比较与版本递增在同一 Write 事务。MaxInt64 返回冲突，避免溢出；未套用 settings 的 2^53−1 限制。无字段 PATCH 仍递增版本并写事件。
- `Authenticate(ctx,botID,Kind,secret) (Identity,error)`：两种 Kind 为 APIToken/WebhookKey；秘密不可互换，SHA-256 摘要恒定时间比较。Identity 私有字段绑定 BotID、种类、公开凭据标识、generation 和认证时能力；仅提供值类型访问器。无能力可认证，但无操作授权；停用拒绝认证。
- `AuthorizeTx(ctx,*sql.Tx,Identity,botID,Operation) error`：不可构造的身份绑定与数据库当前 credential_id/generation、enabled、send/receive 同事务核对。能力取认证快照与当前值交集；撤权立即有效，新增能力须重新认证。Webhook 仅 Send 且需 send 能力；API send 支持 Send/Edit/Upload，receive 支持 Claim/ACK/Renew/Download；Manage、未知操作、跨 Bot、零身份一律拒绝。无凭据摘要进入 Identity 或公开 Bot。

错误：`ErrInvalid`（无效输入/nil Tx）、`ErrNotFound`、`ErrConflict`、`ErrUnauthorized`（凭据、种类、代次失效）、`ErrForbidden`（停用、能力、跨 Bot、管理操作）；存储错误原样传播，调用方须按 API 错误约定脱敏映射，不能透出原始 SQL 错误。

Create/Patch 仅供独立管理员授权后的调用方使用，Bot Identity 不授予管理能力。未来 P05/P06 必须在实际业务 `Store.Write` 回调内调用 AuthorizeTx 并传播错误，不能用请求开始快照代替；轮询不能跨网络持有事务。重置凭据必须增加代次（本卡未实现重置）。

Bot 写入、凭据创建、CAS 版本和公共 `changes.Append(ctx,tx,changes.Metadata)` 在同一个调用方事务完成，事件为 `bot.upsert`、EntityID=BotID、Revision=version，BotID 非 NULL。任何错误不释放 Bot/明文/seq；只有外层 Write 成功后才返回全局 Sequence。无私有 append/嵌套提交/变更后补/epoch 重置。新增 bot_seq 默认0，未实现消息业务；changes 不增加 Bot 级联外键，删除/墓碑属后卡。

## 真实验证

环境：Linux amd64，本机临时 SQLite 文件（非内存桩）；绝对 Go 路径 `/home/lijx/.local/share/mise/installs/go/1.27.3/bin/go`，`go version go1.27.3 linux/amd64`，`GOTOOLCHAIN=local`；依赖保持 modernc.org/sqlite v1.58.0，既有 storage 测试记录实际 SQLite engine 3.53.4。无升级、无新依赖。

以下 Go 命令从 `server/` 执行，使用上述绝对 Go 路径：

| 检查 | 结果 |
|---|---|
| `go test -timeout 60s ./internal/bots ./internal/storage ./internal/changes` | PASS |
| `go test -race -timeout 90s ./internal/bots ./internal/storage ./internal/changes` | PASS |
| `go vet ./internal/bots ./internal/storage ./internal/changes` | PASS |
| `gofmt -l server/internal/bots server/internal/storage/storage_test.go`（仓库根） | PASS，无输出 |
| `git diff --check`（仓库根） | PASS |
| `npm --prefix web ci --no-audit --no-fund`、`npm --prefix web run build`（仓库根） | PASS；现有 Node v22.23.2、npm 10.9.8；仅生成 gitignored Web 依赖和嵌入产物，无依赖或锁变更 |
| `go test -timeout 90s ./...`（额外扩展检查） | 首次 FAIL：基线未生成 internal/webui/dist，go:embed setup 失败；按现锁完成上述构建前置后复跑全部 PASS |

测试对应验收：

- TestPersistenceAndBoundary：创建三键、独立凭据/摘要、重开持久化、公开模型无秘密、unknown/null、List/Get、公共 Epoch 重开稳定。
- TestAuthorizationMatrix：四种能力、全部操作、种类互换/篡改、错误代次、跨 Bot、无效秘密/零身份、停用对 API/Webhook 及旧身份覆盖。
- TestIdentityCapabilitiesAreBound：旧身份不因上调能力升权，新认证获得新能力。
- TestCASRollbackAndErrors：创建/编辑、CAS 冲突、无 Bot、非法参数、关闭存储错误；在 bots/bot_credentials/changes 注入失败验证无部分提交，编辑事件失败验证版本回滚，失败返回值清空。
- TestCommitFailureDoesNotReleaseResult：真实延迟外键在 COMMIT 阶段失败，Bot/凭据/changes 完整回滚且无一次秘密或序号释放。
- TestConcurrentCASAndSequences：12路CAS只有一次成功；12路创建与编辑共享全局唯一单调日志，关联正确 bot.upsert/version。未要求序列无间隙。
- TestCommitTimeReauthorization：两个独立 Store/写池，先持有 SQLite 写锁并并发提交停用、能力撤销、凭据代次变更，旧身份最终写事务拒绝，changes 无新增。
- storage TestMigrationUpgradeFailureAndLock：真实旧库1/2/3升级到4；新增 real bot upgrade 保持管理员代次和 epoch，Bot/凭据表存在且空，重复迁移安全，旧二进制拒绝新库。人工迁移使用真实集合动态下一编号，原有失败整批回滚、持锁等待、checksum/name/future/gap 校验和版本断言全部保留并通过。未修改迁移器。

真 NAS、Android/手机、真实跨 HTTP 路由、正式 AC：NOT_RUN，属于后续卡/阶段验收。当前独立授权证据不能代替 S02 或正式验收。提交 SHA 在提交后交接，不在报告预写自身 SHA。

后续业务接入仍需在上述同事务授权之外检查具体消息/附件/投递归属（例如 application 编辑权限、对应投递附件下载）；本卡 Operation 仅表达能力与 Bot 隔离，不能当作具体业务实体访问证明。
