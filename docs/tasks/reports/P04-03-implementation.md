# P04-03 Web 同源 CSRF 与代理组合验证报告

## 固定对象与结果

- 任务 P04-03，角色 developer（本地验证收口），结果 `implemented`，等待独立审计与精确集成；不自行标记 done。
- 基线和本次验证源码 SHA：`96f6fff8340957104fb627606640adc2bc572eae`；工作树 `/home/lijx/.codex/worktrees/security-closure/RelayNest`，分支 `codex/security-closure`。开始时树干净，仅新增本报告；提交 SHA 在提交后的结构化交接给出。
- 前置 P04-03-S01：实现 `bc36861e21f1f33d35ba33426df82125cb6ddfcb`，A01 PASS `1b61b1760cdafc6dcb147bdb501b778919d359de`，集成组合 `b75b6c348d5912b14e94632096519d22c5179e7c`。实际配置/接口遵循其实施、审计和集成报告。
- 前置 P04-02：认证组件 `6cec43b707b88908712561718988599bc5404988`、A01 PASS `8ead8349afad11e1278dcb0506cf90f3a512466b`；启动实现精确组合 `96f380df88912007ee2862914f249d607f0bd3a4`、最终 A01 PASS `2f425e0d2c3edcf0f18a279f56b8993b6dab0a58`、I01 `e831cf9d9810206f96b21ea16f2dabe0b3df3afe`，均在本基线祖先。采用 [启动 I01](P04-02-S02-S02-I01.md) 的最终本地技术收口，不重做前置实现。
- 依据：AGENTS、CONTRIBUTING、WORKER_PROTOCOL、本卡完整范围；ARCHITECTURE §1 C-01～C-05/§5、TECHNICAL_DESIGN §7.1、server/api/README.md 认证契约。未改需求、安全策略、API、SQL、依赖、台账、卡片或任何源码。

## 五项验收对应

| 本卡验收 | 本次结果与可复核证据 |
| --- | --- |
| 伪造转发头、跨源、端口不匹配拒绝 | 本地 PASS。`httpapi/origin_test.go` 的 TestDirectSource、TestTrustedForwarding、TestBrowserOrigins、TestOriginRealHTTPAndTLS、TestOriginWireParserBoundaries；`auth/http_test.go:565` TestHTTPRealSourceComposition 的真实 HTTP/TLS；`cmd/relaynest/main_test.go:98` TestMainProcessAuthentication 的显式代理/非可信 loopback 两进程。附录探针在真实启动链上新增 Origin 外部端口与转发 Host 端口错误、重复/冲突/非法代理头和跨源写拒绝。 |
| HTTPS Cookie 正确 | 本地 PASS。TestHTTPLifecycle 验证 Secure、HttpOnly、SameSite=Lax、Path=/api/v1；TestHTTPRealSourceComposition 验证直连 TLS/受信代理 HTTPS 发 Cookie、未受信 HTTP 伪造 HTTPS 不能登录。附录验证生产 main 登录 Cookie 和成功注销清除 Cookie 同样四属性、清除 MaxAge=-1（线缆 Max-Age=0）。 |
| 真实来源、配置、认证接线与登录/写/会话 CSRF/代理/端口组合 | 本地 PASS。附录通过实际 `main()` 子进程启动，显式 external-origin 和 trusted-proxy，真实 TCP/SQLite：登录→Cookie→GET session→14 个失败写→无 Origin 的同源 Referer+CSRF 注销→旧 Cookie 401。既有真实 main 测试补非可信 peer；不以 S01 单元测试替代父验收。当前已实现的 Web 状态写操作是登录、注销；未来业务写路由仅交接约束。 |
| 执行受影响检查并报告环境/结果 | PASS，命令见下节；真实外部部署/设备不具备条件，明确 NOT_RUN，未将其写成本地 PASS 的证据。 |
| 仅本任务提交、精确 SHA/范围/证据/遗留、独审集成才 done | 本报告独立提交；源码 SHA 固定如上，报告提交 SHA 提交后交接。独立审计及集成 pending，不能据本报告自宣 done。 |

## 独立组合复验的证明范围

附录测试通过仓库 mainProcess 创建测试子进程；TestMainProcessHelper 只传递 argv 并进入真实 `main()`，使用生产配置构造、生产路由、OriginPolicy、auth.Handler、SessionStore 和迁移后的 SQLite，不使用 routeProbe 或模拟认证结果。`127.0.0.1` 是真实直接网络 peer，`192.0.2.8` 仅转发客户端 fixture；外部 Origin 为 `https://relay.example:8443`。

