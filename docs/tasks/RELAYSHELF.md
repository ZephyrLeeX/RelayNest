# RelayShelf 后置任务

用户已授权 P13～P15 后续范围，但以下32个叶任务全部 `deferred`。G-SHELF pending：须核心实际完成、G-CORE 与 G-DEPLOY 实证；DEL-05-02（旧P12-06）准备报告本身不足以启动。G-SHELF-API pending，原生功能必须以实际矩阵和 PoC 为依据。

RelayShelf 是独立项目，固定 v0.4.4 基线研究，不擅改其后端/数据库。未知原生 API、账号资格、角色权限先依据官方文档、目标源码及获准部署UI/API核实，不能编造字段或由接口存在推定兼容。若缺接口、需要改后端或改变范围，先整理可复核证据与具体方案交用户，未知功能不得悄悄删去。双后端 Cookie、数据库、缓存、身份、SSE 与会话隔离，单端故障、改密、注销不能影响另一个后端或核心版。

完整矩阵覆盖账号安全/隐私、内容格式/生命周期/编辑/版本与历史能力、检索/保存搜索、标签/收藏/回收站、分片恢复/下载预览、跨用户发送/共享、SSE、系统与用户管理；历史/版本能力先按矩阵确认，不据概括新增接口。每项落实在下面具体叶任务，不把18项 Android 原生功能合并成一个 worker。

## 执行与证据约定

仅按下面一个叶任务执行，父组仅导航及收口。默认每任务修改3～5个主要实现文件及对应测试；路径是职责边界，优先已有实现，改名先记录等价路径。需要跨职责、产品、架构、安全或外部接口变更，先交用户确认。Bot Hub 只传输、存储、路由；业务权限及危险确认由业务端负责，不执行命令或 MCP。

开始前读取根 `AGENTS.md`、[工程规范](../CONTRIBUTING.md)、本任务定位的技术章节、具体依赖的已集成交接和相关代码；不整包载入旧卡/报告。单次上下文建议≤12k tokens、总窗口占用≤60%，留≥25%交付余量，超出先记录 checkpoint 和剩余工作。

每个叶任务均须运行受影响检查，逐项记录命令、环境、结果；缺条件保留 `blocked` / `NOT_RUN`，不推定通过。独立任务单独本地提交，提供基线SHA、实现SHA、diff摘要、验收证据和遗留问题；独立审计与精确集成后才可标 `done`。验证可以得出负面结论，但不等于兼容门槛通过。只读研究也保留精简证据，禁止凭据/外部源码入库。不顺带实现前后阶段、全模块重构或另一后端，不修改 RelayShelf 后端/数据库或 MP 核心。不得接管调度、派生子agent或改其他任务状态；新增工作先交回拆分。审计核对范围、精确SHA、验收及越权/故障边界，不写代码；问题经单独修复后重审，集成验证最终组合。交接摘要≤800中文字符，完整证据另存。不得以准备报告、源码推断、协议桩、本地或模拟器结果替代外部系统/真机验证。普通准备、开发、测试、审计和集成已持续授权；真实部署、破坏性数据操作、MP重建/升级、远端推送、公开发布或 Release 仍须在具体结果准备后最后确认。

