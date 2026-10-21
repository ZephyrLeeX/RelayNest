# P03-08-F01 修复规划独立审计

```yaml
task_id: P03-08-F01-PLAN-A01
subject_id: P03-08-F01-PLAN
role: auditor
reviewed_base_sha: 063c270eb0eef463526f9718691fe5ff69f147f2
reviewed_implementation_sha: b98485339996434193209e930dc6c3115decda33
verdict: PASS
findings: []
report: docs/tasks/reports/P03-08-F01-plan-audit-01.md
```

日期：2026-10-22。工作树 `/tmp/relaynest-run2/storage`，分支 `codex/storage-run2`。本轮仅审计另一规划 agent 提交的修复卡及规划报告，不审计尚未产生的修复实现；PASS 不关闭 `P03-08-A01-F01`，也不将 P03-08 标 done。

## 检查结论

已核对上述精确提交间全部差异：仅新增 `docs/tasks/cards/P03-08-F01.md`（49 行）和 `docs/tasks/reports/P03-08-F01-plan.md`（31 行）。与 A01 发现、原卡约束及 WORKER_PROTOCOL 对照，未发现需修改的规划问题。

- 卡片明确仅修复 `P03-08-A01-F01`，复述了空库 6 调用者首次并发 Open 在 WAL 初始化立即 SQLITE_BUSY 的实际触发条件，没有将历史失败证据写成修复成功。
- 等待沿用既有 BusyTimeout，明确总预算、不重置重试预算、不在驱动等待之外叠加完整超时，接受更早 context deadline；要求仅重试经固定驱动查证的 WAL 阶段锁竞争，非锁错误和迁移拒绝不能吞掉。
- 验收要求保留原 6 调用者全部成功/单条迁移台账断言、新库同步并发，100 轮针对测试及25轮完整 storage；明确禁止串行化、降并发、吞错、跳过或弱化断言。另要求真实 SQLite 持锁、预算内释放、持续持锁总耗时、进入等待后的取消及更早 deadline，不能只用桩或现有迁移 BEGIN 竞争代替 WAL 证据。
- 取消与持锁场景要求同步信号/可观测条件、耗时与容差、25轮重复，并覆盖失败/取消后重开和 epoch 保持，足以指导修复者交付可复查证据；具体错误码、取消行为和局部实现仍由修复者核实。
- 修改路径限制为 storage.go/私有 WAL 辅助及对应测试和新实施报告；migrate.go、迁移 SQL、依赖只读，禁止改公共 API、Schema 或配置默认值。保留逐物理连接 PRAGMA、只读限制、迁移校验/原子性及失败不重建，范围保持最小。
- `dependencies: []`，通过 subject_id、发现编号和固定 SHA 关联原卡；不依赖未 done 的父卡。工作树实际 HEAD 允许包含后续规划文档链，但代码基线必须相同。修复后由不同于原实现者/修复者的 A02 审计最终新 SHA，PASS 后再安排集成，未产生父 done 依赖环。

## 实际文档验证

以下命令均在本工作树执行，开始时工作区干净、HEAD 为被审规划提交。未运行源码测试。

| 命令/操作 | 结果 |
| --- | --- |
| `git status --short`、`git rev-parse HEAD` | 无未提交路径；HEAD 精确为上述规划 SHA |
| `git diff --stat <基线> <规划SHA>` 及两个新增文件完整 diff 阅读 | 2 文件、80 行，仅规划文档 |
| `git diff --check <基线> <规划SHA>` | 退出 0，无空白诊断 |
| `git diff --exit-code <基线> <规划SHA> -- server` | 退出 0，服务端代码/迁移/依赖完全未变 |
| Perl（File::Basename、File::Spec）解析两个文档 Markdown 相对链接、逐一 `-e` 检查目标 | 两文件均输出 relative links OK，退出 0 |

`<基线>`、`<规划SHA>` 均为报告头完整 SHA；本轮没有修改旧报告、任务卡或 registry，只新增本审计报告。提交前检查本报告 diff、暂存范围及空白；最终报告提交 SHA 在提交后交接。

## 限制与交接

本结论只证明修复规划最小、完整且具备可核验验收要求，不证明锁竞争、总预算或运行中取消已经修复。原发现保持开放；下一步由调度登记并派发修复者实现，之后以最终实现 SHA 进行 P03-08-A02 独立复审。未重跑源码测试、开展额外方案研究、派生 agent、推送、部署或发布。
