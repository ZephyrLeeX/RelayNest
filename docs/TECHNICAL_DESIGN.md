# RelayNest 详细技术设计

> 日期：2026-10-21。状态：开发输入草案；没有已实现接口或通过运行验证的兼容性结论。  
> 产品依据：`PRD.md` 和 `ARCHITECTURE.md` §1 的 C-01～C-05 用户确认；后者明确 RelayShelf 后置。  
> 本文的 `/api/v1`、表名、JSON 和参数均为 RelayNest 拟定契约，不是既有服务接口。外部接口只在 §10、§11 逐项引用源码。

## 1. 服务端结构与事务约束

单 Go 进程；`cmd/relaynest` 启动 HTTP、短周期调度和优雅退出，另提供管理员管理子命令。`internal/` 按下表分层，各模块通过显式接口组合，不建设微服务或插件执行引擎。

| 模块 | 职责 |
| --- | --- |
| `auth` / `devices` | 唯一管理员、Web/Android 会话、凭据摘要、撤销、设备推送注册 |
| `bots` | Bot 元数据、发送/接收能力、启停、两套独立凭据及版本 |
| `messages` | 创建、应用消息编辑、回复引用、内容校验和幂等 |
| `delivery` | 应用领取、租约、ACK、重试/失败/手动重试 |
| `sync` | 变更序列、共享已读、增量与重建协议 |
| `attachments` / `retention` | 流式上传、权限下载、文件恢复、保护判定与分批清理 |
| `realtime` / `push` | WS 提示、SQLite 中的推送待办、FCM 适配 |
| `admin` / `httpapi` | 管理投影、路由、中间件、错误映射、嵌入 Web |

数据库写连接限制为一个，读连接少量且有界；写任务使用短事务，不在事务内等待网络、Long Polling、文件上传或 MoviePilot。每个连接设置外键、busy timeout；数据库启用 WAL 和 `synchronous=FULL`。SQL 事务成功与文件 flush 错误都要显式检查。调度唤醒允许内存广播丢失，数据库扫描负责恢复。

消息创建事务必须同时完成：授权/启用检查 → 幂等检查 → 附件和引用检查 → 分配序号 → 插入消息或回调 → 创建必要应用投递 → 写变更和推送待办 → 提交。提交失败不返回成功。WS/FCM 在提交后进行，不参与提交成功条件。

## 2. 标识、消息和状态

### 2.1 标识体系

- `id`：服务端 UUID v4，不使用时间或自增整数充当访问授权。
- `bot_seq`：每 Bot 单调递增的内容序号，用于消息排序及共享已读；编辑不重新分配。
- `change_seq`：整个部署的 SQLite 单调自增变更序号，事务内产生；消息、编辑、已读、队列、配置和删除都可形成事件。
- `revision`：消息内容版本，从 1 递增。与 `change_seq`、投递次数、已读无关。
- `sync_epoch`：数据库初始化时生成；更换数据库时改变。同步游标含 epoch 和位置，不接受不同 epoch 的游标。
- 协议中的 64 位序号以十进制字符串序列化，避免 Web JavaScript 精度丢失；时间为 UTC RFC3339，期限按服务端 UTC 计算，设备时区只用于显示及免打扰。

### 2.2 消息结构

```json
{
  "id": "<uuid>",
  "bot_id": "<uuid>",
  "bot_seq": "31",
  "revision": 1,
  "sender": "application",
  "format": "markdown",
  "text": "下载完成",
  "attachments": [{"id": "<uuid>", "kind": "image"}],
  "reply_to_message_id": null,
  "buttons": [[{"id": "details", "label": "详情", "url": "https://example.org/item/1"}]],
  "created_at": "2026-10-21T01:00:00Z",
  "updated_at": "2026-10-21T01:00:00Z"
}
```

`sender` 取 `application|administrator`，由认证上下文确定，调用方不能伪造。正文为 plain/markdown，可与附件组合；文本、附件、按钮至少有有效内容之一。URL 和 callback 按钮互斥；`callback_data` 是有界 UTF-8 字符串，Hub 不 trim、不解释命令、不改变值。按钮、文本及批次数量限制见 §8。

回复只允许引用本 Bot 已存在的消息。源消息自然过期后显示“引用消息已过期”，不无限保留源正文。普通引用不形成永久清理保护；尚未完成的回调投递所需父消息和附件另行保护。

### 2.3 用户输入和交互事件

Android 发送普通消息生成可见消息及 `message.created` 应用事件。点击 callback 生成独立交互记录与 `button.callback` 投递，包含 `event_id`、`message_id`、`message_revision`、`button_id`、`callback_data`、管理员身份及来源设备 ID；不得以 UI 文本反推业务参数。

同一次点击因网络重试复用幂等键；用户再次点击是新事件，Hub 不据相同 callback_data 擅自合并。父消息版本过时或按钮已不存在时返回 `409 BUTTON_STALE`，客户端刷新；业务授权、过期和重复执行判断仍由接入应用负责。callback 只适用于具备接收能力的启用 Bot。

### 2.4 状态分离

| 层次 | 状态/字段 | 含义 |
| --- | --- | --- |
| Android 本地发送 | `pending / submitting / accepted / failed` | 是否得到 Hub 持久化回执，超时是未知，不是服务端一定失败 |
| 应用投递 | `pending / leased / retry_wait / acked / failed` | 是否被应用确认接收，不代表业务完成 |
| 共享已读 | 每 Bot `read_seq` | 管理员已查看的消息水位，跨设备取 max |
| 业务执行 | 应用回复或应用自己的状态 | Hub 不解释，不以传输状态冒充 |

应用发送给 Android 的通知在消息事务成功后具备历史清理资格，通知未读、FCM 未送达、某台设备未同步都不使它成为受保护应用投递。

### 2.5 消息编辑

仅拥有发送权限的 Bot API Token 可编辑该 Bot 的 `application` 消息；Webhook 不可编辑，Android 首版不可编辑已发送消息。请求包含 `expected_revision`，比较并交换，冲突返回 409，不覆盖其他更新。完整校验新附件与按钮后，同事务更新当前内容、revision、附件引用及 `message.updated` 事件。

编辑不生成新的 bot_seq，不增加未读数量，不重新响铃，不改变原创建时间。仅存当前正文，变更事件保存 ID/版本和必要元数据，不建立无限旧正文档案。客户端按 revision 取较新值。已经提交的 callback 保存点击时的参数快照，不因随后编辑被改写。

## 3. 拟定 API 契约

### 3.1 通用规则

JSON 请求使用 `application/json`，拒绝未知字段和超限请求。受保护响应 `Cache-Control: no-store`；标准错误如下：

```json
{"error":{"code":"BOT_DISABLED","message":"Bot 已停用","request_id":"<id>","retryable":false}}
```

400 格式，401 未认证/已撤销，403 越权，404 无权或不存在的资源，409 版本/幂等/租约冲突，410 同步游标过期，413 大小超限，422 字段语义，429 限流，503 存储/暂时不可用。客户端依稳定 code 分支，不匹配中文 message。429/暂态 503 尽量返回 `Retry-After`。错误不泄露 SQL、路径、密钥、正文或远端内部响应。

所有创建消息、callback、手动重试和附件上传请求携带 `Idempotency-Key`（客户端生成 UUID）；消息编辑同时要求 expected_revision。作用域为管理员或 Bot 身份 + 路由类别，不按 Token 版本划分，凭据重置后仍可用原键恢复未知结果。同键同请求返回原结果，同键不同内容返回 409。请求摘要采用规范化后的字段、附件 ID、引用和按钮；不含 Authorization。

消息及投递存在期间保留对应幂等记录；清理后仅保留最小结果墓碑 7 天，不保留正文。墓碑命中返回 `410 RESOURCE_EXPIRED`，不重新创建。墓碑到期后不承诺历史去重；客户端不得自动重放已超过这一窗口的未知旧发送，需重新确认发送意图。受保护投递的幂等记录不按 7 天删除。

### 3.2 路由与权限

全局设置规划输入（POLICY01，须独立审计后由 S01 冻结 API）：`retention_days`、`image_max_bytes`、`file_max_bytes` 均为 1～9223372036854775807 的规范十进制字符串，规则同 §5.4；默认分别为 `"30"`、`"10485760"`、`"52428800"`。不修改公共 `Sequence`、现有 `Attachment.size` 或上传契约，附件技术边界不是 T8 默认值构成的业务上限。

全局资源的 `version`/`expected_version` 沿公共 `Version` 的 JSON integer、从 1 开始，并仅在新全局设置专用 schema 中交集约束 maximum=9007199254740991（2^53−1）；不修改公共 `Version` 或设备版本。CAS 在同一事务核对、更新字段、递增版本、记录服务端 `updated_at`（UTC RFC3339 Z）与 `settings.updated` 变更；成功写入（含同值写入）增 1。GET 返回完整设置/版本/时间；初始资源在持久化时以版本 1 和服务端时间创建。

