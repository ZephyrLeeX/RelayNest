# P03-07-S03-S03 全契约证据与 G-API 收口实施报告

基线/取证 HEAD：`b8c3c2b32c258b7e6f3c63d458faa28bf491e60d`；工作树 `/home/lijx/.codex/worktrees/api-contract-final/RelayNest`，分支 `codex/api-contract-final`。开发报告，尚待不同 agent 固定提交审计与精确集成。

## 结论与范围

本次仅新增此报告，无 API 增量。按 T3.1/T3.2/T3.3、T6.1、T8 及已批准扩展核对，未发现有确认依据而未覆盖的契约缺口。39 操作、115 Schema、1229 例（568 正例、661 反例）与基线逐项相同；124 个不同本地引用全部解析。下列矩阵为契约覆盖证据，不把数量本身作为验收。G-API 建议由调度结合独立审计与精确集成登记；本报告不改变 gate、registry、卡片状态或父卡完成状态。

例子精确索引：E0001 表示冻结 `server/api/examples.json` 数组的第 1 项（零基索引 0），名称、schema、valid、value 与其他分支字段均以该项为准；因此索引和六文件指纹共同唯一定位，不复制正文/URL/凭据。区间两端均包含。路由表列直接引用，完整传递引用沿 Schema 图解析；“传递候选”只表示引用可达，不声称每个候选都实际命中 oneOf 分支。

## 冻结输入与前置

| 文件 | SHA-256 |
| --- | --- |
| server/api/openapi.yaml | `5995c6aa94eb3242797c1a20b493a8b4ba32084e574a6c4bdbbd3275637cfabc` |
| server/api/examples.json | `996f76f8c2eb2ef168c63cfbcc7dbc18b0f3a56721a60e964b2f13fd3e4a8fd1` |
| server/api/validate.py | `6132bffada05f769bbd2df15ea1bc26a0aa45ea96829b6855fad6ddf6654a020` |
| server/api/README.md | `3f8c440488f2dc5120c55a270fd51fdf332e0183b8c02df17b0a509709b071d0` |
| server/api/requirements.in | `4d56f1903c85ca444dde0966a7fd2702f15242b2cbc5f35eb1fcd273a0317d7c` |
| server/api/requirements.txt | `4e1992c8e79a37d1671671afd5e24f686837f719078ac7bdab06b5919231f184` |

前置审计/集成精确映射沿已集成报告；下表 main 集成点均实际验证为本基线祖先。原实现 SHA 与 main cherry-pick SHA 不混用。

| 归属 | 原实现/修复；独审 | main 精确集成点或集成报告提交 |
| --- | --- | --- |
| P03-04 | 见 P03-04-integration-01.md | `30efad4f16234c07f42a2dd6e746d9d980b7dadf` |
| P03-05/上传回归 | 见 P03-05-integration-01.md | `38d52c3bae74010167d4defe0e681e843d497fc8` |
| P03-06 | 见 P03-06-integration-01.md | `57ff401f95d9f747ec85938cbb521190463ae3c2` |
| S01-S01 | 966066831a6b1b75720cae131b2e5d17c22e6e35（sync/read）；d51bceebcf4a81637920efd9c60b802a36943ee1（独审，完整值见前卡报告） | `33cc7585e873a5debaddc5b93c8657b60d5a73a8` |
| S01-S02/F01 | a379d9620c9a0b616c575d749fd05030376437c3；0993016dd18b120ce4c300c7b86087474e5eee9e | `3cdb5e228b7d124890d45250728b260b2091a6db` |
| S01-S03/父同步8项 | 169fd8043e48e37762f11e845f00f56ce8024933；91215226a724679ca163be5279c049cb6bba3ad7 | `5cdaf15f7e01b663dd0f2b68fc044d9e875aaa79` |
| S02-S01/F01 | a63bd59c8f50dcd83353e279b044f25f4383780c；c05a5b93b64b5348bb98270ade4856fc23061fac | `876935a219c3a1bf828aeaabb2e8a82ca2bb5d4a` |
| S02-S02 | 6901bc09de53995435102d11f2dfad9bd3f46059；14081bcb11a5ce8c7ce7b700b6b1ca6e10bdf1e1 | `82ebd62634ecf9025670a75aa956a45667c55ea7` |
| S02-S03/父设备8项 | 692e11964ea74cb84cf62470a08e4c3a1780a01f；36a05a2523a968f384e82de4eb599a10f31226eb | `42cd918c01c34bb7baa918420dad0daa2b4d6a98` |
| S03-S02/health | 966ec9e57150555342d067ebfb9afcf6eb03990a；5a0c8731859cb14d20fcb49044cf41fdb5f12b55；I01 5a91ddcc70e5df4f0388f42d1953b778e4991af0 | `6f89cf633c5ecd8677ca61b53d99a12969a8fcd8` |
| S03-S01/settings | 27c5ac9bbdfd3c5edd09901a7ac01a5cc0ecfb82；8d48a71c59d7c24408f4ca5a59c08e6070211ce2；I01 329238af18cabaf44256b0e55afa7464119f7aac | `394d6117b75605f660663bb7c8087cb6b4410cc1` |

## T3 全路径、方法、身份与实现归属

下列均为契约归属，后续服务端运行时仍需实现/实际验收。除健康探针使用操作级 `/` servers 外，真实前缀 `/api/v1`。AdminBearer 在 client/ws、current 设备路由等还受已冻结 Android 会话约束；不能仅凭 scheme 名认为任意管理员会话可用。Cookie 写分支同时要求 CSRF，Origin/同源按既有安全声明。Webhook 不获取上传/读取能力，Bot send/receive 权限沿各操作描述及反例。

