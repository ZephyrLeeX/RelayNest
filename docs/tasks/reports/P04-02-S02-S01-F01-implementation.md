# P04-02-S02-S01-F01 实施报告

- 角色：修复开发；唯一发现 `P04-02-S02-S01-A01-F01`（P2）。待独立复审，不自行关闭发现，不代表原 S02/父卡完成。
- 派发基线：`86080588cb6113eb774fc2ae946f9363c25511a7`；原被审实现：`117da40467ff76aa59d02b0c73b996b79c519fb8`；A01：`0c2de0d162a597808188251aebf7e7cd8b56870f`。
- 修复实现为包含本报告的独立提交，精确 SHA 在开发交接中提供，避免报告内容自引用提交 SHA。
- 仅修改 `server/internal/auth/http.go`、`server/internal/auth/http_test.go` 与本报告。

## 修复与回归证据

`loginRequest.UnmarshalJSON` 先读取 JSON 对象精确键，只接受既有五个键，再解码原 DTO；不再由结构体大小写匹配决定未知字段合法性。沿用原字段类型、双端验证、输入上限与错误分类。没有修改路由、API、底层会话/来源/管理员、SQL 或依赖。

新增回归前，旧生产实现搭配首次新增的 `TestHTTPLoginExactFieldNames` 运行失败：Web `USERNAME` 返回 200 并带会话/CSRF 响应，而预期为 400。首次测试用了绑定父测试的夹具放入子测试，失败后另报 Goexit；最终测试已移除该层子测试，完整检查通过。A01 原独立 `Username` 200/Set-Cookie/数据库变化证据保持有效。本次不将测试骨架错误当作业务复现依据；业务复现依据是明确的意外 200。

- `TestHTTPLoginExactFieldNames`：真实 TLS HTTP/SQLite，Web 四字段、Android 五字段的首字母大写及全大写变体（含 `Username`、`DEVICE_ID`），分别覆盖别名替换和合法键与别名共存，均为 400 `INVALID_REQUEST`。
- 两个新增测试均先成功创建 Web/Android 会话。每次拒绝通过 `reject` 推进固定时钟，核对错误码、无任何 Set-Cookie 响应头、会话行 ID/闲置期限/绝对期限/撤销状态快照不变，覆盖无创建、无续期和无撤销副作用。加强既有 helper 为直接检查响应头数量。
- `TestHTTPLoginDTOContract`：普通未知键、非对象/畸形 JSON、尾随数组/null/bool/垃圾、四个字符串字段错误类型以及缺失/空字符串/null；错误类型为 400，必填/空值沿用 422。
- 双端 `device_id` 错误类型为 400；null/空/无效 UUID 为 422；Web 提交合法 UUID 也为 422。Android 缺失、超限 413、尾随第二对象由既有 `TestHTTPInputRejectionsHaveNoEffects` 覆盖。正常双端登录由夹具登录及原生命周期测试覆盖。
- 冻结 OpenAPI 未规定禁止重复精确键；保留原解码语义，重复合法 `username` 成功 200，较早的错误类型仍为 400；重复精确键夹带 `Username` 仍为 400。未新增拒绝重复键的协议。
- 整体 `null` 输入沿用原 422；本次没有扩张 F01 范围更改此现状。

## 实际检查

环境：Linux；确认 `/home/lijx/.local/share/mise/installs/go/1.27.3/bin/go version` 为 `go1.27.3 linux/amd64`。以下 Go 命令在 `server` 下执行，均显式设置 `PATH=/home/lijx/.local/share/mise/installs/go/1.27.3/bin:$PATH GOTOOLCHAIN=local`。

- `go test ./internal/auth -run '^TestHTTPLoginExactFieldNames$' -count=1`：修复前如上预期失败。
- `go test ./internal/auth -run '^TestHTTP(LoginExactFieldNames|LoginDTOContract|InputRejectionsHaveNoEffects)$' -count=1`：通过；随后增加 device_id 边界纳入最终 race 检查。
- `go test -race ./internal/auth ./internal/httpapi`：通过（auth 23.223s，httpapi cached）。
- `go vet ./internal/auth ./internal/httpapi`：通过。
- `gofmt -w internal/auth/http.go internal/auth/http_test.go`：完成；随后格式复查、`git diff --check` 通过。
- 固定树现有 embed 满足相关检查，未创建伪造产物。尝试用 `python3` 暂存源码再复跑旧版时，环境报 command not found，未执行脚本且未改变文件；不作为验证证据。
- NOT_RUN：全仓库测试、cmd/真实进程或反代部署、原 S02/父卡全面验收；均超出本修复卡范围。需另一角色针对交付精确 SHA 独立复审。

## 原 dirty 保留

开始/结束核对 SHA-256 一致且不 stage：

- `server/cmd/relaynest/main.go`：`413893b9846ca4e44a586272b2ddf12e1d2518e6d33a35733ef40ac1ef086bf0`。
- `docs/tasks/reports/P04-02-S02-implementation.md`：`dfd20dfe7203776717bdb854a108b1f6c0622c68ad6553703c94e71cfafe7cb9`。

只显式 stage 本卡三个路径，本地 Conventional 提交；不 push、不改 registry。