规划错误：未知字段/类型或结构不符为 400；字符串语法、0、超 int64、expected_version 非整数/越界等字段语义为 422（结构类型错误优先 400）。过期 CAS 返回 409 `GLOBAL_SETTINGS_CONFLICT`，携带最新版本和 `/admin/settings` 重读路径；版本已达上限且 expected_version 匹配时返回 409 `GLOBAL_SETTINGS_VERSION_EXHAUSTED`、`retryable=false`，同样返回最新版本及重读路径。先验证输入、再核对 CAS、最后判断耗尽；耗尽不得写入/发变更、回绕、归零或自动换代。GET 继续可用，写入恢复需后续经确认的契约迁移。请求体超限沿既有 413；保留既有认证、Cookie 写 CSRF/同源规则。S01 必须为各分支建立专用错误 schema/正反例，不冒用设备错误。

以下路由统一位于 `/api/v1`；管理员 Bearer 与 Web Cookie 对应同一唯一管理员，Bot Token 不得用于管理员路由。

| 方法与路径 | 身份 | 契约 |
| --- | --- | --- |
| `POST /auth/login` | 无会话 | 用户名/密码、client_type、device_name；Web 返回安全 Cookie，Android 返回设备会话 Bearer；绝不记录密码 |
| `GET /auth/session`、`POST /auth/logout` | 管理员 | 当前会话、CSRF bootstrap；注销仅当前会话 |
| `GET /devices`、`DELETE /devices/{id}/session` | 管理员 | 查看设备/撤销指定会话；不返回 Token |
| `PUT /devices/current/push` | Android 会话 | 更新 FCM 注册、注册版本、时间；只更新本设备 |
| `PUT /devices/current/notifications` | Android 会话 | 全局、Bot、免打扰时区与时段、预览；版本比较，退出注销推送映射 |
| `GET/POST /admin/bots`、`PATCH/DELETE /admin/bots/{id}` | 管理员 | 创建/列出/修改/永久删除；删除要求显式确认 Bot 名称与当前版本 |
| `POST /admin/bots/{id}/credentials/{api-token或webhook-key}/reset` | 管理员 | 仅重置对应凭据并一次返回新值；旧值立即失效 |
| `GET/PUT /admin/settings` | 管理员 | 保留期限、附件大小等配置；expected_version 避免覆盖 |
| `GET /admin/deliveries` | 管理员 | 状态/错误码/时间/次数/ID，无正文、callback_data、附件地址及原始异常 |
| `POST /admin/deliveries/{id}/retry` | 管理员 | 仅 failed → pending，新重试代次；同键重试不重复增加代次 |
| `POST /client/bots/{id}/messages` | 管理员 | 只向启用且可接收 Bot 发送，返回 201 消息及 delivery_id |
| `POST /client/bots/{id}/callbacks` | 管理员 | 提交按钮事件和消息版本，返回持久化交互/投递 ID |
| `GET /client/bots/{id}/messages`、`GET /client/messages/{id}` | 管理员 | 有界历史分页/详情，按 bot_seq，不使用 offset |
| `PUT /client/bots/{id}/read` | 管理员 | `through_seq`，服务端验证范围并取 max，幂等 |
| `GET /client/sync`、`GET /client/bootstrap` | 管理员 | 见 §6，正文仅由客户端历史/同步接口返回 |
| `GET /client/ws` | Android Bearer | 提交后变更提示；不接收业务写操作 |
| `POST /bot/messages` | Bot send | 创建本 Bot 应用消息 |
| `PATCH /bot/messages/{id}` | Bot send | 编辑自身应用消息及版本 |
| `POST /bot/events/claim` | Bot receive | 有界 Long Polling 领取 |
| `POST /bot/deliveries/{id}/ack`、`/renew` | Bot receive | 租约 ACK/续约，见 §4 |
| `POST /webhooks/{bot_id}` | Webhook Bearer | 仅创建标准消息载荷；不提供字段转换、模板或脚本 |
| `POST /attachments`、`GET /attachments/{id}` | 管理员或 Bot | 流式上传及权限下载；Bot 只能处理本 Bot 范围，receive-only 不得上传新应用消息附件 |

Webhook 密钥放 Authorization，不放 URL 查询或路径；Bot API Token 不能替代 Webhook 密钥。Webhook 可引用该 Bot 已准备好且允许发送的附件，但本入口不提供凭据之外的读/上传能力；仅有 Webhook 密钥的脚本可以发送文本、按钮和普通外部链接，附件上传需另用 Bot send Token。按钮中 callback_data 的存在不意味着 Hub 授予业务权限。

Web 静态页面只调用管理路由。Android 可调用管理路由，并通过 client 路由显示消息。不能将 Web 的队列列表复用成消息历史浏览。

### 3.3 WebSocket

连接使用 `Authorization: Bearer`，不把会话写入查询串；浏览器 Web 管理不建立聊天 WS。服务器验证当前设备会话和 Origin 策略后接受升级，发送 `hello`（epoch、最新 change_seq、协议版本），再发送 `changes_available`（最新序号）或 `session_revoked`。客户端忽略未知可选事件，协议主版本不兼容时明确失败。

客户端先建立 WS、缓存提示，再从已持久游标追赶；或先同步后立即再校验 hello 高水位，避免连接窗口丢变更。提示只触发 REST 同步，不直接推进游标。25 秒 ping、75 秒无响应断开；慢客户端提示合并，不无限缓存正文。断线指数退避加抖动，上限 60 秒；进入后台关闭连接，恢复前台立即同步。

### 3.3 实例身份探测与 Android 分支（P08-04-D01-C01）

本节落实已批准 D01 提案，保证级别为 G1：防止诚实部署替换或误连后继续使用旧会话。公开随机 ID 不是秘密、签名或服务端身份证明；同 origin 的主动伪造不在保证内。探测成功后瞬间从 A 切至 B，旧 Bearer 或显式输入密码的字节仍可能抵达 B；取消不能撤回已发字节，服务端先检查头也不代表代理没有收到 Authorization。用户已接受此窗口，不得宣称 G2 或绝不泄露凭据。

`GET /api/v1/instance` 独立于 health/auth，公开且无凭据，无 query/body。拒绝任何大小写形式的 Authorization、Cookie、X-CSRF-Token、X-RelayNest-Instance-ID（包括空值），400 INVALID_REQUEST；不得解析或记录其值。账号、安装 ID、Session.id 不得随 query/body 发送。只允许 GET；不提供重定向或其他路由兜底。

200 为 UTF-8 `application/json` 单对象 `{"instance_id":"<UUIDv4>"}`，仅此 required 字段。ID 为标准小写 36 字符 UUIDv4，version=4、variant=8/9/a/b。所有成功/错误响应固定 `Cache-Control: no-store`、`X-Content-Type-Options: nosniff`；不得带 ETag/Last-Modified/Location，不返回 304。部署反代须禁此路径缓存。错误沿用封闭 Error 外壳，request_id 非空；固定 code/message/retryable：400 `INVALID_REQUEST` / `请求无效` / false，429 `RATE_LIMITED` / `请求过于频繁` / true，503 `INSTANCE_UNAVAILABLE` / `实例暂不可用` / true；429 固定 `Retry-After: 1`。错误不含实际 ID、路径或 DB 错误。请求非法优先于限流，限流优先于身份读取；未启动可以连接失败，无须保证 503。

客户端独立构造无凭据请求，禁止带默认认证的客户端/拦截器。仅既有 HTTPS 和标准证书/主机名验证；禁 trust-all、HTTP 降级、所有 3xx 自动跟随（含同源），404/405 不降级到 health/session。禁 HTTP/磁盘缓存与缓存命中；已绑定 ID 仅供比较。完整 call 上限 5 秒、无自动重试；解压后的成功或错误 body 上限 1024 字节，超限立即关闭且不落日志，不信任 Content-Length；严格 UTF-8、单 JSON、无重复键/额外字段，Content-Type 仅 application/json 或其 UTF-8 charset 形式。不增加压缩依赖。共享进程级常量内存令牌桶 20 次/秒、burst 40，多客户端共享可用性成本，不承诺吞吐。日志仅 request_id/固定路由/状态/耗时/错误码，不记录头、body 或实例 ID。

Android login（LoginRequest.client_type=android）及一切 AdminBearer 认证分支必须带 `X-RelayNest-Instance-ID`；包括 auth/session/logout、设备列表/撤销/通知/推送、管理状态/设置/Bot/凭据/投递、client 消息/历史/详情/callback/同步/已读/bootstrap、附件上传下载、WS 握手。完整操作矩阵固定在 OpenAPI 每个 `x-instance-binding` 和 validator 的 INSTANCE_BOUND_OPERATIONS；混合附件端点仅 Android 分支适用。Web Cookie 分支仍执行原 Origin/CSRF，Bot API Token/Webhook Key 分支沿用原权限与作用域；实例头不赋予任何认证能力，这些分支忽略实例头，不能用 client_type 或头把 Android Bearer 降级为 Web/Bot。

