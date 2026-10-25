# P08-04-D01-M01-SCOPE01-A01 独立规划审计

- task_id：P08-04-D01-M01-SCOPE01-A01；subject_id：P08-04-D01-M01-SCOPE01；角色：audit。
- reviewed_base_sha：`a346e033da54f7f302fcf2819c90d68737a9d5a7`。
- reviewed_implementation_sha：`455429e9aeab71795502ac9385c7445229e03272`。
- 工作树：`/home/lijx/Projects/RelayNest-recovery-20261024/offline-storage-scope`；分支：`codex/offline-storage-scope-20261025`；审计开始时固定 HEAD 干净。
- verdict：**PASS**；findings：[]。仅表示有限规划范围通过，不表示 M01 已实现或运行时验收完成。

## 检查依据与结论

已读 AGENTS、CONTRIBUTING、WORKER_PROTOCOL、M01/C01 相关卡、C01/S01 集成摘要，并独立比对固定范围三份文档；核对 TECHNICAL_DESIGN:241–243、:311 与 storage.go、migrate.go、instance.go、main.go 及 administrator.go/迁移表名局部。

1. 原 M01 的全部字节是新版本前缀；新增内容仅授权内部打开辅助与有限文件预算，既有命令、参数、人工停全部进程并隔离入口、原子撤销及失败码继续适用。新增卡/报告不改 SQL、公开网络契约、既有三个 Tx API 或 Open。C01/S01 集成组合 SHA 均为基线祖先。
2. storage.go:64–67 无条件启 WAL/迁移；migrate.go:59 与 :100 起分别建台账和补执行 SQL。Store 私有字段及 Write 的启动身份复核证实现存库需要 storage 包内独立构造路径。候选 OpenOffline 使用 mode=rw，拒绝缺失/部分/未知/不匹配迁移台账及缺坏身份，校验全部嵌入迁移 name/checksum，明确禁止 migrate/Init/enableWAL/DDL，范围必要且可局部测试。实现仍须证明关闭清理和错误分类，不能把普通 WAL/SHM 行为误认为授权切换日志模式。
3. 原 Store.Write 保留提交边界；RotateInstanceIDTx 仅返回事务内候选 ID。规划保留同一写事务中的换 ID、auth_generation 推进和全部 sessions 撤销、耗尽与任一步失败回滚。当前 0001–0005 没有生产 push 映射表；未虚构表，保留未来注册/投递实例加代次门禁。未占用在途 0006。
4. 主服务 shared flock 与维护 exclusive flock 限定本地同 inode 协调，保持至排空/Store 关闭；不是旧版、admin 工具、其他机器或任意写者停机证明。人工停服前置继续有效，SQLite 锁不替代前置。此为已许可 main 生命周期范围，无需重新申请普通 helper；未引入在线换 ID/自动停服。
5. 五个主要文件上限、对应测试/最小操作文档及超预算拆卡保留。错误码 0/2/3/4/1 与冻结一致，SQL 原因不得被身份 sentinel 吞没；提交后关闭/输出错误不得声称回滚。原 auth/密码重置非回归、并发/缺库/失败矩阵保留，没有删减父卡验收。

## 实际检查与限制

- PASS：`git diff --check a346e033da54f7f302fcf2819c90d68737a9d5a7 455429e9aeab71795502ac9385c7445229e03272`。
- PASS：使用 `/home/lijx/.local/share/mise/installs/python/3.13.15/bin/python3` 执行独立断言，精确三文档范围、M01 原文字节前缀、16 处相对链接存在、C01/S01 组合祖先、固定 HEAD 干净均通过。系统 PATH 未提供 python3，历史临时契约环境也不存在；本次使用现存固定运行时，未装依赖。
- PASS：人工核对局部 DAG：C01/S01 → SCOPE01 review → A01 → I01 → 调度通知 M01；A/I 以 subject_id 关联、不要求父 done，M01 原依赖未改。registry 只读，后续登记/集成由调度处理。
- NOT_RUN：Go/race/SQLite/受控进程维护、缺库与损坏库拒绝、全事务回滚及真实 NAS/部署验收。本轮只有规划文档；这些测试属于 M01 实施，不能引用前置或本次文本检查声称运行时通过。

本审计只新增此报告，不修业务代码，不改卡/registry，不推送或部署；没有待用户决定的新问题。
