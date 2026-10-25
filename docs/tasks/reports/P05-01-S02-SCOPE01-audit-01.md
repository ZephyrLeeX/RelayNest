# P05-01-S02-SCOPE01 独立规划审计

```yaml
task_id: P05-01-S02-SCOPE01-A01
subject_id: P05-01-S02-SCOPE01
role: auditor
reviewed_base_sha: 07779de822d1aa28ceeb324c1225499fcf20c9c4
reviewed_implementation_sha: 3a601c539399c750bdf91c616acdbf820e64f434
verdict: PASS
findings: []
report: docs/tasks/reports/P05-01-S02-SCOPE01-audit-01.md
```

日期：2026-10-25。工作树 `/home/lijx/Projects/RelayNest-recovery-20261024/message-schema-scope`，分支 `codex/message-schema-scope-20261025`，开始时 HEAD 为上述被审提交且工作区干净。审计者独立于规划作者；仅新增本报告，不修改代码、任务卡或 registry。

## 范围与结论

已读 AGENTS、CONTRIBUTING、WORKER_PROTOCOL，完整审查基线至固定 SHA 的三份文档差异：[S02](../cards/P05-01-S02.md)、[SCOPE01](../cards/P05-01-S02-SCOPE01.md)、[规划报告](P05-01-S02-SCOPE01.md)。定点核对 `server/internal/storage/instance_test.go` 的两个指定测试、`storage_test.go` 的 migrationFixture、`migrate.go` 的连续编号校验及 `server/migrations/embed.go`，未整仓扩查。

- S02 原文为字节一致前缀，原依赖、目标、验收、串行分配规则均保留。新增路径严格限两个测试的夹具准备，禁止改生产、旧 SQL、公共 fixture 语义、其他测试或弱化断言，没有新增需求或架构变化。
- 旧库夹具仅删除0005，新增真实0006后即留下断号；只保留实例迁移前的0001–0004是最小适配。后续 Open 仍须使用全部真实迁移，ledger/epoch、实例身份、幂等与移动重开断言完整保留。
- 提交失败夹具硬编码0006会与新迁移重号，尚未到达延迟外键提交失败。实际 embed 仅包含连续 SQL，migrationFixture 复制全部真实 SQL，故 `len(source)+1` 在当前契约下合理；仍保留全部真实迁移、原失败 SQL、锁失败、migration commit 错误及零残留等断言，fmt 已存在。
- 文档明确只规划未来0006场景，没有冒称该树已有新 Schema 或实际 SQLite 验证。新增路径必须经独审 PASS、精确集成和调度通知后才开放，未提前向实施者授权；父 S02 不因此通过。

## 实际检查与限制

| 检查 | 结果 |
| --- | --- |
| `git status --short`、`git rev-parse HEAD`、分支核对 | PASS，起始干净且固定 SHA/分支匹配 |
| 完整 diff、`git diff --check <base>..<implementation>` | PASS，仅三份规划文档、无空白诊断 |
| `git diff --exit-code <base>..<implementation> -- server` | PASS，源码/真实迁移未变 |
| Perl 核对精确三路径、S02 字节前缀、三文档相对链接 | PASS |
| Perl 文件名模型：实际0001–0005追加假想0006，分别复现断号/重号并校验两修正后连续 | PASS，仅编号模拟，非 Go/SQLite 执行 |

最初尝试的 Python 检查因环境无 python3 未执行；已用实际成功执行的 Perl 检查替代。NOT_RUN：Go 测试/vet/race、SQLite 运行时及完整 S02 验收。本次纯规划审计无需运行代码测试；PASS 仅覆盖范围澄清，不证明后续实现或数据库行为通过。S02 实施仍须在包含真实新迁移的同一源码树执行原卡检查及新增两测试定向命令，再独立审计/集成。

报告提交前核对 diff、空白和显式暂存范围；最终审计提交 SHA 在提交后交接，不预写自身 SHA。无编号发现、未派子 agent、未推送或部署。
