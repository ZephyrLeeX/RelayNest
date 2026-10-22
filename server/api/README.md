# RelayNest API 契约

`openapi.yaml` 是 OpenAPI 3.1.0 契约，统一前缀 `/api/v1`。当前已定义认证、Bot 管理、两套凭据、消息创建/编辑、独立 callback、标准 Webhook、Android 历史/详情、附件及应用投递/失败管理；不表示 Go 服务已实现这些路由。P03-07 继续串行补充设备、设置、同步、已读与 WS。

依据：`docs/TECHNICAL_DESIGN.md` §2、§3、§4、§5.3、§7、§8；安全边界遵循 `docs/ARCHITECTURE.md` §1/§5。唯一管理员离线初始化/重置密码，不开放注册，不依赖 RelayShelf。

## 校验

仓库根目录运行（Python 3.13；推荐使用项目已锁工具环境，不必为本卡安装 Gradle）：

```sh
python3 -m venv /tmp/relaynest-api-venv
/tmp/relaynest-api-venv/bin/python -m pip install --require-hashes -r server/api/requirements.txt
/tmp/relaynest-api-venv/bin/python server/api/validate.py
```

`requirements.in` 是直接校验依赖，`requirements.txt` 固定全部传递版本和发行包哈希；它们不属于服务运行时依赖。更新时用 `uv pip compile server/api/requirements.in --generate-hashes --output-file server/api/requirements.txt --python 3.13` 后重新执行校验。`uv pip sync --require-hashes` 也可安装锁文件。

校验器先检查 OpenAPI 结构及全部 Schema，再以 JSON Schema Draft 2020-12 和显式 `FormatChecker` 检查 `examples.json` 的合法/非法示例；最后检查冻结操作集合、Cookie/CSRF/Bearer、Bot 能力、跨 Bot 引用权限、幂等键和包括公共错误引用的全部响应 no-store。示例包含精度超过 JavaScript 安全整数范围的序号、SQLite 上限、未知字段、删除确认、版本、凭据隔离、内容字段泄漏、UTF-8 字节边界、非法消息/callback、合法冲突错误及 Range 边界；全部是合成数据。失败退出非零。

这些是契约校验，不替代真实服务的鉴权、事务、CSRF、即时撤销与响应脱敏测试。后续实现不能用 Schema 通过代替运行时安全验证。

## 身份及权限

| 身份 | 允许范围 | 约束 |
| --- | --- | --- |
| Web Cookie 管理员 | 本卡所有管理接口 | 写操作同时验证会话 CSRF 与同源 Origin/Referer；登录无会话，验证 Origin；不浏览正文、附件、历史或聊天 WS |
| Android 管理员 Bearer | 所有管理接口和本批 client/附件接口 | 每安装设备 ID、每设备独立会话；登录才返回 Bearer，session 接口不再次返回 |
| Bot API Token / send | 本 Bot 消息创建/编辑、上传授权附件 | 不能访问管理员接口、其他 Bot 或作为 Webhook Key；不提供 MCP/业务执行 |
| Bot API Token / receive | 本 Bot claim、ACK、renew 和相应投递附件 | 不授予创建/编辑、上传或枚举历史附件 |
| Webhook Key | 仅本 Bot `POST /webhooks/{bot_id}` 创建标准消息 | 无读/编辑/上传/领取/管理权限；可引用本 Bot 已准备好且允许发送的附件；密钥只放 Authorization |

OpenAPI 的三个 Bearer scheme 用于区分身份，`security` 不能表达非 OAuth Bearer 的 send/receive scope，也不能仅凭 Bearer 格式识别用途。运行时须验证独立凭据类型、Bot ID、能力、启用状态和代次。Webhook 接口和 Bot 消息接口分别使用 `WebhookKey`/`BotAPIToken`，不能复用管理员 scheme。

Bot 停用覆盖 send/receive；写请求/Long Polling/ACK 提交前重新鉴权。重置 API Token 使旧 API 请求/租约失效并重排队，保留消息/失败记录；重置 Webhook Key 不影响 API 租约。管理员注销只撤销当前设备会话及推送映射，不改变 Bot 凭据或其他设备。两套凭据各至少 256 bit 随机，分别摘要存储，明文仅创建或对应重置响应返回一次；列表/PATCH 仅返回公开标识和代次。

## 通用格式与后续扩展

- `Sequence` 是非负 SQLite 64 位规范十进制字符串，拒绝数字、前导零、指数形式及溢出；`Version` 从 1 起，不等于任何序号。ID 使用 UUID v4，时间是 UTC RFC3339。
- JSON 对象拒绝未知字段。错误为 `{error:{code,message,request_id,retryable}}`；稳定 code 用于分支，message 仅显示。错误不含 SQL、路径、密钥、正文和原始远端响应。后续卡可增加稳定 code，不将全部错误固化为本卡有限枚举。
- 400/401/403/404/409/410/413/422/429/503 遵循技术设计；429/暂态 503 尽量带 Retry-After。受保护成功和错误响应均 no-store。
- 元数据分页采用不透明 `cursor`、正整数 `limit`，响应 `next_cursor`（末页 null）、`has_more`；服务实现须有界，具体资源控制上限留实现任务按技术设计处理。历史与同步须使用各自已设计的序号/高水位协议，不能把本列表当历史分页。
- `expected_version` 用于 Bot 修改/删除的并发比较；删除还要求显式 `confirm_name` 和 UI 二次确认。Bot 连接状态仅描述近期轮询/未知，不将入站 Webhook 宣称为持续在线。
- `relaynest_session`、`X-CSRF-Token` 和字段命名为本卡局部协议命名；Cookie 为 HttpOnly、SameSite=Lax，依据外部 HTTPS 设置 Secure。Web session 提供 CSRF bootstrap。
- 后续扩展直接更新本契约和示例，不复制第二套通用类型。已冻结字段/权限变化须回到确认流程；本次不规定新的产品限制或消息行为。

