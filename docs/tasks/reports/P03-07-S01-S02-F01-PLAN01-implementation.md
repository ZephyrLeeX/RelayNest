# P03-07-S01-S02-F01-PLAN01 规划交付

- 角色：规划；结果：已建立最小修复卡，待不同 agent 独立审计规划；未实现 API，未关闭 BS-A01-01。
- 精确基线：`532ea26e791b9774bcd6c6246769499a40823b26`；工作树 `/home/lijx/.codex/worktrees/api-bootstrap/RelayNest`，分支 `codex/api-bootstrap`，开始工作区干净。
- 被审实现：`9cb95c41488fdf29895900b8288b7c15aacc5abe`；证据：[audit-01](P03-07-S01-S02-audit-01.md)，CHANGES_REQUIRED，仅编号发现 BS-A01-01（P2）。
- 变更仅 [修复卡](../cards/P03-07-S01-S02-F01.md) 与本报告；未改 API、原卡、旧报告或 registry。独立提交 SHA 在提交后的交接返回。

## 结论与范围判断

原 seed 已满足 position=H/cycle_complete=true，verify 无法证明实际 sync 发生。审计仅复现合成声明接受200，不是 HTTP/客户端数据丢失实测。修复卡要求可验证的来源区分，并保留真正零变化追赶完成于 H 的合法性、已有 sync 新轮次捕获高水位和全部恢复门槛；不能只改为 >H 或取消 verify。

本次只将已确认恢复目标变为可执行修复验收，无新增业务/权限、架构或运行时实现。允许在 bootstrap 新增契约范围强化 Schema/声明/例子；签发来源具体字段由实现者合理决定，但不得用不受验证的客户端标志代替证明。若实现需要改前卡已审 sync/read 契约，必须交影响说明、调度确认范围及 S01-S01 重审，本规划未授权该扩展。

编号映射：BS-A01-01 → F01-01初始seed拒绝 / F01-02实际零变化完成接受；原恢复要求 → F01-03期间新增事件追赶 / F01-04失败边界；原冻结与验证 → F01-05全量非回归 / F01-06破坏探针。所有项目均只声明将来修复验收，尚未实测通过。

## registry 与派发建议（仅建议，未写台账）

1. 调度登记本 PLAN01 为规划交付待独立审计，审计对象固定本次最终真实提交 SHA；只有审计通过后登记/派发 F01，不能把文档存在当开发授权。
2. 附加修复条目 `P03-07-S01-S02-F01`：kind=fix、parent_id/subject_id=`P03-07-S01-S02`，finding_ids=[BS-A01-01]、card 指本新卡。状态按现有统一枚举先待派发，规划审计PASS/登记和独占工作树齐备后才进入实施。不要把尚未done的父 S02 写成强依赖造成死锁；审计发现及精确SHA作为触发证据。
3. 父 S02 保持 review/CHANGES_REQUIRED；A01审计报告交付可单独done，发现仍未关闭。修复提交后登记 `P03-07-S01-S02-A02`（kind=audit、subject_id=父S02、精确新修复SHA、范围含原实现+修复），由不同agent复审，不改写 A01 历史证据。PASS后再登记精确集成，S01-S03在原卡集成前不启动。
4. 开发派发基线采用包含 `532ea26`、本规划审计/登记后实际最终 SHA，登记 owner/路径/分支/环境并检查 API 四文件及两锁未漂移；不在本报告编造未来 SHA。API共同文件全串行。

## 实际检查与未执行

已读取根规则、协议职责/预算/交接/修复局部、工程规范、原 bootstrap 卡、父卡冻结/检查、审计报告及最小相关 policy/verify/sync 游标声明、恢复设计段；未整读 API/示例包。卡内精简读取清单供下一角色按 keys 定位，初始目标≤8k/硬≤12k，不声称精确token计数。

本次只新增文档：`git diff --check` PASS；使用锁环境 Python 的局部脚本解析两文档全部 Markdown 相对链接，逐项确认存在，并用 `git merge-base --is-ancestor` 核实三条精确SHA均在 HEAD 祖先链，PASS。`git diff 532ea26e791b9774bcd6c6246769499a40823b26 -- server/api` 无差异；`git status --short` 仅两条授权新增路径。stage 后再检查 `git diff --cached --check` 和完整两文档差异。API与锁无变动，未运行 OpenAPI/validate/破坏探针或运行时测试；卡中命令和矩阵均为后续修复必跑要求，不能当本次通过证据。无推送、部署或额外agent。

## 下一角色摘要

规划卡只覆盖 BS-A01-01：bootstrap seed不能当真实sync完成证明；必须接受实际零变化完成于H，期间新增11须从H开启新轮次追赶。冻结25操作/73 Schema/446例及旧sync/read，当前26/77/520允许显式强化bootstrap。开发前先独立审计本规划、调度登记与固定最终基线，独占共同API。修复后以新SHA交A02复验，开发不得自行销单或放行集成/G-API。若需改旧sync游标契约先报影响并确认/重审。只读keys和必跑矩阵见卡。
