# P03-07-S01-S01 实施交付

角色：developer。基线：`8e85887934d37777ec7f5cd2c9fa340f36c5f3ed`。独占工作树：`/home/lijx/.codex/worktrees/api-sync-read/RelayNest`，分支 `codex/api-sync-read`。本报告与实现同提交，精确实现 SHA 在开发交接消息给出（也可由 `git log -1 --format=%H -- server/api/openapi.yaml` 获取）。未派子 agent，未修改 registry、运行时、迁移、两锁、旧报告或其他卡；未 push/部署。

## 变更与验收映射

| 本卡验收 | 交付证据与边界 |
| --- | --- |
| Android 管理员权限、严格 Schema、正反例、错误/no-store、时间 | 两操作 security 仅 AdminBearer、x-client-type=android；正反权限夹具覆盖 Bearer/Web Cookie/CSRF/Bot/Webhook。严格对象复用 Error；成功及公共错误 no-store。服务端时间位置见下表。 |
| 固定 H、epoch、分页、游标绑定、floor/非法游标、不跳缺口 | `SyncPage` 六个 envelope keys；cursor 绑定 administrator_id/epoch/H/position/cycle_complete。默认及最大 limit=200，末页令牌非空；完成轮次下次请求冻结新 H。`x-sync-policy` 精确断言，cursor/batch 夹具覆盖续页、下轮新 H、篡改/跨身份、epoch、floor 小于/等于、初始 floor、自然序号空隙与遗漏事件拒绝。 |
| 读事务当前投影、删除无旧正文、实体版本、旧版幂等 | `SyncChange` 六类严格 oneOf；同读事务与 tombstone 声明；upsert 当前 revision 可高于事件；删除无 text/附件/buttons。投影/应用夹具覆盖更高 revision、旧投影拒绝、删除旧正文拒绝、旧 revision 与重复删除 no-op。Bot/Read/Delivery 均复用明确整数版本，不使用任意 object。 |
| 实际展示 bot_seq、验证上界/max/shared change、未读语义 | `UpdateReadRequest.through_seq`；`x-read-policy` 冻结曾存在最大 Bot 序号上界、原子 max 与前进同事务共享变更；未前进保持版本/时间且不重复事件。夹具覆盖前进、相等、离线旧读、越界、SQLite 最大值及未读排除自身/callback/过期。实际展示属于客户端责任，服务端不能证明实际展示。 |
| 数字/版本/权限/旧 revision/epoch/floor/删除、后卡交接 | 新增77例，合计446例（180 valid + 266 invalid）；覆盖 JS 安全整数以上、SQLite 边界/溢出、数字序号/前导零/负数/指数、非法版本/额外字段、权限与状态声明。bootstrap/WS/设备通知/设置留后续卡。 |

这些是 Schema、权限和状态声明验证；不是服务端事务、并发、客户端 Room、通知或 WS 运行结果。

## 冻结 keys 与 Schema

- GET `/client/sync`，operationId=`syncClientChanges`，query=`cursor,limit`；返回 `epoch,changes,next_cursor,has_more,server_high_watermark,server_time`。H 即 server_high_watermark；epoch 为不透明世代；floor 仅游标验证状态，不新增响应字段。省略 cursor 从当前 epoch 的0开始，照常检查 floor。
- 游标续页绑定原 H/位置，末页推进至 H 并完成轮次，下次重新冻结 H。非法/篡改/跨身份为 `400 INVALID_SYNC_CURSOR`；有效游标 epoch 不同或 position<floor 为 `410 SYNC_RESET_REQUIRED`。客户端提示历史部分保留并重建缓存，不能记为传输失败或悄悄跳水位。
- PUT `/client/bots/{id}/read`，operationId=`updateClientRead`；输入 `through_seq`，返回 `read,server_time`；read=`bot_id,read_seq,version,updated_at`。超过该 Bot 曾存在最大 bot_seq 为 `422 READ_SEQ_OUT_OF_RANGE`；不存在/无权为 `404 RESOURCE_NOT_FOUND`。停用 Bot 仍允许管理员读历史/同步/已读。
- 变更：`message.upsert(change_seq,message)`、`message.deleted(change_seq,id,bot_id,bot_seq,revision,deleted_at)`、`bot.upsert(change_seq,bot)`、`bot.deleted(change_seq,id,version,deleted_at)`、`read.updated(change_seq,read)`、`delivery.upsert(change_seq,delivery)`。change_seq 是事件序号，非 read_seq；当前投影实体版本可能高于事件记录版本。删除采用实体终态版本，不复活同一ID。
- 新 Schema 共12个：SyncEpoch、ReadState、UpdateReadRequest、UpdateReadResponse、SyncMessageUpsert、SyncMessageDelete、SyncBotUpsert、SyncBotDelete、SyncReadUpdate、SyncDeliveryUpsert、SyncChange、SyncPage。复用 Message、Bot、DeliveryMetadata 及其现有引用；Sequence/Version/UTCDateTime/OpaqueCursor/Error 不变。设置类型由原 S02/S03 显式扩展，不改变 H/epoch/floor。

