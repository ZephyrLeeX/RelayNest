# P1 受控文本入站与定向回复实测

日期：2026-10-21。RelayNest 父基线：`8e8afa6`。实现提交 SHA 见本地交接。

## 结论与范围

**固定 `/noai` 文本经现有 MoviePilot v3.1.5 原生消息链处理，两个独立测试身份均各收到一次 source／userid 正确的原生提示回复。** 不需要新渠道枚举、自定义通知配置、借用 QQ／WebAgent 或核心补丁。

这证明的是限定输入和两组命名空间身份下的文本闭环，不是任意用户文本、真实业务命令或完整双向验收。该回复未保留输入的 original_chat_id、original_message_id；群会话与引用映射仍有缺口。原生 callback、自动编辑能力查询仍为 false。

本轮按用户授权在现有 fnOS / Compose 实例进行最小验证，结束后已停用、卸载及移除远端探针。没有执行搜索、下载、订阅、AI、正式 Hub、Android、FCM 或 RelayShelf 操作。未将 P01-03 的同步／异步主动广播、独立审计或全部验收项写成已完成；**G-MP 仍未通过**，不更新任务台账。

## 变更

- 探针升级至 0.0.2，新增受 API 密钥保护的 `text_preflight`、`text_probe`。
- `text_probe` 不接受任意输入，只调用宿主 `MessageChain().handle_message()` 两次，输入固定 `/noai`，显式 `is_channel_admin=False`。
- 两组 source 为 `RelayNest-P1Probe:A/B`；userid 按 source 及本轮随机标记构造，避免使用真实账号或已有业务会话。
- `post_message` 只捕获当前待处理 source、精确 userid、固定原生提示标题、无正文且 save_history=false 的回复。没有猴子补丁、子类替换、核心方法覆盖或通知事件拦截。
- 同一探针实例重复请求返回缓存；模块用例不清除文本缓存。停用清除全部文本状态，无新增线程、网络服务或依赖。
- 客户端增加固定动作；验证脚本覆盖传播检查、回复拒绝边界和缓存行为；README 补充复现和副作用说明。

最终实际部署的源码 SHA-256：`e5ed7b0c934171445403faf7b27e44c512220c6ae572495b638e583a689443ef`。

## 固定宿主证据与无业务输入选择

主基线仍为 MoviePilot v3.1.5 commit `99ee18d0e6a27422a4affd3d4ec7d8d56772e4c3`。本轮重新检查现有容器关键文件：

| 文件 | SHA-256 |
| --- | --- |
| `app/chain/message.py` | `af8fe2c615cc55c6e983c68b0b4f052ad2d42b9e83a109f1658a94b7aa03b0bb` |
| `app/chain/_messaging.py` | `a098417f1b7b6abbc1d46e397c0181d44dba78e05ed2cba5b64eeccff77983a2` |
| 当前容器 `app/application/messaging/webagent/events.py` | `74c58179acc3fb53a065b689e21ee3822335cbe795093e9cae3a654993e2eb76` |

前两项与此前固定 commit 文件一致。WebAgent 桥以本次读取的容器文件及上列哈希为具体证据，不将其行为外推到未读取版本。