| 方法/路径 | operationId / 归属 | security OR 分支（分支内 AND） | 直接 refs | 操作例 |
| --- | --- | --- | --- | --- |
| GET /admin/settings | `getGlobalSettings` / S03-S01 | AdminBearer OR WebCookie | responses/400, responses/401, responses/403, responses/429, responses/503, schemas/GlobalSettings | E1068–E1071 |
| PUT /admin/settings | `updateGlobalSettings` / S03-S01 | AdminBearer OR WebCookie+CSRF | responses/400, responses/401, responses/403, responses/413, responses/422, responses/429, responses/503, schemas/GlobalSettings, schemas/GlobalSettingsConflict, schemas/GlobalSettingsUpdate | E1077–E1082, E1088, E1095 |
| GET /health/live | `getHealthLive` / S03-S02 | 公开 | schemas/HealthOK | E0923–E0926 |
| GET /health/ready | `getHealthReady` / S03-S02 | 公开 | schemas/HealthOK, schemas/HealthUnavailable | E0927–E0934 |
| GET /admin/status | `getAdminStatus` / S03-S02 | AdminBearer OR WebCookie | responses/401, responses/403, responses/429, responses/503, schemas/AdminSystemStatus | E0900, E0902–E0906, E0959–E0966 |
| GET /client/ws | `connectClientWS` / S01-S03 | AdminBearer | responses/400, responses/401, responses/403, responses/429, responses/503, schemas/WSFrame | E0569–E0579, E0613–E0624 |
| POST /auth/login | `login` / P03-04 | 公开 | responses/400, responses/401, responses/403, responses/413, responses/422, responses/429, responses/503, schemas/LoginRequest, schemas/LoginResponse | — |
| GET /devices | `listDeviceSessions` / S02-S01/F01 | AdminBearer OR WebCookie | responses/400, responses/401, responses/403, responses/429, responses/503, schemas/DeviceSessionPage, schemas/OpaqueCursor | E0638 |
| DELETE /devices/{id}/session | `revokeDeviceSession` / S02-S01/F01 | AdminBearer OR WebCookie+CSRF | responses/400, responses/401, responses/403, responses/404, responses/429, responses/503, schemas/ID | E0657–E0659 |
| GET /auth/session | `getSession` / P03-04 | AdminBearer OR WebCookie | responses/401, responses/403, responses/429, responses/503, schemas/SessionResponse | — |
| POST /auth/logout | `logout` / P03-04 | AdminBearer OR WebCookie+CSRF | responses/401, responses/403, responses/429, responses/503 | — |
| GET /admin/bots | `listBots` / P03-04 | AdminBearer OR WebCookie | responses/400, responses/401, responses/403, responses/422, responses/429, responses/503, schemas/BotPage, schemas/OpaqueCursor | — |
| POST /admin/bots | `createBot` / P03-04 | AdminBearer OR WebCookie+CSRF | responses/400, responses/401, responses/403, responses/413, responses/422, responses/429, responses/503, schemas/CreateBotRequest, schemas/CreatedBotResponse | — |
| PATCH /admin/bots/{id} | `patchBot` / P03-04 | AdminBearer OR WebCookie+CSRF | responses/400, responses/401, responses/403, responses/404, responses/409, responses/413, responses/422, responses/429, responses/503, schemas/Bot, schemas/PatchBotRequest | — |
| DELETE /admin/bots/{id} | `deleteBot` / P03-04 | AdminBearer OR WebCookie+CSRF | responses/400, responses/401, responses/403, responses/404, responses/409, responses/413, responses/422, responses/429, responses/503, schemas/DeleteBotRequest | — |
| POST /admin/bots/{id}/credentials/api-token/reset | `resetAPIToken` / P03-04 | AdminBearer OR WebCookie+CSRF | responses/401, responses/403, responses/404, responses/429, responses/503, schemas/IssuedAPIToken | — |
| POST /admin/bots/{id}/credentials/webhook-key/reset | `resetWebhookKey` / P03-04 | AdminBearer OR WebCookie+CSRF | responses/401, responses/403, responses/404, responses/429, responses/503, schemas/IssuedWebhookKey | — |
| POST /bot/messages | `createBotMessage` / P03-05 | BotAPIToken | parameters/IdempotencyKey, responses/400, responses/401, responses/403, responses/404, responses/409, responses/410, responses/413, responses/422, responses/429, responses/503, schemas/CreateMessageRequest, schemas/Message | E0137 |
| POST /webhooks/{bot_id} | `createWebhookMessage` / P03-05 | WebhookKey | parameters/IdempotencyKey, responses/400, responses/401, responses/403, responses/404, responses/409, responses/410, responses/413, responses/422, responses/429, responses/503, schemas/CreateMessageRequest, schemas/ID, schemas/Message | E0145 |
| PATCH /bot/messages/{id} | `editBotMessage` / P03-05 | BotAPIToken | parameters/IdempotencyKey, responses/400, responses/401, responses/403, responses/404, responses/409, responses/410, responses/413, responses/422, responses/429, responses/503, schemas/EditMessageRequest, schemas/ID, schemas/Message | E0139 |
| POST /client/bots/{id}/messages | `createClientMessage` / P03-05 | AdminBearer | parameters/IdempotencyKey, responses/400, responses/401, responses/403, responses/404, responses/409, responses/410, responses/413, responses/422, responses/429, responses/503, schemas/ClientMessageReceipt, schemas/CreateMessageRequest, schemas/ID | E0147 |
| GET /client/bots/{id}/messages | `listClientMessages` / P03-05 | AdminBearer | responses/400, responses/401, responses/403, responses/404, responses/409, responses/410, responses/413, responses/422, responses/429, responses/503, schemas/ID, schemas/MessagePage, schemas/OpaqueCursor | — |
| POST /client/bots/{id}/callbacks | `createCallback` / P03-05 | AdminBearer | parameters/IdempotencyKey, responses/400, responses/401, responses/403, responses/404, responses/409, responses/410, responses/413, responses/422, responses/429, responses/503, schemas/CallbackReceipt, schemas/CallbackRequest, schemas/ID | E0141 |
| GET /client/messages/{id} | `getClientMessage` / P03-05 | AdminBearer | responses/400, responses/401, responses/403, responses/404, responses/409, responses/410, responses/413, responses/422, responses/429, responses/503, schemas/ID, schemas/Message | — |
| POST /attachments | `uploadAttachment` / P03-05 | AdminBearer OR BotAPIToken | parameters/IdempotencyKey, responses/400, responses/401, responses/403, responses/404, responses/409, responses/410, responses/413, responses/422, responses/429, responses/503, schemas/Attachment, schemas/ID | E0143, E0153, E0155 |
| GET /attachments/{id} | `downloadAttachment` / P03-05 | AdminBearer OR BotAPIToken | responses/400, responses/401, responses/403, responses/404, responses/409, responses/410, responses/413, responses/422, responses/429, responses/503, schemas/ByteRanges, schemas/ContentRange, schemas/Error, schemas/ID, schemas/StrongETag | E0149, E0151, E0210–E0218, E0220–E0231 |
| POST /bot/events/claim | `claimEvents` / P03-06 | BotAPIToken | responses/400, responses/401, responses/403, responses/404, responses/409, responses/413, responses/422, responses/429, responses/503, schemas/ClaimRequest, schemas/ClaimResponse | E0257–E0259, E0318–E0325 |
| POST /bot/deliveries/{id}/ack | `ackDelivery` / P03-06 | BotAPIToken | responses/400, responses/401, responses/403, responses/404, responses/409, responses/413, responses/422, responses/429, responses/503, schemas/AckReceipt, schemas/ID, schemas/LeaseRequest | E0276–E0277, E0326–E0331, E0369 |
| POST /bot/deliveries/{id}/renew | `renewDelivery` / P03-06 | BotAPIToken | responses/400, responses/401, responses/403, responses/404, responses/409, responses/413, responses/422, responses/429, responses/503, schemas/ID, schemas/RenewReceipt, schemas/RenewRequest | E0286, E0332–E0335 |
| GET /admin/deliveries | `listDeliveries` / P03-06 | AdminBearer OR WebCookie | responses/400, responses/401, responses/403, responses/422, responses/429, responses/503, schemas/DeliveryMetadataPage, schemas/DeliveryState, schemas/ID, schemas/OpaqueCursor | E0310–E0311, E0342–E0343 |
| POST /admin/deliveries/{id}/retry | `retryDelivery` / P03-06 | AdminBearer OR WebCookie+CSRF | parameters/IdempotencyKey, responses/400, responses/401, responses/403, responses/404, responses/409, responses/410, responses/413, responses/422, responses/429, responses/503, schemas/ID, schemas/RetryDeliveryReceipt, schemas/RetryDeliveryRequest | E0314–E0317, E0336–E0341 |
| GET /client/bootstrap | `bootstrapClient` / S01-S02/F01 | AdminBearer | responses/400, responses/401, responses/403, responses/404, responses/410, responses/413, responses/422, responses/429, responses/503, schemas/BootstrapResponse, schemas/OpaqueCursor | E0447, E0450, E0469–E0474 |
| GET /client/sync | `syncClientChanges` / S01-S01 | AdminBearer | responses/400, responses/401, responses/403, responses/404, responses/410, responses/413, responses/422, responses/429, responses/503, schemas/OpaqueCursor, schemas/SyncPage | E0370, E0433–E0434, E1226 |
| PUT /client/bots/{id}/read | `updateClientRead` / S01-S01 | AdminBearer | responses/400, responses/401, responses/403, responses/404, responses/410, responses/413, responses/422, responses/429, responses/503, schemas/ID, schemas/UpdateReadRequest, schemas/UpdateReadResponse | E0374, E0435–E0436 |
| GET /devices/current/push | `getCurrentPush` / S02-S02 | AdminBearer | responses/400, responses/401, responses/403, responses/429, responses/503, schemas/PushRegistrationState | E0708 |
| PUT /devices/current/push | `updateCurrentPush` / S02-S02 | AdminBearer | responses/400, responses/401, responses/403, responses/409, responses/413, responses/422, responses/429, responses/503, schemas/PushRegistrationState, schemas/PushRegistrationUpdate | E0709, E0734–E0736 |
| DELETE /devices/current/push/{provider} | `deleteCurrentPush` / S02-S02 | AdminBearer | responses/400, responses/401, responses/403, responses/409, responses/413, responses/422, responses/429, responses/503, schemas/PushProvider, schemas/PushRegistrationDelete, schemas/PushRegistrationState | E0710, E0737–E0739 |
| GET /devices/current/notifications | `getCurrentNotifications` / S02-S03 | AdminBearer | responses/400, responses/401, responses/403, responses/429, responses/503, schemas/NotificationSettingsState | E0820 |
| PUT /devices/current/notifications | `updateCurrentNotifications` / S02-S03 | AdminBearer | responses/400, responses/401, responses/403, responses/404, responses/413, responses/422, responses/429, responses/503, schemas/NotificationSettingsConflict, schemas/NotificationSettingsState, schemas/NotificationSettingsUpdate | E0793, E0821–E0824 |