除下述两条附件例外外，顺序固定：先传输格式/客户端判别，Android 分支缺头 400 INSTANCE_REQUIRED、畸形（含多值/空/非严格 UUID）400 INVALID_REQUEST；身份读取失败/缺失/腐败 503 INSTANCE_UNAVAILABLE，期望不符 409 INSTANCE_MISMATCH；通过后才读取密码/Bearer并执行原认证、Origin/CSRF。实例错误不回显实际 ID、不跳转；400/409 不可重试，503 可重试且不当成 401 清正常会话。Web login 的原 Origin 规则保留，Android login 也不得绕过既有客户端/Origin 策略。认证写操作（含登录创建、续期、注销/撤销）在同一 DB 写事务提交前再校验 ID 和 auth_generation；只读操作在同一一致事务快照里读取 ID/会话/代次与业务投影。二次 ID 不符 409，代次失效沿用 401，DB 失败 503；均不得发布/提交部分认证结果。启动缓存仅可提前拒绝，不能代替事务核对；WS 握手在发布升级前校验，存活连接不跨部署搬迁。

仅 `uploadAttachment` / `downloadAttachment` 采用已批准的 Bot-first 狭窄例外：先检查传输/凭据通道格式（多 Authorization、Cookie/Bearer 混用、非 Bearer/空值按原规则拒绝），允许 ID 前读取 Bearer，仅用于 Bot API 凭据匹配，不认证/续期管理员会话，不读上传流、Range/条件请求或附件资源。分支识别不能仅依未冻结的私有 token 形状。最小独立匹配适配按现有唯一 credential_id 且 kind=api_token 查询、常量时间核对摘要并取得 Bot 身份/代次；下载不得先查附件取得 botID。现有 `bots.Repository.Authenticate(ctx, botID, kind, secret)` 需要 botID，不能直接充当此身份发现接口；本契约不新增 token、表/索引或公开接口。

匹配适配必须区分 NoMatch（确定无 API 凭据或摘要不符）、Matched（摘要匹配即固定 Bot 分支，包含随后拒绝）、Unavailable（依赖/存储错误或超时）。匹配后停用/能力不足沿用403、跨 Bot/资源域不授权沿用404、提交前撤销/代次失效沿用401；均绝不回退 Admin，不能把后续 ErrUnauthorized 降级为 NoMatch。故障返回503 INSTANCE_UNAVAILABLE、固定“实例暂不可用”、retryable=true，不回显底层错误，绝不回退。上传仍须 send 且 bot_id 与已认证身份一致；receive-only 禁上传；下载 send 仅本 Bot 获准发送附件，receive 仅已领取事件必需附件，不能只凭任一能力授权任意附件。原事务/幂等与文件打开顺序保持。

仅确定 NoMatch 才进入 Android：先检查实例头再认证 Admin，保留同一快照/提交前 ID 与代次复核。故接受部分无效/旧 Bot 凭据由401变为缺头400 INSTANCE_REQUIRED、畸形头400 INVALID_REQUEST、错 ID409 INSTANCE_MISMATCH；匹配存储故障即使缺头/错 ID仍先503。有效 Bot 忽略缺失/畸形/错实例头；实例头不选认证分支、不授予权限。全部响应 no-store；日志只含固定路由/request_id/状态/耗时/错误码，禁 token、credential_id、实例 ID、附件路径、头与正文。其余端点继续先 ID后读取 Android密码/Bearer。G1 已接受的 probe后瞬切凭据字节窗口也适用于此例外，不声称 G2。

公共 `Session` 及 DeviceSessionPage 仍为原字段封闭投影，不新增 instance_id；专属 `AndroidSession` 复制公共元数据字段并固定 client_type=android，新增 required instance_id，独立 additionalProperties=false，禁止 allOf 扩展旧封闭 Session。login 与 GET session 的 Android 响应均使用此对象，返回处理请求的数据库 ID且等于期望。只有 AndroidLoginResponse 含 access_token/token_type=Bearer；GET session 不回传 Bearer。WebSession/响应和 Bot DTO 保持既有语义。

## 4. Long Polling、ACK 与失败恢复

### 4.1 领取与负载

`POST /bot/events/claim` 输入 `{consumer_id, wait_seconds:25, limit:20}`；consumer_id 是实例标识，不是授权。响应 `{deliveries:[{delivery_id,event_id,attempt,lease_token,lease_until,event:{...}}]}`。没有数据等待至超时后返回 200 空数组，查询失败不得伪装成空队列。

领取在短写事务内选择到期 pending/retry_wait，按 enqueue_seq 排序并条件更新为 leased，返回不可预测租约 Token。网络等待在事务外；请求被唤醒后重新校验 Bot 启用/凭据代次。多个实例竞争不得领取同一有效租约。返回丢失会造成租约超时再投，符合至少一次投递。

默认批次按序领取但不保证业务执行严格有序；插件针对同一 Bot 使用单工作队列，其他接入应用自行控制并发。不得用全局阻塞一条失败消息的方式阻塞整个 Bot。

### 4.2 状态转换

```text
pending ──领取──> leased ──有效 ACK──> acked
                     ├─有效续约──> leased
                     └─租约到期──> retry_wait ──到期领取──> leased
                                          └─次数耗尽──> failed
failed ──管理员手动重试──> pending（retry_generation + 1）
任意未完成状态 ──永久删除 Bot──> 随 Bot 删除
```

- ACK 必须带当前 lease_token，且事务检查 Token 代次、Bot 状态及租约有效期。旧租约或其他 consumer 的 ACK 返回 409；不能确认后来实例的领取。
- 同一成功 ACK 的重试幂等返回 200，记录已完成租约摘要；Bot 删除或凭据撤销仍优先鉴权。ACK 回复丢失不造成重新执行业务。
- 消息传输 ACK 时机是应用已经可靠接收，插件应先将 event_id、完整载荷及后续执行必需的附件字节持久化到自己的 inbox/文件目录，再 ACK。ACK 后 Hub 可按普通期限清理，应用不可依赖未下载的旧附件继续存在。不要先 ACK 后只保存在内存；也不必等耗时业务完成才 ACK。
- 续约以当前租约为条件，延长不超过单次 120 秒，累计最长 10 分钟；超过后需依赖持久 inbox，不能无限占有 Hub 租约。
- 租约超时才消耗一次已实际领取的尝试；Bot 无消费者、纯等待或停用不自动耗尽重试。
- failed 的手动重试保留原 event_id、载荷和幂等身份，仅产生新的投递代次；应用已有 inbox 时重新 ACK，是否再次执行业务由应用判断，不把“重试投递”当作“重跑业务”。

### 4.3 暂停、删除和重置

停用事务禁止新收发，撤销活跃领取资格；leased 回 pending，保持事件 ID、已消耗次数，停用期间不推进重试计时。暂停前已合法返回的数据无法追回；恢复可能再次投递。ACK/续约在停用期间拒绝，应用恢复后通过稳定 event_id 去重。

重置 Bot API Token 增加凭据代次、取消旧 Long Polling、撤销旧租约并重新排队，保留消息/失败记录；旧凭据不能再 ACK，新凭据重新领取后 ACK。仅重置 Webhook 密钥不影响 API 租约。停用不是清空失败队列。永久删除按 PRD 二次确认，原子删除凭据、消息、事件及队列并写管理/同步删除标记，异步回收无引用文件。

### 4.4 默认参数

| 参数 | 初始值 | 依据 |
| --- | --- | --- |
| Long Polling wait | 25 秒，上限 30 秒 | 减少空轮询并适配代理 |
| 默认租约 | 120 秒 | 接收方持久 inbox 和 ACK 的预算，非业务执行时限 |
| 自动尝试 | 每代最多 8 次实际领取 | 失败保留，不无限后台重发 |
| 重试退避 | `min(5s × 2^(attempt-1), 30min)`，±20% 抖动 | 减少故障时请求峰值 |
| 超时扫描 | 5 秒 | 定时器不作为权威状态，重启后扫描数据库 |
| Claim 并发 | 每 Bot 默认 2 个等待请求 | 单用户资源限制 |

这些属于可调整实现参数，不表示锁屏通知延迟承诺。认证/格式错误不自动重试；网络/429/503 按退避。应用业务失败不用 Hub NACK 表达，应在应用侧处理并发送结果。

## 5. SQLite 数据模型与附件

### 5.1 表与约束

下表是实现所需逻辑模型，DDL 与 OpenAPI 在工程阶段从本契约生成并接受审查。

