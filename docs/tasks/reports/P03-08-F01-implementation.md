# P03-08-F01 WAL 初始化竞争修复实施报告

- 角色：修复；结果：implemented，等待不同实现者的 P03-08-A02 独立复审，不关闭发现或代标 done。
- 对应发现：`P03-08-A01-F01`；实际起始 HEAD：`438343683e2445d89c01f46f840adb539ad12f4f`。
- 代码原基线：`063c270eb0eef463526f9718691fe5ff69f147f2`（含原实现 `88878e6fe1573300f3d6fb6d7fb0d59a1cb87059` 和 A01）；其后仅修复规划 `b98485339996434193209e930dc6c3115decda33` 与规划审计 `4383436`，没有中间源码变更。
- 工作树 `/tmp/relaynest-run2/storage`；分支 `codex/storage-run2`；开始时干净。本报告随实现提交，不预写自身提交 SHA，最终 SHA 在提交后交接。

## 修改与契约

`Open` 仅将原单次 WAL 设置替换为私有辅助函数，迁移仍在 WAL 成功后运行。辅助函数独占写池物理连接，临时设置 `busy_timeout=0`，以单个 `context.WithTimeout` 管理全部 WAL 尝试，间隔 5ms 重试。只对固定驱动 `*sqlite.Error` 数值主码 `SQLITE_BUSY`（5，含扩展码）重试；非锁错误、LOCKED、模式核对失败均直接可见，不匹配错误字符串。预算到期保留 BUSY 错误链及 DeadlineExceeded；调用方取消/更早 deadline 返回可由 `errors.Is` 识别的 context 错误。每次尝试前检查取消，已取消不会发起 WAL 查询。

驱动 busy handler 在这个阶段暂停，避免其内部等待与外部重试叠加完整预算。无论成功失败，退出时恢复该物理连接配置的 busy timeout；恢复错误合并返回，`Open` 原有失败关闭逻辑继续关闭全部资源。成功后仍验证 `wal`，再进入原迁移/只读池建立流程；未来新物理连接的 DSN PRAGMA 未变。

已核对缓存中的固定 `modernc.org/sqlite@v1.58.0/error.go`（Error.Code 数值码）、`stmt.go`（query 的 context 检查及取消错误映射）、`sqlite.go`（interruptOnDone）源码，并以真实 SQLite BUSY/READONLY 与运行中取消补充动态证据。没有依赖升级、锁依赖或迁移重构。

公共 Go API、Config 字段/默认值/限制、Schema、迁移编号/SQL/台账校验均不变。只有 WAL 竞争处理增加有界等待与可识别取消，内部预算耗尽错误链新增 DeadlineExceeded。迁移错误不进入这个重试循环。未接入 HTTP、健康探针或 P4 业务。

## 回归设计与耗时

原 `TestConcurrentOpen` 保持 6 调用者全部成功和每位调用者单条迁移台账断言，只增加统一起跑 channel；每次测试由 TempDir 提供全新库，没有串行化 Open、降低并发或放宽断言。

`wal_test.go` 先用真实 Open 生成有 epoch 的库，关闭后转为 DELETE journal，再以另一个 SQLite 连接 `BEGIN; SELECT` 持有 SHARED 读锁。此时 WAL 切换需要的锁无法获得，尚未执行任何迁移 BEGIN。`TestWALWaitBoundaries` 的 query 每次均实际执行同一 `PRAGMA journal_mode=WAL`；仅在第一次真实返回数值 BUSY 后发出 channel 信号，之后才释放锁或取消。测试闭包负责观测实际 SQLite 结果，不使用桩替代 WAL 竞争，也不以任意 sleep 假定等待已开始。

测试事先固定：总预算 150ms，持续持锁断言耗时在 `[150ms,350ms]`；进入 BUSY 后取消允许 200ms 调度容差；deadline 为 60ms（小于 1s busy 预算），上限 260ms；预取消断言零次查询。释放场景要求至少两次尝试且最终成功。每种场景结束后释放锁、重新真实 Open，检查 epoch 保持和写连接 busy timeout 恢复。额外通过整个 `Open` 验证 150ms 持锁失败、60ms deadline、预取消及释放后的重开。READONLY 非锁错误在 200ms 内返回（其预算 1s），不被持续重试。

