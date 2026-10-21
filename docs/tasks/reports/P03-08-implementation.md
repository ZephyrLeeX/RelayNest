# P03-08 SQLite 基础层实施报告

- 角色：开发；结果：implemented，等待独立审计与集成，不代标 done。
- 基线：`f3dbe794f8afe32573435a94e363eb7a3761924f`。
- 工作树：`/tmp/relaynest-run2/storage`；分支：`codex/storage-run2`。
- 本报告随实现提交；最终 SHA 由提交后的交接记录提供。
- 范围：`server/internal/storage/**`、`server/migrations/**`、本报告；总调度按 PUSH-DECISION-01 扩充允许的 `server/go.mod` / `server/go.sum` 仅增加一个 SQLite 驱动及必要传递依赖。

## 实现与接口

`storage.Open(ctx, Config)` 打开文件数据库、启用并核对 WAL、执行迁移后建立只读池；任何错误均关闭已打开连接并返回，不删除、重建或备份数据库。`Config.Path` 为本地文件路径，使用 URL 编码避免文件名中的问号等字符被解释为驱动参数；不接受内存库。调用方负责选择宿主机本地卷，不能用 SMB/NFS。

写池固定 1 个连接；读池默认 4、允许 1～16 个连接。`BusyTimeout` 默认 5 秒、允许 1 毫秒～1 分钟。DSN 对每个新物理连接执行 `busy_timeout`、`foreign_keys=ON`、`synchronous=FULL`；只读池使用 `mode=ro` 和 `query_only=ON`，普通读事务不会申请写锁。没有新增 ORM 或迁移框架。

- `Store.Read(ctx, func(*sql.Tx) error)`：只读一致快照。
- `Store.Write(ctx, func(*sql.Tx) error)`：短写事务；回调失败回滚，提交失败返回错误。调用方必须关闭 Rows，不保留事务，不在回调中等待外部网络、长轮询或上传。
- `Store.EngineVersion(ctx)`：通过 `sqlite_version()` 查询实际引擎版本。
- `Store.Close()`：关闭两个连接池，合并关闭错误。

迁移编号固定为 `0001_initial.sql`，随 `server/migrations` 包嵌入。启动先以 `BEGIN IMMEDIATE` 取得 SQLite 写锁（WAL 下的独占写入权），然后读取与校验持久台账；其他迁移者必须等待或超时报错。已应用编号必须是嵌入文件的连续前缀，文件名和 SHA-256 必须相符。未知版本、版本空洞或不匹配均拒绝；待升级 SQL 与台账在同一事务内全部提交，失败回滚，不执行自动恢复或降级。

基础表：

- `schema_migrations(version, name, checksum, applied_at)`：连续版本、文件名、SHA-256 和 UTC 应用时间。
- `system_state(id=1, sync_epoch, change_floor, config_version)`：初始化 UUID v4 epoch，floor=0、配置版本=1，重开不修改。
- `changes(change_seq, type, entity_id, revision, bot_id, created_at)`：`INTEGER PRIMARY KEY AUTOINCREMENT` 序号；无正文；保留 nullable bot_id，不向尚未存在的 Bot 表添加外键，避免后续 Bot 删除级联丢失 tombstone。

下一迁移编号为 `0002`，由调度继续串行分配。没有提前创建身份、Bot、消息、投递等业务表，没有接入 HTTP 入口或健康探针。

## 驱动版本与兼容边界

固定 `modernc.org/sqlite v1.58.0`，本机实际引擎为 **SQLite 3.53.4**。驱动模块要求 Go 1.25.0；项目保持 Go 1.27.3。按驱动要求，`modernc.org/libc` 精确保持该版本驱动自身使用的 `v1.75.6`，未单独升级。`go.mod` 只有该 SQLite 直接依赖，其余为必要传递依赖；`go.sum` 同时包含 `go mod tidy` 为依赖测试图收集的校验项。

核对来源：

