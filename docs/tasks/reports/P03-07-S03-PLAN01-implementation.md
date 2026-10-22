# P03-07-S03-PLAN01 预规划交付

## 基线、范围与结论

- 角色：规划；基线 `3cdb5e228b7d124890d45250728b260b2091a6db`。
- 工作树 `/home/lijx/.codex/worktrees/api-final-plan/RelayNest`，分支 `codex/api-final-plan`；开始时干净。
- 仅修改 [S03汇总父卡](../cards/P03-07-S03.md)，新增 [S01设置](../cards/P03-07-S03-S01.md)、[S02健康投影](../cards/P03-07-S03-S02.md)、[S03最终证据](../cards/P03-07-S03-S03.md) 及本报告。不写 API/技术设计/其它卡/旧报告/registry。
- 结论：原整卡输入约7～10k且混合三类职责，拆为三张串行单元。各卡目标≤8k、硬12k tokens；按keys与相关行段读取，超过预算依协议checkpoint，不整灌API/设计/报告。原父八项验收原文及顺序全部保留，并映射到子卡。

## 定向证据与边界

读取根 AGENTS、WORKER_PROTOCOL §1～5/§7局部和 CONTRIBUTING 相关条款；原S03全文，原P03-07验收与规划报告职责缺口矩阵/公开健康摘要。PRD §3.4（30天/保护队列）、§3.6（10MiB/50MiB可调）及§7基础监控；T3.1/3.2（约85～120行，settings在112行）、T6设置/清理关联、T8（约279～286行）；架构§1/§4.3/§5/§6；P06-08职责。仅查API相关key索引和现有health/router局部，不读取全份API。

设置卡负责GET/PUT admin/settings、CAS恢复、清理/附件保护与明确全局changes分支；未定产品允许值提供证据/选项，普通资源有界限制可合理说明。健康卡保持根路径公开纯文本与管理员JSON投影分离，管理路径名称由实施者冻结；源码现状在router.go:12～13、health.go的Live/Ready/respond可核对，不宣称运行时已测。最终卡建立全T3/T3.3/T6/T8矩阵与父验收证据，只补明确缺口，避免重做已有接口。

当前设备S02三子卡尚未实施、WS在另树实现，不能提前冻结未来字段。所有实施均等设备S02父完整审计集成，取最终main实际SHA重验；26操作/77 Schema/528例及更旧计数均是历史点，未来用全量实际基线。D1～D3/FCM仍归设备卡，P1/P2用户放行与手机/OPPO/NAS正式验收不变，先冻结接口后P4实现门槛不变。

## registry 建议（只读，不写入）

| ID | 类型/关系 | dependencies或触发 |
| --- | --- | --- |
| P03-07-S03-PLAN01 | planning，parent_id=P03-07-S03 | 沿调度本轮精确派发；规划通过不解除设备完整收口门槛 |
| P03-07-S03-S01 | implementation，parent_id=P03-07-S03 | P03-07-S02、P03-07-S03-PLAN01，均完整审计集成 |
| P03-07-S03-S02 | implementation，parent_id=P03-07-S03 | P03-07-S03-S01完整审计集成 |
| P03-07-S03-S03 | implementation，parent_id=P03-07-S03 | P03-07-S03-S02完整审计集成 |
| 上述各ID-A01 / -I01 | audit / integration，subject_id=对应ID | review+固定SHA / 最终审计PASS触发，不依赖subject done |

新实施条目初始todo、gates_required沿G-GIT，owner/基线/实现/集成SHA据实际证据填。现有S03父依赖不改，仅追加全部子卡集成后的汇总条件；子不依赖父done，G-API不作生产卡前置。G-API.source_tasks沿P03-04/05/06/07，由父证据包含子卡，不代调度登记或放行。

## 实际检查与交接

本轮使用 `/home/lijx/.local/share/mise/installs/python/3.13.15/bin/python3` 执行临时文档检查脚本（系统PATH无python/python3，未改环境）：

1. PASS：四张卡本地Markdown链接存在，三子卡目标/依赖/输入/冻结/范围/验收/锁校验/非回归/交接/registry建议字段齐全。
2. PASS：用 `git show <基线>:docs/tasks/cards/P03-07-S03.md` 提取原八个勾选项，与修改后逐字及顺序比较完全相同。
3. PASS：API六文件逐一与基线字节比较不变。
4. PASS：`git diff --check`、显式路径diff与status核对；最终提交前复跑并将本报告计入链接与允许路径检查。

纯文档规划无需业务测试；OpenAPI/业务/真机/外部接口验收 NOT_RUN，本轮不声称G-API或运行时通过。显式stage独立本地提交，实际实现SHA由提交后交接给出，不在报告预写自身SHA。交不同agent固定SHA独立审计；未push/部署，registry留调度处理。
