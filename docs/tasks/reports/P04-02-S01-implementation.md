# P04-02-S01 会话存储与生命周期实施报告

## 基线与范围

- 开发基线：`b2d4d5fa8442f39f78d9fcbd3c4feba37fb20d8f`；工作树 `/home/lijx/.codex/worktrees/session-store/RelayNest`，分支 `codex/session-store`。
- P04-01、父卡拆分规划及夹具范围补充已由调度提供独审/精确集成完成证据。依据根 AGENTS、CONTRIBUTING、WORKER_PROTOCOL、父 P04-02、S01、两份 PLAN01 摘要、PRD 设备隔离、架构 C-01～05、技术设计 §7.1 和冻结 OpenAPI 认证局部实施。
- 仅新增 auth 会话实现/测试、分配的 `0003_sessions.sql`，维护 storage_test.go 的迁移版本/未来升级夹具并增加真实 0002→0003 数据保留测试，以及本报告。无 API/设计/CLI/依赖/registry/旧迁移修改。
- 本报告随实现提交；最终精确提交 SHA 由提交后交接给出，不在提交内预写自身 SHA。开发交付为 implemented，须另一个 agent 独审并精确集成才可 done。

## 存储与事务

`NewSessionStore(*storage.Store)` 复用既有写池、`Store.Write` 与 BEGIN IMMEDIATE；没有第二套迁移或数据库连接机制。新表 sessions 保存 UUID v4 会话/设备标识、设备名、端类型、管理员代次、双期限及撤销标记。随机源为标准库 crypto/rand；独立 UUID 随机量和 32 字节会话随机量全部成功后才进入创建事务。会话原值编码为无填充 base64url，仅 SHA-256 摘要入库，随机失败不插入半成品。

创建事务重新检查 `administrator.auth_generation` 与凭据校验观察值相等；reset 原有密码摘要/代次同事务实现无需修改。鉴权、续期、撤销在单个写事务内重新联合管理员代次、端类型及撤销标记检查。时钟在获得写事务之后读取，精确到纳秒，到点即拒绝。Web 闲置 24h / 绝对 7d，Android 30d / 90d；续期不越绝对期，时钟倒退不缩短既有闲置期。撤销与续期的任一串行顺序均不能复活会话。reset 使所有旧代次会话立即失效，无需逐行更新或删除。

会话元数据 `Session` 不含密码、摘要、Token、CSRF。秘密单独作为 `SessionSecrets` 返回，字段标记 `json:"-"`，HTTP 必须按冻结响应显式组装；本实现不记录日志。CSRF 是以本会话随机凭据为 key、固定域 `relaynest.csrf.v1` 为消息的 HMAC-SHA256，无额外持久秘密，可在服务重启后稳定 bootstrap，不能跨会话复用。此选择是实现推导，不宣称冻结文档指定算法或用户逐项批准算法。

