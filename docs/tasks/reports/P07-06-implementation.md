# P07-06 Web设备会话管理实施

- 角色：developer；结果：implemented，等待不同agent独立审计与精确集成，未标done。
- 基线：`5547e6cd0819ae47187351e271e622ce43364ffd`。
- 树：`/home/lijx/Projects/RelayNest-recovery-20261024/web-device-impl`；分支：`codex/web-device-impl-20261025`。开始HEAD一致、工作区干净；最终实现SHA在提交后交接，不预写自身SHA。
- 规划依据：父卡完整原验收及PLAN01/F01、A02 PASS、I01；P07-01/P04-05由调度确认精确集成done，不以旧registry阻塞。

## 范围与实现

仅新增devices/client、page及对应两测试；最小扩展auth/client、shell和既有auth/browser测试，另本报告。main原有`#app → mountAuth`及HMR dispose直接复用，无需修改。无框架/依赖/CSS/server/API/Schema/registry、消息正文附件历史、RelayShelf或其他功能变更。

同一个AuthClient保留Cookie、内存私有CSRF与原安全错误解析；设备GET/DELETE窄方法保持same-origin/no-store/redirect:error，DELETE无body，204不调用json；两种401不依赖特定code清会话。认证接受推进代次，clear/dispose及设备页代次隔离迟到响应。设备DTO七字段白名单、严格分页校验；opaque cursor原值编码，按Session.id定位并保留同install多Session。DOM以textContent显示必要metadata，标识仅显示尾部且短标识全隐藏，完整Session.id只留内存。登录/恢复挂载，退出/到期/401/自撤立即清设备视图；非自撤成功刷新，404/403/429/503/网络/无效DTO安全提示可重试且不假报成功。

## 环境与真实执行命令

Node `/home/lijx/.local/share/mise/installs/node/22.23.2/bin/node` v22.23.2，npm10.9.8；Go `/home/lijx/.local/share/mise/installs/go/1.27.3/bin/go` go1.27.3 linux/amd64，`GOTOOLCHAIN=local`。未安装或升级依赖版本。

已复查Chromium executable、Playwright模块存在，并审阅ENV02 cache.env，仅在测试子进程设置下面五个非凭据变量，未source或污染全局：

| 变量 | 值 |
| --- | --- |
| FONTCONFIG_FILE | `/home/lijx/.cache/relaynest-test-env/p0701-env02-1nvphuox/fonts.conf` |
| LD_LIBRARY_PATH | `/home/lijx/.cache/relaynest-test-env/p0701-env02-1nvphuox/root/usr/lib/x86_64-linux-gnu` |
| PLAYWRIGHT_MODULE | `/mnt/c/Users/lijx/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules/playwright/index.mjs` |
| CHROMIUM_EXECUTABLE | `/home/lijx/.cache/ms-playwright/chromium-1187/chrome-linux/chrome` |
| TMPDIR | `/home/lijx/.cache/relaynest-test-env/p0701-env02-1nvphuox/tmp` |

下表node/npm均使用以上固定Node PATH，go为上述完整路径。浏览器命令额外使用`env -u NODE_OPTIONS -u REAL_REQUEST_ONLY`、表内五变量及`RELAYNEST_BINARY=/tmp/relaynest-p0706-current`。每脚本自行生成临时SQLite/管理员随机密码/OpenSSL证书、生产后端和本地HTTPS可信代理，finally清理进程/文件。设备脚本所有页面静态资源也来自当前生产binary的embed；未mock真实验收响应。

| 实际命令 | 最终结果 |
| --- | --- |
| `npm --prefix web ci` | PASS，16 packages，0 vulnerabilities |
| `npm --prefix web run build` | PASS，tsc --noEmit + Vite7.3.7；最后业务变更后重新构建 |
| `node --test web/src/auth/tests/client.test.mjs web/src/devices/tests/client.test.mjs` | PASS，8/8；含204不读JSON、七字段、opaque URL、两种401、旧401不清新CSRF、clear后迟到restore |
| server目录 `go test ./internal/webui ./internal/httpapi ./cmd/relaynest` | PASS，当前Web构建后执行，三个包均通过 |
| server目录 `go build -o /tmp/relaynest-p0706-current ./cmd/relaynest` | PASS，当前Web构建后重编embed，再运行两浏览器脚本 |
| `node web/src/auth/tests/browser.mjs` | PASS，真实Chromium认证回归及独立协议fixture；手工Cookie jar结果另列，未替代browser |
| `node web/src/devices/tests/browser.mjs` | PASS，真实双context和以下矩阵；fixture另列 |
| `git diff --check`、提交前范围/diff/status核对 | PASS，仅本卡文件，无凭据/构建产物 |

## 验收证据

| 类别 | 实际断言与结果 |
| --- | --- |
| 真实设备缺失CSRF | A以真实Cookie对有效B精确Session.id发`DELETE /api/v1/devices/{id}/session`，无body/CSRF，403。随后B认证200、真实页面刷新且列表保留A/B；A认证200、真实刷新、仍登录。PASS，无撤销副作用。 |
| 真实设备错误CSRF | 同一路由，仅header固定错误值，403；独立重复B认证200/真实刷新/目标与A保留，以及A认证200/真实刷新/仍登录。PASS，无撤销副作用。 |
| 真实撤销隔离 | B页面撤A精确ID得到204；A下一真实设备请求401，页面刷新回登录，B继续真实刷新/认证200。两context重新登录交换方向复验。PASS。 |
| 真实自撤 | B撤自身204后立即回登录，浏览器Cookie消失；重新登录后A自撤同样通过。PASS，无需等待后续GET清状态。 |
| 真实同install | 仅临时服务通过既有Android登录接口种子同device_id的两个会话，丢弃凭据只保留ID；页面两行均保留，精确撤第一行204后第二行仍在。PASS，非route mock。 |
| 不返回Token/显示范围 | 真实设备响应每item恰为七字段；DOM不含完整A/B ID或token字段；客户端丢弃未知凭据字段；fixture无敏感message、HTML不执行，无附件/历史链接。PASS。 |
| 设备fixture（与真实验收分列） | opaque分页/末页、同install两行、恶意metadata纯文本、404/403/429/503保留可重试、DELETE401回登录、无效DTO/网络/空列表/恢复；延迟刷新遇退出/恢复切状态/页面导航不复活；直接调用真实mountDevices dispose后延迟响应根节点仍空。PASS。 |
| 原认证回归 | 真实登录失败密码清理、登录/恢复/Secure+HttpOnly+SameSite/存储为空/退出204/浏览器Origin；独立fixture403/401/429/到期/HTML转义。PASS。 |

执行期间明确失败及修正：环境无python3，最初编辑命令未执行，随即改用固定Node；首次分页fixture重复返回同cursor被生产校验拒绝，改为真实末页DTO后单测通过；首次browser启动早于Go构建完成导致临时初始化失败，等待构建完成重跑；首次browser helper在预期401回登录后仍等设备页超时，改为允许登录终态后完整重跑。未跳过或弱化产品检查，最终结果均来自实际成功重跑。

必跑项无NOT_RUN。未执行现网/发布/手机/NAS等卡外操作；无新决策请求。等待固定实现SHA独立审计及集成，未push、未创建子agent、未修改台账。
