# P05-01-S01 全局变更事务共同基础实现

## 交付

- 基线：`85305dfd3e4b4e15e78a208b5d75e8fd331deea0`；分支 `codex/changes-foundation`。
- 工作树：`/home/lijx/.codex/worktrees/changes-foundation/RelayNest`。
- 仅新增 `server/internal/changes/changes.go`、`changes_test.go` 和本报告；无迁移、依赖、API、storage、Bot 或台账变更。
- 摘要：在调用方 SQLite 事务内追加六字段日志并取真实 LastInsertId；只读 0001 持久 epoch。独立 Sequence 严格输出 int64 范围十进制 JSON 字符串；八种冻结事件仅映射投影键，不持久化正文或组装 SyncChange。真实文件库覆盖回滚、独立 Store 并发、锁失败、重开/删除单调性及耗尽。必需检查全部通过；等待另一角色独审及集成，未标 done、未 push。实现 SHA 由本提交完成后的交接提供，避免自引用。

## 实际导出接口与错误

```go
// 包路径 relaynest/server/internal/changes
type Sequence int64
func ParseSequence(string) (Sequence, error)
func (Sequence) MarshalJSON() ([]byte, error)
func (*Sequence) UnmarshalJSON([]byte) error

type Type string
func (Type) ProjectionKeys() []string

type Metadata struct {
    Type Type
    EntityID string
    Revision int64
    BotID *string
    CreatedAt time.Time
}
func Append(context.Context, *sql.Tx, Metadata) (Sequence, error)
func Epoch(context.Context, *sql.Tx) (string, error)
```

- `Sequence` 接受 0～9223372036854775807；实际追加从正数开始。解析拒绝负数、溢出、空串、前导零、正号、小数、指数；JSON 入口拒绝 number/null。失败不修改解码目标，负值构造在 MarshalJSON 时失败。实体 Revision 是 int64 整数，允许超过 2^53−1，与全局序号分离。
- 事件常量：`MessageUpsert`、`MessageDeleted`、`BotUpsert`、`BotDeleted`、`ReadUpdated`、`DeliveryUpsert`、`DeviceNotificationsUpdated`、`SettingsUpdated`。字面值及键与冻结八分支相同。`ProjectionKeys` 每次返回独立切片，未知类型返回 nil；不包含 type/change_seq，不含实体投影值；settings 的 recovery_path 仍由后续投影固定为 `/admin/settings`。
- Metadata：Type 必须为八种之一，EntityID 非空、Revision≥1；BotID=nil 是 SQL NULL，非 nil 不得空串；CreatedAt 非零且年份可表示，存储统一 UTC RFC3339Nano 的 Z 结尾。ID、版本、时间均由业务调用方拥有，不做生成、授权或 CAS。
- `ErrInvalidSequence`、`ErrInvalidMetadata`、`ErrInvalidTransaction`（nil tx）、`ErrInvalidEpoch` 可用 errors.Is 判断。SQL/上下文/已结束事务/LastInsertId 错误用 `%w` 包装保留原错误；epoch 缺行保留 `sql.ErrNoRows`，格式损坏返回 ErrInvalidEpoch，不修复。
- Epoch 校验的是 0001 实际生成的 lowercase UUID v4；不对外改变 SyncEpoch 不透明字符串契约。不生成/更新 epoch，不新开、提交或回滚事务。

## Bot 同事务使用

业务调用方在既有 `Store.Write(ctx, func(tx *sql.Tx) error {...})` 回调中完成业务插入/版本更新后调用 `changes.Append(ctx, tx, metadata)`，必须返回其错误。回调内所得序号仅是未提交结果；只有外层 Store.Write 返回 nil 后，才可对外承诺、发送响应或广播。失败丢弃结果，由 Store.Write 回滚全部写入；调用方不得忽略 Append 错误而提交。`Epoch` 同样使用调用方当前读/写事务，供一致快照组合。后续 Bot/消息/P06 应复用本独审集成接口，不复制追加实现。

