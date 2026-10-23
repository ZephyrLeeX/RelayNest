# P04-03-S01 显式来源配置与可信代理解析基础实施报告

## 范围与依据

- 角色：developer；结果：implemented，待独立审计及精确集成，不能据此标记 done。
- 基线：`9ca05a0dfe7973a2fb8d077f3e9646085c2451f6`；分支 `codex/request-source`。最终实现 SHA 在提交后的交接中提供，不在提交内预写自身 SHA。
- 仅新增 `server/internal/httpapi/origin.go`、`origin_test.go` 和本报告；未修改 router/main/auth/store、SQL、迁移、API、设计、依赖、任务卡、registry 或旧报告。
- 遵循本卡及已独审规划的单一 HTTPS Origin、显式直接代理、严格单层三头协议。未增加产品或安全政策决定。模块不读写 session/DB，不执行 Cookie 身份判断、CSRF 或业务命令。

## 实际接口与 S02 注入

```go
p, err := httpapi.NewOriginPolicy("https://hub.example:8443", []string{"192.0.2.10"})
if err != nil {
    // 配置错误必须使认证依赖构造/启动失败，不挂载缺保护的路由。
    return err
}
// 将 p 注入认证 handler；以下地址仅文档 fixture，不是部署默认值。
source, err := p.ValidateLogin(r) // Web 登录要求 Origin
// Web Cookie 写操作使用 p.ValidateWrite(r)，再执行会话 CSRF。
// Android Bearer 等通道使用 p.Resolve(r)，不因此强制浏览器头。
```

- 构造器 `NewOriginPolicy(externalOrigin string, trustedProxies []string) (*OriginPolicy, error)` 验证配置并复制代理值到私有不可变字段。空列表不信任任何代理；缺失/非法 Origin 或代理返回 nil 与 `ErrOriginConfig`，零值或 nil policy 也不能解析成功。
- `Resolve(*http.Request) (RequestSource, error)` 只验证传输来源。`ValidateLogin`、`ValidateWrite` 同签名，先 Resolve，再校验浏览器来源，任一失败均返回零结果。
- `RequestSource` 含 `ConfiguredOrigin`、`ExternalOrigin`、`HTTPS`、`Secure`、`DirectPeer`、`ClientIP`、`Forwarded`。IP 为 `netip.Addr`，mapped IPv6 归一成 IPv4；成功 HTTPS/Secure 都为 true。
- `ErrRequestSource`、`ErrBrowserOrigin` 与配置错误均为固定 sentinel，不携带原头、Referer、配置或用户信息。S02 必须先检查错误再消费结果；请求错误应映射为既有拒绝响应，不继续认证或设置安全 Cookie。这里不制定新 HTTP 错误码。
- S02 负责读取显式运行配置并注入，按规划保留现有 listen 语义与管理员命令边界；本卡没有新增启动 flag，也没有挂载认证路由。

## 解析细节

- Origin 为 HTTPS scheme、ASCII host、有效端口三元组；大小写归一，缺省/显式 443 等价，非缺省端口精确比较；IP 使用标准库规范化。域名按 ASCII 字母/数字/连字符标签验证，IDN 要求显式 A-label；尾点保留，别名不合并，不使用 DNS。拒绝 userinfo、path、query、fragment、控制字符、通配符、畸形 authority/端口、null 与多值。
- 仅 `RemoteAddr` 的 IP:port 判 peer；配置只接受 IP/CIDR，禁止 `/0`、主机名与隐式信任。mapped IPv6 CIDR 在前缀不超过映射边界时拒绝，否则转换为等价 IPv4 CIDR，不扩大信任。
- 非可信 peer 完全忽略所有 forwarding 头，按实际 TLS 与 Host 验证；可信 peer 三头均无且无 Forwarded 时也验证直连。其它情况要求 X-Forwarded-Host/Proto/For 各一个完整值，Proto 精确 `https`、For 单 IP、Host 匹配配置；重复/列表/部分缺失/畸形/冲突/任何 Forwarded 均拒绝，无回退。
- 不取 URL.Scheme、X-Forwarded-Port 作 HTTPS 或来源依据；手构 Request 的大小写等价重复键同样拒绝。标准库真实入站先规范头名与去除字段外围空白，模块以其解析后的值检查；不会把模拟 Request 当作 wire 解析证据。
- Web 登录必须有 Origin；写操作只在 Origin 缺失时回退 Referer。任一已出现的头为空、重复、畸形或跨源均拒绝，双头必须都匹配基准。Referer 允许合法绝对 HTTPS URL 的 path/query（包括其中逗号），不以其内容参与同源判断；拒绝 userinfo、fragment、错误转义及非法 authority，不回写完整 Referer。

## 真实验证

环境：Linux amd64；mise 已安装 Go 1.27.3、Node 22.23.2、npm 10.9.8；Go 使用 `GOTOOLCHAIN=local GOFLAGS=-mod=readonly`，没有新增依赖。

- `npm --prefix web ci --no-audit --no-fund`、`npm --prefix web run build`：PASS，本工作树先生成忽略的 webui/dist 后运行 Go 检查。
- `go test ./internal/httpapi`：PASS；`go test -race -count=1 ./internal/httpapi`：PASS；`go vet ./internal/httpapi`：PASS（均在 server 目录）。
- `gofmt -l internal/httpapi/origin.go internal/httpapi/origin_test.go`：空输出；`git diff --check`：PASS。
- `/tmp/relaynest-health-contract-locked/bin/python server/api/validate.py`：PASS，OpenAPI 3.1、115 schemas、568 valid + 661 invalid examples、39 operations/security；未改契约或校验器。
- 配置正反例覆盖域名/IP/IPv6/端口、无默认配置、无效代理、全地址段拒绝、mapped CIDR 与配置拷贝；来源覆盖显式/不可信 loopback 和私网、直接 peer、mapped IPv6、非法 RemoteAddr、TLS/Host/端口差异、伪造转发头不能改变合法直连结果。
- 转发覆盖可信 HTTP 终止成功、客户端 IP、三头缺失/重复/大小写重复/空/多 hop/Forwarded 混用/Host/Proto/IP/端口冲突，以及合法 TLS/Host 也不能挽救失败转发。浏览器覆盖默认 443、非缺省端口/IPv6、跨源/null/多值/重复/缺头、Referer 回退和双头冲突；所有失败断言固定错误及零结果。
- `httptest` 真实 HTTP/TLS server 验证代理 HTTP、非可信 HTTP 伪造拒绝、TLS 直连/伪造头隔离与重复头拒绝；`http.ReadRequest` 验证重复字段保留并由模块拒绝；真实 TCP 入站非法 NUL 头返回标准库 400 且不进入 handler。手构 Request 额外覆盖非规范键、nil 头值与非法 peer 等无法等价从合法 wire 产生的边界。

## 限制与后续

- NOT_RUN：真实 NAS、公网 Origin/端口、OpenWrt/Nginx 直接 peer 清单和三头覆盖部署验证；这些资源必须由用户在部署前提供。代理必须覆盖而非追加三头，本地 fixture 不代表真实部署兼容。
- NOT_RUN：尚未接线的启动配置/认证 HTTP、Cookie/session/CSRF 组合验收；由 S02 与父 P04-03 负责。没有重跑历史手机、MoviePilot、签名或 RelayShelf PoC。
- 无新增用户决策；本卡未涉及迁移版本。独审必须针对最终源码 SHA，源码变化后重新审计。