- [固定版本官方包文档](https://pkg.go.dev/modernc.org/sqlite@v1.58.0)：纯 Go 驱动、连接参数、支持平台及 libc 版本约束。
- [官方固定版本 go.mod](https://gitlab.com/cznic/sqlite/-/blob/v1.58.0/go.mod)：Go 最低版本与依赖；另已核对下载模块中的相同文件。
- [SQLite 事务说明](https://www.sqlite.org/lang_transaction.html)：WAL 下 IMMEDIATE 与 EXCLUSIVE 的写入排他语义。

文档支持平台含 Linux amd64/arm64；本轮在 WSL2 Linux amd64 实际执行测试，另通过 Linux arm64 无 CGO 交叉构建。交叉构建只证明可编译，不证明目标 NAS 文件系统、锁、断电耐久性、吞吐或设备运行兼容。未开展漏洞扫描或上游修复状态完整评估，不声称此版本无安全缺陷，发布门槛仍需后续专项核验。

## 实际验证

环境：2026-10-22；Go `go1.27.3 linux/amd64`；Linux `6.18.33.2-microsoft-standard-WSL2`；本地临时文件数据库，无 NAS 真机和现网凭据。下表 Go 命令均使用 `/home/lijx/.local/share/mise/installs/go/1.27.3/bin/go` 的绝对路径执行，工作目录为 `server/`（格式与 diff 检查从仓库根执行）。

| 命令 | 结果 | 证据摘要 |
| --- | --- | --- |
| `go get modernc.org/sqlite@v1.58.0` | pass | 固定驱动及传递依赖下载成功 |
| `go mod tidy` | pass | 完成直接/间接依赖与校验清单 |
| `gofmt -w server/internal/storage server/migrations/embed.go`，随后 `gofmt -l` 同路径 | pass | 最终无未格式化文件输出 |
| `go test -v ./internal/storage` | pass | 9 个顶层测试全部通过，输出实际引擎 `3.53.4`（此后补充升级新连接持久校验，由后续全服务测试覆盖） |
| `go vet ./...` | pass | 无诊断 |
| `go test ./...` | pass | cmd、health、httpapi、storage 全通过；migrations 无独立测试文件 |
| `go test -race ./...` | pass | 全服务测试通过，无竞态报告 |
| `CGO_ENABLED=0 go build ./...` | pass | Linux amd64 无 CGO 构建成功 |
| `CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build ./...` | pass | Linux arm64 交叉构建成功 |
| `git diff --check` | pass | 无空白错误 |

存储测试对每个池同时占用全部物理连接并发查询 WAL/FULL/外键/超时，实际触发外键拒绝、只读写入拒绝、回调回滚、延迟外键导致的提交失败、取消上下文和 20 个并发写事务。迁移覆盖 6 个同时启动者、持有写锁时的竞争失败、首次迁移故障原子性、升级多步故障原子性、升级成功及重入、新连接持久版本、旧程序拒绝新版本、名称/校验和/未知版本/版本空洞拒绝、无重建 epoch 保留、序号删除后不复用、UUID v4 epoch 重开保持，以及非法配置和迁移文件编号拒绝。

## 验收与遗留

| 卡内验收 | 结果 |
| --- | --- |
| WAL/FULL/外键各连接生效 | pass：同时持有写连接与全部读连接并发检查 |
| 迁移失败不重建 | pass：首建/升级故障、台账损坏拒绝与 epoch 保留 |
| 引擎版本可查 | pass：公开接口和实际查询 `3.53.4` |
| 相关检查与独立提交 | 检查完成；最终提交 SHA 在交接提供 |

未执行：NAS 真机部署、SMB/NFS（架构明确不支持）、断电/磁盘损坏恢复、真实磁盘满故障、长期压测、漏洞扫描；不将上述项目记为通过。本卡不新增自动备份，不保证不可逆迁移降级。没有需要产品决策的新事项。独立审计由其他 agent 针对最终 SHA 执行。