外部格式与校验工具依据：[OpenAPI 3.1.0 官方规范](https://spec.openapis.org/oas/v3.1.0.html)、[jsonschema 4.25.1 校验文档](https://python-jsonschema.readthedocs.io/en/v4.25.1/validate/)、[openapi-spec-validator 0.7.2 官方包](https://pypi.org/project/openapi-spec-validator/0.7.2/)。

## P03-05 冻结接口

下表均相对 `/api/v1`。Web Cookie 不可调用任何消息、callback、历史或附件操作；也没有 Android 消息编辑路由。

| 方法与路径 | 身份/能力 | 成功结果 |
| --- | --- | --- |
| `POST /bot/messages` | 启用 Bot API send | 201 `Message`，sender=application |
| `PATCH /bot/messages/{id}` | 启用 Bot API send，仅自身 application 消息 | 200 `Message`，完整内容替换和 CAS |
| `POST /webhooks/{bot_id}` | 路径 Bot 的独立 Webhook Key | 201 `Message`，与 Bot 创建复用 `CreateMessageRequest` |
| `POST /client/bots/{id}/messages` | Android Bearer，目标启用且 receive | 201 `{message,delivery_id}`，sender=administrator |
| `POST /client/bots/{id}/callbacks` | Android Bearer，目标启用且 receive | 201 独立交互 `{event_id,delivery_id,message_id,message_revision,button_id,callback_data,created_at}` |
| `GET /client/bots/{id}/messages` | Android Bearer | 200 `MessagePage`，从新到旧按 bot_seq 分页 |
| `GET /client/messages/{id}` | Android Bearer | 200 当前 `Message`；过期/无权 404 |
| `POST /attachments` | Android Bearer 或启用 Bot API send | 201 ready `Attachment` |
| `GET /attachments/{id}` | Android Bearer 或启用 Bot API 授权下载 | 200/206 二进制；条件匹配 304；无可用范围 416 |

标准载荷包含 `format: plain|markdown`，以及可选 `text`、`attachments:[{id,kind}]`、`reply_to_message_id`、二维 `buttons`。省略内容字段分别归一化为空字符串/空数组，引用归一化 null；至少有非空文本、附件或按钮。按钮由 `id/label` 及互斥的 `url` 或 `callback_data` 构成；URL 是普通绝对 URI，显式模式检查 scheme 及空白/控制字符；不新增协议白名单，Hub 不自动抓取或解释。当前锁定校验依赖未安装可选 URI format 检查器，该格式注解不能代替运行时 URI 校验及客户端安全呈现。请求不能伪造 sender/Bot/序号/版本。所有 ID 引用必须在同一 Bot 授权范围内，附件 ready、类型匹配且不重复；button id 在消息内唯一。Schema 能检查结构，实体归属、文件状态和 ID 唯一语义由运行时验证，不能因为一个 UUID 合法就视为已授权。

编辑为完整替换：`format/expected_revision` 必传，其他省略字段清空；同事务更新当前内容、revision、附件引用和 `message.updated`。CAS 冲突 409 `REVISION_CONFLICT`；bot_seq、created_at、未读和提醒不变化。普通回复只引用当时存在的本 Bot 消息；自然清理保留原引用 ID供 UI显示过期，不永久保留源正文。callback 请求只传 `message_id/message_revision/button_id`；服务从该版本取原始参数，持久化独立点击快照、管理员及设备身份，之后编辑不改快照。版本过时、按钮消失或已变成 URL 均 409 `BUTTON_STALE`。Hub 不根据相同 callback_data 合并新点击，不解析命令或执行 MCP。

消息创建、编辑、Webhook、callback 和附件上传要求 UUID v4 `Idempotency-Key`。先鉴权，再以身份+路由类别查键，不依 Token 版本分区；新有效凭据可恢复未知结果，撤销凭据不能恢复。规范化摘要包括路径目标 Bot/资源 ID、有效字段、附件/引用/按钮，编辑版本、callback 点击目标/版本和上传元数据/实际内容摘要，不含 Authorization。同键同请求返回原状态及原结果，先恢复已成功编辑/点击，再判断新操作 CAS/按钮版本；不同内容 409 `IDEMPOTENCY_CONFLICT`。消息及投递存活期间保留记录；清理后最小墓碑保留 7 天，命中 410 `RESOURCE_EXPIRED`，不重新创建。受保护投递记录不按此窗口删除。窗口后客户端不可自动重放未知旧发送。

`MessagePage` 延用前卡 `items/next_cursor/has_more`、`OpaqueCursor` 和正整数 limit；历史 cursor 绑定本 Bot 的 bot_seq 边界，不是管理列表游标，不使用 offset，不含同步 epoch。末页 cursor=null/has_more=false；服务有界处理，编辑不改变排序。管理员可继续读取停用 Bot 的历史。P03-05 未添加投递 claim/ACK/renew；这些由下节 P03-06 定义。增量同步、已读、WS 和推送留 P03-07。

## 大小及附件传输

- JSON 原始请求体最多 256 KiB；text 最多 64 KiB UTF-8，callback_data 最多 4 KiB UTF-8。字符串不截断或 trim；callback_data 允许空字符串。每消息最多 10 附件、10 行按钮，每行最多 5 个，label 最多 128 字符。
- `x-utf8-max-bytes` 是契约扩展，由校验器显式执行；标准 JSON Schema 的 maxLength 是字符数，不能代替字节限制。`examples.json` 的 `repeat` 仅为紧凑夹具，校验时先展开真实字符串，再执行相同 Schema。JSON 原始大小由 requestBody `x-max-bytes` 声明，运行时仍需流量限制，示例校验不声称已经测过 HTTP 大小拦截。
- 上传用 `application/octet-stream` 原始单文件流。query 必填 `bot_id/kind/filename/media_type`，Bot Token 的 bot_id 必须匹配自身；显示文件名/媒体类型不决定物理路径或放宽实际大小限制。图片默认 10 MiB、文件 50 MiB，允许全局配置（`x-default-max-bytes`/`x-configurable-max-bytes`），不以静态 binary Schema 限制可配置值。检查实际流字节，不能只信 Content-Length。
- staging、实际字节/SHA-256、fsync、同卷 rename 和 DB ready 成功后才返回成功；半文件不作为完成上传的幂等结果。未完成 staging 可安全重试，完成同键返回原附件。未引用文件 24 小时后回收，上传/下载活动引用保护避免误删；缺文件标 missing，已授权下载返回 404 `ATTACHMENT_MISSING` 并记录健康异常。
- 下载仅开放 ready。Bot send 仅下载本 Bot 授权发送消息文件；receive-only 仅下载领取事件所需附件，不能枚举历史或上传。管理员可下载停用 Bot 授权历史。先鉴权，再取得短租约/引用和打开文件，清理不能在活动下载中删除文件。响应包含 no-store、nosniff、安全 Content-Disposition、强 ETag（SHA-256 加双引号），不泄露存储路径。
- `Range: bytes=0-9`、`bytes=10-`、`bytes=-10` 及多区间有效；显式 start>end 或格式错误 400。可相交区间裁到资源末尾；全部不相交、suffix=0 或零字节文件返回 416 `RANGE_NOT_SATISFIABLE`，带 `Content-Range: bytes */size`。206 单区间带 Content-Range，多区间用 multipart/byteranges 并在每部分携带 Content-Range；Content-Length 是实际响应体长度。206 必须满足 start<=end<size，不能返回 */size。
- If-None-Match 命中先返回无正文 304；If-Range 匹配强 ETag/HTTP 日期才处理 Range，弱 ETag或不匹配则完整 200。Range 的格式/区间扩展与资源大小夹具由校验器检查；真实HTTP条件处理、文件打开、清理竞态尚未实现/实测。

错误继续复用统一 Error：400 格式、401 过期/撤销、403 能力不足、404 `RESOURCE_NOT_FOUND`（跨 Bot/无权/不存在不区分），409 `BOT_DISABLED/REVISION_CONFLICT/IDEMPOTENCY_CONFLICT/BUTTON_STALE`，410 `RESOURCE_EXPIRED`，413 `PAYLOAD_TOO_LARGE`，422 字段语义、429 限流及可重试 503 `STORAGE_UNAVAILABLE`。具体 operation 声明支持的状态；错误只含 code/message/request_id/retryable，不含正文、callback_data、SQL、路径或原始异常。契约权限声明检查和合法错误实例验证均不代表运行时鉴权、幂等事务或文件一致性已通过。


## P03-06 投递与失败管理

| 方法与路径 | 身份 | 成功响应 |
| --- | --- | --- |
| `POST /bot/events/claim` | 启用 Bot API receive | 200 `ClaimResponse`，含不可预测租约 Token 和不可变事件快照；超时空数组 |
| `POST /bot/deliveries/{id}/ack` | 同 Bot API receive、consumer 与当前租约 | 200 `AckReceipt`；同一成功租约重试恢复原 `acked_at` |
| `POST /bot/deliveries/{id}/renew` | 同 Bot API receive、consumer 与有效租约 | 200 `RenewReceipt`，更新服务端到期时间 |
| `GET /admin/deliveries` | Android Bearer 或 Web Cookie 管理员 | 200 `DeliveryMetadataPage`，`state=failed` 为失败队列 |
| `POST /admin/deliveries/{id}/retry` | Android 管理员，或 Web Cookie+CSRF+同源校验 | 200 `RetryDeliveryReceipt`，空 JSON 请求及必需 UUID v4 `Idempotency-Key` |

receive-only 可以领取、ACK、续约和下载所领取事件的必要附件，不允许上传、发送/编辑消息、枚举历史或进入管理路由。send-only、Webhook Key 和管理员凭据均不能替代 Bot receive 身份。实例 `consumer_id` 是长度 1～128 的标识，不是授权。服务须每次提交/长轮询唤醒后重新验证 Bot 启用、凭据类型、能力和代次；重置 API Token 撤销旧等待/租约并重新排队，重置 Webhook Key 不影响它们。

claim 的 `wait_seconds` 默认25，允许0～30；`limit` 默认20，本契约采用资源上限100。每 Bot 默认最多2个等待请求，超额429。短写事务按 `enqueue_seq` 领取到期 pending/retry_wait 并置 leased，等待发生在事务外；并发实例不可取得同一有效租约。200空列表只表示等待结束无数据；存储/查询失败返回503，不伪装空队列。领取有序不等于业务执行严格有序，领取响应丢失可能到期再投。claim 不使用幂等键。

`delivery_id/event_id` 跨自动重投和手动重试稳定；`retry_generation` 初始 `"0"`、采用既有 `Sequence`，手动重试加一。`attempt` 在领取响应为本代当前尝试编号，从1起；本代未领取的管理元数据为0。只有实际领取后的租约到期才消耗该次尝试；停用/凭据重置撤销租约不额外消耗，回 pending 保持已消耗次数，因此下一领取可重用未消耗的尝试编号。纯等待/无消费者不耗尽次数。默认每代8次，退避 `min(5s × 2^(attempt-1),30min)` 加±20%抖动；扫描默认5秒，数据库状态及服务端UTC为权威。到期转 retry_wait，耗尽转 failed；failed 无自动TTL，事件及必要父消息/附件免于普通历史清理。

普通输入的事件类型为 `message.created`，包含创建时的 `Message` 快照且 sender=administrator；callback 为独立 `button.callback`，包含点击时 message_id/revision、button_id、原样 callback_data、来源管理员/设备及时间。后续消息编辑不改投递快照，Hub 不解释参数、不执行 MCP。`event_id` 位于领取条目，与不可变 `event` 一起由应用持久化去重。

ACK请求为 `{consumer_id,lease_token}`。提交时按当前凭据代次、Bot状态及同consumer的有效租约校验；旧租约、过期、其他consumer返回409 `LEASE_CONFLICT`，跨Bot或无权资源404 `RESOURCE_NOT_FOUND`。同一成功ACK先验证当前身份和授权，再凭已保存成功租约摘要恢复200原结果；原到期时间已过不误报租约过期，其他租约不能利用acked状态确认。凭据撤销、Bot停用/删除仍优先；撤销凭据不能恢复结果。应用须先持久化完整事件和必要附件字节再ACK，之后在本地执行业务。ACK表示可靠接收，区别于业务成功、用户已读或恰好执行一次；ACK后Hub可以按普通期限清理。

初始租约默认120秒。renew另带 `extend_seconds`（1～120），在当前lease_until上延长；结果不得超过本次领取时刻+600秒，包含初始租约。超过总预算409 `LEASE_LIMIT_EXCEEDED`，不截断；已ACK、过期或错租约409 `LEASE_CONFLICT`。续约不更换租约Token，不增加领取次数，不承诺响应丢失的幂等恢复：同一请求重复可能再次延长，但始终受600秒上限约束。耗时业务应依赖持久Inbox。

管理投影只有ID、状态、版本、次数/代次、时间和脱敏稳定错误码；严格 `additionalProperties:false`，禁止正文、callback_data、附件信息/地址、租约Token、consumer和原始异常。`error_code` 必须由服务映射，不能装入远端错误串。`lease_until/acked_at/next_attempt_at`分别只在leased/acked/retry_wait时非空。筛选支持bot_id/state及既有不透明cursor、正整数limit；它不是历史/同步游标。

手动重试仅failed→pending：同事务代次加一、attempt重置0、清除错误/租约/退避，保留原投递/事件/载荷/幂等身份；非failed409 `DELIVERY_NOT_FAILED`。停用Bot重新排队后仍暂停，恢复启用才可领取。不同键并发只可一个转换成功。同键在当前有效管理员鉴权后先恢复原结果，再检查failed条件，不能重复加代次；摘要含路径目标及空对象，跨目标同键409 `IDEMPOTENCY_CONFLICT`。资源存活期间保留幂等记录，清理后7天墓碑410 `RESOURCE_EXPIRED`，受保护记录不按7天删除；版本/代次耗尽返回503且不回绕。重试传输不要求应用重跑业务，已有Inbox的应用可重新ACK。

`x-lease-policy/x-claim-policy/x-ack-replay/x-max-total-lease-seconds/x-error-codes/x-management-projection/x-idempotency-policy` 是由本地校验器检查的契约声明；权限示例只检查声明组合，错误示例只检查响应结构及相应状态可用性。所有这些均不证明真实运行时鉴权、数据库原子领取、ACK/重试事务、竞争和即时撤销已经实测。

## P03-07-S01-S01 增量与共享已读

本批只新增 `syncClientChanges`（GET `/client/sync`）和 `updateClientRead`（PUT `/client/bots/{id}/read`），均仅 Android 管理员 Bearer。Web Cookie、Bot Token、Webhook Key 均不授予同步正文或更新共享已读权限。受保护成功/错误响应沿用 no-store，JSON 拒绝未知字段；read 请求上限 262144 字节。

- 每轮追赶冻结 H，按 `(position,H]` 返回事件；`limit` 默认/上限 200。游标为服务端验证的不透明令牌，绑定管理员身份、epoch、H、position、轮次结束标记。续页沿用 H，末页推进至 H，下一请求开始新轮次。`next_cursor` 始终非空；空轮次返回 `has_more=false`。省略游标从当前 epoch 的 0 开始，仍检查 floor。
- 有效游标 epoch 不同或 position 小于 floor 返回 `410 SYNC_RESET_REQUIRED`；非法/篡改/跨身份游标返回 `400 INVALID_SYNC_CURSOR`。不得跳过历史缺口；客户端提示历史仅部分保留并重建服务端缓存。
- 事件及当前投影来自同一读事务。`message.upsert` 可携带更高 revision；已删除实体转换为无正文墓碑。六类为 `message.upsert`、`message.deleted`、`bot.upsert`、`bot.deleted`、`read.updated`、`delivery.upsert`。按 Message.revision、Bot.version、ReadState.version、DeliveryMetadata.state_version 应用，重复/旧版本幂等。Bot 删除和消息删除采用实体终态版本，不复活同一 ID。设置类型由原 S02/S03 显式扩展。
- `through_seq` 为实际展示过的本 Bot bot_seq（客户端责任），不能使用 change_seq；服务端验证不超过该 Bot 曾存在的最大序号，超出返回 `422 READ_SEQ_OUT_OF_RANGE`，无权/不存在返回 `404 RESOURCE_NOT_FOUND`。原子 max 合并；前进才同事务增加版本、更新服务端时间并写共享 read.updated。未前进保持版本/updated_at且不重复写变更。未读仅计现存 application 且 bot_seq>read_seq；编辑、自身、callback、过期不增加未读。
- 服务端时间：SyncPage.server_time、UpdateReadResponse.server_time 为响应时间；ReadState.updated_at 为已读前进时间；Message.created_at/updated_at、删除 deleted_at、Bot.connection.last_poll_at、DeliveryMetadata 的各时间保持各自语义。并发顺序只依序号/实体版本，不依客户端时间。

新 Schema：SyncEpoch、ReadState、UpdateReadRequest、UpdateReadResponse、SyncMessageUpsert、SyncMessageDelete、SyncBotUpsert、SyncBotDelete、SyncReadUpdate、SyncDeliveryUpsert、SyncChange、SyncPage。所有序号复用非负 SQLite64 字符串 Sequence，实体版本复用从 1 开始的整数 Version。

`x-sync-envelope` 执行批次序号/高水位/有界分页结构检查，`x-sync-policy`/`x-read-policy` 冻结状态规则；examples 的 `sync_state` 核对合成游标、分页、投影、旧版本、max 合并和未读声明。它们不是运行时事务、并发、客户端、通知撤销或真实 WS 测试。bootstrap/WS 为后续 S01-S02/S01-S03；设备通知/全局设置留原 S02/S03。本批不声称 G-API 全域完成。

## P03-07-S01-S02 bootstrap 与重建

`GET /client/bootstrap`（`bootstrapClient`）仅允许 Android 管理员 Bearer；所有成功/错误响应 no-store。默认 `phase=page`、limit默认/上限200。首次省略 token，在短读事务冻结 H、epoch、Bot 与共享已读快照；后续请求不保持 SQLite 长读事务。每页返回固定 `bots/read_states` 快照和当前 `messages:[{created_change_seq,message}]`；按 Bot UUID 文本升序、Bot 内不可变 bot_seq 升序扫描，只包含创建 change_seq≤H 的记录。`server_high_watermark` 始终指本次原始 H；`retention_floor` 是该响应复核的当前 floor。H 与 bot_seq/read_seq 不混用。

每页短读事务重新检查当前 revision、删除和附件文件现存性；删除消息不返回旧正文，附件消失仍保留消息引用，后续下载返回既有 `404 ATTACHMENT_MISSING`，不得伪造文件可用。快照之后的删除由 `(H,current]` tombstone 补齐。Bot、共享已读按实体 version 合并，消息按 revision 合并；高版本优先，删除是终态。快照不覆盖追赶阶段已取得的高版本。

响应 `kind=page`：epoch/H/floor、bots/read_states/messages、page_token/next_token/has_more、sync_cursor、issued_at/expires_at/server_time 全部必填。page_token 是扫描本页后位置的签名 receipt（绑定是否完成全部页）；next_token 是从该扫描位置续页的 resume 令牌，两者用途不能互换。末页 next_token=null/has_more=false；删除可导致空非末页。重复相同续页令牌会重新复核当前投影，不承诺响应字节相同。初次不带令牌重试会创建新快照，客户端选定一代继续。

令牌为服务端完整性验证的不透明值，绑定 administrator_id/epoch/high_watermark/position（bot_id,bot_seq）/snapshot_id/issued_at/expires_at/purpose/all_pages_complete。有效期严格为首次签发起900秒，续页和复核不续期。非法、篡改、跨身份、位置不符、用途或参数组合错误 → `400 INVALID_BOOTSTRAP_TOKEN`。`now>=expires_at`、epoch变化、`floor>H` → `410 BOOTSTRAP_RESET_REQUIRED`；floor=H仍有效。必须提示仅保留部分历史并重新启动重建，不跳过缺口，不记为发送失败。

所有分页完成后，从响应 `sync_cursor`（同身份同epoch、已完成轮次位置H的 bootstrap-seed）调用已有 `/client/sync` 追赶 `(H,current]`。seed保留完成位，使sync按既有规则开启新轮次、捕获调用时当前高水位；不能固定旧H而跳过分页期间新增事件。批次和游标原子保存，提示不会推进游标。再调用 `phase=verify&token=<末页page_token>&sync_cursor=<追赶完成游标>`；page阶段禁止sync_cursor，verify必须携带末页receipt和完成轮次且位置≥H的有效sync游标，limit仅page生效。先验证bootstrap令牌及epoch/floor，再验证sync游标；bootstrap专属 `cursor_provenance` 要求服务端验证受完整性保护的签发用途：原seed即使完整性、身份、epoch、完成位及位置H均有效也必须拒绝；仅接受实际sync返回的结果，零变化完成结果位置H合法。用途不是客户端可提交的source字段，也不要求游标等于verify时最新水位；不改变既有sync/read的公开字段或轮次规则。后者为原seed、非法、跨身份/epoch、未完成或位置不足 → `400 INVALID_SYNC_CURSOR`。成功返回 `kind=verified`、epoch、原H、当前floor、catchup_position、server_time。该复核不冻结后续写入；切换后仍从已保存sync游标继续增量同步，发现epoch/floor失效继续重建。

重建写入独立 generation，保留本地未确认 outbox 和上传任务。全部页、追赶、最终epoch/floor复核完成后，客户端在同一本地事务原子切换 generation，并移除不在服务器集合的旧缓存；失败或崩溃不能暴露半代数据，重放按版本幂等。issued_at/expires_at是本次重建会话的服务端签发/固定到期时间，server_time为响应时间；实体时间沿既有语义，不决定排序或版本胜负。

新增 BootstrapMessage/BootstrapPage/BootstrapVerified/BootstrapResponse 四个严格 Schema，复用原 Message/Bot/ReadState/Sequence/Version/SyncEpoch/OpaqueCursor。`x-bootstrap-policy` 与两个 envelope 扩展及合成状态例核对快照、期限、分页、恢复门槛；不代表真实 SQLite 事务、HTTP鉴权/令牌实现、附件竞态或 Android/Room generation 已实现或实测。WS、设备通知/全局设置分支分别留后续 S01-S03、原 S02/S03，不放行 G-API。

F01回归：verify合成声明的 `verified_cursor_purpose` 表示服务端已验证的签发用途，不是HTTP参数或客户端自报；`catchup` 将冻结sync游标/批次规则的结果接入verify，覆盖零变化、新增事件和多页未完成。这里只声明令牌用途隔离，真实签发与防篡改仍须运行时验证。

## P03-07-S01-S03 WS 与父同步域联合交接

`connectClientWS` 是 GET `/client/ws`，仅当前 Android 管理员设备 Bearer。握手前重新验证会话，禁止查询串凭据；原生不携带 Origin 可连接，携带 Origin 时必须通过服务端配置的 Origin 策略。Web Cookie、Bot API Token、Webhook Key 不能升级。请求使用标准 Upgrade/Connection、Sec-WebSocket-Version=13、Sec-WebSocket-Key；成功 HTTP 101 仅含升级头及 no-store，无 JSON 正文。400 INVALID_WS_HANDSHAKE、401 AUTHENTICATION_REQUIRED、403 FORBIDDEN、429、503 是升级前 HTTP 错误，仍用 Error/no-store；标准 Sec-WebSocket-Accept 必须由真实实现计算，形状例不证明握手算法。HTTP 101 与 JSON frame 通过独立验证通道检查。

六个新增 Schema：WSProtocolVersion、WSHello、WSChangesAvailable、WSSessionRevoked、WSUnknownOptional、WSFrame。WSHello 的 keys 为 type=hello、epoch、change_seq（最新已提交水位）、protocol_version={major,minor}；WSChangesAvailable 为 type=changes_available、change_seq；WSSessionRevoked 仅 type=session_revoked。WSUnknownOptional 为未知 type、optional=true 的无载荷 envelope，已知事件不能借 optional 绕过严格 Schema。全部事件不携带正文、凭据、业务命令或游标。WSFrame 用 oneOf 联合四类；控制 ping/pong 是 WebSocket 控制帧，不能作为 JSON 事件。协议当前 major=1，minor 是非负整数；未知可选事件仅在主版本兼容时忽略；结构合法的 major=2 hello 仍须明确失败并关闭，不得处理其事件。撤销停止同步、关闭连接并清理会话，不展示缓存正文。

25 秒 ping，75 秒无响应断开；慢端合并为最新 changes_available，有界缓存且不缓存正文。重连指数退避加抖动，实际等待不超过60秒。后台关闭连接，恢复前台立即通过同一单飞 SyncCoordinator REST同步。可先连接并缓存提示再从持久游标追赶，或先同步后立即复核 hello 高水位；同 epoch 内只要 hello/缓存提示高于 REST 完成位置就继续追赶，不同 epoch 交给 REST 重置检查。hello/提示不能直接推进持久游标，只有 REST批次与游标同事务提交。REST 410 后按 bootstrap重建/真正sync追赶/最终epoch和floor复核/原子generation切换；保留 outbox/上传。bootstrap-seed 的服务端受保护来源即使完成位为真也不能冒充实际 sync-result；本卡复用前卡判定，无更改 H/epoch/floor 或错误优先级。

### 父 S01 八项验收映射（契约范围）

| 父验收 | 已交付 Schema/例子/声明与联合检查 |
| --- | --- |
| 1 四路由和身份 | syncClientChanges、bootstrapClient、updateClientRead、connectClientWS；四操作严格 AdminBearer；各卡权限例及 ws_permission_*、ws_state_*握手；HTTP 101无content和 no-store检查 |
| 2 固定H/epoch/floor/分页 | SyncEpoch/SyncPage、x-sync-envelope/x-sync-policy；冻结 sync_state cursor/batch 正反例；ws_state_recovery_epoch/floor 复用其410判定 |
| 3 当前投影/类型/版本 | 六种 SyncChange 分支、现存Message/Bot/DeliveryMetadata；sync_state projection/apply；删除无正文，设置分支保留原S02/S03归属 |
| 4 bootstrap与恢复 | BootstrapMessage/Page/Verified/Response、两个bootstrap扩展、x-bootstrap-policy；冻结token/page/verify/generation/catchup例；ws_state_recovery_sync-result/bootstrap-seed 联合验证来源隔离 |
| 5 read max与未读 | UpdateReadRequest/Response、ReadState、x-read-policy；冻结read/unread状态、上界错误/旧值正反例及时间声明 |
| 6 WS事件/窗口/生命周期 | 六个WS Schema、x-ws-policy、独立HTTP响应例、ws_state event/window/transport及握手正反例；明确主版本失败、提示不推进游标 |
| 7 数字/版本/身份/恢复边界 | 所有前两卡例保留；本卡ws_bad_seq*/bad_version*/leak*/unknown*，主版本结构合法但处理失败例；全例校验和隔离声明破坏探针，不宣称运行时验证 |
| 8 完整keys/时间/no-store/锁 | 下表与前两节，所有原组件/例子/操作/锁逐值或逐字节非回归；统一HTTP响应no-store，frame与HTTP分开；实体时间不决定并发次序 |

### 四操作与全同步域 keys

| 位置/Schema | keys或约束 |
| --- | --- |
| GET /client/sync | query cursor、limit；SyncPage={epoch,changes,next_cursor,has_more,server_high_watermark,server_time} |
| PUT /client/bots/{id}/read | path id；UpdateReadRequest={through_seq}；UpdateReadResponse={read,server_time}；ReadState={bot_id,read_seq,version,updated_at} |
| GET /client/bootstrap | query phase、token、sync_cursor、limit；BootstrapResponse 联合 page/verified |
| BootstrapPage | kind、epoch、server_high_watermark、retention_floor、bots、read_states、messages、page_token、next_token、has_more、sync_cursor、issued_at、expires_at、server_time |
| BootstrapMessage / BootstrapVerified | {created_change_seq,message} / {kind,epoch,server_high_watermark,retention_floor,catchup_position,server_time} |
| SyncMessageUpsert / SyncMessageDelete | {type=message.upsert,change_seq,message} / {type=message.deleted,change_seq,id,bot_id,bot_seq,revision,deleted_at} |
| SyncBotUpsert / SyncBotDelete | {type=bot.upsert,change_seq,bot} / {type=bot.deleted,change_seq,id,version,deleted_at} |
| SyncReadUpdate / SyncDeliveryUpsert | {type=read.updated,change_seq,read} / {type=delivery.upsert,change_seq,delivery}；SyncChange oneOf六种 |
| GET /client/ws / WSFrame | 握手头与四种frame keys见本节；HTTP与frame无交叉正文 |
| 共用格式与错误 | Sequence为SQLite64非负规范十进制字符串；Version为>=1整数；SyncEpoch/OpaqueCursor不透明；Error={error:{code,message,request_id,retryable}}；400 INVALID_SYNC_CURSOR、400 INVALID_BOOTSTRAP_TOKEN、410 SYNC_RESET_REQUIRED、410 BOOTSTRAP_RESET_REQUIRED、404 RESOURCE_NOT_FOUND、422 READ_SEQ_OUT_OF_RANGE；WS握手错误见上文 |
| 服务端时间位置 | SyncPage/UpdateReadResponse/BootstrapPage/BootstrapVerified.server_time是响应时间；BootstrapPage.issued_at/expires_at为固定15分钟签发/期限；ReadState.updated_at为已读前进时间；两个删除分支deleted_at为删除时间；内嵌Message.created_at/updated_at、Bot.connection.last_poll_at、DeliveryMetadata各时间沿原语义；WS不新增时间，水位仅是提示 |

以上共22个同步域新增 Schema（前卡12+4，本卡6），复用 Message/Bot/DeliveryMetadata 与基础类型。原S02仍补设备注册/注销、FCM/通知配置及其所属设置资源/变更分支；原S03仍补全局配置资源及其变更分支，不能提前把任意 object 或未解析引用加进 SyncChange。父S01联合契约覆盖不等于完整 G-API，不改变P1/P2放行；手机/OPPO/NAS留正式验收。

`ws_state` 是合成状态声明。OpenAPI、Schema、权限、恢复状态和破坏探针均不能证明真实 HTTP/WS握手、Origin部署配置、Token签发/防篡改、心跳计时、断网退避、SQLite事务/并发、Android/Room 或设备运行行为；这些仍为 NOT_RUN。

## P03-07-S02-S01 设备会话契约

相对 `/api/v1` 新增两个管理接口，复用 `Session`，不定义注册或通知偏好资源：

| 路径 / operationId | 权限 | 响应 |
| --- | --- | --- |
| `GET /devices` / `listDeviceSessions` | Android 管理员 Bearer 或 Web Cookie | 200 `DeviceSessionPage`；400/401/403/429/503 |
| `DELETE /devices/{id}/session` / `revokeDeviceSession` | Android 管理员 Bearer 或 Web Cookie + CSRF + 同源 Origin/Referer | 204 无正文；400/401/403/404/429/503 |

`DeviceSessionPage` 是唯一新增 Schema，keys 为 `items: Session[]`、`next_cursor`、`has_more`。复用 `cursor`/`limit` 有界分页，只列当前管理员有效的 Web/Android 会话，过期或撤销会话不列出；分页不承诺跨页快照。列表拒绝 token、CSRF、密码摘要、推送凭据和其他未知字段。时间直接复用 `Session.idle_expires_at` / `absolute_expires_at`，均由服务端产生 UTC RFC3339，不能用客户端时间判定代次。全部成功/错误响应沿用 `Cache-Control: no-store`；错误复用 `Error`，401 `AUTHENTICATION_REQUIRED`、403 `FORBIDDEN`、未知或其他管理员会话统一404 `RESOURCE_NOT_FOUND`，不披露资源归属；400格式错误，429/503沿公共响应。

后卡只需读取 `Session.id`、`device_id`、`client_type`、上述两个到期时间与两操作的 `x-device-session-policy`：路径 `{id}` 是列表条目的 `Session.id`，`device_id` 仅为元数据，同一 device_id 的多个有效 Session 均可列出并独立撤销；`Session.id` 本身就是不可复用的会话身份/代次，每次重新登录必须生成新值，无需新增 generation 数字或修改旧 Session Schema。服务端事务仅按所给 Session.id 定位并在提交前复核归属及有效性，撤销该会话代次，使其所有 provider 映射与 WS 失效、阻止新发送；WS 继续已有 `session_revoked` / 停止同步、关闭及清理会话规则。旧回调、迟到外发结果在提交前必须比对会话身份及有效性，不得复活旧映射、解绑或覆盖重新登录的新会话。已经外发的通知无法追回，不把契约声明当作真实断连/外发撤回证据。

撤销自身 Web 会话时清 Cookie，撤销其他设备不清调用方 Cookie。`logout` 保持只撤销当前会话及其所有通道/WS，不影响其他设备或 Bot 凭据。同 device_id 的其他会话保持；未知、无权或已无有效会话的目标返回404，迟到或重试 DELETE 旧 S1 绝不回退选择新 S2；DELETE无请求体，不承诺跨重新登录的重试幂等，重试前可重读列表。后卡须按 T9.3 分别核验注册与偏好归属，不得假定 device_id 唯一对应 Session；注册接口必须绑定当前认证设备/会话，不接受客户端选择其他设备/会话；本卡没有预造注册字段或 provider Schema。

`examples.json` 的 `device_session_state` 按步骤核对权限、完整会话快照、同设备双会话、撤销/登录交错、迟到回调和重登隔离的合成状态声明；正反例和破坏探针不等于运行时撤销、真实 WS 断连、SDK、多 worker 或手机/NAS测试。父验收1及7/8会话部分由本段及本卡报告追溯，注册/通知及父卡完整收口仍由后两卡承担。

## P03-07-S02-S02 注册生命周期

以下均为契约，前缀 `/api/v1`；仅当前有效 Android `AdminBearer`，Web Cookie、Bot Token、Webhook Key 均不可调用。提交事务前重验当前 `Session.id`，同 device_id 多个 Session 相互独立，不使用 device_id 作为注册代次。客户端不接受 device_id/session_id 目标选择器，也不能以重新登录的凭据提交旧会话回调：客户端须保存回调来源 Session.id，在新会话登记前丢弃旧来源；这不是新增 HTTP 字段。

| 路径 / operationId | 请求 | 成功 |
| --- | --- | --- |
| `GET /devices/current/push` / `getCurrentPush` | 无请求体/选择器 | 200 `PushRegistrationState` |
| `PUT /devices/current/push` / `updateCurrentPush` | `PushRegistrationUpdate` | 200 同上 |
| `DELETE /devices/current/push/{provider}` / `deleteCurrentPush` | `PushRegistrationDelete` | 200 同上 |

新增 Schema：`PushProvider=fcm|opush`、`PushBarrierVersion`（0..SQLite64上限）、`PushRegistrationVersion`（1..上限）、`PushRegistrationUpdate`、`PushRegistrationDelete`、`PushRegistration`、`PushRegistrationState`。PUT keys 为 `provider/token/registration_version/expected_version`，DELETE body 仅 `expected_version`；未知字段拒绝，单次只更新指定 provider，不全量替换。token 为非空有界字符串且 writeOnly，不回显/不记录。恢复结果 keys 为 `session_id/registrations`；后者必须恰含 fcm、opush 各一条，仅 `provider/registration_version/registered/updated_at`，既含有效注册也含缺失/墓碑，不含 token。

每个 `(Session.id, provider)` 持久屏障独立：从未注册为 version=0、registered=false、updated_at=null，初次 PUT 的 expected_version=0，新 registration_version 必须大于0。当前**有效**注册同 version 同 token 为幂等重放，在 CAS 前判定并保留 updated_at（expected_version可为原请求旧值）；其余写入必须 expected_version 等于当前屏障且 registration_version 严格增加。同版本不同 token、旧版本、预期不匹配均409，墓碑不能当作同值重放。服务端在同一事务完成身份重验、比较、单通道修改和 UTC RFC3339 updated_at；客户端不提交时间。

DELETE 先匹配屏障：有效注册清 token 并将屏障加一、生成服务端时间，墓碑持久保留；匹配从未注册或已有墓碑则无副作用200，重复原删除请求的旧 expected_version 仍409。重建须GET取得删除后的屏障，再用更高 registration_version 与匹配 expected_version；版本不可归零、回绕或部分提交。有效注册已达9223372036854775807时，同值重放仍成功，删除返回409 `REGISTRATION_VERSION_EXHAUSTED`；无法再提高的写入以409拒绝，需恢复状态并提示会话重新认证，不能私自重建旧身份。

409 `REGISTRATION_CONFLICT` 或 `REGISTRATION_VERSION_EXHAUSTED` 沿用脱敏 `Error`，不附 token/厂商原错或内部任务字段。冲突或请求结果未知时先GET当前会话状态，再用当前 SDK 注册及新版本协调；不得用迟到旧意图覆盖。GET没有token，不能据此推断token内容。共有400/401 `AUTHENTICATION_REQUIRED`/403 `FORBIDDEN`/429/503，PUT和DELETE另有413/422；成功和全部错误均no-store。401后不能将旧回调重定向到新登录会话。

内部发送结果不是HTTP接口。最小条件解绑快照为 `Session.id + provider + registration_version`，发送前和结果提交时重验会话有效及精确当前注册；仅明确 UNREGISTERED，或已确认合法载荷的无效注册错误可以解绑。该解绑与客户端删除同样保留递增屏障；耗尽则不提交，记录归一化内部失败等待后续处置。旧版本结果、网络错误、未知结果、不合法载荷均保留注册，不删除新token、其他provider、其他会话或偏好。撤销/退出先使对应Session全部映射不可用，旧工作即使晚到也不能影响重新登录的新Session。完整用途/配置/提醒快照、通知模式与偏好CAS/同步union交下一卡；本卡不新增后台HTTP job、SDK或厂商payload。

`examples.json` 的 `push_registration_state` 是合成事务序列，每步比对全部会话、两通道、版本/时间和偏好的快照；正反序列验证CAS、删除屏障、旧失败、未知结果与撤销/重登隔离。它不执行HTTP、数据库、多worker、SDK、系统展示或真实注销；以上以及手机/NAS现场均NOT_RUN，不能据此放行G-OPPO。

## P03-07-S02-S03 设备通知设置与联合发送条件

新增 `GET/PUT /devices/current/notifications`（`getCurrentNotifications/updateCurrentNotifications`），仅当前有效 Android AdminBearer，Web Cookie（含CSRF）、Bot/Webhook凭据均无权。没有目标设备/会话参数；权限错误401/403，共有400/429/503，PUT另有404（Bot不存在/无权）、409、413、422，全部响应no-store。请求 `NotificationSettingsUpdate` 完整替换设备配置：`expected_version/enabled/bot_overrides[{bot_id,enabled}]/dnd{enabled,timezone,start,end}/preview/notify_channel/notify_provider`，Bot不在覆盖列表时默认启用，与设备总开关取AND；重复Bot或未知字段拒绝。`fcm/fcm`、`vendor/opush`、`none/null` 是仅有的模式/provider组合。

归属依据T6.3“通知偏好始终属于设备”和T9.3当前认证设备/会话：配置键为认证 `Session.device_id`，注册仍以不可复用 `Session.id + provider` 隔离。不能假定一个device_id只有一个Session。同设备多Session共享偏好，设置写入与发送检查当前有效Session自己的注册；另一Session缺注册不得借用其token。撤销只撤销目标Session，不删除设备偏好，不修改登录/单活规则。

不存在持久设置时，GET返回虚拟默认：`settings_version=0, updated_at=null, enabled=true, bot_overrides=[], dnd={enabled:false,timezone:UTC,start:22:00,end:07:00}, preview=false, notify_channel=fcm, notify_provider=fcm`；无设置变更事件。首次PUT expected_version=0。之后每个成功PUT（含同值）在同一事务重验身份/归属、Bot授权、注册/平台前置、CAS，服务端版本+1、UTC `updated_at`和 `device.notifications.updated` 事件一起提交。并发失配409 `SETTINGS_CONFLICT`，耗尽409 `SETTINGS_VERSION_EXHAUSTED`，均为 `NotificationSettingsConflict`：公共Error四字段外仅增加 `error.latest_settings_version/recovery_path`；恢复路径固定 `/devices/current/notifications`。先GET并协调当前意图，不能盲重放未知旧请求。上限9223372036854775807，耗尽不回绕、不部分提交时间/事件/注册。版本不是客户端时间。

GET和PUT的200是 `NotificationSettingsState{settings,server_time}`；settings含 `device_id/settings_version/updated_at` 及全部配置字段，无token/厂商凭据。`server_time`仅服务端当前时间；配置时间只由服务端产生。vendor仅显式选择且本会话opush注册有效、服务器已核对应平台前置时可保存；缺任一条件422 `PROVIDER_NOT_READY`，不静默回退。平台资格来自服务器配置/验证，不能由请求伪造。OPPO账号未准备只阻实际激活/实测；不编造SDK/REST/资格/签名/分类/错误码。注册成功不改配置；首次fcm即使无注册仍沿前台和周期补偿获取数据。

`NotificationDND`使用可解析IANA时区，HH:mm的[start,end)，start>end跨午夜，start=end拒绝。按事件当时和发送当时的本地墙钟过滤；DST重复时刻均按同一窗口，跳过的时刻无匹配。命中时丢弃该事件提醒资格，静默同步继续，结束不补响。`preview`只管FCM同步后本地自绘：保留系统通知权限/渠道、当前可见会话、设备/Bot开关、锁屏私有可见性、无正文缓存兜底、共享已读/删除取消和 `(deployment_id,message_id)` 去重；编辑不响铃、缓存重建不批量提醒。vendor永远固定title「RelayNest 有新消息」、content「打开 RelayNest 同步查看」，不按preview发送正文。

### 同步与恢复keys

`SyncChange` 有意新增严格分支 `SyncDeviceNotifications{type:device.notifications.updated,change_seq,settings}`，使用原 `SyncPage` envelope与 `server_time`。应用时先比较认证device_id，不同设备忽略投影但仍按既有分页完成规则推进游标；同设备仅较高settings_version替换，重复/旧版不倒退。GET恢复同样先校验当前认证设备及版本，不把旧设备响应应用到新设备。bootstrap/epoch/floor/generation重建沿原S01流程，另取当前通知配置后才做提醒决策；设置GET不能充当完成sync-result或bootstrap-seed。H/floor/epoch/种子与WS衔接规则、bootstrap形状均不变。其余全局历史/附件配置、健康及全局设置变更分支由原P03-07-S03补齐；没有任意object占位。

### 内部job快照与D1～D3

`NotificationJobSnapshot`仅用于内部契约/合成夹具，未新增HTTP队列或厂商payload。字段为 `session_id/device_id/purpose(sync|alert)/target_watermark/provider/registration_version/settings_version/reminder_key{deployment_id,message_id}/bot_id/bot_seq/expires_at/alert_eligible`。Session.id即不可复用会话代次。稳定键跨重试保留；sync必须FCM，vendor/none可继续FCM静默同步，不能派生旧提醒。创建时按事件资格过滤并固定原配置代次；DND命中过的事件不能因发送延迟/水位合并恢复资格。

发送前检查有效精确会话/设备、当前注册版本、配置版本、目的/模式、有效租约与同设备串行所有权、expiry；alert另查设备/Bot开关、时区DND、共享已读、目标仍存在及保留的提醒资格，vendor还查服务器平台条件。只有pending且与当前job完整快照相等时可开始发送，避免重复worker发送。发送后以整个相同快照、当前注册/配置/有效会话和活租约作CAS；旧worker/旧注册/旧配置/旧水位/旧稳定键结果均不能修改新状态。外发后新已读/DND/过期不能倒转已经发生的发送；只能同快照记结果，不重新获得提醒资格。同会话/注册/配置代次和相同原job才能合并；合并只取sync目标水位max，不改变原提醒的代次、稳定键、expiry或已丢弃资格。

切换只作用于未来新事件；先停止旧版本新发送，使旧job失效，并隔离已外发/在途/未知结果。超时不跨FCM补响；客户端只根据最新确认配置决定是否自绘，离线不能确认vendor生效时不凭旧fcm配置补响。收敛可漏提醒，数据仍补偿。accepted仅平台受理，绝不是展示/同步/已读ACK；未知/暂态/限流/配置错误保留注册，不能跨通道重响。内部明确无效注册仍遵循前卡同Session/provider/registration_version条件解绑与递增墓碑；耗尽不部分提交。

仅vendor接受平台离线积压恢复时DND/已读/删除/退出/切换后无法可靠撤回的限制，以及旧通知/在途残余。FCM继续自绘、去重、共享已读、preview、DND全套要求，不扩豁免。适配器仅收固定通用提示或无正文FCM hint，禁止正文、附件URL、callback、凭据/原厂错误串。这些是声明及状态夹具；HTTP、数据库、多worker、SDK、平台展示、真机DND、手机/NAS均NOT_RUN，不放行G-OPPO或完整G-API。

### 父S02八项联合证据

| 父验收 | 实际证据（均为契约/合成检查） |
| --- | --- |
| 1 会话列表/撤销 | 前卡 `listDeviceSessions/revokeDeviceSession`，GET /devices，DELETE /devices/{id}/session，按Session.id精确撤销；`Session/DeviceSessionPage`；S01-F01/A02已集成，保留687例基段中的会话/权限例。 |
| 2 多通道注册 | 前卡 `updateCurrentPush/getCurrentPush` PUT/GET /devices/current/push；`PushRegistrationUpdate/PushRegistrationState`与全部注册版本/恢复例保持。 |
| 3 条件删除/迟到结果 | 前卡 `deleteCurrentPush` DELETE /devices/current/push/{provider}，body `PushRegistrationDelete{expected_version}`；有效删除屏障递增、墓碑保留、重登隔离；本卡结果快照模型联合旧结果/未知结果/耗尽。 |
| 4 通知CAS/恢复 | 本卡两操作及9个Schema，`通知 配置CAS恢复跨设备同设备多Session撤销`、合法/非法模式、缺注册/平台、冲突响应、耗尽全状态例。 |
| 5 设置变更/版本时间 | typed SyncDeviceNotifications与当前GET恢复，跨设备忽略、同设备只取更高版本，耗尽拒绝全事务；原sync/bootstrap/WS全部例保留。 |
| 6 D1～D3 | 固定通用文案、显式vendor、首次FCM、仅未来事件、FCM无豁免及DND资格永久丢弃；本段政策与job状态例，外部展示NOT_RUN。 |
| 7 联合夹具/快照 | 全部旧注册/会话例保留，新配置全状态、before/result/merge、跨设备/同设备多Session、租约/串行、旧快照及敏感字段正反例；隔离破坏检查见本卡实施报告。 |
| 8 完整交接/非回归 | 本文枚举三卡5条路径7个操作（含恢复GET和条件DELETE）、权限/状态/Schema及同步扩展；锁/旧组件/765例比较见实施报告；原S03只接剩余全局分支。真实运行时与G-API未验收。 |