## 验收证据

| 卡项 | 真实测试及结果 |
| --- | --- |
| 业务/版本/事件原子性 | TestCallerTransaction：测试专用实体表插入、版本更新、追加在提交前其他读连接均不可见；提交后三者存在。追加后主动失败与 SQLite CHECK 拒绝 NUL 开头 ID 两种错误均回滚全部业务/版本/事件，epoch 不变。验证实际行序号、超 2^53 revision、NULL bot_id 及 UTC 时间。 |
| 初始空日志、全局单调 | TestReopenAndEpoch：初始无事件；混合消息/已读/Bot/设置事件与不同实体共享递增序列；每次删除日志并关闭重开，下一提交仍大于上一已提交序号。 |
| 多独立 Store 并发 | TestIndependentStoresConcurrent：同文件三个独立 Store 各 20 短事务，60 次全提交成功，distinct change_seq 和 distinct entity_id 均为 60；并非单 Store 写池。 |
| 锁/上下文失败 | TestLockFailure：第一 Store 保持 immediate 写事务，第二独立 Store 10ms busy timeout 返回错误且回调未进入；已取消上下文保留 context.Canceled，实体无半写，只有第一条事件提交。 |
| epoch 所有权与损坏 | TestReopenAndEpoch：首次合法 UUID v4，空库多次重开、事件删除后重开稳定；新独立文件 epoch 不同；36字符损坏值显式失败；删除状态后反复返回 sql.ErrNoRows 且不重建。TestCallerTransaction 证明失败事务后 epoch 稳定。 |
| 线缆和映射 | TestSequenceJSON 覆盖 0、1、2^53−1/2^53/2^53+1、MaxInt64 精确字符串和非法入口；TestFrozenTypes 核对八字面值/投影键及切片隔离，revision 仍为 JSON integer。无完整 SyncChange 输出接口。 |
| SQLite 耗尽 | TestSequenceExhaustion：测试专用库显式插入 MaxInt64−1，Append 成功提交 MaxInt64，下一次失败并回滚测试实体写入；旧两行/序号完整，epoch 不变。生产无 sqlite_sequence 修改。 |
| 参数与事务错误 | TestMetadataValidationAndErrors：未知类型、空 ID、非法版本/可空 Bot 参数/时间拒绝且不留行；nil tx、sql.ErrTxDone 保留；实际 SQLite 成功保存 MaxInt64 revision，未施加 2^53 上限。 |

不承诺序号连续、无空洞或未提交序号不复用；并发顺序和已提交单调性由 SQLite AUTOINCREMENT 与现有事务约束提供。

## 检查环境与执行结果

执行日期 2026-10-23，Go `go1.27.3 linux/amd64`，工具绝对路径 `/home/lijx/.local/share/mise/installs/go/1.27.3/bin/`；所有 Go 命令设置 `GOTOOLCHAIN=local`。`go list -m modernc.org/sqlite` 确认 `v1.58.0`。使用 testing.TempDir 的真实 SQLite 文件，不用 mock 或内存库。Go 检查 cwd 为本工作树 server，格式/Git 检查 cwd 为工作树根。

| 命令 | 结果 |
| --- | --- |
| `go test ./internal/changes ./internal/storage` | PASS：changes 0.035s，storage cached（首次执行 0.505s 通过）。 |
| `go test -race ./internal/changes` | PASS：1.258s。 |
| `go vet ./internal/changes ./internal/storage` | PASS，exit 0。 |
| `gofmt -l server/internal/changes/changes.go server/internal/changes/changes_test.go` | PASS，无输出。 |
| `git diff --cached --check` | PASS，无空白错误。 |
| `git diff --cached --stat` / `git status --short` | 只含本卡三个新增路径；提交前核对。 |

范围内无遗留功能或未执行的必需检查。未运行其他阶段、真实 NAS、HTTP 或 Bot 集成测试，本卡不实现这些职责；消费者仍等待精确实现 SHA 的独立审计 PASS 和集成。