没有操作级例索引的旧操作通过其请求/响应 Schema 例、既有声明检查及对应前置审计取证，不伪报存在 HTTP 集成例。参数/响应/错误的精确引用及状态码以冻结 operation 为准。

## T3.3 握手与帧单列

GET /api/v1/client/ws（connectClientWS，S01-S03）：Android Bearer，Authorization 不进查询串；101 无 JSON 正文；400/401/403/429/503 为受保护 JSON 错误。Origin、主协议不兼容、会话撤销、无业务写入、提示不推进游标及重连恢复，沿 WS 契约与 ws_state 合成夹具。25 秒 ping、75 秒断开、60 秒退避上限是实现要求，未实测。

| Frame schema | keys / refs | 精确例（直接或 WSFrame 内嵌） |
| --- | --- | --- |
| WSProtocolVersion | major, minor;  | — |
| WSHello | type, epoch, change_seq, protocol_version; #/components/schemas/Sequence, #/components/schemas/SyncEpoch, #/components/schemas/WSProtocolVersion | E0529, E0533–E0541, E0545, E0549, E0553, E0557, E0563, E0595, E0627, E0630–E0633 |
| WSChangesAvailable | type, change_seq; #/components/schemas/Sequence | E0530, E0542, E0546, E0550, E0554, E0558 |
| WSSessionRevoked | type;  | E0531, E0544, E0548, E0552, E0556, E0560, E0565–E0568, E0580–E0594, E0596–E0612, E0626, E0628–E0629, E0634–E0637 |
| WSUnknownOptional | type, optional;  | — |
| WSFrame | ; #/components/schemas/WSChangesAvailable, #/components/schemas/WSHello, #/components/schemas/WSSessionRevoked, #/components/schemas/WSUnknownOptional | E0529–E0564, E0595, E0625–E0627, E0630–E0633 |

## T6 全 changes 类别

| 分支/类型 | keys | 直接或分支正反例 |
| --- | --- | --- |
| SyncMessageUpsert / message.upsert | type, change_seq, message | E0392–E0393 |
| SyncMessageDelete / message.deleted | type, change_seq, id, bot_id, bot_seq, revision, deleted_at | E0394–E0395 |
| SyncBotUpsert / bot.upsert | type, change_seq, bot | E0396–E0397 |
| SyncBotDelete / bot.deleted | type, change_seq, id, version, deleted_at | E0398–E0399 |
| SyncReadUpdate / read.updated | type, change_seq, read | E0400–E0401 |
| SyncDeliveryUpsert / delivery.upsert | type, change_seq, delivery | E0402–E0403 |
| SyncDeviceNotifications / device.notifications.updated | type, change_seq, settings | E0888–E0890, E1105 |
| SyncGlobalSettings / settings.updated | type, change_seq, settings_version, recovery_path | E1102–E1104, E1106–E1119, E1121–E1123 |

固定 H、epoch/floor/Sequence 十进制字符串、当前实体投影与事件版本分离、删除无正文、重复旧版 no-op、Room 批次与游标原子保存、410 重建和 bootstrap seed 真实 sync-result 来源沿 S01/F01。settings.updated 只带配置版本与恢复读取关联；device.notifications.updated 按认证 device_id 与 settings_version 应用，管理配置正文不经同步泄露。

## T8、父卡与 S03 验收逐项映射

| 验收项 | 精确契约/证据 | 结论边界 |
| --- | --- | --- |
| P03-07 原验收1 epoch/floor/序号/版本 | SyncPage/SyncChange/SyncResetRequired、BootstrapPage、ReadState、Sequence、Version；S01-S01/S02/F01/S03 报告；附录图与例 | 声明/合成可核对；DB/Room NOT_RUN |
| 原验收2 正式 G-API | T3 全39操作、T3.3、T6八类、T8两类、115组件图及1229例冻结；本次锁定校验 | 提供收口建议；待独审/集成由调度登记 |
| 原验收3 多通道并存/局部更新/轮换迟到失败/CAS/退出迟到/非法provider/越权/D1-D3 | S02-S01/F01 会话、S02-S02 push_registration_state、S02-S03 notification_settings_state/notification_job_state；见对应报告和附录所有例索引 | 已集成契约与合成证据；无SDK/REST/系统展示实测 |
| 原验收4 受影响检查 | 下文三个命令、六文件/清单逐项比较 | 实际 PASS；无条件运行时 NOT_RUN/后续 blocked |
| 原验收5 本卡提交/证据/独审集成 | 仅此报告；精确提交在交接提供 | 开发不能自审/自行标 done |
| S03-1 全局GET/PUT/CAS/默认/权限/错误 | GlobalSettingsValue/GlobalSettingsVersion/GlobalSettings/GlobalSettingsUpdate/GlobalSettingsConflict；S03-S01 实施/A01/I01及冻结操作 | 无新增参数/权限 |
| S03-2 保留期/清理保护 | GlobalSettings 操作及README说明；T5.4和P06-08负责实际清理 | 不承诺PUT同步清理；未ACK/failed及附件引用保护；NOT_RUN |
| S03-3 全局变更与T6全分支 | 上述八分支逐类正例、严格 refs；SyncGlobalSettings | 无任意object占位；不扩大读取权限 |
| S03-4 根公开health | getHealthLive/getHealthReady 操作级servers=/、security=[]、text/plain; charset=utf-8，200 ok\n / ready503 unavailable\n，no-store/nosniff | health_response声明/合成；不套JSON Error；HTTP NOT_RUN |
| S03-5 管理投影 | AdminSystemStatus：observed_at/queue/connections/attachment_recovery/database/push；QueueStatus/ConnectionStatus/AttachmentRecoveryStatus/DatabaseStatus/PushStatus | AdminBearer/WebCookie；脱敏无正文/路径/密钥/原始异常；Webhook非持续在线，FCM降级非unready |
| S03-6 全索引矩阵/非回归 | 本报告路由、WS、Schema/例、T6/T8与前置SHA表；原有六文件未改 | 不以局部PASS代全量 |
| S03-7 父验收与G-API | 本表原验收1–5；命令实绩；独审集成待后续 | 不放行G-OPPO或签名/发布 |
| S03-8 下游移交/父汇总 | 下一节精确keys、前置SHA、NOT_RUN | 父done由调度在独审/集成后判断 |

## 下游精确移交与外部门槛

P06-08：GET/PUT /admin/settings，retention_days/image_max_bytes/file_max_bytes 为规范正十进制 SQLite64 字符串 1..9223372036854775807，0不支持；默认30/10485760/52428800。全局专用 version/expected_version 为1..9007199254740991 JSON整数，公共Version不变；GET完整配置/version/updated_at，PUT成功含同值写增1，同事务CAS/更新时间/settings.updated。先输入、再CAS、再耗尽；GLOBAL_SETTINGS_CONFLICT / GLOBAL_SETTINGS_VERSION_EXHAUSTED 为409且带最新版本/重读路径，耗尽原子不写且retryable=false；400结构、422语义、413超限。恢复路径/admin/settings，服务端UTC Z时间。管理状态六组keys见上表，根live仅存活、ready依DB/挂载，FCM只降级；配置缩短后保护与清理、附件恢复/队列统计尚待运行时实现。精确来源 S03-S01/health SHA见前置表。

P4 服务端：实现本报告全路由、身份/错误/版本与事务规则；Bot Hub仅传输、存储、路由，禁止执行业务命令/MCP。P5 Web：auth/session、admin/bots、admin/deliveries及retry、admin/settings/status、devices会话管理；Cookie写CSRF/同源；不复用失败队列为正文历史，不建立聊天WS。P8 Android：client/bootstrap/sync/read/history/detail/ws、current push GET/PUT/条件DELETE、notifications GET/PUT；epoch/floor/H与真实sync seed、当前device/session代次、registration_version/settings_version、notify_channel=fcm|vendor|none、notify_provider分离；不得回显token或跨设备更新。全部精确schema/operationId/错误例沿图与冻结源；401撤销、409冲突重读、410同步重建、413大小、422语义、429/503及Retry-After沿T3.1。

