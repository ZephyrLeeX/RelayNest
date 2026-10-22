# P03-07-PLAN01 派发范围核对与串行拆卡报告

日期：2026-10-22；角色：planner；结果：implemented（规划交付，尚待不同 agent 独立审计与集成，不代表契约或G-API通过）。

- 精确基线：`2d2c633366742011a96e9cac5297c7b4bb30eafd`，开始HEAD相同，工作树干净。
- 工作目录：`/home/lijx/.codex/worktrees/api-delivery/RelayNest`；分支：`codex/api-delivery-run3`。
- P03-06实现：`fd0f3cb9526637fa25d472ad801f995d88f0d426`；本树HEAD为其独立PASS审计提交，已读其精确摘要。调度已通知main集成完成于`57ff401f95d9f747ec85938cbb521190463ae3c2`；本规划不承担该集成的复验，也不向其写入。
- 修改仅父卡派发/子卡链接、新增三子卡及本报告；源码/API/旧报告/registry/技术设计未改。提交SHA在提交后交接，不在提交内预写自身SHA。

## 拆分依据及预算

原卡合并四个同步操作、设备会话、注册更新/删除/恢复读取、设备通知配置及模式CAS，另以G-API覆盖正式契约。T3/T8全索引核对还发现全局配置与健康投影没有明确P3承接卡。一张实现同时处理所有类型、权限及变更union，会跨多个独立状态域；不宜仅因四份共同文件就认为上下文小。

实测只读keys得到：23操作、61 Schema、369示例，P03-04新增9操作，P03-05新增9，P03-06新增5。文件大小：openapi.yaml 88,359字节、examples.json 130,629、validate.py 15,415、README.md 19,089（合计253,492字节）；不能让每个开发者全量灌入再加所有设计/旧报告。粗估整卡输入约25～40k tokens，工作结果与新增边界测试还将增长，超过协议默认输入≤12k；这是字符/职责估算，不是假称模型实际用量。

采用[S01](../cards/P03-07-S01.md)→[S02](../cards/P03-07-S02.md)→[S03](../cards/P03-07-S03.md)，分别约7～10k、8～11k、7～10k输入tokens；前置摘要只给keys/版本/错误/限制，不传全量日志/源码。各卡同时遵守模型窗口总占用≤60%、至少25%留交付；无法压至预算则先checkpoint，不靠长历史压缩硬吞整阶段。

已读AGENTS、CONTRIBUTING、WORKER_PROTOCOL、原P03-07、T3/T6/T7.1/T8/T9局部、架构§1、阶段P3、PUSH-PLAN/PUSH-DECISION已审集成摘要、P03-04/05摘要及P03-06实施/审计摘要；契约仅取keys和Sequence/Version/Session/Message/Bot/DeliveryMetadata相关局部，health只读router/handler/对应测试局部。旧报告附录与全部源码/示例未作为本卡输入。

## 全索引核对及归属缺口

T3.2表内全部方法按keys核对，以下表覆盖其每行（路由均以设计的/api/v1为前缀，另注明公开健康例外）。

| T3来源/操作组 | 本基线 | 明确承接 |
| --- | --- | --- |
| auth login/session/logout，admin bots列表/创建/修改/删除，两套凭据reset | 9操作已存在 | P03-04已交付，后卡非回归 |
| Bot消息创建/编辑、标准webhook、client消息发送/callback/历史/详情、附件上传/下载 | 9操作已存在 | P03-05已交付，含后续F01既有修复，不重写 |
| bot claim/ACK/renew、admin deliveries列表/retry | 5操作已存在 | P03-06已独立PASS并由调度精确集成 |
| client sync/bootstrap、共享read、WS | 缺4操作 | S01；WS包含独立frame Schema/升级声明，不能仅计HTTP路由 |
| devices列表/指定会话撤销 | 缺2操作 | S02，管理权限和当前会话注册权限分开 |
| devices/current/push、devices/current/notifications | 缺2个PUT；恢复读/条件删除也尚未冻结 | S02按T9.3补齐必要恢复接口与版本屏障，不预报新增操作总数 |
| admin/settings GET/PUT | 两方法均缺 | S03补正式契约；设置变更同步分支同时归S03 |
| T3.3 hello/changes_available/session_revoked | 缺frames | S01定义，后续S02撤销/偏好边界关联 |
| T8公开live/ready | 不在OpenAPI，但最小运行时已有 | S03文档化根路径纯文本现状；运行时扩展仍P06-08 |
| T8管理员系统状态/健康投影 | OpenAPI无对应路径/Schema | S03补已有需求的管理员读取投影；路径命名留实施者冻结，非T3原有路径 |

