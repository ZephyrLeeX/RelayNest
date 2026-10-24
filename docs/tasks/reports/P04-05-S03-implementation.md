# P04-05-S03 设备 HTTP、装配与组合验证实施报告

角色：developer；结果：implemented，等待另人固定 SHA 独审与最终集成，未标 done。

## 基线与恢复链

- 唯一业务审查基线：`79c4601deb71d5865e3539ddc7da7f138737ee15`。
- 保存的原实现草稿：`03b95101c3bf0671118aa54b1c70f7b56d9389ef`，唯一父为上述基线。开始时核对 `main.go=243ab8c`、`devices/http.go=586d233`、`router.go=abdeaa0`，与保存 blob 一致；草稿无已交付测试和实施报告。
- 派发 HEAD：`de9f2ca91536f442a4230a68dd653053e6524a17`，仅恢复报告。工作树 `/home/lijx/Projects/RelayNest-recovery-20261024/device-admin`，分支 `codex/device-admin-recovery-20261024`，开始时干净。
- 本次保留并补齐三源码草稿，没有重复另造实现。业务完整组合必须包含 snapshot `03b9510` 和本报告所在后续实现提交，审计范围始终为 `79c4601..最终实现SHA`，不能仅审后续提交或以恢复提交作为业务基线。
- `reports/WORKSPACE-RECOVERY-DEVICE-LIFECYCLE-01.md` 原样只读，未再次 stage，也未顺手纠正其中目录。最终 I 精确选择业务组合，不能直接把恢复分支整体并入 main。

## 实现与消费接口

`devices.NewHandler(*auth.SessionStore, *auth.RequestSecurity)` 消费现有公共身份/来源安全入口及事务仓储，不复制认证 SQL。`httpapi.DeviceHandlers` 只声明 `ListDeviceSessions/RevokeDeviceSession`，真实 mux 挂载 `GET /api/v1/devices` 与 `DELETE /api/v1/devices/{id}/session`。cmd 的 managementHandlers 在装配层适配设备能力，避免 auth 已依赖 httpapi 时的反向导入循环；同一次启动共享 SessionStore、RequestSecurity 和 OriginPolicy。已有 Bot 三管理路由、auth/health/static 与启动清理逻辑保留。

GET 默认 50、最大 100；拒绝显式 0/负数/非法或重复 limit、空/非法或重复 cursor。按 Session.id 分页，保留同安装多个 Session；通过显式七字段与 administrator.username 投影输出冻结元数据。仓储同事务校验 actor、读取及成功续期。无 token、摘要、CSRF 或 provider 数据输出路径。

DELETE 校验 UUIDv4 路径，畸形路径 ID 为 400；合法未知、安装 ID、过期或已撤目标为 404 RESOURCE_NOT_FOUND。拒绝任何正文（包含空白），只按 Session.id 撤销。事务归属/有效性检查、续期与提交后精确 ID hook 均沿 S02。成功 204 空正文/no-store；只有成功自撤 Web 清 Cookie，沿实际来源解析保留 Secure/HttpOnly/SameSite；Android 自撤及远程 Web/Android 目标不清 Cookie。401/403/503 使用公共安全实现；拒绝不续期、不改目标、不通知。

未更改 auth、storage、origin、API、Session、SQL、迁移、依赖或 registry；未引入尚未冻结的 G1 header/instance 字段。

## 验收与真实证据

