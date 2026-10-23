# P04-02-S02-S02 认证启动接线与父验收证据

## 固定对象与边界

- 角色：开发；outcome：implemented，待独审/精确集成，非父卡 done。
- 基线：`a8cde39d73af3c47a0f271a0b2fbf0b51b1f5e23`；工作树 `/home/lijx/.codex/worktrees/auth-startup/RelayNest`，分支 `codex/auth-startup`。实现 SHA 由提交后交接给出，不预写自身 SHA。
- 接管唯一继承 dirty `server/cmd/relaynest/main.go`，开始 SHA256 `413893b9846ca4e44a586272b2ddf12e1d2518e6d33a35733ef40ac1ef086bf0`；旧 auth-http 树 main/checkpoint 未改。最终仅提交本报告及卡内三个 Go 文件。
- 继承 P04-01、P04-02-PLAN01、P04-02-S01、P04-03-S01、S02-PLAN01 的已审集成前置，不因子卡拆分免除。依据原父卡、S02、原 checkpoint、T7.1、A5 和冻结 API。无 SQL/API/依赖/业务路由改动，Bot 正式 0004 保持不变。

## 实现与本地证据

[main.go](../../../server/cmd/relaynest/main.go) 的 `main → runServer → authenticationDependencies → serve` 使用显式 `-db`、`-external-origin`、可重复 `-trusted-proxy`，缺配置拒绝，无默认 DB/域名/代理。`runServer` 返回到 main 后才允许 os.Exit，数据库关闭错误转换为固定代码；监听失败和正常关闭均释放资源。原 listen 默认 `127.0.0.1:8080`、admin 分派和 graceful shutdown 排空保留；ready 仍 `health.New(nil)`。

[main_test.go](../../../server/cmd/relaynest/main_test.go)：

