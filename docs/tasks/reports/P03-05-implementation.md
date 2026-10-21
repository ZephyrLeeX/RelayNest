# P03-05 消息与附件 OpenAPI 契约实施报告

- 角色：developer；结果：implemented，待独立审计及集成，不标父任务 done。
- 基线：`30efad4f16234c07f42a2dd6e746d9d980b7dadf`。
- 工作树：`/tmp/relaynest-run2/api-next`；分支：`codex/api-next-run2`。开始时 HEAD 精确匹配基线且干净。
- 实现 SHA 在提交后交接，本报告不预写自身 SHA。

## 变更与接口冻结

仅修改 `server/api/openapi.yaml`、`validate.py`、`examples.json`、`README.md` 及本报告；无迁移、Go/Android/Web 服务代码、锁依赖、其他卡或 registry 修改。材料按卡局部读取：ARCHITECTURE §1，TECHNICAL_DESIGN §2/§3/§5.3/§7.2/§8，PRD 对应消息/按钮/附件/标准 Webhook 范围，DEVELOPMENT_PLAN P3，P03-04 实施/审计摘要及已有契约。调度确认历史/详情按 T3 明确范围纳入；同步/已读/WS/设备留后卡。

在前卡 9 操作、28 Schema 基础上新增 9 操作、16 Schema，合计 18 操作、44 Schema。既有路径、Schema、公共响应、安全 scheme 全部语义不变，既有 61 个正反例全部保留。

| 接口 | 冻结内容 |
| --- | --- |
| `POST /bot/messages` | 启用 Bot send；标准载荷；201 Message；sender 服务生成 |
| `PATCH /bot/messages/{id}` | 仅本 Bot 自身 application 消息；完整内容替换；expected_revision CAS；200 Message；不改变 bot_seq/created_at/未读/响铃 |
| `POST /webhooks/{bot_id}` | 独立 WebhookKey；只创建同一标准消息；不提供字段转换、模板、脚本或附件读/上传 |
| `POST /client/bots/{id}/messages` | Android Bearer；目标启用且 receive；201 message + delivery_id，仅传输持久化回执 |
| `POST /client/bots/{id}/callbacks` | Android Bearer；独立 button.callback；只接收消息/版本/按钮 ID，服务取参数快照与身份；201 event_id/delivery_id 等回执 |
| `GET /client/bots/{id}/messages`、`GET /client/messages/{id}` | Android Bearer；当前快照/按 bot_seq 从新到旧有界历史；复用前卡分页字段和 OpaqueCursor，非同步游标；管理员可看停用 Bot 历史 |
| `POST /attachments` | Android Bearer 或启用 Bot send；query bot_id/kind/filename/media_type；原始单文件流；完成持久化/ready 才 201 Attachment |
| `GET /attachments/{id}` | Android Bearer 或 Bot 授权文件；receive-only 限领取事件所需文件；200/206/304/416、Range/ETag/条件下载、安全头 |

所有消息/编辑/Webhook/callback/上传要求 UUID v4 幂等键。身份+路由类别作用域独立于 Token 代次；摘要包含路径目标、内容、引用、附件、按钮与版本，上传包含实际摘要。同键同请求恢复原结果，不同内容409 IDEMPOTENCY_CONFLICT；编辑及点击重放先恢复已提交结果，再校验新操作版本。仍先验证当前有效身份/授权，旧已撤销凭据不能恢复结果。清理后7天最小墓碑命中410 RESOURCE_EXPIRED，不重建；受保护投递记录不按7天删除。

引用/附件/消息/按钮须在当前 Bot 授权域内；跨 Bot/无权/不存在统一404。附件仅 ready、类型匹配且无重复；按钮 ID 唯一为运行时语义约束。普通引用自然过期只保留原 ID供显示过期。编辑 CAS 冲突409 REVISION_CONFLICT，点击陈旧版本/失效按钮409 BUTTON_STALE；点击数据不 trim、不解释，后续编辑不修改已提交快照。无 Android 编辑接口，Web Cookie 无消息、callback、附件或历史权限。

