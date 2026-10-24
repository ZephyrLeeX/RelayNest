# P08-04-D01-C01 实例身份契约实施

- 角色：developer；结果：implemented，等待最终实现 SHA 的独立 A01/PASS/I01，**未自行冻结**。
- 实施提交前精确 HEAD：`5d0e64ba453f93f7c1d97ee07b370f539e6b6e4e`；原源码审查基线：`8138b794e8023dd2eff4a1e03ec1e0a8c07f022c`。前者仅增加混合鉴权提案/checkpoint及其独审，六份源码草稿沿原基线保留并继续实施，未 reset/cherry-pick覆盖草稿。
- 工作树：`/home/lijx/.codex/worktrees/instance-contract-freeze/RelayNest`；分支：`codex/instance-contract-freeze`。最终实施 SHA 在提交后交接，报告不预写自身 SHA。
- 用户最新批准已由调度传达：原三项 G1、生命周期、具体契约/兼容成本及仅 upload/download 的 Bot-first 例外均已批准；历史 proposal/checkpoint/A01 中“待批准”仅描述当时状态，不推翻本轮授权。

## 变更与验收

最终差异为7路径、2939行新增/43行删除（六份契约2896/43，本报告43/0）。只改六个契约路径和本报告：docs/ARCHITECTURE.md、docs/TECHNICAL_DESIGN.md、server/api/{README.md,openapi.yaml,examples.json,validate.py}。不改 runtime、SQL、Android、任务卡、registry、依赖或迁移号。

1. 正式设计与机器契约包含公开无凭据 GET /api/v1/instance：随机小写 UUIDv4单字段封闭对象、拒绝凭据头、不解析记录值、400/429/503、no-store/nosniff、无缓存/304/redirect、标准 TLS、解压后1024字节/完整call 5秒/共享20rps burst40。probe请求/响应、UTF-8、类型、schema和header正反例均由全量validator检查。
2. 31个Android login/AdminBearer条件头操作与所有认证分支矩阵固定；缺头400 INSTANCE_REQUIRED、畸形400 INVALID_REQUEST、错ID409 INSTANCE_MISMATCH、不回显实际ID、读取失败503。除两附件例外，先ID再读取密码/Bearer，事务内复核ID与auth_generation；Web/health及纯Bot/Webhook规则保留。
3. 两附件使用独立精确 x-instance-binding：仅Bot API识别可在ID前读Bearer，按API凭据查询/常量时间摘要匹配，不能仅依私有token形状或实例头判别。下载需独立身份发现，不能先读附件取botID；现Authenticate需要botID，不能冒称可直接复用。Matched后停用/能力/跨Bot/撤销/代次失败终止原分支，绝不把后续401/403降为NoMatch；依赖/存储错误与超时503 INSTANCE_UNAVAILABLE、retryable=true、不回退。仅确定NoMatch才先ID后Admin，接受部分无效凭据缺头400/错ID409；不引入新token、表/索引或公开端点。
4. 独立封闭AndroidSession与required instance_id，login独有Bearer，GET session无Bearer。公共Session/WebSession/WebSessionResponse/DeviceSessionPage与原base完全相同；12处全部Session消费者引用图固定，未知消费者也会被拒绝。原1229例保留，仅三个Android响应value补instance_id；新增1226例。原base已有三个重复示例名，各出现两次；初始辅助检查误设全名唯一而失败，改为核对重复名集合/次数与base相同后通过，本轮没有新增重复名或删改旧用例。
5. 旧cipher强制重登、旧Android缺ID拒绝、旧server无probe失败关闭；停旧服务→迁移→新server→新Android，不承诺透明滚动升级。重启/原库迁移保留ID，backup恢复/clone人工停服换ID并原子撤销全部管理员会话和推送映射，清库新ID，缺失/腐败停服。G1明确接受probe后瞬切凭据字节可能到B；公开随机ID不证明秘密，不声称G2。