| 表 | 主要字段与约束 |
| --- | --- |
| `schema_migrations` / `system_state` | 版本、校验和、epoch、变更清理 floor、配置版本 |
| `administrator` | 固定唯一记录、username UNIQUE、password_hash、auth_generation |
| `devices` / `sessions` | device_id、名称、设备类型；session 摘要、过期、撤销、管理员代次；Web 与 Android 独立 |
| `push_registrations` / `device_notification_settings` | device/session、FCM 注册版本、更新时间；设备全局/Bot 设置与版本 |
| `bots` / `bot_credentials` | bot_seq 计数、enabled、send/receive、元数据版本；两种 credential 摘要/代次 |
| `messages` | UUID PK、bot_id、bot_seq、created_change_seq、sender、revision、正文格式、按钮 JSON、引用、created/updated；UNIQUE(bot_id,bot_seq) |
| `interaction_events` | event_id、bot_id、父消息/点击版本、callback 参数快照、来源设备、created_at |
| `deliveries` | delivery_id、event_id UNIQUE、不可变载荷快照、state_version、状态、attempt、generation、consumer、租约摘要/时间、next_attempt、acked_at、错误码 |
| `attachments` | UUID、bot_id、存储键 UNIQUE、文件名、MIME、size、sha256、staging/ready/deleting/missing、创建时间 |
| `message_attachments` / `delivery_attachment_refs` | 复合唯一键，显式外键；后者保护不可变投递所需文件 |
| `bot_read_states` | bot_id PK、read_seq、state_version；事务内 max 更新 |
| `changes` | change_seq INTEGER PRIMARY KEY AUTOINCREMENT、类型、实体 ID、版本、bot_id、时间；不保存旧正文 |
| `idempotency_keys` | 身份/操作/key UNIQUE、请求摘要、资源/响应元数据、清理时间 |
| `push_jobs` | device/session、目标序号、状态、尝试、下次执行；允许按设备合并 |

`messages.reply_to_message_id` 外键可 SET NULL，另存不可变原始引用 ID 用于 UI；不以级联引用阻止自然清理。`created_change_seq` 与创建事件在同一事务设置，bootstrap 以此限定创建高水位。deliveries 的必要载荷是不可变快照，不在重试时从最新消息重新拼装。callback 必要父消息在该投递未完成期间保持；依赖附件通过显式关系保护。ACK 事务释放该投递的附件/父消息保护引用，failed 不释放；仍被普通消息引用的文件继续保留。删除 Bot 的同步 tombstone 不使用会被 Bot 外键级联删除的行。

关键索引：messages(bot_id,bot_seq)；deliveries(bot_id,state,next_attempt_at)、(state,lease_until)；changes(change_seq)；messages(created_at)；attachments(state,created_at)；sessions(token_hash)、(device_id,revoked_at)。不在大正文/密钥上建日志型索引。


实例身份存储接口冻结于 `server/internal/storage`，后续 S01 实现，不在本卡创建 SQL/代码：`InitInstanceIDTx(ctx context.Context, tx *sql.Tx) (string, error)`、`ReadInstanceIDTx(ctx context.Context, tx *sql.Tx) (string, error)`、`RotateInstanceIDTx(ctx context.Context, tx *sql.Tx, expectedID string) (string, error)`。三者不自行开启/提交事务，不发布未提交 ID。Init 仅供编号迁移首次初始化：不存在时用系统安全随机源创建，已存在则严格读取并返回原 ID；已有迁移完成标记但行缺失/畸形不能重建。Read 严格验证唯一行/格式，缺失、重复、腐败统一 `ErrInstanceUnavailable`；读失败原始错误只能内部包装，HTTP 映射 503。Rotate 严格核对 expectedID，错配 `ErrInstanceMismatch`，随机失败或更新失败整体回滚；返回候选新 ID 只能在外层事务提交成功后使用。`ErrInstanceUnavailable`/`ErrInstanceMismatch` 为包级可 errors.Is 判别的 sentinel；随机/SQL/锁/提交错误不得吞掉。数据库约束保证单例；随机 UUIDv4 不由域名/管理员/安装 UUID/机器路径派生，不复用 sync_epoch。

正常启动/重启读取已提交 ID；首次建库及旧库一次性迁移在独占迁移事务内初始化，两个进程必须读同一提交值，随机/提交失败不就绪。迁移完成后的 ID 缺失/腐败停服，不能自动修复。在线替换 DB 文件不支持；运行中读错或身份变化停止新认证/就绪，不能继续用缓存 ID。

### 5.2 迁移

启动持独占迁移锁，按编号执行嵌入 SQL，事务内更新版本与 checksum；版本不兼容或 checksum 改动即拒绝启动，不能自动重建数据库。首次创建与升级分别测试。大数据迁移拆小批，版本兼容说明随发布提供。禁止自动生成 SQLite 备份；任何需要不可逆数据转换的升级必须先说明影响，不提供虚假的一键降级保证。

### 5.3 文件一致性

1. 鉴权及大小校验后建立 staging 元数据，使用随机服务端文件名；原始文件名只作显示，不拼接物理路径。
2. 在附件卷临时目录流式写入、限制实际字节数、计算 SHA-256，flush 并 fsync；不把完整文件读入 Go 内存。
3. 同卷 rename 到最终路径并 fsync 目录，再在短事务标 ready/size/hash。只有 ready 文件可绑定消息。
4. 文件成功而 DB 更新失败：保留 staging，恢复扫描确认文件、大小/哈希后完成状态或回收，不对客户端宣称上传成功。
5. 消息关联失败或用户放弃上传：24 小时后扫描未引用 ready/staging 文件。上传中以活动租约防止扫描误删。
6. 清理时事务确认无消息、投递、活动下载/上传引用并标 deleting；随后 unlink；再删除元数据。失败可重试。删除状态不得被重新关联。
7. 启动恢复 deleting、过期 staging、无元数据孤儿文件；扫描限定目录、拒绝符号链接和越界路径。ready 但缺文件标 missing，下载返回明确错误，记录健康异常；不伪造恢复内容。

下载先获得引用/短租约再打开文件，清理与下载串行判定；支持 Range、ETag 和断点下载，下载只开放 ready。响应设置 `nosniff`、安全 Content-Disposition，正文不暴露存储路径。Bot 只可下载本 Bot 其授权发送消息/领取事件所需文件，receive-only 不能枚举历史附件；管理员可下载仍存在的授权附件。停用 Bot 不允许 Bot API 下载，管理员历史仍可查看。

Bot Hub 首版上传为单文件流式请求，失败可以重试并保持幂等键；PRD 对 RelayShelf 的分片/续传不能错误扩展为 Hub 必须具有分片协议。完成上传的幂等结果可返回原附件；未完成 staging 可安全重试，不用半文件作为完成结果。

### 5.4 历史清理

- 以消息 `created_at` 计算全局保留期限，默认 30 天；编辑/已读不重置时间。已 ACK 的旧消息在下一轮满足条件时可清理，不额外延长 30 天。
- 保留天数为正 int64 规范十进制字符串（1～9223372036854775807），禁止 0、前导零、符号、小数、指数和 JSON 数字；这是表示/存储技术边界，不另设业务上限。清理比较 UTC 实际经过的整日年龄 `floor((now-created_at)/86400秒) >= retention_days`；使用日/秒分解的安全整数运算，未来时间不满足期限。不将天数乘成 Duration，不将巨大天数从 now 相减；巨大合法值自然不会满足清理条件。仍保持现有 RFC3339 Z 时间字段。
- 每小时执行小批次（默认 200 条），在删除事务再次检查：是否有 pending/leased/retry_wait/failed 投递，是否为这些投递所需父消息/附件。保护优先于时间，查询与删除不可分离造成竞态。
- 本 Bot 普通历史引用不永久保护父消息；不存在的引用显示过期。受保护投递和失败记录没有隐含 TTL，资源不足时拒绝新写入，不自动丢弃。
- 删除消息时写 `message.deleted`；Bot 删除写 `bot.deleted`。变更日志默认保留 30 天，最旧可用游标 floor 单调前进；内容和变更日志保留分别判断，不为保护消息无限保留所有旧变更。
- 不能以所有设备已同步/已读为清理前提；FCM 重试也不保护历史。调整保留期限会影响后续清理，设置界面在缩短时显示明确影响。

## 6. Android 增量同步与共享已读

### 6.1 增量协议

`GET /client/sync?cursor=<opaque>&limit=200` 返回 `{epoch,changes,next_cursor,has_more,server_high_watermark}`。每次追赶固定一个高水位 H，按 `(cursor,H]` 分页。changes 包括消息 upsert（返回当前 revision，允许高于事件记录版本）、删除、Bot/设置变更、已读及投递状态，不返回已删除正文。客户端按实体版本应用；重复删除/旧版本是幂等 no-op。

全局设置变更规划为 `settings.updated` 的明确分支，携带全局设置版本与 `/admin/settings` 获取关联；重复/旧版本幂等忽略。与设备通知设置分支区分，不通过 changes 返回管理配置正文或扩大读取权限；具体 schema 由 S01 在最终 main 上冻结。

