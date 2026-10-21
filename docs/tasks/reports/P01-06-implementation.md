# P01-06 MoviePilot 兼容性门槛结论

日期：2026-10-21。执行基线：`c49acb359e9170fc9263c2649322d62261092d95`；探针精确实现：`061c0b8`（沿用 `498d5b7`、`213f832`、`8e8afa6`）。本轮只新增 P01-06 报告与可复现检查记录，不修改探针、产品设计或任务台账。

## 门槛判定

**G-MP：BLOCKED。当前不能声称专用插件已实现“不改 MP 核心、独立渠道、完整双向”。** 已证明若干 source-only 插件路径可用，但结构化 callback、引用上下文、配置到独立渠道的完整链路未满足必需验收。该判定也不等于证明所有可能的纯插件方案均不可行。

P01-05 尚未独立审计及集成为 done；本报告按用户“继续下一步测试”形成补充检查及门槛研究结论，**不将前置任务、P01-06 或 P1 整阶段标记完成**。沿用不启动子代理的指示，未执行独立审计；本报告不是作者自审后的审计凭证。

最低适配目标仍是 v3.0.0，拟议市场声明 `>=3.0.0` 与实际验证范围分开。实际运行基线仅 v3.1.5 当前镜像和固定合成用例，v3.0.0 及之后其它版本均不得据此宣称兼容。

## 五步链路与补充验收矩阵

| 必需项 | 已有实测证据 | 当前判定 |
| --- | --- | --- |
| 自定义配置保存→独立插件身份/重启保留 | 前端/后端源码确认只有 name/type 不能注册 channel；未在现网保存 | NOT_RUN；不满足完整链路 |
| 主动通知 | 同步/异步明确目标→HTTP桩，每 Bot 各一次 | 局部 PASS；无目标广播和真实管理员/用户路由 NOT_RUN |
| 文本入站 | 固定 /noai 经原生 handle_message，未改业务链 | 局部 PASS；Long Polling、任意业务命令、权限与危险确认 NOT_RUN |
| 定向回复 | 两组 source/userid 各一次原生提示→HTTP桩 | 局部 PASS；chat/message 上下文未保留 |
| 按钮 callback | 空 channel 能力 false；原生调用捕获 AttributeError | FAIL；G-MP 阻点 |
| 媒体/种子候选 | 16 项原生合成候选控制→HTTP数量摘要通过 | 局部 PASS；完整内容、有效选择及业务执行 NOT_RUN |
| 直接发送与编辑 | HTTP桩返回自有ID、同ID更新各2次 | 手动局部 PASS；原生自动编辑 gate 不进入 provider |
| 图片/文件 | 精确夹具URL/路径读取→字节上传校验 | 局部 PASS；正式附件元数据/故障矩阵未完成 |
| 多入口不漏不重 | P01-05 24条创建、2次编辑、同键重放不新增 | 限定路径 PASS；无广播、持久重放、真实渠道组合证据 |
| 停用/卸载/重载/重启 | 每轮清理；重载后旧模块回归通过 | 重启、持久恢复、进行中停用 NOT_RUN |
| 多Bot/多设备 | 命名空间userid的合成候选互不影响 | 同userid跨source覆盖；真实群会话/多设备 NOT_RUN |
| v3.0.0 | 固定源码及旧基类导入回退 | 运行 NOT_RUN |

历史完整证据：[P01-01](P01-01-implementation.md)、[P01-02](P01-02-implementation.md)、[P01-03](P01-03-implementation.md)、[P01-04](P01-04-implementation.md)、[P01-05](P01-05-implementation.md)。源码事实、容器独立进程组件检查、宿主桩、主进程和临时HTTP接收桩各有不同证明范围，不能互换。

## 本轮新增测试

完整命令、脚本和结果保存在 [P01-06 检查记录](P01-06-checks.md)。

