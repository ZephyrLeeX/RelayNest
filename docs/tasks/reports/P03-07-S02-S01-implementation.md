# P03-07-S02-S01 设备会话契约实施报告

- 角色：developer；结果：implemented，等待不同 agent 固定 SHA 独立审计与精确集成；不是父卡完成或阶段放行。
- 精确基线：`5cdaf15f7e01b663dd0f2b68fc044d9e875aaa79`。
- 工作树：`/home/lijx/.codex/worktrees/api-device-session/RelayNest`；分支 `codex/api-device-session`，起始工作区干净。
- 仅修改四份 API 文件和本报告；实现 SHA 由提交后交接记录，不在提交中预写自身 SHA。两 requirements 锁只读。

## 变更与冻结接口

新增 `GET /devices`（`listDeviceSessions`）和 `DELETE /devices/{id}/session`（`revokeDeviceSession`），路径相对 `/api/v1`。读取为管理员 Android Bearer 或 Web Cookie；撤销为 Bearer 或 Cookie + CSRF + 同源 Origin/Referer。Bot API Token / Webhook Key 均无权。读取返回200 `DeviceSessionPage`；撤销204无JSON正文、无请求体；公共错误均复用 `Error` 和 no-store。读取状态为200/400/401/403/429/503，撤销为204/400/401/403/404/429/503。401 `AUTHENTICATION_REQUIRED`、403 `FORBIDDEN`，未知/他人/已无有效会话的目标统一404 `RESOURCE_NOT_FOUND`，避免披露归属；格式、限流、暂不可用沿原约定。

唯一新增 Schema `DeviceSessionPage` 的 keys 为 `items: Session[]`、`next_cursor`、`has_more`；复用 `cursor`/`limit` 有界分页，复用旧 `Session` 全部元数据，不改旧 Schema。列表只含当前管理员有效设备会话，不返回凭据、CSRF、密码摘要、推送token；时间复用服务端 UTC `idle_expires_at` / `absolute_expires_at`。列表分页不承诺跨页快照，撤销时重新校验。

路径 `{id}` 为 `Session.device_id`；`Session.id` 是会话身份/代次，每次登录新建且不复用，设备id可以保持。撤销事务定位目标当前有效会话，使整个会话所有provider映射、WS失效并阻止新发送；旧提示不展示缓存正文。旧回调及迟到结果在提交前校验会话身份/有效性，不得复活已撤销映射、覆盖或解绑新代次。Cookie撤销自身时清Cookie，撤销其他设备不清调用方Cookie。原 `logout` 操作完全保持基线：只注销当前会话及其通道/WS，不影响其他设备与Bot凭据。DELETE不承诺跨重新登录重试幂等，重试前可重读列表。新不变量以两操作 `x-device-session-policy` 明示，未创建provider注册字段/Schema、通知资源或占位接口，未修改sync/bootstrap/WS。

## 验收映射

| 子卡验收 | 结果与证据 | 父卡对应 |
| --- | --- | --- |
| 1 路径/参数/权限/状态/Schema | PASS（契约）：两路径、`DeviceSessionPage`、`ID`路径参数与既有分页/安全/公共错误；校验器检查操作集合、状态、无请求体/204无正文、权限及no-store | 1 |
| 2 脱敏、全会话撤销、logout范围 | PASS（契约）：复用Session、7类敏感/未知字段反例，`x-device-session-policy`全通道/WS声明、logout合成范围例；旧logout逐项等值非回归 | 1、7会话部分 |
| 3 撤销/迟到回调/重登录隔离 | PASS（声明）：Session.id每次登录新建；撤销后拒绝回调、旧新id不匹配拒绝，新代次可用；拒绝错误复活/跨代次/漏撤通道/logout波及其他设备的反例 | 1、7会话部分 |
| 4 正反夹具、错误/时间/no-store | PASS（结构与声明）：新增39例，含Bearer/Cookie许可，Cookie缺CSRF/跨源，Bot越权、未知/他人设备404、失效会话401、列表凭据/非UTC时间拒绝及错误响应 | 7会话部分 |
| 5 清单及交接 | PASS（文档）：本报告和README列两操作、1Schema、权限、状态、keys、服务端时间和范围；真实运行时限制如下 | 8本卡部分 |