JSON 256 KiB；文本64 KiB UTF-8、callback_data 4 KiB UTF-8；附件10个、按钮10行/每行5个、label128字符。图片10 MiB、文件50 MiB 默认值可配置；检查实际流字节，半文件不能返回上传成功。文件 staging/ready/缺失/清理及活动引用边界按设计描述，无分片协议。Range 含闭区间/开放结尾/suffix/多区间；语法或start>end为400，全不满足/零长度资源/suffix0为416，部分满足为206。单区间与416 Content-Range 结构区分，206满足start<=end<size；If-None-Match与If-Range顺序、nosniff/安全Content-Disposition/no-store均明确。

## 实际验证

环境：Linux，复用前卡锁环境 `/tmp/relaynest-p0304-locked/bin/python`，Python 3.13.15。未重装/升级依赖，未访问真实服务或外部系统。默认 `python3` 不在 PATH，后续统一使用该绝对解释器。

| 实际命令/检查 | 结果 |
| --- | --- |
| `/tmp/relaynest-p0304-locked/bin/python -m openapi_spec_validator server/api/openapi.yaml` | PASS，`server/api/openapi.yaml: OK` |
| `/tmp/relaynest-p0304-locked/bin/python server/api/validate.py` | PASS，`44 schemas; 92 valid + 127 invalid examples; 18 operations/security` |
| Python stdin 基线语义比较（用 `git show 30efad4...:server/api/openapi.yaml` 载入旧契约，逐项相等断言） | PASS，原9操作及28 Schema/全部公共组件不变；前61个示例逐项保留；requirements.in/txt字节不变 |
| Python stdin 临时契约负面变异，见下方 | PASS，8个错误声明全部被现有校验拒绝，临时文件在临时目录回收 |
| `py_compile.compile('server/api/validate.py', cfile='/tmp/relaynest-p0305-validate.pyc', doraise=True)` | PASS，编译输出在仓库外，无 pycache 入库 |
| `git diff --check` | PASS |
| `git status --short`/提交前 diff 审阅 | 仅本任务允许路径 |

案例包含标准文本/Markdown/附件/按钮、空内容/非法种类/未知字段/伪造sender、callback不可伪造参数或身份、CAS字段缺失/非法类型、ASCII/中文/emoji UTF-8字节等于与超过上限、非法代理项、合法冲突/过期/存储错误与错误隐私字段反例。Range 36个资源大小组合（size=0/1/100），包含首/末字节、开放范围、suffix0、suffix超资源、部分越界、全部越界、多区间和超64位端点；另有13个非法语法/逆区间及9个具体响应Content-Range反例/正例。大字符串以 repeat 夹具展开，不用字符数冒充UTF-8字节数。

八个临时负面变异均在原文档深拷贝后独立执行 validate.main：删除跨Bot引用声明、Webhook误用Bot凭据、下载加入Web Cookie、上传幂等键变可选、callback删除receive约束、编辑CAS变可选、公共404改成public缓存、上传图片默认上限改1。全部非零拒绝；最后一个以外均为权限/幂等/隐私边界负例，不以有合法UUID假称归属验证通过。

## 验收与限制

- expected_revision/幂等/大小限制明确：PASS（契约、真实正反例和权限声明层）；运行时事务不在本卡。
- 无 Android 编辑接口：PASS，唯一编辑路由为 Bot send，client 路由无PUT/PATCH；Web无正文或附件权限。
- 合法/非法载荷、幂等冲突错误、跨Bot引用权限声明、Range边界及前卡非回归：PASS，以上固定检查。
- 仅本任务文件、独立本地提交：提交后交接精确SHA；不推送/部署/发布，不修改台账。

扩展说明：`x-utf8-max-bytes`/Range扩展由校验器显式执行，普通JSON Schema不自动执行；JSON原始体大小和上传可配置上限是请求体扩展声明，运行时仍须流式拦截。当前锁依赖没有可选 URI format 检查器，URL用显式scheme/空白/控制字符模式检查，不新增协议白名单；format注解不证明完整URI合法性或客户端安全渲染。实例的合法错误 code 不意味着服务已经实现错误分支。

未执行：真实HTTP身份/跨Bot隔离/撤销/幂等与CAS并发事务、callback快照事务、文件fsync/rename/缺失恢复/清理竞态、真实Range多段编码及条件请求、Android/Web客户端、真机、RelayShelf、部署。Go仍只有前阶段入口，本卡没有对应运行时实现，不能将声明/Schema通过冒充这些测试通过。没有未决需求/架构决策；后续实现必须按本契约和技术设计补真实运行时验证。独立审计和集成由其他角色执行。