1. **现有本地回归 PASS**：`PYTHONDONTWRITEBYTECODE=1 python integrations/moviepilot/poc/verify_probe.py`。沿用既有探针，不另写生产实现。
2. **新增11项本地HTTP桩负面检查 PASS**：同键不同载荷、外部source、外部userid、跨Bot编辑、未知消息ID、损坏附件均返回400；拒绝请求不修改消息；编辑后重放原创建载荷不回滚编辑；夹具文件缺失不发送；关闭后端口不可达；异常退出不残留线程/文件。均为协议桩测试，不是MP主进程、正式Hub或持久幂等验收。
3. **容器独立进程模型回归 PASS（符合预期限制）**：IncomingMessage、Message、MessageResponse 接受 channel=None 并保留 source，全部拒绝 channel="RelayNest"（channel/enum）。None 与字符串 RelayNest 的 buttons/callbacks/editing 共6个能力查询均 false。通知路由scope只在运行态，不进入model_dump。
4. **实际宿主函数＋记录桩检查 PASS（证明阻点）**：调用容器真实 `update_or_post_message()`，提供原消息/聊天ID，channel=None；记录链的 edit 调用为0，post回退为1。即使手动编辑provider返回成功，宿主在调用它之前仍受能力门槛阻止。回退消息能保留显式提供的两个ID，不能据此推断上游文本交互会传递它们。
5. **远端残留检查 PASS**：探针目录不存在，status HTTP404，临时夹具目录无残留，8个插件ID与顺序不变；容器healthy、StartedAt不变。

本轮未重新安装探针，未调用 handle_message 或真实发送方法，未新增原生测试输入历史。容器脚本经 stdin 在独立进程运行，未将临时源码写入远端、未读取/输出凭据；只有既有API客户端在loopback请求头中使用容器现有API_TOKEN以读取插件清单和status。没有停用现有插件、改变通知设置、重启容器、清除数据库或实施核心补丁。

## 扩展契约与阻点

| 路径 | 已确认契约 | 限制 |
| --- | --- | --- |
| 插件模块 | get_module声明 post_message、post_medias_message、post_torrents_message、send_direct_message、edit_message | provider扩展面存在，不会同步扩展渠道模型和能力表 |
| 入站 | MessageChain.handle_message 接收 source/userid/原消息上下文/callback_data | source不是渠道能力授权；is_channel_admin必须由MP身份映射产生 |
| 普通通知 | 同步/异步路由→NoticeMessage→消息队列/provider | 有userid可避免本轮管理员广播；广播仍须评估真实targets及运行态scope丢失 |
| 候选 | 专用候选provider，普通控制提示走post_message | 仅监听NoticeMessage会漏；本轮摘要不代表完整对象协议 |
| 直接发送 | MessageResponse返回source/message_id/chat_id | 正式插件仍需持久映射；手动provider可调用不等于自动编辑可用 |
| 模块错误 | 当前契约默认 isolate_provider | 顶层无异常不能证明投递；P01-05失败轮次已有反例，需核对实际回执 |

具体阻点：

- **渠道模型**：`app/schemas/types.py::NotificationChannel` → `app/schemas/message.py::{IncomingMessage,Message,MessageResponse}`，新字符串在模型边界被拒绝。
- **callback**：`app/chain/message.py::_handle_message_core` → `ChannelCapabilityManager.supports_callbacks(channel)`；空channel为false，不支持日志又访问channel.value。修复日志空值只能去掉异常，不能赋予callback能力。已有真实调用只保留异常类型，不能冒称取得精确调用栈。
- **自动编辑**：`app/application/messaging/interaction/__init__.py::update_or_post_message` → 同时要求原消息ID、聊天ID和supports_editing(channel)，失败则post回退。本轮容器组件检查直接证实空channel不会调用edit provider。
- **会话/引用**：原生交互store按userid索引；source不参与隔离。文本路由适配链未完整传递原chat/message上下文。插件命名空间和上下文映射需单独设计验证，不能把Hub管理员直接映射为MP管理员。

## 若继续完整双向，需要先确认的方案范围

未发现已验证的完整纯插件方案；不擅自选用下列重大变更。本轮仅给出后续设计输入，不实施，也不对外发送上游请求。

