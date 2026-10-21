# P03-01 集成报告（I01）

日期：2026-10-21。角色：integrator，非本卡开发/审计。结论 PASS，无冲突或代码变化，无需重审。

## 精确提交

目标 `/home/lijx/Projects/RelayNest` 的 `main`；集成前 `fefad6f1ced17aa7a2dd328f8683d1ae8397f1fe`。

| 对象 | 原 SHA | 目标 SHA |
| --- | --- | --- |
| 实现 | `47d33bb7f5ca356947f4107d81429b9ace66dac1` | `c6d29deecf02891b8cb11ed64efe991066f378c9` |
| 独立审计 PASS | `eb56c4acc36b0a3986bd17858632923c5c798261` | `53a76190ec534e65fbbec4544dbb547846ea07a5` |

依序 cherry-pick 成功；本报告提交前 HEAD 为 `53a76190ec534e65fbbec4544dbb547846ea07a5`。本报告独立提交，最终 SHA 提交后交接。

## 组合验证

实际环境 Linux amd64，Go 可执行文件 `/home/lijx/.local/share/mise/installs/go/1.27.3/bin/go`，`go version` 为 `go1.27.3 linux/amd64`。Go 检查 cwd 为主目录 `server/`；冒烟使用已有 Python 3.13.15 绝对路径。

- `go test -race -count=1 ./...`：三个包全部 PASS（cmd 1.081s，health/httpapi 各 1.012s），无竞态报告。
- `go vet ./...`、`go mod tidy -diff`、`gofmt -l cmd internal`：均退出 0，无输出或差异。
- `GOTOOLCHAIN=local GOPROXY=off CGO_ENABLED=0 go build -mod=readonly -trimpath -o <临时目录>/relaynest ./cmd/relaynest`：PASS。复用现有缓存；此前独立审计已验证空缓存，本次组合只叠加报告，未重复昂贵空缓存构建。产物在仓库外并自动清理。
- Python 对该产物启动两个独立进程，使用随机回环端口，依次请求 GET live/ready、HEAD live、POST live、GET /admin：两轮均为 200/503/200/405/404；健康正文固定。分别发 SIGTERM/SIGINT，均七秒内退出 0，日志含 shutdown_complete、无模拟 Authorization 值，监听端口关闭。
- Python 枚举原实现/审计提交每个路径，逐字节比较原/目标 blob：全部一致。无冲突，没有修改代码、测试或原报告。
- `git diff --check <集成前> HEAD`：PASS。registry、`.directory` 哈希及除授权新增文件外完整 index entries 与本卡开始快照一致；未编辑、stage 或提交用户文件。

## 限制与交接

默认无 DB/附件卷探针，ready 为 503；不解释为资源健康通过。无真机、FCM、fnOS 或部署验证；本轮未重复独立审计的临时五秒超时测试，相关原审计证据保留。未修改共同契约或台账，未推送、部署或发布。

组合验证已通过，正式状态交总调度登记。按调度后续要求，将从最终 main HEAD 准备 api/storage 两个独立工作树，只准备不实现；后续文件归属分别为 `server/api/**` 与 storage/migrations，`go.mod` 等共同配置须串行处理。