P9 MoviePilot：bot/messages、bot/events/claim、deliveries ack/renew、attachments及Webhook（仅凭独立WebhookKey发送）；租约/幂等/消息revision和业务权限保留前卡边界。v3.1.5是既有研究证据版本，目标>=3.0.0须另验；P1用户最小PoC例外/G-MP与P2历史PoC/G-FCM不作为正式AC或用户放行。D1-D3：显式vendor用通用文案、仅未来新事件切换、超时不跨FCM补响，剩余离线展示限制不扩到FCM；通知同步职责独立，FCM仍用于同步，provider可用性不等于用户已启用。vendor激活保留未满足前置状态。

实际 HTTP、WS升级/帧收发/心跳/断线、SQLite事务/多worker/清理、Android Room、SDK/厂商REST、FCM与OPPO真机展示、手机/NAS、签名发布均 NOT_RUN。没有OPPO账号/手机/NAS仅阻塞相应后续实际验收；声明和合成不替代这些验收。未解决契约增量：无已确认缺口；独立审计与精确集成尚未执行。

## 实际验证

解释器 `/tmp/relaynest-health-contract-locked/bin/python`，Python 3.13.15；importlib.metadata 实际枚举18个锁定发行包，无升级。该venv无pip，未将pip list失败误计成功；无需恢复环境。

- `python -m openapi_spec_validator server/api/openapi.yaml`：实际 OK。
- `python server/api/validate.py`：实际 PASS: OpenAPI 3.1; 115 schemas; 568 valid + 661 invalid examples; 39 operations/security。
- `git diff --check`：实际 PASS（含新增报告的 stage 后检查）。
- 自基线逐字节比较六文件，以及逐项比较 paths/完整operation对象、components/115 Schema、1229例及两锁：实际全部一致；18依赖名称/版本与requirements.txt逐项相等。无API新增故未机械重跑各域历史声明变异矩阵，已运行validate现有全部声明/正反例检查。

## Schema / 例 / 引用图完整附录

组件 keys 和例编号构成全部115 Schema的直接图。没有直接例的组合组件沿引用边在父schema夹具覆盖；此处不伪称每个组件均有独立例。全局/操作组件引用均实际解析。

