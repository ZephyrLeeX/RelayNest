# P08-04-D01-CONTRACT-PLAN01-A01 独立规划审计

- 角色：独立 audit；被审任务 `P08-04-D01-CONTRACT-PLAN01`。
- 基线：`00d9f79a4a009bd93505eaa6c9bdbce4eefbd687`。
- 固定被审 SHA：`ca8cd805b0dc7f13aba8fed2a4ece5319eff92d6`。
- 工作树：`/home/lijx/.codex/worktrees/instance-contract-plan/RelayNest`；分支 `codex/instance-contract-plan`；开始干净。
- 结论：**PASS（仅规划）**。无阻断发现；不表示新契约、实例身份实现、S04 或父卡验收通过。

## 依据与授权

读取根 AGENTS、CONTRIBUTING、WORKER_PROTOCOL 审计/交付节；固定差异中的拆卡报告、8 卡和父/S04 追加内容；原 D01 提案及其 A01/I01、父/S04 原验收、T7/T9 必要局部、Session/AndroidSession 和 API README 索引、存储迁移与管理员认证代次局部。

原提案审计对象 `46db3851c7d50e488ac3e45df65c08651ef0890d`，既有 I01 记录其精确集成及独审证据。其历史待批准文字不覆盖后续用户决定。本轮调度授权与 root 当前未提交 registry 的 `P08-04-D01-CONTRACT` answered 条目相符：G1 残余窗口、生命周期、具体接口及旧版本/额外探测成本三项均已批准，允许正式契约冻结及实现，不需重复批准；未授权真实维护。

## 检查结论

1. **有限可执行链完整。** C01→S01→M01→B01→N01→K01→R01→V01，各卡标明局部允许路径、主要文件边界、相关检查、精确最终 SHA 的另人 A/PASS/I；A/I 用 subject_id，不等待被审任务 done。S04 在 V01 后汇总，原父全部目标/验收保留，无环。C01 可直接写正式设计、OpenAPI、schema/examples/validator 并全量契约回归、独审冻结，无继续无限提案前置。
2. **契约影响受控。** C01 明确公开无凭据 probe、UUIDv4/header、Android login/session 字段、400/409/429/503、安全 headers、解析/限流/超时和旧客户端阻断窗口；既有封闭 Session 不直接 allOf 扩字段，要求核对全部消费者、设备投影与判别。Web/Origin/CSRF、Bot、health 保持既有语义。旧 G-API 的 39 operations/115 schemas/1229 cases 只代表旧范围；新增门槛仍是建议，必须记录新计数和独审冻结 SHA。
3. **生命周期与撤销未削弱。** S01 在既有 BEGIN IMMEDIATE 迁移事务内将 DDL/随机初始化/台账共同提交，覆盖并发、随机源/锁/提交/崩溃故障及损坏失败关闭。原库无回滚停机迁移保留 ID；恢复/clone 由人工停服维护换 ID，与 auth_generation/全部管理员会话/现有 push 映射撤销同事务。M01 要求实证枚举现有撤销路径；不存在 push 映射时保留源码/schema 证据、扩展测试和未来强门禁，未编造表或删除要求。未预占迁移 0005。
4. **G1 与客户端边界准确。** N/K/R 分别覆盖无凭据逐请求探测、密文版本/AAD 与旧记录重登、统一票据/代次协调及迟到响应丢弃；失败保留密文但禁发送凭据。明确发送前发现变化未发凭据，与 probe 后瞬切已发字节但诚实 B 拒绝认证/业务两类证据；不声称 G2 或公开 ID 持密证明。
5. **共享文件与外部验收门禁保留。** B01 等待 Bot/设备共享 router/main 释放；Android N/K/R 逐卡串行，不能按文件名假定可安全并行。审计时 root 当前台账显示 Bot F01 已 done、设备 S03 running；这是规划时点后推进，不解除设备依赖。V01/S04 仍依赖完整 P04-05、P03-03/S03；phone/NAS/OPPO、真机 Keystore、release 签名及未来 Room/outbox/WS/push 均不冒充已测。

## Findings

无需要修复的编号发现；无 CHANGES_REQUIRED/BLOCKED 项。上述 PASS 仅允许规划精确集成后登记、派发 C01。

## 实际验证与限制

- `git rev-parse HEAD`、`git status --short`：固定 SHA 一致，开始无修改。
- `git diff --stat/--name-only 00d9f79..ca8cd80`：11 个文档文件，8 新卡、1 新报告、父/S04 各追加 5 行；无 API、源码、SQL、registry 修改。
- Perl 只读检查：11 文件中的 71 个相对链接存在；父/S04 相对基线全文为当前文件原样前缀，原验收未删改。人工核对 8 卡依赖链及 A/I 触发规则。
- `git diff --check 00d9f79..ca8cd80`：PASS。
- 环境 `python3` 命令不可用；尝试用其只读摘要 registry 返回 127，随后用 jq/定向文本完成台账核对。未安装工具。
- 本次只有规划文档审计，未改正式 API，未运行业务/schema/Android/Go 或真机测试；不据此放行新契约或运行时门槛。未来各卡必需检查仍按卡执行。

本审计只新增本报告并独立本地提交；不修改被审材料、不写 registry、不 push/部署。报告提交 SHA 由提交后交接提供，不在文件内预写自身 SHA。
