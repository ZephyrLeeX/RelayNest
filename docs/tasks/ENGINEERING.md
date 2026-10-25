# 跨模块工程、契约与 CI

## 已冻结基线

P03-01～P03-12 已全部完成：Go/Web/Android 最小工程、固定工具链、静态检查和CI配置，以及认证、Bot、消息、附件、投递、同步、设备、多通道推送的 shared 契约及 SQLite 迁移框架。G-API pass；实例身份扩展契约 G-INSTANCE-CONTRACT pass。详见[已完成基线](COMPLETED.md)和[决定摘要](COMPLETED.md#e2)，不重做P3，不以契约冻结推定真实路由或运行时通过。

## ENG-01 跨端契约落地验收索引

本条只索引已有任务的验收门槛，无旧待办ID、无新增可派发范围或任务状态，不作为依赖节点；不可据此派整个跨端组合给一个worker。

| 尚待实证 | 负责模块实际任务 | 收口条件 |
| --- | --- | --- |
| 完整实例身份运行时 | [BE-01-AUTH](BACKEND.md#be-01-auth)、[BE-01-HTTP](BACKEND.md#be-01-http)及 Android 模块的实例任务 | G-INSTANCE-RUNTIME pending；公开探测和真实路由必须读取同一已提交快照，不能以契约通过代运行时 |
| 实例跨端组合 | [AND-01-V](ANDROID.md#and-01-v)及 [AND-01](ANDROID.md#and-01)各具体子任务 | G-INSTANCE-COMPOSITION pending；实际独立审计及精确集成组合通过，不能按父组构造伪循环 |
| 正式 Hub/MP/Android 全链路 | [DEL-02-02](DELIVERY.md#del-02-02)、[DEL-02-05](DELIVERY.md#del-02-05)及 MP-02 各叶任务 | G-CORE pending；局部通过或PoC协议桩不能替代完整端到端 |
| 正式构建与交付 | [DEL-04-01](DELIVERY.md#del-04-01)、[DEL-04-02](DELIVERY.md#del-04-02)、[DEL-05-01](DELIVERY.md#del-05-01) | G-DEPLOY pending；CI配置和本地组合不等于远端发布成功 |

所有 shared 契约与迁移变更由对应实现任务负责，同步其文档和受影响测试；迁移编号须在实施时核对已集成状态后串行分配，不预设下一号为0008、不争用编号或重复并行迁移。重要需求、架构、安全、外部接口变化先提交具体方案等待用户确认。默认每任务3～5个主要实现文件及测试，不扩职责。

本模块没有剩余P3实现卡；CI失败只能按实际证据定位受影响任务，不能为了“重建工程基线”新增无关工作。镜像/插件、APK buildRelease、签名、真正部署升级与 Release 的准备和最终确认遵循[部署交付模块](DELIVERY.md)。