状态、ID、具体依赖及门槛按 `086fa51d79d7726545a9abbab8feebb25770034e` 基线重组冻结；已完成依赖链接 表示[已完成基线](COMPLETED.md)中的历史任务。重要既有例外见[决定摘要](COMPLETED.md#e2)。历史原卡可用 `git show 086fa51d79d7726545a9abbab8feebb25770034e:docs/tasks/cards/旧ID.md` 取证，不恢复旧调度台账。

## 导航

- `SHELF-01` 矩阵与原生PoC：[SHELF-01-01](#shelf-01-01)、[SHELF-01-02](#shelf-01-02)、[SHELF-01-03](#shelf-01-03)、[SHELF-01-04](#shelf-01-04)、[SHELF-01-05](#shelf-01-05)、[SHELF-01-06](#shelf-01-06)、[SHELF-01-07](#shelf-01-07)。
- `SHELF-02` 双后端原生功能：[SHELF-02-01](#shelf-02-01)、[SHELF-02-02](#shelf-02-02)、[SHELF-02-03](#shelf-02-03)、[SHELF-02-04](#shelf-02-04)、[SHELF-02-05](#shelf-02-05)、[SHELF-02-06](#shelf-02-06)、[SHELF-02-07](#shelf-02-07)、[SHELF-02-08](#shelf-02-08)、[SHELF-02-09](#shelf-02-09)、[SHELF-02-10](#shelf-02-10)、[SHELF-02-11](#shelf-02-11)、[SHELF-02-12](#shelf-02-12)、[SHELF-02-13](#shelf-02-13)、[SHELF-02-14](#shelf-02-14)、[SHELF-02-15](#shelf-02-15)、[SHELF-02-16](#shelf-02-16)、[SHELF-02-17](#shelf-02-17)、[SHELF-02-18](#shelf-02-18)。
- `SHELF-03` 扩展版验收与交付：[SHELF-03-01](#shelf-03-01)、[SHELF-03-02](#shelf-03-02)、[SHELF-03-03](#shelf-03-03)、[SHELF-03-04](#shelf-03-04)、[SHELF-03-05](#shelf-03-05)、[SHELF-03-06](#shelf-03-06)、[SHELF-03-07](#shelf-03-07)。

<a id="shelf-01-01"></a>

### SHELF-01-01 RelayShelf 账号安全隐私功能矩阵

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

- 旧ID：`P13-01`；状态：`deferred`。
- 具体依赖：[DEL-05-02](DELIVERY.md#del-05-02)。
- 门槛：G-GIT（pass）, G-SHELF（pending）。
- 外部阻塞：G-SHELF尚未放行；其余前置接口、账号/环境以原生矩阵证据核实，缺失保留NOT_RUN。
- 技术定位：`T11;D13`（T=TECHNICAL_DESIGN、A=ARCHITECTURE、D=DEVELOPMENT_PLAN、PRD=PRD；按实际标题核对）。

允许范围：

- 本任务脱敏证据栏（worker 回交，由总调度登记）

工作：

1. 固定v0.4.4核对账号/TOTP/设备/会话/隐私页面与接口。
2. 记录角色权限和用例。

验收（另适用上方共同检查/提交/审计要求）：

- [ ] 每项有源码/部署UI/API证据。
- [ ] 不编造接口。
- [ ] 未实测明确标记。

<a id="shelf-01-02"></a>

### SHELF-01-02 RelayShelf 认证原生 PoC

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

- 旧ID：`P13-02`；状态：`deferred`。
- 具体依赖：[SHELF-01-07](RELAYSHELF.md#shelf-01-07)。
- 门槛：G-GIT（pass）, G-SHELF（pending）。
- 外部阻塞：G-SHELF尚未放行；其余前置接口、账号/环境以原生矩阵证据核实，缺失保留NOT_RUN。
- 技术定位：`T11.1,11.3`（T=TECHNICAL_DESIGN、A=ARCHITECTURE、D=DEVELOPMENT_PLAN、PRD=PRD；按实际标题核对）。

允许范围：

- `android/poc/relayshelf/auth/**`
- 本任务脱敏证据栏（worker 回交，由总调度登记）

工作：

1. 最小验证Cookie/CSRF/Origin/端口/TOTP/会话/部分登录。
2. 不改后端。

验收（另适用上方共同检查/提交/审计要求）：

- [ ] 挑战前无Cookie。
- [ ] 双端隔离。
- [ ] 重定向不泄露凭据。
- [ ] 错误/过期路径。

<a id="shelf-01-03"></a>

### SHELF-01-03 RelayShelf 文件协议原生 PoC

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

- 旧ID：`P13-03`；状态：`deferred`。
- 具体依赖：[SHELF-01-02](RELAYSHELF.md#shelf-01-02)。
- 门槛：G-GIT（pass）, G-SHELF（pending）。
- 外部阻塞：G-SHELF尚未放行；其余前置接口、账号/环境以原生矩阵证据核实，缺失保留NOT_RUN。
- 技术定位：`T11.2`（T=TECHNICAL_DESIGN、A=ARCHITECTURE、D=DEVELOPMENT_PLAN、PRD=PRD；按实际标题核对）。

允许范围：

- `android/poc/relayshelf/transfers/**`
- 本任务脱敏证据栏（worker 回交，由总调度登记）

工作：

1. 继续追踪实际上传校验/去重字段。
2. 验证分片/续传/下载/预览和URI权限。

验收（另适用上方共同检查/提交/审计要求）：

- [ ] 中断恢复/文件变化/校验失败/去重/下载权限有证据。

<a id="shelf-01-04"></a>

### SHELF-01-04 RelayShelf SSE PoC 与矩阵冻结

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

- 旧ID：`P13-04`；状态：`deferred`。
- 具体依赖：[SHELF-01-03](RELAYSHELF.md#shelf-01-03)。
- 门槛：G-GIT（pass）, G-SHELF（pending）。
- 外部阻塞：G-SHELF尚未放行；其余前置接口、账号/环境以原生矩阵证据核实，缺失保留NOT_RUN。
- 技术定位：`T11.2,11.3`（T=TECHNICAL_DESIGN、A=ARCHITECTURE、D=DEVELOPMENT_PLAN、PRD=PRD；按实际标题核对）。

允许范围：

- `android/poc/relayshelf/realtime/**`
- 本任务脱敏证据栏（worker 回交，由总调度登记）

工作：

1. 验证元数据SSE/断线重取。
2. 汇总矩阵和兼容缺口，形成G-SHELF-API。

验收（另适用上方共同检查/提交/审计要求）：

- [ ] 不依Last-Event-ID重放。
- [ ] 若需后端改动先needs_user。
- [ ] 不得据存在接口标通过。

<a id="shelf-01-05"></a>

### SHELF-01-05 RelayShelf 内容生命周期检索矩阵

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

- 旧ID：`P13-05`；状态：`deferred`。
- 具体依赖：[SHELF-01-01](RELAYSHELF.md#shelf-01-01)。
- 门槛：G-GIT（pass）, G-SHELF（pending）。
- 外部阻塞：G-SHELF尚未放行；其余前置接口、账号/环境以原生矩阵证据核实，缺失保留NOT_RUN。
- 技术定位：`T11.2;D13`（T=TECHNICAL_DESIGN、A=ARCHITECTURE、D=DEVELOPMENT_PLAN、PRD=PRD；按实际标题核对）。

允许范围：

- 本任务脱敏证据栏（worker 回交，由总调度登记）

工作：

1. 核对内容格式/编辑/生命周期/回收站/标签/收藏/搜索/保存搜索。
2. 逐项定位接口与用例。

验收（另适用上方共同检查/提交/审计要求）：

- [ ] 矩阵覆盖简表外保存搜索。
- [ ] 权限明确。
- [ ] 不把接口存在当兼容通过。

<a id="shelf-01-06"></a>

### SHELF-01-06 RelayShelf 文件共享实时管理矩阵

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

- 旧ID：`P13-06`；状态：`deferred`。
- 具体依赖：[SHELF-01-05](RELAYSHELF.md#shelf-01-05)。
- 门槛：G-GIT（pass）, G-SHELF（pending）。
- 外部阻塞：G-SHELF尚未放行；其余前置接口、账号/环境以原生矩阵证据核实，缺失保留NOT_RUN。
- 技术定位：`T11.2;D13`（T=TECHNICAL_DESIGN、A=ARCHITECTURE、D=DEVELOPMENT_PLAN、PRD=PRD；按实际标题核对）。

允许范围：

- 本任务脱敏证据栏（worker 回交，由总调度登记）

工作：

1. 核对文件/分片/下载/预览/跨用户发送转发/SSE/系统管理。
2. 定位每项源码与接口。

验收（另适用上方共同检查/提交/审计要求）：

- [ ] 不遗漏UI内现有管理与共享。
- [ ] 未知能力写缺口。
- [ ] 按域过滤源码避免全仓输入。

<a id="shelf-01-07"></a>

### SHELF-01-07 RelayShelf 矩阵合并与PoC用例冻结

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

- 旧ID：`P13-07`；状态：`deferred`。
- 具体依赖：[SHELF-01-06](RELAYSHELF.md#shelf-01-06)。
- 门槛：G-GIT（pass）, G-SHELF（pending）。
- 外部阻塞：G-SHELF尚未放行；其余前置接口、账号/环境以原生矩阵证据核实，缺失保留NOT_RUN。
- 技术定位：`T11;D13`（T=TECHNICAL_DESIGN、A=ARCHITECTURE、D=DEVELOPMENT_PLAN、PRD=PRD；按实际标题核对）。

允许范围：

- 本任务脱敏证据栏（worker 回交，由总调度登记）

工作：

1. 合并三份矩阵摘要并查覆盖空洞。
2. 为认证/文件/SSE PoC固定用例。

验收（另适用上方共同检查/提交/审计要求）：

- [ ] 全功能均有域内证据定位。
- [ ] 未知项不自动删除。
- [ ] 缺口若影响后端先needs_user。

<a id="shelf-02-01"></a>

### SHELF-02-01 双后端配置与独立登录状态

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

- 旧ID：`P14-01`；状态：`deferred`。
- 具体依赖：[SHELF-01-04](RELAYSHELF.md#shelf-01-04)、[AND-01](ANDROID.md#and-01)。
- 门槛：G-GIT（pass）, G-SHELF-API（pending）。
- 外部阻塞：G-SHELF尚未放行；其余前置接口、账号/环境以原生矩阵证据核实，缺失保留NOT_RUN。
- 技术定位：`T11.3`（T=TECHNICAL_DESIGN、A=ARCHITECTURE、D=DEVELOPMENT_PLAN、PRD=PRD；按实际标题核对）。

允许范围：

- `android/feature/auth/**`
- `android/core/network/relayshelf/**`

工作：

1. 实现可选RelayShelf配置、统一输入/双端状态/专属CookieJar/TOTP挑战。

验收（另适用上方共同检查/提交/审计要求）：

- [ ] 单端失败/重认证/注销不影响另一端。
- [ ] 不长期存密码。

<a id="shelf-02-02"></a>

### SHELF-02-02 RelayShelf 账号安全与会话 UI

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

- 旧ID：`P14-02`；状态：`deferred`。
- 具体依赖：[SHELF-02-01](RELAYSHELF.md#shelf-02-01)。
- 门槛：G-GIT（pass）, G-SHELF-API（pending）。
- 外部阻塞：G-SHELF尚未放行；其余前置接口、账号/环境以原生矩阵证据核实，缺失保留NOT_RUN。
- 技术定位：`T11.2`（T=TECHNICAL_DESIGN、A=ARCHITECTURE、D=DEVELOPMENT_PLAN、PRD=PRD；按实际标题核对）。

允许范围：

- `android/feature/relayshelf/security/**`

工作：

1. 按矩阵实现TOTP设置、密码/会话/设备及权限异常。

验收（另适用上方共同检查/提交/审计要求）：

- [ ] 跨设备撤销/挑战过期/改密和部分登录。
- [ ] 不自动同步密码。

<a id="shelf-02-03"></a>

### SHELF-02-03 RelayShelf 内容列表详情与创建

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

- 旧ID：`P14-03`；状态：`deferred`。
- 具体依赖：[SHELF-02-01](RELAYSHELF.md#shelf-02-01)。
- 门槛：G-GIT（pass）, G-SHELF-API（pending）。
- 外部阻塞：G-SHELF尚未放行；其余前置接口、账号/环境以原生矩阵证据核实，缺失保留NOT_RUN。
- 技术定位：`T11.2`（T=TECHNICAL_DESIGN、A=ARCHITECTURE、D=DEVELOPMENT_PLAN、PRD=PRD；按实际标题核对）。

允许范围：

- `android/feature/relayshelf/content/**`

工作：

1. 实现文本/Markdown/代码/链接列表/详情/创建。
2. 图片文件等待传输任务接入。

验收（另适用上方共同检查/提交/审计要求）：

- [ ] 格式/版本/权限/错误/空状态按矩阵。
- [ ] 无WebView。

<a id="shelf-02-04"></a>

### SHELF-02-04 RelayShelf Temporary/Permanent 生命周期

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

- 旧ID：`P14-04`；状态：`deferred`。
- 具体依赖：[SHELF-02-03](RELAYSHELF.md#shelf-02-03)。
- 门槛：G-GIT（pass）, G-SHELF-API（pending）。
- 外部阻塞：G-SHELF尚未放行；其余前置接口、账号/环境以原生矩阵证据核实，缺失保留NOT_RUN。
- 技术定位：`T11.2`（T=TECHNICAL_DESIGN、A=ARCHITECTURE、D=DEVELOPMENT_PLAN、PRD=PRD；按实际标题核对）。

允许范围：

- `android/feature/relayshelf/lifecycle/**`

工作：

1. 实现临时/永久转换、期限延长和过期展示。
2. 不处理回收站。

验收（另适用上方共同检查/提交/审计要求）：

- [ ] 规则/权限依冻结矩阵。
- [ ] 版本冲突明确。
- [ ] 不改服务端。

<a id="shelf-02-05"></a>

### SHELF-02-05 RelayShelf 标签与收藏

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

- 旧ID：`P14-05`；状态：`deferred`。
- 具体依赖：[SHELF-02-13](RELAYSHELF.md#shelf-02-13)、[SHELF-02-14](RELAYSHELF.md#shelf-02-14)。
- 门槛：G-GIT（pass）, G-SHELF-API（pending）。
- 外部阻塞：G-SHELF尚未放行；其余前置接口、账号/环境以原生矩阵证据核实，缺失保留NOT_RUN。
- 技术定位：`T11.2`（T=TECHNICAL_DESIGN、A=ARCHITECTURE、D=DEVELOPMENT_PLAN、PRD=PRD；按实际标题核对）。

允许范围：

- `android/feature/relayshelf/organize/**`

工作：

1. 实现标签管理/内容标签和收藏。
2. 列表筛选接入现有模型。

验收（另适用上方共同检查/提交/审计要求）：

- [ ] 角色权限/并发修改/空状态可用。
- [ ] 无未确认功能删减。

<a id="shelf-02-06"></a>

### SHELF-02-06 RelayShelf 分片上传状态仓储

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

- 旧ID：`P14-06`；状态：`deferred`。
- 具体依赖：[SHELF-02-01](RELAYSHELF.md#shelf-02-01)、[SHELF-01-03](RELAYSHELF.md#shelf-01-03)。
- 门槛：G-GIT（pass）, G-SHELF-API（pending）。
- 外部阻塞：G-SHELF尚未放行；其余前置接口、账号/环境以原生矩阵证据核实，缺失保留NOT_RUN。
- 技术定位：`T11.2`（T=TECHNICAL_DESIGN、A=ARCHITECTURE、D=DEVELOPMENT_PLAN、PRD=PRD；按实际标题核对）。

允许范围：

- `android/core/transfers/relayshelf/**`
- `android/core/database/relayshelf/**`

工作：

1. 实现上传会话/分片进度/指纹/校验/恢复记录，先无UI。

验收（另适用上方共同检查/提交/审计要求）：

- [ ] 崩溃恢复/去重/校验/并发分片按真实契约。
- [ ] 不自创字段。

<a id="shelf-02-07"></a>

### SHELF-02-07 RelayShelf 上传调度与续传 UI

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

- 旧ID：`P14-07`；状态：`deferred`。
- 具体依赖：[SHELF-02-06](RELAYSHELF.md#shelf-02-06)。
- 门槛：G-GIT（pass）, G-SHELF-API（pending）。
- 外部阻塞：G-SHELF尚未放行；其余前置接口、账号/环境以原生矩阵证据核实，缺失保留NOT_RUN。
- 技术定位：`T11.2`（T=TECHNICAL_DESIGN、A=ARCHITECTURE、D=DEVELOPMENT_PLAN、PRD=PRD；按实际标题核对）。

允许范围：

- `android/core/transfers/relayshelf/**`
- `android/feature/relayshelf/uploads/**`

工作：

1. 实现调度/失败恢复/系统URI权限/断点续传和图片文件创建入口。

验收（另适用上方共同检查/提交/审计要求）：

- [ ] 重启/权限丢失/文件变化/网络切换。
- [ ] 完成后内容关联正确。

<a id="shelf-02-08"></a>

### SHELF-02-08 RelayShelf 下载打包与原生预览

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

- 旧ID：`P14-08`；状态：`deferred`。
- 具体依赖：[SHELF-02-07](RELAYSHELF.md#shelf-02-07)。
- 门槛：G-GIT（pass）, G-SHELF-API（pending）。
- 外部阻塞：G-SHELF尚未放行；其余前置接口、账号/环境以原生矩阵证据核实，缺失保留NOT_RUN。
- 技术定位：`T11.2`（T=TECHNICAL_DESIGN、A=ARCHITECTURE、D=DEVELOPMENT_PLAN、PRD=PRD；按实际标题核对）。

允许范围：

- `android/feature/relayshelf/attachments/**`
- `android/core/transfers/relayshelf/**`

工作：

1. 按矩阵实现单文件/打包/缩略图/预览/恢复和受控分享。

验收（另适用上方共同检查/提交/审计要求）：

- [ ] Range/敏感权限/不支持类型清晰。
- [ ] 不以WebView替代业务。

<a id="shelf-02-09"></a>

### SHELF-02-09 RelayShelf 敏感内容保护

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

- 旧ID：`P14-09`；状态：`deferred`。
- 具体依赖：[SHELF-02-04](RELAYSHELF.md#shelf-02-04)、[SHELF-02-08](RELAYSHELF.md#shelf-02-08)。
- 门槛：G-GIT（pass）, G-SHELF-API（pending）。
- 外部阻塞：G-SHELF尚未放行；其余前置接口、账号/环境以原生矩阵证据核实，缺失保留NOT_RUN。
- 技术定位：`T11.2`（T=TECHNICAL_DESIGN、A=ARCHITECTURE、D=DEVELOPMENT_PLAN、PRD=PRD；按实际标题核对）。

允许范围：

- `android/feature/relayshelf/privacy/**`
- `android/feature/relayshelf/content/**`

工作：

1. 对齐显式揭示/编辑/缓存/预览及现有隐私行为。
2. 不新增未经确认安全策略。

验收（另适用上方共同检查/提交/审计要求）：

- [ ] 揭示前无正文泄露。
- [ ] 会话失效/切后台的既有规则逐项验证。

<a id="shelf-02-10"></a>

### SHELF-02-10 RelayShelf 跨用户发送与转发

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

- 旧ID：`P14-10`；状态：`deferred`。
- 具体依赖：[SHELF-02-16](RELAYSHELF.md#shelf-02-16)。
- 门槛：G-GIT（pass）, G-SHELF-API（pending）。
- 外部阻塞：G-SHELF尚未放行；其余前置接口、账号/环境以原生矩阵证据核实，缺失保留NOT_RUN。
- 技术定位：`T11.2`（T=TECHNICAL_DESIGN、A=ARCHITECTURE、D=DEVELOPMENT_PLAN、PRD=PRD；按实际标题核对）。

允许范围：

- `android/feature/relayshelf/sharing/**`

工作：

1. 按真实矩阵实现接收人/发送/转发/已有共享能力。

验收（另适用上方共同检查/提交/审计要求）：

- [ ] 普通/管理员权限。
- [ ] 撤销用户/对象不存在。
- [ ] 不能凭共享词编造接口。

<a id="shelf-02-11"></a>

### SHELF-02-11 RelayShelf SSE 更新与恢复

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

- 旧ID：`P14-11`；状态：`deferred`。
- 具体依赖：[SHELF-02-09](RELAYSHELF.md#shelf-02-09)、[SHELF-02-10](RELAYSHELF.md#shelf-02-10)、[SHELF-01-04](RELAYSHELF.md#shelf-01-04)。
- 门槛：G-GIT（pass）, G-SHELF-API（pending）。
- 外部阻塞：G-SHELF尚未放行；其余前置接口、账号/环境以原生矩阵证据核实，缺失保留NOT_RUN。
- 技术定位：`T11.2`（T=TECHNICAL_DESIGN、A=ARCHITECTURE、D=DEVELOPMENT_PLAN、PRD=PRD；按实际标题核对）。

允许范围：

- `android/core/sync/relayshelf/**`

工作：

1. 实现元数据事件触发重取、断线重建和会话过期处理。

验收（另适用上方共同检查/提交/审计要求）：

- [ ] 不复用Hub游标。
- [ ] 并发编辑/删除/重连一致。
- [ ] 双端流量隔离。

<a id="shelf-02-12"></a>

### SHELF-02-12 RelayShelf 矩阵查漏与补充卡规划

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

- 旧ID：`P14-12`；状态：`deferred`。
- 具体依赖：[SHELF-02-18](RELAYSHELF.md#shelf-02-18)、[SHELF-02-09](RELAYSHELF.md#shelf-02-09)、[SHELF-02-13](RELAYSHELF.md#shelf-02-13)、[SHELF-02-16](RELAYSHELF.md#shelf-02-16)。
- 门槛：G-GIT（pass）, G-SHELF-API（pending）。
- 外部阻塞：G-SHELF尚未放行；其余前置接口、账号/环境以原生矩阵证据核实，缺失保留NOT_RUN。
- 技术定位：`T11.2`（T=TECHNICAL_DESIGN、A=ARCHITECTURE、D=DEVELOPMENT_PLAN、PRD=PRD；按实际标题核对）。

允许范围：

- 本任务脱敏证据栏（worker 回交，由总调度登记）

工作：

1. 仅核对冻结矩阵与已交付证据。
2. 发现遗漏交规划补充卡并登记，不在本卡兜底实现。

验收（另适用上方共同检查/提交/审计要求）：

- [ ] 每功能有任务/证据。
- [ ] 遗漏未完成则本卡保持blocked直到补充卡done。
- [ ] 不得据已写代码标完成。

<a id="shelf-02-13"></a>

### SHELF-02-13 RelayShelf 回收站恢复与永久删除

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

- 旧ID：`P14-13`；状态：`deferred`。
- 具体依赖：[SHELF-02-04](RELAYSHELF.md#shelf-02-04)。
- 门槛：G-GIT（pass）, G-SHELF-API（pending）。
- 外部阻塞：G-SHELF尚未放行；其余前置接口、账号/环境以原生矩阵证据核实，缺失保留NOT_RUN。
- 技术定位：`T11.2`（T=TECHNICAL_DESIGN、A=ARCHITECTURE、D=DEVELOPMENT_PLAN、PRD=PRD；按实际标题核对）。

允许范围：

- `android/feature/relayshelf/trash/**`

工作：

1. 实现回收站分页/恢复/永久删除及确认。
2. 对齐现有权限。

验收（另适用上方共同检查/提交/审计要求）：

- [ ] 生命周期结果一致。
- [ ] 版本/权限/失效对象错误明确。
- [ ] 不删除未授权内容。

<a id="shelf-02-14"></a>

### SHELF-02-14 RelayShelf 内容编辑与版本冲突

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

- 旧ID：`P14-14`；状态：`deferred`。
- 具体依赖：[SHELF-02-03](RELAYSHELF.md#shelf-02-03)。
- 门槛：G-GIT（pass）, G-SHELF-API（pending）。
- 外部阻塞：G-SHELF尚未放行；其余前置接口、账号/环境以原生矩阵证据核实，缺失保留NOT_RUN。
- 技术定位：`T11.2`（T=TECHNICAL_DESIGN、A=ARCHITECTURE、D=DEVELOPMENT_PLAN、PRD=PRD；按实际标题核对）。

允许范围：

- `android/feature/relayshelf/content/**`

工作：

1. 实现已有各正文格式编辑与CAS。
2. 保存失败保留输入。

验收（另适用上方共同检查/提交/审计要求）：

- [ ] 并发版本不静默覆盖。
- [ ] 格式/权限/错误按矩阵。
- [ ] 不新增后端规则。

<a id="shelf-02-15"></a>

### SHELF-02-15 RelayShelf 搜索与过滤

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

- 旧ID：`P14-15`；状态：`deferred`。
- 具体依赖：[SHELF-02-05](RELAYSHELF.md#shelf-02-05)。
- 门槛：G-GIT（pass）, G-SHELF-API（pending）。
- 外部阻塞：G-SHELF尚未放行；其余前置接口、账号/环境以原生矩阵证据核实，缺失保留NOT_RUN。
- 技术定位：`T11.2`（T=TECHNICAL_DESIGN、A=ARCHITECTURE、D=DEVELOPMENT_PLAN、PRD=PRD；按实际标题核对）。

允许范围：

- `android/feature/relayshelf/search/**`

工作：

1. 实现冻结矩阵内条件/组合/排序/分页。
2. 不含保存搜索管理。

验收（另适用上方共同检查/提交/审计要求）：

- [ ] 复杂条件序列化与服务端一致。
- [ ] 分页稳定。
- [ ] 权限隔离。

<a id="shelf-02-16"></a>

### SHELF-02-16 RelayShelf 保存搜索

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

- 旧ID：`P14-16`；状态：`deferred`。
- 具体依赖：[SHELF-02-15](RELAYSHELF.md#shelf-02-15)。
- 门槛：G-GIT（pass）, G-SHELF-API（pending）。
- 外部阻塞：G-SHELF尚未放行；其余前置接口、账号/环境以原生矩阵证据核实，缺失保留NOT_RUN。
- 技术定位：`T11.2`（T=TECHNICAL_DESIGN、A=ARCHITECTURE、D=DEVELOPMENT_PLAN、PRD=PRD；按实际标题核对）。

允许范围：

- `android/feature/relayshelf/search/saved/**`

工作：

1. 实现保存/重命名/更新/删除/调用已保存搜索。
2. 复用搜索模型。

验收（另适用上方共同检查/提交/审计要求）：

- [ ] 条件不丢失。
- [ ] 权限和错误正确。
- [ ] 覆盖v0.4.4现有行为。

<a id="shelf-02-17"></a>

### SHELF-02-17 RelayShelf 系统设置与存储管理

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

- 旧ID：`P14-17`；状态：`deferred`。
- 具体依赖：[SHELF-02-11](RELAYSHELF.md#shelf-02-11)。
- 门槛：G-GIT（pass）, G-SHELF-API（pending）。
- 外部阻塞：G-SHELF尚未放行；其余前置接口、账号/环境以原生矩阵证据核实，缺失保留NOT_RUN。
- 技术定位：`T11.2`（T=TECHNICAL_DESIGN、A=ARCHITECTURE、D=DEVELOPMENT_PLAN、PRD=PRD；按实际标题核对）。

允许范围：

- `android/feature/relayshelf/admin/settings/**`
- `android/feature/relayshelf/admin/storage/**`

工作：

1. 按矩阵实现系统设置/存储状态。
2. 不变更后端管理规则。

验收（另适用上方共同检查/提交/审计要求）：

- [ ] 普通用户不可越权。
- [ ] 敏感配置不日志输出。
- [ ] 设置错误可恢复。

<a id="shelf-02-18"></a>

### SHELF-02-18 RelayShelf 用户管理

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

- 旧ID：`P14-18`；状态：`deferred`。
- 具体依赖：[SHELF-02-17](RELAYSHELF.md#shelf-02-17)。
- 门槛：G-GIT（pass）, G-SHELF-API（pending）。
- 外部阻塞：G-SHELF尚未放行；其余前置接口、账号/环境以原生矩阵证据核实，缺失保留NOT_RUN。
- 技术定位：`T11.2`（T=TECHNICAL_DESIGN、A=ARCHITECTURE、D=DEVELOPMENT_PLAN、PRD=PRD；按实际标题核对）。

允许范围：

- `android/feature/relayshelf/admin/users/**`

工作：

1. 按矩阵实现管理员用户列表/创建/禁用/删除/重置。
2. 明确现有确认流程。

验收（另适用上方共同检查/提交/审计要求）：

- [ ] 角色权限和失败路径。
- [ ] 不联动Hub管理员或密码。
- [ ] 不改服务端。

<a id="shelf-03-01"></a>

### SHELF-03-01 RelayShelf 账号安全与隐私验收

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

- 旧ID：`P15-01`；状态：`deferred`。
- 具体依赖：[SHELF-02-12](RELAYSHELF.md#shelf-02-12)。
- 门槛：G-GIT（pass）, G-SHELF-API（pending）。
- 外部阻塞：G-SHELF尚未放行；其余前置接口、账号/环境以原生矩阵证据核实，缺失保留NOT_RUN。
- 技术定位：`D15`（T=TECHNICAL_DESIGN、A=ARCHITECTURE、D=DEVELOPMENT_PLAN、PRD=PRD；按实际标题核对）。

允许范围：

- `tests/integration/relayshelf/**`
- 本任务脱敏证据栏（worker 回交，由总调度登记）

工作：

1. 仅执行账号/TOTP/设备会话/敏感内容验收。
2. 失败派修复卡。

验收（另适用上方共同检查/提交/审计要求）：

- [ ] 认证和隐私矩阵有实测。
- [ ] 无Cookie混用/隐私泄露。
- [ ] 缺环境不标通过。

<a id="shelf-03-02"></a>

### SHELF-03-02 双端互不影响与核心回归

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

- 旧ID：`P15-02`；状态：`deferred`。
- 具体依赖：[SHELF-03-07](RELAYSHELF.md#shelf-03-07)。
- 门槛：G-GIT（pass）。
- 外部阻塞：G-SHELF尚未放行；其余前置接口、账号/环境以原生矩阵证据核实，缺失保留NOT_RUN。
- 技术定位：`D15`（T=TECHNICAL_DESIGN、A=ARCHITECTURE、D=DEVELOPMENT_PLAN、PRD=PRD；按实际标题核对）。

允许范围：

- `tests/integration/**`
- 本任务脱敏证据栏（worker 回交，由总调度登记）

工作：

1. 验证单端宕机/改密/过期和Bot核心/MP/通知/升级回归。

验收（另适用上方共同检查/提交/审计要求）：

- [ ] Cookie/数据库/缓存/身份隔离。
- [ ] 核心AC不退化。

<a id="shelf-03-03"></a>

### SHELF-03-03 扩展版交付准备

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

- 旧ID：`P15-03`；状态：`deferred`。
- 具体依赖：[SHELF-03-02](RELAYSHELF.md#shelf-03-02)。
- 门槛：G-GIT（pass）。
- 外部阻塞：G-SHELF尚未放行；其余前置接口、账号/环境以原生矩阵证据核实，缺失保留NOT_RUN。
- 技术定位：`D15`（T=TECHNICAL_DESIGN、A=ARCHITECTURE、D=DEVELOPMENT_PLAN、PRD=PRD；按实际标题核对）。

允许范围：

- 本任务脱敏证据栏（worker 回交，由总调度登记）
- `deploy/README.md`

工作：

1. 汇总完整验收/产物/升级/限制，准备扩展版交付。

验收（另适用上方共同检查/提交/审计要求）：

- [ ] 只有实测后改延期为通过。
- [ ] 不自动发布/推送。

<a id="shelf-03-04"></a>

### SHELF-03-04 RelayShelf 内容与生命周期验收

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

- 旧ID：`P15-04`；状态：`deferred`。
- 具体依赖：[SHELF-03-01](RELAYSHELF.md#shelf-03-01)。
- 门槛：G-GIT（pass）, G-SHELF-API（pending）。
- 外部阻塞：G-SHELF尚未放行；其余前置接口、账号/环境以原生矩阵证据核实，缺失保留NOT_RUN。
- 技术定位：`D15`（T=TECHNICAL_DESIGN、A=ARCHITECTURE、D=DEVELOPMENT_PLAN、PRD=PRD；按实际标题核对）。

允许范围：

- `tests/integration/relayshelf/content*`
- 本任务脱敏证据栏（worker 回交，由总调度登记）

工作：

1. 执行内容格式/编辑/生命周期/回收站/恢复验收。
2. 失败派修复卡。

验收（另适用上方共同检查/提交/审计要求）：

- [ ] 所有矩阵条目有结果。
- [ ] 冲突权限和过期正确。
- [ ] 无WebView。

<a id="shelf-03-05"></a>

### SHELF-03-05 RelayShelf 整理检索验收

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

- 旧ID：`P15-05`；状态：`deferred`。
- 具体依赖：[SHELF-03-04](RELAYSHELF.md#shelf-03-04)。
- 门槛：G-GIT（pass）, G-SHELF-API（pending）。
- 外部阻塞：G-SHELF尚未放行；其余前置接口、账号/环境以原生矩阵证据核实，缺失保留NOT_RUN。
- 技术定位：`D15`（T=TECHNICAL_DESIGN、A=ARCHITECTURE、D=DEVELOPMENT_PLAN、PRD=PRD；按实际标题核对）。

允许范围：

- `tests/integration/relayshelf/search*`
- 本任务脱敏证据栏（worker 回交，由总调度登记）

工作：

1. 执行标签/收藏/过滤/搜索/保存搜索验收。
2. 失败派修复卡。

验收（另适用上方共同检查/提交/审计要求）：

- [ ] 复杂条件和分页不丢。
- [ ] 保存搜索可还原。
- [ ] 角色隔离。

<a id="shelf-03-06"></a>

### SHELF-03-06 RelayShelf 文件传输恢复验收

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

- 旧ID：`P15-06`；状态：`deferred`。
- 具体依赖：[SHELF-03-05](RELAYSHELF.md#shelf-03-05)。
- 门槛：G-GIT（pass）, G-SHELF-API（pending）。
- 外部阻塞：G-SHELF尚未放行；其余前置接口、账号/环境以原生矩阵证据核实，缺失保留NOT_RUN。
- 技术定位：`D15`（T=TECHNICAL_DESIGN、A=ARCHITECTURE、D=DEVELOPMENT_PLAN、PRD=PRD；按实际标题核对）。

允许范围：

- `tests/integration/relayshelf/transfers*`
- 本任务脱敏证据栏（worker 回交，由总调度登记）

工作：

1. 执行分片/续传/校验/去重/下载/预览/打包和URI异常验收。

验收（另适用上方共同检查/提交/审计要求）：

- [ ] 重启/切网/权限失效/文件改变真实验证。
- [ ] 不只正常上传。

<a id="shelf-03-07"></a>

### SHELF-03-07 RelayShelf 分享实时管理验收

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

- 旧ID：`P15-07`；状态：`deferred`。
- 具体依赖：[SHELF-03-06](RELAYSHELF.md#shelf-03-06)。
- 门槛：G-GIT（pass）, G-SHELF-API（pending）。
- 外部阻塞：G-SHELF尚未放行；其余前置接口、账号/环境以原生矩阵证据核实，缺失保留NOT_RUN。
- 技术定位：`D15`（T=TECHNICAL_DESIGN、A=ARCHITECTURE、D=DEVELOPMENT_PLAN、PRD=PRD；按实际标题核对）。

允许范围：

- `tests/integration/relayshelf/sharing*`
- 本任务脱敏证据栏（worker 回交，由总调度登记）

工作：

1. 执行跨用户发送/转发/共享/SSE/管理权限验收。
2. 汇总AC10/11域内报告。

验收（另适用上方共同检查/提交/审计要求）：

- [ ] 无未经批准后端改动。
- [ ] 断线重取正确。
- [ ] 全矩阵可追踪。
