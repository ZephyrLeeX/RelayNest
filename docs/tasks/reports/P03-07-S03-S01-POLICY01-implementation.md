# P03-07-S03-S01-POLICY01 预算退出检查点

角色：规划。结果：checkpoint / incomplete；不是政策落文完成或 API 验收证据。

## 基线、已批准规则与退出原因

- 基线 `a3cbec4442a1968c0e50005b145f707dbb675bc2`；独占 `/home/lijx/.codex/worktrees/api-global-settings/RelayNest`，分支 `codex/api-global-settings`，开始干净。
- 调度转达用户已批准：保留期为正整数天、附件上限为正整数字节；0 不支持，也不代表立即清理、禁用或无限；默认30天/图片10MiB/文件50MiB不变，不新增业务上限。不得重复请求同项审批。
- 首轮读取把卡片、旧checkpoint和设计命中拼成过大输出并截断，随后定向补读；工具原始输出统计合计约16.9k tokens，另有外部资料，已超过12k材料硬限。该统计不是模型精确输入用量，不能宣称符合8k/12k预算。
- 调度明确要求依协议退出，只提交本检查点；不再读取，不改PRD、设计、任务卡或API。旧checkpoint的“待用户”描述是历史状态，已被上述决定解除。本轮未落实政策文档，须交新上下文继续。

## 已核实定位（均为本基线）

| 来源 | 证据与含义 |
| --- | --- |
| [PRD §3.4/§3.6](../../PRD.md) 第77/100行 | 只写默认30天、图片10MiB/文件50MiB及全局可调；需最小追加已批准正整数/0语义，无新业务上限 |
| [T3.2](../../TECHNICAL_DESIGN.md) 第112行 | GET/PUT admin/settings、管理员、expected_version防覆盖；尚无新全局字段的编码与技术范围 |
| [T5.1/T5.4/T6/T8](../../TECHNICAL_DESIGN.md) 第185～210/230～236/238～252/280～282行 | SQLite模型；created_at清理、保护优先、缩短影响后续清理；设置变更属于changes；图片/文件明确为可配置初始值，不能认作绝对上限 |
| [OpenAPI](../../../server/api/openapi.yaml) 第2384～2400/3048～3075行 | Sequence为0..9223372036854775807规范十进制字符串；Attachment.size引用Sequence。Version为integer、minimum=1，无maximum。UTCDateTime为date-time字符串、Z结尾，不能擅加新的时间范围或换编码 |
| 同上第987/4096～4120行 | 上传按全局配置检查实际流字节，超限413、不信任Content-Length；400结构、422字段语义、413请求超限的既有职责需保留 |
| 同上第2173～2193/3705～3710行；[validator](../../../server/api/validate.py) 第260～291行 | 设备版本是独立integer/int64范围，409有SETTINGS_CONFLICT/SETTINGS_VERSION_EXHAUSTED；全局只能参考，不能改设备或把公共Version重编码 |
| [P06-08](../cards/P06-08.md) 第28～44行 | 后续实现配置CAS、压力错误及缩短结果；本轮不改此卡，技术交接留待全局契约冻结 |

## 待独立审计的建议，不是已冻结契约

