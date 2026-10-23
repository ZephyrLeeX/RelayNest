# P04-03-PLAN01 来源基础前置与无环依赖规划

## 交付边界

- task_id：P04-03-PLAN01；role：planner；outcome：implemented（文档规划待独审/集成，非来源实现通过）。
- 精确基线：`b2d4d5fa8442f39f78d9fcbd3c4feba37fb20d8f`；分支：`codex/origin-plan`；工作树：`/home/lijx/.codex/worktrees/origin-plan/RelayNest`。实施 SHA 在提交后交接，不在提交内预写自身 SHA。
- 仅修改 P04-03、P04-02-S02，新建 P04-03-S01 及本报告；不改源码、API、设计、registry 或旧报告，不 push。

## 证据与缺口

1. [ARCHITECTURE §5](../../ARCHITECTURE.md) 明确 Web 同源 Cookie+CSRF、公网 HTTPS 在 OpenWrt 终止、内网 HTTP，以及仅配置代理地址可信；伪造头不能改变外部 Origin、客户端地址或 Secure 判断。[TECHNICAL_DESIGN §7.1](../../TECHNICAL_DESIGN.md) 明确外部 HTTPS 的 Secure、写操作 Origin/Referer+会话 CSRF、登录 Origin。没有据此增加自动可信环回/私网、HTTP Web 登录或新的域名白名单。
2. [OpenAPI](../../../server/api/openapi.yaml) `/auth/login` 与 AdminBearer/WebCookie/CSRF 组件保持上述边界；CSRF 不适用于无会话登录，Android 与 Web 凭据隔离。本规划不改接口或认证期限。
3. 固定基线 `server/cmd/relaynest/main.go` 仅 `-listen`（默认 127.0.0.1:8080）及 health 启动；`server/internal/httpapi/router.go` 仅健康/静态挂载，没有来源配置/解析注入入口。此前只读符号核查未发现来源安全模块；本次定点复核两个入口，不读取动态会话开发树。
4. 原 [S02](../cards/P04-02-S02.md) 第5项要求最低来源保护，第6项明确缺基础依赖须补卡；原 [P04-03](../cards/P04-03.md) 却依赖 P04-02 完成。继续让 S02 等父卡会留下安全缺口或环，因此只抽取无 session/DB 的基础，父卡原工作与全部原验收保留。
5. Origin 三元组、默认端口及序列化参考 [RFC 6454 §4/6/7](https://www.rfc-editor.org/rfc/rfc6454.html)；直接 peer、TLS 状态、Host 的字段职责参考 [Go net/http Request](https://pkg.go.dev/net/http#Request)。单一 Origin、单值覆盖式转发协议与歧义拒绝是落实既定边界的最小实现方案，不能宣称 RFC 为本项目定义了 X-Forwarded-* 协议。

## 拆分方案及适用范围

[P04-03-S01](../cards/P04-03-S01.md) 完成显式 HTTPS Origin/代理配置、真实直接 peer 判定、严格单层转发解析、外部来源/客户端 IP/HTTPS 成功结果、无会话的浏览器来源校验及反例；仅最小 httpapi 专用模块与测试。配置是权威比较基准，请求 Host/头只是候选；无效结果不得回落。空代理列表不信任任何代理。

选取单一外部 HTTPS Origin 与覆盖式 X-Forwarded-Host/Proto/For 的原因：已有确认部署是 OpenWrt HTTPS 终止后内网 HTTP，A5 点名 X-Forwarded-*，但没有多域名、多层代理或 RFC Forwarded 的已确认需求。用三头明确携带外部 authority/HTTPS/客户端 IP，可信 peer 与配置 Origin 双重约束；拒绝多 hop、重复、缺失、混用，避免猜选首尾造成信任扩大。非代理合法 TLS 直连仍可匹配同一配置；伪造转发头不改变其判定。本方案不声称兼容所有 Nginx 默认配置，部署代理必须验证覆盖而非追加头；若出现已确认的其他拓扑，交调度核证后补方案，不能临场宽松解析。

来源比较保留有效端口，默认/显式 443 等价；登录要求 Origin，Cookie 写操作缺 Origin 才回退 Referer，不能用 Referer 覆盖坏 Origin，双头冲突拒绝。S02 使用同一模块，保留会话 CSRF 与凭据类型约束；父 P04-03 在 P04-02 后执行完整 Cookie/CSRF/来源组合验证。基础单测不代表登录/会话/部署通过，未实现业务路由不能标为实测。

## S02 配置注入与运行影响

- S01 先提供程序化配置构造/校验和解析器，实际签名在实现独审、精确集成后交接；不预冻结未来函数名。
- S02 在现有 main 启动路径读取显式外部 Origin 与可信代理配置，再构造来源依赖，注入认证 handler/router。建议新增 `-external-origin` 字符串和可重复 `-trusted-proxy` IP/CIDR 参数；命名/类型可按工程合理调整，须在报告注明。空可信列表合法，未配置任何代理；不从监听地址推导外部 Origin。
- 为新认证运行接线要求显式有效外部 HTTPS Origin：缺失/非法配置在启动依赖构造时返回配置错误，不能挂载缺保护的认证路由。其影响是接入认证后的服务启动需补该配置；既有 `-listen` 含义/默认值保持，不添加认证开关、默认域名或默认 DB 路径，不改管理员子命令语义。S02 补启动正反例及健康/静态回归证据，并复用已集成认证生命周期；若实际启动边界另有已确认约束，再以具体差异交调度。
- 真实部署 Origin/端口、直接代理 IP/CIDR 清单、代理覆盖配置及 NAS/独立测试资源在部署前提供和验证；当前可用显式 HTTPS fixture 和可信代理 HTTP 入站/TLS 直连测试推进，不要求用户现在给域名，不写真实凭据/资源到仓库。

## registry 登记建议（仅 root 执行）

| ID | kind / parent | dependencies | gates / 触发 |
| --- | --- | --- | --- |
| P04-03-PLAN01 | planning / P04-03 | P03-07 | G-GIT、G-API；本规划 A/I 通过后 done |
| P04-03-S01 | implementation / P04-03 | P03-07、P04-03-PLAN01 | G-GIT、G-API；前置 done 后派发 |
| P04-02-S02 | 原记录保留 | 保留 P04-02-S01，再加 P04-03-S01 | 原 gates 不删 |
| P04-03 | 原记录保留 | 保留 P04-02，再加 P04-03-S01 | 原 gates 不删 |

依赖方向为 `P03-07 + PLAN01 → 来源S01 → 认证S02 → P04-02 → P04-03`，认证S01 的原前置和所有父原验收继续保留。父子归属不是额外 done 前置，PLAN01/来源S01 都不依赖 P04-03 父完成。各 A/I 用 subject_id+精确 SHA 的 review/PASS 条件触发，不反向依赖被审卡 done。建议登记不替代 root 核实际台账，也不改变其他卡。

## 验证、未决项与交接

- 文档检查：提交前核四文档 diff、相对链接、允许范围及依赖无环；命令/结果随下方验证记录。
- 业务测试：not_run，文档规划未修改业务代码；来源解析、配置读取、Cookie/CSRF/HTTP 生命周期均待对应实现卡运行，不以提案冒充测试。
- 重要安全/产品政策决定：未新增用户政策请求。单一 HTTPS Origin/单层代理为本卡实现适用范围，独审若发现与真实已确认部署需求冲突或安全语义变化，须带证据交 root，不自行扩大信任。真实配置值与部署覆盖证据未具备，不阻本地基础实现，阻部署验收。
- 下一步：另一技术 agent 独审本规划精确 SHA；PASS 后精确集成、root 唯一登记；来源 S01 先独审集成，认证 S02 再消费，P04-03 最后原验收收口。

### 提交前验证记录

- `git status --short`、`git rev-parse HEAD`：开始时工作区/index 干净，HEAD 与精确基线一致；最终 stage 仅上述四份文档。
- `git diff --check` 与 `git diff --cached --check`：通过；后者包含两份新文件。已阅读两份原卡 diff，原职责/最低安全要求和生命周期验收未删除。
- `perl - <<'PL'` 内联断言：四文档全部相对 Markdown 链接存在；两张原卡的每条原验收逐行保留；报告列出的新增依赖子图无环，全部通过。这不是全 registry 扫描，台账精确登记仍由 root 核验。
- 同一内联检查最初尝试 `python3` 返回 command not found，改用系统已有 `/usr/bin/perl` 成功；未安装依赖或修改环境。业务测试按上文 not_run。
