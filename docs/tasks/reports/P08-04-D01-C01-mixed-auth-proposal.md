# P08-04-D01-C01 两条混合附件接口的鉴权顺序提案

状态：**待独立审查及用户批准，未冻结**。本提案只处理 `POST /api/v1/attachments`（uploadAttachment）和 `GET /api/v1/attachments/{id}`（downloadAttachment）。不是重新审批已批准的 G1、实例生命周期或公开 probe 成本；也不授权维护、部署、修改运行时或弱化其他端点的身份先行规则。

## 精确基线与冲突

依据固定提交 `8138b794e8023dd2eff4a1e03ec1e0a8c07f022c` 的 `server/api/openapi.yaml` 两操作及 securitySchemes，而非未提交 C01 草稿。两操作均是 `security: [{AdminBearer: []}, {BotAPIToken: []}]`，两 scheme 都使用 `Authorization: Bearer <value>`；无已冻结的显式分支 header、参数或独立路由。Web Cookie、Webhook Key 均无权；上传 Bot 须启用且有 send 能力，下载 Bot 须按原 send/receive 资源域授权。上传有 bot_id 参数；下载仅附件 ID，不能用上传专属参数充当统一鉴权判别。

值的现有实现并不相同：`server/internal/auth/session.go` 的会话值为 32 随机字节的无 padding base64url（43 字符）；`server/internal/bots/repository.go` 的 Bot API 值为两个独立 32 字节随机数的小写 hex，用点连接（64+1+64 字符），`bots/authorization.go` 识别后核对摘要、启用状态与代次。`0004_bots.sql` 已有 credential_id UNIQUE。**这些是基线实现事实，不是当前 OpenAPI 已冻结的分支协议**；按值语法粗分仍然需要在检查实例 ID 前读取 Bearer 值。此前“同形”特指 HTTP scheme 相同，不能误称两种 token 字符串格式完全一致。

已批准规则要求 Android 请求先检查期望 ID，再读取密码/Bearer；同时 Bot 分支沿用原规则且可不带实例头。纯 AdminBearer 路由可按凭据通道先选分支，但这两操作不能仅凭相同的 Bearer scheme 决定是否允许无 ID。按 header 有无选分支也无法区分旧 Android 与正常 Bot；先对所有 Bearer 强制实例头则破坏 Bot 兼容。G1 接受的是 probe 后瞬切的字节窗口，**不自动批准此处 ID 前读取 Bearer 的服务端顺序例外**。

## 推荐候选：仅两操作允许 Bot-first 狭窄例外

明确批准后，才把下述顺序写入正式设计、OpenAPI 与校验器；其他端点一律不加例外。

1. 先执行原传输和凭据通道格式检查。拒绝多 Authorization、Cookie/Bearer 混用、非 Bearer/空值；不读附件流、不处理 Range/条件请求、不查业务资源、不执行管理员会话认证。错误保持既有通用错误外壳和 no-store，不回显值。
2. **允许在实例 ID 检查之前读取 Bearer 值，仅用于 Bot API 识别/认证。** 可用现有 Bot 值语法作快速否定，不能仅凭字符串形状授予 Bot 身份。Bot 候选以 credential_id 查询现有 API 凭据并常量时间核对摘要，获得 Bot ID/代次；下载应先按现有唯一 credential_id 定位身份，再查附件授权，不能先读目标附件取得 Bot ID或暴露其存在。此内部适配不引入第二种 Token、表/索引、在线接口或跨 Bot 权限。
3. Bot 认证成功即固定 Bot 分支，忽略实例头，按原启用状态、send/receive 能力、bot_id/资源域、幂等、下载范围和提交前凭据代次规则执行。已确认 Bot 身份但停用/能力不足/资源不授权，返回原 403/404/相关错误，**不得降级重试 Admin**。实例头即使缺失、畸形或不匹配也不使有效 Bot 进入 Android 分支；实例头不授予任何权限。
4. 仅明确的“不是有效 Bot 凭据”（非 Bot 候选形状、credential_id 不存在、摘要不匹配）进入 Android 路径。此时先检查实例头：缺失 400 INSTANCE_REQUIRED；畸形 400 INVALID_REQUEST；身份读失败 503 INSTANCE_UNAVAILABLE；ID 不匹配 409 INSTANCE_MISMATCH。通过后才执行 AdminBearer 会话认证与一致事务 ID/auth_generation 检查。管理员 token 不再用于 Bot 以外的任何 ID 前认证动作，不延长会话，不解析为管理员身份，不发布认证结果。
5. Bot 凭据读取出现存储错误/超时，**返回 503，不当作“不匹配”且不回退 Admin**；推荐两操作在此复用通用 `INSTANCE_UNAVAILABLE`、固定“实例暂不可用”、retryable=true，不含实际 ID或底层错误。身份读取错误同样 503。认证或业务写事务的存储/提交失败仍整体回滚；只读下载先取得授权引用再打开文件，原保护不变。