## 实际检查

环境：项目mise Python **3.13.15**。原临时venv不存在，按原锁重新执行（未升级）：

```sh
/home/lijx/.local/bin/mise exec -- python -m venv /tmp/relaynest-health-contract-locked
/tmp/relaynest-health-contract-locked/bin/python -m pip install --require-hashes -r server/api/requirements.txt
/tmp/relaynest-health-contract-locked/bin/python -m openapi_spec_validator server/api/openapi.yaml
/tmp/relaynest-health-contract-locked/bin/python server/api/validate.py
/tmp/relaynest-health-contract-locked/bin/python /tmp/c01_baseline_check.py
git diff --check
```

最终实际结果全部退出0：OpenAPI CLI `server/api/openapi.yaml: OK`；validator **40 operations / 118 schemas / 2455 cases（1537 valid + 918 invalid）**，**34项声明变异拒绝**且Session引用图检查PASS。变异已写入validator，覆盖原20项及header/ID类型、两附件匹配/回退/故障/资源先行/顺序、例外范围与未知Session消费者。本轮追加265例（混合分支260例及probe权限声明5例），包含有效/停用/能力不足/跨Bot/旧凭据/错误摘要/Webhook、缺/畸形/错头、故障/超时/匹配后撤销，以及Admin事务ID/代次变化；配对错误预期检查不会静默跳过状态断言。

辅助脚本`/tmp/c01_baseline_check.py`只读git show原base与当前JSON/YAML，逐项比较公共四Schema、原1229示例顺序及允许的三个value变化，并以importlib.metadata逐项核对requirements.txt：**18个锁依赖版本全部一致**。该临时脚本不是提交产物；上述核心Schema、引用图和全部示例检查均保留在提交内validator，审计可直接对原base复核。git diff --check无输出。

这些检查是声明/JSON Schema/合成状态模型，**不是HTTP/SQLite事务并发/Go/Web/Android/WS/Room/outbox/真机验证**。phone/NAS/OPPO/签名/Keystore未运行；旧运行时audit不自动满足新G1门槛。本卡无需真机，未操作现网。

## 后续接口与范围缺口

- storage包：`InitInstanceIDTx(ctx context.Context, tx *sql.Tx) (string,error)`、`ReadInstanceIDTx(ctx context.Context, tx *sql.Tx) (string,error)`、`RotateInstanceIDTx(ctx context.Context, tx *sql.Tx, expectedID string) (string,error)`；不自行提交，提交前不能发布候选ID；sentinel为ErrInstanceUnavailable/ErrInstanceMismatch。初始化/严格读/换ID和撤销事务语义见TECHNICAL_DESIGN §5.2。
- 离线命令：`relaynest instance rotate-id --db PATH --expected-instance-id UUID --confirm-offline`；退出码0提交成功、2参数/确认错误、3数据库忙/独占锁不可用、4身份缺失/腐败/expected不匹配/迁移未完成、1其他错误；不得输出新旧ID/秘密或自动迁移。人工停服要求不声称检测跨机器副本。命令仅冻结契约，本卡未执行。
- 下载身份发现需最小独立匹配适配，区分NoMatch/Matched/Unavailable；现`bots.Repository.Authenticate(ctx,botID,kind,secret)`依赖botID且将多种拒绝映射为错误，不能以统一err!=nil回退。现AuthorizeTx(Download)单独检查receive，不能冒称覆盖send下载与领取事件资源授权。
- **B01现卡scope不含botsAuth模块**，调度须补狭窄scope或另卡后实现该适配及真实附件资源门禁；本卡没有修改runtime/cards，也不把该缺口当已实现。日志与秘密边界、Bot Hub仅传输边界保持。
- 本地提交仅显式stage本卡7路径；不push/部署/发新聊天。G-INSTANCE-CONTRACT须针对最终实现SHA另人独审与精确集成后判定，runtime/composition、S04及父gate未改变。
