# P03-01 Go 最小工程与健康入口实施报告

日期：2026-10-21。角色：developer。基线：`b0b614723d8055247d77fbeaa9c6dd88214d3c27`；工作树：`/tmp/relaynest-run2/server`；分支：`codex/server-run2`。实施提交 SHA 在提交后交接，不在本报告预写自身 SHA。

## 变更和边界

- 新建 `server/go.mod`、`cmd/relaynest`、`internal/httpapi`、`internal/health` 及对应测试。单进程、标准库 `net/http`，无外部模块，因此无 `go.sum`。
- `go.mod` 固定 `go 1.27.3`。基线没有 Go 工程/版本锁，使用现有 mise 安装的 Go 1.27.3，没有安装或升级工具链。实际版本输出 `go version go1.27.3 linux/amd64`；初始化时查阅 [Go 1.27 官方发布说明](https://go.dev/doc/go1.27)，该版本线保持 Go 1 兼容承诺。
- `-listen` 默认 `127.0.0.1:8080`。仅注册 `GET /health/live` 与 `GET /health/ready`（标准库同时支持 HEAD），其他业务路由不存在。
- live 固定返回 `200 ok\n`；ready 未配置依赖、检查失败、请求取消或正在关闭时返回 `503 unavailable\n`。探针成功才返回 `200 ok\n`。固定正文，不带版本、路径、探针错误或管理信息；禁止缓存。
- 数据库和附件卷尚未实现，进程显式传入空探针，ready 保持 503。后续存储任务须组合真实 DB/挂载检查并注入 `health.New(Probe)`，探针须响应 context；本卡没有模拟真实资源健康。探针 context 限时一秒。
- SIGTERM/SIGINT 触发撤销 ready、停止监听、最多五秒排空在途请求，超时强制关闭并以失败退出。HTTP 头读取限时五秒、空闲连接限时六十秒、头大小 16 KiB。启动/停止错误仅记录固定错误码，不记录原始错误或请求内容。
- 不变更共同 API、迁移、其他工程、需求文档或任务台账，没有部署/推送/发布。

## 实际验证

环境：Linux amd64；Go 1.27.3（`/home/lijx/.local/share/mise/installs/go/1.27.3/bin`）；Python 3.13.15（仅用于本地冒烟，`/home/lijx/.local/share/mise/installs/python/3.13.15/bin/python3`）。下列 Go 命令在 `server/` 中使用上述绝对路径执行。

| 命令/动作 | 结果 |
| --- | --- |
| `gofmt -w cmd internal`；随后 `test -z "$(gofmt -l cmd internal)"` | PASS，格式一致 |
| `go test -race ./...` | PASS；cmd/relaynest 1.082s，health/httpapi 各 1.012s，无竞态报告 |
| `go vet ./...` | PASS，退出 0 |
| `go mod tidy -diff` | PASS，退出 0，无锁文件差异 |
| `git diff --check` | PASS，无空白错误 |
| Python 创建全新临时 GOCACHE/GOMODCACHE；`GOTOOLCHAIN=local GOPROXY=off CGO_ENABLED=0 go build -mod=readonly -trimpath -o <临时目录>/relaynest ./cmd/relaynest` | PASS，空缓存离线干净构建；产物在仓库外，临时目录自动删除 |
| 对干净构建产物分别执行 SIGTERM、SIGINT 真实进程冒烟 | 两次均 PASS：live=200 固定正文；ready=503 固定正文；收到信号七秒内退出 0；停止日志包含 shutdown_complete；监听端口关闭 |

自动测试覆盖：探针成功/失败/缺失/停止状态，错误包含模拟敏感路径和 token 时仍只输出固定正文，请求取消和一秒 deadline 传递，健康路由/非法方法/不存在路径；真实 TCP 在途请求在取消服务上下文后仍可完成响应，然后监听关闭。冒烟只使用回环地址与临时随机端口，不连接外部服务。

第一次调用冒烟脚本时 shell 找不到 `python3`（退出 127，脚本未执行），之后核查并使用已有 mise Python 绝对路径，完整重跑成功。没有将第一次失败写为通过。

## 完成标准与后续接口

| 卡内条目 | 证据/结论 |
| --- | --- |
| 干净构建 | PASS，空缓存、离线、禁用工具链自动下载和 CGO |
| live/ready 不泄露信息 | PASS，固定响应断言、模拟敏感探针错误及真实进程冒烟 |
| 优雅退出最小冒烟 | PASS，真实 SIGTERM/SIGINT 退出 0，TCP 请求排空测试 |
| 相关格式/静态/测试 | PASS，上表真实执行结果 |
| 单卡独立提交/精确 SHA | 提交后交接 SHA；独立审计与集成由调度派发 |

后续组合入口：`httpapi.New(*health.Handler) http.Handler`；`health.New(health.Probe)`；`health.Probe` 为 `func(context.Context) error`，必须及时响应取消；`Stop()` 可并发撤销就绪。普通启动：在 server 目录 `go run ./cmd/relaynest -listen 127.0.0.1:8080`。Go 最低精确补丁版本由模块指令锁定；检查使用现有 1.27.3，后续 CI 应保持相同工具链。

遗留/限制：无 DB/卷检查实际资源验证（尚属后续存储任务，本卡 ready 明确 503）；无 fnOS/真机或部署验证，本卡不要求。未对五秒排空超时强制关闭分支做独立计时测试。无用户决策请求。仅交付 implemented，未自行登记 done。