证据：`docs/TECHNICAL_DESIGN.md:112`明确GET/PUT admin/settings与expected_version；§8（约284～286行）分别要求公开live/ready与管理员队列/等待/失败/附件恢复/DB和推送摘要。PRD §3.4默认保留30天、§3.6图片10MiB和文件50MiB均可全局调整，§6要求基础健康/连接/失败状态。P03-04报告明确设置留P03-05～07；P03-05报告只处理9个消息/附件操作；P03-06报告只新增5个投递/失败操作。原P03-07正文只有设备通知/注册和同步，尚没有明确全局设置/健康条目；API README虽提后续“设置”，不能代替任务验收归属。

[P06-08](../cards/P06-08.md)已承接全局配置CAS、队列/连接摘要和readiness运行时，但允许范围只有internal/admin、health、httpapi，没有server/api；[P03-01](../cards/P03-01.md)是最小工程/健康入口，不能代替正式契约。故补S03为原G-API父卡的既有需求归属，P06-08继续实现对应运行时。没有新增保留策略/权限/配置产品参数，没有把阶段门槛改成只验已有23操作。

## 公开健康与管理投影分开

只读源码证据：`server/internal/httpapi/router.go:12`挂GET /health/live、下一行GET /health/ready，均在根路径而非/api/v1；`server/internal/health/health.go`的Live/Ready/respond返回纯文本：live为200与`ok\n`，ready在有成功probe时200，否则503与`unavailable\n`；Content-Type为text/plain; charset=utf-8，含no-store和nosniff。Ready缺probe、停止、探针错误/超时/取消不泄露错误内容。测试局部也断言body/状态。这里是源码和现有测试阅读，不声称本轮运行过Go测试。

T8管理员状态包含内部脱敏摘要，只能给管理员Bearer/Web Cookie；不是这两个公开纯文本接口。S03应在同一正式OpenAPI用操作级servers等规范表达根路径；不能把/api/v1前缀误套给health、不能将公开503当管理JSON Error，也不能把管理摘要挂公开ready。建议管理投影名称/admin/status仅是普通命名建议，具体名称由实施者交接，不冒充技术设计已冻结路由。

## 依赖、冻结输入与验收映射

| 子卡 | 最小执行前置 | 写职责与冻结交接 | 父验收覆盖 |
| --- | --- | --- | --- |
| S01 | P03-06、PLAN01均审计集成/登记 | 共同四API文件独占；同步envelope、H/epoch/floor、令牌、read、WS；版本/错误/时间位置与剩余变更类型摘要 | epoch/floor/序号字符串/版本；同步/read/WS合法非法边界 |
| S02 | S01审计集成 | 同四文件独占；当前会话多provider注册及条件删除/恢复读、设备撤销、设备通知CAS；补设备设置变化union | 多通道并存/局部更新/迟到失败/迟到回调/CAS恢复/非法provider/越权/D1～3 |
| S03 | S02审计集成 | 同四文件独占；全局配置、公开health和管理员系统投影；补全局设置变化union，核对整个T3/T8矩阵 | 全部子卡证据、所有操作/Schema/例子和非回归；最终正式G-API契约覆盖 |

三卡报告分别用`reports/P03-07-S0N-*.md`，依赖锁文件只读；不得并发写共同OpenAPI/校验器/例子/README。允许新增分支必须列出相对基线的有意扩展，不用任意object/无法解析引用作为设置Schema占位。S01先交付其域，S02/S03补齐设置域后才是全量契约；中间通过不宣称G-API正式完整。