本地 HTTP 模拟可信 TLS 终止代理到 Hub 的一跳链路，测试显式附带收到的 Cookie；浏览器不会在该 HTTP 测试链路自动回送 Secure Cookie。实际 TLS 握手由既有 TestHTTPRealSourceComposition/TestOriginRealHTTPAndTLS 覆盖，二者均本次重跑。没有声称执行真实 OpenWrt/Nginx 或浏览器端到端部署。

每次拒绝先快照 administrator、sessions、system_state、changes、bots、bot_credentials 的全部列/全部行（排序），并断言返回 403/FORBIDDEN、无 Set-Cookie、快照相等。登录和 session 成功后等待 1.1 秒，避免秒精度期限掩盖意外续期；既有 f.reject 另以注入时钟每次前移一分钟验证 idle/absolute/revoked 不变。快照仅存内存、不会输出凭据或 DB 原始内容；这里的无副作用指可观察业务行状态，不宣称 SQLite 文件字节/WAL 不变。

14 个真实 main 拒绝分支为：CSRF 缺失/错误/重复；跨源 Origin、Origin 错端口/重复、缺 Origin 与 Referer、坏 Origin 配正确 Referer（不得回退）；跨源/重复 Referer；转发 Host 错端口、重复 Proto、Forwarded 冲突、非法转发 IP。成功 Referer 回退含 path/query。既有 TestHTTPRejectionsHaveNoEffects 还覆盖正确 Origin 配跨源 Referer拒绝、登录不可用 Referer替代 Origin，以及混合/错客户端凭据拒绝。

## 实际检查（2026-10-23，本地）

环境：WSL2 Linux 6.18.33.2-microsoft-standard-WSL2 amd64；Go `/home/lijx/.local/share/mise/installs/go/1.27.3/bin/go`（go1.27.3），所有 Go 命令 `GOTOOLCHAIN=local`，以下工作目录为本树 `server/`。已有 ignored Web embed dist 可编译，Web 源及依赖未改，本次不重跑 npm ci/build。

| 命令（go 为上述绝对路径） | 实际结果 |
| --- | --- |
| `go test ./internal/auth ./internal/httpapi ./cmd/relaynest -count=1` | PASS，auth 6.281s、httpapi 0.101s、main 1.255s。 |
| `go test -overlay=/tmp/relaynest-p04-03/overlay.json ./cmd/relaynest -run '^TestP0403StartupSecurityComposition$' -count=1 -v` | 该次 PASS，1.487s，14 个子用例 PASS，完整生命周期 PASS。首次探针把 health/ready 的纯文本响应当 JSON 解析，探针失败；修正探针仅解码 /api/ 响应后复跑通过。未改产品代码。 |
| `go test -race ./...` | PASS，main/auth/bots/changes/health/httpapi/storage/webui 八包，migrations 无测试；auth 21.620s，未出现 race。 |
| `go test -race -overlay=/tmp/relaynest-p04-03/overlay.json ./cmd/relaynest -run '^TestP0403StartupSecurityComposition$' -count=1` | PASS，最终 name=value Cookie 线缆格式探针，4.963s（加 -v）。 |
| `go vet ./...` | PASS，退出 0，无输出。 |
| `git diff --check` 与 `git diff --cached --check`（仓库根） | 提交前执行，PASS。 |

临时详细日志为 `/tmp/p04-03-tests.log`、`/tmp/relaynest-p04-03/{probe,probe-race,probe-final,race,vet}.log`，不作为跨机器永久证据；附录保存完整探针和复现命令，可在相同源码 SHA 独立重跑。探针 SHA256 `199b4ac6350e90a8eb35614d8bbb0f2dc8182db567bcd7e890b32f43b731dff6`。

## 交接与未执行边界