| 服务端时间位置 | 语义 |
| --- | --- |
| SyncPage.server_time / UpdateReadResponse.server_time | 本次服务端响应/批次生成时间，不作并发比较 |
| ReadState.updated_at | 最近共享已读前进的服务端时间；无前进保持原值 |
| Message.created_at / updated_at | 消息创建/最近编辑；编辑不改变 bot_seq 或创建时间 |
| SyncMessageDelete.deleted_at / SyncBotDelete.deleted_at | 实体终态删除服务端时间 |
| Bot.connection.last_poll_at | 原 Bot 元数据轮询时间，允许 null |
| DeliveryMetadata.created_at / updated_at / lease_until / next_attempt_at / acked_at | 复用既有投递生命周期语义与状态约束 |

## 验证记录

解释器：`/tmp/relaynest-p0305-f01-locked/bin/python`，实际 `Python 3.13.15`；环境已存在，未恢复/升级依赖。

| 命令/检查 | 实际结果 |
| --- | --- |
| `.../python -m openapi_spec_validator server/api/openapi.yaml` | PASS，OpenAPI OK |
| `.../python server/api/validate.py` | PASS：73 schemas；180 valid + 266 invalid examples；25 operations/security |
| `.../python /tmp/relaynest-sync-checks.py`（本轮临时脚本） | PASS：对精确基线逐值比较全部旧23操作、61 Schema及全部旧组件/安全/公共响应/参数；旧369例逐值及顺序保留；两锁逐字节一致 |
| 同脚本12项临时副本破坏探针 | 全部 exit=1 被拒绝；未污染源码 |
| `git diff --check` | PASS |

破坏探针逐项：sync 改 WebCookie、read 改 BotAPIToken、两个成功响应各改 public 缓存、H 改 moving、read max 改 replace、移除 x-sync-envelope、through_seq 改 integer、删除分支放行额外正文、floor 夹具错判200、漏事件夹具错判可接受、离线旧读夹具错误回退水位。运行时每项独立复制 YAML/examples/validator 到临时目录并确认非零退出。

锁 SHA256：requirements.in=`4d56f1903c85ca444dde0966a7fd2702f15242b2cbc5f35eb1fcd273a0317d7c`；requirements.txt=`4e1992c8e79a37d1671671afd5e24f686837f719078ac7bdab06b5919231f184`。基线全部原操作/组件/例子保持一致；有意新增仅同步/共享已读合同、77合成例与对应声明检查，未弱化原安全/数字/格式。

NOT_RUN：HTTP服务、SQLite事务/并发、Android/Room/真实WS/通知撤销、手机/OPPO账号/NAS。按授权留后，不阻本卡契约审查；不放行 G-API 或改变 P1/P2 用户放行。

## 后卡精简交接（≤800中文字）

本卡仅sync/read：GET /client/sync返回epoch/changes/next_cursor/has_more/server_high_watermark/server_time；游标绑定管理员/epoch/H/position/cycle_complete，续页固定H，完成后下一轮取新H；省略游标从0开始仍检查floor。有效游标epoch不符或position<floor→410 SYNC_RESET_REQUIRED；非法/篡改/跨身份→400 INVALID_SYNC_CURSOR。不跳缺口。SyncChange六类message.upsert/deleted、bot.upsert/deleted、read.updated、delivery.upsert，同读事务取当前投影，高revision合法，删除无正文，旧版本/重复删除no-op。PUT read输入through_seq=实际展示bot_seq，验证曾存在最大序号后max合并，前进才更新version/updated_at及共享事件；越界422 READ_SEQ_OUT_OF_RANGE，无权/不存在404 RESOURCE_NOT_FOUND。未读仅现存application且bot_seq>read_seq。时间为服务端server_time及实体时间，不决定并发。新增12Schema，冻结25操作/73Schema/446例及两锁。bootstrap/WS留S01-S02/S01-S03；设备通知/全局设置留原S02/S03，只显式扩展变更分支。所有验证仅契约声明，未做运行时/真机/WS实测。