每张实施后不同agent独立审计，再精确集成main；下一张取实际main集成SHA与前卡≤800字摘要。父卡原P03-06依赖保留，子卡不依赖尚未done的父卡；父卡最终汇总全部原验收和三子卡证据。S03完整矩阵/审计/集成前，不放行G-API；审计或集成用subject_id而非强依赖父done，不形成环。

D1～D3已批准：固定通用title+content、vendor显式可选、切换仅未来新事件且接受已外发/在途残余、超时不跨通道补响、vendor离线DND/已读/退出撤回限制。仅vendor例外，不扩大FCM §9.2豁免。注册成功不激活vendor，未准备OPPO账号/权限、G-OPPO未实测不阻核心通道无关契约。厂商SDK/REST、签名/分类/参数仍待P02-05证据；本轮不访问外部接口，无需重做厂商研究或PoC。

本轮未发现需要选择新需求/架构/权限的明确矛盾。初始注册/删除后的版本屏障、provider校验及恢复读字段命名留S02在现有T9.3内冻结；若实施发现需要改变权限/新增体验规则，应带证据返回调度，不把本规划视为预先授权。

## registry新增条目建议（本轮不写入）

调度可按已有字段形状补owner/SHA/证据；新实施卡初始todo，base_sha/implementation_sha/audit_report/integration_sha保持null直至真实派发/产物，gates_required仅沿用G-GIT。不得把G-API作为其自身生产卡前置。

| ID | kind / 关系 | dependencies | card / evidence |
| --- | --- | --- | --- |
| P03-07-PLAN01 | planning，parent_id=P03-07 | [P03-06] | 本报告；完成规划审计集成后才done |
| P03-07-PLAN01-A01 | audit，subject_id=P03-07-PLAN01 | 不以PLAN01 done作强依赖；触发review+固定实现SHA | 独立审计报告待创建 |
| P03-07-PLAN01-I01 | integration，subject_id=P03-07-PLAN01 | 按协议：原卡review+最终审计PASS触发 | 精确实现/审计SHA与目标集成报告待创建 |
| P03-07-S01 | implementation，parent_id=P03-07 | [P03-06, P03-07-PLAN01] | cards/P03-07-S01.md |
| P03-07-S02 | implementation，parent_id=P03-07 | [P03-07-S01] | cards/P03-07-S02.md |
| P03-07-S03 | implementation，parent_id=P03-07 | [P03-07-S02] | cards/P03-07-S03.md |

子卡各自后续派生A01/I01按统一协议，不提前填写虚假SHA/通过证据。父P03-07 dependencies不改；汇总收口条件放card及调度检查。G-API.source_tasks保留P03-04/05/06/07即可，通过父卡证据显式包含三子卡；不需加入导致循环的G-API前置。若调度schema没有planning.card专用路径，可用父卡路径并以本报告作为evidence，不能虚构不存在的PLAN01卡。

## 下一派发包

先派PLAN01-A01，只读此精确规划提交和基线，允许仅写其审计报告；审计通过后由集成者只集成本次五份文档。之后派S01 developer：单卡S01、实际main精确SHA、独占工作树/分支、四份API写范围+自己的报告、锁环境状态、T3/T6相关局部、P03-06摘要、23/61/369 keys基线与Sequence/Version等局部。必跑锁定OpenAPI/例子/声明校验、前卡非回归、diff检查。PLAN01审计前不得据本卡直接派实现；S02/S03均等其直接前置审计精确集成后再派。

## 本轮实际检查与限制

Linux，复用`/tmp/relaynest-p0305-f01-locked/bin/python`，实际版本Python 3.13.15；YAML/JSON只读索引确认23/61/369。对本轮五份文档执行本地Markdown文件链接解析：33个文件链接全部存在；逐字节对照精确基线：父卡之外313个已跟踪文件全部不变，三张子卡目标/范围/验收/上下文/交接字段存在。`git diff --check`通过，显式stage后另跑`git diff --cached --check`覆盖新增文件。未校验外链可达性与Markdown锚点；无API源码变化，不重复运行API全套/Go/Android/平台/真机测试，不把本规划交付作为任一运行能力证明。未推送、发布、部署，不自行派agent或更新台账。
