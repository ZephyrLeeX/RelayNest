# P04-02-S02-PLAN01 预算退出续作规划

## 事实与结论

本次为只读规划与新增文档，不是源码实施或审计。规划工作树为 `/home/lijx/.codex/worktrees/auth-http/RelayNest`，分支 `codex/auth-http`；观察 HEAD `b60f9ae09878bc6f3af06861ad19a90a73f088e1` 仅为源码祖先，不能称作完整实现或源码已审基线。

现状：tracked dirty 为 `server/cmd/relaynest/main.go`（33 行增、3 行删）、`server/internal/httpapi/router.go`（13 行增、1 行删）；未跟踪为 `server/internal/auth/http.go`、`http_test.go` 与原 `P04-02-S02-implementation.md`。未改变或暂存这些文件。原开发 checkpoint 记载 Web ci/build 与普通 Go 三包测试 PASS，本规划未重跑，不能将历史结果解释为 race/vet/契约/完整验收通过。

索引和局部核查确认：handler 已有通道筛选与统一 JSON 入口；生命周期测试通过真实 TLS HTTP/SQLite，并调用新增 router 注入参数；可变时钟尚未同步；现有 cmd 测试仅见 graceful shutdown，router 测试仅见 health/static。原 checkpoint 列出的 HTTP 重置竞争、通道/来源反例、启动关闭、race/vet/契约和父验收映射仍须作为待验证范围，不能仅凭源码推断填 PASS。

一张续作会同时承担 HTTP 安全/生命周期和进程配置/资源关闭，涉及至少六个源码/测试文件及报告，仍易重现预算退出。因此拆成两张串行卡；router 与 handler 同批，以免首卡测试依赖未提交 router。拆分仅改变工程交付边界，不改变需求、安全模型、API 或原父验收。

## 待调度登记

| ID | 强依赖 | 唯一允许写路径 |
| --- | --- | --- |
| P04-02-S02-PLAN01 | 原 S02 前置沿用；本规划需独审/精确集成 | 本报告、两张新增子卡 |
| P04-02-S02-S01 | PLAN01 已独审集成；继承 P04-01、P04-02-PLAN01、P04-02-S01、P04-03-S01 已审集成门槛 | auth/http.go、auth/http_test.go、httpapi/router.go、本卡新实施报告 |
| P04-02-S02-S02 | PLAN01 与 S02-S01 已独审集成；继承原 S02 全前置 | cmd/relaynest/main.go、main_test.go、httpapi/router_test.go、本卡新实施报告 |

源码前缀均为 `server/`；报告完整路径为 `docs/tasks/reports/<子卡ID>-implementation.md`。精确路径以两张卡为准。原 S02 保留完整目标与全部验收，待两个子卡独审/集成后仍须组合验收；不修改原卡或 registry，由调度登记依赖与派发基线。

## 检查包与收口责任

S02-S01 负责 HTTP/SQLite 竞争、生命周期、安全反例和 auth/httpapi race/vet；S02-S02 负责配置/启动关闭、router 回归、三包 race/vet、现有契约检查与完整父验收矩阵。每卡约三主要代码/测试文件加独立报告，输入目标 6–8k、硬退出 12k。不得改已审 session/origin/管理员、SQL/API/依赖；底层缺陷交调度修复。缺工具应先定位锁定环境，本次仅发现默认 PATH 无 python3，未判断契约检查不可运行。

父 P04-02 的错误统一映射至首卡；期限/撤销/摘要与重置失效由既有 S01 + 首卡真实 HTTP 组合共同覆盖；检查与精确交付由次卡汇总。原 S02 的启动/来源接线、三条契约、运行回归与所有负例分别在两卡列明，不因拆分省略。独审、精确集成及外部真实环境门槛继续存在；ready 探针未齐保持 503，真实 NAS/手机/MP/WS/推送/正式 AC 不宣称完成。

本规划仅新增三份 Markdown，验证为路径/引用、边界与 diff 检查；不运行源码检查、不改原 checkpoint、不提交原 dirty。提交后通过交接提供精确规划 SHA，独立文档审计和精确集成后才派源码工作。未发现需要新增安全/架构/外部接口决策的事项。
