# P07-01 Web 登录会话与错误壳层实施报告

- 角色：developer；结论：实现已提交候选，浏览器验收 blocked，等待独审及集成，不能标记 done。
- 基线：`3f39182da4cd67522a015a2ec675a3ad43878944`；工作树：`/home/lijx/.codex/worktrees/web-auth/RelayNest`；分支：`codex/web-auth`。实现 SHA 在提交后交接，不预写自身 SHA。
- 前置：读取 P03-02-integration-01、P04-03-I01 及冻结 OpenAPI auth 三路径/Web DTO/CSRF，遵守 PRD 无 Web 聊天历史及 RelayShelf 独立边界。

## 变更与接口边界

真实现有栈为 TypeScript 5.9.3 / Vite 7.3.7，原 main.ts 仅空入口；保持零运行时依赖，未引入 React。四主要实现文件：auth/client.ts、auth/shell.ts、shared/errors.ts、main.ts；另有两验证文件。不修改 API、后端、数据库、依赖锁、registry 或其他卡；构建 dist 被忽略且不提交。

- `/api/v1/auth/login` 使用严格 Web DTO（username/password/client_type=web/device_name），不发送 Android device_id、Authorization 或自造 Origin。`fetch` 固定相对同源路径、credentials=same-origin、no-store、redirect=error。
- `/api/v1/auth/session` GET 仅依靠浏览器 Cookie 恢复会话，Web DTO 的 csrf_token 仅保留于内存；不读 session Cookie，不访问 document.cookie、localStorage、sessionStorage，不把响应中未知字段传播到视图。
- `/api/v1/auth/logout` POST 携带 X-CSRF-Token，成功必须为 204；Cookie 设置/清除交由浏览器。403 保留当前会话，显示安全提示并允许显式重新检查会话后重试；不会自动重放写入。
- 初次无会话回到登录；已登录后401显示过期/撤销，清理 CSRF。界面按服务端 idle_expires_at/absolute_expires_at 较早期限退出，focus/visibilitychange 再检查；不设后台保活。客户端时钟只作提示，最终权限始终由服务端裁决。
- 密码仅当前输入及当前请求临时使用，提交后立即清空输入；隐藏页面/pagehide/卸载时清空。无凭据日志、URL参数或持久化，无记住密码或密码自动重试。
- 网络、401、403、429、503及其他错误仅显示本地文案，不渲染服务端 message/raw body/code；用户名等动态数据用 textContent。按钮在请求期间禁用。管理登录后只显示管理员和设备名称，不提供其他管理域、正文、附件或历史。

## 实际检查

环境 Linux amd64；mise Node v22.23.2 / npm 10.9.8；Go go1.27.3，GOTOOLCHAIN=local。命令中的 PATH 前置本机 mise 对应 bin。

| 命令/检查 | 结果与证据 |
| --- | --- |
| `npm ci --prefix web --no-audit --no-fund` | PASS，16 packages，锁文件无变更 |
| `npm --prefix web run build` | PASS，含 tsc --noEmit；Vite 7 modules |
| `node --test web/src/auth/tests/client.test.mjs` | PASS，4/4：严格DTO/同源/CSRF/204、401清理、403/429/503安全文案、拒Android/畸形DTO/非204、未知字段与网络错误隔离；transport为fetch协议桩，不是E2E |
| `GOTOOLCHAIN=local go -C server build -o /tmp/relaynest-p0701 ./cmd/relaynest` | PASS，当前树生产入口二进制 |
| `REAL_REQUEST_ONLY=1 RELAYNEST_BINARY=/tmp/relaynest-p0701 node web/src/auth/tests/browser.mjs` | PASS，真实临时SQLite/admin + 生产二进制 + 本机HTTPS代理；手工Cookie jar验证login200/session200/无CSRF403/logout204/旧会话401及HttpOnly/Secure/SameSite响应头。不是浏览器证据 |
| 完整 browser.mjs（下方命令） | 环境 FAIL；浏览器交互 NOT_RUN。Chromium进程在加载 libglib-2.0.so.0 时退出，页面未启动；不能把脚本中浏览器断言算通过 |
| `git diff --check` | PASS |
| Web lint | NOT_RUN，无独立 lint 脚本/配置；采用已有 tsc/build |
| 真 OpenWrt 反代/手机 | NOT_RUN，无该环境，本卡未部署现网 |

完整浏览器尝试使用已有工具，无安装/升级：

```sh
PLAYWRIGHT_MODULE=/mnt/c/Users/lijx/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules/playwright/index.mjs \
CHROMIUM_EXECUTABLE=/home/lijx/.cache/ms-playwright/chromium-1187/chrome-linux/chrome \
RELAYNEST_BINARY=/tmp/relaynest-p0701 node web/src/auth/tests/browser.mjs
```

本机失败日志 `/tmp/relaynest-p0701-browser.log`：`error while loading shared libraries: libglib-2.0.so.0: cannot open shared object file`。永久可复现输入在已提交 browser.mjs。脚本将真实请求和后续协议桩交互分别输出；真实请求先成功，不代表浏览器启动成功。测试自建临时 HTTPS Origin、完整三项转发头和 trusted-proxy=127.0.0.1，不猜测生产 Origin；测试凭据随机生成、仅传 stdin/内存，finally 关闭服务并删除临时 DB/证书。

初次探针的 readiness 路径使用 /health/ready，而当前生产 health.New(nil) 不提供 ready；改为 /health/live。首次代理少 X-Forwarded-For 被既有安全层拒403，补全已确认三头后真实请求通过。这些仅是测试接线修正，未改变服务端校验。

## 验收及遗留

| 卡片验收 | 状态 |
| --- | --- |
| 登录/退出/过期/CSRF错误可用 | 客户端协议单元与真实请求部分 PASS；实际浏览器DOM交互、计时到期及浏览器Origin/Cookie发送规则 NOT_RUN，保留 blocked |
| 安全 Cookie 不被 JS 读取 | 源码无 Cookie/存储访问、真实 Set-Cookie HttpOnly PASS；浏览器不可读性 NOT_RUN |
| 受影响检查 | tsc/build/4单元/真实本地请求 PASS；浏览器受运行库缺失阻塞，未掩盖 |
| 本卡提交与交接 | 仅精确暂存本卡文件；独审/集成未进行 |

下一步在具备 Chromium 共享库的环境补跑完整脚本，再由独审核对实现 SHA；其中协议桩的401/403/429/短期限交互必须与真实后端请求证据分别报告。页面当前只接认证壳层，后续管理模块应复用相同 Cookie/CSRF/错误边界，不能新增消息历史浏览。