- [TestMainProcessAuthentication](../../../server/cmd/relaynest/main_test.go#L98) 通过测试二进制子进程直接调用生产 main，真实 admin init stdin、SQLite、TCP HTTP、生产参数解析与来源/handler/router 注入。显式双代理参数中第一项匹配、最后一项不匹配，证明重复参数累积；对照不配代理时相同转发头 403，环回不默认可信。合法代理 HTTPS 登录 200/Secure+HttpOnly Cookie/GET session 200，缺 HTTPS 403、ready 503，SIGTERM 后 `shutdown_complete`、监听拒绝连接、数据库可重开。
- [TestStartupConfigurationAndCleanup](../../../server/cmd/relaynest/main_test.go#L227) 拒绝缺 db/origin、HTTP origin、无效代理和多余位置参数；监听占用报 listen_failed，预取消启动失败；Linux `/proc/self/fd` 断言无本测试目录数据库 FD。
- [TestRunServerCancellationClosesDatabase](../../../server/cmd/relaynest/main_test.go#L266) 实际启动后取消，监听和数据库 FD 全部关闭。
- [TestGracefulShutdownDrainsRequest](../../../server/cmd/relaynest/main_test.go#L22) 保留在途请求排空断言。

[router_test.go](../../../server/internal/httpapi/router_test.go) 的 [TestExactAuthenticationRoutes](../../../server/internal/httpapi/router_test.go#L75) 覆盖准确三条 method/path、Go GET 对应 HEAD、错误 method 405、尾斜线/子路径/未知 API 404 且非 HTML；原 health/static 回归继续运行。该 probe 仅证明路由匹配，实际认证由上述进程与既有 HTTP/SQLite 套件证明。

## 原 S02 七项矩阵

下表本地 PASS 是实现证据，不替代本卡 pending 的独审/集成或父组合验收。S01/S02-S01 的精确证据链在后文。

| 原验收 | 本地结果与真实函数/断言 | 独审/集成 |
| --- | --- | --- |
| 三路由、双端 DTO、204/Cookie/no-store、未知 API | PASS：S02-S01 [TestHTTPLifecycle](../../../server/internal/auth/http_test.go#L26)、[TestHTTPLoginDTOContract](../../../server/internal/auth/http_test.go#L668)；本卡 [TestExactAuthenticationRoutes](../../../server/internal/httpapi/router_test.go#L75)、[TestMainProcessAuthentication](../../../server/cmd/relaynest/main_test.go#L98) | S02-S01 PASS/已集成；本卡 pending |
| 统一错误、无管理员/错密/过期撤销旧代次/Bot/CSRF 来源拒绝、不泄密 | PASS：S02-S01 [TestHTTPSourceAndInput](../../../server/internal/auth/http_test.go#L210)、[TestHTTPRejectionsHaveNoEffects](../../../server/internal/auth/http_test.go#L398)、[TestHTTPInputRejectionsHaveNoEffects](../../../server/internal/auth/http_test.go#L627)、[TestHTTPClientDeadlinesAndLogout](../../../server/internal/auth/http_test.go#L447)、[TestHTTPResetLoginOrdering](../../../server/internal/auth/http_test.go#L510) | S02-S01 PASS/已集成 |
| 真实来源模块、代理 Secure、合法直连、伪造畸形冲突头/缺失跨源端口/CSRF 与运行接线 | PASS：S02-S01 [TestHTTPRealSourceComposition](../../../server/internal/auth/http_test.go#L565)、[TestHTTPSourceAndInput](../../../server/internal/auth/http_test.go#L210)；本卡 [TestMainProcessAuthentication](../../../server/cmd/relaynest/main_test.go#L98) 两进程对照 | S02-S01 PASS/已集成；运行接线 pending |
| 双端绝对/空闲期限续期、重启、当前注销隔离、重置竞争、真实 SQLite | PASS：S01 [TestSessionDeadlines](../../../server/internal/auth/session_test.go#L59)、[TestSessionPersistenceSecretsAndIsolation](../../../server/internal/auth/session_test.go#L111)、[TestSessionResetCreateCompetition](../../../server/internal/auth/session_test.go#L226)、[TestSessionRevokeRenewCompetition](../../../server/internal/auth/session_test.go#L270)、[TestSessionResetBeforeCreateTransaction](../../../server/internal/auth/session_test.go#L322)；S02-S01 [TestHTTPLifecycle](../../../server/internal/auth/http_test.go#L26)、[TestHTTPClientDeadlinesAndLogout](../../../server/internal/auth/http_test.go#L447)、[TestHTTPResetLoginOrdering](../../../server/internal/auth/http_test.go#L510) | S01、S02-S01 PASS/已集成 |
| 父三项安全验收逐项映射 | PASS（本地证据）：见下方父矩阵前三项，未遗漏期限/撤销/摘要/代次；父组合收口 pending | 已有来源 PASS，本卡 pending |
| auth/httpapi/cmd race/vet 与契约 | PASS：本报告检查表；不是历史检查替代 | 本卡 pending |
| 独立提交、精确 SHA、独审与集成 | 本次独立实现提交，SHA 在交接；本卡独审 pending、集成 pending | pending，不能 done |

## 父 P04-02 五项矩阵

| 原验收 | 函数、真实证据与结论 | 尚余门槛 |
| --- | --- | --- |
| 错误统一 | S02-S01 [authError](../../../server/internal/auth/http.go#L82)、[TestHTTPRejectionsHaveNoEffects](../../../server/internal/auth/http_test.go#L398)/[TestHTTPSourceAndInput](../../../server/internal/auth/http_test.go#L210)/[TestHTTPResetLoginOrdering](../../../server/internal/auth/http_test.go#L510) 统一 401/403 与空副作用；本地 PASS | 本卡及父组合审核 pending |
| 过期/撤销/摘要存储 | S01 [TestSessionDeadlines](../../../server/internal/auth/session_test.go#L59)、[TestSessionPersistenceSecretsAndIsolation](../../../server/internal/auth/session_test.go#L111) 验期限边界/摘要不存原文/重启/单会话注销；S02-S01 [TestHTTPLifecycle](../../../server/internal/auth/http_test.go#L26)、[TestHTTPClientDeadlinesAndLogout](../../../server/internal/auth/http_test.go#L447) 消费真实 SQLite，PASS | 父组合审核 pending |
| 重置代次使会话失效 | S01 [TestSessionResetCreateCompetition](../../../server/internal/auth/session_test.go#L226)、[TestSessionResetBeforeCreateTransaction](../../../server/internal/auth/session_test.go#L322)；S02-S01 [TestHTTPResetLoginOrdering](../../../server/internal/auth/http_test.go#L510)、[TestHTTPLifecycle](../../../server/internal/auth/http_test.go#L26) 旧代次拒绝，PASS | 父组合审核 pending；WS/推送外部门槛仍 NOT_RUN |
| 受影响检查逐项结果，缺必需则 blocked | 本卡三包 race/vet、契约、格式/diff PASS，无缺失必跑检查 | 本卡独审 pending |
| 仅本任务提交、精确基线/实现/证据/遗留、独审集成才 done | 本报告+提交后 SHA 交接；来源已审集成，本卡 pending | S02/P04-02 仍需原前置和全部子卡完成后组合收口 |

精确证据链：

- S01 实现 `1da5e72aed508e01fb0d172b79fba0052af61a52`，[实施](P04-02-S01-implementation.md)、[A02 PASS](P04-02-S01-A02.md) 提交 `7e6a8d5cfd59d6069711fd3106ebc2ec718933bb`、[I01](P04-02-S01-I01.md) 集成组合 `9269876a5b57f18cc43d8100e88dc0f4a0b3b4ca`。
- S02-S01 最终实现 `e57095f5ec71802cbc321de849d97b572447567c`，包括 F01 DTO 修复；[实施](P04-02-S02-S01-implementation.md)、[F01](P04-02-S02-S01-F01-implementation.md)、[A02 PASS](P04-02-S02-S01-A02.md) 提交 `246adb19cfcf082cbcaba4364c2a1479219a3272`、[I01](P04-02-S02-S01-I01.md) 提交 `f8a2304a407c589261995f866ac04c83c12b24e4`、main 固定组合 `4e74575fc1ed4c3c597ec8eee7feb3b830cd9a15`。不将 A01 的 CHANGES_REQUIRED 历史写成 PASS。
- 本卡精确基线见首节；实现 SHA 在交接，本卡独审/精确集成均 pending。

## 实际检查

Linux amd64；Go 使用绝对路径 `/home/lijx/.local/share/mise/installs/go/1.27.3/bin/go`、`GOTOOLCHAIN=local`，实测 go1.27.3。以下均实际执行：

| 命令（相对仓库根，Go 包命令在 server） | 结果 |
| --- | --- |
| `go test -race ./internal/auth ./internal/httpapi ./cmd/relaynest` | PASS，最终 auth/httpapi cached，cmd 7.440s；此前非缓存 auth 20.444s、httpapi 1.076s 通过 |
| `go vet ./internal/auth ./internal/httpapi ./cmd/relaynest` | PASS |
| 三允许 Go 文件 `gofmt -w` / `gofmt -l` | PASS，无格式残留 |
| `git diff --check` | PASS |
| web `npm ci`、`npm run build` | PASS；新树缺 embed，锁 Node 22.23.2/npm 10.9.8 生成 gitignored dist，没有提交产物 |
| Python 3.13.15 `-m venv /tmp/relaynest-health-contract-locked`；该环境 `-m pip install --require-hashes -r server/api/requirements.txt` | PASS；原环境不存在，按 README 用现有 mise Python 和原 hashes 锁恢复，无升级重锁 |
| `/tmp/relaynest-health-contract-locked/bin/python -m openapi_spec_validator server/api/openapi.yaml` | PASS，OK |
| 同 Python `server/api/validate.py` | PASS，115 schemas；568 valid + 661 invalid examples；39 operations/security |

首轮测试夹具遗漏必填 device_name，得到 422；补齐冻结 DTO 后通过。初始试用默认 python3 不存在，改用锁定 Python，未虚报失败命令。测试未修改只读实现或弱化验收。

## 遗留与交接

本地真实进程 HTTP/SQLite 已测；来源 helper 与直接 TLS 由既有真实 HTTP 测试覆盖。真实反向代理部署、NAS/手机/MP/WS/推送与正式 AC 全部 NOT_RUN；本地转发头测试不代表真实 NAS/代理部署。ready 必需挂载等探针未齐，仍必须 503。P04-03 完整业务来源安全、P04-04 限流/公共日志不扩入本卡。

无新增只读模块缺陷；无待用户决策。无 push/部署/新聊天/子 agent。另一个 agent 对固定实现 SHA 独审 PASS 后精确集成，再由调度组织原 S02 与父 P04-02 组合验收；本报告不标父任务 done。
