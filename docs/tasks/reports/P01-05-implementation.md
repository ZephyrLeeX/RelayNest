# P01-05 编辑、附件与多入口最小验证

日期：2026-10-21。探针前置实现 `498d5b7`；提交前仓库 HEAD `7cf1a3148ffebfd05cc8f4d6d5a6ae9b5ed6fadc`（含独立 P00-04 提交）。本任务实现 SHA 见本地 Git 提交，不修改台账，不代替独立审计。

## 结论

**v3.1.5 主进程向临时 loopback HTTP 协议桩的最小矩阵通过：24 次创建、2 次同 ID 编辑、1 次显式重放未新增。** source-only 插件可以承接限定的出站传输及固定原生文本/候选控制，但仍不能证明专用插件已经实现完整独立渠道双向通信。**P01-05 全部验收、P01-06、P1/G-MP 和 AC-07 均未据此通过。**

现有实例测试沿用用户明确授权，覆盖任务卡原先要求的隔离实例环境选择。未触发搜索、下载、订阅或危险业务；未改 MP 核心、枚举、能力表、用户通知设置或现有插件；不借用 QQ/WebAgent，不连接正式 Hub、Android、FCM、RelayShelf。

## 实现与证据分层

- 在 0.0.3 上扩展到 0.0.4，新增 `transport.py`，沿用原有五类 provider、传播检查、固定原生文本及候选夹具，不重写宿主消息链。
- 传播检查增加其它直接发送/编辑 provider 拒绝；仍只排除真实提取函数已拒绝空 channel 的内置 WebAgent 事件桥，没有关闭它。
- 新固定、API-key 保护的同步 HTTP 入口 `transport_probe` 在请求期间建立随机 loopback HTTPServer 和临时夹具目录，finally 停止服务、关闭 socket、join 线程并删除夹具。不常驻轮询，不接受任意命令。
- 临时桩通过 HTTP 接收 JSON 并产生 UUID 消息 ID，按 source/userid 核对编辑目标，修改同一内存行的正文和 revision。它与 MP 在同进程不同线程，不代表跨机器网络、Hub 落库或 Android 显示验证。
- 图片由插件读取本轮唯一允许的 loopback URL；文件只读本轮临时夹具路径；插件把字节转为 base64、附完整性哈希发送。桩解码后与固定 PNG/文本字节逐一比较。没有要求桩抓取 URL，没有使用真实用户附件。
- 候选传输的是类型、目标、消息标题/正文及数量摘要，不包含完整 MediaInfo/Context、按钮或真实媒体对象。它证明专用 provider 输出未漏入传输桩，不代表正式候选内容转换完成。

四层证据分别是：固定源码调用链；本地轻量宿主桩拒绝/清理回归；实际 v3.1.5 主进程原生调用；主进程到临时 HTTP 桩传输。后两者组合仍不是正式 Hub/客户端端到端证据。

## 源码、环境与指纹

固定研究来源保持不变：

| 对象 | 固定 commit | 本轮范围 |
| --- | --- | --- |
| MP v3.0.0 | `773d8490645e5b7dd3c07a397e6e6d5ac58ef48e` | 沿用源码对照，运行 NOT_RUN |
| MP v3.1.5 | `99ee18d0e6a27422a4affd3d4ec7d8d56772e4c3` | 主要源码和实际主进程验收基线 |
| Plugins | `7b6c84ce506791c85c1160d4be8766c8763870f4` | 沿用既有研究，未安装市场插件 |
| Frontend v3.1.5 | `21166c45fc1596af42978659ddf253b207b939cf` | 沿用研究，未保存生产通知配置 |

SSH 显式 `-F /home/lijx/.ssh/config fnos`。容器 `moviepilot-v2`，镜像 `jxxghp/moviepilot-v3:3.1.5`。每轮前后均 running/healthy，StartedAt 始终 `2026-10-14T02:45:30.155912298Z`。