| Schema | properties keys / enum / const | 直接 Schema refs | 直接例 | 传递候选例 |
| --- | --- | --- | --- | --- |
| GlobalSettingsValue |   | Sequence | — | E0988–E1042, E1053–E1068, E1072–E1077, E1083–E1087, E1124–E1224 |
| GlobalSettingsVersion |   | Version | E1043–E1052 | E0370–E0373, E0384–E0387, E0392–E0413, E0419–E0432, E0437–E0446, E0888–E0890, E0894, E0988–E1068, E1072–E1077, E1083–E1229 |
| GlobalSettings | retention_days, image_max_bytes, file_max_bytes, version, updated_at  | GlobalSettingsValue, GlobalSettingsVersion, UTCDateTime | E0988, E1060–E1068, E1072–E1077, E1083–E1087, E1124–E1224 | E0988, E1060–E1068, E1072–E1077, E1083–E1087, E1124–E1224 |
| GlobalSettingsUpdate | retention_days, image_max_bytes, file_max_bytes, expected_version  | GlobalSettingsValue, GlobalSettingsVersion | E0989–E1042, E1053–E1059 | E0989–E1042, E1053–E1059 |
| GlobalSettingsConflict |   | GlobalSettingsVersion | E1088–E1101, E1225 | E1088–E1101, E1225 |
| SyncGlobalSettings | type, change_seq, settings_version, recovery_path  | GlobalSettingsVersion, Sequence | E1102, E1104, E1106, E1108, E1110, E1112, E1114, E1116, E1118 | E0370–E0373, E0384–E0387, E0392–E0413, E0419–E0432, E0437–E0446, E0888–E0890, E0894, E1102–E1123, E1226–E1229 |
| HealthOK |  {"const": "ok\n"} | — | E0923–E0930, E0935–E0946, E0967–E0987 | E0923–E0930, E0935–E0946, E0967–E0987 |
| HealthUnavailable |  {"const": "unavailable\n"} | — | E0931–E0934 | E0931–E0934 |
| StatusCount |   | — | — | E0900–E0922, E0947–E0958 |
| QueueStatus | pending, leased, retry_wait, failed, oldest_waiting_at  | StatusCount, UTCDateTime | — | E0900–E0922, E0947–E0958 |
| ConnectionStatus | ws_online_sessions, recent_poll_bots, unknown_bots, webhook_state  | StatusCount | — | E0900–E0922, E0947–E0958 |
| AttachmentRecoveryStatus | anomaly_count, last_error_code  | StatusCount | — | E0900–E0922, E0947–E0958 |
| DatabaseStatus | state, last_error_code  | — | — | E0900–E0922, E0947–E0958 |
| PushStatus | state, last_error_code  | — | — | E0900–E0922, E0947–E0958 |
| AdminSystemStatus | observed_at, queue, connections, attachment_recovery, database, push  | AttachmentRecoveryStatus, ConnectionStatus, DatabaseStatus, PushStatus, QueueStatus, UTCDateTime | E0900–E0922, E0947–E0958 | E0900–E0922, E0947–E0958 |
| PushProvider |  {"enum": ["fcm", "opush"]} | — | — | E0688–E0701, E0708–E0733, E0740–E0765, E0832–E0885, E0887, E0895–E0899 |
| PushBarrierVersion |   | — | — | E0688–E0733, E0740–E0765 |
| PushRegistrationVersion |   | — | — | E0688–E0701, E0832–E0885, E0887, E0895–E0899 |
| PushRegistrationUpdate | provider, token, registration_version, expected_version  | PushBarrierVersion, PushProvider, PushRegistrationVersion | E0688–E0701 | E0688–E0701 |
| PushRegistrationDelete | expected_version  | PushBarrierVersion | E0702–E0707 | E0702–E0707 |
| PushRegistration | provider, registration_version, registered, updated_at  | PushBarrierVersion, PushProvider, UTCDateTime | E0718–E0721 | E0708–E0733, E0740–E0765 |
| PushRegistrationState | session_id, registrations  | ID, PushRegistration | E0708–E0717, E0722–E0733, E0740–E0765 | E0708–E0717, E0722–E0733, E0740–E0765 |
| WSProtocolVersion | major, minor  | — | — | E0529–E0564, E0595, E0625–E0627, E0630–E0633 |
| WSHello | type, epoch, change_seq, protocol_version  | Sequence, SyncEpoch, WSProtocolVersion | — | E0529–E0564, E0595, E0625–E0627, E0630–E0633 |
| WSChangesAvailable | type, change_seq  | Sequence | — | E0529–E0564, E0595, E0625–E0627, E0630–E0633 |
| WSSessionRevoked | type  | — | E0565–E0568, E0580–E0594, E0596–E0612, E0628–E0629, E0634–E0637 | E0529–E0568, E0580–E0612, E0625–E0637 |
| WSUnknownOptional | type, optional  | — | — | E0529–E0564, E0595, E0625–E0627, E0630–E0633 |
| WSFrame |   | WSChangesAvailable, WSHello, WSSessionRevoked, WSUnknownOptional | E0529–E0564, E0595, E0625–E0627, E0630–E0633 | E0529–E0564, E0595, E0625–E0627, E0630–E0633 |
| ID |   | — | E0061 | E0017–E0027, E0044–E0045, E0048–E0055, E0057–E0136, E0219, E0257–E0269, E0276–E0277, E0286–E0311, E0314–E0317, E0370–E0374, E0384–E0413, E0419–E0432, E0437–E0468, E0475–E0508, E0512–E0528, E0638–E0656, E0660–E0687, E0708–E0717, E0722–E0733, E0740–E0792, E0798–E0821, E0825–E0899, E1103, E1105, E1107, E1109, E1111, E1113, E1115, E1117, E1119–E1123, E1226–E1229 |
| Sequence |   | — | E0001–E0013 | E0001–E0013, E0119–E0120, E0122–E0136, E0257–E0269, E0287–E0311, E0314–E0317, E0370–E0432, E0437–E0468, E0475–E0564, E0595, E0625–E0627, E0630–E0633, E0832–E0885, E0887–E0890, E0894–E0899, E0988–E1042, E1053–E1068, E1072–E1077, E1083–E1087, E1102–E1224, E1226–E1229 |
| Version |   | — | — | E0034–E0038, E0044–E0045, E0048–E0055, E0057, E0104–E0128, E0257–E0269, E0287–E0311, E0314–E0317, E0370–E0374, E0384–E0413, E0419–E0432, E0437–E0468, E0475–E0508, E0512–E0528, E0888–E0890, E0894, E0988–E1068, E1072–E1077, E1083–E1229 |
| UTCDateTime |   | — | E0028–E0031 | E0022–E0031, E0044–E0045, E0048–E0055, E0057–E0060, E0119–E0136, E0257–E0269, E0276–E0277, E0286–E0311, E0314–E0317, E0370–E0374, E0384–E0413, E0419–E0432, E0437–E0468, E0475–E0528, E0638–E0656, E0660–E0687, E0708–E0733, E0740–E0766, E0791–E0792, E0820–E0821, E0832–E0890, E0894–E0922, E0947–E0958, E0988, E1060–E1068, E1072–E1077, E1083–E1087, E1103, E1105, E1107, E1109, E1111, E1113, E1115, E1117, E1119–E1224, E1226–E1229 |
| OpaqueCursor |   | — | E0569–E0576, E0615–E0624 | E0045–E0047, E0057, E0122, E0310–E0311, E0370–E0373, E0384–E0387, E0404–E0413, E0419–E0432, E0437–E0468, E0475–E0508, E0512–E0528, E0569–E0576, E0615–E0624, E0638–E0656, E0660–E0687, E0894, E1226–E1229 |
| Pagination | next_cursor, has_more  | OpaqueCursor | E0046–E0047 | E0046–E0047 |
| Error | error  | — | E0014–E0016, E0137–E0156, E0318–E0343, E0369, E0433–E0436, E0469–E0474, E0577–E0579, E0613–E0614, E0657–E0659, E0734–E0739, E0822–E0823, E0959–E0966, E1069–E1071, E1078–E1082 | E0014–E0016, E0137–E0156, E0318–E0343, E0369, E0433–E0436, E0469–E0474, E0577–E0579, E0613–E0614, E0657–E0659, E0734–E0739, E0822–E0823, E0959–E0966, E1069–E1071, E1078–E1082 |
| Capabilities | send, receive  | — | — | E0032–E0035, E0039, E0043–E0045, E0048–E0055, E0057, E0370–E0373, E0384–E0387, E0392–E0413, E0419–E0432, E0437–E0468, E0475–E0508, E0512–E0528, E0888–E0890, E0894, E1103, E1105, E1107, E1109, E1111, E1113, E1115, E1117, E1119–E1123, E1226–E1229 |
| LoginRequest |   | ID | E0017–E0021 | E0017–E0021 |
| Administrator | username  | — | — | E0022–E0027, E0058–E0060, E0638–E0656, E0660–E0687 |
| DeviceSessionPage | items, next_cursor, has_more  | OpaqueCursor, Session | E0638–E0656, E0660–E0687 | E0638–E0656, E0660–E0687 |
| Session | id, administrator, client_type, device_id, device_name, idle_expires_at, absolute_expires_at  | Administrator, ID, UTCDateTime | — | E0022–E0027, E0058–E0060, E0638–E0656, E0660–E0687 |
| WebSession |   | Session | — | E0022–E0027, E0058, E0060 |
| AndroidSession |   | Session | — | E0022–E0026, E0058–E0059 |
| WebSessionResponse | session, csrf_token  | WebSession | E0027, E0060 | E0022–E0027, E0058, E0060 |
| AndroidSessionResponse | session  | AndroidSession | — | E0024–E0026, E0058 |
| SessionResponse |   | AndroidSessionResponse, WebSessionResponse | E0024–E0026, E0058 | E0024–E0026, E0058 |
| AndroidLoginResponse | session, access_token, token_type  | AndroidSession | E0059 | E0022–E0023, E0059 |
| LoginResponse |   | AndroidLoginResponse, WebSessionResponse | E0022–E0023 | E0022–E0023 |
| CredentialMetadata | credential_id, generation  | — | E0056 | E0039–E0045, E0048–E0057, E0370–E0373, E0384–E0387, E0392–E0413, E0419–E0432, E0437–E0468, E0475–E0508, E0512–E0528, E0888–E0890, E0894, E1103, E1105, E1107, E1109, E1111, E1113, E1115, E1117, E1119–E1123, E1226–E1229 |
| BotCredentials | api_token, webhook_key  | CredentialMetadata | — | E0044–E0045, E0048–E0055, E0057, E0370–E0373, E0384–E0387, E0392–E0413, E0419–E0432, E0437–E0468, E0475–E0508, E0512–E0528, E0888–E0890, E0894, E1103, E1105, E1107, E1109, E1111, E1113, E1115, E1117, E1119–E1123, E1226–E1229 |
| Bot | id, name, enabled, capabilities, version, credentials, connection  | BotCredentials, Capabilities, ID, UTCDateTime, Version | E0048–E0055 | E0044–E0045, E0048–E0055, E0057, E0370–E0373, E0384–E0387, E0392–E0413, E0419–E0432, E0437–E0468, E0475–E0508, E0512–E0528, E0888–E0890, E0894, E1103, E1105, E1107, E1109, E1111, E1113, E1115, E1117, E1119–E1123, E1226–E1229 |
| CreateBotRequest | name, enabled, capabilities  | Capabilities | E0032–E0033 | E0032–E0033 |
| PatchBotRequest | expected_version, name, enabled, capabilities  | Capabilities, Version | E0034–E0035 | E0034–E0035 |
| DeleteBotRequest | confirm_name, expected_version  | Version | E0036–E0038 | E0036–E0038 |
| IssuedAPIToken | credential, token, capabilities  | Capabilities, CredentialMetadata | E0039, E0043 | E0039, E0043–E0044 |
| IssuedWebhookKey | credential, key  | CredentialMetadata | E0040–E0042 | E0040–E0042, E0044 |
| CreatedBotResponse | bot, api_token, webhook_key  | Bot, IssuedAPIToken, IssuedWebhookKey | E0044 | E0044 |
| BotPage | items, next_cursor, has_more  | Bot, OpaqueCursor | E0045, E0057 | E0045, E0057 |
| MessageText |   | — | — | E0062–E0110, E0119–E0120, E0122–E0128, E0219, E0257–E0269, E0370–E0373, E0384–E0387, E0392–E0413, E0419–E0432, E0437–E0468, E0475–E0508, E0512–E0528, E0888–E0890, E0894, E1103, E1105, E1107, E1109, E1111, E1113, E1115, E1117, E1119–E1123, E1226–E1229 |
| CallbackData |   | — | — | E0062–E0110, E0119–E0128, E0219, E0257–E0269, E0370–E0373, E0384–E0387, E0392–E0413, E0419–E0432, E0437–E0468, E0475–E0508, E0512–E0528, E0888–E0890, E0894, E1103, E1105, E1107, E1109, E1111, E1113, E1115, E1117, E1119–E1123, E1226–E1229 |
| MessageButton |   | CallbackData | — | E0062–E0110, E0119–E0120, E0122–E0128, E0219, E0257–E0269, E0370–E0373, E0384–E0387, E0392–E0413, E0419–E0432, E0437–E0468, E0475–E0508, E0512–E0528, E0888–E0890, E0894, E1103, E1105, E1107, E1109, E1111, E1113, E1115, E1117, E1119–E1123, E1226–E1229 |
| MessageButtons |   | MessageButton | — | E0062–E0110, E0119–E0120, E0122–E0128, E0219, E0257–E0269, E0370–E0373, E0384–E0387, E0392–E0413, E0419–E0432, E0437–E0468, E0475–E0508, E0512–E0528, E0888–E0890, E0894, E1103, E1105, E1107, E1109, E1111, E1113, E1115, E1117, E1119–E1123, E1226–E1229 |
| AttachmentReference | id, kind  | ID | — | E0062–E0110, E0119–E0120, E0122–E0128, E0219, E0257–E0269, E0370–E0373, E0384–E0387, E0392–E0413, E0419–E0432, E0437–E0468, E0475–E0508, E0512–E0528, E0888–E0890, E0894, E1103, E1105, E1107, E1109, E1111, E1113, E1115, E1117, E1119–E1123, E1226–E1229 |
| CreateMessageRequest | format, text, attachments, reply_to_message_id, buttons  | AttachmentReference, ID, MessageButtons, MessageText | E0062–E0103, E0219 | E0062–E0103, E0219 |
| EditMessageRequest | format, text, attachments, reply_to_message_id, buttons, expected_revision  | AttachmentReference, ID, MessageButtons, MessageText, Version | E0104–E0110 | E0104–E0110 |
| Message | id, bot_id, bot_seq, revision, sender, format, text, attachments, reply_to_message_id, buttons, created_at, updated_at  | AttachmentReference, ID, MessageButtons, MessageText, Sequence, UTCDateTime, Version | E0119, E0123–E0127 | E0119–E0120, E0122–E0128, E0257–E0269, E0370–E0373, E0384–E0387, E0392–E0413, E0419–E0432, E0437–E0468, E0475–E0508, E0512–E0528, E0888–E0890, E0894, E1103, E1105, E1107, E1109, E1111, E1113, E1115, E1117, E1119–E1123, E1226–E1229 |
| ClientMessageReceipt | message, delivery_id  | ID, Message | E0120, E0128 | E0120, E0128 |
| CallbackRequest | message_id, message_revision, button_id  | ID, Version | E0111–E0118 | E0111–E0118 |
| CallbackReceipt | event_id, delivery_id, message_id, message_revision, button_id, callback_data, created_at  | CallbackData, ID, UTCDateTime, Version | E0121 | E0121 |
| MessagePage | items, next_cursor, has_more  | Message, OpaqueCursor | E0122 | E0122 |
| Attachment | id, bot_id, kind, filename, media_type, size, sha256, state, created_at  | ID, Sequence, UTCDateTime | E0129–E0136 | E0129–E0136 |
| ByteRanges |   | — | E0157–E0205 | E0157–E0205 |
| ContentRange |   | — | E0208–E0218, E0220–E0238 | E0208–E0218, E0220–E0238 |
| StrongETag |   | — | E0206–E0207 | E0206–E0207 |
| ConsumerID |   | — | — | E0239–E0256, E0270–E0275, E0278–E0285 |
| LeaseToken |   | — | — | E0257–E0267, E0270–E0275, E0278–E0285 |
| ClaimRequest | consumer_id, wait_seconds, limit  | ConsumerID | E0239–E0256 | E0239–E0256 |
| MessageDeliveryEvent | type, message  | Message | — | E0257–E0269 |
| CallbackDeliveryEvent | type, message_id, message_revision, button_id, callback_data, administrator_id, device_id, created_at  | CallbackData, ID, UTCDateTime, Version | — | E0257–E0269 |
| DeliveryEvent |   | CallbackDeliveryEvent, MessageDeliveryEvent | E0268–E0269 | E0257–E0269 |
| ClaimedDelivery | delivery_id, event_id, attempt, retry_generation, lease_token, lease_until, event  | DeliveryEvent, ID, LeaseToken, Sequence, UTCDateTime | E0260–E0267 | E0257–E0267 |
| ClaimResponse | deliveries  | ClaimedDelivery | E0257–E0259 | E0257–E0259 |
| LeaseRequest | consumer_id, lease_token  | ConsumerID, LeaseToken | E0270–E0275 | E0270–E0275 |
| RenewRequest | consumer_id, lease_token, extend_seconds  | ConsumerID, LeaseToken | E0278–E0285 | E0278–E0285 |
| AckReceipt | delivery_id, event_id, state, acked_at  | ID, UTCDateTime | E0276–E0277 | E0276–E0277 |
| RenewReceipt | delivery_id, lease_until  | ID, UTCDateTime | E0286 | E0286 |
| DeliveryState |  {"enum": ["pending", "leased", "retry_wait", "acked", "failed"]} | — | — | E0287–E0311, E0314–E0317, E0370–E0373, E0384–E0387, E0392–E0413, E0419–E0432, E0437–E0446, E0888–E0890, E0894, E1103, E1105, E1107, E1109, E1111, E1113, E1115, E1117, E1119–E1123, E1226–E1229 |
| DeliveryMetadata | delivery_id, event_id, bot_id, state, state_version, attempt, retry_generation, created_at, updated_at, lease_until, next_attempt_at, acked_at, error_code  | DeliveryState, ID, Sequence, UTCDateTime, Version | E0287–E0309 | E0287–E0311, E0314–E0317, E0370–E0373, E0384–E0387, E0392–E0413, E0419–E0432, E0437–E0446, E0888–E0890, E0894, E1103, E1105, E1107, E1109, E1111, E1113, E1115, E1117, E1119–E1123, E1226–E1229 |
| DeliveryMetadataPage | items, next_cursor, has_more  | DeliveryMetadata, OpaqueCursor | E0310–E0311 | E0310–E0311 |
| RetryDeliveryRequest |   | — | E0312–E0313, E0344–E0368 | E0312–E0313, E0344–E0368 |
| RetryDeliveryReceipt | delivery  | DeliveryMetadata | E0314–E0317 | E0314–E0317 |
| SyncEpoch |   | — | — | E0370–E0373, E0384–E0387, E0404–E0413, E0419–E0432, E0437–E0468, E0475–E0564, E0595, E0625–E0627, E0630–E0633, E0894, E1226–E1229 |
| ReadState | bot_id, read_seq, version, updated_at  | ID, Sequence, UTCDateTime, Version | E0388–E0391 | E0370–E0374, E0384–E0413, E0419–E0432, E0437–E0468, E0475–E0508, E0512–E0528, E0888–E0890, E0894, E1103, E1105, E1107, E1109, E1111, E1113, E1115, E1117, E1119–E1123, E1226–E1229 |
| UpdateReadRequest | through_seq  | Sequence | E0375–E0383, E0414–E0418 | E0375–E0383, E0414–E0418 |
| UpdateReadResponse | read, server_time  | ReadState, UTCDateTime | E0374 | E0374 |
| SyncMessageUpsert | type, change_seq, message  | Message, Sequence | — | E0370–E0373, E0384–E0387, E0392–E0413, E0419–E0432, E0437–E0446, E0888–E0890, E0894, E1103, E1105, E1107, E1109, E1111, E1113, E1115, E1117, E1119–E1123, E1226–E1229 |
| SyncMessageDelete | type, change_seq, id, bot_id, bot_seq, revision, deleted_at  | ID, Sequence, UTCDateTime, Version | — | E0370–E0373, E0384–E0387, E0392–E0413, E0419–E0432, E0437–E0446, E0888–E0890, E0894, E1103, E1105, E1107, E1109, E1111, E1113, E1115, E1117, E1119–E1123, E1226–E1229 |
| SyncBotUpsert | type, change_seq, bot  | Bot, Sequence | — | E0370–E0373, E0384–E0387, E0392–E0413, E0419–E0432, E0437–E0446, E0888–E0890, E0894, E1103, E1105, E1107, E1109, E1111, E1113, E1115, E1117, E1119–E1123, E1226–E1229 |
| SyncBotDelete | type, change_seq, id, version, deleted_at  | ID, Sequence, UTCDateTime, Version | — | E0370–E0373, E0384–E0387, E0392–E0413, E0419–E0432, E0437–E0446, E0888–E0890, E0894, E1103, E1105, E1107, E1109, E1111, E1113, E1115, E1117, E1119–E1123, E1226–E1229 |
| SyncReadUpdate | type, change_seq, read  | ReadState, Sequence | — | E0370–E0373, E0384–E0387, E0392–E0413, E0419–E0432, E0437–E0446, E0888–E0890, E0894, E1103, E1105, E1107, E1109, E1111, E1113, E1115, E1117, E1119–E1123, E1226–E1229 |
| SyncDeliveryUpsert | type, change_seq, delivery  | DeliveryMetadata, Sequence | — | E0370–E0373, E0384–E0387, E0392–E0413, E0419–E0432, E0437–E0446, E0888–E0890, E0894, E1103, E1105, E1107, E1109, E1111, E1113, E1115, E1117, E1119–E1123, E1226–E1229 |
| SyncChange |   | SyncBotDelete, SyncBotUpsert, SyncDeliveryUpsert, SyncDeviceNotifications, SyncGlobalSettings, SyncMessageDelete, SyncMessageUpsert, SyncReadUpdate | E0392–E0403, E0888–E0890, E1103, E1105, E1107, E1109, E1111, E1113, E1115, E1117, E1119–E1123 | E0370–E0373, E0384–E0387, E0392–E0413, E0419–E0432, E0437–E0446, E0888–E0890, E0894, E1103, E1105, E1107, E1109, E1111, E1113, E1115, E1117, E1119–E1123, E1226–E1229 |
| SyncPage | epoch, changes, next_cursor, has_more, server_high_watermark, server_time  | OpaqueCursor, Sequence, SyncChange, SyncEpoch, UTCDateTime | E0370–E0373, E0384–E0387, E0404–E0413, E0419–E0432, E0437–E0446, E0894, E1226–E1229 | E0370–E0373, E0384–E0387, E0404–E0413, E0419–E0432, E0437–E0446, E0894, E1226–E1229 |
| BootstrapMessage | created_change_seq, message  | Message, Sequence | — | E0447–E0468, E0475–E0508, E0512–E0528 |
| BootstrapPage | kind, epoch, server_high_watermark, retention_floor, bots, read_states, messages, page_token, next_token, has_more, sync_cursor, issued_at, expires_at, server_time  | BootstrapMessage, Bot, OpaqueCursor, ReadState, Sequence, SyncEpoch, UTCDateTime | E0448–E0449, E0451–E0468, E0475–E0508, E0512–E0528 | E0447–E0468, E0475–E0508, E0512–E0528 |
| BootstrapVerified | kind, epoch, server_high_watermark, retention_floor, catchup_position, server_time  | Sequence, SyncEpoch, UTCDateTime | E0509–E0511 | E0447, E0450, E0509–E0511 |
| BootstrapResponse |   | BootstrapPage, BootstrapVerified | E0447, E0450 | E0447, E0450 |
| NotificationSettingsVersion |   | — | — | E0370–E0373, E0384–E0387, E0392–E0413, E0419–E0432, E0437–E0446, E0766–E0821, E0824–E0899, E1103, E1105, E1107, E1109, E1111, E1113, E1115, E1117, E1119–E1123, E1226–E1229 |
| NotificationDND | enabled, timezone, start, end  | — | — | E0370–E0373, E0384–E0387, E0392–E0413, E0419–E0432, E0437–E0446, E0766–E0792, E0798–E0821, E0825–E0831, E0886, E0888–E0894, E1103, E1105, E1107, E1109, E1111, E1113, E1115, E1117, E1119–E1123, E1226–E1229 |
| NotificationBotOverride | bot_id, enabled  | ID | — | E0370–E0373, E0384–E0387, E0392–E0413, E0419–E0432, E0437–E0446, E0766–E0792, E0798–E0821, E0825–E0831, E0886, E0888–E0894, E1103, E1105, E1107, E1109, E1111, E1113, E1115, E1117, E1119–E1123, E1226–E1229 |
| NotificationSettingsUpdate | expected_version, enabled, bot_overrides, dnd, preview, notify_channel, notify_provider  | NotificationBotOverride, NotificationDND, NotificationSettingsVersion | E0767–E0790, E0798–E0819, E0825–E0831, E0891–E0893 | E0767–E0790, E0798–E0819, E0825–E0831, E0891–E0893 |
| NotificationSettings | device_id, settings_version, updated_at, enabled, bot_overrides, dnd, preview, notify_channel, notify_provider  | ID, NotificationBotOverride, NotificationDND, NotificationSettingsVersion, UTCDateTime | E0791–E0792, E0886 | E0370–E0373, E0384–E0387, E0392–E0413, E0419–E0432, E0437–E0446, E0766, E0791–E0792, E0820–E0821, E0886, E0888–E0890, E0894, E1103, E1105, E1107, E1109, E1111, E1113, E1115, E1117, E1119–E1123, E1226–E1229 |
| NotificationSettingsState | settings, server_time  | NotificationSettings, UTCDateTime | E0766, E0820–E0821 | E0766, E0820–E0821 |
| NotificationSettingsConflict | error  | NotificationSettingsVersion | E0793–E0797, E0824 | E0793–E0797, E0824 |
| SyncDeviceNotifications | type, change_seq, settings  | NotificationSettings, Sequence | — | E0370–E0373, E0384–E0387, E0392–E0413, E0419–E0432, E0437–E0446, E0888–E0890, E0894, E1103, E1105, E1107, E1109, E1111, E1113, E1115, E1117, E1119–E1123, E1226–E1229 |
| NotificationJobSnapshot | session_id, device_id, purpose, target_watermark, provider, registration_version, settings_version, reminder_key, bot_id, bot_seq, expires_at, alert_eligible  | ID, NotificationSettingsVersion, PushProvider, PushRegistrationVersion, Sequence, UTCDateTime | E0832–E0885, E0887, E0895–E0899 | E0832–E0885, E0887, E0895–E0899 |