- 当前检查未发现需要源码修复的缺陷，无新用户决策。生产来源只消费 `NewOriginPolicy`、`Resolve`、`ValidateLogin`、`ValidateWrite`；直接 peer 决定转发头信任，不默认信任私网或 loopback。Web 登录必须 Origin，Cookie 状态写须同源 Origin/Referer 与当前会话 CSRF。
- auth.current 先无续期 Inspect、再检查 Web CSRF，失败不调用 Authenticate/Revoke；注销成功才清 Cookie。未来管理写操作必须复用同等顺序，并验证失败无续期、无 Cookie/DB 副作用。当前尚未导出公共管理 security 函数；P04-05-S01 提取工作不在本报告实现。未对尚未实现的设备/Bot/失败队列等业务 HTTP 路由冒做运行验收。
- NOT_RUN：真实 NAS、公网 Origin/端口与 OpenWrt/Nginx 三头覆盖策略、真实浏览器/手机、MoviePilot、真实 WS、推送、发布、正式 AC。无环境不代表这些兼容门槛通过，后续相应验收仍须保留未运行/阻塞。`/health/ready` 仍 503，与既定占位状态相符。
- 没有 push、部署、修改 RelayShelf、业务命令/MCP、额外子 agent、消息或新聊天。只新增本报告，等待另一 agent 固定提交审计，再精确集成。

## 附录：可重跑的只读 overlay 探针

以下 Go 文件只放 `/tmp`，overlay 映射新增测试路径；不会写入仓库源码。它复用的是生产 main 子进程入口启动器，不替代真实 handler。

