# P04-01 开发交付

- task_id: P04-01
- role: developer
- outcome: implemented（待另一 agent 对精确提交独审及集成，非 done）
- base_sha: `fd7ca5173c540796aa97e3efb9bf90a7adc94c32`
- implementation_sha: 本报告所在提交；精确 SHA 随交付消息提供（避免报告自引用哈希）。
- worktree: `/home/lijx/.codex/worktrees/auth-init-impl/RelayNest`
- branch: `codex/auth-init-impl`

## 范围与实现选择

只修改 `server/internal/auth/{administrator,password}{,_test}.go`、`server/cmd/relaynest/{main.go,admin.go,admin_test.go}`、`server/migrations/0002_admin.sql`、必要 `server/go.mod/go.sum`、`server/internal/storage/storage_test.go` 和本报告。未改 storage 实现、API、设计或其他卡。未实现 HTTP 登录、会话、推送和 WS 撤销。

依据 T5.1 固定唯一管理员，使用 `administrator.id=1` 主键加 CHECK；字段为 `username UNIQUE`、`password_hash`、`auth_generation`。用户名原样保存、区分大小写，仅拒绝空字符串；密码仅拒绝空值，无额外强度或归一化规则。数据库用户名非空检查使用 BLOB 字节长度，避免 SQLite 文本 NUL 长度规则额外拒绝非空输入。

以下为开发选择，非额外用户批准的产品政策：代次使用 SQLite INTEGER / Go int64，初始 1，达到 MaxInt64 拒绝重置；下一代次与新摘要同事务提交。CLI 使用设计中命令形态，显式要求 `--db PATH`，不猜测默认持久目录，复用 `storage.Open`。只有 init 接受必填 `--username`。命令参数解析错误为固定用法信息，不回显原始参数；数据库和输入错误也不返回原始异常。原有无 admin 子命令的 HTTP 启动和优雅退出入口保留。

终端用 `term.ReadPassword` 隐藏输入；stdin 读取一行，仅去除一个 LF 或 CRLF 结束符，不去空格，EOF 前孤立 CR 保留。密码不接受参数或环境变量。盐 16 字节、摘要 32 字节；PHC 为 Argon2id v19、64 MiB、3 次、并行 1。校验只接受固定参数、严格 Base64 和固定长度，常数时间比较，最多同时两次校验；登录限流由后续登录任务实现。

## 依赖查证

