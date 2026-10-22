# P03-07-S01-S02-F01-PLAN01-I01 修复规划分支集成

日期：2026-10-22；角色：integrator；结果：implemented（修复规划文档精确组合确认）。BS-A01-01 仍未关闭，原 bootstrap 仍为 CHANGES_REQUIRED。

## 目标与精确链

- 本次目标仅 `/home/lijx/.codex/worktrees/api-bootstrap/RelayNest`，分支 `codex/api-bootstrap`，不是 main。
- 分支集成前 SHA：`8bff2ea39183cb3bb5d29f0b61691bbfbbe3785b`，起始 HEAD/分支精确匹配且工作树干净。
- 规划基线（原负面审计）：`532ea26e791b9774bcd6c6246769499a40823b26`；规划实现：`7922f0f6704a6b52df7f2b290b4595cf6d50fc68`；独立规划 PASS 审计：`8bff2ea39183cb3bb5d29f0b61691bbfbbe3785b`。
- Git 父链精确为上述基线→规划→规划审计，全部已在目标分支。按 WORKER_PROTOCOL §7 不重复 cherry-pick，仅确认准确组合后新增本报告。
- 本报告单独本地提交，提交后交接分支最终 SHA，不预写自身 SHA；规划 PASS 不替代 F01 实施和原卡 A02。

## 实际检查

Linux/Git；使用 `/tmp/relaynest-p0305-f01-locked/bin/python` 执行只读脚本：

- 精确 HEAD、分支、干净状态及两次父提交关系断言通过。
- 相对 `532ea26` 恰新增修复卡、规划交付、独立规划审计三文档；工作区与固定提交 blob 一致。
- 三文档 Markdown 本地文件链接6处均存在；不验证外链和锚点。
- API四文件及两 requirements 锁与 `532ea26` 逐字节相同，没有实施修复或改变旧 sync/read。
- `git diff --check 532ea26e791b9774bcd6c6246769499a40823b26 HEAD`：PASS；本报告提交前另做暂存差异及限定路径检查。

纯文档集成，不执行 OpenAPI/validate.py、破坏探针或业务/现场测试，均 NOT_RUN；修复及 A02 仍须各自执行必跑检查。未修改主工作树、registry、`.directory`，未推送、部署或派 agent。

## 交接门槛与释放

分支记录仅为 F01 提供精确验证基线：seed 必须不能冒充真实 sync 完成，真实零变化完成 position=H 仍合法；旧 sync/read 冻结，若需改其公开语义须报告影响并安排重审。实施者不得自行关闭 BS-A01-01。

F01 后由不同 agent 做原卡 A02，PASS 后才将原 bootstrap、负面审计、修复规划链、F01、A02 的准确组合集成 main。本轮未将未修复 API 放入 main，未放行 G-API 或后卡。提交后确认工作树干净并释放给 F01 developer。