同一服务端读事务获取事件及当前投影，避免返回已删除对象的旧内容。已过期事件涉及已删除实体时返回 tombstone/无正文删除结果，不能造成整个批次失败。App 用 Room 事务保存批次及 next_cursor；未提交前崩溃则重放，不先更新游标。WS、FCM、前台打开和 WorkManager 全部进入同一单飞 SyncCoordinator。

客户端发现 epoch 不同或游标小于 floor，返回 `410 SYNC_RESET_REQUIRED`，必须提示服务器仅保留部分历史并重建服务端缓存。不能悄悄跳到当前水位，也不能把历史缺口误写为消息传输失败。

### 6.2 初次同步与重建

`/client/bootstrap` 首次响应在短读事务固定 H、epoch、Bot/共享已读快照和分页令牌；消息按不可变 bot_seq 分页读取，范围只包含创建序列不超过 H 的现存记录。分页期间读取当前 revision；可能缺少期间已删除内容，也可能读到更新后的版本，随后重放 `(H,current]` 补齐。客户端合并时永不以低 revision 覆盖高 revision。

重建数据写入独立本地 generation；保留尚未确认的本地 outbox 和上传任务，不能把它们当旧缓存删除。所有分页和追赶完成后再次检查 epoch/floor，仍有效才原子切换 generation、移除不在服务器集合的旧缓存。期间删除靠 tombstone 应用；Bot 元数据按版本处理。分页令牌绑定管理员、H 和位置，15 分钟过期；过期或 floor 已越过 H 时重新开始，不能保持跨请求长 SQLite 读事务。每页引用的附件也需重新校验现存状态。

### 6.3 已读与未读

用户查看会话后提交实际展示过的最大 bot_seq；服务端验证不超过该 Bot 已存在序号，以 max 合并所有设备，防止离线旧设备倒退。未读是现存 application 消息中 bot_seq > read_seq 的数量，不计自己的发送、callback 或单纯编辑；已过期记录不计数。客户端不能用 change_seq 作 read_seq。

已读更新发变更；其他设备下次同步取消相应系统通知。离线设备的既有系统通知不能保证即时撤回，不能声称跨设备原子去重。通知偏好始终属于设备，不因另一设备阅读/静音而覆盖。

## 7. 认证、初始化与设备

### 7.1 管理员和会话

拟定管理命令 `relaynest admin init --username ...`、`relaynest admin reset-password`，通过交互终端或 stdin 读密码，避免命令参数、环境回显和 shell 历史。init 仅数据库无管理员时可用；reset 在数据库事务内更新密码摘要和 auth_generation，撤销全部管理员会话/推送映射并关闭 WS。无需 RelayShelf 在线，也不改其密码。

密码使用带独立盐的 Argon2id，初始参数 64 MiB、3 次、并行度 1，部署性能测试后调整；登录校验并发默认 2，失败率限制避免资源放大。用户名/密码错误返回统一信息。会话随机值至少 256 bit，数据库仅保存 SHA-256 摘要；固定过期与滑动过期分别记录，撤销以数据库为准，不采用无法立即撤销的自包含 JWT。

拟定初始会话期限：Web 空闲 24 小时、绝对 7 天；Android 空闲 30 天、绝对 90 天。有效业务请求可延长空闲期，但不突破绝对期限。到期单端重新登录，不保存明文密码自动续期；实际 UI 显示原因并保留本地未发送内容。参数在安全实现阶段复核，不引入邮件恢复/公开注册。

Web Cookie 为 HttpOnly、SameSite=Lax，依据外部 HTTPS 设置 Secure，所有写操作验证同源 Origin/Referer 与会话 CSRF（包括登录的 Origin 验证）。Android Bearer 保存在 Keystore 密钥加密存储中，不写日志或普通备份；每安装生成设备 ID，重装不复用旧会话。撤销会话使对应推送映射失效；系统收到旧 FCM 提示也必须先验证当前会话，不展示缓存正文。


实例生命周期：正常重启、原库无回滚的停机迁移保留 ID，旧进程必须停止接流量；清库重建生成新 ID。备份恢复、回滚或复制为第二个独立部署，必须由维护者先停所有服务进程、隔离入口、备份并核对影响，再人工换 ID；单写事务同时推进管理员 auth_generation、撤销全部管理员会话（Web/Android）及现有推送映射，失败全部回滚，成功后重启并要求重新登录。不自动改 Bot 凭据或 sync_epoch。数据库副本无法自行识别 clone/回滚，此人工流程是保证前提，不提供在线 rotation API 或保留旧 ID 的恢复快捷项。

离线命令冻结为 `relaynest instance rotate-id --db PATH --expected-instance-id UUID --confirm-offline`。三个参数必填，拒绝未知/重复参数、位置参数、畸形 UUID，不接受新 ID 参数（始终安全随机生成），无在线调用/HTTP 接口。`--confirm-offline` 表示维护者确认已停全部进程并隔离入口；进程锁必须排他获取，无法获取不尝试修改。命令仅打开已存在、迁移完整数据库，不能创建库/自动跑迁移；expected ID 比对与 ID 更换/代次推进/全会话及推送映射撤销在 Store.Write 同一事务，任何一步/代次耗尽/提交失败整体回滚。退出码：0 已提交；2 用法/确认参数错误；3 数据库忙/独占锁不可用；4 身份缺失/腐败/expected 不匹配/迁移未完成；1 其他存储/随机/撤销/提交错误。输出只给固定成功/错误文本，不打印新旧 ID、会话/路径/SQL；成功后维护者重启再通过公开 probe 读取 ID。该参数是人工确认，不声称可检测任意其他机器上的副本进程。本卡不运行维护命令。

升级顺序固定：停旧服务及入口→备份/迁移原库→启动新 server→安装新 Android 并显式重登。旧 Android 缺 header 被新 server 拒绝，旧 server 无 instance 端点被新 Android fail-closed；因此存在客户端阻断窗口，不承诺透明滚动升级。旧 cipher（含无 ID 的现格式）清旧 Bearer并重登，不允许先 GET session 补认绑定。

### 7.2 Bot 凭据和传输

Bot API Token 和 Webhook Key 均至少 256 bit 随机、带公开凭据标识便于查找，摘要恒定时间比较；send/receive 能力绑定凭据授权，不能仅依 UI 是否显示输入框。重置只显示一次明文。receive 权限包括 claim/ACK/renew 及对应投递附件，不授予消息创建/编辑或其他 Bot 访问。

长轮询、写请求和 ACK 都在提交前验证凭据代次；Token 撤销后仍在运行的业务请求可以完成本地业务，但不能用旧 Token 再访问 Hub。管理员会话独立撤销，不改变接入应用凭据。Bot 停用状态覆盖其发送/接收授权。

## 8. 资源控制、错误与观测

初始限制：JSON 256 KiB、文本 64 KiB、callback_data 4 KiB、每消息附件 10 个、按钮 10 行且每行 5 个，label 128 字符；图片 10 MiB、文件 50 MiB（后两者按 PRD 可配置）。超限明确返回错误，不截断消息或回调参数。单用户默认管理员写请求 10/s、Bot 写请求 5/s，突发 20；附件并发默认 2；具体根据 PoC 调整，不作为固定吞吐承诺。

保护队列没有自动容量丢弃规则。磁盘写失败、SQLite busy 超时和资源压力时返回明确可重试错误；检查附件卷和 DB 卷空间，避免只检查其中之一。清理任务和同步分页有界，慢请求可取消。大附件不进入正文日志和常驻内存。

结构化日志仅记录 request_id、实体 ID、状态、耗时、错误码；删除 Authorization、Cookie、CSRF、FCM 注册、Webhook Key、密码、正文和 callback_data。原始 MoviePilot 错误先映射和脱敏；Web 失败队列不显示原始错误串。`/health/live` 仅进程存活，`/health/ready` 验证数据库/挂载可用，不暴露管理信息；FCM 不可用属于降级，不能令消息接收整体不可用。

管理员系统状态展示队列数量/最旧等待时间/失败数、附件恢复异常、数据库和推送错误摘要。连接状态区分 WS 在线、近期应用轮询与未知，单向 Webhook Bot 不宣称“持续在线”。无独立监控平台、消息队列、Redis 或自动备份。

实例 probe 的资源/日志规则以 §3.3 为准，公开 ID 不进入实体 ID 日志；原登录限流保持原值，不与公开令牌桶合并。

## 9. Android 架构与通知

### 9.0 实例门禁与凭据发布

新加密记录严格版本化并同时绑定规范化 endpoint、instance_id、Session.id、本地代次；AAD 和严格序列化版本共同升级，安装 UUID 独立，URL 不是实例身份。旧记录无 ID 清旧凭据并要求显式登录，不自动认领服务器 ID。

