# MoviePilot 剩余任务

P1 最小 PoC 已由用户按缩小范围验收（G-MP pass），正式 P10 全部未开始，P11/AC-07 及发布均未验。唯一运行证据为 v3.1.5，方案A核心文件补丁和助手只适配固定 fingerprint。最低目标 v3.0.0、拟议声明 `>=3.0.0` 不能写成已验证；原 NOT_RUN 和未独立审计缺口保留。完整上下文恢复、自动编辑是可选能力，不扩大已验范围。

PoC 使用临时 HTTP 协议桩、手动更新和附件夹具，不代表正式 Hub/Android 端到端。正式插件须绑定可信 Bot/user/request 身份，跨 Bot 使用命名空间 userid，不能声称宿主按 source 原生隔离；不得硬编码管理员、绕过 MP 业务权限或危险确认。

环境仍 `blocked_repoDigest`：缺实际镜像 RepoDigest、独立隔离实例、完整重建/恢复证据及现场确认；现网恢复快照镜像不等于 Compose 声明，重建可能覆盖补丁。用户已移除原 `/netdisk` 挂载，不擅自恢复或重建；相关功能需另配有效路径。不能据环境缺口撤销已批准的最小 PoC 例外，也不重跑 P1 代替正式任务。

## 执行与证据约定

仅按下面一个叶任务执行，父组仅导航及收口。默认每任务修改3～5个主要实现文件及对应测试；路径是职责边界，优先已有实现，改名先记录等价路径。需要跨职责、产品、架构、安全或外部接口变更，先交用户确认。Bot Hub 只传输、存储、路由；业务权限及危险确认由业务端负责，不执行命令或 MCP。

开始前读取根 `AGENTS.md`、[工程规范](../CONTRIBUTING.md)、本任务定位的技术章节、具体依赖的已集成交接和相关代码；不整包载入旧卡/报告。单次上下文建议≤12k tokens、总窗口占用≤60%，留≥25%交付余量，超出先记录 checkpoint 和剩余工作。

每个叶任务均须运行受影响检查，逐项记录命令、环境、结果；缺条件保留 `blocked` / `NOT_RUN`，不推定通过。独立任务单独本地提交，提供基线SHA、实现SHA、diff摘要、验收证据和遗留问题；独立审计与精确集成后才可标 `done`。验证可以得出负面结论，但不等于兼容门槛通过。只读研究也保留精简证据，禁止凭据/外部源码入库。不顺带实现前后阶段、全模块重构或另一后端，不修改 RelayShelf 后端/数据库或 MP 核心。不得接管调度、派生子agent或改其他任务状态；新增工作先交回拆分。审计核对范围、精确SHA、验收及越权/故障边界，不写代码；问题经单独修复后重审，集成验证最终组合。交接摘要≤800中文字符，完整证据另存。不得以准备报告、源码推断、协议桩、本地或模拟器结果替代外部系统/真机验证。普通准备、开发、测试、审计和集成已持续授权；真实部署、破坏性数据操作、MP重建/升级、远端推送、公开发布或 Release 仍须在具体结果准备后最后确认。

