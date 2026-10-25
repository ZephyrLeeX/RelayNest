# P05-01-S02 消息与最小待办 Schema 实施

- 角色 developer；开发基线 `0355406b7ddd90492dbd9b0ba60d2da20c49a30c`；review base / 实例最终集成报告 `07779de822d1aa28ceeb324c1225499fcf20c9c4`，实例实际组合 `eea5094237fdc24fd967948795a3cdf1a0d26888`。
- 工作树 `/home/lijx/Projects/RelayNest-recovery-20261024/message-schema`，分支 `codex/message-schema-20261025`。开始时干净；唯一新 SQL `server/migrations/0006_messages.sql` 来自 ALLOC01 串行分配，旧 SQL/checksum、storage 生产代码、API、registry、delivery/push 目录与依赖均未改。实现 SHA 提交后交接，不在本报告预写自身 SHA。

## 实现与冻结依据

已读 AGENTS、CONTRIBUTING、WORKER_PROTOCOL、S02/S03 卡及 PLAN02 实施/A/I、S01 共同 changes 和实例最终 I；定点核对 TECHNICAL_DESIGN §1/2/4/5、ARCHITECTURE §1、DEVELOPMENT_PLAN P5 以及 Message/MessageDeliveryEvent/Sequence/Version。合同现行 40 operations / 118 schemas / 2455 cases 仅沿用冻结证据，本卡未重跑 API 合同，不代表 runtime 通过。

- `messages` 持久化 UUID v4、Bot FK、正整数 bot_seq/revision、sender、plain/markdown 当前正文与按钮数组 JSON。唯一 `(bot_id,bot_seq)`，所有序号/版本使用 SQLite integer 检查并覆盖 MaxInt64 和溢出。依 §2.1，模型只供持久化，BotSeq 为 int64，不能直接充当 HTTP DTO；对外序号须规范十进制字符串，版本仍为整数，created_change_seq 使用既有 changes.Sequence。
- 依 §5.1，当前引用 FK 为 SET NULL，原引用 ID 不可变；触发器拒绝跨 Bot 当前引用。源消息删除不保留正文，原 ID 继续用于“引用已过期”展示。创建水位可在创建事务内暂为 NULL，非空后禁止改写；不 FK 到会被保留期裁剪的 changes。Schema 不独立证明已创建事实：**S03 必须在同一 Store.Write 中 Append 并回填真实创建事件序号，任何失败回滚，未回填不得提交或发布消息**。
- 依 §4/5.1 与 MessageDeliveryEvent，`deliveries` 保存稳定唯一 event_id、Bot、不可变 JSON 快照及排队序号、pending/state_version、attempt/retry_generation 和到期时间。快照检查 JSON 对象、message.created、administrator sender 与 Bot 归属，阻止关键身份/快照更新；完整 Message 内容/按钮和时间验证由 S03/后续正式入口负责。SQL 默认 pending/1/0/0；retry_generation 初始 0 与现行 DeliveryMetadata 描述一致。仅支持 pending；P06 扩展领取/租约/状态迁移需串行新增迁移。内部 next_attempt_at 是排队扫描到期时间，不是管理 DTO；未来管理投影仍须遵守 pending 的 next_attempt_at=null 契约。
- 依 §5.1/9.1，`push_jobs` 仅有 id/session/device/全局目标水位/pending/attempt/到期时间；没有正文、附件 URL、业务凭据或假想注册表。新增 sessions(id,device_id) 唯一索引及复合 FK，拒绝会话绑错设备；删除会话级联清理。依 §4.3 Bot 永久删除边界，Bot 删除级联消息/应用投递；全局同步提示不引用 Bot，允许保留用于同步删除事件，不能因此保护消息。Android 会话有效期、撤销、代次/授权与生产目标枚举由 S03/P07 在真实事务内验证，FK 不代替授权。
- 必要索引覆盖消息时间/创建水位/引用、Bot pending delivery 扫描及 push/session 扫描；无正文或凭据索引。迁移只执行 DDL，不初始化/修复/旋转实例 ID 或 epoch；缺失/损坏身份仍由原存储校验失败关闭，升级 DDL/台账完整回滚。

## 实际检查

环境：Linux amd64；mise Go `1.27.3`，全部 Go 命令设置 `GOTOOLCHAIN=local`，使用 `/home/lijx/.local/share/mise/installs/go/1.27.3/bin/go`；真实 SQLite `3.53.4`（storage TestPersistenceAndSequence 实测）。Go 命令在 server 执行。

