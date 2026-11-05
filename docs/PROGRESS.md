# RelayNest 项目进度

- 更新日期：2026-10-27（UTC+8）。
- 后端本轮恢复基线：`bde0193a5e1f561cb0b3d30a08bea691fcd499a8`；唯一权威目标为项目根 `/home/codex/projects/RelayNest` / `resume/backend-20261026`。推送 STORAGE 与 [STORAGE-BUSY-01](tasks/COMPLETED.md#g5-storage-busy-01) 已完成；[ADMIN 有限文本 HTTP](tasks/COMPLETED.md#g5-be02-admin) 完整 WIP 链在准确 `665edd785d8784ae0e3304d3d3bdfed2d474f872` 组合全服普通／race／vet／build PASS、A04 独立 PASS、I01 精确 ff 后完成，旧监听／夹具／SQLITE_BUSY／额度未完成日志全部保留。[BE-02 原四项有限父组合](tasks/COMPLETED.md#g5-be02-compose) 已独立 R01／A01／I01 完成，保留直接 HTTP 内部 COMMIT 观测 NOT_RUN 等方法限制。`main@01c74a7` ref 未变，历史克隆不再双目标推进，其他模块运行状态不在本轮核实范围，未部署或发布。
- 状态来源：[模块任务入口](tasks/README.md)中七个模块的条目是未完成任务唯一状态源；已完成结果见 [COMPLETED](tasks/COMPLETED.md)，执行与独审／精确集成规则见 [WORKFLOW](tasks/WORKFLOW.md)。本页只作模块摘要，证据仍须核对实际提交。
- **已完成**：限定范围已完成实现、独立审计和精确集成；用户批准的 PoC 验收例外单列说明，不等于整阶段或正式端到端通过。
- **开发中**：已有实现但父功能尚未闭环；**未开始**：尚未启动功能实现；**待验收**：实际环境验证未完成；**受阻**：缺少相关外部条件；**后置**：尚未满足启动门槛。
- 当前后端调度证据见 [BACKEND](tasks/BACKEND.md)：2026-10-26 AUTH、BE-02-BOT 与 [附件 STORAGE 准备](tasks/COMPLETED.md#g5-be05-storage) 已完成独审和本地精确集成；[HTTP01 叶](tasks/COMPLETED.md#g5-be01-http) 也已完成 A02 PASS 与本地精确集成；[BE-01 既定现有后端父范围](tasks/COMPLETED.md#g5-be01) 经 COMPOSE R01／A01／I01 收口；BE-02 限定父组合已另行闭环，BE-05-01 仍未完成，全局两实例门槛仍 pending；其他模块运行状态不在本轮核实范围。会话结束后不承诺后台运行；契约、任务卡或准备报告存在，不计作运行时功能完成。

## 1. Android 程序

本模块任务：[条目与验收](tasks/ANDROID.md)。

### 已完成

- **已完成｜原生工程基础**：最小原生工程和固定工具链已集成；跨端检查配置见工程模块。
- **已完成｜登录局部基础（P08-04-S01～S03）**：HTTPS 网络层与安装身份、加密会话存储、登录 UI 的本地组合分别审计集成。仅为局部基础，不代表完整登录父功能完成。
- **已完成｜FCM 最小 PoC（P2）**：一加 Ace 3 Pro、Android 16／ColorOS 16 上的 data-only 提示有实测证据；打开应用补偿和 WorkManager 周期调度／模拟落盘已验证。G-FCM 限定放行，非真实 Hub 最终同步或正式 AC-09。见[门槛结论](tasks/COMPLETED.md#a2)及[独立审计](tasks/COMPLETED.md#a2)。

### 未完成

- **开发中｜Hub 登录与安全会话父功能（P08-04）**：局部基础已完成；服务端实例绑定、新 G1 探测接入及会话升级、正式组合仍未闭环。
- **未开始｜新 G1 接入**：公开实例探测、请求绑定与会话升级的 Android 正式接入，依赖后端实例绑定闭环。
- **待验收｜真实安全存储与登录**：实际 Keystore、真实服务 TLS、签名安装和真机组合未完成。用户已决定真机运行留到最终验收，不重复索取 ADB。
- **未开始｜本地数据与同步**：Room 缓存、增量同步协调、bootstrap／历史缺口重建和 WS 前台生命周期。
- **未开始｜消息与附件**：本地发送 outbox、上传下载／预览、Bot 列表与消息浏览、发送回复及按钮交互、共享已读与双设备回归。
- **未开始｜管理功能**：Bot／凭据、全局配置、设备会话及失败队列管理。
- **未开始｜正式通知客户端（P9）**：通道注册与插槽、FCM 提示入口、正式 WorkManager、通知权限／偏好、提醒互斥／去重／已读取消及完整后台真机回归。Go 正式推送适配另见后端模块。
- **待验收｜后台边界补证**：Google 网络不可达时退化为打开即同步＋周期补偿；ColorOS 自启动未开、force-stop、HIGH 优先级边界保留。积压冲刷和 `onDeletedMessages` 不能当作可靠补偿保证；长窗睡眠、切网及未测项仍交正式回归。
- **受阻｜OPPO Push PoC（P02-05）**：开放平台账号、应用权限及审核条件尚未准备，G-OPPO pending。可选 OPPO 模式及已批准的通用标题／正文保留，不阻塞其他独立开发。

## 2. Go 后端（Bot Hub）

本模块任务：[条目与验收](tasks/BACKEND.md)。

Bot Hub 只负责消息传输、存储和路由，不执行业务命令或 MCP 工具。

Debian 恢复：推送 STORAGE、共享写准入修复及 ADMIN 有限文本 HTTP 均已独审／精确集成；[BE-02 原四项有限父组合](tasks/COMPLETED.md#g5-be02-compose) 也已由 R01 实际双入口普通／race、不同 A01 PASS 和 I01 等值核收完成。COMMIT companion／同 DDL 生产 HTTP 回滚的配对方法经 A01 判断充分，同次 HTTP 内部直接观测仍 NOT_RUN，不改写为已直接观测；HTTP jobs=0／未来范围仍保留。[BE-10-01-HTTP 有限五端点](tasks/COMPLETED.md#g5-be10-http) 已经 A01 两编号修复、完整 df 源检查、不同 A02 PASS及准确539新组合 I01全检查／精确集成完成；保留 harness 原目录冲突 FAIL与仅临时修正后的完整复验。[BE-10-01 限定服务器父组合](tasks/COMPLETED.md#g5-be10-compose) 已 R01／不同A01／准确74e新组合I01完成；平台发送／真机及跨端门槛仍未完成。历史失败与额度中断事实保留，未 push、部署、发布或创建完成标记。

### 已完成

- **已完成｜服务与存储基础**：Go 工程、健康入口、SQLite 连接及迁移框架。
- **已完成｜管理员认证基础**：唯一管理员初始化、密码摘要、会话存储、登录与过期、密码重置的认证代次基础。
- **已完成｜Web 安全基础**：同源、CSRF、可信代理校验、登录防护及敏感日志过滤。
- **已完成｜设备会话后端**：列表、分页、精确会话撤销、当前会话处理，以及真实 HTTP／SQLite 局部回归。
- **已完成｜Bot 基础管理与事务**：元数据、独立凭据授权、启停、密钥轮换和永久删除的基础事务；完整队列联动及正式 HTTP 收口另列未完成。
- **已完成｜消息事务仓储（P05-01）**：消息和序列、全局 changes、最小投递／推送待办同事务提交；失败回滚、并发单调和父卡原验收已收口。见[父卡集成报告](tasks/COMPLETED.md#g2)，父任务收口提交 `ed488e543d892f2353fbc95f72a2f0af98ba9a8d`。
- **已完成｜幂等创建存储基础（P05-02-S01）**：稳定身份作用域、同键重放原回执、异内容冲突、同事务创建、无正文墓碑及 Bot 删除清理。`0007_idempotency.sql` 已精确集成；Bot 有限文本 HTTP 已完成，Android 管理员有限文本入口已独审／精确集成，父组合仍未闭环。见[集成报告](tasks/COMPLETED.md#g3)，组合 `b7e59b64116fdd4c4f7b06009473341df38471e9`、报告收口 `e674801f3640508327e102bda37805b687c3108d`。
- **已完成｜实例身份存储**：独立实例 ID 原子初始化、严格读取和失败关闭；冻结契约见工程模块，完整运行时门槛尚未通过。
- **已完成｜离线实例维护工具**：换 ID、代次推进与全会话撤销、无迁移打开入口、本地进程互斥；真实六迁移库的推送待办清理／保留／失败回滚组合已通过。仅临时库验证，未操作现网；保留人工停服和入口隔离前置。见[集成报告](tasks/COMPLETED.md#g4)，提交 `71b6e5ea3fd08b86f7af6848068a0dd975150161`。
- **已完成｜Bot API 内部身份发现**：区分 NoMatch、Matched、Unavailable；匹配后拒绝不得回落管理员，故障失败关闭。真实附件 HTTP 分流、资源域及文件打开顺序尚未验收。见[集成报告](tasks/COMPLETED.md#g4)。
- **已完成｜认证入口与实例事务绑定（AUTH01）**：A02 固定修复 SHA 独审 PASS，A01-001／002 关闭；已精确集成目标 `work` 至 `1aa0aa9e0c390a46a7293cae56410e0baae8ce71`。仅 AUTH 子范围完成，完整 G1 仍 pending。见[完成证据](tasks/COMPLETED.md#g5-be01-auth)。
- **已完成｜Bot 正式创建 HTTP（BE-02-BOT）**：当前有限 plain/markdown 文本入口已独审并精确集成 `907c2a4dbf3a8774bcd438fb22b2381def7394a5`；保留并行 race 夹具 SQLITE_BUSY 首轮失败及串行复验 PASS，404／Webhook 限制见[完成证据](tasks/COMPLETED.md#g5-be02-bot)。
- **已完成｜管理员正式创建 HTTP（BE-02-ADMIN）**：完整 WIP／修复链在准确 `665edd7…` 组合取得 A04 PASS、全服普通／race／vet／build PASS、I01 精确合入；仅当前有限 plain／markdown 文本与持久化回执，父组合及未来附件／引用／按钮／跨端仍未完成。见[完成证据](tasks/COMPLETED.md#g5-be02-admin)。
- **已完成｜正式幂等创建既定有限父组合（P05-02／BE-02）**：生产 Bot＋管理员 HTTP／SQLite 的提交后回执、丢响应重试、同键异内容409、token轮换稳定身份经独立 R01／A01／I01 收口；保留 plain／markdown、持久化回执、HTTP jobs=0与 COMMIT 方法限制，不等于正式 reset HTTP／Webhook／附件／引用／按钮／ACK 已实现。见[完整完成证据](tasks/COMPLETED.md#g5-be02-compose)。
- **已完成｜附件流式持久化准备（BE-05-01-STORAGE）**：staging/ready、稳定身份去重及活动尝试 fencing 已独审并精确集成 `064807f9e5e126f48c9360d4d02a40fc209f790e`；仅存储准备，真实上传未接线。见[完成证据](tasks/COMPLETED.md#g5-be05-storage)。
- **已完成｜公开探测与真实路由 HTTP 子范围（HTTP01）**：A02 固定 `d8706747…` PASS，A01-001 关闭；实现／修复精确集成至 `cf2c8b49b4b5338e58e4b9d6e9698e5e648bdc31`。历史 HTTP/2 自测与并发超时 FAIL、串行 PASS 和未测限制见[完成证据](tasks/COMPLETED.md#g5-be01-http)；现有后端父范围另见 [BE-01](tasks/COMPLETED.md#g5-be01)。
- **已完成｜实例绑定现有后端父范围（BE-01）**：公开 probe＋八个现有 auth／device／admin-bot 路由经独立 COMPOSE R01／A01 与 I01 精确组合核实 PASS；保留既定 G1 瞬切窗口。见[父验收与限制](tasks/COMPLETED.md#g5-be01)。Android／真机／跨端、未来附件／WS／代理验收仍 NOT_RUN，全局 G-INSTANCE-RUNTIME／G-INSTANCE-COMPOSITION 仍 pending。

- **已完成｜历史／详情／同 Bot 引用只读事务仓储准备**：[HISTORY-STORAGE](tasks/COMPLETED.md#g5-be04-history-storage) 源必检和独立八组测试 PASS，I01 源码／输入等值复用检查后精确集成；未声称新跑全服普通或提供历史 HTTP。

- **已完成｜既有双入口同Bot文本回复引用**：[REFERENCE-CREATE](tasks/COMPLETED.md#g5-be04-reference-create)完整源链A01及新HTTP组合I01通过；保留配对COMMIT方法限制，不含Webhook引用／附件／按钮或父历史已读HTTP。
- **已完成｜服务器注册／偏好／条件解绑父范围**：[BE-10-01](tasks/COMPLETED.md#g5-be10-compose)R01／不同A01／精确I01通过，保留原overlay编译FAIL及修正；不含发送适配器／资格／真机／全局实例门禁。

### 未完成

- **未开始｜Bot 生命周期与权限收口**：重置／删除正式 HTTP、领取租约／等待请求／投递队列完整联动，以及身份权限整体矩阵（P04-08）。
- **已完成｜BE-04-01既定后端父组合**：[完整父档案](tasks/COMPLETED.md#g5-be04-compose)保留原目标／验收、所有子项完整源链及既有FAIL历史。准确e8产品＝R01准备ad6ce，真实TLS／SQLite组合覆盖历史分页／跨Bot引用拒绝、两个Android HTTP设备共享max不倒退、own与已读／未读application持久编辑不增未读、附件ready上传＋可信关系读投影、缺物理文件保引用及最终授权失败全快照回滚。R01首轮Bot响应解码fixture FAIL保留，仅新overlay修正后普通2.451秒／race14.161秒PASS；不同A01两顶层5.125秒PASS，补精确编辑内容及真正前进PUT最终撤销回滚。COMPOSE-I01核173产品＋3embed及原overlay、宽矩阵245＋3＋4输入和实际日志／用例身份完全适用e8，精确复用PASS、未新跑Go；原宽普通343／1526、十race312／1375、补bots26／142和vet／build／产品fmt／合同全PASS。只既定后端父done，不代CAS HTTP、生产附件绑定／下载／保护／GC、SYNC策略／平台发送、Android应用／WS或global；CAS仅父依赖满足、reply替换语义疑点仍todo非ready。
- **开发中｜附件功能**：STORAGE、真实上传HTTP叶及[限定上传父组合](tasks/COMPLETED.md#g5-be05-compose)均已done；原a647新组合检查、R01外URL原文保存不抓取／计数0及附件卷无变化补证、不同A01独立4.284秒、准确eeef Parent-I01产品／154输入等值复用全部闭环，I01未新跑Go。原失败及14顶层／3新增授权场景计数更正完整保留。消息关联、下载、后台恢复／回收、设置API、真机部署和global仍未验收。
- **未开始｜投递与清理（P6）**：原子领取、Long Polling、ACK／续约、退避重试、失败管理、历史／墓碑清理、队列生命周期、全局配置及健康投影。
- **未开始｜Hub 同步运行时**：增量同步、bootstrap 重建、WS 提示及连接撤销。
- **开发中｜正式推送服务端（P9）**：[STORAGE](tasks/COMPLETED.md#g5-be10-storage)、[五端点HTTP](tasks/COMPLETED.md#g5-be10-http)与[限定服务器父组合](tasks/COMPLETED.md#g5-be10-compose)各自独审／精确集成完成，父R01／A01／74e准确I01补齐并发模式、撤销重登旧回调及条件解绑；[PENDING-READ有限legacy只读叶](tasks/COMPLETED.md#g5-be10-pending-read)已done，完整源 `1d1… → bc716… → 11f53…`／不同A01独立6.224秒PASS，准确745d新union原6顶层＋5身份6.324秒／全服普通169.965秒／六包race304.773秒／vet2.699秒／build1.554秒／格式hash全PASS并已ff root。01／02 embed setup及03非法fixture CHECK失败→04PASS、口头7主更正和首次commit身份失败历史均保留，未改runtime或原产品测试断言。仅可信Tx legacy同步hint只读归并，输出有界扫描未界定，无发送资格；JOB0011真实mixed legacy＋完整sync＋alert现已另经准确I01通过，限模型／内部预留。当时完整待办调度的生产HTTP非零接线仍待实施（现已由下列SYNC-CREATE有限叶完成）；DDL／内部租约后由JOB完成，READ提醒策略／最终租约复核及FCM／OPPO Adapter／真机仍未完成；[F01已done／编号-01已关闭](tasks/COMPLETED.md#g5-be10-preflight-f01)：源 `1d1… → b305… → e42…`、不同A02六例4.001秒PASS，准确20f新组合六例4.913秒／全服普通159.039秒／八包race374.494秒／vet／build／格式hash全PASS并已ff root；A01原4过期FAIL／harness失败保留，HTTP仍nil targets无平台外发。UPLOAD有限HTTP叶已done，PENDING有限legacy只读叶已done，父调度仍未完成，真实非零接线见下列新完成叶。失效token仅按发送快照条件解绑对应注册，不删除设备会话。

- **已完成｜JOB-STORAGE有限叶**：[兼容持久模型＋内部租约预留](tasks/COMPLETED.md#g5-be10-job-storage)完整34b→74f→c145，经原A01需修、不同F01／A02关闭-01及准确0e98新组合I01完成；双映射93040fe5／c5097保留，旧d1全部Go NOT_RUN。新组合原Job14＋Pending6＋新混合1顶层／2purpose共21顶层PASS7.568秒、whole177.913秒／九race539.356秒／vet2.784秒／build1.547秒／格式hash全PASS，161输入＋3embed同并安全ff root0e98。旧SQL1～10不变，0011首次canonical永久占用；真实ReadPendingTx仅legacy、完整sync／alert可内部Reserve而legacy不可、五列writer／高水位隔离／device互斥／reopen均PASS。非最终发送资格，当时生产非零HTTP／快照形成未完成，现由下列SYNC-CREATE叶完成；父BE10-02的DND-READ策略／最终租约复核／attempt-retry-late-result／worker／平台及真机global仍未完成。

- **已完成｜SYNC-CREATE-HTTP有限生产者**：[完整source／不同A01／准确新组合I01](tasks/COMPLETED.md#g5-be10-sync-create-http)首次真实双入口TLS＋注册HTTP产生同事务完整sync快照，幂等命中不重新枚举／写入job；source35e准确一次映射ae39352并冻cfa，在保留附件0012／BE04父范围的准确并集定向19／57、全服350／1551、十一包race345／1542及vet／build／产品format全部PASS。177产品＋3真实embed＋两份原字节overlay前后相同，ordinary1551／race1542测试身份并集无缺失；原source／A01新fixture FAIL与复验、作者预检128、source wrapper暂停耗时及I01 range-diff来源注释脚本断言完整保留。Web／API不改，npm／标准validator本次NOT_RUN，不冒称平台发送。只本叶done；原BE10父仍running。[SYNC-LEASE-CHECK有限叶](tasks/COMPLETED.md#g5-be10-sync-lease-check)现已done：完整八节点最终b13、A01～A07／F1～F6闭环，七源准确映射至4289候选；当前含CAS新组合I01最小21／97、全服386／1750、四包race155／564、新cmd定向race2／6及vet／build／fmt全PASS，196＋3与原overlay等值。初次联合fixtureFAIL／修正、原全部FAIL／非Go异常与UUID窄归一raw差异保留。仅只读复核，非最终发送资格；[ATTEMPT-RESULT有限内部叶](tasks/COMPLETED.md#g5-be10-sync-attempt-result)现已done，A01两P2经F1／不同A02关闭，准确I01集成至0e01b88、206输入同值；最小13／150为I01新跑，whole394／1883、四race163／685及cmd1／13等为精确复用PASS，非新跑／全CI。0013永久占用，历史FAIL及NOT_RUN限制保留；[POSTCOMMIT-ATTEMPT-CHECK](tasks/COMPLETED.md#g5-be10-sync-postcommit-attempt-check)只读叶已done：source c6f／不同A01 PASS、准确I01映射7efebf8，210输入同值；最小33／270为I01新跑，whole398／1923、race166／722、cmd1／3及A01边界为精确复用PASS，历史草稿／FAIL与全CI NOT_RUN保持；[SYNC-SCOPED-RESERVE](tasks/COMPLETED.md#g5-be10-sync-scoped-reserve)有限叶已done，source c201经不同A01 PASS及准确I01映射2cc95a5、211输入同值；I01新22／56 PASS，whole406／1947、race174／746、cmd7／41及A01普通／race25／59为精确复用，历史失败／方法限制与全CI NOT_RUN保留；[SYNC-WORKER-CORE](tasks/BACKEND.md#be-10-02-sync-worker-core)有限假adapter叶ready，父策略／worker／真实发送／平台及Android global仍未done。

- **已完成｜可信Tx文本／按钮CAS子叶**：[完整source／F1／F2／最终A03及准确I01](tasks/COMPLETED.md#g5-be04-cas-text-buttons-storage)保原A01／A02四编号FAIL与原字节复验，8a08三次准确映射及最终666d44d新组合定向31／140、普通374／1666、五race176／686、新cmd组合race2／2和vet／build／格式全部PASS；185产品＋3embed＋4overlay同值。只内部事务文本按钮编辑，不重推、不改创建快照／已读；完整CAS HTTP、reply契约决定及附件绑定／callback范围仍未完成。

## 3. Web 管理端

本模块任务：[条目与验收](tasks/WEB.md)。

### 已完成

- **已完成｜工程与登录**：Web 工程、内嵌产物、登录／错误壳层。
- **已完成｜设备会话页面**：设备分页、精确撤销、自撤退出及迟到响应隔离；真实 HTTPS Chromium 双会话和 CSRF 拒绝验证通过。见[集成报告](tasks/COMPLETED.md#w1)。

### 未完成

- **未开始｜Bot 与凭据管理页面**。
- **未开始｜全局配置、失败队列、系统健康及连接摘要页面**。
- **未开始｜Web 嵌入与边界验收**：保持管理端不提供消息正文、附件或历史浏览的边界。

## 4. MoviePilot 插件

本模块任务：[条目与验收](tasks/MOVIEPILOT.md)。

### 已完成

- **已完成｜最小 PoC（P1）**：用户按调整后的范围验收，G-MP 已放行；v3.1.5 有固定文本入站、定向回复、媒体／种子候选与命名空间按钮导航等运行证据。方案 A 核心补丁及助手仅支持固定 v3.1.5 指纹；临时 HTTP 协议桩的收发、手动更新及附件夹具不代表正式 Hub／Android 端到端。见[用户验收与接受缺口](tasks/COMPLETED.md#mp1)。

### 未完成

- **未开始｜正式插件（P10）**：配置与生命周期、HubClient、持久 Inbox／Outbox 和 ID 映射、文本与原生业务权限接入、主动通知／定向回复、候选回调、附件转换上传、直接发送／编辑映射、插件包与恢复兼容验收。
- **待验收｜兼容与完整业务链路**：最低目标 v3.0.0／拟议声明 >=3.0.0 均未经对应运行验证；原 NOT_RUN 和未独立审计等缺口保留。P1 放行不等于 AC-07、P10／P11 或正式发布通过。
- **待验收｜权限及身份边界**：正式插件须绑定可信 Bot／user／request 身份；跨 Bot 结果依赖命名空间 userid，未证明宿主按 source 原生隔离。业务权限与危险确认仍由 MoviePilot 负责。
- **待验收｜环境与重建补证**：独立隔离实例、实际镜像／RepoDigest、重建／升级及补丁恢复仍有缺口；现网恢复快照镜像与 Compose 声明不同，重建可能覆盖补丁。原 `/netdisk` 挂载已授权移除，相关功能需重配有效路径。不撤销已批准的最小 PoC 例外，不擅自重建现网或恢复挂载。

## 5. 跨模块工程、契约与 CI

本模块任务：[条目与验收](tasks/ENGINEERING.md)。

### 已完成

- **已完成｜P3 工程与工具链**：Go、Web、Android 最小工程、固定工具链及 Go／Web／Android／插件静态检查 CI 配置；P03-01～P03-12 均已收口。
- **已完成｜API 契约与存储框架**：认证、Bot、消息、附件、投递、同步、设备及多通道推送契约；SQLite 连接和迁移框架。G-API 已通过，契约不等于运行时实现。
- **已完成｜实例身份扩展契约**：公开探测与请求绑定契约冻结，G-INSTANCE-CONTRACT 已通过；不代替运行时及组合门槛。

### 未完成

- **未开始｜正式构建发布链路**：P3 完成仅表示工程配置与局部检查有证据，不能推定远端发布流水线成功。镜像／插件构建发布、APK 正式签名和发布工作流归部署交付模块。
- **待验收｜跨端契约落地**：真实路由、客户端与完整实例绑定的组合验收尚未通过；各模块中的局部通过不得替代正式端到端。

## 6. 部署联调与交付

本模块任务：[条目与验收](tasks/DELIVERY.md)。

### 已完成

- 本模块尚无正式整体联调、真实部署或核心交付已完成的结论；前述 PoC 与局部集成证据不计作本模块完成。

### 未完成

- **未开始｜整体联调（P11）**：fnOS 反代拓扑、核心双向跨组件、撤销与 Bot 生命周期交叉、历史缺口／文件故障、资源／日志／隐私综合验收。
- **未开始｜部署与发布准备（P12）**：容器／Compose、OpenWrt Nginx 配置、镜像／插件构建发布、APK 签名、首次部署／正常升级验收，以及核心版交付证据收口。
- **待验收｜部署环境**：NAS／反代可信只读入口、独立测试资源、HTTPS Origin 清单待部署验收前提供，不阻塞其他本地独立开发。
- **待验收｜核心与部署门槛**：G-CORE、G-DEPLOY 均 pending。真实部署、MP 重建／升级、破坏性数据操作、公开发布、远端推送和 Release，仍需具体结果准备完成后的最后确认。

## 7. RelayShelf 后置范围

本模块任务：[条目与验收](tasks/RELAYSHELF.md)。

### 已完成

- 本模块尚无已完成的功能；后续范围已获用户授权，但核心版实际完成与 G-SHELF 门槛尚未满足。

### 未完成

- **后置｜功能矩阵与原生 PoC（P13）**：账号／安全／隐私、内容生命周期／检索、文件／共享／实时／管理矩阵，以及认证、文件、SSE PoC 均未开始。
- **后置｜Android 双后端与原生功能（P14）**：独立登录、账号会话、内容及生命周期、编辑／标签／收藏／回收站／检索、上传续传／下载预览、敏感内容、跨用户转发、SSE、设置与用户管理均未开始。
- **后置｜扩展版验收交付（P15）**：账号隐私、内容检索、传输恢复、分享实时管理、双端互不影响与核心回归、扩展版交付准备均未开始。
- **后置｜启动门槛**：G-SHELF、G-SHELF-API 均 pending；P12-06 准备报告不能自动开启后续开发。RelayShelf 是独立项目，不擅自修改其后端或数据库。

历史交接按[任务入口](tasks/README.md#历史追踪)检索原基线；后续调度遵循[执行规则](tasks/WORKFLOW.md)。旧 checkpoint 或运行标记不能代替最新实际提交和执行证据；本文不据此新增任务或启动开发。
