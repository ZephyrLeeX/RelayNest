# P03-07-S03-S02 实施报告

## 结论与范围

本卡契约实现完成，待不同 agent 对最终提交独立审计及精确集成；不宣布 G-API 放行。基线 `2fc964dc2c967f6ac53b1d2512d0e0d3ff2734e7`，独占工作树 `/home/lijx/.codex/worktrees/api-health-contract/RelayNest`，分支 `codex/api-health-contract`。实现 SHA 由提交后交接给调度记录，本报告不预写自身 SHA。

前置：设备父完整集成 `42cd918c01c34bb7baa918420dad0daa2b4d6a98`；ORDER01 实施 `29240002bd291620ba14ab84efa6b40e4e68ec26`、独立审计 `e3cf38bd337b2bf6bdd389fc7865b5221247a655` 已在基线。按 ORDER01 健康先行，无 settings 新 Schema 前置，API 独占串行。settings 政策在另一树只写文档，不冻其字段。

恢复执行重新检查根 AGENTS、CONTRIBUTING、WORKER_PROTOCOL、子卡、T8/PRD健康相关片段及健康处理器/路由/测试局部；此前草稿列出的全部设计阅读过程未作为本次证据采信。未整灌 API 或全部旧例。

预算偏差：本次首次两批工具返回约13.8k tokens，超过初始读取≤12k要求，原因是将协议全文、校验器差异和旧报告合读且输出截断。此后改为定向片段与脚本实测；不声称预算合规，也未借此扩展写入范围。

只修改 `server/api/{openapi.yaml,examples.json,validate.py,README.md}` 与本报告。两锁、源码、迁移、PRD/设计、卡片、旧报告、registry不改；不派 agent、不 push/部署。

## 四条验收映射及移交

| 子卡验收 | 本次交付与边界 |
| --- | --- |
| 1 根公开健康 | `/health/live#get`→getHealthLive、`/health/ready#get`→getHealthReady；操作 servers `/` 覆盖全局 `/api/v1`，security为空。HealthOK/HealthUnavailable 为精确 `ok\n`/`unavailable\n` 字符串；live200，ready200/503；媒体 `text/plain; charset=utf-8`、no-store/nosniff，503不引用 JSON Error。 |
| 2 管理员投影 | `/admin/status#get`→getAdminStatus，实际 `/api/v1/admin/status`；AdminBearer/WebCookie，Bot/Webhook/匿名拒绝，GET不需CSRF；200 AdminSystemStatus，401/403/429/503共享JSON Error及no-store。严格汇总允许列表，无自由错误串或正文/原始异常/密钥/路径。 |
| 3 含义及正反例 | Live仅进程存活；Ready为数据库/挂载Probe，缺失/失败/超时/停止503。FCM失败仅degraded不决定ready。WS有效管理员会话连接、应用近期轮询/未知分别统计；Webhook固定unknown，不宣称持续在线。新增88例覆盖Schema/错误/权限/纯文本信封/探针状态；不是运行时扩展。 |
| 4 移交 | P06-08实现真实Probe、汇总、分类错误映射和HTTP鉴权/脱敏/故障注入；P4/P5据此冻结路径、权限、Schema、503分离；P8/P9验证部署与手机/NAS现场。最终S03-S03同时汇合settings与本卡；settings从本卡实际集成后最新main派发，G-API未放行。 |

新 Schema keys（9个）：HealthOK、HealthUnavailable、StatusCount、QueueStatus、ConnectionStatus、AttachmentRecoveryStatus、DatabaseStatus、PushStatus、AdminSystemStatus。