- 首次四包测试：messages/changes 通过；bots 因新树缺 ignored webui/dist 的 embed 失败；storage 明确报实例两处旧夹具断号。随后从主树复制现有 ignored `server/internal/webui/dist` 作为构建输入，不创建伪页面、不提交构建产物。
- 本机 PATH 没有 python3，首次编辑命令未执行；随后使用已安装 `/home/lijx/.local/share/mise/installs/python/3.13.15/bin/python` 完成编辑。此失败没有改变代码或产生通过结论。
- 新建真实文件库的 messages schema 测试覆盖合法边界、非法 sender/revision/序号、重复 bot_seq/event_id、错误 Bot/session-device 绑定、不可变快照/原引用/创建水位、重开后的 FK/索引、源消息自然删除以及 Bot/session 删除边界。
- `go test -count=1 -run 'TestMessagesUpgradeAndFailureBoundaries|TestPersistenceAndSequence' -v ./internal/storage`：PASS；真实 v5 实例库升级并重开两次，既有 system_state/identity/admin/session/Bot 和前五条完整迁移台账逐值不变；0006 SQL 中途冲突、缺失/损坏实例身份均无半 Schema/成功台账，不修复身份，Open 同样拒绝；v5 二进制拒绝 v6。
- SCOPE01 最小夹具补充经独审 PASS 并精确集成：源规划 `3a601c539399c750bdf91c616acdbf820e64f434`，独审 `7e396edeb26314ea9e7e4188e6de92522eb60217`，main 组合 `121181eaec2f8999d491a78a5372fef99e7dc47a`，I 报告提交 `fad32a126a4c0b95dde20c5cdc7b358f8a0dd188`，报告路径 `docs/tasks/reports/P05-01-S02-SCOPE01-I01.md`。调度正式通知后仅改 instance_test.go 两处夹具：实例迁移前旧库按文件编号过滤 `instanceMigration` 及之后；人工提交失败迁移以 `len(source)+1` 取下一号（当前 0007），保留全部真实迁移和原断言。此树仍保持原开发基线，不额外 cherry-pick 文档。
- 最终 `go test -count=1 ./internal/messages ./internal/storage ./internal/bots ./internal/changes`：PASS，分别 0.015s / 0.671s / 0.380s / 0.052s。
- `go test -count=1 ./internal/storage -run 'TestInstance(PersistenceUpgradeAndMove|MigrationCommitAndLockFailures)$'`：PASS，0.033s；真实含 0006 的同一源码树，没有删掉消息迁移来模拟通过。原 checksum/name/gap/future 拒绝、真实延迟外键提交失败零残留等回归均在最终四包测试内通过。
- `go vet ./internal/messages ./internal/storage ./internal/bots ./internal/changes`：PASS，退出 0。
- mise `gofmt -l` 检查四份修改 Go 文件、`git diff --check` 与旧 0001–0005 SQL / migrate.go 精确 diff：PASS，无输出；提交前显式核对全部当前任务路径。未执行 race、全仓构建、HTTP/API/真机测试，本卡未修改并发业务实现。

## S03 交接及未完成门槛

`Message` 持久字段与列同义，ButtonsJSON 为 JSON 数组；`CreatedChangeSeq *changes.Sequence` 的 nil 仅为创建事务临时态。`PendingDelivery{DeliveryID,EventID,BotID,PayloadJSON,EnqueueSeq,NextAttemptAt}`；快照必须是完整冻结 MessageDeliveryEvent，不能把当前 Message 模型直接 JSON 序列化替代合同快照。`PendingPush{ID,SessionID,DeviceID,TargetChangeSeq,NextAttemptAt}`；全局水位复用真实 Append 结果。

S03 必须证明授权/计数/消息/回填/真实 delivery 与 push 目标/changes 共用一次短事务，覆盖 SQL/Append/提交失败、独立 Store 并发、重开单调与会话失效；本卡仅结构与迁移，不证明原子业务创建。完整幂等、附件/编辑/引用输入校验、P06 scheduler、P07 provider/注册、HTTP/G1、手机/NAS 组合仍未验收。另角色独审 PASS 和精确集成后才可由调度置 done；无 push/部署/额外 agent。
