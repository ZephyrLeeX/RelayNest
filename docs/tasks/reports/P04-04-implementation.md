# P04-04 登录防护与敏感日志过滤实施

- 角色：developer；基线 `96f6fff8340957104fb627606640adc2bc572eae`。
- 工作树 `/home/lijx/.codex/worktrees/login-protection/RelayNest`，分支 `codex/login-protection`；开始工作区 clean，本卡唯一 writer。
- 本报告随实现提交，不预写自身 SHA。等待独立审计和集成，不宣称 done。

## 实现与契约

1. `password.go` 保持 Argon2id 固定 64 MiB、3 次、并行度 1、随机盐与常量时间比较。保留原全局 `verificationSlots` 两槽，原 `VerifyPassword` 仍阻塞获取并释放槽；新增内部非阻塞登录适配，与原 API 共用同一槽、调用不再获取槽的内部计算，避免重复获取死锁。登录繁忙立即 429，无新增等待队列；CLI 初始化/重置的 HashPassword 不变。两槽约束针对验证内存，不包括独立 HashPassword 管理操作。
2. 登录来源/DTO 校验后、读管理员及 Argon 前按 `RequestSource.ClientIP` 申请失败预算。普通实现参数：固定一分钟窗口、五次失败、最多 1024 个 IP；在途占位与失败原子结算，避免并发穿透。成功/繁忙/存储失败/取消不记密码失败，成功不清空既有失败。过期无在途键清理；有在途键保留，跨窗口结算保守计入当前窗口。全表满时保守 429，一分钟后可重试，不驱逐活跃键放大攻击。
3. `OriginPolicy.Resolve` 直接连接返回 peer 的 ClientIP；只有显式可信 peer 才读取且严格验证单值转发三头，使用校验后的 X-Forwarded-For。新代码不自行解析头，也不改变策略。非可信 peer 换伪造转发地址仍共享一个预算；可信代理上的不同校验客户端可分别计数。同一 NAT IP 的用户共享限制；代理若统一填同一客户端 IP，也共享预算，属于按 IP 防护的保守可用性代价。端口变化不改变键。
4. 繁忙/失败限流均复用冻结 Error 外形，稳定 `RATE_LIMITED`、HTTP 429、`Cache-Control: no-store`、正整数 `Retry-After`；沿用既有 Error 的 retryable 语义，未改 API 文件。未知用户名与错密码均真实执行相同 Argon 路径，返回既有统一 401；限流键/预算也不依赖用户名。
5. 新增公共 `httpapi.NewSafeLogHandler`：只允许 request_id/entity_id、status、duration、error_code 的约束标量，原始消息替换为固定 event；不解析分组、不调用任意 LogValuer/Stringer、不输出 error/请求对象/正文。WithAttrs/WithGroup 同样过滤；字段名采用封闭白名单，未开放任意 *_id。标识符必须为服务端生成值，禁止调用者把用户输入伪装成白名单标识符。登录 NewHandler 创建经过该边界的 logger，实际登录只写服务端耗时；不修改全局 slog 设置，也不声称其他未来直接使用原始 logger 的代码会自动被过滤。当前 main 的固定启停日志不修改。

未改 Origin/CSRF、严格 JSON、Cookie/Bearer 通道、重置代次、会话续期、API/SQL/依赖/main/registry。实现共四个主要文件及两个对应测试文件。

## 验证

环境：Go `/home/lijx/.local/share/mise/installs/go/1.27.3/bin/go`，`GOTOOLCHAIN=local`；Node 22.23.2/npm 10.9.8，经 Node 安装目录加入 PATH。

- 根目录 `npm ci --prefix web`、`npm run build --prefix web`：PASS，生成当前 worktree embed 必需资源，资源未提交。
- server 下 `go test -count=1 ./internal/auth ./internal/httpapi`：PASS（首次实现局部检查）。
- server 下 `go test -race -count=1 ./...`：PASS（退出 0；auth 24.825s、httpapi 1.187s、CLI 7.900s，其他服务包均通过）；覆盖 auth/httpapi/CLI 等全服务包，包含现有 strictJSON、来源、CSRF、固定凭据通道、重置排序及失败不续期测试。
- server 下 `go vet ./...`：PASS。
- 修改文件 gofmt；`git diff --check`：PASS。
- 新增验证：固定窗口边缘、过期清理、1024 键上限；100 goroutine 竞争只能取得五个失败预算；24 并行真实 HTTP 登录的统一失败或 429；真实摘要槽占满时入口立即 429，释放后不残留锁；未知账号/错密码、无 Cookie 的 429 与 no-store/Retry-After；伪造头不能分散 IP 预算；可信代理同客户端换端口仍限流、不同客户端可登录；已取消 context 不成功、不泄漏在途/失败计数。
- 日志验证覆盖 WithAttrs、WithGroup、嵌套组、禁止执行的 LogValuer、原始消息，以及 password/Cookie/Bearer/CSRF/provider token/正文哨兵；另捕获真实 Login 日志，注入登录字段/伪造头中的哨兵，只出现允许耗时，不出现哨兵。
- 锁定 API Python `/tmp/relaynest-health-contract-locked/bin/python` 不存在，完整 API 契约矩阵 NOT_RUN；未重建/升级环境，API 未改，不将此项写成 PASS。
- 过程中一次准备修改的命令因 cwd 路径错误未写入，随后的旧版本 race 运行被主动中断；不计为最终通过证据。修正路径后重新执行最终完整检查。

## 独审提示与遗留

重点复验新 password.go 的固定参数、CLI 与阻塞/非阻塞槽共用无死锁，以及当前认证边界；不得沿用旧审计自动 PASS。限流计数为单进程内存，进程重启清空，不引入数据库或多实例共享设施。固定窗口边界允许相邻窗口突发；并发与五次失败预留仍有限。攻击者使用大量 IP 可耗尽键容量并导致保守拒绝；这是有界内存的已说明取舍，无用户名存在性分支。尚待独立审计及集成，本实现不对外 push/deploy。
