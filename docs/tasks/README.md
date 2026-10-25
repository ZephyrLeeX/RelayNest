# RelayNest 任务入口

本目录按产品模块组织任务；一个未完成目标只在所属模块保存一条状态，已完成目标统一归档。用户已授权持续推进 P3～P15；核心版优先，RelayShelf 仍以后置实质门槛为准。任务存在、已有提交或局部检查通过都不代表完成。

## 模块索引

| 模块 | 未完成任务 |
| --- | --- |
| Android 程序 | [ANDROID.md](ANDROID.md) |
| Go 后端／Bot Hub | [BACKEND.md](BACKEND.md) |
| Web 管理端 | [WEB.md](WEB.md) |
| MoviePilot 插件 | [MOVIEPILOT.md](MOVIEPILOT.md) |
| 跨模块工程、契约与 CI | [ENGINEERING.md](ENGINEERING.md) |
| 部署、联调与交付 | [DELIVERY.md](DELIVERY.md) |
| RelayShelf 后置扩展 | [RELAYSHELF.md](RELAYSHELF.md) |

## 使用方式

1. 从模块文档选一个目标，读取其范围、依赖、验收、环境限制和证据；跨模块依赖通过对应任务 ID／链接引用，不复制任务或建立另一套状态。
2. 按 [WORKFLOW.md](WORKFLOW.md) 派发有限 worker，经过本地提交、不同 agent 固定 SHA 审计、编号修复重审及精确集成验证后，由总调度更新状态。
3. 完成条目及真实证据移入 [COMPLETED.md](COMPLETED.md)；未完成父目标仍留在所属模块，只引用已完成子项。总调度唯一维护执行状态，其他角色回报精简证据。

[项目进度](../PROGRESS.md) 仅供按模块快速浏览，不替代任务条目。条目中的新 ID 用于后续调度，旧 ID／原 P 阶段用于追踪历史范围；不能因重新编号丢失依赖、未验收部分或既有限制。

## 设计与规则

- [产品需求](../PRD.md)、[架构](../ARCHITECTURE.md)、[技术设计](../TECHNICAL_DESIGN.md)：产品及技术依据。
- [开发规范](../CONTRIBUTING.md)：编码、检查及本地提交要求。
- [开发计划](../DEVELOPMENT_PLAN.md)：原阶段和验收背景；当前任务状态查本目录。
- [执行与交接规则](WORKFLOW.md)：授权、角色、上下文预算、并发、审计、集成及异常恢复。

## 历史追踪

旧 cards、reports、registry 和两份协议不再作为当前任务入口；本次重组已将当前有效范围、状态、证据与限制归入模块条目／COMPLETED，并用 WORKFLOW 合并执行规则。历史文件保留在 Git，无须继续维护双账。

如需复查迁移前原文，在仓库内执行下例；将路径换成所需的旧文件，只读查看，不覆盖当前文件：

```sh
git show 086fa51d79d7726545a9abbab8feebb25770034e:docs/tasks/cards/P05-02.md
git show 086fa51d79d7726545a9abbab8feebb25770034e:docs/tasks/registry.json
```

旧任务和报告也可在该基线的 `docs/tasks/` 下按原 ID 查找；历史报告引用的 SHA 与范围只证明当时结论，不能自动证明新实现或完整父目标已验收。

原始实施／独审来源提交可能不在 main 祖先链，恢复方式与本地 bundle 位置见 [关键提交索引](COMPLETED.md#关键提交索引)；main 集成历史与迁移前文件仍从上述基线检索。