| 候选方向 | 最小待评估改动 | 维护成本/影响 |
| --- | --- | --- |
| 上游可扩展渠道与能力接口 | 模型接受已注册渠道身份；按插件注册能力；配置/发现与路由携带独立身份；callback保持MP原生授权流程 | 首次接口设计与上游协作成本较高；需分别验证v3.0.0/v3.1.5，不能假定旧版本可直接使用 |
| 自维护专用RelayNest渠道分支 | types中的身份、notification能力、消息模型/路由契约、配置服务发现；另补上下文传播和None安全处理 | 每次MP升级需处理核心/前端差异和完整回归；违背当前优先不改核心的目标，必须明确授权 |
| 继续source-only插件研究 | 不改能力表，只补插件侧映射/完整附件/故障与重载验证 | 可继续积累局部证据，但现有callback gate仍不通过，不能作为完整AC-07替代方案 |

只给None强开callback/editing、借用QQ/WebAgent、猴子补丁均不作为可接受的自动下一步。只修callback日志、只加枚举或只实现edit provider也不足以让门槛通过。正式Hub仍只传输/存储/路由消息和通用交互，MP继续负责命令、业务权限及危险确认。

下一步应先对“独立渠道身份＋能力入口＋会话上下文”形成可审查的具体方案；涉及上表的核心/架构变更须等用户选择。现有最小测试授权不等于已授权自维护核心分支、重启或真实业务测试。

## 指纹与环境

容器 `moviepilot-v2`，镜像 `jxxghp/moviepilot-v3:3.1.5`；始终running/healthy；StartedAt `2026-10-14T02:45:30.155912298Z`。当前文件SHA-256：

| 容器文件 | SHA-256 |
| --- | --- |
| app/schemas/message.py | `047d6f96ac36658e6deb4cf1b8736c6f33825b33f64a8bcfeea14acd4562f82a` |
| app/schemas/notification.py | `a83d9e1123fea976e30dad3bd258e63813157d1d2373ac9074ed55335983b31d` |
| app/chain/message.py | `af8fe2c615cc55c6e983c68b0b4f052ad2d42b9e83a109f1658a94b7aa03b0bb` |
| app/chain/_messaging.py | `a098417f1b7b6abbc1d46e397c0181d44dba78e05ed2cba5b64eeccff77983a2` |
| app/application/messaging/interaction/__init__.py | `a415d8505f8081b10fcda4dcff5d56e007c2b09170023e644638e79544207261` |
| app/runtime/extensions/module/contracts.py | `db99026f54671b3bdee44169cd525940af35fcba71ae6ebe6bb0bfc334ef434a` |

前4项与既有固定v3.1.5研究文件一致，后2项为本轮实际容器读取指纹，不冒称已逐字核对固定commit。固定外部研究ref保持P01-05记录，不更新为最新版本。

## 交接与验收

```yaml
task_id: P01-06
role: researcher
outcome: blocked
base_sha: c49acb359e9170fc9263c2649322d62261092d95
implementation_sha: 见本报告所属独立本地提交及交接回复
worktree: /home/lijx/Projects/RelayNest
changed_paths:
  - docs/tasks/reports/P01-06-implementation.md
  - docs/tasks/reports/P01-06-checks.md
summary: 完成11项协议桩负面检查和容器模型/编辑gate只读复核；整理完整证据，G-MP仍BLOCKED。未改核心/能力、未重放真实业务，未独立审计或更新台账。
acceptance:
  five_step_chain_and_candidates_edit: not_run_or_partial
  core_change_decision: pending_design_no_change_implemented
  g_mp_evidence: blocked_conclusion_documented
  relevant_checks: pass_with_evidence_limits
  independent_audit_and_integration: not_run
unresolved:
  - callback能力、独立渠道配置和上下文映射
  - 广播、v3.0.0运行、重启、持久恢复及正式端到端
  - 前置任务及本报告独立审计与集成
```

用户已有 registry.json、.directory 和执行期间出现的Android修改均不暂存/提交；未启动子代理、新聊天、推送或发布。

提交授权记录：此前两次暂存申请因自动权限审核超时未执行；用户随后明确允许仅提交两份 P01-06 报告。本次按该授权创建独立本地提交，不推送。