[原生消息链](https://github.com/jxxghp/MoviePilot/blob/99ee18d0e6a27422a4affd3d4ec7d8d56772e4c3/app/chain/message.py) 的 `_strip_no_ai_prefix()` 将无正文 `/noai` 解析为空；`_handle_message_core()` 随即构造标题为“请输入要使用传统交互处理的内容”的 Message，并在进入斜杠命令事件、AI、媒体交互及 UserMessage 事件前返回。

本次实际调用路径为：

```text
鉴权后的探针固定 HTTP 入口
  → 未修改的 MessageChain.handle_message(channel=None, source=A/B, text=/noai)
  → 原生输入记录、消息核心的 /noai 分支
  → 原生 post_message、通知派发与模块分发
  → 探针 post_message 捕获固定提示
```

没有直接构造成功回复来代替 MP 处理，也没有跳过 `handle_message()` 的输入处理。此次采用与 NapCat 类似的直接 handle_message 入口；`MessageChain.process()` 和 message_parser 不在本轮验证范围。

## 传播边界检查

首次只读检查返回 success=false，没有执行文本输入：活动 NoticeMessage 监听器中存在 MP 内置的 `app.application.messaging.webagent.events.dispatch_web_agent_message_event`。

继续读取该桥源码：它先调用 `extract_web_agent_message_from_event_data()`，提取结果为空就立即返回；提取函数要求 channel 为 WebAgent，空 channel 会返回 None。探针随后用当前宿主真实 Message 模型和真实提取函数，分别构造两组固定提示验证，结果均为 None。

因此仅排除此已核实不接收本次消息的内置桥；未知活动监听器仍会阻止测试。未停用、替换或修改任何现有监听器。最终主进程预检响应为：

```json
{
  "success": true,
  "active_notice_handlers": ["app.application.messaging.webagent.events.dispatch_web_agent_message_event"],
  "other_post_providers": [],
  "source_collisions": [],
  "blocked_notice_handlers": [],
  "webagent_rejects_probe": true
}
```

本轮还静态查看了现有 8 个插件的事件声明。部分插件监听 UserMessage，但固定 `/noai` 分支在发送该事件前返回；没有借此触发其业务处理。此检查针对当前实例当时的活动配置，不代表所有插件组合安全。

## 实测结果

| 检查 | A | B |
| --- | --- | --- |
| 固定原生输入 | `/noai` | `/noai` |
| 渠道 | None | None |
| is_channel_admin | false | false |
| handle_message 异常 | 无 | 无 |
| 捕获回复数 | 1 | 1 |
| source／userid 正确定向 | true | true |
| 原生保留 chat ID | false | false |
| 原生保留 message ID | false | false |

HTTP 响应的 kind 为 `native_handle_message_fixed_noai`，success=true。两组 userid 分别为各自 source 加独立命名空间和相同本轮随机标记，没有借用真实用户账号。

chat/message 的丢失与源码一致：该原生提示 Message 传递了 channel/source/userid/username，但没有传入 original_chat_id 或 original_message_id。探针如实记录 false，未使用旁路补全后宣称原生已保留。后续必须单独验证插件映射以及并发、异步回复归属，不能仅靠 source 或最近活跃会话解决。

`is_channel_admin=False` 的提示回复成功不代表管理员权限映射或危险确认流程通过。此用例没有受权限保护的业务动作。

## 重复请求、历史与回归

- 同一实例再次请求 text-probe，返回同一本轮结果与两条回复，未再次调用原生入站。
- 运行原有模块 probe，五类入口、两组 source 仍通过，共 10 次命中。
- 模块 probe 后再次请求 text-probe，仍返回同一本轮缓存，没有新增输入。
- 通过 SQLite `mode=ro` 连接，仅按本轮精确 source／userid／固定文本查询测试记录：A、B 各 1 条，合计 2 条，action 均为 0。未直接写入或删除数据库。
- 本轮仅由 MP 原生流程保存这两条测试输入历史，记录保留；没有假称卸载会清除历史。

## 清理与环境状态

通过官方 API 停用后，status 显示 enabled=false、messages=0、calls=[]、text_replies=0、text_result=null。再通过官方卸载接口移除动态 API；status 返回预期 HTTP 404，客户端退出码 1。

删除前校验远端源码哈希及目录内容，仅移除了本次 `/app/app/plugins/relaynestprobe`。最终该目录不存在；原有 8 个安装插件 ID 和顺序与上一轮一致。MP 按正常机制保留探针停用元数据及操作日志。

容器仍为 running / healthy，StartedAt 保持 `2026-10-14T02:45:30.155912298Z`。没有重启、改 Compose、改 app.env、改通知配置、改其他插件或改 MP 核心；凭据仅在容器 loopback 请求头中使用，未输出或落盘。

## 验证和遗留

本地 `PYTHONDONTWRITEBYTECODE=1 python integrations/moviepilot/poc/verify_probe.py` 通过。宿主桩检查包括未知监听器拒绝、其它 provider 拒绝、配置名冲突拒绝、WebAgent 提取结果非空时拒绝、固定输入和回复身份限制、重复请求缓存、模块用例不重置缓存、停用状态清理。这些桩检查与上面的真实主进程实测分开记录。

全部探针 Python 文件通过语法编译检查；报告 JSON、实际部署源码 SHA-256 和文档相对链接检查通过；`git diff --check` 通过。仅提交本轮探针及报告文件，保留用户已有的 registry.json 和 .directory 改动。

NOT_RUN：主动广播（同步／异步）、其他实际通知渠道非回归、任意文本、真实搜索和候选选择、业务权限及危险确认、群聊／引用上下文映射、结构化 callback、自动编辑、真实附件、真实 Hub、并发双 Bot／多设备、容器重启恢复、v3.0.0 运行兼容、独立审计。

下一步可在插件侧研究 source/user/chat/message 映射的公开扩展契约，并分别验证媒体与种子候选。原生 callback 和编辑能力仍需独立处理，不能因文本闭环通过而豁免。