```go
package main
import("testing";"strings";"path/filepath";"net";"net/http";"time";"syscall";"database/sql";"fmt";"io";"encoding/json")
func TestP0403StartupSecurityComposition(t *testing.T) {
 path:=filepath.Join(t.TempDir(),"security.sqlite")
 init:=mainProcess("admin","init","-db",path,"-username","Admin");init.Stdin=strings.NewReader("correct\n")
 if out,err:=init.CombinedOutput();err!=nil{t.Fatalf("init: %v %s",err,out)}
 reservation,err:=net.Listen("tcp","127.0.0.1:0");if err!=nil{t.Fatal(err)}
 addr:=reservation.Addr().String();reservation.Close()
 cmd:=mainProcess("-listen",addr,"-db",path,"-external-origin","https://relay.example:8443","-trusted-proxy","127.0.0.1")
 if err:=cmd.Start();err!=nil{t.Fatal(err)}
 t.Cleanup(func(){cmd.Process.Signal(syscall.SIGTERM);if err:=cmd.Wait();err!=nil{t.Error(err)}})
 client:=&http.Client{Timeout:time.Second};base:="http://"+addr;deadline:=time.Now().Add(10*time.Second)
 for{res,err:=client.Get(base+"/health/live");if err==nil{res.Body.Close();break};if time.Now().After(deadline){t.Fatal(err)};time.Sleep(20*time.Millisecond)}
 db,err:=sql.Open("sqlite",path);if err!=nil{t.Fatal(err)};t.Cleanup(func(){db.Close()})
 snapshot:=func()string{
 var all []string
 for _,table:=range []string{"administrator","sessions","system_state","changes","bots","bot_credentials"}{
 rows,err:=db.Query("SELECT * FROM "+table+" ORDER BY rowid");if err!=nil{t.Fatal(err)};cols,_:=rows.Columns()
 for rows.Next(){values:=make([]any,len(cols));ptrs:=make([]any,len(cols));for i:=range values{ptrs[i]=&values[i]};if err:=rows.Scan(ptrs...);err!=nil{t.Fatal(err)};all=append(all,table+fmt.Sprint(values))}
 if err:=rows.Err();err!=nil{t.Fatal(err)};rows.Close()
 };return fmt.Sprint(all)}
 send:=func(method,endpoint,body string,h http.Header)(*http.Response,map[string]any){
 req,_:=http.NewRequest(method,base+endpoint,strings.NewReader(body));req.Host="relay.example:8443";req.Header=h.Clone();req.Header.Set("Content-Type","application/json")
 res,err:=client.Do(req);if err!=nil{t.Fatal(err)};raw,_:=io.ReadAll(res.Body);res.Body.Close();var out map[string]any;if len(raw)>0&&strings.HasPrefix(endpoint,"/api/"){if err:=json.Unmarshal(raw,&out);err!=nil{t.Fatal(err)}};return res,out}
 headers:=func()http.Header{return http.Header{"Origin":{"https://relay.example:8443"},"X-Forwarded-Proto":{"https"},"X-Forwarded-Host":{"relay.example:8443"},"X-Forwarded-For":{"192.0.2.8"}}}
 body:=`{"username":"Admin","password":"correct","client_type":"web","device_name":"security-probe"}`
 res,out:=send("POST","/api/v1/auth/login",body,headers());if res.StatusCode!=200||len(res.Cookies())!=1{t.Fatalf("login %d",res.StatusCode)}
 cookie:=res.Cookies()[0];csrf:=out["csrf_token"].(string)
 if !cookie.Secure||!cookie.HttpOnly||cookie.SameSite!=http.SameSiteLaxMode||cookie.Path!="/api/v1"{t.Fatal("cookie flags")}
 h:=headers();h.Set("Cookie",cookie.Name+"="+cookie.Value);res,_=send("GET","/api/v1/auth/session","",h);if res.StatusCode!=200{t.Fatal("session",res.StatusCode)}
 time.Sleep(1100*time.Millisecond)
 for _,tc:=range []struct{name string;change func(http.Header)}{
 {"csrf missing",func(h http.Header){h.Del("X-CSRF-Token")}},
 {"csrf wrong",func(h http.Header){h.Set("X-CSRF-Token","wrong")}},
 {"csrf duplicate",func(h http.Header){h.Add("X-CSRF-Token",csrf)}},
 {"origin cross",func(h http.Header){h.Set("Origin","https://evil.invalid")}},
 {"origin port",func(h http.Header){h.Set("Origin","https://relay.example")}},
 {"origin duplicate",func(h http.Header){h.Add("Origin","https://relay.example:8443")}},
 {"no browser source",func(h http.Header){h.Del("Origin")}},
 {"bad origin with good referer",func(h http.Header){h.Set("Origin","null");h.Set("Referer","https://relay.example:8443/settings")}},
 {"cross referer",func(h http.Header){h.Del("Origin");h.Set("Referer","https://evil.invalid/settings")}},
 {"referer duplicate",func(h http.Header){h.Del("Origin");h["Referer"]=[]string{"https://relay.example:8443/a","https://relay.example:8443/b"}}},
 {"forwarded port",func(h http.Header){h.Set("X-Forwarded-Host","relay.example")}},
 {"forwarded proto duplicate",func(h http.Header){h.Add("X-Forwarded-Proto","https")}},
 {"forwarded conflict",func(h http.Header){h.Set("Forwarded","proto=https")}},
 {"forwarded malformed ip",func(h http.Header){h.Set("X-Forwarded-For","bad")}},
 }{
 t.Run(tc.name,func(t *testing.T){h:=headers();h.Set("Cookie",cookie.Name+"="+cookie.Value);h.Set("X-CSRF-Token",csrf);tc.change(h);before:=snapshot()
 res,out:=send("POST","/api/v1/auth/logout","",h)
 if res.StatusCode!=403||out["error"].(map[string]any)["code"]!="FORBIDDEN"||len(res.Header.Values("Set-Cookie"))!=0||snapshot()!=before{t.Fatalf("rejection changed state/status: %d",res.StatusCode)}
 })}
 h=headers();h.Del("Origin");h.Set("Referer","https://relay.example:8443/settings?q=x");h.Set("Cookie",cookie.Name+"="+cookie.Value);h.Set("X-CSRF-Token",csrf)
 res,_=send("POST","/api/v1/auth/logout","",h)
 if res.StatusCode!=204||len(res.Cookies())!=1{t.Fatal("referer logout",res.StatusCode)}
 cleared:=res.Cookies()[0];if !cleared.Secure||!cleared.HttpOnly||cleared.SameSite!=http.SameSiteLaxMode||cleared.Path!="/api/v1"||cleared.MaxAge!=-1{t.Fatal("clear cookie flags")}
 before:=snapshot();res,_=send("GET","/api/v1/auth/session","",h)
 if res.StatusCode!=401||len(res.Header.Values("Set-Cookie"))!=0||snapshot()!=before{t.Fatal("revoked session",res.StatusCode)}
 res,_=send("GET","/health/ready","",headers());if res.StatusCode!=503{t.Fatal("ready",res.StatusCode)}
}
```

在本树 server/ 下复现（临时路径可按需改；不加入 Git）：

```sh
mkdir -p /tmp/relaynest-p04-03
awk '/^```go$/{copy=1;next} /^```$/{if(copy)exit} copy' ../docs/tasks/reports/P04-03-implementation.md > /tmp/relaynest-p04-03/probe_test.go
printf '{"Replace":{"%s/cmd/relaynest/p0403_probe_test.go":"/tmp/relaynest-p04-03/probe_test.go"}}\n' "$PWD" > /tmp/relaynest-p04-03/overlay.json
GOTOOLCHAIN=local /home/lijx/.local/share/mise/installs/go/1.27.3/bin/go test -race -overlay=/tmp/relaynest-p04-03/overlay.json ./cmd/relaynest -run '^TestP0403StartupSecurityComposition$' -count=1 -v
```