## 实际检查

固定解释器 `/tmp/relaynest-p0305-f01-locked/bin/python`，`--version` 实际为 **Python 3.13.15**，使用已有锁环境；没有安装或升级依赖。

| 命令/检查 | 结果 |
| --- | --- |
| `/tmp/relaynest-p0305-f01-locked/bin/python -m openapi_spec_validator server/api/openapi.yaml` | PASS：`server/api/openapi.yaml: OK` |
| `/tmp/relaynest-p0305-f01-locked/bin/python server/api/validate.py` | PASS：OpenAPI 3.1；84 schemas；320 valid + 356 invalid = 676 examples；29 operations/security |
| `/tmp/relaynest-p0305-f01-locked/bin/python /tmp/device_session_checks.py` | PASS：基线全部27操作、83Schema及其他全部components、637例逐项等值；两锁字节等值；8个破坏探针均拒绝（非零退出） |
| `git diff --check`、提交前diff/status | PASS；仅本任务允许文件 |
| 可选 `python -m pip check`（同一锁解释器） | 未执行依赖一致性检查：环境未安装pip，命令报 `No module named pip`；未补装。必需OpenAPI/全部契约校验不受影响 |

非回归脚本从 `git show 5cdaf15:server/api/{openapi.yaml,examples.json,requirements.in,requirements.txt}` 取得基线，逐项比较旧paths操作与所有components值，断言新例子前637项等于原数组，两锁比较原始字节。新操作/Schema/例子分别增量2/1/39。锁 SHA-256：

- requirements.in：`4d56f1903c85ca444dde0966a7fd2702f15242b2cbc5f35eb1fcd273a0317d7c`
- requirements.txt：`4e1992c8e79a37d1671671afd5e24f686837f719078ac7bdab06b5919231f184`

临时探针脚本仅位于 `/tmp`，每次将当前validate/examples和变异的OpenAPI复制到独立临时目录运行验证，断言返回码非零，不污染源码：删Cookie写CSRF；将列表no-store改public；删撤销404；将登录代次改为复用；将撤销范围改为仅FCM；允许Session返回token；将目标ID参数退为任意字符串；将“禁止跨代次写入”错误状态反例改成应接受。最后一项仅改独立副本examples预期，验证状态拒绝检查确实执行。8项全部拒绝；标准OpenAPI结构通过不代替这些语义声明检查。

## 遗留与后卡摘要（不超过800字）

新路径为GET `/devices`及DELETE `/devices/{id}/session`，管理员Bearer/WebCookie可读，Cookie写需CSRF同源；GET200 `DeviceSessionPage`（items:Session[]/next_cursor/has_more），DELETE204无正文。公共400/401/403/429/503，撤销另404；401 AUTHENTICATION_REQUIRED、403 FORBIDDEN、404 RESOURCE_NOT_FOUND（未知/他人统一）；全部no-store。时间复用服务端UTC到期时间。路径id=Session.device_id；Session.id=每次登录新建不可复用会话代次。撤销整个目标会话全部provider与WS，迟到回调/结果提交校验身份有效性，不能复活或改写重登代次；logout仅当前会话、其他设备/Bot不变。注册输入不得指定其他设备/会话；注册/通知资源尚未定义，后两卡承担。旧27/83/637及两锁等值；新29/84/676、锁Python3.13.15验证和8探针PASS。运行时撤销、真实WS断连、多worker、SDK、手机/OPPO/NAS均NOT_RUN；保留原阶段验收决定，不把合成状态当真机结果。无新增待决权限/需求矛盾；需不同agent审计PASS并集成后才派下一卡。
