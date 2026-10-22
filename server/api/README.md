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