其它组件精确keys/ref图：

- `responses/400` → #/components/schemas/Error；keys=description, headers, content
- `responses/401` → #/components/schemas/Error；keys=description, headers, content
- `responses/403` → #/components/schemas/Error；keys=description, headers, content
- `responses/404` → #/components/schemas/Error；keys=description, headers, content
- `responses/409` → #/components/schemas/Error；keys=description, headers, content
- `responses/410` → #/components/schemas/Error；keys=description, headers, content
- `responses/413` → #/components/schemas/Error；keys=description, headers, content
- `responses/422` → #/components/schemas/Error；keys=description, headers, content
- `responses/429` → #/components/schemas/Error；keys=description, headers, content
- `responses/503` → #/components/schemas/Error；keys=description, headers, content
- `securitySchemes/AdminBearer` → 无引用；keys=type, scheme, description
- `securitySchemes/WebCookie` → 无引用；keys=type, in, name, description
- `securitySchemes/CSRF` → 无引用；keys=type, in, name, description
- `securitySchemes/BotAPIToken` → 无引用；keys=type, scheme, description
- `securitySchemes/WebhookKey` → 无引用；keys=type, scheme, description
- `parameters/IdempotencyKey` → #/components/schemas/ID；keys=name, in, required, schema, description

## 操作状态码/媒体与错误枚举补充