每次 login/session/logout、后续受保护 HTTP（含未来业务/outbox）及每次 WS 握手派发前，都要进行独立无凭据探测；与绑定 ID 一致才发送期望头和凭据。首次登录先探测，再接受显式密码；响应 ID/endpoint/代次不符，不保存/发布 Bearer，不用旧密码自动重试，不用新 Bearer另发“后探测”。每 endpoint/代次最多一个 probe 在飞；等待者串行，各自新探测和派发检查，成功只用于一次派发，不能缓存成功布尔值或无限共享。

endpoint/instance/session/代次同一协调锁核对，网络在锁外；返回时按原票据复核。切地址/注销在派发临界区前生效则不发旧请求，之后仅取消/丢弃旧结果。有效 probe 得到新 ID 时锁内推进代次、清 Bearer、取消旧请求、结束认证；缓存/草稿/outbox留在旧 endpoint+ID隔离，不自动迁移或重登后自动发送。用户显式登录新实例。网络/TLS/超时/3xx/404/429/503/畸形/超限均停止凭据发送，进入身份待验证，暂留加密记录但不得交网络；显式重试重新探测，不提供继续旧会话旁路。离线本地注销立即清凭据，远端 logout 未经 probe 不发送并展示未确认。存活 WS 对端固定，重连重新门禁。未来 Room/outbox/WS/推送仍须继承门禁，当前设计/机器契约检查不等于这些运行时已通过。


采用单向 UI 状态：Compose → ViewModel → Repository → Room/网络，协程和 Flow 驱动界面。`core/network` 按后端独立客户端，`core/sync` 负责唯一同步协调；`feature/bots`、`chat`、`management`、`settings` 先实现，RelayShelf feature 后置。不预建大量空模块。

本地 outbox 保存稳定请求 ID、载荷、附件上传状态及服务端回执；超时复用同一键查询/重试，服务端返回 accepted 后以服务端 ID 合并乐观记录。进程被杀后恢复，不重复生成幂等键。身份撤销后停止该会话自动发送；重新认证再恢复，不跨后端或跨服务实例发送旧数据。

附件通过系统选择器/Storage Access Framework 获取 URI，长任务检查持久访问权限，不假设文件路径可直接读取；下载至应用受控缓存或用户选定位置，按权限分享。图片解码按尺寸采样，Markdown 禁用原始 HTML，打开链接校验协议，敏感信息不进入系统剪贴板/通知默认内容。minSdk 和目标机型兼容范围尚未确认，工程前根据实际设备确定，不在本文擅自排除旧设备。

### 9.1 推送与补偿

- Hub 将“设备需要同步”的目标水位持久化到 push_jobs，可按设备合并；处理前再次检查会话、共享已读和设备配置。不把 FCM 成功当 ACK，也不无限保护消息。
- FCM 使用 data payload，只含不透明部署 ID、目标水位和类型；不含正文、附件 URL、callback 或凭据。客户端收到后调度唯一同步任务。默认普通优先级；高优先级仅在确需用户可见通知且经真机验证后使用，不能用于无限后台保活。
- SDK Token 更新回调和成功登录/前台恢复时登记当前有效注册及版本；更新只替换该设备/会话的对应通道注册。发送失败只有明确 UNREGISTERED 或确认合法载荷后的无效注册错误才按发送快照的通道及注册代次条件解绑；迟到响应不能删除已轮换的新注册，不删除设备、会话、其他通道或通知偏好，网络失败不删。
- 推送待办暂态失败指数退避，默认最多 8 次、最长 24 小时；耗尽仅记录推送降级，不进入业务失败队列。目标消息已清理/全部已读时丢弃提醒任务，不重新生成历史。
- WorkManager 周期补偿初始 30 分钟、要求网络可用，唯一任务，失败退避；系统最短周期是 15 分钟而非实时定时器。每次前台打开立即同步，`onDeletedMessages` 等丢提示情形触发补偿。
- Firebase 项目配置、服务器凭据、Hub 访问 Google 的网络、目标机 Google 服务及 ColorOS 行为都需 PoC；未取得这些环境前只能形成实现计划，不能声称通知可用。

### 9.2 提醒策略

同步落库后，在当前设备以 `(deployment_id, message_id)` 去重；Room 记录已处理提醒，系统 notification tag 使用同一键。崩溃后重复调用 notify 更新同一通知，不生成第二条。编辑更新通知内容时不响铃；删除/共享已读取消通知。由于系统通知与 Room 没有原子事务，不能承诺绝对恰好一次响铃，需故障测试评估窗口。

提醒前验证 Android 通知权限、系统渠道、全局/Bot 开关、免打扰、当前会话可见状态及最新共享已读。后台断网只收到提示时不展示过期缓存正文；等待补偿获取权威数据。锁屏预览关闭时使用通用文本和私有可见性，不将正文发给 FCM。免打扰按设备所选时区计算，跨午夜正确；时段内同步但不响铃，结束不补发旧提醒。完整缓存重建不对历史批量响铃；只对同步确定的新且未读消息产生提醒。