- AdminSystemStatus：observed_at、queue、connections、attachment_recovery、database、push；时间复用 UTCDateTime，未加 Version/settings 字段。
- QueueStatus：pending、leased、retry_wait、failed、oldest_waiting_at。pending/retry_wait 为0时最旧时间必须null，否则必须UTC时间；取这两类最早入队时刻，含停用Bot，不含leased等待。
- ConnectionStatus：ws_online_sessions、recent_poll_bots、unknown_bots、webhook_state（固定unknown）。沿用旧 Bot.connection，不新增近期阈值；Webhook-only归unknown。
- AttachmentRecoveryStatus：anomaly_count、last_error_code（null或ATTACHMENT_RECOVERY_FAILED/ATTACHMENT_STORAGE_UNAVAILABLE/ATTACHMENT_OTHER）。DatabaseStatus：state（available/unavailable/unknown）、last_error_code（null或DB_UNAVAILABLE/DB_BUSY/DB_IO_ERROR/DB_OTHER）。PushStatus：state（available/degraded/unknown）、last_error_code（null或PUSH_UNAVAILABLE/PUSH_REJECTED/PUSH_OTHER）。分类命名是局部协议实现细节，last为最近可展示分类，可为历史记录，不等于当前状态；unknown表示尚无观测。
- 所有对象拒绝未知字段；StatusCount上限9007199254740991为JSON精确表示边界，非产品容量或丢弃规则。计数无法精确表示时不能伪造成功值；未来实现按管理503处理读取失败。

现状证据：router已注册根健康；Live/Ready/respond已有固定正文、no-store/nosniff，Ready上下文一秒，空Probe/停止/失败/超时不ready。Probe注释要求组合数据库及挂载，但本卡未检查其真实接入或运行服务。新增admin/status路由、汇总和错误分类仅契约，不能宣称已运行。

## 验证与非回归

旧 `/tmp/relaynest-p0305-f01-locked` 环境与探针脚本已消失，本次恢复环境后重新运行全部下列检查，未采信旧通过记录。解释器 `/tmp/relaynest-health-contract-locked/bin/python`，Python 3.13.15，源解释器 `/home/lijx/.local/share/mise/installs/python/3.13.15/bin/python3.13`。使用现有 uv 0.12.13 执行（未升级任何依赖）：

```sh
/home/lijx/.local/share/mise/installs/uv/0.12.13/uv-x86_64-unknown-linux-gnu/uv venv /tmp/relaynest-health-contract-locked --python /home/lijx/.local/share/mise/installs/python/3.13.15/bin/python3.13
/home/lijx/.local/share/mise/installs/uv/0.12.13/uv-x86_64-unknown-linux-gnu/uv pip sync --python /tmp/relaynest-health-contract-locked/bin/python --require-hashes server/api/requirements.txt
/tmp/relaynest-health-contract-locked/bin/python -m openapi_spec_validator server/api/openapi.yaml
/tmp/relaynest-health-contract-locked/bin/python server/api/validate.py
/tmp/relaynest-health-contract-locked/bin/python /tmp/relaynest-health-checks.py
git diff --check
```

安装结果18包，逐条用 importlib.metadata 比较 requirements.txt 全部18锁定版本一致；两锁哈希及字节均与基线相同。uv因跨文件系统采用copy替代hardlink，不影响哈希安装结果。

结果：OpenAPI OK；`PASS: OpenAPI 3.1; 109 schemas; 470 valid + 517 invalid examples; 37 operations/security`；diff检查通过。基线34操作、100 Schema、899例；仅新增3操作、9 Schema、88例。使用 `git show <基线>:server/api/<文件>` 与当前文件解析比较：全部旧path操作及全部旧components类别/子项逐项相等，当前examples前899项与基线逐项相等，两锁字节相等；恢复重建 `/tmp/relaynest-health-checks.py` 实测通过：34旧操作、100旧Schema、全部旧components子项、899旧例（450正/449反）、两锁字节、18安装版本。另从基线git show导出3文件到隔离目录，用同一解释器运行旧validate.py，实测 `PASS: OpenAPI 3.1; 100 schemas; 450 valid + 449 invalid examples; 34 operations/security`。