状态、ID、具体依赖及门槛按 `086fa51d79d7726545a9abbab8feebb25770034e` 基线重组冻结；已完成依赖链接 表示[已完成基线](COMPLETED.md)中的历史任务。重要既有例外见[决定摘要](COMPLETED.md#e2)。历史原卡可用 `git show 086fa51d79d7726545a9abbab8feebb25770034e:docs/tasks/cards/旧ID.md` 取证，不恢复旧调度台账。

## 导航

- `MP-01` 环境与重建补证：[MP-01](#mp-01)。
- `MP-02` 正式插件完整生命周期：[MP-02-01](#mp-02-01)、[MP-02-02](#mp-02-02)、[MP-02-03](#mp-02-03)、[MP-02-04](#mp-02-04)、[MP-02-05](#mp-02-05)、[MP-02-06](#mp-02-06)、[MP-02-07](#mp-02-07)、[MP-02-08](#mp-02-08)、[MP-02-09](#mp-02-09)、[MP-02-10](#mp-02-10)。

<a id="mp-01"></a>

### MP-01 收集 MoviePilot 测试环境

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

- 旧ID：`P00-03`；状态：`blocked`。
- 具体依赖：[已完成基线 P00-01](COMPLETED.md#e3)。
- 门槛：无额外冻结门槛。
- 外部阻塞：缺RepoDigest、完整重建证据、独立隔离实例及当前现场确认；不撤销P1用户验收例外
- 技术定位：`T10;D1`（T=TECHNICAL_DESIGN、A=ARCHITECTURE、D=DEVELOPMENT_PLAN、PRD=PRD；按实际标题核对）。

允许范围：

- 本任务脱敏证据栏（worker 回交，由总调度登记）

工作：

1. 核对 v3.1.5 commit、测试镜像 digest 和配套前端。
2. 登记独立测试实例与访问方式，不记录凭据。

验收（另适用上方共同检查/提交/审计要求）：

- [ ] 版本可复现。
- [ ] 测试实例与现网隔离。
- [ ] 缺失条件逐项列出。

<a id="mp-02-01"></a>

### MP-02-01 正式 MP 插件配置与生命周期

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

- 旧ID：`P10-01`；状态：`todo`。
- 具体依赖：[已完成基线 P01-06](COMPLETED.md#mp1)、[BE-08-02](BACKEND.md#be-08-02)。
- 门槛：G-GIT（pass）, G-MP（pass）。
- 外部阻塞：无新增独立外部阻塞；依赖、门槛或所需真实环境未满足时不得标通过。
- 技术定位：`T10`（T=TECHNICAL_DESIGN、A=ARCHITECTURE、D=DEVELOPMENT_PLAN、PRD=PRD；按实际标题核对）。

允许范围：

- `integrations/moviepilot/plugin/**`

工作：

1. 依已通过PoC创建正式插件壳、Bot/MP身份绑定和启动/停止。
2. 不复制实验桩进生产。

验收（另适用上方共同检查/提交/审计要求）：

- [ ] 精确配置命中。
- [ ] 停止/重载无线程残留。
- [ ] 不硬编码管理员权限。

<a id="mp-02-02"></a>

### MP-02-02 插件 HubClient 与协议错误

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

- 旧ID：`P10-02`；状态：`todo`。
- 具体依赖：[MP-02-01](MOVIEPILOT.md#mp-02-01)、[已完成基线 P03-07](COMPLETED.md#e1)。
- 门槛：G-GIT（pass）, G-MP（pass）。
- 外部阻塞：无新增独立外部阻塞；依赖、门槛或所需真实环境未满足时不得标通过。
- 技术定位：`T3,4,10`（T=TECHNICAL_DESIGN、A=ARCHITECTURE、D=DEVELOPMENT_PLAN、PRD=PRD；按实际标题核对）。

允许范围：

- `integrations/moviepilot/plugin/hub_client*`

工作：

1. 实现REST/claim/ACK/renew/上传下载的有界客户端及错误分类。

验收（另适用上方共同检查/提交/审计要求）：

- [ ] 凭据不入日志。
- [ ] 网络超时复用幂等。
- [ ] 撤销/停用不死循环。

<a id="mp-02-03"></a>

### MP-02-03 插件持久 Inbox 和必要附件

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

- 旧ID：`P10-03`；状态：`todo`。
- 具体依赖：[MP-02-02](MOVIEPILOT.md#mp-02-02)。
- 门槛：G-GIT（pass）, G-MP（pass）。
- 外部阻塞：无新增独立外部阻塞；依赖、门槛或所需真实环境未满足时不得标通过。
- 技术定位：`T4.2,10.1`（T=TECHNICAL_DESIGN、A=ARCHITECTURE、D=DEVELOPMENT_PLAN、PRD=PRD；按实际标题核对）。

允许范围：

- `integrations/moviepilot/plugin/inbox*`
- `integrations/moviepilot/plugin/storage*`

工作：

1. 实现持久事件/附件后ACK、去重和恢复，先不执行业务。

验收（另适用上方共同检查/提交/审计要求）：

- [ ] ACK前崩溃重收。
- [ ] ACK后Hub清理仍有执行数据。
- [ ] event_id稳定。

<a id="mp-02-04"></a>

### MP-02-04 插件发送 Outbox 与 ID 映射

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

- 旧ID：`P10-04`；状态：`todo`。
- 具体依赖：[MP-02-03](MOVIEPILOT.md#mp-02-03)。
- 门槛：G-GIT（pass）, G-MP（pass）。
- 外部阻塞：无新增独立外部阻塞；依赖、门槛或所需真实环境未满足时不得标通过。
- 技术定位：`T10.1`（T=TECHNICAL_DESIGN、A=ARCHITECTURE、D=DEVELOPMENT_PLAN、PRD=PRD；按实际标题核对）。

允许范围：

- `integrations/moviepilot/plugin/outbox*`
- `integrations/moviepilot/plugin/mapping*`

工作：

1. 实现回复发送持久待办、稳定键和Hub↔MP上下文ID。

验收（另适用上方共同检查/提交/审计要求）：

- [ ] 回复超时不重复。
- [ ] 重启映射可用。
- [ ] 两Bot/设备不串上下文。

<a id="mp-02-05"></a>

### MP-02-05 插件文本入站与原生业务权限

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

- 旧ID：`P10-05`；状态：`todo`。
- 具体依赖：[MP-02-04](MOVIEPILOT.md#mp-02-04)。
- 门槛：G-GIT（pass）, G-MP（pass）。
- 外部阻塞：无新增独立外部阻塞；依赖、门槛或所需真实环境未满足时不得标通过。
- 技术定位：`T10.1`（T=TECHNICAL_DESIGN、A=ARCHITECTURE、D=DEVELOPMENT_PLAN、PRD=PRD；按实际标题核对）。

允许范围：

- `integrations/moviepilot/plugin/ingress*`

工作：

1. 按PoC入口将文本/引用/附件注入MP原有消息链。

验收（另适用上方共同检查/提交/审计要求）：

- [ ] 身份绑定明确。
- [ ] 非管理员被MP拒绝。
- [ ] 不让Hub解析命令。

<a id="mp-02-06"></a>

### MP-02-06 插件主动通知与定向回复

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

- 旧ID：`P10-06`；状态：`todo`。
- 具体依赖：[MP-02-05](MOVIEPILOT.md#mp-02-05)。
- 门槛：G-GIT（pass）, G-MP（pass）。
- 外部阻塞：无新增独立外部阻塞；依赖、门槛或所需真实环境未满足时不得标通过。
- 技术定位：`T10`（T=TECHNICAL_DESIGN、A=ARCHITECTURE、D=DEVELOPMENT_PLAN、PRD=PRD；按实际标题核对）。

允许范围：

- `integrations/moviepilot/plugin/notifications*`

工作：

1. 实现普通同步/异步主动通知和指定来源回复的唯一转发入口。

验收（另适用上方共同检查/提交/审计要求）：

- [ ] 管理员/用户/广播路径不漏不重。
- [ ] 其他渠道非回归。

<a id="mp-02-07"></a>

### MP-02-07 插件候选列表与 callback

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

- 旧ID：`P10-07`；状态：`todo`。
- 具体依赖：[MP-02-06](MOVIEPILOT.md#mp-02-06)。
- 门槛：G-GIT（pass）, G-MP（pass）。
- 外部阻塞：无新增独立外部阻塞；依赖、门槛或所需真实环境未满足时不得标通过。
- 技术定位：`T10`（T=TECHNICAL_DESIGN、A=ARCHITECTURE、D=DEVELOPMENT_PLAN、PRD=PRD；按实际标题核对）。

允许范围：

- `integrations/moviepilot/plugin/candidates*`
- `integrations/moviepilot/plugin/callback*`

工作：

1. 分别实现媒体/种子候选映射及透明回调、引用上下文。

验收（另适用上方共同检查/提交/审计要求）：

- [ ] 能力路径经验证。
- [ ] callback重投不跳业务确认。
- [ ] 两Bot隔离。

<a id="mp-02-08"></a>

### MP-02-08 插件图片文件转换与上传

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

- 旧ID：`P10-08`；状态：`todo`。
- 具体依赖：[MP-02-07](MOVIEPILOT.md#mp-02-07)。
- 门槛：G-GIT（pass）, G-MP（pass）。
- 外部阻塞：无新增独立外部阻塞；依赖、门槛或所需真实环境未满足时不得标通过。
- 技术定位：`T10`（T=TECHNICAL_DESIGN、A=ARCHITECTURE、D=DEVELOPMENT_PLAN、PRD=PRD；按实际标题核对）。

允许范围：

- `integrations/moviepilot/plugin/media*`

工作：

1. 实现合法读取MP附件字节后上传Hub。
2. 将授权图片文件绑定回复。

验收（另适用上方共同检查/提交/审计要求）：

- [ ] 内网URL不当持久附件。
- [ ] 下载/上传失败显式反馈。
- [ ] 无Hub远程抓取。

<a id="mp-02-09"></a>

### MP-02-09 插件包与恢复兼容验收

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

- 旧ID：`P10-09`；状态：`todo`。
- 具体依赖：[MP-02-08](MOVIEPILOT.md#mp-02-08)、[AND-03-05](ANDROID.md#and-03-05)、[MP-02-10](MOVIEPILOT.md#mp-02-10)。
- 门槛：G-GIT（pass）, G-MP（pass）。
- 外部阻塞：无新增独立外部阻塞；依赖、门槛或所需真实环境未满足时不得标通过。
- 技术定位：`D10`（T=TECHNICAL_DESIGN、A=ARCHITECTURE、D=DEVELOPMENT_PLAN、PRD=PRD；按实际标题核对）。

允许范围：

- `integrations/moviepilot/**`
- 本任务脱敏证据栏（worker 回交，由总调度登记）

工作：

1. 准备可安装包和说明。
2. 执行Hub/MP重启/停用/重置/断网恢复矩阵。

验收（另适用上方共同检查/提交/审计要求）：

- [ ] AC-07全链路。
- [ ] 其他渠道不受影响。
- [ ] 业务幂等窗口明确。
- [ ] 只修包装不扩业务。

<a id="mp-02-10"></a>

### MP-02-10 插件直接发送 ID 与编辑映射

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

- 旧ID：`P10-10`；状态：`todo`。
- 具体依赖：[MP-02-08](MOVIEPILOT.md#mp-02-08)。
- 门槛：G-GIT（pass）, G-MP（pass）。
- 外部阻塞：无新增独立外部阻塞；依赖、门槛或所需真实环境未满足时不得标通过。
- 技术定位：`T10`（T=TECHNICAL_DESIGN、A=ARCHITECTURE、D=DEVELOPMENT_PLAN、PRD=PRD；按实际标题核对）。

允许范围：

- `integrations/moviepilot/plugin/edit*`
- `integrations/moviepilot/plugin/mapping*`

工作：

1. 实现直接发送返回Hub与MP ID映射。
2. 编辑转换到Hub CAS。

验收（另适用上方共同检查/提交/审计要求）：

- [ ] 重启后编辑找对消息。
- [ ] 重复编辑和冲突有结果。
- [ ] 两Bot不串映射。