所有操作的成功/错误状态码如下；响应引用沿前述图，错误code只取既有合法例/声明，不产生新错误约定。

| operationId | 状态码 → media（无正文记空） |
| --- | --- |
| getGlobalSettings | 200 → application/json; 400 → application/json; 401 → application/json; 403 → application/json; 429 → application/json; 503 → application/json |
| updateGlobalSettings | 200 → application/json; 400 → application/json; 401 → application/json; 403 → application/json; 429 → application/json; 503 → application/json; 413 → application/json; 422 → application/json; 409 → application/json |
| getHealthLive | 200 → text/plain; charset=utf-8 |
| getHealthReady | 200 → text/plain; charset=utf-8; 503 → text/plain; charset=utf-8 |
| getAdminStatus | 200 → application/json; 401 → application/json; 403 → application/json; 429 → application/json; 503 → application/json |
| connectClientWS | 101 → 空; 400 → application/json; 401 → application/json; 403 → application/json; 429 → application/json; 503 → application/json |
| login | 200 → application/json; 400 → application/json; 401 → application/json; 403 → application/json; 413 → application/json; 422 → application/json; 429 → application/json; 503 → application/json |
| listDeviceSessions | 200 → application/json; 401 → application/json; 403 → application/json; 429 → application/json; 503 → application/json; 400 → application/json |
| revokeDeviceSession | 204 → 空; 401 → application/json; 403 → application/json; 429 → application/json; 503 → application/json; 400 → application/json; 404 → application/json |
| getSession | 200 → application/json; 401 → application/json; 403 → application/json; 429 → application/json; 503 → application/json |
| logout | 204 → 空; 401 → application/json; 403 → application/json; 429 → application/json; 503 → application/json |
| listBots | 200 → application/json; 400 → application/json; 401 → application/json; 403 → application/json; 422 → application/json; 429 → application/json; 503 → application/json |
| createBot | 201 → application/json; 400 → application/json; 401 → application/json; 403 → application/json; 413 → application/json; 422 → application/json; 429 → application/json; 503 → application/json |
| patchBot | 200 → application/json; 400 → application/json; 401 → application/json; 403 → application/json; 404 → application/json; 409 → application/json; 413 → application/json; 422 → application/json; 429 → application/json; 503 → application/json |
| deleteBot | 204 → 空; 400 → application/json; 401 → application/json; 403 → application/json; 404 → application/json; 409 → application/json; 413 → application/json; 422 → application/json; 429 → application/json; 503 → application/json |
| resetAPIToken | 200 → application/json; 401 → application/json; 403 → application/json; 404 → application/json; 429 → application/json; 503 → application/json |
| resetWebhookKey | 200 → application/json; 401 → application/json; 403 → application/json; 404 → application/json; 429 → application/json; 503 → application/json |
| createBotMessage | 201 → application/json; 400 → application/json; 401 → application/json; 403 → application/json; 404 → application/json; 409 → application/json; 410 → application/json; 413 → application/json; 422 → application/json; 429 → application/json; 503 → application/json |
| createWebhookMessage | 201 → application/json; 400 → application/json; 401 → application/json; 403 → application/json; 404 → application/json; 409 → application/json; 410 → application/json; 413 → application/json; 422 → application/json; 429 → application/json; 503 → application/json |
| editBotMessage | 200 → application/json; 400 → application/json; 401 → application/json; 403 → application/json; 404 → application/json; 409 → application/json; 410 → application/json; 413 → application/json; 422 → application/json; 429 → application/json; 503 → application/json |
| createClientMessage | 201 → application/json; 400 → application/json; 401 → application/json; 403 → application/json; 404 → application/json; 409 → application/json; 410 → application/json; 413 → application/json; 422 → application/json; 429 → application/json; 503 → application/json |
| listClientMessages | 200 → application/json; 400 → application/json; 401 → application/json; 403 → application/json; 404 → application/json; 409 → application/json; 410 → application/json; 413 → application/json; 422 → application/json; 429 → application/json; 503 → application/json |
| createCallback | 201 → application/json; 400 → application/json; 401 → application/json; 403 → application/json; 404 → application/json; 409 → application/json; 410 → application/json; 413 → application/json; 422 → application/json; 429 → application/json; 503 → application/json |
| getClientMessage | 200 → application/json; 400 → application/json; 401 → application/json; 403 → application/json; 404 → application/json; 409 → application/json; 410 → application/json; 413 → application/json; 422 → application/json; 429 → application/json; 503 → application/json |
| uploadAttachment | 201 → application/json; 400 → application/json; 401 → application/json; 403 → application/json; 404 → application/json; 409 → application/json; 410 → application/json; 413 → application/json; 422 → application/json; 429 → application/json; 503 → application/json |
| downloadAttachment | 400 → application/json; 401 → application/json; 403 → application/json; 404 → application/json; 409 → application/json; 410 → application/json; 413 → application/json; 422 → application/json; 429 → application/json; 503 → application/json; 200 → application/octet-stream; 206 → application/octet-stream,multipart/byteranges; 304 → 空; 416 → application/json |
| claimEvents | 200 → application/json; 400 → application/json; 401 → application/json; 403 → application/json; 404 → application/json; 409 → application/json; 413 → application/json; 422 → application/json; 429 → application/json; 503 → application/json |
| ackDelivery | 200 → application/json; 400 → application/json; 401 → application/json; 403 → application/json; 404 → application/json; 409 → application/json; 413 → application/json; 422 → application/json; 429 → application/json; 503 → application/json |
| renewDelivery | 200 → application/json; 400 → application/json; 401 → application/json; 403 → application/json; 404 → application/json; 409 → application/json; 413 → application/json; 422 → application/json; 429 → application/json; 503 → application/json |
| listDeliveries | 200 → application/json; 400 → application/json; 401 → application/json; 403 → application/json; 422 → application/json; 429 → application/json; 503 → application/json |
| retryDelivery | 200 → application/json; 400 → application/json; 401 → application/json; 403 → application/json; 404 → application/json; 409 → application/json; 410 → application/json; 413 → application/json; 422 → application/json; 429 → application/json; 503 → application/json |
| bootstrapClient | 200 → application/json; 400 → application/json; 401 → application/json; 403 → application/json; 404 → application/json; 410 → application/json; 413 → application/json; 422 → application/json; 429 → application/json; 503 → application/json |
| syncClientChanges | 200 → application/json; 400 → application/json; 401 → application/json; 403 → application/json; 404 → application/json; 410 → application/json; 413 → application/json; 422 → application/json; 429 → application/json; 503 → application/json |
| updateClientRead | 200 → application/json; 400 → application/json; 401 → application/json; 403 → application/json; 404 → application/json; 410 → application/json; 413 → application/json; 422 → application/json; 429 → application/json; 503 → application/json |
| getCurrentPush | 200 → application/json; 400 → application/json; 401 → application/json; 403 → application/json; 429 → application/json; 503 → application/json |
| updateCurrentPush | 200 → application/json; 400 → application/json; 401 → application/json; 403 → application/json; 409 → application/json; 422 → application/json; 429 → application/json; 503 → application/json; 413 → application/json |
| deleteCurrentPush | 200 → application/json; 400 → application/json; 401 → application/json; 403 → application/json; 409 → application/json; 422 → application/json; 429 → application/json; 503 → application/json; 413 → application/json |
| getCurrentNotifications | 200 → application/json; 400 → application/json; 401 → application/json; 403 → application/json; 429 → application/json; 503 → application/json |
| updateCurrentNotifications | 200 → application/json; 400 → application/json; 401 → application/json; 403 → application/json; 429 → application/json; 503 → application/json; 404 → application/json; 413 → application/json; 422 → application/json; 409 → application/json |