官方依据（检索于 2026-10-21）：[FCM Android 接收](https://firebase.google.com/docs/cloud-messaging/android/receive-messages)、[优先级](https://firebase.google.com/docs/cloud-messaging/android-message-priority)、[注册管理](https://firebase.google.com/docs/cloud-messaging/manage-tokens)、[WorkManager 周期约束](https://developer.android.com/reference/androidx/work/PeriodicWorkRequest)、[Android 通知权限](https://developer.android.com/develop/ui/compose/notifications/notification-permission)。Firebase 注册 API 和 SDK 有演进，实施时核对所锁版本实际采用的 Token/FID 接口，不将文档中的未来注册形式直接当作现有 Android SDK 能力。

### 9.3 推送通道适配器规划（PUSH-PLAN-01 已集成，PUSH-D1～D3 已批准）

本节细化用户已授权的 OPPO Push 与适配器方向；不是已经冻结的 HTTP/DDL/SDK 实现。PUSH-PLAN-01 已完成独立审计与集成；P03-07 继续冻结通道无关契约。用户于 2026-10-21 批准下列 PUSH-D1～D3，决定来源和边界见 [PUSH-DECISION-01](tasks/COMPLETED.md#e2)。§9.2 保持 FCM 自绘模式验收；本节仅对显式可选 OPPO 提醒模式记录已批准的体验差异。原提案的 title-only、注册后自动优先 vendor 及固定审核天数不作为实施依据。官方证据与不可读取限制见 [外部研究](tasks/COMPLETED.md#e2)，执行验证见 [P02-05](tasks/ANDROID.md#and-06)。Hub 只传输、存储和路由，不执行业务命令或 MCP。

#### 职责与插槽

- 同步提示为 FCM data hint，只含 §9.1 元数据；不可用时继续 app-open 与 WorkManager。提醒模式 `notify_channel=fcm|vendor|none` 单选，vendor 同时指定 `notify_provider=opush` 等具体适配器。vendor 模式 FCM 即使同步到新消息也不得自绘/响铃；none 仍可静默同步。不能将未注册、未验证或被拒绝的厂商渠道自动启用。
- 服务端适配器仅收策略过滤后的类型化输入：FCM hint 或厂商通用提示，不收消息对象/正文/附件 URL/callback/业务凭据；输出 `accepted`（仅平台受理）、`target_invalid`、`transient`、`rate_limited`、`configuration_error` 或 `unknown_delivery`。超时可能已受理，属于结果不确定，不以 HTTP 成功当展示、同步或已读 ACK。未知错误不删注册。
- Android provider 仅负责可用性探测、注册/刷新/注销与点击入口；协调器持有部署/会话并做单飞同步，provider 不执行业务。SDK 不可用/用户拒绝/注册失败为明确状态；只初始化已获必要同意的 SDK。点击只打开已验证的本应用入口，重新校验当前部署/会话后补偿同步，不携带正文、URL 或登录凭据，不依赖通知拉起应用做后台同步。
- 适配器静态装配于启动期；新增提供方增加对应适配实现与装配、能力声明/测试，不要求核心路由理解厂商字段，不引入动态插件执行平台。

#### 注册、模式与并发契约草案（P03-07 冻结输入）

| 内容 | 拟定约束 |
| --- | --- |
| 注册键 | 当前认证设备/会话 + `provider`，同设备允许 FCM 与 OPPO 共存；不接受客户端指定其他设备/会话 |
| 注册写入 | `provider`、`token`（不回显/不日志）、单调 `registration_version`、`expected_version`；同版本同内容幂等，旧版本/同版本不同内容冲突；服务端生成 `updated_at`；重装/重新登录隔离会话代次 |
| 更新/删除语义 | 增删只影响指定提供方；省略其他通道表示保持，禁止把局部上报解释成全量替换；条件删除带预期注册版本，避免旧失败覆盖新 token |
| 提醒配置 | `notify_channel`、`notify_provider`、设备全局/Bot/DND/预览与服务端单调 `settings_version`；`expected_version` CAS，冲突返回最新版本供重新读取；vendor 必须存在有效对应注册，不能以 SDK 注册成功覆盖用户偏好 |
| 撤销/退出 | 撤销整个会话的映射；设备会话生命周期与单个通道 token 失效分开；本地取消该会话工作，迟到回调不得重新登记 |
| job 快照 | 设备/会话代次、用途（sync/alert）、目标水位、提供方、注册版本、配置版本、稳定提醒键/过期时间；合并目标水位不使旧提醒复活 |

模式切换、配置/注册轮换使旧版本提醒 job 失效；发送前检查最新会话、注册代次、模式、配置、共享已读与过期时间，发送后以同一快照条件更新状态。工作租约/同设备串行投递防止双 worker；CAS 失败的旧结果不能改写新状态。已开始的外部发送无法由本地事务撤回：切换先停止旧代次新发送，隔离在途/结果不确定 job；发送超时不自动跨通道补响。按已批准 PUSH-D2，切换仅对切换后新事件生效，不重放旧提醒；用户接受已外发旧通知及在途残余。客户端根据最新确认配置验证延迟 FCM 提示/同步结果，离线不能确认 vendor 是否生效时不凭旧 fcm 配置补响；配置收敛前允许漏提醒，数据仍补偿同步。CAS/幂等/版本与切换边界仍由 P03-07 冻结并测试，不因用户接受风险而省略。

#### 厂商展示边界与已批准决定

厂商提醒在 Hub 发送前按设备时区、全局/Bot 开关、DND、共享已读及目标有效性过滤；DND 时丢弃提醒资格，静默同步可继续，结束不补响。发送前检查不能约束平台离线积压恢复时的展示：即使设置较短离线 TTL，跨入 DND、期间已读/删除/退出或模式切换后仍可能显示。`show_ttl`/覆盖 ID 不等于远程撤回或展示时策略校验；未有官方及真机证据前不能承诺厂商已展示通知可即时取消，也不能承诺两渠道绝对只响一次。

| 决定 | 用户已批准内容（2026-10-21） | 仍需验证/落实 |
| --- | --- | --- |
| PUSH-D1（原 D1）通用文案 | 固定标题「RelayNest 有新消息」和固定正文「打开 RelayNest 同步查看」；无业务正文、URL、凭据。不得通过空格或伪造正文绕过平台要求 | P02-05 核实现行请求、文案审核适用性与真机展示；用户批准不等于平台接受 |
| PUSH-D2（原 D2）模式激活/切换 | OPPO 提醒为显式可选模式；超时不自动跨 FCM 补响，切换仅作用未来新事件；接受配置收敛前漏提醒及已外发旧通知残余。首次默认沿现有 FCM/补偿，注册成功不静默更改；自动择优/回退需另行设计与确认 | P03-07 冻结模式 CAS；P09 验证互斥、旧 job/迟到结果与切换边界；OPPO 仅在外部门槛通过后可供选择 |
| PUSH-D3（原 D3）厂商能力边界 | 接受 OPPO 可选模式无业务预览、已发通知不保证远程撤回、离线积压展示无法保证 DND/已读取消/退出后撤回；TTL 只缓解，不作保证 | P02-05 记录实测边界；保留发送前策略过滤，FCM 原 §9.2 验收保持；不承诺绝对只响一次 |

PUSH-D1～D3 已批准，不再作为用户待决；历史规划/审计/集成报告保留其当时状态，以本节及决定报告为当前依据。账号/资格/测试与正式权限、SDK/API 及版本、签名关联、分类/模板/限额/错误码与真机能力仍未验证，由 P02-05 提供证据；不能把个人认证支持等同于个人账号拥有正式 Push 权限，不能自动降为公信或承诺审核天数。批准体验边界不激活厂商、不授权平台注册/提交或创建正式密钥。服务端鉴权秘密不得打包进 APK；SDK 必需客户端参数按锁定 SDK 文档分类并安全注入，不能把 MasterSecret 当客户端 AppSecret。

P02-05 外部审核只阻 OPPO 分支；P03 签名注入准备、通道无关契约及 P4～8 可继续。P09-03/04 应先完成框架+FCM，再独立验收 OPPO；建议子卡与依赖见 [实施报告](tasks/COMPLETED.md#e2)，模块任务状态仅由调度登记。

## 10. MoviePilot v3.1.5 适配研究与条件方案

本节只读研究，不包含最小插件代码或运行结果。固定仓库 `jxxghp/MoviePilot`，v3.1.5，commit `99ee18d0e6a27422a4affd3d4ec7d8d56772e4c3`。以下链接锁定 commit，行号是本次读取位置。

| 能力 | 源码证据 | 判断 |
| --- | --- | --- |
| 插件模块入口 | [`app/sdk/plugin/base.py` get_module，约 L236](https://github.com/jxxghp/MoviePilot/blob/99ee18d0e6a27422a4affd3d4ec7d8d56772e4c3/app/sdk/plugin/base.py#L236)；[`dispatcher.py`](https://github.com/jxxghp/MoviePilot/blob/99ee18d0e6a27422a4affd3d4ec7d8d56772e4c3/app/runtime/extensions/module/dispatcher.py) | 源码确认可声明方法映射，调度器按契约执行插件 provider；不是任意方法都可安全接管 |
| 通知配置 | [`NotificationConf`](https://github.com/jxxghp/MoviePilot/blob/99ee18d0e6a27422a4affd3d4ec7d8d56772e4c3/app/schemas/system.py#L88)；[`application/notification.py`](https://github.com/jxxghp/MoviePilot/blob/99ee18d0e6a27422a4affd3d4ec7d8d56772e4c3/app/application/notification.py) | type 为字符串且有配置/稳定 ID；未因此确认配套前端“自定义”可形成完整第三方消息渠道 |
| 通知事件 | [`_messaging.py` L387 起](https://github.com/jxxghp/MoviePilot/blob/99ee18d0e6a27422a4affd3d4ec7d8d56772e4c3/app/chain/_messaging.py#L387) | 普通同步/异步通知经路由后发送 NoticeMessage，再调 post_message；事件会受管理员/用户目标路由影响，不能把每次事件都视为独立原始通知 |
| 候选列表 | [`_messaging.py` L512 起](https://github.com/jxxghp/MoviePilot/blob/99ee18d0e6a27422a4affd3d4ec7d8d56772e4c3/app/chain/_messaging.py#L512) | post_medias_message、post_torrents_message 独立调用模块发送；仅监听 NoticeMessage 的设计已发现覆盖不足 |
| 文本与回调入站 | [`MessageChain.process/handle_message`](https://github.com/jxxghp/MoviePilot/blob/99ee18d0e6a27422a4affd3d4ec7d8d56772e4c3/app/chain/message.py) | process 经 message_parser 规范化；handle_message 支持 source、userid、引用、附件及 callback_data；如何提供合规独立 channel 待验证 |
| 渠道身份 | [`types.py` L485](https://github.com/jxxghp/MoviePilot/blob/99ee18d0e6a27422a4affd3d4ec7d8d56772e4c3/app/schemas/types.py#L485)；[`schemas/message.py`](https://github.com/jxxghp/MoviePilot/blob/99ee18d0e6a27422a4affd3d4ec7d8d56772e4c3/app/schemas/message.py) | NotificationChannel 固定，无 RelayNest；消息/响应模型依赖该类型。直接传入新字符串不是已支持方案 |
| 回调能力 | [`notification.py` L233、L300](https://github.com/jxxghp/MoviePilot/blob/99ee18d0e6a27422a4affd3d4ec7d8d56772e4c3/app/schemas/notification.py#L233)；[`message.py` L483](https://github.com/jxxghp/MoviePilot/blob/99ee18d0e6a27422a4affd3d4ec7d8d56772e4c3/app/chain/message.py#L483) | handle_message 下游检查 supports_callbacks；普通 Web 无回调能力，WebAgent 有但语义不同，不能因能力合适擅自借用 |
| 编辑和直接发送 | [`_messaging.py` L575、L627](https://github.com/jxxghp/MoviePilot/blob/99ee18d0e6a27422a4affd3d4ec7d8d56772e4c3/app/chain/_messaging.py#L575)；[`contracts.py`](https://github.com/jxxghp/MoviePilot/blob/99ee18d0e6a27422a4affd3d4ec7d8d56772e4c3/app/runtime/extensions/module/contracts.py) | edit_message / send_direct_message 及结果 ID 契约存在；Hub ID 映射和插件命中范围待实测 |

**明确结论**：存在可研究的插件扩展面，但尚不能证明“不改核心、独立渠道、完整双向”三者同时满足。仅靠自定义配置、NoticeMessage 或传新渠道字符串的方案均不能直接定稿。固定枚举带来的限制已由源码确认，是否有不改变既定边界的完整扩展路径仍待最小 PoC。

### 10.1 PoC 前的适配边界

拟定插件组件：HubClient、持久 Inbox/发送 Outbox、消息映射、MP 入站桥、通知/候选/编辑 provider、生命周期管理。均放插件目录，持久数据使用 MP 插件自己的数据目录，不修改 MP 业务数据库或 Hub 数据库。

插件按明确映射绑定一个 Hub Bot、MP 配置 ID 和 MP 管理员身份。Hub 管理员身份不自动等于 MoviePilot 管理员；配置需显式绑定且由 MP 原生权限流程验证，不能对所有输入硬编码 `is_channel_admin=true`。上下文至少保存 Hub Bot/message/event ↔ MP source/user/chat/message，支持并发两台 Android 指向同一对话，防止回复串 Bot。

消息接收时先持久 Inbox 和必要附件字节再 ACK，执行结果通过发送 Outbox + 稳定键发布；长耗时业务不持有 Hub 租约，附件在插件侧保留至业务不再需要。主动通知、定向回复、候选列表、直接发送、编辑必须分别覆盖，并保证每条路径只有一个负责转发的入口，避免事件监听和 provider 双重发送。不能靠正文哈希永久去重不同真实通知；优先 MP 稳定事件键，无稳定身份的路径要在 PoC 量化重复窗口。

图片/文件由插件合法读取字节后上传 Hub；Hub 不接受命令它抓取远程 URL。插件不能把 MP 内网 URL 当作 Android 一定可达的图片；不可读附件明确报告，不伪装为持久附件。MP 业务参数、命令和确认步骤留在 MP 原有链路；Hub callback 保持透明。

### 10.2 必须通过的验证门槛

完整顺序：匹配部署前端的“通知→添加→自定义”保存/重启保留 → 只命中目标插件 → 主动广播和定向回复 → 入站文本经原有业务链 → 媒体/种子候选及回调 → 引用上下文 → 直接发送返回 ID → 编辑更新 → 停用/重置/重启与 ACK 重放 → 多路径不遗漏、不重复、不影响已有渠道。

PoC 要提交准确源码入口、身份/能力方案和运行记录。不得通过修改枚举、全局能力表或猴子补丁后宣称“纯插件兼容已通过”。如必须采用这些方式，停止正式插件设计，向用户提交：具体受阻调用链、必要最小修改、维护成本、可替代路径及对 AC-07 的影响；等待选择。其他 Hub/Android 通用模块可继续设计，但核心版 AC-07 不自行豁免。

## 11. RelayShelf v0.4.4 后置接入

固定仓库 `ZephyrLeeX/RelayShelf`，tag v0.4.4，commit `5a537cb0728d86e0c27d384d65d2c48d19e01002`。本节为源码预研及未来兼容入口，不是首版开发依赖。OpenAPI 内部 info.version 为 0.11.0，与 tag 编号不同；依固定 commit 识别目标，不误读为已改用另一个发布版本。

### 11.1 已确认的认证约束

- [`internal/auth/router.go`](https://github.com/ZephyrLeeX/RelayShelf/blob/5a537cb0728d86e0c27d384d65d2c48d19e01002/internal/auth/router.go)：基路径 `/api/v1`；登录和 TOTP 登录需要同源，非安全操作叠加 Authenticate、SameOrigin、CSRF，SSE 单独认证。
- [`middleware.go`](https://github.com/ZephyrLeeX/RelayShelf/blob/5a537cb0728d86e0c27d384d65d2c48d19e01002/internal/auth/middleware.go)：Host/外部 scheme 校验；Origin 缺失退到 Referer，均缺失拒绝；scheme、host（含端口）必须匹配配置；写请求读取 `X-CSRF-Token`。
- [`cookie.go`](https://github.com/ZephyrLeeX/RelayShelf/blob/5a537cb0728d86e0c27d384d65d2c48d19e01002/internal/auth/cookie.go)：HTTPS 使用 `__Host-session`、Secure、HttpOnly、SameSite=Lax、Path=/；HTTP 使用 `relayshelf_session`。不能按内网 HTTP 判断其公网 Cookie 策略。
- [`handler.go`](https://github.com/ZephyrLeeX/RelayShelf/blob/5a537cb0728d86e0c27d384d65d2c48d19e01002/internal/auth/handler.go)：密码通过但需 TOTP 时 202 返回 challengeToken/expiresAt，不发会话 Cookie；完成 `/auth/login/totp` 后才建立会话，bootstrap 含 csrfToken。

拟定 Android 专属 OkHttp CookieJar 持久保存两端隔离的凭据；RelayShelf 写请求以其配置的 canonical HTTPS Origin（包含自定义端口）设置 Origin 和 CSRF。原生客户端设置 Origin 不构成跳过服务端认证；禁用跨 Origin 凭据重定向，不降低 TLS 校验。此方案理论上可满足源码规则，仍需在真实反代/TOTP/会话更新条件下验证。

### 11.2 接口/功能矩阵初稿

来源为固定 commit 的 [`api/openapi.yaml`](https://github.com/ZephyrLeeX/RelayShelf/blob/5a537cb0728d86e0c27d384d65d2c48d19e01002/api/openapi.yaml)。表中“存在”只代表契约存在，不代表 Android 实测通过或完整界面核对完成。

| 功能域 | 已确认的接口入口（相对 /api/v1） | 后续核对任务 |
| --- | --- | --- |
| 登录与安全 | `/auth/login`、`/auth/login/totp`、`/auth/session`、TOTP enroll/confirm/disable、`/auth/password/change` | 挑战失效、改密撤销、TOTP 设置和部分登录 |
| 会话/设备 | `/sessions`、`/sessions/{id}`、`/devices`、`/devices/{id}` | 当前设备、远程撤销、重命名与权限 |
| 内容 | `/messages`、`/messages/{id}`、tags、attachments 子资源 | 文本/Markdown/代码/链接/图片/文件、编辑版本、展示和校验 |
| 生命周期 | make-permanent、extend、trash、`/trash`、restore、永久删除 | Temporary/Permanent、期限、回收站及权限全部对齐 |
| 文件 | `/uploads`、`/uploads/{id}`、`parts/{partNumber}`、complete；附件 download/preview/thumbnail、消息打包下载 | chunkSize/partCount/completedParts、重启续传、校验去重、Range/错误与 URI 权限 |
| 搜索/整理 | `/search`、`/tags`、消息 tags/favorite、`/saved-searches` | 含 PRD 简表未展开的保存搜索及复杂条件，不能漏掉 |
| 跨用户 | `/messages/direct-send`、`/messages/{id}/forward`、`/users/recipients` | 发送/转发权限与实际共享语义，不凭“共享”一词编造新接口 |
| 隐私 | sensitive、sensitive-body | 显式揭示、编辑、内存/缓存/截图与预览策略核对既有产品 |
| 实时 | `/events` | 只含元数据，事件作为重新取数提示 |
| 系统/管理 | `/storage/status`、admin settings/storage/status/users 等 | 与现有界面及账号权限核对；“完整现有功能”不能只覆盖 PRD 简表 |

上传校验/去重的具体哈希字段、校验时机、下载恢复与各 MIME 预览规则还需继续追踪 handler/service/finalize 和 Web 上传实现；当前不能声称已完成源码全量核对。

SSE 的 OpenAPI 明确 `Last-Event-ID is ignored`；[`internal/realtime/handler.go`](https://github.com/ZephyrLeeX/RelayShelf/blob/5a537cb0728d86e0c27d384d65d2c48d19e01002/internal/realtime/handler.go) 有心跳和会话验证。因此恢复连接后重新获取列表、详情及待上传状态，不能沿用 Hub 的持久增量游标或假定可事件重放。

### 11.3 后续阶段门槛

先逐页核对部署 v0.4.4 界面、OpenAPI、服务端实际行为，形成完整矩阵，再做 Android 最小 API PoC，最后开发 Compose 功能。双端登录状态为 `Unconfigured / SignedOut / Authenticating / AwaitingTOTP / Authenticated / Expired / Failed`，分别维护；一个后端异常不阻止另一端使用。统一输入密码只在登录过程短暂持有，不保存用于无限自动登录。

若 API 缺失，提交源码证据、客户端替代方案、服务端最小修改和影响供用户确认；未经确认不变更 RelayShelf、不删原有功能、不用 WebView 替代。尚未完成全量矩阵和真机验证，不将延期阶段标为“兼容设计已冻结”。

## 12. 定稿与验证状态

核心 Hub 协议、状态和数据边界可作为后续 OpenAPI/DDL 审查输入；外部库版本、目标 Android 兼容范围和发布环境在工程前锁定。MoviePilot 适配内部身份/能力路径仍为条件方案，必须先通过 §10.2；RelayShelf 全量矩阵和 PoC 按 C-05 延后；FCM、文件故障恢复及并发可靠性均无运行结果。

本轮没有启动服务、执行插件、调用真实设备、上传凭据或运行网络集成测试。准确验收用例、前置条件与未通过时停止规则见 `DEVELOPMENT_PLAN.md`。
