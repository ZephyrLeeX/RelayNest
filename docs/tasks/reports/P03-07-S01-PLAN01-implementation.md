# P03-07-S01-PLAN01 输入预算与串行派发规划

角色：planner；结果：implemented（仅规划，待不同agent独立审计/精确集成）。精确基线ccbf114f4b6c82e4fcb73420281096596a7e7817，开始HEAD一致、工作树干净。工作树/home/lijx/.codex/worktrees/api-delivery/RelayNest，分支codex/api-delivery-run3。提交后交接真实实现SHA，不在提交内预写。

## 依据与决策

只读根约定、CONTRIBUTING、WORKER_PROTOCOL局部、原S01及原PLAN01相关规划/拆分段、T2.4/T3/T6和架构§1/§5相关局部、API README/Schema/validator索引与针对性段落、P03-06实施/审计摘要。另只读已提交e1a0d78aaa68318a4063725f05e79272507fd993的S01-checkpoint：此前原始输出累计约13k，未实现、无技术失败，不能重复同大输入包重启。

先压缩输入而非仅换卡号：父卡列明确设计行段、Schema keys、validator局部、旧例筛选字段与最多条数，以及每次输出≤1800tokens、初始分配2k/2k/3k/1k（合计≤8k目标）。基于本树源码Schema定向序列化，通用五Schema仅1599字节，Message/Bot/DeliveryMetadata三块5000字节；四API文件全量253492字节，因而无需灌入全部文件和369例。以上字节可复核，不当作token精确换算。原完整S01还同时覆盖增量/已读、重建恢复、WS三个接口与状态验证边界，压缩初始输入仍不能消除工作职责体积，故按完整边界串行拆为三张，每张估算初始5～8k，实际超预算checkpoint，不假造模型上下文占用比例。

- S01-S01：sync固定H/epoch/floor、当前投影类型/版本、read max更新及共享变更同卡。避免把读写两面拆开损害状态不变量。
- S01-S02：bootstrap短快照、令牌分页、当前revision/删除/附件重查、追赶与generation原子切换完整恢复闭环。
- S01-S03：WS握手/frame及REST恢复衔接，逐项映射父S01全部原验收并联合声明非回归。

父S01原八条验收完整保留；每张同改四份共同API，按接口/事务拆而非按文件拆。原S02/S03仍只补各自设备/设置类型，不改高水位/恢复语义。S01整体未完成前不能派原S02；原S03仍等原S02。没有新增需求、架构、安全或外部接口决策；不放行G-API，不改变P1/P2用户放行，不以手机/OPPO未准备阻本域。

## 冻结与环境证据

使用/tmp/relaynest-p0305-f01-locked/bin/python解析本树YAML/JSON，实际23操作/61 Schema/369例。git show逐字节比较本树openapi.yaml/examples.json/validate.py/README.md/requirements.in/requirements.txt与ca56c5e3a4c98f3ba88b089b2059024a6e93b324：六项均相同。未改API，也未读其他域源码。调度后续main已前进至8a50e384732e3e882d609ae3b38f73d8e952c45d（调度提供，仅作背景），不得据此预写未来实现基线；实现必须由调度以本规划审计集成后的实际main最终SHA派发。

所有子卡明确允许路径、锁环境恢复、每卡OpenAPI/validate.py/diff检查、全基线旧例/操作/组件/锁非回归、权限/状态声明破坏验证、结构与运行时证据区分、独立提交/审计/集成和≤800字交接。两锁只读、所有共同API写入串行；未知外部环境标NOT_RUN。

## registry建议（本轮不写入）

调度按真实登记字段补owner/base_sha，不预填未来SHA；gates_required沿用G-GIT，不把G-API当生产前置。父S01保留作为汇总收口任务，保留P03-06与原PLAN01依赖，并以本规划集成及三子卡证据约束派发/收口，不要求父done才执行子卡。

| ID | 关系/种类 | 实施前置/触发 |
| --- | --- | --- |
| P03-07-S01-PLAN01 | planning，parent_id=P03-07-S01 | 本次规划review后独立审计固定SHA，PASS后精确集成 |
| P03-07-S01-S01 | implementation，parent_id=P03-07-S01 | P03-06、P03-07-PLAN01、P03-07-S01-PLAN01均已审计集成/登记 |
| P03-07-S01-S02 | implementation，parent_id=P03-07-S01 | S01-S01已审计精确集成 |
| P03-07-S01-S03 | implementation，parent_id=P03-07-S01 | S01-S02已审计精确集成 |

各卡后续派生-A01/-I01分别kind audit/integration、subject_id为被审/集成ID，按review+固定SHA/最终PASS触发，不以尚未done父卡为强依赖；新实施条目初始todo，SHA/证据null至真实产生。S01父卡三子卡全审计集成后按原验收收口；原S02依赖父S01、原S03依赖原S02不变。调度维护registry及父验收勾选，规划不代写。

## 实际验证与限制

已执行：YAML/JSON只读计数、六文件对ca56c5e逐字节比较、文档本地链接/子卡字段/父验收完整保留检查、git diff --check，均通过（具体命令结果随本轮交接）。首两次尝试用系统python/python3落盘返回command not found，未写文件；改用既有锁环境解释器后正常完成，无依赖升级。此为文档规划，无API实现，未运行OpenAPI/validate.py或运行时/真机测试，均NOT_RUN；已在每张实施卡列为必跑。只修改父S01、新增三子卡和本报告；未改registry/API/技术设计/旧报告/其它卡。未推送。
