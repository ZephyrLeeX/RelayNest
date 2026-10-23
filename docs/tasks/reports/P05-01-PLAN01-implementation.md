# P05-01-PLAN01 共同基础先行边界规划报告

## 交付

- 角色：规划；结果：implemented（仅规划交付，待独立审计/集成，非实现验收）。
- 基线：`9ca05a0dfe7973a2fb8d077f3e9646085c2451f6`。
- 工作树：`/home/lijx/.codex/worktrees/changefeed-plan/RelayNest`；分支：`codex/changefeed-plan`。
- 变更：[父卡](../cards/P05-01.md)、[新子卡](../cards/P05-01-S01.md)、本报告。最终本地提交 SHA 在提交后的调度交接登记，不在提交内预写自身 SHA。
- 摘要：从消息事务仓储早拆可供 Bot 同事务消费的最小全局变更基础；仅授权新专用 changes 模块及测试，不写业务/HTTP/Schema。父卡原 P04-07/P03-08 前置、工作和全部原验收逐字保留；新增子卡汇总/复用验收。未改变产品、权限或信任边界，未冻结函数签名，未修改台账或其他卡。

## 定点证据与范围结论

| 现有证据 | 结论 |
| --- | --- |
| `TECHNICAL_DESIGN.md` §1、§2.1、§5.1、§6 | 全局变更序列、共享业务事务、持久 epoch 和当前投影是既定职责；日志基础不依赖消息 CRUD 或管理员 HTTP。 |
| `server/migrations/0001_initial.sql:2-23` | 现有 system_state 初始化 epoch；changes 六字段齐备，AUTOINCREMENT、revision>=1、可空 bot_id，无 Bot FK/正文/payload。无需迁移。 |
| `server/internal/storage/storage.go:79-124` | 写池单连接且 immediate；Write 已向调用方提供 *sql.Tx 并最终提交，错误回滚，可直接复用，无需扩大 storage 权限。 |
| `server/api/openapi.yaml` Sequence/Version/SyncEpoch/SyncChange 八分支 | 全局序号线缆是 int64 范围十进制字符串；公共版本为整数。子卡列明冻结 type/投影键，日志元数据不冒充完整同步响应。 |
| `server/internal/storage/storage_test.go:200,310-318` | 当前升级夹具读 0003、构造人工 0004/未来版本拒绝。本规划不新增 Schema，不必维护夹具；将来若确需迁移，先补最小权限并按当时 main 串行分号，0004 未预占。 |

子卡包含共享调用方回滚、独立连接并发、重开 epoch 持久化、删除后序号单调、JSON 精度与 SQLite 最大值失败原子性验收。消息/路由序列、业务版本/CAS、同步分页、鉴权、清理/floor 和墓碑载荷策略不进入基础。未来消费者等待同一真实接口独审集成，禁止私有旁路或先漏事件再补历史。

## 调度建议与无环关系

- 新子卡 `kind=implementation`、`parent_id=P05-01`，强依赖 `P05-01-PLAN01`、`P03-07`、`P03-08` 的 done；门槛 G-GIT/G-API；规划审计集成前保持 `todo/planning_pending`，不冻结/ready。
- 父卡原强依赖完整保留，收口增加子卡 done 与实际复用证明；子卡不依赖父卡、P04-07/P04-06 或 Bot 子卡。Bot 后续强依赖本基础独审集成，方向单向。
- A/I 使用 subject_id 与精确 subject SHA，触发分别为 review、review+审计 PASS，不能以 subject done 作前置。规划同样须独立审计和集成后再释放实现。
- 真实 Go 签名、错误、集成 SHA 和调用方事务示例留待实现审计结果交接；本规划未发明冻结接口。schema/公共契约若出现必要变化，先拿具体证据回调度，不在本卡扩大架构。

## 检查与限制

- 已读 AGENTS、CONTRIBUTING、WORKER_PROTOCOL 必要段及上表定点资料，核对真实 SQL/接口，未依赖摘要假定字段。
- 文档检查：相对链接存在；父卡原前置、工作与原全部勾选验收保留；新子卡无父子反向强依赖；只改授权三文档；`git diff --check` 通过。执行结果由本地检查输出留证，提交前再次核对。
- 业务构建/测试：not_run，本次只修改规划文档，无业务代码或 Schema 变更；新卡列出的 SQLite/Go 检查是未来实现要求，不记为已通过。
- 遗留：无范围内未决设计项；实现可派发性等待本规划独审集成、调度登记及既有门槛。未 push，未自行派发 agent。