| 情形 | 候选结果与先后 |
| --- | --- |
| 有效 Bot，无实例头或错误实例头 | Bot 认证→原 Bot 能力/资源授权；不做管理员身份门禁 |
| 已匹配 Bot，但停用/越权 | 原 Bot 拒绝；终止，不尝试 Admin |
| Bot lookup 无行/摘要不匹配，且无实例头 | 转 Android→400 INSTANCE_REQUIRED；不执行 Admin 会话认证 |
| 非 Bot 候选，格式正确的旧 Android，无实例头 | 读取值作 Bot 否定→400 INSTANCE_REQUIRED；不执行 Admin 会话认证 |
| Bot 不匹配，实例头与当前 ID 不同 | 409 INSTANCE_MISMATCH 优先于 Admin 401；不回显实际 ID |
| Bot 不匹配，实例头正确、Admin 无效 | 通过身份门禁后沿用 401 |
| Bot lookup 存储故障（即使实例头缺失/错误） | 503 优先，不尝试身份读取或 Admin；不能把 DB 故障误当无 Bot |
| Bot 不匹配，身份读取失败 | 503，不能误记 401 或清除正常客户端会话 |
| Admin 认证成功后事务 ID/代次变化 | 原已批准二次绑定：ID 错配409、代次失效401、DB失败503；不提交/发布部分结果 |

可观察兼容变化必须一并批准：**无效/过期 Bot 值若不再匹配有效凭据且缺实例头，可能从旧认证失败 401 变为 400 INSTANCE_REQUIRED**；错误实例头可得到400/409，Bot lookup 存储故障可先得503。不能把“有效 Bot 业务能力不变”写成“所有 Bot 错误优先级完全不变”。若要求无效 Bot 也固定沿旧401，则还需批准稳定的显式分支协议（下节），不能私下用未冻结 token 语法掩盖这一变化。

ID 前 Bot 识别本身不能保证凭据未到达错误部署；不允许把此例外叙述成防泄漏方案。日志仅固定路由/request_id/状态/耗时/错误码，禁 header、token、credential_id、实例 ID、附件路径和正文；错误文本不区分“ID不存在”与“摘要不符”、不回显实际 ID/账号/原始 DB 错误。不同分支的400/401/403/409仍是可观察状态；不承诺消除所有身份/时序侧信道。所有响应 no-store，Android 客户端逐次无凭据 probe/失败关闭仍强制执行。

## 对比候选：显式分支标识或独立路由

| 候选 | 身份先行和代价 |
| --- | --- |
| 推荐 Bot-first | 无新 wire 标识、有效旧 Bot 无需改请求；仅两操作批准 ID 前 Bot 凭据读取/认证及上述失败优先级变化。需要两分支事务、存储故障、无效 Bot 错误和旧 Android 缺头回归。 |
| 显式分支标识 | 冻结如 X-RelayNest-Auth-Mode: android/bot（这里只是候选名，未授权使用）。先读非秘密标识，Android 严格先ID再读Bearer，Bot按原认证。必须定义缺标识的旧客户端行为；若强制所有客户端发送，旧Bot也要升级并产生阻断窗口；若缺标识默认Bot，则无标识旧Android只能得Bot401，不能再保证统一缺ID400。伪称bot的Admin token只能失败，不能授予Admin权限；此选择也需新批准，不能称完全兼容。 |
| 独立路由 | 将两种凭据拆到明确Android/Bot端点，可在读Bearer前由路由选分支；需新增公开路径/API契约/调用端修改并决定旧路径保留/废弃/错误，增加实现与兼容测试。保留旧混合路由不解决旧路由的原冲突；透明重定向亦不可接受。 |

不推荐对所有附件 Bearer 强制实例头（改变已批准 Bot 边界），也不推荐只靠实例头存在与否自动赋予认证类型。若选显式协议，应先单独冻结缺标识/旧路由处理，不能留到运行时作者自决。

## 审查、批准与后续验收

独审以本报告固定 SHA 和基线 API为输入，不依赖未提交草稿证明结论。需核查：两操作现认证类型/参数；Bot/Admin值格式的实现事实与非冻结性质；推荐例外是否精确局限两操作；无效Bot及存储故障优先级是否完整公开；显式标识/路由替代成本是否真实。此轮只提交提案与checkpoint，未改机器契约为已批准状态。

用户后续需要选择是否接受推荐的 **两操作 Bot-first 顺序例外、无效Bot错误优先级变化及503候选映射**，或选择先冻结显式分支协议。获准后继续现 C01 草稿，无需重做已完成 probe/Session/生命周期工作，但必须使正式设计与机器矩阵显式表达获准混合顺序，补匹配Bot/停用/越权/无效Bot/缺头/错ID/存储故障/事务变化的正反例，再跑全量锁定契约检查，最终 SHA 另人 A01/I01。当前2190例仅合成分支模型，预设认证类型，**没有解决 HTTP Bearer 的分支识别问题**，不能作为新 G-INSTANCE-CONTRACT 或运行时 gate通过。

本轮未执行维护、部署、Go/Web/Android/SQLite运行时/真机测试；不修改 RelayShelf、Bot Hub 业务边界、数据库或registry。