[Go 官方 Argon2 文档](https://pkg.go.dev/golang.org/x/crypto/argon2) 核实 IDKey、版本 19 和内存单位 KiB；`go list -m -json golang.org/x/crypto@v0.58.0 golang.org/x/term@latest` 核实锁定版本分别为 v0.58.0、v0.47.0，均要求 Go 1.26.0，可由本地 Go 1.27.3 构建。term 网站查询失败后，核实下载的官方模块 `golang.org/x/term@v0.47.0/term.go` 的 ReadPassword 无回显契约，并实际验证终端行为。crypto v0.58.0 的官方 go.mod 要求 x/sys v0.49.0，故必要传递升级原 v0.47.0；Go、SQLite、modernc/libc 版本未变。执行 `go mod tidy` 成功。

## 实际检查

环境：Linux/amd64，Go 1.27.3（`GOTOOLCHAIN=local`）、Node 22.23.2、npm 10.9.8、Python 3.13.15。工具均来自 `/home/lijx/.local/share/mise/installs/{go,node,python}/` 对应版本目录；Go 命令在 `server/`，npm 命令在仓库根。仅本地临时数据库，未连接现网。

| 命令或检查 | 结果 |
| --- | --- |
| `npm --prefix web ci --no-audit --no-fund` | PASS，安装锁定依赖 |
| `npm --prefix web run build` | PASS，含 typecheck；本树重新生成 gitignored webui/dist |
| `go test -race -count=1 ./internal/auth ./internal/storage ./cmd/relaynest` | 最终 PASS：auth 5.511s、storage 2.007s、CLI 2.401s |
| `go vet ./internal/auth ./internal/storage ./cmd/relaynest` | PASS，最终夹具变更后已重跑 |
| `gofmt -l server/internal/auth server/cmd/relaynest server/internal/storage/storage_test.go` | PASS，无输出 |
| `git diff --check` | PASS |
| `go build -o /tmp/relaynest-p04-01 ./cmd/relaynest` | PASS |
| Python 标准库 PTY / subprocess / sqlite3 检查真实构建程序 | PASS，见下方 |

首轮 race 的 auth/CLI 通过，storage 模拟升级持久版本断言仍为旧版本 2 而失败；已调整为新基线 2 加模拟升级后的 3，再运行上表完整 race 命令全部通过。未删除或弱化旧断言。真实 0001→0002 升级新增检查 epoch 不变、管理员空表存在及旧二进制拒绝；原整批失败回滚、持锁拒绝及重新打开台账检查仍保留。

真实 CLI 检查用 Python 3.13.15 here-document 驱动 `/tmp/relaynest-p04-01`，临时目录退出后删除。步骤：PTY 启动 `admin init --db <temp> --username Admin`；等待密码提示并检查 termios ECHO 关闭，再输入合成测试值，退出后确认 ECHO 恢复且输出不含输入值；第二次 init 拒绝；stdin 执行 `admin reset-password --db <temp>` 成功；未知密码参数拒绝且不回显参数值；Python sqlite3 查询仅有 id=1 一行、用户名未变、摘要参数正确、代次=2、schema=2。输出：`PASS real CLI + local SQLite: TTY echo off/restored, stdin reset, duplicate rejection, argument redaction, unique row, generation=2, schema=2`。首次尝试系统 `python3` 不在 PATH，后改用上述 mise 绝对路径成功；未将缺工具当作通过。

## 验收映射与后续接口

- 竞争 init / 唯一管理员 / 重复拒绝：PASS，`TestConcurrentInitIndependentStores` 在同一文件使用两个独立 Store/连接池并发；数据库约束额外拒绝第二记录。此项为真实 SQLite 的独立连接覆盖，无跨进程竞争通过宣称。
- 无明文/参数日志：PASS，CLI 无密码参数入口；`TestAdminCLI`、`TestAdminInputFailureIsSafe` 覆盖未知参数、位置参数、输入异常脱敏；真实 PTY 确认隐藏输入。
- 摘要/盐/参数：PASS，`TestPasswordHash` 覆盖独立盐、Unicode/空格、正确/错误/空密码、畸形与恶意成本参数。
- reset 原子性及边界：PASS，`TestResetAtomicAndExhaustion` 用延迟外键在提交阶段制造故障，验证摘要/代次均未变；末次递增达到 MaxInt64 后再 reset 拒绝且无变化。`TestInvalidInitAndCancelledReset` 覆盖取消、空密码失败。
- 存储升级/失败回滚/锁/旧版本拒绝：PASS，上述 race 包含完整 storage 回归。
- `TestGracefulShutdownDrainsRequest`：PASS，包含在 CLI race 包中。

交接 P04-02：`auth.ReadAdministrator(ctx,*storage.Store)` 返回 `Administrator{Username,PasswordHash,AuthGeneration int64}`；摘要仅内部校验，禁止直接向 HTTP 序列化。`HashPassword([]byte)` / `VerifyPassword([]byte,string)` 使用固定 PHC 参数。`InitAdministrator(ctx,store,username,password)`、`ResetPassword(ctx,store,password)` 返回稳定 `ErrAlreadyInitialized` / `ErrNotInitialized` / `ErrGenerationExhausted` / `ErrStorage`，无原始 SQL 异常。会话需要记录并在认证时检查管理员代次；本卡提供事务基础，未声称已撤销任何会话。管理员 username 比较应与数据库 BINARY 原样语义一致。下一迁移版本是 0003。

unresolved: []。requested_decisions: []。NOT_RUN：真机、MoviePilot、NAS/容器部署、推送、登录会话撤销、正式 AC、跨进程竞争压测及部署性能调参；这些不在本卡验证范围。待独立审计与集成，未推送。
