# P1 候选控制、会话隔离及回调验证

日期：2026-10-21。RelayNest 父基线：`213f832`。实现 SHA 见本地交接。

## 结论

**在现有 MoviePilot v3.1.5 主进程中，合成媒体／种子候选的首屏、越界数字、翻页和退出共 16 项控制检查通过。** 使用不同命名空间 userid 的两组 source 同时存在并交错操作，另一组会话未被改变。

同时得到三个明确缺口：

1. 宿主媒体会话按 userid 索引；同 userid 创建不同 source 的会话，会替换前一会话。独立 source 本身不足以隔离 Bot。
2. 显式向原生渲染器提供的 chat/message ID 能出现在首屏消息；经原生文本交互翻页后的消息不再包含它们。
3. `channel=None` 的原生结构化 callback 能力为 false，真实调用未正常返回，捕获异常类型为 AttributeError。

本轮没有执行有效候选对应的实际搜索、订阅或下载。**候选控制通过不等于有效选择执行通过；P01-04 全部验收和 G-MP 仍未通过。** 未修改任务台账，未豁免回调、引用或业务权限门槛。

## 实现与限定范围

- 探针升级到 0.0.3，新增仅 MP API 密钥可调用的 `candidate_probe`，不接受任意输入。
- 用宿主真实 MediaInfo、TorrentInfo、Context 创建无真实 ID、地址或站点凭据的合成数据，通过原生 MediaInteractionManager 播种自有内存会话。
- 通过真实 MediaInteractionChain 渲染候选列表，媒体和种子分别进入专用发送 provider；不是直接调用插件方法冒充原生候选发送。
- 每类 9 条候选，按当前宿主文本分页大小 8 验证两页。没有真实索引器搜索或外部站点访问。
- 后续输入统一走未修改的 `MessageChain.handle_message()`。只允许 `/noai 999999`、`/noai n`、`/noai q`，显式 is_channel_admin=false。`0`、有效序号和其它业务输入在探针入口被拒绝。
- 只捕获精确测试 source／userid、channel=None、save_history=false 且标题具有本轮前缀的候选；普通回复只收集固定越界／退出提示。
- 仅按本轮记录的 request_id 清理内存会话；没有调用宿主 store.clear()，没有禁用监听器或修改业务权限。
- 未修改 MP 核心、NotificationChannel、能力表；无猴子补丁、后台线程、Hub、Android、FCM 或 RelayShelf。

用户此前明确选择现有 fnOS / Compose 实例做最小测试，本轮沿用该授权。

## 宿主调用链及固定证据

目标仍为 MP v3.1.5，先前固定 commit `99ee18d0e6a27422a4affd3d4ec7d8d56772e4c3`。本次读取的实际容器文件指纹：

| 文件 | SHA-256 |
| --- | --- |
| `app/chain/interaction.py` | `6fb1370ac03dbfd39754ab336a0cd1ed375ab9ad3ab8c9f90ca945cd059856ab` |
| `app/application/messaging/media.py` | `c74d4b6d8ca478c97dbe5c7a11df2a0100ef74563bacb1e3dd8f885ee727f983` |
| `app/application/messaging/router.py` | `d9e21bdfdd0833b9e45144ebc4763e37f5a1e5e131921e65862b4dcda9c514f4` |
| `app/chain/message.py` | `af8fe2c615cc55c6e983c68b0b4f052ad2d42b9e83a109f1658a94b7aa03b0bb` |

最后一项与此前固定 commit 文件相同。新增读取文件以上述实际容器哈希记录，不将未逐项比对文件冒充已核对 commit 内容。

实际部署探针源码 SHA-256：`55778af8cf3917d87add74a24c14a190a399bbc1d439253e2e8f4a054bf6343c`。

调用路径：

```text
原生 MediaInteractionManager：创建测试会话及合成 items
  → MediaInteractionChain._render_interaction
  → 原生 post_medias_message / post_torrents_message
  → 探针捕获列表

固定 HTTP 输入入口
  → MessageChain.handle_message（/noai + 白名单控制文本）
  → 原生 InteractionRouter.dispatch_active_text
  → MediaInteractionChain.handle_text_interaction
  → 越界提示／原生下一页／q 退出
  → 专用候选 provider 或普通 post_message
```

本轮传播检查沿用 P01-03：无其它活动消息／候选插件 provider，无同名通知配置；唯一活动 NoticeMessage 桥为已核实拒绝空 channel 消息的 WebAgent 桥。没有停用它。

## 候选控制实测

两种 phase 均为原生支持的 media、torrent，action 为 Search；不会发送使该 action 进入实际搜索或下载的有效候选序号。

| phase | source | 首屏 | 越界 999999 | n 下一页 | q 退出 |
| --- | --- | --- | --- | --- | --- |
| media | A | 8 条，通过 | 固定错误提示，通过 | 1 条，通过 | 会话移除，通过 |
| media | B | 8 条，通过 | 固定错误提示，通过 | 1 条，通过 | 会话移除，通过 |
| torrent | A | 8 条，通过 | 固定错误提示，通过 | 1 条，通过 | 会话移除，通过 |
| torrent | B | 8 条，通过 | 固定错误提示，通过 | 1 条，通过 | 会话移除，通过 |

