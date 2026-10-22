# P03-07-S02-S01-F01-PLAN01-I01 修复规划分支组合确认

日期：2026-10-22；角色：integrator；结果：implemented（修复规划文档集成）。DS-A01-01 / P1 未关闭，原设备契约仍待修复及 A02。

## 精确目标与组合

- 目标仅 `/home/lijx/.codex/worktrees/api-device-session/RelayNest`，分支 `codex/api-device-session`，不是 main。
- 分支集成前 SHA：`8f203ee84212a5003e47b1fdac429262c7a1ddc5`，接手 HEAD/分支匹配、工作区干净。
- 负面审计基线 `de2a6f915e20354e16738f3d17736d50f16894c8` → 规划 `7938f479a2ed05c6c27e2bfb631b3ec44a288f9d` → 独立规划 PASS 审计 `8f203ee84212a5003e47b1fdac429262c7a1ddc5`，直接父链逐项核实。
- 准确组合已在目标分支，依 WORKER_PROTOCOL §7 不重复 cherry-pick。仅新增本报告并独立本地提交，最终分支 SHA 提交后交接，不预写自身 SHA。

## 实际检查

Linux/Git，使用 `/tmp/relaynest-p0305-f01-locked/bin/python` 做只读断言：

- 精确 HEAD/分支及上述父链通过；相对负面审计基线恰为修复卡、规划交付、规划独审三文档，与固定提交 blob 相同。
- 三文档本地 Markdown 文件链接9处均存在；未验证外链或锚点。
- API四文件及两 requirements 锁与 `de2a6f915e20354e16738f3d17736d50f16894c8` 逐字节相同，没有实施修复。
- `git diff --check <负面审计基线> HEAD`：PASS；提交本报告前另检查暂存差异及限定范围。

纯文档确认，不运行 OpenAPI/validate.py、破坏探针或业务测试，均 NOT_RUN；F01 和 A02 仍须执行各自必跑检查。未修改主树、registry、`.directory`，未 push、部署或派额外 agent。

## 交接与释放

[规划独审](P03-07-S02-S01-F01-PLAN01-audit-01.md)仅通过修复规划，不关闭 DS-A01-01。F01 需在冻结旧契约基础上按 Session.id 精确定位，保留同设备多会话合法性及迟到撤销隔离；不得靠新增单活规则消除反例。具体验收见[修复卡](../cards/P03-07-S02-S01-F01.md)。

原设备实现和失败审计当前不入 main。F01 完成后不同 agent 以原卡 A02 复验，PASS 后才将原实现、负面审计、规划链、修复及 A02 准确组合集成 main。本报告不放行后卡或 G-API。提交后确认本树与 index 干净并释放给 F01 developer。