| 验收 | 结果与定位 |
| --- | --- |
| 真实启动注入、登录和 mux | PASS：`cmd/relaynest.TestDeviceHTTPProductionAssembly` 使用 authenticationDependencies、真实认证 handler/TLS server 与文件 SQLite，真实 Web A、同安装 Android S1/S2、另一安装 B 登录 |
| 列表是 Session 页、不按安装去重 | PASS：同测试列表四会话；limit=1 遍历精确四独立 ID。`devices.TestDeviceHTTPBoundedPagesAndExpiredTarget` 103 会话验证默认50/上限100；同安装101会话保留 |
| 父原验收 A 撤销不影响 B | PASS：A 撤 S1，S2/B/A 继续有效；S1 旧凭据 401；同安装重新登录新 ID，重试旧 ID 404，新会话保持有效 |
| 父原验收旧会话后续请求失败 | PASS：S1、Android S2 自撤、Web A 自撤后均 401；关闭文件库后重新启动装配，S1/A 仍401，B和新登录会话仍200 |
| 父原验收无 Token 返回 | PASS：真实登录得到的会话 Token/CSRF、真实创建 Bot 的 API Token/Webhook Key 均不在列表；逐项七字段/administrator单字段断言，读取真实 token_hash 的 hex 值验证未泄露。provider 尚无真实数据，严格字段投影证明本接口不输出扩展秘密 |
| Bot 凭据不授管理权 | PASS：同生产装配测试，真实 Bot 两种凭据分别请求 GET/DELETE 均401，前后 Session id/revoked/idle_expires_at 状态一致 |
| 无正文/分页/path/401/403/404/503 | PASS：`devices.TestDeviceHTTPRejectedRequestsDoNotMutate` 对未知凭据、坏来源/缺CSRF、正文/空白、畸形ID/合法未知/安装ID及非法分页验证状态、错误码、no-store、无Cookie与真实库不变。`TestDeviceHTTPCommittedHookAndUnavailable` 关闭库后 GET/DELETE 均503；`httpapi.TestExactDeviceRoutes` 核方法、尾斜杠、额外路径及 PathValue 传递 |
| 自撤/远程 Cookie 与来源 | PASS：生产装配验证直接TLS Web自撤安全清Cookie、Android自撤无Cookie；`TestDeviceHTTPRemoteWebDoesNotClearCookie` 另一个Web目标无清Cookie；`TestDeviceHTTPSelfRevokeThroughTrustedProxy` 明文内网请求携可信HTTPS代理字段及Referer成功自撤，HTTP代理来源403且不改库 |
| S02 hook 组合 | PASS：真实 mux + RequestSecurity/SessionStore 回调重读真实库，必须看见已提交 revoked=1；只通知精确目标，重试404不重复通知，logout同路通知Web身份；失败矩阵不通知 |
| 既有行为与生命周期 | PASS：完整 cmd/auth/httpapi 测试保留 Bot 创建/列出/修改、认证来源/CSRF/事务复核、health/static、真实子进程认证、优雅关闭、启动失败资源释放及取消关闭；新增设备关闭重开持久验证 |
| 真实 WS/provider/Android 手机 | NOT_RUN：无真实WS/provider映射及手机运行时，本卡只证明提交后失效通知和本地持久会话失效，不声称断开真实连接、provider原子解绑或手机表现 |

## 实际检查

环境：Linux amd64，本地临时文件 SQLite；Go `/home/lijx/.local/share/mise/installs/go/1.27.3/bin/go`，`GOTOOLCHAIN=local`，go1.27.3、CGO_ENABLED=1；modernc.org/sqlite v1.58.0，实际 `SELECT sqlite_version()` 返回 **3.53.4**。HTTP 包测试使用真实 mux/存储，cmd 组合含进程/本机TLS请求，不依赖外网。

- Web：PATH锁 `/home/lijx/.local/share/mise/installs/node/22.23.2/bin`，Node v22.23.2、npm10.9.8；`cd web && npm ci && npm run build` PASS，生成 Go embed 必需 dist，产物未提交。
- `cd server && go test -count=1 ./internal/devices ./internal/auth ./internal/httpapi ./cmd/relaynest` PASS。
- 同四包 `go test -race -count=1` PASS。
- 同四包 `go vet` PASS，无诊断。
- `go test -count=1 -v ./internal/devices -run '^TestDeviceHTTPSelfRevokeThroughTrustedProxy$'` PASS，记录实际 SQLite 版本。
- 六个本卡 Go 文件 `gofmt -l` 无输出；`git diff --check` PASS。

迭代中曾修正测试夹具的 storage.Config 类型及204专用断言；仅路径误用的一次命令显示 no tests to run，未计入通过证据。最终使用明确测试名称及全包非缓存/race执行，未弱化既有断言。

## 交接与限制

实现提交 SHA 由提交后的交接给出，报告不预写自身 SHA。精确审查须同时覆盖恢复 snapshot 中三源码差异和本次测试/补齐；恢复报告不属于业务实现。当前父验收具备本地证据，父 P04-05 收口仍需另人 A/I 与前置精确核对；真实 WS/provider/手机留后续对应任务。本卡未 push、部署、创建新聊天或子 agent。
