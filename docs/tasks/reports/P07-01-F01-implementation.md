# P07-01-F01 默认原生 fetch 接收者修复

- 角色：修复开发；只处理 `P07-01-A01-F01`（P1），等待独立 A02 审计，不自行销单。
- 基线：`b652e203c516edf185e3f9e1eb3918c858d824ec`；工作树 `/home/lijx/.codex/worktrees/web-auth/RelayNest`，分支 `codex/web-auth`，开始 clean。最终修复 SHA 随提交后交接。
- 只修改 `web/src/auth/client.ts`、`web/src/auth/tests/client.test.mjs` 及本报告。默认 transport 改为 `fetch.bind(globalThis)`，使成员调用仍使用浏览器全局接收者；显式注入 transport 保持原行为。同源路径、credentials、内存 CSRF、错误壳层和密码不持久化策略保持原样。

## 前后验证

先只新增回归、保留旧 client：普通函数模拟浏览器 fetch 的接收者检查，覆盖默认客户端 login/restore/logout，验证同源 credentials 与退出 CSRF，并在 finally 恢复全局 fetch。原 4 项测试均保留。

| 检查 | 实际结果 |
| --- | --- |
| `node --test web/src/auth/tests/client.test.mjs`，新增回归但未修源码 | FAIL，4/5 PASS；新回归 actual 为 AuthClient、expected 为 globalThis。使用 dot reporter 复跑确认退出 1；这只是 receiver 回归证据，不替代原生浏览器 |
| 同命令，单行修复后 | PASS，5/5，退出 0 |
| `npm ci --prefix web --no-audit --no-fund` | PASS，16 packages，退出 0，锁文件未变 |
| `npm --prefix web run build` | PASS，含 `tsc --noEmit`，Vite 7 modules，退出 0 |
| `GOTOOLCHAIN=local /home/lijx/.local/share/mise/installs/go/1.27.3/bin/go -C server build -o /tmp/p0701-f01-relaynest ./cmd/relaynest` | PASS，前端构建后重建当前源码的生产入口，退出 0 |
| 下方原始完整 browser.mjs | PASS，退出 0，三组 PASS 输出全部出现 |
| `git diff --check`、限定 diff/status | PASS；无脚本、断言、依赖、后端、注册表及其他 UI 变更 |

Node/npm 命令的 PATH 前置 `/home/lijx/.local/share/mise/installs/node/22.23.2/bin`；实测 Node v22.23.2、npm 10.9.8、Go go1.27.3 linux/amd64，Go 使用 `GOTOOLCHAIN=local`。

## 浏览器环境与复现

2026-10-23 Linux amd64。使用调度提供的 ENV02 私有缓存 `/home/lijx/.cache/relaynest-test-env/p0701-env02-1nvphuox`；先只读核对 `cache.env`，再显式传进程变量。ENV02 恢复的是 [ENV01](P07-01-ENV01.md) 清单的原 34 个官方 Debian trixie/main 固定包和精确版本（8,529,408 bytes），来源 `https://deb.debian.org/debian`。环境恢复者提供固定包/SHA/size 校验及 ldd/字体 smoke PASS；本轮未重复下载或系统安装。缓存保存 `packages.txt`、`verified-packages.txt`、`debs.sha256`；独立环境证据见 ENV02 报告。Chromium 140.0.7339.16 / Playwright 1.62.1 为环境恢复者核验值。

树根实际执行命令如下；未添加 preload，也未修改原脚本或放宽断言：

```sh
env -u REAL_REQUEST_ONLY -u NODE_OPTIONS \
FONTCONFIG_FILE=/home/lijx/.cache/relaynest-test-env/p0701-env02-1nvphuox/fonts.conf \
LD_LIBRARY_PATH=/home/lijx/.cache/relaynest-test-env/p0701-env02-1nvphuox/root/usr/lib/x86_64-linux-gnu \
PLAYWRIGHT_MODULE=/mnt/c/Users/lijx/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules/playwright/index.mjs \
CHROMIUM_EXECUTABLE=/home/lijx/.cache/ms-playwright/chromium-1187/chrome-linux/chrome \
TMPDIR=/home/lijx/.cache/relaynest-test-env/p0701-env02-1nvphuox/tmp \
RELAYNEST_BINARY=/tmp/p0701-f01-relaynest \
/home/lijx/.local/share/mise/installs/node/22.23.2/bin/node web/src/auth/tests/browser.mjs
```

构建输入为上述基线加本次 client/test diff，执行完成至提交前未再次修改产品源码。SHA-256：

| 输入/产物 | SHA-256 |
| --- | --- |
| 原始 `browser.mjs` | `77d7275d079a95dc58ff253b5b2e42c0f945d14b33bbf2e4425ed6724dedce7f` |
| 修复 `client.ts` | `12d1671a541cc2516c57f2c5d4c26e37a5fba6adf5cc30ba7ecd4416b63a04ee` |
| `client.test.mjs` | `2f51de31f5fb76bfcfe56075186e5e04cad7816f96b98e37cd838eece5244007` |
| `/tmp/p0701-f01-relaynest` | `f6bfe33c470c1ef4f2e0451c2b101f998fbab2d3323c1c89dbed05b2a8293b16` |

## 证据边界与遗留

- 真实生产后端 + 临时 SQLite/随机测试管理员 + 本地 HTTPS/trusted proxy，手工 Cookie jar：login200/session200/无 CSRF403/logout204/旧会话401、安全 Cookie 响应头 PASS。这部分不是浏览器证据。
- 同一生产后端的真实 Chromium：错误密码提示及密码清空、登录、刷新恢复、HttpOnly/Secure/SameSite=Lax、JS 不可读 Cookie、localStorage/sessionStorage 为空、CSRF403、退出204与 Cookie 移除、浏览器 Origin/CSRF 发送 PASS。
- 原脚本协议 fixture：403 安全错误壳层、401 过期、429、空闲过期、HTML 文本转义、无敏感服务端文案及无 pageerror PASS。这部分为协议桩，不能宣称真实后端过期/限流全链路通过。
- 测试使用私有临时数据库、随机密码、自签本地证书、独立浏览器 context，结束由原脚本清理；无真实账号、生产/profile 操作，凭据及原始敏感日志不入库。
- 独立 lint：NOT_RUN，项目无独立 lint 配置，使用已有 tsc/build。正式 NAS/OpenWrt/手机、正式证书链、OS 休眠及正式完整 AC：NOT_RUN；不扩张为正式环境验收完成。
- 无需新增需求或架构决策。交调度按最终修复 SHA 独立 A02 重审并精确集成；本轮不 push、不修改父卡/registry。