本轮容器源码复核：`app/chain/_messaging.py` SHA-256 `a098417f1b7b6abbc1d46e397c0181d44dba78e05ed2cba5b64eeccff77983a2`；`app/schemas/message.py` 为 `047d6f96ac36658e6deb4cf1b8736c6f33825b33f64a8bcfeea14acd4562f82a`，与先前固定源码一致。

最终部署源码与本地文件哈希一致：

| 文件 | SHA-256 |
| --- | --- |
| `relaynestprobe/__init__.py` | `00f8d630e9bc5536432a6f31d4455f0b10ea12513addefd7f5f7c4780691c5e2` |
| `relaynestprobe/transport.py` | `cf7568eb90f30fe9c37020d97ebbdbcef8eb0fc4a2f07d6439f914ad67e6ce0a` |

## 调用链和唯一发送责任

```text
固定主动通知（明确 source/userid，无 mtype，不保存历史）
  → MessageChain.post_message / async_post_message
  → 原生路由：有 userid，action=None，不查真实管理员目标
  → NoticeMessage（现有 WebAgent 桥拒绝空 channel）
  → 消息队列 immediately=True → 探针 post_message → HTTP 桩

固定 /noai → 原生 handle_message → 原生提示 → post_message → HTTP 桩
原生候选夹具渲染 → post_medias_message / post_torrents_message → HTTP 桩
候选越界/退出提示 → post_message → HTTP 桩
send_direct_message → 宿主 run_module → provider → HTTP 创建 → MessageResponse
edit_message → 宿主 run_module → provider → HTTP 编辑 → 同一行 revision=2
```

探针不监听 NoticeMessage：事件没有第二份发送责任。候选列表由专用 provider 负责，候选控制提示与文本回复在普通 provider 中按固定上下文互斥捕获。同步和异步各调用一次发送方法，不把一种通知同时投给事件监听器和 provider。

这些是带目标的合成主动通知，**无 userid 的广播、管理员/用户通知配置路由为 NOT_RUN**：该路径会读取真实用户渠道配置并可能广播，不能在本轮不改用户设置的边界内安全执行。source 名字本身不是广播隔离保证。

## 最终主进程结果

传播预检：success=true；活动 NoticeMessage handler 只有已核实的 WebAgent 桥；其它五类插件 provider、同名通知配置及 blocked handler 均为空。测试中未禁用原有插件或监听器。

| 路径 | A | B | HTTP 桩证据 |
| --- | --- | --- | --- |
| 同步主动定向通知 | 1 | 1 | 恰好创建 2 行 |
| 异步主动定向通知 | 1 | 1 | 恰好创建 2 行 |
| 直接发送 | 1 | 1 | 2 个接收桩生成 ID，均属于对应目标 |
| 同 ID 编辑 | 1 | 1 | 原行正文变更，revision=2，未新建消息 |
| 图片与文件 | 各 1 | 各 1 | 4 个附件字节及哈希均与夹具一致 |
| 固定原生 /noai 回复 | 1 | 1 | 各 1 行，source/userid 正确 |
| media 候选首屏/下一页 | 8/1 | 8/1 | 4 行列表摘要 |
| torrent 候选首屏/下一页 | 8/1 | 8/1 | 4 行列表摘要 |
| 两种候选越界/退出提示 | 4 | 4 | 8 行提示，未执行有效选择 |

合计 A/B 各 12 行，共 24 行；另有 2 次编辑。候选全部 16 个控制断言通过，自有 request_id 全部移除。原生 callback 仍为 supported=false、returned_without_error=false、error_type=AttributeError；原生自动编辑能力仍 false。原生文本回复 chat/message 上下文仍均 false。

最终响应核心摘要：

```json
{
  "success": true,
  "error_type": null,
  "kind": "main_process_to_temporary_loopback_protocol_stub",
  "counts": {"creates": 24, "edits": 2, "replays": 1},
  "text_success": true,
  "candidate_success": true,
  "broadcast": "NOT_RUN",
  "persistent_replay": "NOT_RUN",
  "temporary_receiver_closed": true,
  "fixture_removed": true
}
```

