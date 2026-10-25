# Android 剩余任务

推送分支依赖说明：原 BE-10-03 父依赖字段保留；下游只做框架／FCM 的准备或验证时，执行包必须明确消费 [BE-10-03-FCM](BACKEND.md#be-10-03-fcm) 精确独审已集成子结果，OPPO 外部阻塞不传播至 FCM。全范围最终验收仍须两通道分别有证据，OPPO 缺条件保留 blocked／NOT_RUN，不能以 FCM 代验或将父项标完成。

本文件是本模块未完成步骤与门槛的状态源。功能组只供导航，一次派发一个叶步骤、一个有限可验证目标；默认 3～5 个主要实现文件及对应测试，目录范围不是整包开发授权。依赖指最终独立审计通过且精确集成；开工可先做不依赖运行端点的界面准备，完整验收必须等待下列依赖和收口条件。

需求、架构、接口仍以 [PRD](../PRD.md)、[架构](../ARCHITECTURE.md)、[技术设计](../TECHNICAL_DESIGN.md) 及已冻结 OpenAPI（按现有契约目录定位） 为准。按标题定位局部，不整本加载；先读 [工程规范](../CONTRIBUTING.md) 和 [工作流](WORKFLOW.md)。初始上下文≤12k tokens、总窗口≤60%，保留≥25%交付余量，超预算先给具体 checkpoint。

下面状态由基线 `086fa51d79d7726545a9abbab8feebb25770034e` 迁移，无活跃 worker；旧 running 只表示已有未闭环草稿。尚无本步骤新实现/独审/集成结果；调度派发时补 owner、工作树/分支、固定基线 SHA、实际写入路径，交付时补检查命令/环境/结果、实现 SHA、独审结论、精确集成 SHA 与遗留项。不得把协议桩、模拟器或未执行项目记为真机/真实服务通过。已完成基线只见 [完成记录](COMPLETED.md)，不重派。

每步均须运行受影响检查；缺条件记 NOT_RUN/blocked 并指出具体分支，不将报告交付等同验收通过。实现、不同实施者的独立审计、精确集成全部完成才可 done；发现问题编号修复后重审。只改授权路径及本步骤精简证据，不擅自改任务状态或契约。本文重组只需链接、ID/依赖覆盖与原验收检查，不需业务测试。

## 已完成基线与共同约束

原生工程及固定工具链、HTTPS 网络与安装身份、加密会话存储、登录 UI 的局部组合均已完成（旧 `P03-03`、`P08-04-S01～S03`，见[完成记录 A1/E1](COMPLETED.md)）；它们不代表完整登录父功能、新 G1 或真实安全存储验收。包名 `top.jianxinli.relaynest`、minSdk 29、单一 `:app`、mise/Gradle/catalog/现有锁定版本保持既有值，不借任务升级。旧 `android/core/**`、`android/feature/**` 为职责规划路径，对应 `android/app/src/main/kotlin/top/jianxinli/relaynest/` 下同名包；已有布局优先，等价映射须登记。

所有受保护业务请求、未来 WS 握手与后台补偿复用 AND-01 的实例门禁；Room/缓存/草稿/outbox 按 endpoint + instance_id 隔离，不自动迁移或显式重登后重放旧实例内容。安装 UUID、URL、证书、Session.id 均不能冒充实例身份。

G1 及实例生命周期、两附件 Bot-first 狭窄例外已批准，不重复询问。实例随机公开 UUID 非秘密；仅保证诚实替换/误连，接受 probe 后瞬切服务仍可能让旧 Bearer/密码字节到达新 B，不宣称 G2 零泄露。重启/无回滚原库停机迁移保留 ID；恢复/克隆先停服换 ID、全撤管理员 session/推送映射；清库新 ID，初始化 ID 缺坏则 fail-close。两附件仅在确定 NoMatch 后先实例 ID 再管理员认证；Matched 拒绝不得 fallback，Unavailable 为 503，缺 ID 400/错 ID 409 优先于无效凭据 401。客户端消费冻结契约，不自行实现后端策略。

Android 实施步骤默认使用既有环境运行相关单测及 `cd android && mise exec -- ./gradlew :app:lintDebug :app:assembleDebug --offline`；真实服务/设备的验证按各步单列。实际 Keystore、真实 TLS、真机组合及成功 release 签名未完成，用户已决定真机放最终验收，不重复索要 ADB；NAS/OPPO条件只阻相关分支。

## 门槛

- G-GIT 已通过；G-API 与 G-INSTANCE-CONTRACT 已限定通过，见[完成记录 E1/E2](COMPLETED.md)。G1 当前冻结契约源 `09deddba6ace42261c227b57254f8ae9ebbd1a2a`，精确集成 `f72a11c147269b4c81f0e319998292a3228a83f9`；旧 39/115/1229 证据不替代新 40/118/2455 契约，契约通过不等于 runtime。
- **G-INSTANCE-RUNTIME：pending**，AND-01-R 实际运行专项独审/精确集成后才可判定。
- **G-INSTANCE-COMPOSITION：pending**，AND-01-V 与 AND-01-S 的真实服务组合及保留最终条目完成后才可判定，不以子步骤 done 自动放行。
- G-FCM 限定 pass：一加 Ace 3 Pro / Android 16 / ColorOS 16 data-only 提示有证据；WorkManager 仅调度/模拟落盘，非真实 Hub 最终同步或 AC-09。历史证据见[完成记录 A2](COMPLETED.md)。
- **G-OPPO：pending**，资格/应用权限/审核尚未准备，只阻 AND-06、AND-05 的厂商适配和相关真机验收。通用框架/FCM可继续。

## 功能导航

AND-01 登录与安全会话；AND-02 本地数据/同步；AND-03 消息/附件/已读；AND-04 管理；AND-05 通知；AND-06 OPPO 外部 PoC。AND-01 是聚合父项，不整体派发；其余组只是阅读导航。


<a id="and-01"></a>

## AND-01 Android Hub 登录与安全会话

- 完成门槛角色：`G-INSTANCE-RUNTIME` 须通过后父项／消费者才能收口。

- 完成门槛角色：`G-INSTANCE-COMPOSITION` 须通过后父项／消费者才能收口。

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

设计定位：T7,9（T=TECHNICAL_DESIGN，A=ARCHITECTURE，D=DEVELOPMENT_PLAN；按标题核对）。

- 旧 ID：`P08-04`；类型：父聚合，仅汇总；状态：开发中（已有局部基础/草稿，尚未闭环，无活跃 worker）。
- 依赖（父项为完整收口依赖，叶步骤为开工依赖）：[已完成基线 P03-03](COMPLETED.md#e1)（Android 最小原生工程）；[已完成基线 P04-05](COMPLETED.md#g1)（设备登记会话列表与撤销）；[AND-01-S](ANDROID.md#and-01-s)（完整设备撤销与登录父收口）；[AND-01-V](ANDROID.md#and-01-v)（身份专项组合验收）。
- 必需门槛：`G-GIT`。

仅聚合，状态为开发中（已有局部基础/草稿，未闭环）。完整 BE-01 → N → K → R → V → S 为独立链，父只汇总 S/V 与父原验收，不要求 N/K/R 依赖父，避免环。

### 允许修改范围

- `android/core/network/**`
- `android/core/security/**`
- `android/feature/auth/**`

### 本任务工作

1. 实现地址/登录/Bearer 加密保存/注销/撤销与部署实例隔离。

### 测试与完成标准

- [ ] 核心版无 RelayShelf 强制配置。
- [ ] 自定义端口。
- [ ] 不明文保存密码。

### PLAN01 依赖映射（不替代上述原目标与验收）

基础卡 [S01](COMPLETED.md)（旧 `P08-04-S01`） → [S02](COMPLETED.md)（旧 `P08-04-S02`） → [S03](COMPLETED.md)（旧 `P08-04-S03`），每条边均要求前卡独审并精确集成。基础卡只使用已集成 P03-03、P04-04、P04-05-S01 三端点，可与完整 P04-05 设备分支并行；不将三端点或 S01 公共安全等同完整 P04-05。

[S04](ANDROID.md#and-01-s) 等待 S03、P03-03、**完整 P04-05**、部署隔离决策及对应获准工作，承接真实服务远程撤销和父卡组合验收。父卡继续保留原 P03-03 + 完整 P04-05 依赖，并汇总 S04；不得因拆卡删除原验收。

路径映射：`android/core/network/**`、`android/core/security/**`、`android/feature/auth/**` 分别落在 `android/app/src/main/kotlin/top/jianxinli/relaynest/` 下同名包，继续单一 `:app`。S01 拥有最小 Manifest 网络权限、OkHttp 既定选型的 Gradle/catalog/必需新增锁条目；S03 在 S01/S02 集成后拥有 MainActivity 及必要现有生命周期依赖声明；S04 最后拥有测试 runner 接线。既有工具链、minSdk29、包名、mise 和已有锁定版本不升级。

注销、撤销、部署实例隔离持续属于父目标。地址绑定不能证明同地址部署替换隔离；原 PLAN01 时 DTO 无部署 ID；当前按已冻结 G1 扩展接入，仍不能以 Session.id、安装 UUID、URL 或 TLS 证书冒充实例身份。详见 PLAN01（历史证据见[COMPLETED](COMPLETED.md)，按旧 ID 检索） 的决策门槛与逐项验收映射。

必跑检查：本步骤相关 Android 单测、离线 lintDebug/assembleDebug；上列真实服务或真机组合按条执行。 记录实际命令、环境、结果与未测边界，遵循本文件共同交付规则。

<a id="and-02-01"></a>

## AND-02-01 Android Room 缓存与版本模型

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

设计定位：T6,9（T=TECHNICAL_DESIGN，A=ARCHITECTURE，D=DEVELOPMENT_PLAN；按标题核对）。

- 旧 ID：`P08-05`；类型：可单独派发的叶步骤；状态：未开始。
- 依赖（父项为完整收口依赖，叶步骤为开工依赖）：[AND-01](ANDROID.md#and-01)（Android Hub 登录与安全会话）。
- 必需门槛：`G-GIT`。

### 允许修改范围

- `android/core/database/**`

### 本任务工作

1. 实现消息/Bot/版本/游标/本地 generation 与 outbox 基础表，不做网络同步。

### 测试与完成标准

- [ ] Room 迁移/事务。
- [ ] 序号精度。
- [ ] 不同部署隔离。
- [ ] 未发送数据独立保存。

必跑检查：本步骤相关 Android 单测、离线 lintDebug/assembleDebug；上列真实服务或真机组合按条执行。 记录实际命令、环境、结果与未测边界，遵循本文件共同交付规则。

<a id="and-02-02"></a>

## AND-02-02 Android 增量同步协调器

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

设计定位：T6.1,9（T=TECHNICAL_DESIGN，A=ARCHITECTURE，D=DEVELOPMENT_PLAN；按标题核对）。

- 旧 ID：`P08-06`；类型：可单独派发的叶步骤；状态：未开始。
- 依赖（父项为完整收口依赖，叶步骤为开工依赖）：[AND-02-01](ANDROID.md#and-02-01)（Android Room 缓存与版本模型）；[BE-09-01](BACKEND.md#be-09-01)（Hub 增量同步服务端）。
- 必需门槛：`G-GIT`。

### 允许修改范围

- `android/core/sync/**`

### 本任务工作

1. 实现单飞 REST 拉取、版本合并、批次/游标原子落库和取消。

### 测试与完成标准

- [ ] 中途崩溃可重放。
- [ ] 旧 revision no-op。
- [ ] 删除/已读同步正确。

必跑检查：本步骤相关 Android 单测、离线 lintDebug/assembleDebug；上列真实服务或真机组合按条执行。 记录实际命令、环境、结果与未测边界，遵循本文件共同交付规则。

<a id="and-02-03"></a>

## AND-02-03 Android bootstrap 与历史缺口

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

设计定位：T6.2（T=TECHNICAL_DESIGN，A=ARCHITECTURE，D=DEVELOPMENT_PLAN；按标题核对）。

- 旧 ID：`P08-07`；类型：可单独派发的叶步骤；状态：未开始。
- 依赖（父项为完整收口依赖，叶步骤为开工依赖）：[AND-02-02](ANDROID.md#and-02-02)（Android 增量同步协调器）；[BE-09-02](BACKEND.md#be-09-02)（Hub bootstrap 重建协议）。
- 必需门槛：`G-GIT`。

### 允许修改范围

- `android/core/sync/**`
- `android/feature/sync/**`

### 本任务工作

1. 实现410/epoch重建、generation切换/缺口提示，保留outbox。

### 测试与完成标准

- [ ] 重建分页/追赶/到期重试。
- [ ] 并发删除。
- [ ] 旧缓存不残留。
- [ ] 不批量响铃。

必跑检查：本步骤相关 Android 单测、离线 lintDebug/assembleDebug；上列真实服务或真机组合按条执行。 记录实际命令、环境、结果与未测边界，遵循本文件共同交付规则。

<a id="and-02-04"></a>

## AND-02-04 Android WS 前台生命周期

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

设计定位：T3.3,9（T=TECHNICAL_DESIGN，A=ARCHITECTURE，D=DEVELOPMENT_PLAN；按标题核对）。

- 旧 ID：`P08-08`；类型：可单独派发的叶步骤；状态：未开始。
- 依赖（父项为完整收口依赖，叶步骤为开工依赖）：[AND-02-02](ANDROID.md#and-02-02)（Android 增量同步协调器）；[BE-09-03](BACKEND.md#be-09-03)（Hub WS 提示与连接撤销）。
- 必需门槛：`G-GIT`。

### 允许修改范围

- `android/core/network/ws/**`
- `android/core/sync/**`

### 本任务工作

1. 实现连接提示/重连退避、前后台启停与打开补偿入口。

### 测试与完成标准

- [ ] WS 提示不推进游标。
- [ ] 连接窗口无漏。
- [ ] 进入后台释放连接。

必跑检查：本步骤相关 Android 单测、离线 lintDebug/assembleDebug；上列真实服务或真机组合按条执行。 记录实际命令、环境、结果与未测边界，遵循本文件共同交付规则。

<a id="and-03-01"></a>

## AND-03-01 Android 本地发送 outbox

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

设计定位：T9;T2.4（T=TECHNICAL_DESIGN，A=ARCHITECTURE，D=DEVELOPMENT_PLAN；按标题核对）。

- 旧 ID：`P08-09`；类型：可单独派发的叶步骤；状态：未开始。
- 依赖（父项为完整收口依赖，叶步骤为开工依赖）：[AND-02-02](ANDROID.md#and-02-02)（Android 增量同步协调器）；[BE-02](BACKEND.md#be-02)（消息幂等创建和接收回执）。
- 必需门槛：`G-GIT`。

必须等待 BE-02 的 Bot 与 Android 管理员正式 HTTP 都独审集成，不能以已完成存储或协议桩替代真实接收/幂等回执。未知结果与已接收、业务成功分开。

### 允许修改范围

- `android/core/outbox/**`
- `android/core/database/**`

### 本任务工作

1. 实现稳定幂等/本地状态/超时未知恢复和服务端 ID 合并。

### 测试与完成标准

- [ ] 杀进程/响应丢失不重复。
- [ ] 会话撤销停止发送。
- [ ] 过期未知键不自动重放。

必跑检查：本步骤相关 Android 单测、离线 lintDebug/assembleDebug；上列真实服务或真机组合按条执行。 记录实际命令、环境、结果与未测边界，遵循本文件共同交付规则。

<a id="and-03-02"></a>

## AND-03-02 Android 附件上传下载与预览

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

设计定位：T9;T5.3（T=TECHNICAL_DESIGN，A=ARCHITECTURE，D=DEVELOPMENT_PLAN；按标题核对）。

- 旧 ID：`P08-10`；类型：可单独派发的叶步骤；状态：未开始。
- 依赖（父项为完整收口依赖，叶步骤为开工依赖）：[AND-03-01](ANDROID.md#and-03-01)（Android 本地发送 outbox）；[BE-05-03](BACKEND.md#be-05-03)（附件下载 Range 和安全响应）。
- 必需门槛：`G-GIT`。

### 允许修改范围

- `android/core/transfers/**`
- `android/feature/attachments/**`

### 本任务工作

1. 实现 URI 选择/权限、上传幂等、Range下载、受控缓存和采样图片预览。

### 测试与完成标准

- [ ] 无路径假设。
- [ ] 权限丢失/文件改变/断网可恢复。
- [ ] 不一次载入大文件。

必跑检查：本步骤相关 Android 单测、离线 lintDebug/assembleDebug；上列真实服务或真机组合按条执行。 记录实际命令、环境、结果与未测边界，遵循本文件共同交付规则。

<a id="and-03-03"></a>

## AND-03-03 Android Bot 列表与消息浏览

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

设计定位：T9;PRD4.1（T=TECHNICAL_DESIGN，A=ARCHITECTURE，D=DEVELOPMENT_PLAN；按标题核对）。

- 旧 ID：`P08-11`；类型：可单独派发的叶步骤；状态：未开始。
- 依赖（父项为完整收口依赖，叶步骤为开工依赖）：[AND-02-02](ANDROID.md#and-02-02)（Android 增量同步协调器）；[BE-04-01](BACKEND.md#be-04-01)（历史分页引用和共享已读）。
- 必需门槛：`G-GIT`。

### 允许修改范围

- `android/feature/bots/**`
- `android/feature/chat/**`

### 本任务工作

1. 实现 Bot 会话/未读、历史分页、Markdown/图片/引用失效展示。

### 测试与完成标准

- [ ] 无原始HTML执行。
- [ ] 链接协议过滤。
- [ ] 编辑不新增未读。
- [ ] 加载/空/错误状态。

必跑检查：本步骤相关 Android 单测、离线 lintDebug/assembleDebug；上列真实服务或真机组合按条执行。 记录实际命令、环境、结果与未测边界，遵循本文件共同交付规则。

<a id="and-03-04"></a>

## AND-03-04 Android 发送回复与按钮交互

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

设计定位：T2.3,2.4,9（T=TECHNICAL_DESIGN，A=ARCHITECTURE，D=DEVELOPMENT_PLAN；按标题核对）。

- 旧 ID：`P08-12`；类型：可单独派发的叶步骤；状态：未开始。
- 依赖（父项为完整收口依赖，叶步骤为开工依赖）：[AND-03-01](ANDROID.md#and-03-01)（Android 本地发送 outbox）；[AND-03-02](ANDROID.md#and-03-02)（Android 附件上传下载与预览）；[AND-03-03](ANDROID.md#and-03-03)（Android Bot 列表与消息浏览）；[BE-04-02](BACKEND.md#be-04-02)（应用消息 CAS 编辑）；[BE-04-05](BACKEND.md#be-04-05)（按钮回调独立事件与投递）。
- 必需门槛：`G-GIT`。

### 允许修改范围

- `android/feature/chat/**`

### 本任务工作

1. 接入发送/引用/附件/callback、单向 Bot 限制和准确状态文案。

### 测试与完成标准

- [ ] 旧按钮409刷新。
- [ ] 点击重试复用键。
- [ ] 不把 ACK 显示业务成功。
- [ ] 无编辑已发入口。

必跑检查：本步骤相关 Android 单测、离线 lintDebug/assembleDebug；上列真实服务或真机组合按条执行。 记录实际命令、环境、结果与未测边界，遵循本文件共同交付规则。

<a id="and-04-01"></a>

## AND-04-01 Android Bot 与凭据管理

- 完成追加依赖（不改变上述开工依赖）：[BE-03-01](BACKEND.md#be-03-01)、[BE-03-03](BACKEND.md#be-03-03)；基础准备可先进行，真实全范围收口必须满足。

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

设计定位：D8;PRD3.1（T=TECHNICAL_DESIGN，A=ARCHITECTURE，D=DEVELOPMENT_PLAN；按标题核对）。

- 旧 ID：`P08-13`；类型：可单独派发的叶步骤；状态：未开始。
- 依赖（父项为完整收口依赖，叶步骤为开工依赖）：[AND-01](ANDROID.md#and-01)（Android Hub 登录与安全会话）；[已完成基线 P04-07](COMPLETED.md#g1)（Bot 启停重置与删除基础事务）。
- 必需门槛：`G-GIT`。

允许先做不依赖正式端点的 UI 准备；完整生命周期收口还须 [BE-03-01](BACKEND.md#be-03-01) 正式重置/删除 HTTP 与 [BE-03-03](BACKEND.md#be-03-03) 队列联动独审集成。[已完成基线 P04-07](COMPLETED.md#g1) 只是基础，不能据此宣称正式后端已齐备。

### 允许修改范围

- `android/feature/management/bots/**`

### 本任务工作

1. 实现 Bot CRUD/启停/二次删除/两种凭据重置。

### 测试与完成标准

- [ ] 与 Web 同生命周期。
- [ ] 明文一次显示。
- [ ] 版本冲突和撤销响应正确。

必跑检查：本步骤相关 Android 单测、离线 lintDebug/assembleDebug；上列真实服务或真机组合按条执行。 记录实际命令、环境、结果与未测边界，遵循本文件共同交付规则。

<a id="and-04-02"></a>

## AND-04-02 Android 全局配置管理

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

设计定位：D8（T=TECHNICAL_DESIGN，A=ARCHITECTURE，D=DEVELOPMENT_PLAN；按标题核对）。

- 旧 ID：`P08-14`；类型：可单独派发的叶步骤；状态：未开始。
- 依赖（父项为完整收口依赖，叶步骤为开工依赖）：[AND-04-01](ANDROID.md#and-04-01)（Android Bot 与凭据管理）；[BE-08-01](BACKEND.md#be-08-01)（配置资源控制和系统健康投影）。
- 必需门槛：`G-GIT`。

冻结政策：`retention_days`、`image_max_bytes`、`file_max_bytes` 均为 1～9223372036854775807 的规范十进制字符串，禁止 0、符号、前导零、小数、指数、JSON 数字；默认分别 `"30"`、`"10485760"`、`"52428800"`（30 天/10 MiB/50 MiB），不得另设默认值构成的业务上限。专用全局设置 `version`/`expected_version` 是 JSON integer，范围 1～9007199254740991；与配置值 int64 字符串及公共 Version/设备版本区别处理，不改公共类型。CAS 过期/耗尽 409、不回绕；保存成功（含同值写入）增版本，取消/失败保留输入，缩短保留期限明确影响。

### 允许修改范围

- `android/feature/management/settings/**`

### 本任务工作

1. 实现全局配置及版本保存。
2. 缩短期限提示和错误恢复。

### 测试与完成标准

- [ ] 权限与Web一致。
- [ ] CAS冲突不覆盖。
- [ ] 取消修改无副作用。

必跑检查：本步骤相关 Android 单测、离线 lintDebug/assembleDebug；上列真实服务或真机组合按条执行。 记录实际命令、环境、结果与未测边界，遵循本文件共同交付规则。

<a id="and-03-05"></a>

## AND-03-05 Android 共享已读与双设备回归

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

设计定位：T6.3;D8（T=TECHNICAL_DESIGN，A=ARCHITECTURE，D=DEVELOPMENT_PLAN；按标题核对）。

- 旧 ID：`P08-15`；类型：可单独派发的叶步骤；状态：未开始。
- 依赖（父项为完整收口依赖，叶步骤为开工依赖）：[AND-02-03](ANDROID.md#and-02-03)（Android bootstrap 与历史缺口）；[AND-02-04](ANDROID.md#and-02-04)（Android WS 前台生命周期）；[AND-03-04](ANDROID.md#and-03-04)（Android 发送回复与按钮交互）；[AND-04-02](ANDROID.md#and-04-02)（Android 全局配置管理）；[AND-04-03](ANDROID.md#and-04-03)（Android 设备会话管理）；[AND-04-04](ANDROID.md#and-04-04)（Android 失败队列管理）。
- 必需门槛：`G-GIT`。

### 允许修改范围

- `android/**/src/*Test/**`
- 本步骤脱敏交接证据（由调度汇入本文件）

### 本任务工作

1. 集成实际可见消息 read_seq 提交，补双设备/重建/撤销回归。
2. 缺陷另开修复卡。

### 测试与完成标准

- [ ] 已读 max 不倒退。
- [ ] 缓存/游标/设置独立。
- [ ] 两设备同一部署不串任务。

必跑检查：本步骤相关 Android 单测、离线 lintDebug/assembleDebug；上列真实服务或真机组合按条执行。 记录实际命令、环境、结果与未测边界，遵循本文件共同交付规则。

<a id="and-04-03"></a>

## AND-04-03 Android 设备会话管理

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

设计定位：D8;T7（T=TECHNICAL_DESIGN，A=ARCHITECTURE，D=DEVELOPMENT_PLAN；按标题核对）。

- 旧 ID：`P08-16`；类型：可单独派发的叶步骤；状态：未开始。
- 依赖（父项为完整收口依赖，叶步骤为开工依赖）：[AND-04-01](ANDROID.md#and-04-01)（Android Bot 与凭据管理）；[已完成基线 P04-05](COMPLETED.md#g1)（设备登记会话列表与撤销）。
- 必需门槛：`G-GIT`。

### 允许修改范围

- `android/feature/management/devices/**`

### 本任务工作

1. 实现设备列表/远程撤销和单设备退出。
2. 会话失效处理。

### 测试与完成标准

- [ ] 注销不影响其他设备。
- [ ] Token不展示。
- [ ] 当前会话撤销停止网络写入。

必跑检查：本步骤相关 Android 单测、离线 lintDebug/assembleDebug；上列真实服务或真机组合按条执行。 记录实际命令、环境、结果与未测边界，遵循本文件共同交付规则。

<a id="and-04-04"></a>

## AND-04-04 Android 失败队列管理

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

设计定位：D8;T4（T=TECHNICAL_DESIGN，A=ARCHITECTURE，D=DEVELOPMENT_PLAN；按标题核对）。

- 旧 ID：`P08-17`；类型：可单独派发的叶步骤；状态：未开始。
- 依赖（父项为完整收口依赖，叶步骤为开工依赖）：[AND-04-01](ANDROID.md#and-04-01)（Android Bot 与凭据管理）；[BE-06-04](BACKEND.md#be-06-04)（失败元数据列表和手动重试）。
- 必需门槛：`G-GIT`。

### 允许修改范围

- `android/feature/management/deliveries/**`

### 本任务工作

1. 实现失败元数据列表和幂等手动重试。
2. 可关联Android已有原消息。

### 测试与完成标准

- [ ] 重试不重跑业务。
- [ ] 网络重试不增代次。
- [ ] 失效消息明确显示。

必跑检查：本步骤相关 Android 单测、离线 lintDebug/assembleDebug；上列真实服务或真机组合按条执行。 记录实际命令、环境、结果与未测边界，遵循本文件共同交付规则。

<a id="and-05-01"></a>

## AND-05-01 Android 注册和 FCM 提示入口

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

设计定位：T9.1（T=TECHNICAL_DESIGN，A=ARCHITECTURE，D=DEVELOPMENT_PLAN；按标题核对）。

- 旧 ID：`P09-04`；类型：可单独派发的叶步骤；状态：未开始。
- 依赖（父项为完整收口依赖，叶步骤为开工依赖）：[AND-02-04](ANDROID.md#and-02-04)（Android WS 前台生命周期）；[BE-10-01](BACKEND.md#be-10-01)（Hub 推送注册与设备通知偏好）；[已完成基线 P02-04](COMPLETED.md#a2)（形成后台可用性门槛结论）。
- 必需门槛：`G-FCM`、`G-GIT`。

FCM 为 data-only 提示，不携正文、附件 URL 或凭据；HIGH 仅用于符合规则的用户可见及时提醒，不全量提升。FCM provider 独立闭环；OPPO 是显式可选分支，须 AND-06/G-OPPO 实证后再派发，不把未有外部权限扩大为整卡阻塞。默认全任务 3～5 文件；需要独立 OPPO 子范围时先登记有限步骤，不整包派发。

### 允许修改范围

- `android/core/notifications/**`（提供方接口、FCM/OPPO适配及注册协调，不扩展其他业务）

### 本任务工作

1. 登记/更新注册、data提示进入单飞同步、退出清理和延迟提示校验。
2. 先交付 provider 插槽+FCM；按冻结契约分别上报通道/注册代次，旧回调绑定旧部署/会话并丢弃，注销不得重新登记。无GMS/未同意SDK/不支持厂商/注册失败有明确状态，继续app-open/WorkManager。
3. OPPO适配在P02-05对应外部门槛通过后，按已批准PUSH-D1～D3独立派发，锁SDK来源/版本，取得所需同意后初始化，区分SDK客户端参数与服务端鉴权秘密；MasterSecret不得入APK。通知点击仅打开受控本应用入口，校验当前部署/会话后进入同一补偿同步，不在推送载荷携带正文/URL/凭据。
4. vendor模式FCM仅静默同步；配置确认前不以旧fcm模式补响，SDK注册不得自动激活vendor。建议P09-04-S01（框架+FCM）/S02（OPPO），调度登记后派发；父验收保留两通道及组合边界。

### 测试与完成标准

- [ ] Token变化/旧会话提示/重复消息/无Google服务均有明确状态。
- [ ] 双通道轮换不互删；拒绝SDK初始化仍可同步；冷启动/锁屏点击/退出后迟到点击不展示旧会话；模式切换与延迟FCM不双路自绘。OPPO真机未测则blocked，不能以桩代替。

必跑检查：本步骤相关 Android 单测、离线 lintDebug/assembleDebug；上列真实服务或真机组合按条执行。 记录实际命令、环境、结果与未测边界，遵循本文件共同交付规则。

<a id="and-05-02"></a>

## AND-05-02 Android WorkManager 补偿调度

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

设计定位：T9.1（T=TECHNICAL_DESIGN，A=ARCHITECTURE，D=DEVELOPMENT_PLAN；按标题核对）。

- 旧 ID：`P09-05`；类型：可单独派发的叶步骤；状态：未开始。
- 依赖（父项为完整收口依赖，叶步骤为开工依赖）：[AND-05-01](ANDROID.md#and-05-01)（Android 注册和 FCM 提示入口）；[AND-02-03](ANDROID.md#and-02-03)（Android bootstrap 与历史缺口）。
- 必需门槛：`G-GIT`。

Google 网络不可达时退化为打开即同步 + 周期 WorkManager；积压 flush 和 onDeletedMessages 均不能作为可靠补偿保证。通知/自启动权限、Google 服务/网络与 force-stop 分别记录，WorkManager 不承诺准点或强停后自救。

### 允许修改范围

- `android/core/sync/work/**`

### 本任务工作

1. 实现唯一周期网络任务/退避、前台立即补偿与重启恢复。

### 测试与完成标准

- [ ] 通知拒绝仍同步。
- [ ] 周期不承诺准点。
- [ ] 无常驻服务。
- [ ] 无并行重复同步。

必跑检查：本步骤相关 Android 单测、离线 lintDebug/assembleDebug；上列真实服务或真机组合按条执行。 记录实际命令、环境、结果与未测边界，遵循本文件共同交付规则。

<a id="and-05-03"></a>

## AND-05-03 Android 通知权限与偏好 UI

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

设计定位：T9.2（T=TECHNICAL_DESIGN，A=ARCHITECTURE，D=DEVELOPMENT_PLAN；按标题核对）。

- 旧 ID：`P09-06`；类型：可单独派发的叶步骤；状态：未开始。
- 依赖（父项为完整收口依赖，叶步骤为开工依赖）：[BE-10-01](BACKEND.md#be-10-01)（Hub 推送注册与设备通知偏好）；[AND-01](ANDROID.md#and-01)（Android Hub 登录与安全会话）。
- 必需门槛：`G-GIT`。

模式切换只影响未来事件；未知超时/受理结果不得自动 fallback FCM 补响。OPPO 通用 title/body 已批准，显式选择且只在验证通过后启用，不重新询问已批政策。

### 允许修改范围

- `android/feature/settings/notifications/**`
- `android/core/notifications/policy*`

### 本任务工作

1. 实现系统权限、全局/Bot开关、跨午夜DND/时区、锁屏预览并同步设备配置。
2. 按已冻结的T9.3/D2提供当前设备提醒模式与具体厂商可用状态，配置CAS冲突重新读取；注册成功不自动改偏好。fcm/vendor/none互斥，none保留静默同步。厂商不可用明确显示状态，超时不自动切FCM补响。
3. PUSH-D1～D3已批准（历史证据见[COMPLETED](COMPLETED.md)，按旧 ID 检索），vendor启用仍须PoC与对应外部门槛满足；向用户说明固定通用文案、无正文预览、已发通知取消及离线DND/已读/退出限制，不能把服务端发送前检查称为展示时保证。由用户显式选择已验证OPPO模式，注册成功不得自动激活；尚未满足外部门槛时保留厂商未启用状态，FCM按原T9.2验收。

### 测试与完成标准

- [ ] 设备独立。
- [ ] 权限拒绝可用。
- [ ] 不把系统权限当应用已读。
- [ ] 不补响静音历史。
- [ ] 注册/权限/偏好状态区分；配置CAS冲突不静默覆盖；vendor/none不会触发FCM自绘，OPPO已批准边界有明确说明，外部门槛未通过不启用，注册成功不默认启用。

必跑检查：本步骤相关 Android 单测、离线 lintDebug/assembleDebug；上列真实服务或真机组合按条执行。 记录实际命令、环境、结果与未测边界，遵循本文件共同交付规则。

<a id="and-05-04"></a>

## AND-05-04 Android 去重提醒与已读取消

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

设计定位：T9.2（T=TECHNICAL_DESIGN，A=ARCHITECTURE，D=DEVELOPMENT_PLAN；按标题核对）。

- 旧 ID：`P09-07`；类型：可单独派发的叶步骤；状态：未开始。
- 依赖（父项为完整收口依赖，叶步骤为开工依赖）：[AND-05-02](ANDROID.md#and-05-02)（Android WorkManager 补偿调度）；[AND-05-03](ANDROID.md#and-05-03)（Android 通知权限与偏好 UI）；[AND-03-05](ANDROID.md#and-03-05)（Android 共享已读与双设备回归）。
- 必需门槛：`G-GIT`。

模式切换只作用未来事件，发送快照/代次与当前配置分别核对。在途外发残留、离线跨 DND、已读/退出/撤销后展示只有有限保证，不以本地取消证明厂商可远程撤回。

### 允许修改范围

- `android/core/notifications/**`
- `android/core/database/**`

### 本任务工作

1. 实现消息ID通知tag/已处理记录、编辑不响/删除和共享已读取消。
2. fcm模式保留原T9.2去重与取消验收；vendor/none下所有入口（FCM、WS、app-open、周期、点击同步）均不得再自绘。按确认模式/配置版本验证延迟同步结果，不将vendor提醒当消息同步或共享已读ACK。
3. vendor服务端在发送前过滤共享已读/删除，稳定提醒键重试去重；平台已经受理/显示的通知能否取消以P02-05证据为准，默认不保证远程撤回。DND跨界积压、退出与模式切换迟到仍可能展示，其限制已由PUSH-D2/D3批准（历史证据见[COMPLETED](COMPLETED.md)，按旧 ID 检索），仍需P02-05实测；不以客户端本地通知取消测试声称厂商通过。

### 测试与完成标准

- [ ] 重复提示不多条。
- [ ] Room/系统通知崩溃窗口测试。
- [ ] 重建无历史轰炸。
- [ ] vendor下FCM/WS/app-open并发同步不追加自绘；fcm→vendor延迟结果与旧job有明确边界；已读前/发送中/受理后、断网跨DND及退出矩阵分别记录本地验证和真机证据，厂商不可测明确blocked。

必跑检查：本步骤相关 Android 单测、离线 lintDebug/assembleDebug；上列真实服务或真机组合按条执行。 记录实际命令、环境、结果与未测边界，遵循本文件共同交付规则。

<a id="and-05-05"></a>

## AND-05-05 完整 App 后台真机回归

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

设计定位：D9;T9（T=TECHNICAL_DESIGN，A=ARCHITECTURE，D=DEVELOPMENT_PLAN；按标题核对）。

- 旧 ID：`P09-08`；类型：可单独派发的叶步骤；状态：未开始。
- 依赖（父项为完整收口依赖，叶步骤为开工依赖）：[BE-10-03](BACKEND.md#be-10-03)（FCM 服务端适配）；[AND-05-04](ANDROID.md#and-05-04)（Android 去重提醒与已读取消）。
- 必需门槛：`G-GIT`。

承接 P2 所有 NOT_RUN：长窗睡眠、切网、Google 网络不可达、ColorOS 自启动未开、force-stop 与 HIGH 边界分列到达/提示/同步/展示。真实最终同步须正式 Hub；不依赖积压冲刷/onDeletedMessages。用户决定真机留最终验收，未执行保留 NOT_RUN，不重复索要 ADB。

### 允许修改范围

- `android/**/src/*Test/**`
- 本步骤脱敏交接证据（由调度汇入本文件）

### 本任务工作

1. 仅执行完整App上的P2矩阵并记录结果。
2. 缺陷交修复任务。

### 测试与完成标准

- [ ] 锁屏/切网/重启/force-stop/无Google/ColorOS/注册变化。
- [ ] 推送失败补偿正确。

必跑检查：本步骤相关 Android 单测、离线 lintDebug/assembleDebug；上列真实服务或真机组合按条执行。 记录实际命令、环境、结果与未测边界，遵循本文件共同交付规则。

<a id="and-06"></a>

## AND-06 OPPO Push 外部审核与通知栏 PoC

- 完成门槛角色：`G-OPPO` 由本步骤产出证据；不是本步骤开工／完成前须预先通过的条件，由调度在全部实际证据齐备后确认。

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

设计定位：T9.1～9.3、已批准 PUSH-D1～D3（T=TECHNICAL_DESIGN，A=ARCHITECTURE，D=DEVELOPMENT_PLAN；按标题核对）。

- 旧 ID：`P02-05`；类型：可单独派发的叶步骤；状态：blocked（OPPO 账号、应用权限及审核条件尚未准备）。
- 依赖（父项为完整收口依赖，叶步骤为开工依赖）：[已完成基线 PUSH-PLAN-01](COMPLETED.md#e2)（推送适配器提案授权边界与任务细化）。
- 必需门槛：无额外开工门槛；仍须满足依赖与下列完成前置。

显式 optional vendor，固定通用 title/body，无正文/URL/凭据。当前缺账号/应用权限/审核，blocked 只限厂商分支；不代办身份认证、注册、平台提交/上架或发布。未知超时不跨通道补响，切换仅未来事件；保留在途及离线 DND/已读/退出撤回有限保证，不能用展示 TTL 替代撤回。

### 允许修改范围

- `android/poc/opush/**`，不与正式工程混编，不改正式Hub/Android业务。
- 本步骤脱敏交接证据（由调度汇入本文件） 及该前缀的脱敏证据文件。
- 如最小REST测试器不能容纳在该PoC目录，先向调度登记等价路径，不自扩目录/依赖。

### 本任务工作

1. 重新核查现行官方资格与API/SDK：区分个人开发者身份认证、应用签名验证、通知栏测试/正式权限、侧载/未上架限制；给日期、原始链接和可读性。分类/模板、额度/QPS/提醒等级、审核周期未知即未核实，不把身份认证成功当Push开通。
2. 核实包名与签名指纹关联、debug/正式签名更换和regId生命周期；只记录指纹及安全引用，不生成永久发布身份，不记录私钥/口令。锁定SDK来源/版本/校验值及权限、同意后初始化要求；服务端MasterSecret与客户端SDK配置严格区分。
3. 取得合法可用的测试配置并核实现行接口后，按已批准的PUSH-D1～D3（历史证据见[COMPLETED](COMPLETED.md)，按旧 ID 检索）做最小通知栏单推；固定通用文案之外不含消息正文/URL/凭据。Hub职责不变；点击受控入口后校验会话并触发同一app-open同步。实际端点/鉴权/请求格式需现行官方源验证，不依据旧SDK或第三方示例编造。
4. 一加目标真机记录型号/ROM/Android/SDK/安装来源/签名、设备无代理、Hub外网条件、通知权限/系统渠道/自启动；前台、后台、进程死亡、Doze、重启/force-stop、锁屏点击分列到达/展示/点击/同步证据。不能用HTTP受理成功替代真机展示，不能把厂商通道当后台业务执行入口。
5. 受控测试有效/格式错误/已注销regId，鉴权失败、限流、网络失败、受理后超时、重复同提醒键；记录脱敏HTTP状态与平台错误码、现行文档和可复现步骤，形成错误归类表。不通过格式错误推断设备已卸载；未知错误不删注册。注销只影响对应通道/代次，旧失败不能删新token。
6. 验证厂商离线TTL/去重与通知覆盖的真实含义：断网发送后已读、删除、跨入DND、退出/撤销、切换模式，再恢复网络；发送中切换、旧job/旧FCM迟到与重复worker。明确哪些由桩验证，哪些由真机验证，哪些平台不支持；不保证绝对无双响，不以展示TTL替代撤回。
7. 提交能力矩阵及已批准PUSH-D1～D3的验证证据，区分用户接受的边界与平台实际能力。资格失败、只能测试权限或分类不获准时报告具体限制及可继续分支，不自动改类别、申请企业账号或降低隐私边界。

### 测试与完成标准

- [ ] 当前资格/权限、签名、分类/限额与SDK/API版本均标核实/未核实，给官方证据，测试与正式门槛分开。
- [ ] 按已批准PUSH-D1固定文案，真机无代理通用通知展示、点击打开同步有独立时间线；缺环境not_run/blocked。
- [ ] 进程/锁屏/重启/force-stop/权限/网络矩阵不漏不伪造；SDK未同意不得提前初始化。
- [ ] 错误分类、单通道代次解绑、重复发送/不确定受理、跨DND/已读/退出/模式切换积压有证据或明确限制。
- [ ] vendor与FCM互斥不把桩结果当厂商真机通过；PUSH-D1～D3已批准，但资格/权限、SDK/API、签名/分类/额度/错误码及真机仍未验证，不记vendor启用通过。
- [ ] 实际构建/检查命令、环境、结果及脱敏证据；提交前diff/status/敏感信息检查。

必跑检查：PoC 实际构建/检查及上列资格、通知栏真机矩阵；缺外部条件保留 blocked。 记录实际命令、环境、结果与未测边界，遵循本文件共同交付规则。

<a id="and-01-s"></a>

## AND-01-S 完整设备撤销与登录父收口

- 完成门槛角色：`G-INSTANCE-COMPOSITION` 由本步骤产出证据；不是本步骤开工／完成前须预先通过的条件，由调度在全部实际证据齐备后确认。

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

设计定位：T3/5/7/8/9 的本步骤相关局部（T=TECHNICAL_DESIGN，A=ARCHITECTURE，D=DEVELOPMENT_PLAN；按标题核对）。

- 旧 ID：`P08-04-S04`；类型：可单独派发的叶步骤；状态：未开始。
- 依赖（父项为完整收口依赖，叶步骤为开工依赖）：[已完成基线 P08-04-S03](COMPLETED.md#a1)（Android登录与入口）；[已完成基线 P03-03](COMPLETED.md#e1)（Android 最小原生工程）；[已完成基线 P04-05](COMPLETED.md#g1)（设备登记会话列表与撤销）；[AND-01-V](ANDROID.md#and-01-v)（身份专项组合验收）。
- 必需门槛：无额外开工门槛；仍须满足依赖与下列完成前置。

本步骤产出 G-INSTANCE-COMPOSITION 证据，并完成父全部原验收；不等待自己产出的 gate 预先通过。最终真机/Keystore/release 签名条目不能因专项或模拟器通过而自动放行。

### 允许范围

3 主要验证源：`android/app/src/androidTest/kotlin/top/jianxinli/relaynest/auth/HubAuthContractTest.kt`、`SessionRevocationTest.kt`、`SessionIsolationTest.kt`；对应报告。仅允许必要 app Gradle/androidTest Manifest runner 接线，优先平台Instrumentation，若需新增小型测试依赖须说明官方固定版本与必要性、仅最小新增锁条目，不升级已有版本或引入大型测试架构。测试布局可等价映射。

本卡只验证，不修业务代码；发现编号后交修复卡再重审。账户由安全环境加载，日志不得含Token/密码/原始认证正文。

### 验收

- 真实Hub登录→恢复/session校验→logout；HTTPS、自定义端口、无RelayShelf强制配置、不明文保存密码、错误原因及本地会话结束，区分实测/静态证据。
- A/B真实独立会话；经完整P04-05用精确Session.id远程撤销A，A后续真实请求401、Android清除会话，B仍有效。不能用本机logout冒充远程撤销，不能用device_id选择撤销；过期和迟到响应不得复活身份。
- 地址/端口/后端切换不得附旧Bearer；同地址部署替换按D01获准方案补充实测，未获准或无真实证据保持blocked，不能降低为仅地址隔离。
- T9不跨实例发送旧数据的长期约束交给后续同步/outbox；暂无outbox不能证明未来业务已通过。此卡不实现推送、管理页面或业务重放。
- 汇总父全部原目标（地址/登录/加密/注销/撤销/部署隔离）与全部原验收，记录逐项命令/环境/结果与精确版本。真机、Keystore真机和成功release签名留最终用户验收NOT_RUN，未执行不能放行对应门槛；OPPO账户/NAS入口仅挡相关分支。

### 检查与交接

执行真实服务与可用Android测试环境的上述组合验证，及既有mise下 `cd android && mise exec -- ./gradlew :app:lintDebug :app:assembleDebug --offline`；缺真实服务/设备/SDK等按条目标记NOT_RUN/blocked，不反复询问ADB。不改源码/API/工具链或无关范围，不直接改模块状态、不推送部署。

必跑检查：本步骤相关 Android 单测、离线 lintDebug/assembleDebug；上列真实服务或真机组合按条执行。 记录实际命令、环境、结果与未测边界，遵循本文件共同交付规则。

<a id="and-01-n"></a>

## AND-01-N Android实例门禁网络

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

设计定位：T3/5/7/8/9 的本步骤相关局部（T=TECHNICAL_DESIGN，A=ARCHITECTURE，D=DEVELOPMENT_PLAN；按标题核对）。

- 旧 ID：`P08-04-D01-N01`；类型：可单独派发的叶步骤；状态：未开始。
- 依赖（父项为完整收口依赖，叶步骤为开工依赖）：[BE-01](BACKEND.md#be-01)（服务实例绑定）。
- 必需门槛：`G-GIT`、`G-API`、`G-INSTANCE-CONTRACT`。

### 允许范围

android/app/src/main/kotlin/top/jianxinli/relaynest/core/network/HubClient.kt、HubAuthDto.kt、最多3个小型probe/绑定类型文件及对应单测。仅network包，不改安全存储、仓库/UI/工具链。

### 工作与冻结接口

严格实现C01 probe解析/1024解压后字节上限/5秒call超时/TLS/所有3xx拒绝/404无降级/无缓存无自动重试。独立无凭据构造，无Authorization/Cookie/CSRF/账号/install/session信息。冻结对后卡的probe、绑定请求、取消与票据接口，login/session DTO带instance_id，缺错响应不交凭据给上层。
每次受保护请求或未来WS握手取得独立一次发送前probe，不共享可无限复用的成功布尔；同endpoint/代次最多一个probe在飞，等待者轮流各自探测。网络层提供门禁能力，下一协调卡完成状态原子联动；中间提交保持编译且旧仓库调用失败关闭，不能为兼容保留直接带Bearer旁路。

除C01外以C01最终冻结SHA为唯一契约；接口偏离先报调度，不自创需求。G1只覆盖诚实替换/误连，接受probe后瞬切旧Bearer/密码字节可能到B；随机公开ID不是秘密。保持Bot Hub传输边界，不改RelayShelf/MP，不做现网维护/部署，不直接改模块状态或他卡。

### 验收与失败边界

抓包/MockWebServer证明探测不带任何凭据；超限含gzip/Content-Length不可信、重复JSON键/额外字段/坏UUID/content-type、DNS/TLS/超时/429/503/redirect/旧server均阻止后续带凭据派发。login/session缺错ID拒绝；每请求新GET、取消与并发独立探测测试。仅协议桩不等于真实服务。

### 检查、环境与交接

既有mise/SDK运行相关network单测及 :app:lintDebug :app:assembleDebug --offline；不升级依赖。S03已独审集成，派发基线包含其集成结果，Android三个后卡共享文件串行A/I。

必跑检查：本步骤相关 Android 单测、离线 lintDebug/assembleDebug；上列真实服务或真机组合按条执行。 记录实际命令、环境、结果与未测边界，遵循本文件共同交付规则。

<a id="and-01-k"></a>

## AND-01-K 实例加密记录迁移

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

设计定位：T3/5/7/8/9 的本步骤相关局部（T=TECHNICAL_DESIGN，A=ARCHITECTURE，D=DEVELOPMENT_PLAN；按标题核对）。

- 旧 ID：`P08-04-D01-K01`；类型：可单独派发的叶步骤；状态：未开始。
- 依赖（父项为完整收口依赖，叶步骤为开工依赖）：[AND-01-N](ANDROID.md#and-01-n)（Android实例门禁网络）。
- 必需门槛：`G-GIT`、`G-API`、`G-INSTANCE-CONTRACT`。

### 允许范围

android/app/src/main/kotlin/top/jianxinli/relaynest/core/security/SessionStore.kt、SessionCipher.kt、InstallationIdentity.kt（仅必要兼容核对）及最多2个记录类型文件、对应测试。不得改仓库/UI或网络实现。

### 工作与冻结接口

新记录严格同时绑定规范化endpoint、instance_id、Session.id及本地代次；升级序列化版本/AAD，随机安装UUID独立。旧加密记录（含现S02无ID格式）不可探测后补认，清旧Bearer并要求显式重登，不先发session。无明文密码/Token回退。
给R01冻结可编译的读/写/清除结果接口；保留旧调用签名只可明确返回需重登/失败关闭，不得留下可发送旧记录旁路。暂时身份待验证可保留密文但不得作为已认证状态或解密交网络；协调行为由下一卡整合。

除C01外以C01最终冻结SHA为唯一契约；接口偏离先报调度，不自创需求。G1只覆盖诚实替换/误连，接受probe后瞬切旧Bearer/密码字节可能到B；随机公开ID不是秘密。保持Bot Hub传输边界，不改RelayShelf/MP，不做现网维护/部署，不直接改模块状态或他卡。

### 验收与失败边界

现S02旧记录/缺ID/错endpoint/错session/错代次/AAD篡改/损坏密文/密钥失效均不可恢复成可发送Bearer；新格式往返/严格解析；新ID与安装UUID不混用；并发清除/写入不能恢复旧值，不保存明文密码。JVM或模拟器结果标注，真实Keystore未测保留NOT_RUN。

### 检查、环境与交接

安全存储相关既有单测、Android离线lint/assemble；不要求本卡真机或成功release签名。N01独审集成后才写，签名与SDK沿用已有环境。

必跑检查：本步骤相关 Android 单测、离线 lintDebug/assembleDebug；上列真实服务或真机组合按条执行。 记录实际命令、环境、结果与未测边界，遵循本文件共同交付规则。

<a id="and-01-r"></a>

## AND-01-R Android身份协调接线

- 完成门槛角色：`G-INSTANCE-RUNTIME` 由本步骤产出证据；不是本步骤开工／完成前须预先通过的条件，由调度在全部实际证据齐备后确认。

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

设计定位：T3/5/7/8/9 的本步骤相关局部（T=TECHNICAL_DESIGN，A=ARCHITECTURE，D=DEVELOPMENT_PLAN；按标题核对）。

- 旧 ID：`P08-04-D01-R01`；类型：可单独派发的叶步骤；状态：未开始。
- 依赖（父项为完整收口依赖，叶步骤为开工依赖）：[AND-01-K](ANDROID.md#and-01-k)（实例加密记录迁移）。
- 必需门槛：`G-GIT`、`G-API`、`G-INSTANCE-CONTRACT`。

本步骤产出 G-INSTANCE-RUNTIME 的实际专项证据；gate 目前 pending，不是本步骤的预先放行条件。

### 允许范围

android/app/src/main/kotlin/top/jianxinli/relaynest/feature/auth/AuthRepository.kt、AuthViewModel.kt、AuthScreen.kt（仅身份待验证/明确重登/远端注销未确认提示）、必要network/security适配最多2文件及相关测试；MainActivity仅现S03既有构造接线必要时使用。不得引入Room/outbox/WS。

### 工作与冻结接口

登录先无凭据probe I→带I login→核对响应ID/票据才保存；不持久化密码、不自动重试旧密码。恢复每次先probe；I相同才发带I session，变化原子推进代次/清Bearer/取消在飞/结束认证，显式重登J。探测失败保留密文和草稿但处于身份待验证，禁带Bearer；离线本地注销立即清除，远端未探测不发且显示未确认。
统一endpoint/ID/Session.id/代次协调锁：网络在锁外、结果回锁核票据、发送临界区前切址/注销生效不得发送，之后仅取消/丢弃迟到响应；成功probe仅授权一次对应派发。不得持锁跨网络等待或用一次probe释放所有等待者。
Room/outbox/WS尚无接口时只登记将来强依赖：缓存/草稿/outbox按endpoint+ID隔离、不得自动迁移或重登后重放旧内容；未来握手/受保护业务走相同门禁，不冒充当前验收。

除C01外以C01最终冻结SHA为唯一契约；接口偏离先报调度，不自创需求。G1只覆盖诚实替换/误连，接受probe后瞬切旧Bearer/密码字节可能到B；随机公开ID不是秘密。保持Bot Hub传输边界，不改RelayShelf/MP，不做现网维护/部署，不直接改模块状态或他卡。

### 验收与失败边界

覆盖登录/恢复/注销、远程401撤销、并发探测与注销/切址/同origin换ID、迟到响应及密文写回不能复活、失败后显式重试、旧记录重登、旧server不降级。分开证明“变化/失败在发送前发现未发Bearer”与“probe后切B已发但server拒绝操作”，承认G1残余窗口。父无RelayShelf/自定义端口/不明文存密码继续回归。

### 检查、环境与交接

既有Android仓库/ViewModel单测和离线lint/assemble；与S03共享入口串行，无工具链升级。网络桩/模拟器结论不能充真实Hub/phone；交接完整状态转换和未来接入门禁。

必跑检查：本步骤相关 Android 单测、离线 lintDebug/assembleDebug；上列真实服务或真机组合按条执行。 记录实际命令、环境、结果与未测边界，遵循本文件共同交付规则。

<a id="and-01-v"></a>

## AND-01-V 身份专项组合验收

- 完成门槛角色：`G-INSTANCE-COMPOSITION` 由本步骤产出证据；不是本步骤开工／完成前须预先通过的条件，由调度在全部实际证据齐备后确认。

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

设计定位：T3/5/7/8/9 的本步骤相关局部（T=TECHNICAL_DESIGN，A=ARCHITECTURE，D=DEVELOPMENT_PLAN；按标题核对）。

- 旧 ID：`P08-04-D01-V01`；类型：可单独派发的叶步骤；状态：未开始。
- 依赖（父项为完整收口依赖，叶步骤为开工依赖）：[AND-01-R](ANDROID.md#and-01-r)（Android身份协调接线）；[已完成基线 P04-05](COMPLETED.md#g1)（设备登记会话列表与撤销）；[已完成基线 P03-03](COMPLETED.md#e1)（Android 最小原生工程）；[已完成基线 P08-04-S03](COMPLETED.md#a1)（Android登录与入口）。
- 必需门槛：`G-GIT`、`G-API`、`G-INSTANCE-CONTRACT`。

本步骤产出 G-INSTANCE-COMPOSITION 的组合证据；gate 目前 pending，不等待自己产出的 gate 预先通过，最终 AND-01-S 保留独立父收口。

### 允许范围

仅组合测试与 本步骤脱敏交接证据（由调度汇入本文件）；复用S04允许的HubAuthContractTest/SessionRevocationTest/SessionIsolationTest等，单次3–5主要验证文件，S04最终runner接线仍归S04。不改业务/契约/SQL，失败转编号修复。

### 工作与冻结接口

组合C01精确契约、S01/M01/B01服务与N01/K01/R01 Android最终独审集成版本；验证诚实同origin替换、原库迁移保留、恢复/clone先停服换ID全撤销、清库新ID及损坏停服、旧版本矩阵和升级阻断窗口。维护使用隔离临时部署，不操作现网。
强依赖完整P04-05设备分支与P03-03/S03独审集成；A/B真实会话及精确Session.id撤销，经实例门禁后401清A且B有效。区分发送前阻止与probe后切换拒绝业务，不宣称G2零字节泄露。逐项输出身份专项证据供S04汇总；本卡不替代S04/父卡最终全设备验收。

除C01外以C01最终冻结SHA为唯一契约；接口偏离先报调度，不自创需求。G1只覆盖诚实替换/误连，接受probe后瞬切旧Bearer/密码字节可能到B；随机公开ID不是秘密。保持Bot Hub传输边界，不改RelayShelf/MP，不做现网维护/部署，不直接改模块状态或他卡。

### 验收与失败边界

真实Hub HTTPS、自定义端口、登录/恢复/logout/远撤、无RelayShelf/无明文密码；probe各失败与真实A→B竞争；恢复/clone旧会话不可用、ID与代次一致；缓存/草稿未跨实例证据仅覆盖现已存在部分。真实服务缺失逐条NOT_RUN/blocked；未来Room/outbox/WS/push映射门禁仍待各实现卡，不因本卡通过解除。

### 检查、环境与交接

真实服务+可用Android环境执行组合测试及既有离线lint/assemble，记录OS/SDK/服务与Android SHA、HTTPS拓扑、模拟器/真机差别。phone/NAS/OPPO/真机Keystore/成功release签名原限制保留，最终相关S04条目不得自动通过。

必跑检查：本步骤相关 Android 单测、离线 lintDebug/assembleDebug；上列真实服务或真机组合按条执行。 记录实际命令、环境、结果与未测边界，遵循本文件共同交付规则。
