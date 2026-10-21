# Web 最小管理工程

本工程仅提供空管理首页，由 Go 服务同源提供 `/` 和 `/assets/*`。没有业务接口调用、登录、聊天、历史、正文或附件 UI；管理能力由后续任务实现。构建依赖仅 Vite 和 TypeScript，无 UI 框架。生产运行只需 Go 二进制，不单独部署 Node 或前端服务。

## 工具与锁文件

- Node `22.23.2`、npm `10.9.8`（使用该 Node 自带 npm），在 `mise.toml`、`package.json` 中固定；`.npmrc` 拒绝不匹配的引擎。
- TypeScript `5.9.3`、Vite `7.3.7` 在 `package.json` 精确固定；完整依赖版本与完整性摘要见 `package-lock.json`，安装使用 `npm ci`。
- Go 版本沿用 `server/go.mod` 的 `1.27.3`。
- [Vite 7 官方兼容说明](https://v7.vite.dev/guide/migration)要求 Node `20.19+` 或 `22.12+`；[TypeScript 5.9 官方说明](https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-9.html)。当前版本满足兼容条件。

## 干净克隆的构建和检查

工具链放入 PATH 后，在仓库根运行：

```sh
sh web/check.sh
```

脚本按顺序执行 `npm ci --no-audit --no-fund`、类型检查、Vite 构建、全部 Go 竞态测试、`go vet` 与 Go 格式检查。依赖首次下载需要访问 npm registry；Go 此阶段没有外部模块。也可分步执行：

```sh
cd web
npm ci --no-audit --no-fund
npm run typecheck
npm run build
cd ../server
go build -mod=readonly -trimpath -o /tmp/relaynest ./cmd/relaynest
/tmp/relaynest -listen 127.0.0.1:8080
```

访问 `http://127.0.0.1:8080/` 可见管理入口；现有 `/health/live` 为 200，未接入真实存储的 `/health/ready` 仍为 503。

Vite **只**清理并输出到 `server/internal/webui/dist/`，此目录是 `go:embed` 编译输入，已忽略，不入库。干净克隆直接执行 Go 构建/测试会因缺少 `dist` 报错，必须先完成上述 Web 构建；没有占位资源或自动联网构建。修改 Web 后必须重新构建 Web 和 Go，旧二进制不会读取磁盘上的新文件。打包/CI 后续任务也必须遵守此顺序。

相同源码及锁文件重复构建应生成相同内容与哈希文件名；审计可以对 `dist` 逐文件取摘要后再次执行 `npm run build` 比较。未知页面和 API 保留 404，不提供 SPA 首页回退或目录列表。
