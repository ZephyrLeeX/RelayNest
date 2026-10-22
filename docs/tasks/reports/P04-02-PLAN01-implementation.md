# P04-02-PLAN01 会话职责拆分规划交接

## 范围与结论

- 角色：规划；基线 `fd7ca5173c540796aa97e3efb9bf90a7adc94c32`。
- 工作树 `/home/lijx/.codex/worktrees/session-plan/RelayNest`，分支 `codex/session-plan`；开始时干净。
- 仅修改 [父卡](../cards/P04-02.md)，新增 [S01](../cards/P04-02-S01.md)、[S02](../cards/P04-02-S02.md) 与本报告；无源码、registry、API、设计或旧报告改动，无 push。
- 建议拆两张串行实现卡。存储与事务并发全部留 S01，HTTP 凭据校验/路由/双端响应与组合验证留 S02；没有按安全难点继续碎拆。父卡保持原前置和全部原验收。
- 本次为规划交付，尚须不同 agent 对本提交精确 SHA 独审和集成；不代表 P04-02 实现、P04-01 实现或任何安全门槛通过。提交自身 SHA 按协议仅在提交后交接/台账记录。

## 固定证据与最小路径映射

| 基线定点 | 核对结果与规划用途 |
| --- | --- |
| TECHNICAL_DESIGN §7.1 | >=256 bit 会话原值、仅 SHA-256 摘要；Web 24h/7d、Android 30d/90d；续期不超绝对期限，数据库撤销/代次校验 |
| OpenAPI servers、/auth/login/session/logout | 实际前缀 /api/v1；login 无既有会话，session 接受 AdminBearer 或 WebCookie，logout 接受 AdminBearer 或 WebCookie+CSRF，成功 204 |
| LoginRequest/Session 及相关 response/securitySchemes | 双端设备字段、两期限、Web session+csrf_token、Android 登录独有 access_token/token_type、Cookie/CSRF 名称与最低响应安全已写入 S02 |
| internal/httpapi/router.go | 固定基线仅健康与静态路由；S02 最小补三条认证路由注册/注入和测试，不授权其它业务 API |
| cmd/relaynest/main.go | 现有 serve 使用 httpapi.New；S02 仅补已集成存储/认证实例的装配和关闭及测试，不冻结未来构造函数 |
| migrations/0001_initial.sql | 基线仅基础状态/changes；不能据此推断正在实施的 P04-01 schema，后续迁移须复用其最终集成结果 |

上下文只取根约定、贡献/执行协议相关节、父卡及上述章节与入口小片段；未读取 P04-01 动态工作树或推断其实现。规划不冻结管理员读取、摘要、generation 或 reset 的实际存储接口。

父原范围 auth/migrations 无法单独完成 HTTP 可达性，故补 httpapi 和 cmd 两处现存入口的最小职责映射。迁移仅 S01 确需时新增，编号从派发时最新 main 下一可用值串行分配；0002 属 P04-01，不预占 0003。测试夹具限 auth 内，沿用既有 Go/SQLite 依赖；如须触及其它存储模块或依赖清单，先交调度补边界。

## 建议 registry 条目与依赖（由总调度写入）

| 条目 | kind / 关系 | 强依赖或触发条件 |
| --- | --- | --- |
| P04-02-PLAN01 | planning，parent_id=P04-02 | 既有已完成基线门槛；不依赖 P04-02 done，也不解除 P04-01 实现前置 |
| P04-02-S01 | implementation，parent_id=P04-02 | P04-01 done、P04-02-PLAN01 done（均须独审、精确集成） |
| P04-02-S02 | implementation，parent_id=P04-02 | P04-02-S01 done，传递保留原前置 |
| P04-02 | 保留父条目 | 保留 P04-01，并追加 PLAN01、S01、S02；原验收及组合证据齐备才可收口 |
| 各条目的 A01 | audit，subject_id=被审 ID | 被审 review + 精确基线/实现 SHA；不以被审 done 为强依赖 |
| 各条目的 I01 | integration，subject_id=被集成 ID | review + 最终精确 SHA 的审计 PASS；不以被集成 done 为强依赖 |

子卡的 parent_id 仅归属关系，不作为对父 done 的依赖。父→子、S02→S01→PLAN01/P04-01，无反向强边；A/I 采用 subject 与触发条件，避免 review→done 死锁。registry 的状态、owner、最终 SHA 由调度按真实结果填写，本报告不复制/预填。

建议 gates：S01 生命周期/SQLite 事务并发证据与独审集成；S02 冻结认证 HTTP 契约、最低安全与组合验证及独审集成；父保留原三项业务验收及通用检查/提交/审计集成验收。不新增产品门槛或声称通过后续 P04-03/P04-04。

## 派发前仍须核查

1. P04-01 最终实现、独审和精确集成 done；实际管理员/Argon2id/generation 入口、数据库注入、schema/迁移/测试夹具入口。当前规划不指定函数签名。
2. reset 更新密码/代次与撤销的事务边界；S01 登录创建、鉴权续期如何在同一一致性约束下防止旧代次复活。缺必要存储协作点须明确报告再补边界，不擅改 P04-01。
3. 当时最新 main 的下一可用迁移号与既有表，避免重复建表/并行抢号；只有确需 schema 变化才新增。
4. Web 来源/HTTPS 的现有安全输入与 CSRF 会话绑定实现；三条路由最低要求不能延期。完整可信代理与全局策略仍属 P04-03，若无法安全接线须报阻塞，不信任任意转发头。
5. 推送映射/连接是否已经存在；复用实际撤销协作，否则明确未来接入点与未验证项，不新增或假称验证 WS/推送系统。P04-04 登录并发/限流/公共日志继续保留。

## 验证与限制

- 已核对父原五项完成标准逐字保留，原实现目标保留；三条路由、schema/security 标识和两端期限对照固定基线。
- 实际检查：`git diff --check` 通过；Perl 定点解析四份文档的相对链接全部存在（PASS）；提取基线父卡 checkbox 并逐字比较，5/5 保留（PASS）。`git status --short`/diff 核对仅四份授权文档；依赖表及路径职责人工复核无环、一致。
- 纯文档规划不运行业务全量测试；提交前另检查 staged diff 空白及路径范围，提交后确认工作区状态与精确 SHA。
- 无业务实现/外部系统/真机验证，不能以本报告替代后续实际检查。总调度需另派独审与精确集成；实施前置未满足时不得启动 S01。