1. 天数、图片/文件字节新字段可采用1..9223372036854775807的规范十进制字符串：SQLite有符号64位可存储，上传size已有对应表示，Web无需经Number失精。拒绝0、负号、小数、指数、前导零、空白和范围外值；具体keys及结构400/语义422分界由续作精确定义并补正反例。不把Sequence的0或“序号”业务语义带入配置。
2. 保留期不要直接乘24小时纳秒或生成超出UTCDateTime表示的cutoff。可比较完整UTC时间的已过整日年龄与天数，使用可证明无溢出的计算，未来时间不判过期；很大但有限天数不等于无限模式。Go Duration为int64纳秒，约290年，不能把Duration上限冒充产品上限。续作需定清分数秒/日边界及持久化时间比较方法，不修改既有UTCDateTime声明。
3. 字节计数避免`limit+1`与累计加法溢出；在读有界chunk时用剩余额度比较，实际超限沿413。SQLite须在转换/绑定前检查范围，不能任由INTEGER affinity把过大值变成REAL。磁盘/空间/并发边界是独立资源错误，不承诺配置上限即磁盘容量或单次上传成功。
4. 全局版本继续JSON integer，禁止通过新Schema把版本改成Sequence式字符串。续作比较：全局专用、引用公共Version的安全整数有界方案（例如Web Number可精确的2^53-1）与int64全范围方案；核对现有客户端栈与冻结规则后说明技术边界依据、耗尽409及CAS/同步一致性。不得直接规定所有客户端新增无损JSON库。公共Version无maximum，不代表已批准任意新版本上限；本轮不选择最终版本范围。
5. 409最新版本/GET恢复、唯一管理员鉴权、Cookie写CSRF/同源、全局设置+版本+change同事务、清理保护闭环均保留在S01验收；不因预算缩减验收。现有证据未证明必须改已审上传/消息/设备/公共Version/UTC契约。若续作确有必要，列具体API schema/例子/validate/关联卡影响，先交调度重审，不暗改。

技术依据：[SQLite存储与类型转换](https://www.sqlite.org/datatype3.html#storage_classes_and_datatypes)、[ECMAScript MAX_SAFE_INTEGER](https://tc39.es/ecma262/multipage/numbers-and-dates.html#sec-number.max_safe_integer)、[Go Duration](https://pkg.go.dev/time#Duration)。均已访问官方资料；网页不代表本项目运行环境或运行验证。

## 新规划续作的精简读取recipe

目标初始≤8k、硬12k；先统计局部输出，预算包含工具结果，留交付空间。调度派发准确新基线/独占树/允许四文档路径及已集成设备父摘要。本检查点替代旧checkpoint全文，摘要≤800字，不重复AGENTS/协议长文/旧报告/899例。

1. 核验HEAD/status，读根AGENTS一次，WORKER_PROTOCOL仅§1/§3/§4；S01卡读取一次。以本表keys定位，不全量打印rg命中。
2. PRD仅§3.4/§3.6；设计仅T3.2 admin/settings行、T5.4六行、T6设置变更所在段及T8初始限制行；P06-08仅工作/验收。政策已批准，直接最小落文。
3. API仅Version/UTCDateTime/Sequence/Attachment.size、上传描述、相关400/409/413/422及版本错误局部；按需定向查现有Web/Android JSON处理入口，先明确无代码/实现缺失证据，避免全仓研究。不整读API、examples或validate快照。
4. 完成技术范围/字段/错误语义方案、兼容影响和最小修订S01卡；写自己的续作报告。API六文件不改；文档链接/范围/规则默认值/六API字节/diff检查，独立提交后不同agent审计。

本卡不必拆掉settings/CAS/清理/change闭环。下一规划只完成政策与技术方案；审计通过后实施worker初始仅读最终政策摘要、S01卡、API keys局部和≤800字设备前置摘要，再按需读实现相关validator函数，避免重复此次过量预读。

## 检查与未完成项

实际执行：使用 `/tmp/relaynest-p0305-f01-locked/bin/python` 的内联标准库检查，本报告全部相对链接存在、状态只新增本报告、批准规则与默认值文字断言均PASS；读取六API字节与 `git show a3cbec4:<path>` 比较全部相等。`git diff --check` PASS；stage后复核cached diff。PATH中的 `python`/`python3` 均不存在，前两次检查未执行，随后用上述既有绝对路径完成，未安装或升级依赖。

NOT_RUN：新政策文档落文、技术方案最终选择/审计、API声明/例子新增、运行时/HTTP/SQLite并发/清理/客户端与真机验证。未重跑旧899例，不把旧checkpoint的PASS当本轮执行。本轮无新产品决定；版本技术取舍尚待续作，不自动升级为用户决策。