响应中两个 source 的 sync/async/direct/attachments/edit，以及 native_text/native_candidates/path_counts/explicit_replay/same_text_distinct 共 15 个断言均为 true。同一已加载实例再次请求 transport-probe，响应逐字段相等，返回缓存，没有重启接收桩或再次入站。

## 身份、去重和重放的实际边界

同正文的同步/异步两条真实测试发送保留不同 ID；没有用正文哈希去重。附件 SHA-256 只做完整性校验。

本轮显式键为随机 run ID + 发送序号；仅在重传同一已构造载荷时复用，桩以该键返回原 ID，并拒绝同键不同载荷。重放第 1 条后仍为 24 行，不代表插件已获取 MP 稳定事件身份。没有证明宿主队列重试、多个路由分支、崩溃恢复能重用同键，也没有持久保存消息映射或去重状态。路径责任和本轮数量检查通过，与持久幂等通过是两个结论。

## 两轮失败与纠正

前两轮均整体 success=false：sync/async/direct/edit/附件及文本通过，但只有 8 次 HTTP 创建、2 次编辑、1 次重放；候选 16 项均无捕获，整体路径数不符。没有把这些失败计入最终通过结果。

首轮采用 async handler，先怀疑调度时序，清理后改成同步入口，在工作线程中用 asyncio.run 单独调用宿主异步通知；第二轮仍失败，排除了该假设作为主要原因。实际发现接收桩仅允许 `<source>:` userid 前缀，而既有候选使用 `RelayNest-P1Probe:<phase>:<A/B>:`，导致 HTTP 400。宿主调度没有将 provider 错误作为候选顶层 error_type 返回，所以无异常不等于已投递。

最终仅增加 media/torrent 两种固定测试身份前缀白名单；未放宽到真实用户/任意 source。新增本地回归确保原生候选测试前缀能传输。三轮入口均执行两条固定 /noai 和十二条候选控制输入；本轮调用共 42 条固定测试输入，MP 原生流程可能保留相应历史。本轮未直接核对数据库最终行数，不把调用数冒充数据库计数，未清除历史或其它用户数据。

三轮结束都执行官方 disable/status/unload，停用状态确认全部内存计数和结果清空；卸载后 status 404；校验两文件哈希及目录中仅自有源码/pycache 后删除。每轮临时目录扫描无残留，安装清单及健康/启动时间不变。

## 重载与最终清理

最终成功后官方 reload 返回 success=true，status 显示 transport/text/candidate 结果均 null、所有内存记录为空。只运行旧模块 probe 回归，10 次命中通过；没有重载后再次执行原生输入。故**重载后旧模块分发通过，重载后 HTTP 传输/幂等恢复为 NOT_RUN**。

最终 disable 后 enabled=false，messages=0、calls=[]、text_replies=0、candidate_calls=0、transport_active=false，全部结果 null。unload 成功，status HTTP 404。探针目录不存在，`/tmp/relaynest-p105-*` 无残留。保留 MP 正常卸载后的停用元数据及操作日志，不直接改数据库。

原有插件 ID 和顺序每轮一致：P115OfflineDownloader、PluginUnInstall、RemoteIdentifiers、CategoryEditor、PersonMeta、BangumiColl、SubtitleManualUpload、P115StrmHelper。

## 剩余兼容性矩阵

