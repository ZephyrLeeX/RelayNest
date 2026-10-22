# P04-01-PLAN01 路径边界补充报告

角色：规划；日期：2026-10-23。结果：已补充任务卡，待独立审计和精确集成；本报告不表示 P04-01 已实现或验收通过。

基线：`a8503f6cb9669224cfa64ed7ec9d598e851217c6`；工作树 `/home/lijx/.codex/worktrees/auth-init/RelayNest`，分支 `codex/auth-init`。开始时工作树干净。实现 SHA 在提交后交接，不预写自身 SHA。

## 变更及依据

仅修改 [P04-01 任务卡](../cards/P04-01.md) 和新增本报告。保留原三个实现范围、原验收及 init/reset-password、摘要、代次职责。

- 将 `server/go.mod`、`server/go.sum` 限定纳入已设计 Argon2id 和必要安全终端输入依赖；不选版本、不新增密码参数或产品/架构规则。
- 将 `server/internal/storage/storage_test.go` 限定纳入迁移夹具维护。已有 `TestMigrationUpgradeFailureAndLock` 将真实库视作版本 1，并构造测试 `0002/0003`；真实管理员迁移加入后须维护夹具，保留升级、失败回滚、锁及旧版本拒绝语义。
- 明确实际 CLI 为 `server/cmd/relaynest/`，迁移目录无需映射搬迁；候选 `0002_admin.sql` 由调度串行独占分配，保留 0001。复用 `storage.Open/Read/Write`，不允许修改 storage 实现。
- 补充共享文件协调、竞争 init、密码不进参数/日志、reset 摘要与代次原子性及受影响 storage 回归说明；明确会话/WS 接线留给后续任务。
- 字段、CLI 数据库路径和代次类型留实施时按最终契约核查；未确定的重要语义及进一步超范围改动交回调度。

依据：先前只读准备定位上述源码；本轮定向比较 `46dc206bb061db3c32f29bac1589dc644b62d53e..HEAD` 的 AGENTS、CONTRIBUTING、WORKER_PROTOCOL、CLI、storage 实现/测试、迁移与依赖文件无差异。按调度提供的 G-API 通过状态承接本规划，未修改或重新审计 API/registry。

## 验证与交接

- `git diff --check`：通过，无输出。使用 shell、`rg`、`sed` 逐项验证两文档相对链接存在、原卡全部验收行保留、已跟踪修改与新增文件恰为授权两路径：通过。
- 初次 Python 校验入口因环境无 `python3` 未执行，已用上述 shell 检查完成等价验证；不将失败入口记为通过。
- 文档规划变更不执行 Go/Web 测试；本轮没有源码改动，P04-01 功能验收全部留待开发、独审和集成，不记为通过。
- 未改源码、registry、其他任务卡或旧报告，未推送。规划须由不同 agent 独审并精确集成后，另行派开发 agent 实现。

遗留：依赖具体版本、管理员字段/路径/代次类型的最终契约核查，以及 P04-01 全部功能实现与测试。
