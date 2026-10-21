# RelayNest API 契约

`openapi.yaml` 是 OpenAPI 3.1.0 契约，统一前缀 `/api/v1`。本批仅定义认证、Bot 管理、两套凭据及通用 Schema；不表示 Go 服务已实现这些路由。消息/Webhook 载荷、投递、设备、设置与同步由 P03-05～07 串行补充，不在本卡建立占位业务接口。

依据：`docs/TECHNICAL_DESIGN.md` §2.1、§3、§7；安全边界遵循 `docs/ARCHITECTURE.md` §1/§5。唯一管理员离线初始化/重置密码，不开放注册，不依赖 RelayShelf。

## 校验

仓库根目录运行（Python 3.13；推荐使用项目已锁工具环境，不必为本卡安装 Gradle）：

```sh
python3 -m venv /tmp/relaynest-api-venv
/tmp/relaynest-api-venv/bin/python -m pip install --require-hashes -r server/api/requirements.txt
/tmp/relaynest-api-venv/bin/python server/api/validate.py
```

`requirements.in` 是直接校验依赖，`requirements.txt` 固定全部传递版本和发行包哈希；它们不属于服务运行时依赖。更新时用 `uv pip compile server/api/requirements.in --generate-hashes --output-file server/api/requirements.txt --python 3.13` 后重新执行校验。`uv pip sync --require-hashes` 也可安装锁文件。

校验器先检查 OpenAPI 结构及全部 Schema，再以 JSON Schema Draft 2020-12 和显式 `FormatChecker` 检查 `examples.json` 的合法/非法示例；最后检查当前管理路由的 Cookie/CSRF/Bearer 声明和响应 no-store。示例包含精度超过 JavaScript 安全整数范围的序号、SQLite 上限、未知字段、删除确认、版本、凭据隔离和内容字段泄漏；全部是合成数据。失败退出非零。

这些是契约校验，不替代真实服务的鉴权、事务、CSRF、即时撤销与响应脱敏测试。后续实现不能用 Schema 通过代替运行时安全验证。

## 身份及权限

| 身份 | 允许范围 | 约束 |
| --- | --- | --- |
| Web Cookie 管理员 | 本卡所有管理接口 | 写操作同时验证会话 CSRF 与同源 Origin/Referer；登录无会话，验证 Origin；不浏览正文、附件、历史或聊天 WS |
| Android 管理员 Bearer | 本卡所有管理接口；后续 client 接口 | 每安装设备 ID、每设备独立会话；登录才返回 Bearer，session 接口不再次返回 |
| Bot API Token / send | 本 Bot 消息创建/编辑、上传授权附件 | 不能访问管理员接口、其他 Bot 或作为 Webhook Key；不提供 MCP/业务执行 |
| Bot API Token / receive | 本 Bot claim、ACK、renew 和相应投递附件 | 不授予创建/编辑、上传或枚举历史附件 |
| Webhook Key | 仅本 Bot `POST /webhooks/{bot_id}` 创建标准消息 | 无读/编辑/上传/领取/管理权限；可引用本 Bot 已准备好且允许发送的附件；密钥只放 Authorization |

OpenAPI 的三个 Bearer scheme 用于区分身份，`security` 不能表达非 OAuth Bearer 的 send/receive scope，也不能仅凭 Bearer 格式识别用途。运行时须验证独立凭据类型、Bot ID、能力、启用状态和代次。Webhook 接口和 Bot 消息接口将在后续卡补入，并分别使用 `WebhookKey`/`BotAPIToken`，不能复用管理员 scheme。

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
