# P08-04-D01-B01-SCOPE01-A01 独立规划审计

- subject_id：P08-04-D01-B01-SCOPE01；角色：audit；结论：**PASS**；findings：[]。
- reviewed_base_sha：`1aaaf77991a13f46a8cff26c8f59aa2b3d1a1c92`。
- reviewed_implementation_sha：`e988ea3ccaac43a9c3fc54b5b312a03102752cd1`。
- 工作树：`/home/lijx/Projects/RelayNest-recovery-20261024/instance-bot-scope`；分支：`codex/instance-bot-scope-20261025`；进入时 HEAD 为被审 SHA，工作区干净。

## 检查范围与结论

读取 AGENTS、CONTRIBUTING、WORKER_PROTOCOL；逐项审查固定 base→实现的四文档差异。对照 TECHNICAL_DESIGN §3.3 第154–162行、C01-I01及获批混合提案/A01局部；定点核查 bots/authorization.go、repository.go凭据查询局部、0004_bots.sql唯一约束及httpapi/router.go。代码、卡片及registry只读，本审计只新增此报告。

B01 原文按字节完整保留，包括 M01 依赖、全部验收及文件预算。BOT01只允许 authorization.go/测试及可选 api_identity.go/测试；仓储/storage只读，必须修改则回报阻塞。其不依赖父done或M01，父收口须消费其独审及精确集成证据；C01→S01→M01→B01与C01→BOT01→审集成→B01收口无环。A/I使用subject_id及review触发，预算/超限checkpoint与真实基线要求完整。

已批准政策未被重新提问或扩大：仅两附件可在ID前读取Bearer做API身份发现；摘要匹配即固定Bot分支，停用、能力/资源拒绝、撤销/代次失效及故障不回落。仅确定NoMatch进入ID→Admin，故障503及旧401→400/409优先级明确；其余Android仍ID先行。未把私有token形状变成wire协议，未引入表/索引/公开接口或业务/MCP执行。

现Authenticate确需botID，AuthorizeTx的Download仅检查receive；规划正确区分身份发现、事务授权与附件资源域。router当前没有附件路由；文档明确禁止造路由/桩资源假验收，未来须补真实HTTP+SQLite、send/receive资源范围、文件顺序、ID/auth_generation/凭据代次复核及无秘密日志证据。父原验收与未来门禁均未被内部适配PASS替代。差异未见秘密、虚构外部接口/vendor事实或运行通过声明。

## 实际检查与限制

以下均在本工作树实际执行并通过：

- `git rev-parse HEAD`、`git branch --show-current`、`git status --short`：固定SHA/分支匹配且初始干净。
- `git diff --name-only <base>..<implementation>`：恰为规定四文档；`git diff --check <base>..<implementation>`退出0。
- `git show <base>:docs/tasks/cards/P08-04-D01-B01.md`导出临时文件，以`head -c`/`cmp`核当前B01原文前缀：PASS。
- shell逐项提取四文档Markdown相对链接并`test -f`：全部存在；局部DAG及冻结语义人工逐项核对通过。

NOT_RUN：Go/race/静态检查、SQLite/HTTP/API validator、Web/Android/WS、phone/NAS/签名与阶段验收。本轮仅规划文档，无运行时代码或机器契约变化；PASS仅覆盖规划范围，不豁免未来实施必跑检查，不表示B01、附件运行时或父gate通过。未复现原实施报告的工作树创建过程，不将其历史叙述当作运行时证据。无push、部署、新聊天或额外agent。
