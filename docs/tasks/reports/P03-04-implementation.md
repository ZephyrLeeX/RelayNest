# P03-04 认证与 Bot OpenAPI 契约实施报告

- 角色：developer；结果：implemented，待独立审计/集成，不标总任务 done。
- 基线：`f3dbe794f8afe32573435a94e363eb7a3761924f`。
- 工作树：`/tmp/relaynest-run2/api`；分支：`codex/api-run2`；开始时工作区干净，HEAD 精确匹配基线。
- 实现 SHA 提交后交接，不在本报告预写自身 SHA。

## 变更

创建 `server/api/openapi.yaml`：OpenAPI 3.1.0，9 个认证/Bot/独立凭据重置操作、28 个 Schema。通用错误/分页/UUID v4/UTC 时间/SQLite 64 位序号字符串；管理员 Web Cookie+CSRF 与 Android Bearer；Bot 创建、元数据列表、修改、显式名称+版本删除；API Token 与 Webhook Key 仅创建/对应重置返回明文。

创建 `server/api/validate.py`、`examples.json`、`README.md`、`requirements.in`、`requirements.txt`：OpenAPI 结构和 Schema 校验、正反例、权限声明检查及复现说明。锁 3 个直接校验依赖和全部 18 个传递包的版本/发行包 SHA-256；未改 `go.mod`，无大型框架或服务运行时依赖。

依据局部 PRD §3.1/3.5、ARCHITECTURE §1/§5、TECHNICAL_DESIGN §2.1/§3/§7；前置 P03-01 集成报告已读取。Webhook 仅本 Bot 标准消息创建，API send/receive 能力独立，不能访问管理员接口或互换用途。两套重置互不影响；API 重置撤销旧租约但保留消息/失败记录。Web 管理响应严格限定元数据，无正文/附件/历史/callback_data/原始异常。Hub 不解释业务或执行 MCP。

本卡未扩展消息/租约/同步/设备/设置路由，留 P03-05～07；不实现认证服务、持久化、迁移或业务路由。Cookie/header/字段等局部命名均在 README 交接，没有增加产品或权限限制。分页大小上限留运行时按既定资源控制有界处理。

## 实际验证

环境：Linux x86_64；Python 3.13.15（mise 已安装解释器，绝对路径调用）；uv 0.12.13；独立临时 venv，不修改仓库环境。

| 命令 | 结果 |
| --- | --- |
| `/home/lijx/.local/share/mise/installs/uv/0.12.13/uv-x86_64-unknown-linux-gnu/uv venv --python /home/lijx/.local/share/mise/installs/python/3.13.15/bin/python /tmp/relaynest-p0304-locked` | PASS，创建干净校验环境 |
| `/home/lijx/.local/share/mise/installs/uv/0.12.13/uv-x86_64-unknown-linux-gnu/uv pip sync --python /tmp/relaynest-p0304-locked/bin/python --require-hashes server/api/requirements.txt` | PASS，18 包从固定版本/哈希安装 |
| `/tmp/relaynest-p0304-locked/bin/python -m openapi_spec_validator server/api/openapi.yaml` | PASS，`server/api/openapi.yaml: OK` |
| `/tmp/relaynest-p0304-locked/bin/python server/api/validate.py` | PASS，`28 schemas; 20 valid + 41 invalid examples; 9 operations/security` |
| `/tmp/relaynest-p0304-locked/bin/python -m py_compile server/api/validate.py` | PASS；产生的临时 pycache 已删除 |
| `git diff --check` | PASS，无空白错误 |

额外临时范围实验：加载 Sequence Schema，用 `random.Random(304)` 生成 10,000 个 `[0,2**64)` 整数，与 9 个边界（0、1、JS 安全整数附近、`2**63-1` 附近及 `2**64-1`）一起校验，逐个断言 `is_valid(str(n)) == (n <= 2**63-1)`；实际输出 `PASS: 10009 deterministic sequence boundary/range samples`。固定反例已保留在 examples.json，包括结尾换行、前导零、浮点形式、溢出及 JSON 数字。

首次尝试 `mise exec -- uv ...` 因仓库未安装 Gradle 9.8.2 而触发工具安装，GitHub rate limit 403 导致命令失败。随后直接调用已有 Python/uv 完成上述全部本卡检查，无仓库修改；没有宣称 Gradle/Android 检查通过。

## 验收与限制

| 条目 | 结论/证据 |
| --- | --- |
| 合法/非法示例实际校验 | PASS，20 合法/41 非法；完整输入在 examples.json |
| Webhook 与 API 权限不同 | PASS（契约层）：独立 security scheme、独立凭据响应 Schema、README 权限矩阵；管理员路由拒绝声明 Bot/Webhook scheme，Web 写操作要求 CSRF；反例拒绝 Webhook receive 字段和 API/Webhook 响应互换 |
| 无新产品规则 | PASS（实现范围自查），无新功能/接口域/权限规则；独立审计待执行 |
| 受影响检查 | PASS，上表；Go 未改，未重复运行其健康入口测试 |
| 仅本卡修改 | 提交前核查仅 server/api 下 6 个文件及本报告，无 registry/其他卡/迁移/后端改动 |

未执行：真实服务 HTTP 认证、密码摘要、CSRF/Origin、凭据熵/摘要存储、即时撤销与并发事务、浏览器 Cookie、Android 真机、RelayShelf、部署。当前 Go 仅健康入口，以上均由后续实现卡验证；不能将 Schema/声明校验视为运行时访问隔离通过。无发布/推送，无未决需求决策。

后续接口：统一 `/api/v1`；新增消息/投递/同步时复用本通用 Schema；Webhook 使用 `WebhookKey`、Bot 消息使用 `BotAPIToken`，非 OAuth Bearer 的能力须在运行时根据用途/范围/代次校验，不能依赖 OpenAPI scheme 名自动实现。Web 仅管理接口；需要变更冻结字段或权限时走确认流程。