依据：[OWASP CSRF 防护指南](https://cheatsheetseries.owasp.org/cheatsheets/Cross-Site_Request_Forgery_Prevention_Cheat_Sheet.html) 的每会话不可预测令牌、保密、请求验证与会话绑定原则；HMAC 使用 Go 标准库 `crypto/hmac` 配合 `crypto/sha256`。这里是会话秘密派生 bootstrap，并未引入双提交 Cookie。CSRF 请求比较与同源判断留给 HTTP 层，须常量时间比较、不得写日志。

## S02 真实入口与调用约束

- `Create(ctx, observedGeneration int64, SessionInput) (Session, SessionSecrets, error)`：先 `ReadAdministrator`，按原样用户名比较并 `VerifyPassword`，成功后传入该次读取的 `AuthGeneration`；禁止校验密码后再读新代次替代原值。Web 输入 DeviceID 必须空，由服务生成；Android 必须提供冻结格式 UUID v4，设备名非空。每次登录生成新会话 ID，不按 device_id 替换旧会话。
- `Inspect(ctx, token, expectedClient) (Session, SessionSecrets, error)`：读取当前有效会话，不续期，适合校验 CSRF 前取得 bootstrap 材料；不保证返回后一直有效。
- `Authenticate(ctx, token, expectedClient) (Session, SessionSecrets, error)`：所有其它请求校验成功后调用，再次核数据库有效性并滑动续期。失败请求不要先调用该入口；有状态业务写入未来仍须在提交前按其事务契约复核会话。
- `Revoke(ctx, token, expectedClient) error`：重新验证并撤销当前会话，不续期；重试无效会话返回 ErrInvalidSession。仅按当前 token 作用于精确会话，不影响其他设备/Bot。
- expectedClient 必须由 HTTP 凭据通道决定，Cookie=`web`，Bearer=`android`，不能信任请求体自报类型。空/非规范 Token、到期、撤销、代次不符和渠道错误统一 `ErrInvalidSession`；创建元数据错误 `ErrInvalidSessionInput`，随机失败 `ErrSessionRandom`，数据库/取消/提交失败 `ErrStorage`。错误没有底层 SQL 或凭据，不自动映射 HTTP 状态。
- Web 显式组装 session+csrf_token，并以 Cookie 发会话；Android 登录才返回 access_token/token_type，session bootstrap 不回发 access_token。日期按冻结 UTCDateTime 编码。此卡不实现这些 HTTP 行为。
- 时间/随机注入为 auth 包内测试字段，生产构造固定 time.Now 与 crypto/rand；无公开可削弱随机性的配置。

## 实际验证

环境：Linux amd64、Go 1.27.3 / GOTOOLCHAIN=local、Node 22.23.2、npm 10.9.8，使用 mise 已安装 runtime 的 bin 路径；SQLite 沿现有 modernc 驱动与原锁，不改依赖。

已运行并通过：

- `npm --prefix web ci --no-audit --no-fund`、`npm --prefix web run build`，生成本工作树被忽略的 embed dist 后再测 Go。
- server 目录：`go test -race ./internal/auth ./internal/storage ./cmd/...`，通过；另以 `go test -race -count=1 ./internal/auth ./internal/storage ./cmd/...` 强制重跑，auth 7.234s、storage 2.189s、cmd 2.487s，全部通过。
- server 目录：`go vet ./internal/auth ./internal/storage ./cmd/...`；三个修改 Go 文件 gofmt；`git diff --check`。
- 真实 SQLite：双端闲置/绝对期限前 1ns、到点、后 1ns，持续滑动至绝对上限；失败和 Inspect 不续期；摘要持久化、秘密/元数据分离、完整关闭重开与稳定 CSRF；跨渠道拒绝、当前撤销隔离。
- 创建随机源在 0/16/32/63 字节处失败及取消/无效输入/旧代次均无半成品；4 次真实 ResetPassword 与创建并发，并以阻塞随机源确定覆盖 reset 先提交的顺序；24 次独立 SQLite 连接的撤销/续期同时起跑，终态持续拒绝且失败不续期。
- storage 原有新库、历史 checksum、未来版本拒绝、锁、升级失败整批回滚/重开台账检查仍保留。真实 0002→0003 验证管理员用户名/代次保留、sessions 空表建立和旧程序拒绝；未来测试迁移改为 0004_upgrade/0005_failure。

首轮 storage 检查曾因夹具维护误改原并发计数断言 21→31 失败；已恢复为原 21，重跑通过，无弱化原断言。

## 遗留与未运行

无本卡已知阻塞。会话表暂保留已撤销/过期行，未增加清理策略。推送映射与 WS 关闭尚无子系统可接，本卡只提供数据库代次/撤销事实；后续实现必须核对该事实，不能把设备标识当会话身份，也不能据本报告声称已验证连接关闭。远程撤销路由/设备列表、HTTP handler/Cookie/中间件/启动接线、P04-03 同源与 CSRF 完整策略、限流和日志均未实现。

手机/NAS/MoviePilot/推送、会话 HTTP 实际 E2E 及正式 AC：NOT_RUN，不以存储测试冒充。