授权差异例外仅：新增上述3条路径/操作、9个Schema和88例；README健康交接段；校验器新增健康声明/合成状态/信封检查与按media_type选Schema。已有全部操作/组件/示例和两锁冻结。校验器仅在getHealthLive/getHealthReady且空security的声明例允许匿名{}；login及旧操作不新增此匿名声明例外。恢复时将原草稿泛化的空security判断收紧为这两个固定ID；继续核验全部旧管理权限，根路径/servers/响应/新Schema还做精确声明对照。响应夹具按明确media_type选择所声明的Schema，默认仍application/json；HTTP101及WS frame原逻辑不变。`health_response`仅合成契约信封，headers只放两项规范头，不声称覆盖真实HTTP自动头。`health_state`仅合成Probe/停止/FCM输入声明，不能证明DB/挂载真实状态。

隔离破坏：在临时目录复制当前三份yaml/json/validate，单项变更声明，再用相同锁解释器运行临时validate.py；正式工作树从未写入坏声明。15项最终均非零且明确拒绝；不把脚本构造异常当作PASS。检查脚本位于临时路径，不是运行时或仓库新依赖。可按下表逐项修改副本重现：

| 破坏项 | 实际拒绝消息 |
| --- | --- |
| root-server-removed | 健康/管理操作声明错误：/health/live |
| root-prefixed | 健康/管理操作声明错误：/health/live |
| health-protected | 健康/管理操作声明错误：/health/ready |
| admin-public | 健康/管理操作声明错误：/admin/status |
| admin-bot-auth | 健康/管理操作声明错误：/admin/status |
| old-admin-public | 权限声明错误：get /admin/bots |
| health-json-error | 健康/管理操作声明错误：/health/ready |
| health-wrong-media | 健康/管理操作声明错误：/health/live |
| health-cache | 健康/管理操作声明错误：/health/live |
| health-nosniff-missing | 健康/管理操作声明错误：/health/ready |
| fcm-unready | 健康/管理操作声明错误：/health/ready |
| admin-free-text | 健康/管理Schema错误：AdminSystemStatus |
| webhook-online | 健康/管理Schema错误：ConnectionStatus |
| health-newline | 健康/管理Schema错误：HealthOK |
| json503-text | 健康/管理操作声明错误：/admin/status |

本次隔离探针使用标准库复制/JSON序列化（JSON为YAML合法子集、ensure_ascii保留旧非法代理字符），不改正式文件。每项既要求非零退出，也要求健康/Schema/权限明确拒绝且无Traceback。15项均通过；脚本位于临时目录，可按上表逐项重建，不将工具内部异常计为通过。

## 基线六文件 SHA-256

| 文件（server/api/） | SHA-256 |
| --- | --- |
| openapi.yaml | `104e8e7f823dcc2f10cd222bd3498a633ef057b564f0cb1b10cd0e2097effe1b` |
| examples.json | `1207d8373e20fbb4b80470575fe078206e01050e8d87d2bbbb0fc1914099fe39` |
| validate.py | `7b5e166980275d9b2dde806052d6a5cfa466340a6815101b7ac06890073eb7d9` |
| README.md | `512c37d2b2d51da8745de3f8ff40ae5f6f2dd4b5ee77ed508aabfe8dfa196e7f` |
| requirements.in | `4d56f1903c85ca444dde0966a7fd2702f15242b2cbc5f35eb1fcd273a0317d7c` |
| requirements.txt | `4e1992c8e79a37d1671671afd5e24f686837f719078ac7bdab06b5919231f184` |

## 未执行项

NOT_RUN：Go服务运行时/真实HTTP与部署、实际数据库/挂载故障注入、汇总事务/并发/权限即时撤销、真实WS/轮询及FCM、SDK/手机/OPPO/NAS、签名和发布。本卡没有修改运行时，不运行其模块测试替代这些验收。不得据契约PASS释放G-API/G-OPPO或批准P4业务实现；P1/P2与D1～D3既有用户决定不重开。

独立审计应锁定本卡最终实际SHA和精确五文件差异；通过后再精确集成及由调度更新registry。工作树提交后保持干净并释放共同API写入窗口。
