# P03-02 集成 I01

日期：2026-10-22；角色：integrator；结论：PASS。目标分支 `main`，工作区 `/home/lijx/Projects/RelayNest`。本次只精确集成已独立审计的实现、审计报告并补当前组合验证，没有修改业务代码或台账。

## 精确提交映射

| 项目 | SHA |
| --- | --- |
| 原实现基线 | `6bdd6ef540494321ed6e22015212f9a93bfd6bdc` |
| 集成前 main | `491fc873657006f3f037b57175b35783a2ea388f` |
| 原实现 | `6e9171946099457aad4f7696700927bd5751e77d` |
| main 中实现 | `b9f8c74fd2424622dd3646a0addef02e6d1d288b` |
| 原独立审计提交 | `cca49567239bf034a1f65209019303a602797811` |
| main 中审计报告 / 组合验证 HEAD | `458096973405e0d5797529ee8dd195284f9b6dda` |

审计证据：[P03-02-audit-01.md](P03-02-audit-01.md)，固定实现 SHA 的结论为 PASS。源审计工作树 `/tmp/relaynest-run3/web-audit` 开始时干净。两次 cherry-pick 均无冲突；逐路径 `git diff` 确认实现全部 18 文件与原实现相同，审计报告与原审计提交相同。包含已批准的 `server/internal/httpapi/router.go` / `router_test.go` 最小接线；无需代码重审。本报告为独立元数据提交，自身 SHA 提交后交接。

## 当前组合实际验证

环境：Linux amd64；已有 mise Node v22.23.2、npm 10.9.8、Go go1.27.3；`GOTOOLCHAIN=local`。当前 main 包含 modernc.org/sqlite v1.58.0，与原 Web 审计基线不同，以下为当前组合实际执行。

| 检查 | 结果 |
| --- | --- |
| `npm --prefix web ci --no-audit --no-fund`，随后 `npm --prefix web run build` | PASS，安装 16 包；tsc 类型检查和 Vite 7.3.7 构建通过，四模块生成 HTML/CSS/JS |
| server 下 `CGO_ENABLED=1 go test -race ./...` | PASS；cmd、health、httpapi、storage、webui 五包通过，migrations 无测试文件 |
| server 下 `go vet ./...` 与 `test -z "$(gofmt -l cmd internal)"` | PASS，退出 0 |
| server 下 `CGO_ENABLED=0 go build -mod=readonly -trimpath -o /tmp/relaynest-run3/p0302-integrated ./cmd/relaynest` | PASS，退出 0 |
| 临时 Node 内置模块驱动 `/tmp/relaynest-run3/p0302-smoke.mjs`，真实二进制 cwd=/tmp，随机回环端口 | PASS；先将整个 dist 移出源码路径，首页 200/text/html 且字节等于构建 HTML，CSS/JS 字节等于构建资源，no-cache/nosniff；首页及资源 HEAD=200 空体，POST /=405 |
| 同一真实进程路由与健康检查 | PASS；`/api/v1/missing`、`/api/v1/`、`/chat`、`/history`、`/src/main.ts`、`/index.html`、`/assets/`、`/assets/missing.js`、`/assets/.git/config` 全部 404 且非 HTML；live=200、ready=503；首页无聊天输入控件 |
| 同一进程 SIGTERM | PASS，七秒内退出 0；finally 恢复本次生成的 dist |
| `git diff --check`、忽略规则、来源文件相同性核对 | PASS；dist/node_modules 均被忽略，生成物未 stage/提交 |

本次生成 HTML、CSS、JS 的 SHA-256 分别为 `134555eec33cfa5d98e63e065bdb46032ba69540873da83a68de1a094e2bd7de`、`f821e8a944558d9713eff25f5a792221e47478398b70af7d52d17683309bb1cf`、`6d52fc8dd4a8b45582b191ec3b8e11c0bed55a5d9c8b47b2d6116a477c7d5dfe`，与固定审计记录相同。未执行浏览器渲染、GitHub Actions、真实设备、fnOS 或部署，不将这些后续门槛记为通过。

## CI 前置与工作区保护

任何 Go 测试/检查/构建前，必须在同一工作空间先执行 `npm --prefix web ci --no-audit --no-fund`、`npm --prefix web run build`，生成 `server/internal/webui/dist/`；不可依赖旧 dist 缓存或提交生成物。后续 Go CI 应使用固定 Node/npm、锁文件安装，并让路径过滤覆盖 `web/**`；调用 `web/check.sh` 的 race 检查需具备 Go/CGO 环境。这里只提供交接，不实现 CI。

开始前已有 `docs/tasks/registry.json` 修改、未跟踪 `.directory` 和 `docs/tasks/reports/ORCHESTRATOR-CHECKPOINT.md` 均保留且不 stage。集成后 registry diff 与开始前逐字节一致，两个未跟踪文件 SHA-256 未变。未运行破坏性 Git 操作，未推送/发布/部署，未创建额外 agent。Go CI 工作树在本报告提交后的最终 main 精确 SHA 上另行准备，实际分支/路径在交接消息给出。
