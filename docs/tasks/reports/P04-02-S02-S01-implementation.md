# P04-02-S02-S01 认证 HTTP 行为与竞争收敛实施

- 角色：开发；交付待独立审计及精确集成，不自行标记 done，不收口原 S02 或父 P04-02。
- 实际执行基线：`f1c739b9c9aaf84b2fb6a1eaad6edc9a88b69dda`，源码祖先 `b60f9ae09878bc6f3af06861ad19a90a73f088e1`；工作树 `/home/lijx/.codex/worktrees/auth-http/RelayNest`，分支 `codex/auth-http`。
- 实现版本：本报告与三份源码同提交，精确 implementation SHA 由提交后的交接输出固定（本提交不能内嵌自身哈希）。审计必须使用该固定提交，不能包含工作树 main dirty。
- 规划实现 `3022e333def87c51795db442d623f79584633491`，独审 PASS `b138739f7a134a60750b56235a59b4535e38c08e`，精确集成源码/卡内容已在本树。前置管理员、session、来源模块沿调度已审集成证据，只读取实际接口，不修改其实现。

## 变更

接管原作者保留的 `server/internal/auth/http.go`、`http_test.go` 和 `server/internal/httpapi/router.go`。三条认证 handler 同批注册准确路径；消费正式管理员、会话及来源模块，不执行业务命令。保持 Web Cookie/CSRF 与 Android Bearer JSON 隔离、统一 JSON 错误、no-store、当前注销。对 `device_id` 非字符串类型显式返回 400，避免将结构错误误报为 422。

新增真实 TLS HTTP/SQLite 测试夹具，拒绝检查比较请求前后所有会话的 ID、闲置/绝对期限和撤销状态，且不允许 Set-Cookie。可变时钟使用 atomic，生命周期不依赖未提交 main。重置竞争通过现有会话随机源屏障在已校验密码后暂停 Create；另一排序在 Create 完成、HTTP 响应放行前重置。两种顺序均用通道控制，无睡眠碰概率，无生产测试钩子。

## 原 S02 前四验收项映射

| 原验收 | 本卡实际证据 | 交接边界 |
| --- | --- | --- |
| 三条路径、双端 DTO、204、Cookie、no-store、未知 API | `TestHTTPLifecycle` 经真实 TLS 驱动三条路由；7 字段 session、Web/Android 顶层字段数量、Secure/HttpOnly/SameSite/Path、GET 不重复发 token、Web 清 Cookie；夹具要求 204 空体与每响应 no-store。`TestHTTPInputRejectionsHaveNoEffects` 检验未知字段/结构/尾随 JSON/语义及 256 KiB 限制 | 未知 API/健康/静态全面路由回归由 S02-S02 完成；既有 httpapi 测试本轮通过 |
| 错误统一及负例 | `TestHTTPRejectionsHaveNoEffects` 覆盖无管理员、错账号/错密码、混合/重复 Cookie/Bearer、错通道、Bot 字符串、缺/坏凭据、CSRF/来源失败；`TestHTTPClientDeadlinesAndLogout` 与 `TestHTTPResetLoginOrdering` 覆盖过期/撤销/旧代次。拒绝码、无 Cookie、数据库快照不变均显式断言；生命周期响应检查不含密码/hash/generation 元数据 | Bot 用非会话秘密字符串验证凭据通道，未引入 Bot 业务配置 |
| 真实来源模块组合 | `TestHTTPRealSourceComposition` 使用真实 TLS/普通 HTTP 与正式 policy，检查合法直连、不可信对端伪造头不能建立 HTTPS、可信 HTTPS 终止时 Secure Cookie、重复/畸形/冲突转发头、缺/重复 Origin；`TestHTTPRejectionsHaveNoEffects` 经真实 TLS 验证缺源/跨源及 Referer 正负例与冲突、重复 CSRF。`TestHTTPSourceAndInput` 另覆盖端口差异和部分转发头 | 此为 HTTP handler 组合；代理请求由测试客户端发送，不冒称真实反向代理/NAS 部署。进程实际配置读取与接线由 S02-S02 证明 |
| 双端生命周期/续期/重启/隔离/重置竞争 | `TestHTTPLifecycle` Web/Android 登录、有效 GET、关闭 HTTP/SQLite 后重开、Web 当前注销与其它设备隔离、旧凭据拒绝；`TestHTTPClientDeadlinesAndLogout` 分别显式比较双端有效续期、绝对期限不变、当前注销/其它同端设备存活、闲置和持续续期至绝对期限后拒绝；`TestHTTPResetLoginOrdering` 两排序验证旧代次不能建会话或留下有效凭据 | 重启为正式包/SQLite 重开，进程启动/关闭证据留下一卡；WS/推送不在范围 |

## 实际检查

显式环境：`PATH=/home/lijx/.local/share/mise/installs/go/1.27.3/bin:$PATH GOTOOLCHAIN=local`，`go version` 输出 `go1.27.3 linux/amd64`。

- `cd server && go test -race ./internal/auth ./internal/httpapi`：PASS，auth 20.858s，httpapi cached；最终代码检查，无 race 报告。
- `cd server && go vet ./internal/auth ./internal/httpapi`：PASS。
- 三个允许 Go 文件 `gofmt -w`，`git diff --check`：PASS。
- 已有 `server/internal/webui/dist/index.html` 与 assets，无需重跑 npm 安装或构建。
- 首轮补测发现错通道 Cookie 注销同时缺 Origin 会先返回 403；已让通道测试提供合法 Origin，以隔离认证断言，未修改正式来源优先顺序。之后最终 race PASS。
- 默认 PATH 无 python3；一次编辑脚本未执行，改用已锁 Node 22.23.2 完成编辑。未将失败编辑或中间失败测试记为通过。

## 保留与 NOT_RUN

未修改、未暂存原 `server/cmd/relaynest/main.go`（SHA-256 `413893b9846ca4e44a586272b2ddf12e1d2518e6d33a35733ef40ac1ef086bf0`）及原 `docs/tasks/reports/P04-02-S02-implementation.md`（`dfd20dfe7203776717bdb854a108b1f6c0622c68ad6553703c94e71cfafe7cb9`）。仅提交本卡四路径，未改 session/origin/administrator、SQL/API/依赖/registry。

进程 main 组合、cmd race/vet、完整契约检查与父卡完整验收汇总由 S02-S02 继续；本卡 NOT_RUN。真实 NAS/手机/MP/WS/推送/正式 AC、真实代理部署均 NOT_RUN。未 push、部署、新建聊天或派生子 agent。需另一 agent 对固定实现 SHA 独审，PASS 后精确集成再派下一卡。
