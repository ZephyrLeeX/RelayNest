# 推送通道插槽/适配器架构与 OPPO Push 接入规划（提案）

日期：2026-10-21。提出：P2 FCM 线会话。性质：**规划提案**，任务卡登记与 T9 修订由总调度线合入后生效。

> PUSH-DECISION-01 当前决定注记（2026-10-21）：用户已批准 [PUSH-D1～D3](PUSH-DECISION-01-implementation.md)：固定标题「RelayNest 有新消息」与固定正文「打开 RelayNest 同步查看」，无业务正文/URL/凭据；OPPO 为显式可选提醒模式，超时不跨 FCM 补响，切换仅未来新事件，并接受已外发通知残余及离线积压展示时 DND/已读取消/退出后撤回无法保证。当前以 [T9.3](../../TECHNICAL_DESIGN.md) 为准；厂商资格、SDK/API、签名/分类/额度/错误码及真机仍未验证，本次不激活厂商、不注册平台、不创建正式密钥。下方复核注记和原提案保留历史语境，其中待决已由本注记更新，不再重复询问；注册后自动 vendor 等旧建议仍无效。

> PUSH-PLAN-01 历史复核注记（2026-10-21）：下文保留为原始提案历史，**不能作为冻结契约或平台现行事实实施**。后续以 [T9.3条件规划](../../TECHNICAL_DESIGN.md)、[官方研究](PUSH-PLAN-01-external-research.md) 与审计/集成后的任务卡为准。具体纠正：官方通知参数要求title与content均必填，通用title+content须用户D1确认；单通道失效只条件解绑该通道/发送代次，不能删设备会话；注册厂商后自动默认vendor、已读不可撤回降级和离线DND/退出展示限制尚待D2/D3，不能写作已接受；个人认证不等于Push资格，固定审核天数、分类/限额及签名影响未核实；超时不自动切FCM，模式CAS、旧job隔离及在途残余风险需验收。P02-05只阻OPPO分支，P3～8和框架/FCM可继续；本轮不代办任何平台审核或真实发布。

## 决策背景与授权

用户 2026-10-21 决策：

1. 新增 OPPO Push 厂商通道（个人开发者、通知栏消息形态），补「设备侧无 Google 网络（日常不开代理）时 FCM 不可达」的缺口（P02-04 结论与用户实际使用模式）。
2. 厂商通道文案只发**通用标题**（如「RelayNest 有新消息」），满足 T9.1 隐私边界（正文/凭据不过厂商服务器）。
3. 整体推送架构改为**插槽/适配器模式**，后续新增厂商（MiPush/Huawei/vivo 等）只需各写一个适配器，不改核心。

P2 已有结论不受影响：G-FCM 维持；补偿架构（打开即同步 + WorkManager 周期）通道无关，是本双通道设计的地基。

## 架构设计

### 通道职责模型（双通知规避是关键约束）

每个设备登记**两个独立职责**：

- **静默同步通道**：仅 FCM data hint（T9.1 原样，watermark/deployment_id，不产通知）。
- **提醒通道（notify_channel）**：三选一且互斥——`vendor`（OPPO 等厂商通知栏）/ `fcm`（FCM hint + 应用自绘通知，T9.2 原样）/ `none`。

互斥规则：`notify_channel=vendor` 时，客户端收到 FCM hint 只静默同步、**不自绘通知**（提醒已由厂商系统通知负责）；Hub 侧对同一 (device, bot) 的待提醒合并（T9.1 push_jobs 已有）保证厂商通道不重复打扰。默认取值：设备注册了厂商通道→`vendor`，否则 `fcm`；用户可改。

### 服务端插槽（Go，Hub）

```go
// PushChannel 是厂商/协议通道适配器的唯一插槽；核心投递逻辑只依赖本接口
type PushChannel interface {
    Type() string                                  // "fcm" | "opush" | ...
    Send(ctx context.Context, target DeviceTarget, n Notification) error
    ClassifyError(err error) SendError             // 归类：目标失效 / 暂态 / 永久配置错
}
```

- `Notification` 对厂商通道只含通用标题（服务端策略化后的产物）；对 FCM 是 data payload。
- 错误归类是插槽契约的一部分：FCM 实测 404 `UNREGISTERED` → 目标失效（删设备记录，T9.1）；OPPO 对应 `target_invalid` 类错误码（具体码 P02-05 实测定型）；网络/5xx → 暂态（指数退避，不删记录）。
- 路由器按设备 `notify_channel` 选适配器；适配器注册表启动期装配，新增厂商=新文件+注册，无核心改动。

### 客户端插槽（Kotlin，Android）

```kotlin
// PushChannelProvider：厂商 SDK 适配器插槽；同步协调器只认本接口
interface PushChannelProvider {
    val type: String
    suspend fun register(context: Context): ChannelRegistration   // {type, token/regId}
    fun onAppStart(context: Context)                               // 注册刷新/点击意图处理入口
}
```