25 轮精确边界命令的实际观测摘要：

| 场景 | 实际观测 |
| --- | --- |
| BUSY 后释放 | 每轮 2 次查询，约 5.390～5.613ms 成功 |
| 持续持锁，150ms 总预算 | 每轮 29 次查询，约 150.122～151.128ms 失败；保留 BUSY 错误链 |
| 第一次真实 BUSY 后取消 | 每轮 1 次查询，从启动至返回约 58～145µs，错误为 context.Canceled |
| 60ms deadline / 1s busy 预算 | 每轮 12 次查询，约 60.011～61.059ms 返回 DeadlineExceeded |
| 调用前取消 | 每轮 0 次查询，约 9～43µs 返回 Canceled |

这些是本地调度与临时库结果，不能解释成目标 NAS 的延迟保证。精确边界详细输出保存在本机 `/tmp/relaynest-storage-f01-boundaries.log`（临时验证日志，不入库）；测试会自行输出尝试次数、总耗时和取消/释放后耗时，可按下列命令复验。

## 实际检查

日期 2026-10-22；Go `go1.27.3 linux/amd64`；Linux `6.18.33.2-microsoft-standard-WSL2`；固定 SQLite 驱动 v1.58.0，本地临时文件数据库。下列 Go 命令均用 `/home/lijx/.local/share/mise/installs/go/1.27.3/bin/go`，cwd 为 `server/`；格式工具为同目录 `gofmt`，Git/格式检查 cwd 为根目录。

| 命令 | 实际结果 |
| --- | --- |
| `go test -run '^TestWAL' -v ./internal/storage` | 初次新增真实边界测试通过，约 0.391s（当时尚未补充 OpenContext 与超时竞争错误合并分支，最终版本由下述全部命令覆盖） |
| `go test -run '^TestConcurrentOpen$' -count=100 ./internal/storage` | PASS，1.153s，100 个全新库，每轮 6 调用者成功 |
| `go test -count=25 ./internal/storage` | PASS，12.469s，包含原迁移/连接/事务与新增全部边界 |
| `go test -run '^TestWAL(WaitBoundaries\|OpenBudgetAndRecovery\|NonLockError\|OpenContext)$' -count=25 -v ./internal/storage` | PASS，11.268s；表中 `\|` 仅为 Markdown 转义，实际 shell 单引号中使用普通正则 `|` |
| `go test -race ./...` | PASS，storage 1.866s，无竞态；cmd/health/httpapi 使用 Go 已有缓存通过；migrations 无独立测试 |
| `go vet ./...` | PASS，退出 0，无诊断 |
| `go test ./...` | PASS，storage 0.497s；其他包已有缓存通过；migrations 无独立测试 |
| `gofmt -l server/internal/storage/{storage.go,storage_test.go,wal.go,wal_test.go}` | PASS，无输出（实际逐路径传入，与此 shell 展开等价） |
| `git diff --check` | PASS，无空白诊断；提交前检查暂存范围及 `git diff --cached --check` |

完整 storage 的 25 轮维持逐连接 WAL/FULL/FK/busy_timeout、只读拒写、事务失败回滚、迁移首建/升级失败原子性、台账 name/checksum/未来版本/空洞拒绝、旧程序拒绝新版本、不重建/epoch 保留和并发单次迁移通过。原审计 25/100 轮的失败是历史证据，未计为本轮通过；本轮所有执行命令均无失败。

## 交接与限制

允许路径内只改 `storage.go`、`storage_test.go`，新增 `wal.go`、`wal_test.go` 和本报告。`migrate.go`、迁移 SQL、依赖文件、registry、卡片和旧报告均未修改。没有推送、部署、发布或派生额外 agent。

本卡验收的并发 100 轮、完整 25 轮、真实持锁/运行中取消/预算/提前 deadline/预取消/非锁失败/失败后重开与迁移连接非回归均有上述证据；仍须独立 A02 针对最终精确 SHA 核验后关闭发现。未执行 NAS/ARM 真机、SMB/NFS、断电、磁盘满、磁盘损坏、长期负载或漏洞扫描；没有将这些后续专项记为通过。无新增需要用户决策的接口或范围变更。
