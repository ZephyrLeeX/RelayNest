# P08-04-D01-S01 实例身份存储实施

- 日期：2026-10-25；角色：开发；结果：implemented，等待不同 agent 独审及精确集成，不代表父卡或最终真机验收完成。
- 工作树：`/home/lijx/Projects/RelayNest-recovery-20261024/instance-storage`，分支 `codex/instance-storage-20261024`；实际开发基线 `aa290fcf775b62c5d131ef5389d60b9080d2d5d2`，独审比较基线 `7e18fd5d97e74203e8f9cc5f5598e9cc4d2af18b`（因此独审还覆盖 ALLOC01 报告）。最终本地提交 SHA 由提交后交接提供。
- 冻结依据：C01 实施 `09deddba6ace42261c227b57254f8ae9ebbd1a2a`、独审 `8d012c9941f771a64236e38c735d09d379936259`、集成 `f72a11c147269b4c81f0e319998292a3228a83f9`；TECHNICAL_DESIGN §5 的存储接口及错误语义。

## 实现与接口交接

唯一新迁移为 ALLOC01 核定的 `server/migrations/0005_instance_identity.sql`：独立单例表 `instance_identity`，主键限定 1，SQL CHECK 与 Go 读取均验证小写规范 UUIDv4。使用 `crypto/rand.Reader` 和 `io.ReadFull` 生成 16 字节，再设置 UUID version/variant；不复用 `sync_epoch`，不新增依赖。现有 `*.sql` embed 自动包含迁移，无需修改接线。

现有迁移 `BEGIN IMMEDIATE` 内先执行 DDL，再初始化身份，再写版本台账，最后统一提交；每次迁移也严格复核已完成的身份。旧 SQL 与 checksum 均未改。完成迁移后缺失、空、NULL、畸形、额外行或读错误都失败关闭；Init 看到完成台账不会重建缺失行。

冻结的三个包函数均不自行开始/提交事务：

- `InitInstanceIDTx(ctx context.Context, tx *sql.Tx) (string, error)`：仅首次迁移创建，存在则严格返回原值。
- `ReadInstanceIDTx(ctx context.Context, tx *sql.Tx) (string, error)`：严格读取事务快照；无法可靠读取统一可 `errors.Is(err, ErrInstanceUnavailable)`，保留内部 SQL/上下文错误。
- `RotateInstanceIDTx(ctx context.Context, tx *sql.Tx, expectedID string) (string, error)`：先严格读及核对，不符返回 `ErrInstanceMismatch`；随机或 SQL 错误返回空候选并保留错误。只更新身份，调用方负责在同一短写事务完成撤销/代次更新和提交，提交失败不得使用候选值。本卡不执行维护，也不实现维护命令。

`Open` 仅在迁移提交、读池建立和严格读取提交身份成功后返回 Store。Store 的启动身份只作对比；`Read`/`Write` 每次都在提供回调的同一事务快照里重新读身份，读错或变化返回 `ErrInstanceUnavailable`，不执行回调。离线 Rotate 成功提交后，原 Store 后续读写失败关闭；关闭并重开 Store 后采纳新提交值。`EngineVersion` 仍只是引擎诊断，不可当作认证或就绪探测。新增接口没有 HTTP 映射；公开探测 200/503、认证顺序及事务末端复核留给 S02，不能据此宣称运行时 HTTP 已完成。

## 验收证据

新增真实文件 SQLite 回归 `server/internal/storage/instance_test.go`：

| 边界 | 证据 |
| --- | --- |
| 新库/旧库初始化、重启及原库停机迁移保留 ID | `TestInstancePersistenceUpgradeAndMove`、`TestInstanceConcurrentProcesses`；旧库 0001–0004 升级后 epoch、旧版本 name/checksum/applied_at 字节值不变；关闭全部连接后移动原库再开 |
| 两个独立进程首次并发启动同 ID | `TestInstanceConcurrentProcesses` 同时 Start 两个真实测试二进制，由各自 Open 完成迁移并输出已提交 ID，父进程比较，不使用进程内锁代替 SQLite |
| 安全随机源失败 | `TestInstanceRandomFailureAtomic` 受控替换 crypto/rand.Reader 为失败 reader；首次 Open 返回 nil Store，真实 SQLite 中无残留 DDL/身份/台账；换 ID 随机失败保留原值 |
| 锁超时与提交失败 | `TestInstanceMigrationCommitAndLockFailures` 两个独立连接池持有/竞争真实写锁；追加仅测试迁移通过 deferred FK 制造真实 COMMIT 失败，确认 DDL/身份/ledger 全部回滚，再执行真实迁移成功 |
| 进程崩溃回滚 | `TestInstanceCrashRollback` 子进程开启 WAL、BEGIN IMMEDIATE，按真实迁移 SQL 在同一事务写入 DDL/身份/台账后直接 os.Exit，不运行 defer/Close；父进程确认全部表未提交并能重新 Open。此项验证进程中断恢复，不声称断电/磁盘硬件故障测试 |
| 升级完成后损坏严格拒绝、在线不以缓存放行 | `TestInstanceDamageFailsClosed` 覆盖缺失/空/畸形/缺表/重复/NULL：Store Read/Write 回调不执行，Init 不修复，Open 失败；`TestInstanceConstraintsAndReadError` 覆盖单例/格式 SQL 约束与取消上下文错误保留 |
| 事务复核与候选发布边界 | `TestInstanceRotateTransaction` 覆盖 expectedID 错配、候选回滚、真实 trigger 更新失败、提交后旧 Store 拒绝读写及重开使用新值 |
| 旧迁移/台账回归 | 既有 storage 测试继续运行；仅修改旧 Bot 升级夹具，明确移除 0004 之后的 0005，保持真实旧版本连续边界；未删除断言 |

## 实际检查

环境：Linux 本地临时文件库，Go `1.27.3`（`GOTOOLCHAIN=local`，`/home/lijx/.local/share/mise/installs/go/1.27.3/bin/go`），Node `22.23.2` / npm `10.9.8`；使用仓库固定依赖，不升级版本。Web 构建仅为 Go embed 提供被忽略的构建资源，未提交产物。

- `go fmt ./internal/storage`：通过。
- `go test ./internal/storage`、`go vet ./internal/storage`：通过。
- `go test -race -count=1 ./internal/storage`：通过。
- `go test -count=10 -run 'TestInstance(ConcurrentProcesses|CrashRollback|MigrationCommitAndLockFailures)$' ./internal/storage`：通过。
- Web `npm ci --no-audit --no-fund && npm run build`：通过。
- 服务端 `go test -race ./... && go vet ./...`：通过（10 个包；migrations 无测试文件）。
- `git diff --check`：通过；旧 0001–0004 SQL 的相对基线差异为空。

没有现网数据库、HTTP、Android 真机、离线维护操作或断电测试；这些未被冒充为已通过。状态台账与他卡未改；不 push，不自行派生 agent。迁移锁继续由调度保持本卡独占到最终集成。