| 合法错误code | 精确例 |
| --- | --- |
| ATTACHMENT_MISSING | E0149 |
| AUTHENTICATION_REQUIRED | E0318, E0578, E0657, E0959, E1070, E1079 |
| BOOTSTRAP_RESET_REQUIRED | E0473 |
| BOT_CAPABILITY_REQUIRED | E0320 |
| BOT_DISABLED | E0014, E0147, E0322, E0328 |
| BUTTON_STALE | E0141 |
| DELIVERY_NOT_FAILED | E0336 |
| FORBIDDEN | E0579, E0658, E0961, E1071, E1080 |
| GLOBAL_SETTINGS_CONFLICT | E1088 |
| GLOBAL_SETTINGS_VERSION_EXHAUSTED | E1095 |
| IDEMPOTENCY_CONFLICT | E0137, E0338 |
| INVALID_BOOTSTRAP_TOKEN | E0469 |
| INVALID_REQUEST | E1069, E1078 |
| INVALID_SYNC_CURSOR | E0433, E0471 |
| INVALID_WS_HANDSHAKE | E0577 |
| LEASE_CONFLICT | E0326, E0332 |
| LEASE_LIMIT_EXCEEDED | E0334 |
| PAYLOAD_TOO_LARGE | E0153 |
| PERMISSION_DENIED | E0342 |
| PROVIDER_NOT_READY | E0822 |
| RANGE_NOT_SATISFIABLE | E0151 |
| RATE_LIMITED | E0613, E0963 |
| READ_SEQ_OUT_OF_RANGE | E0435 |
| REGISTRATION_CONFLICT | E0734, E0737 |
| REGISTRATION_VERSION_EXHAUSTED | E0735, E0738 |
| REQUEST_TOO_LARGE | E1081 |
| RESOURCE_EXPIRED | E0143, E0340 |
| RESOURCE_NOT_FOUND | E0145, E0330, E0436, E0659, E0823 |
| REVISION_CONFLICT | E0139 |
| SERVICE_UNAVAILABLE | E0614 |
| SETTINGS_CONFLICT | E0793 |
| SETTINGS_VERSION_EXHAUSTED | E0824 |
| STORAGE_UNAVAILABLE | E0155, E0324, E0965 |
| SYNC_RESET_REQUIRED | E0434 |
| VALIDATION_FAILED | E1082 |