- FCM 适配器：现有 PoC 行为产品化（token 上报、onMessageReceived→触发唯一同步任务）。
- OPPO 适配器：SDK 注册拿 regId 上报；通知栏消息由系统展示；**点击回调→打开应用→触发 app-open 同步**（复用 P02-02 的补偿入口）。
- 提醒展示策略：`notify_channel=vendor` 时通知展示模块（P09-06/07）跳过自绘；=fcm 时按 T9.2 客户端策略自绘。

### 设备注册契约（P03-07/P09-01 修订点）

设备登记记录从单 token 扩为通道集合：`channels: [{type, token, updated_at}]` + `notify_channel: vendor|fcm|none`。服务端按通道类型分表/分字段存，Token 轮换只替换同通道记录（T9.1 规则通道无关化）。

### 通知策略的职责迁移（T9.2 修订要点）

厂商通道下，以下策略从客户端挪到 Hub 服务端发送前判断（Hub 本就是发送方，具备全部状态）：免打扰时段、(deployment,message) 去重、已读取消（取消场景厂商通道不可撤回已发通知——**降级接受**，与 FCM 自绘通知的「删除/共享已读取消通知」存在能力差，写入约束）；锁屏隐私由「只发通用文案」满足。

## 任务卡规划（待总调度登记）

| 卡 | 动作 | 内容 | 前置 |
| --- | --- | --- | --- |
| **P02-05**（新） | 新卡 | OPPO Push 通知栏 PoC：开放平台接入（认证/创建应用/推送审核）、真机**无代理**收到通用标题通知、点击打开应用触发同步、REST 单发与错误码实测（含无效 regId）、分类字段/频次限额实测。范围 `android/poc/opush/**` + 报告 | OPPO 审核通过；签名决策（见下） |
| **P03-03**（增注） | 修订 | Android 工程初始化时**固定签名方案**（keystore 生成、保管位置、CI 注入方式）——OPPO 应用注册绑定包名，签名与注册签名的绑定程度由 P02-05 先行验证 | — |
| **P09-02**（微注） | 修订 | push_jobs 目标水位合并保持通道无关 | — |
| **P09-03**（修订） | 改名扩容 | 「FCM 服务端适配」→「推送通道适配器框架」：`PushChannel` 插槽 + FCM/OPush 双实现 + 错误归类 + 路由 | P02-05 结论 |
| **P09-04**（修订） | 改名扩容 | 「Android 注册和 FCM 提示入口」→「Android 推送通道插槽」：`PushChannelProvider` + 双适配器 + 通道集合上报 + notify_channel 配置 + 点击意图→补偿入口 | P02-05 结论 |
| **P09-06/07**（微注） | 修订 | 通知展示与 `notify_channel` 联动（vendor 模式不自绘）；已读取消在厂商通道的降级说明 | — |
| **T9.1/T9.2** | 文档修订 | 增「厂商通知通道」小节：职责模型、错误归类、服务端策略化、通用文案边界、不可撤回降级 | 本提案获批 |

## 用户侧外部前置（可即刻并行启动，审核周期不等开发）

1. OPPO 开放平台（open.oppomobile.com）**个人开发者认证**（身份证，审核约 1-3 工作日）。
2. 创建应用：包名 `top.jianxinli.relaynest`（AppKey/AppSecret 拿到后与 Firebase 凭据同等对待：仅存 `~/.config/relaynest-dev/`，不入库不进报告）。
3. 开通推送服务并按 2024-11 新规配置**消息分类字段**；按需申请私信类目（IM 类消息的及时性依赖它）。
4. 决定正式签名 keystore 的生成与保管方式（passphrase 存放约定）。

## 风险与验证点（P02-05 必须给出结论）

- **签名绑定程度**：PoC 期用临时签名注册，正式签名后是否需重新登记/更换（影响 P03-03/P12-04 顺序）。
- **私信类目审核**：个人开发者 IM 类目能否通过、限额多少；不通过则退化为公信类目（到达率/频次受限，可接受性由用户判断）。
- **通知点击拉起**：锁屏/灭屏下点击通知打开应用并触发同步的实际行为（对标 P02-02 补偿路径）。
- **审核时效**：1-3 工作日 ×（认证+推送开通+类目）串行，规划上不阻塞 P3-P8 核心开发，P9 集成前完成即可。
- 透传消息维持不可用假设（个人开发者），本架构不依赖它。

## 时间线建议

P02-05 与 P3 工程化并行（外部审核耗时，先提交认证）；P4-P8 不受影响；P9 按修订卡落地插槽框架与双适配器；P11 联调补「无代理真机端到端」场景（OPPO 通知→点击→同步→已读）。