| 能力 | v3.0.0 | v3.1.5 当前证据 | 缺口/判定 |
| --- | --- | --- | --- |
| 插件导入与五类模块 | 仅旧基类回退源码，未运行 | 主进程通过；重载后模块回归通过 | 最低版本运行 NOT_RUN |
| 自定义通知保存/恢复/服务发现 | 源码研究 | 未操作 UI/配置 | NOT_RUN；配置 name/type 不等于 channel |
| 同步/异步明确目标通知 | 未运行 | 原生链→HTTP 桩各 2 条通过 | 正式 Hub NOT_RUN |
| 无目标主动广播、管理员/用户路由 | 源码研究 | 路由及事件已追踪 | NOT_RUN，避免真实渠道扩散 |
| 固定文本入站及回复 | 未运行 | 原生 /noai→HTTP 桩，两组通过 | 任意文本、权限和危险确认 NOT_RUN |
| 媒体/种子候选控制 | 未运行 | 原生合成候选16项→HTTP数量摘要通过 | 完整内容协议、有效选择业务 NOT_RUN |
| 结构化 callback | 固定能力模型限制 | false；真实调用 AttributeError | BLOCKED，未改能力/核心 |
| 引用与会话上下文 | 未运行 | 同 userid 跨 source 会覆盖；回复上下文丢失 | BLOCKED；命名空间 userid 仅局部隔离证据 |
| 直接发送与手动编辑 | 未运行 | 主进程→HTTP桩返回 ID及原行更新通过 | 原生自动编辑 false，正式 ID 持久映射未实现 |
| 图片/文件 | 模型存在，不是传输证据 | 固定 PNG URL/本地文本文件→HTTP字节通过 | 真附件、MIME/文件名保留、大文件、不可读/重定向/内网URL策略 NOT_RUN |
| 多入口不遗漏不重复 | 未运行 | 本轮24行数量、唯一provider责任通过 | 广播及真实插件/渠道组合 NOT_RUN |
| 同键重放、同文不同消息 | 未运行 | 本轮载荷键重放和不同ID通过 | MP稳定事件键、持久Outbox/Inbox/ACK、崩溃恢复 NOT_RUN |
| 停用/卸载/重载 | 未运行 | 每轮清理通过；重载清空、模块回归通过 | 进行中并发停用/重载和状态恢复 NOT_RUN |
| 容器重启/重新部署 | 未运行 | 未重启现有实例 | NOT_RUN，最小范围内避免服务中断 |
| 正式Hub、Android、多设备 | 未运行 | 无 | NOT_RUN，明确排除范围 |
| 独立审计、P01-06/G-MP | 无 | 未执行 | 不得标为通过 |

最低适配目标仍 v3.0.0，拟议市场声明 `>=3.0.0` 不是验证范围，也不是本次批准的发布声明。实际运行仅覆盖 v3.1.5 当前镜像、当前插件组合与固定用例，不宣称后续所有版本兼容。

## 架构阻点与后续决策输入

本轮未证明必须改核心，也未找到完整独立渠道方案。已有明确受阻链：`handle_message(callback_data)` → `supports_callbacks(None)=false` → 不支持分支访问 `channel.value`；以及原生候选自动更新 → `supports_editing(None)=false`。修正空值日志本身不能赋予 callback/编辑能力。

如后续选择新增独立渠道，最小候选方向需同时评估 Message/IncomingMessage/MessageResponse 的渠道模型、能力注册、通知配置/发现、callback身份及交互上下文传播；不是只加枚举一行即可。需要分别维护 v3.0.0 与 v3.1.5 的 SDK/路由差异、配套前端和每版回归；成本高于 source-only 插件。另一方向是先向上游提出可扩展渠道/能力接口，待接口存在后重新验证。上述仅是缺口和评估入口，尚非已确认设计；本轮没有实施核心/能力修改、渠道借用或权限边界变更。正式方案须给出更精确 diff、版本成本后由用户选择。

## 检查与交接

- `PYTHONDONTWRITEBYTECODE=1 python integrations/moviepilot/poc/verify_probe.py`：通过，覆盖旧边界及 HTTP ID/编辑/附件/显式重放、非夹具URL拒绝、候选命名空间回归、线程/文件清理；仅宿主桩。
- 实际 MP 官方 API：enable/load/preflight/transport-probe（两轮失败、一轮通过）、缓存重复请求、reload/status/probe、disable/status/unload/status 404；结果如上。
- Python语法编译、`git diff --check`：交付前通过。未运行无关工程测试。
- 仅提交 `integrations/moviepilot/poc/**` 与本报告。用户已有 registry.json、.directory、mise.toml 及执行期间出现的 android/ 均不暂存、不提交；P00-04 已在独立提交中，不改动。没有推送、创建聊天或启动子代理。
