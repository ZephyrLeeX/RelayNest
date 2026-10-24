# P08-04-D01-CONTRACT-PLAN01

planner；基线 `00d9f79a4a009bd93505eaa6c9bdbce4eefbd687`，分支 `codex/instance-contract-plan`。仅拆卡；用户已批准[提案](P08-04-D01-PLAN01.md)的G1窗口、生命周期、具体契约/兼容成本，不重问。

registry建议登记8张implementation卡，pending，owner/base派发填；每卡另人A01/PASS/I01后解锁下一卡，A/I用subject_id，避免父done环。

DAG：规划A/I→[C01](../cards/P08-04-D01-C01.md)契约→[S01](../cards/P08-04-D01-S01.md)存储→[M01](../cards/P08-04-D01-M01.md)维护→[B01](../cards/P08-04-D01-B01.md)服务→[N01](../cards/P08-04-D01-N01.md)网络→[K01](../cards/P08-04-D01-K01.md)密文→[R01](../cards/P08-04-D01-R01.md)协调→[V01](../cards/P08-04-D01-V01.md)组合→S04→父。下一卡直接执行C01。

共享入口：Bot P04-06-S02-F01当前A02/PASS/I→设备P04-05-S03 A/I→B01；M01等待main释放。Android S03已集成，N/K/R串行。V01/S04仍依赖P03-03及完整P04-05。

gate建议：旧G-API历史39ops/115schemas/1229例仅原范围。新增G-INSTANCE-CONTRACT等C01全量契约/Session兼容回归和精确冻结，runtime不并行；G-INSTANCE-RUNTIME等S01至R01；G-INSTANCE-COMPOSITION等V01及S04原全设备验收。

父全部目标/原验收保留，身份由8卡补齐，其余映射见父/S04追加节。Room/outbox/WS/未来push仍强依赖门禁；迁移号待storage集成串行分配。phone/NAS/OPPO/真机/签名未测保留NOT_RUN，不阻契约。

本地71条链接、DAG、原卡前缀保留及diff/允许范围检查通过；无源码/API/SQL/registry变化，未跑业务/新契约/真机，不声称新gate通过。本地SHA交接，另人独审；无push/部署。
