# P03-02 Web 最小工程与内嵌产物实施报告

日期：2026-10-22。角色：developer。基线：`6bdd6ef540494321ed6e22015212f9a93bfd6bdc`；工作树：`/tmp/relaynest-run2/web`；分支：`codex/web-run2`。实施 SHA 提交后交接，不在本报告预写自身 SHA。结果：implemented，独立审计/集成待调度。

## 变更与边界

- `web/` 新建无框架 TypeScript/Vite 管理入口，页面只有产品标题与“管理入口准备中。”；TS 只设置就绪标记。无业务请求、聊天、正文、附件、历史或登录实现。
- 依赖精确固定为 Vite `7.3.7`、TypeScript `5.9.3`；`package-lock.json` v3 固定全部传递依赖、registry 地址和完整性摘要。`npm ls --depth=0` 确认仅此两个直接开发依赖。
- 工具使用现有 mise Node `22.23.2` 自带 npm `10.9.8`，无安装或升级工具链；web/mise.toml 固定 Node，package.json 的 engines 和 packageManager 固定 Node/npm，`.npmrc` 启用 engine-strict。Go 沿用 `server/go.mod` 的 `1.27.3`，没有改动 Go 模块。
- 核对 [Vite 7 官方兼容说明](https://v7.vite.dev/guide/migration)与 [TypeScript 5.9 官方说明](https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-9.html)；`npm view vite@7 version engines --json` 及 `npm view typescript@5.9.3 version engines --json` 确认实际发布版本与 Node 引擎约束。选择现有 Node 满足的普通稳定版本，未使用浮动 latest。
- Vite 输出专用目录 `server/internal/webui/dist/`，Go `//go:embed dist` 内嵌资源，由 `webui.New()` 提供；目录与 node_modules 均忽略，不提交产物。缺少 dist 时明确编译失败，不以内置占位页面掩盖缺构建步骤。
- 调度明确批准必要等价接线路径：`server/internal/httpapi/router.go` 与 `router_test.go`。沿用 `httpapi.New(h)` 和已有 cmd 入口，仅增加 `GET /{$}` 和 `GET /assets/`（兼容 HEAD），没有更改 cmd 或健康行为。不存在路径返回 404，不给未知 API 返回 HTML；不列目录、不提供源文件。静态响应 no-cache、nosniff。
- `web/README.md` 给出干净克隆构建顺序和启动方法，`sh web/check.sh` 是可执行的一键检查入口。未改共同 API、SQLite、CI、台账、需求或其他项目；无发布、远端推送或部署。

## 实际检查

环境：Linux amd64，Go `1.27.3`，Node `22.23.2`，npm `10.9.8`，Python `3.13.15`（仅临时本地冒烟驱动，工程运行不依赖 Python）。PATH 使用 `/home/lijx/.local/share/mise/installs/{node/22.23.2,go/1.27.3}/bin`。npm 下载使用官方 registry。

| 命令/动作 | 结果与证据 |
| --- | --- |
| `npm --prefix web install --package-lock-only --no-audit --no-fund` | PASS，生成 lockfile，不升级工具链 |
| `gofmt -w server/internal/httpapi server/internal/webui`；`sh web/check.sh` | PASS；npm ci 安装 16 包，tsc 无错误，Vite 构建 4 模块，Go 四个包的竞态测试通过，vet/格式退出 0 |
| `cd server && go mod tidy -diff` | PASS，无 Go 模块差异 |
| `git diff --check` | PASS，无空白错误 |
| `git check-ignore web/node_modules server/internal/webui/dist/index.html` | 两者均命中忽略规则 |
| `rg -n '<(input\|textarea\|form)\|contenteditable\|fetch\(\|WebSocket\|/api/\|/chat\|/history' web/src web/index.html` 的等价扩展正则扫描与逐文件阅读 | 无匹配；只有静态标题/提示和就绪标记，无聊天/历史/网络入口 |
| 临时目录复制 `git ls-files --cached --others --exclude-standard` 所列源码，排除所有忽略产物；创建全新 GOCACHE/GOMODCACHE，设置 GOTOOLCHAIN=local/GOPROXY=off | 干净源直接 `go build ./cmd/relaynest` 按预期失败，stderr 明确包含 `pattern dist`（缺少编译输入） |
| 对上述干净源执行 `sh web/check.sh`（CGO_ENABLED=1） | PASS，实际重新安装依赖、构建 Web，四包竞态测试通过，vet/格式通过；Go 无外部模块，未依赖旧产物或 Go 缓存 |
| 同一干净源再次 `npm run build`，逐文件比较 SHA-256 | PASS，index.html、CSS、JS 三个文件名称和内容摘要一致，见下表 |
| `CGO_ENABLED=0 go build -mod=readonly -trimpath -o <临时目录>/relaynest ./cmd/relaynest` | PASS，构建单一静态 Go 二进制 |
| 删除该干净源的 dist 后，从仓库外目录启动上述二进制，回环随机端口 HTTP 冒烟 | PASS：首页 200、text/html，引用 CSS/JS 均 200 且带 nosniff；首页无 input/textarea/form/contenteditable；`/api/v1/missing`、`/chat`、`/history`、`/src/main.ts`、`/assets/`、缺失资源均 404；live=200、ready=503 |
| 对真实进程发送 SIGTERM | PASS，七秒内退出 0；临时目录自动清理 |

首次执行一键脚本时，类型检查、构建、竞态测试和 vet 已通过，但尚未 gofmt，末尾格式断言退出 1；随后格式化，完整脚本及干净源码检查均重跑退出 0。未把首次失败记录成通过。上述临时 Python 驱动 `/tmp/relaynest-p0302-verify.py` 未入库，自动化长期复验入口是 `web/check.sh` 和 Go 内嵌/路由测试；无浏览器自动化或实际发布验证。

| 文件 | 重复构建相同的 SHA-256 |
| --- | --- |
| `index.html` | `134555eec33cfa5d98e63e065bdb46032ba69540873da83a68de1a094e2bd7de` |
| `assets/index-BC4YGnT-.css` | `f821e8a944558d9713eff25f5a792221e47478398b70af7d52d17683309bb1cf` |
| `assets/index-o9XC6usj.js` | `6d52fc8dd4a8b45582b191ec3b8e11c0bed55a5d9c8b47b2d6116a477c7d5dfe` |

Go 测试覆盖真实内嵌资源逐字节响应、构建入口而非 TS 源码、缺资源/目录/非法路径 404、根首页 GET/HEAD、非法方法 405、聊天/历史/未知 API 404，并保留健康路由原有断言。真实二进制冒烟另验证路由已接线、移除磁盘资源仍可用。

## 验收与后续交接

- 类型检查/构建/静态资源冒烟：PASS，见上述实际命令、Go 测试、移除 dist 的真实进程证据。
- 无聊天入口：PASS，最小源码审阅/扫描、首页控件检查及聊天/历史路径 404；不声称未来业务阶段已实现。
- 受影响检查：PASS；独立提交 SHA 在交接中提供；独立审计/集成仍由调度安排。

CI/干净克隆必须先运行 `npm --prefix web ci --no-audit --no-fund`、`npm --prefix web run build`，再执行任何 Go 构建/测试；或者仓库根运行 `sh web/check.sh`。生成目录精确为 `server/internal/webui/dist/`。直接运行 Go 检查缺 dist 会失败，这是必须遵守的构建依赖；不能缓存旧 dist 冒充通过。后续 CI 卡应显式建此前置。本卡无 CI 修改权限。

后续接口：`webui.New() http.Handler`；静态入口 `/`、资源 `/assets/*`。不存在 SPA 回退；当前无管理业务接口或会话，后续不得在此加入正文/历史/聊天入口。ready 未配置真实存储仍为 503。遗留：没有浏览器渲染/截图、fnOS 或部署验证（本卡未要求），没有其他实现阻塞或用户决策请求。