每一步恰好捕获一条目标 source／userid 的消息。两组用户会话同时存在，按 A/B 交错输入；每步还对比另一组 request_id 和 page，均保持不变。所有列表 buttons_present=false，符合空 channel 的文本候选路径。

重要安全事实：源码 `_handle_torrent_selection(page_index=0)` 会进入目录选择或自动下载；0 不是取消。实际取消使用 q。本轮没有发送 0，也未发送任何有效序号，不把这轮结果写成成功下载或成功订阅证据。

## 会话与上下文

同一随机测试 userid 依次通过原生 create_or_replace 创建 source=A 和 source=B 两个会话；按 userid 查询得到 B，按 A 的 request_id 查询已不存在。实测 `same_user_replaces_previous_source=true`，与宿主 store 按 userid 索引的源码一致。这是管理器级覆盖验证，不冒充真实 Bot 账号串话实测。

正常控制用例使用不同命名空间 userid：`RelayNest-P1Probe:<phase>:<A/B>:<run>:user`，因此避免了该覆盖。后续正式适配必须明确 Bot／用户／会话身份映射；不能只改变 source 而复用原始用户 ID。

首屏由探针显式传入 original_chat_id、original_message_id，四组均保留；越界提示、翻页列表和退出提示四组均未保留。这区分了发送模型能够承载字段，以及原生文本交互是否传递字段。探针没有偷偷补齐后宣称原生字段完整。

源码对照：文本会话路由的适配器只转交 channel、source、userid、username、text；媒体文本处理器也未接收 chat/message ID。`reply_to_message_id` 虽然提供给 handle_message，本轮仍没有建立原生引用回复闭环，不能视为引用验收通过。

## 结构化 callback 的实际失败

候选会话完成后，使用独立随机测试 userid，向未修改的 handle_message 传入固定、无对应真实请求的 `media:relaynest-probe-<run>:noop` callback_data；channel=None、is_channel_admin=false，不执行任何业务 callback。

实际响应摘要：

```json
{
  "success": true,
  "kind": "native_candidate_fixture_controls",
  "same_user_replaces_previous_source": true,
  "owned_sessions_removed": true,
  "callback": {
    "supported": false,
    "returned_without_error": false,
    "error_type": "AttributeError"
  },
  "valid_selection_executed": false
}
```

这里 success 只表示候选控制用例通过，callback 单列失败。实际仅记录异常类型，没有保留宿主异常堆栈；不能将特定行号冒充实际堆栈证据。

源码定位：MessageChain 的 callback 分支先检查 supports_callbacks(channel)，不支持时日志参数访问 `channel.value`。空 channel 在该表达式会引发 AttributeError，与本轮捕获类型一致。无论该日志问题是否修复，独立渠道能力仍未赋予空 channel，不能据此宣称结构化 callback 可用。

没有尝试借用 QQ／WebAgent，没有动态扩展枚举或能力表，也没有将 callback 翻译成有效下载数字来绕过原生能力门槛。

## 重放、历史和清理

- 重复调用 candidate-probe 返回同一本轮缓存，不重建会话、不再次入站。
- 基础模块 probe 回归通过，仍为 10 次命中；之后 candidate-probe 仍返回原缓存。
- SQLite 只读查询仅匹配本轮 4 个 userid、两个 source 和 3 个固定文本：合计 12 条输入，12 个 user/text 组合各一次。记录由 MP 正常输入链保存并保留，未手工写入或删除数据库。
- 所有本轮自有 request_id 在 finally 中清理；主进程检查 owned_sessions_removed=true。正常 q 也已移除对应活动会话。
- 停用后 status 显示 enabled=false、candidate_calls=0、candidate_result=null，原有模块／文本内存状态也为空；随后官方卸载成功，按哈希及目录内容核对后删除远端源码。

收尾实测 status 返回 HTTP 404（客户端退出码 1 为接口不存在的预期结果），探针目录不存在，原有 8 个安装插件 ID 和顺序一致。容器仍为 running / healthy，StartedAt 保持 `2026-10-14T02:45:30.155912298Z`，没有重启。按 MP 正常机制保留停用元数据及操作日志，不直接删除宿主数据库行。

## 验证边界与下一步

本地宿主桩验证覆盖候选捕获、非夹具数据拒绝、错误身份拒绝、有效序号／0 拒绝及停用清理；候选数据另在实际镜像中通过真实模型序列化检查。真实原生控制行为由上面的主进程 HTTP 结果单独支持，不将桩验证冒充兼容性通过。

命令 `PYTHONDONTWRITEBYTECODE=1 python integrations/moviepilot/poc/verify_probe.py` 和 `git diff --check` 均通过。保留用户已有的 registry.json、.directory 和 P00-04 环境报告改动，不纳入本任务提交。

NOT_RUN：真实搜索生成候选、有效媒体选择后的搜索／订阅、有效种子选择后的目录／下载、真实业务权限及危险确认、群会话或引用映射、并发多设备、自动编辑、图片／文件、主动广播与路由去重、容器重启恢复、v3.0.0 运行兼容、独立审计。

当前证据已把问题收敛为：source-only 的文本及候选控制可走纯插件模块路径，但完整交互仍缺独立 callback 能力和稳定会话上下文映射。涉及核心、全局能力表、借用渠道或产品范围改变时，须先提交具体方案由用户决定；本轮未实施这些变更。
