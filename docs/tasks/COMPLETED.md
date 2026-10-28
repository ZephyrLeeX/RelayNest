# 已完成任务记录

核对日期：2026-10-26。压缩前历史基线：`086fa51d79d7726545a9abbab8feebb25770034e`。本页将原任务卡、实施、修复、独审和集成记录按交付结果汇总；局部已完成不扩大为父功能、阶段、部署或正式验收完成。

历史原件统一恢复：`git show 086fa51d79d7726545a9abbab8feebb25770034e:docs/tasks/<原路径>`，例如 `...:docs/tasks/reports/P05-01-I01.md`。该提交是本项目主历史祖先，可随 Git 历史恢复；不另外保存旧卡、重复报告或 archive 副本。原台账全部 331 条 `done`（含规划、修复、审计、集成、工作区准备）按下列 ID 前缀聚合；过程任务不另计产品功能。精确历史元数据仍在该基线的 `docs/tasks/registry.json`。main 集成提交及该基线文件可从主历史检索；表中的原始实现／独审来源提交不一定是 main 祖先，不能保证 fresh clone 可直接解析。分支清理后的来源对象可从 `/home/lijx/Projects/RelayNest-backups/branch-cleanup-20261025T114418.040950Z/all-local-heads.bundle` 恢复后只读核对；这是本地历史备份，不是运行依赖。

## Android

<a id="a1"></a>

### A1 登录局部基础

- 原 ID：`P08-04-S01～S03` 及 `P08-04-PREP01/PLAN01`、其审计/集成链；原生工程 `P03-03` 归 E1。
- 已完成 HTTPS 地址与三端点网络边界、安装身份、加密会话存储、登录/恢复/退出及入口 UI。本地恢复必须验证真实 session 再发布认证状态，票据及代次隔离迟到响应；endpoint 绑定不等于服务端实例身份。
- 最终局部组合：Android 42 项单元测试、lint 0 errors/10 warnings、Debug APK 组装通过。测试中的状态、文件和网络夹具不等于真实服务端组合。
- 未测移交：生产 TLS 三端点、真机/平板 Compose、实际 Keystore/硬件保障、设备密钥失效、卸载重装、备份迁移、进程杀死/断电、成功 release 签名。没有承诺目录 fsync、Keystore/文件单事务或跨进程同步。用户已将真机运行留到最终验收，不重复索取 ADB。
- 父 `P08-04`、S04、Android 新 G1 接入及正式组合仍未完成；下方 G1 已批准决定继续有效，不沿用旧 S03 报告“仍待用户确认”的历史状态。

<a id="a2"></a>

### A2 FCM 最小 PoC

- 原 ID：`P00-04`、`P02-01～P02-04` 及 A/I。G-FCM 按 E01～E07 限定放行。
- 一加 Ace 3 Pro（PJX110）、Android 16/API36、ColorOS16 有真实 data-only 提示证据；打开应用补偿及 WorkManager 30 分钟周期/网络约束、重启调度与模拟 `last_sync` 落盘有证据。它们不证明真实 Hub 最终同步或 AC-09。
- Google 不可达退化为打开即同步＋周期补偿，不能承诺实时；ColorOS 自启动未开与 force-stop 必须分别处理。默认 normal，HIGH 只服务确需及时且用户可见的提醒，不得把所有提示升 HIGH；积压冲刷及 `onDeletedMessages` 不构成可靠补偿。
- 未测移交 P9/P11：长窗睡眠、Wi-Fi/蜂窝切网、TTL、通知权限/登录退出、`onDeletedMessages` 实际触发和正式通知行为。失效 token 仅按发送快照条件解绑对应通道注册，不能删除设备会话；原早期报告“删设备记录”不得作为正式实现依据。
- `P02-05`/G-OPPO 尚未通过，缺账号/应用权限及审核条件；不阻塞独立开发。已批准 OPPO 边界见 E2。

## Go 后端（Bot Hub）

<a id="g1"></a>

### G1 管理员、设备与 Bot 基础

- 原 ID：`P04-01～P04-07` 及其中已完成的子卡、修复、A/I；不含未完成 `P04-08`，也不含未完成的生命周期 HTTP 子项 `P04-07-S03`（见 [BE-03-01](BACKEND.md#be-03-01)）。
- 已完成唯一管理员初始化、密码摘要、会话生命周期/登录过期及认证代次基础；Web 同源、CSRF、可信代理、登录防护和敏感日志过滤。
- 设备本地 HTTP/SQLite 组合完成安装元数据、分页、精确 session 撤销、当前会话处理及隔离/安全矩阵；不代表真实 WS/provider/phone 或新 G1 已验。
- Bot 完成元数据、独立凭据/授权、三管理路由、轮换/启停扩展与名称版本确认、基础永久删除和 typed mark。变更与 changes 使用同事务；原凭据与 Webhook 边界保留。
- 真实文件 SQLite、HTTP/路由、并发与回滚、相关包和 race/vet/build 的局部证据已审计集成。完整生命周期 HTTP/UI、租约/领取/等待/投递队列级联、身份权限总矩阵与正式 AC 尚未完成。

<a id="g2"></a>

### G2 消息事务仓储

- 原 ID：`P05-01`、S01～S03、PLAN01/02、SCOPE/ALLOC、A/I；父卡原验收已独审并收口。
- 消息及序列、全局 changes、最小 delivery/push 待办同事务提交；稳定消息回执、不可变投递快照、真实 Android session push 目标及 MessageUpsert/DeliveryUpsert 有持久化断言。
- 已验证真实 SQLite 语句/Append/回填/队列及 deferred-FK COMMIT 失败、主动回滚、跨连接/重开、同/多 Bot 并发和 int64 边界；承诺已提交序列单调。最终组合复用 M01 同源码全 Go 包、五包 race、vet/build 的实际通过结果，未声称重复新跑。
- 这不表示正式消息 HTTP、同步、完整幂等、引用/历史/共享已读、编辑/附件，或 P6 调度/租约/ACK/重试、正式推送完成。

<a id="g3"></a>

### G3 幂等创建存储基础

- 原 ID：`P05-02-S01`、SCOPE/ALLOC、PLAN01 及 A/I；父 `P05-02` 仍未完成。
- 稳定身份/操作/UUID 键作用域、同键重放原创建快照/回执、异内容冲突、同事务计数、无正文墓碑及 Bot 永久删除清理；`0007_idempotency.sql` 已精确集成。内部冲突/墓碑/过大错误留给后续 HTTP 409/410/413 映射。
- 五包普通及 race、vet/build、storage 额外复验、三轮并发/重开/九类故障与真实 COMMIT 失败、删除成功/失败回滚通过；0001～0006 迁移及 checksum 不变。
- 本节存储结果不代表正式 HTTP；Bot 当前有限入口已另见 [BE-02-BOT](#g5-be02-bot)，Android 管理员入口、G1/浏览器/发布仍未完成，不以协议桩代替。

<a id="g4"></a>

### G4 实例身份存储与离线工具、内部 Bot 身份发现

- 原 ID：`P08-04-D01-S01`、`M01`、`B01-BOT01` 及 scope/alloc/A/I；`B01-PLAN02` 只完成实施规划，不表示绑定运行时完成。契约和用户决定见 E2。
- 存储完成独立随机实例 UUID 原子初始化、严格读取和失败关闭；已初始化 ID 缺失/损坏不自动修复。
- 离线工具完成无迁移打开、进程锁、换 ID/认证代次推进及全管理员会话撤销；恢复/克隆清理推送待办，正常移动保留。真实六迁移临时库 `pending=1` 组合及失败回滚、普通/race 通过；未操作现网。人工停服、入口隔离和正确操作模式仍是前置。
- 内部身份发现区分 NoMatch/Matched/Unavailable；完整摘要匹配后须业务事务再 `AuthorizeTx`，停用/降权/代次/轮换/撤销等拒绝不得回落管理员，依赖故障失败关闭。真实 SQLite、并发轮换/取消/超时、相关包/race/vet/build 通过。
- 未测移交：摘要匹配后读事务 COMMIT 故障定点注入；真实附件 HTTP 顺序、状态/no-store、send/receive 资源域及文件打开顺序。已有写事务 COMMIT 失败不能冒称覆盖上述读事务分支。
- AUTH 子范围现已独审并集成实际目标 work，见 [BE-01-AUTH](#g5-be01-auth)；完整 `B01`、HTTP01、G-INSTANCE-RUNTIME/COMPOSITION、Android 新 G1/S04 仍未闭环。

<a id="g5-be01"></a>

### G5 BE-01 现有后端实例绑定父范围

- 子项完成引用：[AUTH 已完成独审与精确集成](COMPLETED.md#g5-be01-auth)；[HTTP01 已完成独审与精确集成](COMPLETED.md#g5-be01-http)。以最终代码组合 `cf2c8b49b4b5338e58e4b9d6e9698e5e648bdc31`／元数据快照 `373cc241317410a8b8aa0755f9a41bb2ca676499` 完成 [COMPOSE 独立组合验收](#g5-be01-compose)。本父范围仅公开 probe＋八个现有后端 auth／device／admin-bot 路由；Android 客户端、真机及跨端全局门槛仍未完成。

- 旧ID：`P08-04-D01-B01`；类型：父收口，不整组派发；状态：`done`（2026-10-26，主调度依据 AUTH／HTTP 完成及 COMPOSE R01／A01／I01 齐全证据归档既定现有后端父范围）。
- 依赖：[已完成基线 P08-04-D01-M01](COMPLETED.md#g4)、[已完成基线 P04-05-S03](COMPLETED.md#g1)、[已完成基线 P04-06](COMPLETED.md#g1)、[已完成基线 P08-04-D01-B01-SCOPE01](COMPLETED.md#g4)、[已完成基线 P08-04-D01-B01-BOT01](COMPLETED.md#g4)、[已完成基线 P08-04-D01-B01-PLAN02](COMPLETED.md#g4)、[已完成 BE-01-AUTH](COMPLETED.md#g5-be01-auth)、[已完成 BE-01-HTTP](COMPLETED.md#g5-be01-http)。
- 门槛：`G-GIT`、`G-API`、`G-INSTANCE-CONTRACT`；条件未实证不记通过。

原父收口验收：公开 probe 与 Android 登录/会话/设备/Bot 管理真实路由完整 G1 组合；依赖中的 M01/SCOPE01/BOT01/PLAN02 仅是已完成局部基线。AUTH 和 HTTP 两包及 BOT01 最终集成后，在同一组合 SHA 逐项验收；不以 AUTH 局部通过放行消费者。当前 G-INSTANCE-RUNTIME、G-INSTANCE-COMPOSITION 均 pending。

G1 已批准生命周期：Android 先验证实例 ID 再读取密码/Bearer，授权读同一事务快照，写提交前复核 ID/auth_generation；ID/会话/代次变化不得提交部分结果。probe 后诚实切库仍可能使旧凭据字节抵达 B，这是已接受的有限窗口，不宣称 G2。未来未实现业务/附件/WS 路由保留强门禁及 NOT_RUN。

附件仅 uploadAttachment/downloadAttachment 是 Bot-first 例外：格式校验后可先读 Bearer 用于真实 API 凭据发现，不能提前读上传流/Range/附件资源；NoMatch 才进入 Android ID 校验，Matched 后拒绝不回退管理员，Unavailable 失败关闭503。下载不能先查附件求 BotID；send/receive 必须按资源域授权。有效 Bot 忽略实例头；未来真实两附件路由、授权及文件打开顺序由 BE-05 验收，不能拿内部发现已完成替代。Web Origin/CSRF、设备精确撤销/自撤、Bot 与 health 非回归，日志禁头/正文/实例ID。

- 最终收口：受测元数据 `373cc241317410a8b8aa0755f9a41bb2ca676499`／业务 `cf2c8b49b4b5338e58e4b9d6e9698e5e648bdc31`，研究快照与准确组合核实均为 `7d2fcec4a534d32816dfa49aff288d04a4c49dfe`；[COMPOSE R01／不同 A01／I01](#g5-be01-compose) PASS。当前现有后端范围完成；全局 G-INSTANCE-RUNTIME／G-INSTANCE-COMPOSITION 仍 pending，Android 完整 call 5 秒／1024 字节、真机、未来附件／WS 路由及反向代理缓存策略均 NOT_RUN，不宣称 G2。未来真实路由仍须逐项接入门禁并验收。

<a id="g5-be01-compose"></a>

### G5 BE-01-COMPOSE 最终实例绑定组合验收

- 类型：有限验证 leaf；状态：`done`（2026-10-26，主调度依据 R01 PASS、研究快照、不同 A01 PASS 与 I01 准确组合核实归档）；owner Backend Orchestrator；Researcher `/root/instance_compose_verify`；固定基线 `373cc241317410a8b8aa0755f9a41bb2ca676499` / `work`，其业务代码最终组合为 `cf2c8b49…`。不以父 BE-01 done 为前置，避免验收依赖环。
- 历史派发准备：Backend Researcher，仅验证上述父完整 G1 标准和八个真实 auth／device／admin-bot 路由、公开 probe 与 Web／Bot／health 非回归；源码只读，允许 `/tmp` 固定 SHA 归档／临时验证 overlay，不改业务／任务文档／契约／迁移。真实当前组合逐项收口，不用子卡报告替代新组合证据；不重复已由 I01 同代码验证通过的无关全服矩阵。
- 验收：真实生产装配＋SQLite 证明 ID 先于密码／Bearer、同快照读和写前代次复核、错误优先／安全响应、A→B／事务中变化无部分结果；公开 probe 格式／限流／缓存／非规范路径安全；设备精确撤销／自撤与全撤、Web Origin/CSRF、Bot 发现与文本创建、日志无头／正文／实例 ID。接受既定 G1 瞬切窗口，不声称 G2。
- 未来附件／WS 路由、Android 完整 call 5秒／1024字节、真机与跨模块完整验收仍逐项 NOT_RUN，不修改其他模块状态。研究返回固定 SHA／真实命令／逐项 PASS/FAIL/NOT_RUN；结果由主调度写回原条目并本地快照后交不同 Auditor 固定 SHA 审查，再由 Integrator 核实准确组合；全部齐全才关闭父任务。
- R01 结果：`PASS`，仅现有后端 G1 范围；固定 `373cc241…` 归档 `/tmp/be01-compose-373cc241/src`，server/web 与业务组合 `cf2c8b49…` 无差异，锁版离线 Web ci/build PASS。独立真实生产装配＋TLS＋SQLite 补测八路由严格实例头／非法密码优先、A→B、读快照不拼接、auth/device 五路由及 Bot POST/PATCH 事务 ID／代次／撤销／延迟 FK COMMIT 故障无部分结果，probe 格式／限流优先／安全头／别名拒绝，设备精确撤销／自撤／全撤、Web/CSRF、Bot 发现／文本创建、health/static 与生产日志敏感标记均 PASS。
- R01 实测：针对生产 cmd 组合及 httpapi/auth/devices/bots 的普通测试、五包定向串行 race、补充 devices HTTP race、相关 vet 均退出 0；未重复无关全服矩阵。五包首条 devices 无匹配项已单独补实测；临时 overlay 初次撤销机器码预期写错而 FAIL，按冻结 REQUIRED 修正 `/tmp` 断言后重跑 PASS，无产品缺陷，原日志保留。命令及日志 `/tmp/be01-compose-373cc241/{production-rerun.log,targeted.log,race.log,devices-race.log,vet.log}`，补测源码同目录 `src/server/cmd/relaynest/compose_independent_test.go`。
- 门槛范围核对：ENGINEERING 第 13～14 行与 ANDROID 第 26～27 行的全局 G-INSTANCE-RUNTIME／COMPOSITION 包含 Android 专项及跨端组合，本结果仅支持 BE-01 现有后端父范围，不将两全局门槛置为通过。Android完整call／真机、未来附件HTTP／WS、反向代理缓存策略等 NOT_RUN；既定 G1 瞬切窗口保留，不宣称 G2。研究快照、独立 A01 及准确组合 I01 证据现已齐全。
- 研究结果本地快照：`7d2fcec4a534d32816dfa49aff288d04a4c49dfe`（BE-DOC-S04，仅原 BACKEND 元数据）；不同 A01 `/root/instance_compose_audit` 固定此快照核代码与受测 `373cc241…` 相同及 R01 证据真实性，最终 PASS 见下文。
- A01 结果：Auditor `/root/instance_compose_audit` `PASS`，仅既定现有后端 G1 父范围，无新增产品问题；固定 `7d2fcec4…`，核 120 个受控源码与 R01 归档逐字节一致，server/web 与 `373cc241…`／`cf2c8b49…` 无差异。独立生产普通／定向 race、probe 三协议安全矩阵、进程装配、Bot 写 COMMIT 故障、vet 与锁版 Web 均 PASS，命令与退出码 `/tmp/be01-compose-a01-7d2f/commands.txt`。
- A01 证据补齐：R01 原生产日志可能空缓冲，Auditor 已独立强制非空真实登录日志且敏感值不入日志；原严格头普通 rerun 未覆盖新增项，A01 普通与 race 已覆盖；R01 退出码源于实际工具记录而非独立文件，不虚称存在。全局两实例门槛及跨端／未来路由等仍 pending／NOT_RUN，既定 G1 窗口不变。
- 历史 I01 派发：Agent `/root/http_integrate_i01` 执行有限 BE-01-COMPOSE-I01，只核准确受审组合已在目标 `work` / `7d2fcec4a534d32816dfa49aff288d04a4c49dfe`，不重复 cherry-pick／不改业务；校验基线、祖先、代码／构建输入同版本和已运行必检证据、未提交 BACKEND 保护。当前源码无新增变更，无需重复无关矩阵；主调度自本记录后冻结文档至其释放，父任务尚不标 done。
- I01 结果：Integrator `/root/http_integrate_i01`（BE-01-COMPOSE-I01）`PASS`，准确组合已在目标，前后均 `7d2fcec4a534d32816dfa49aff288d04a4c49dfe`，no-op 无新提交／冲突；AUTH／HTTP 映射 range-diff 全部 `=`，120 受控源码与 R01／A01 归档逐字节相同，embed 输入产物亦同，无业务／依赖／构建漂移。同版既有必检证据逐项复核，本轮不重复运行时矩阵；BACKEND patch 完整保留，index 空，锁已释放。R01／A01／I01 与本地证据快照齐全，主调度据此归档 COMPOSE 及 BE-01 既定现有后端父范围，全局跨端两 gate 仍 pending。

- 归档：BE-DOC-S05 `/root/storage_archive_resume` 仅转录主调度完成决定与既有证据；没有新增业务实现或重新运行运行时矩阵。

<a id="g5-be01-auth"></a>

### G5 BE-01-AUTH Android 实例入口与事务绑定

- 原 ID：`P08-04-D01-B01-AUTH01`；类型：有限 leaf；状态：`done`（2026-10-26，由主调度依据 A02 PASS 与 I01 精确组合证据从 integrating 归档）。仅完成 AUTH 子范围，父 BE-01、HTTP01、G-INSTANCE-RUNTIME／COMPOSITION 仍未完成。
- 依赖：[已完成基线 P08-04-D01-M01](#g4)、[已完成基线 P08-04-D01-B01-PLAN02](#g4)。
- 门槛：`G-GIT`、`G-API`、`G-INSTANCE-CONTRACT`；条件未实证不记通过。

- 基线与恢复：历史实施基线 `5519ebfee27e3ad74145466eb27051945804f07e`，源实现 `d894f14175249bbd17609a0eb3521462577151c8`（历史分支 `codex/instance-server-bind-20261025`）。R01 `/root/backend_recovery` 从远端非破坏恢复并准备固定 SHA 隔离树；目标 `work` 初始 `01c74a776b0b76fcc6616f60ae637a5a53f18384`，M01／PLAN02 均已集成。
- 短历史：A01 `/root/auth_audit_a01` 给出 CHANGES_REQUIRED，`BE-01-AUTH-A01-001` 为旧 Bot／Device／装配测试缺真实实例头导致全服普通检查 FAIL；追加 `BE-01-AUTH-A01-002` 为闭库场景旧断言误用 SERVICE_UNAVAILABLE，冻结身份错误应为 INSTANCE_UNAVAILABLE。未确认生产 AUTH 缺陷。F01 `/root/auth_fix_f01` 提交 `39529d3d3f0f5b070af2237d6361025d436de20e`（parent `d894f141…`），仅修 `server/internal/bots/lifecycle_auth_test.go`、`server/internal/devices/http_test.go`、`server/cmd/relaynest/main_test.go`，保留原权限／回滚断言及一般会话存储故障覆盖，75 插入／5 删除。
- 最终独审：A02 `/root/auth_audit_a01` 固定 `39529d3d3f0f5b070af2237d6361025d436de20e` PASS，关闭 A01-001／002；原九 AUTH 文件不变。独立全服普通、相关五包 race、全服 vet、十二文件 gofmt/diff、锁版 npm ci/build PASS。
- 精确集成：I01 `/root/auth_integrate_i01` 在目标 `work` 无冲突 cherry-pick，`907c2a4dbf3a8774bcd438fb22b2381def7394a5` → `1aa0aa9e0c390a46a7293cae56410e0baae8ce71`；映射 `d894f14175249bbd17609a0eb3521462577151c8` → `c2e978323d364b39bbab72dd494f40dac1a4dfae`、`39529d3d3f0f5b070af2237d6361025d436de20e` → `1aa0aa9e0c390a46a7293cae56410e0baae8ce71`。range-diff 两提交均 `=`，无额外业务修改。
- 组合验证：全服普通、vet/build，auth/httpapi/devices/bots/storage/messages/idempotency/cmd 八包串行 race，锁版 Node 22.23.2／npm 10.9.8 ci/build、gofmt/diff 和目标关键路由 PASS；主调度两文档 patch 受保护。一次 gofmt 误用目录造成 lstat 失败，正确重跑 PASS，不是产品 FAIL。全服 race、HTTP01 与完整 G1 父验收 NOT_RUN，不扩大局部完成结论。

允许五个 auth 主文件：http.go、request_security.go、session.go、device_sessions.go、instance_binding.go 及测试。实现不可伪造的私有请求绑定、同 SessionStore 控制的读/写闭包；复用 ReadInstanceIDTx、不缓存/嵌套事务，失败清空输出，提交前不发布凭据。Android login 分阶段解码，不能在 ID 校验前先解完整 Password；AndroidSession 保持独立封闭 DTO，GET session 不返 Bearer。其他 AdminBearer 分支先 ID 后读 Bearer，混合/重复凭据失败关闭。

检查：真实 auth HTTP+SQLite 验证缺头400 INSTANCE_REQUIRED、畸形/空/多值400 INVALID_REQUEST、错ID409 INSTANCE_MISMATCH、身份/DB故障503 INSTANCE_UNAVAILABLE，代次失效401；错误no-store且不泄实际ID。Create、读取/续期、logout、WithAuthorizedWrite、设备列表/撤销同快照/提交复核，revoked自撤、Session.id与提交后通知保持；登录校验后切库和事务中变化无部分结果，Web/CSRF不回归。运行 auth/storage 定向及 race/静态检查；交付真实内部接口后才允许 HTTP 包接入。

<a id="g5-be01-http"></a>

### G5 BE-01-HTTP 实例探测与真实路由同快照组合

- 历史调度（2026-10-26，UTC+8）：owner Backend Orchestrator，基线 `388dd8ae1f3641bdfd26a7a421e4171b8da16971` / 目标 `work`；AUTH 最终 `1aa0aa9e0c390a46a7293cae56410e0baae8ce71` 已 A02 PASS 与 I01 精确集成，BOT01／PLAN02 依赖已由 R01 验证。由“未开始”转 `ready` 后派发 Backend Implementer `/root/http_implement`，隔离 `/workspace/RelayNest-be01-http` / `codex/be01-http-20261026`；独占 router/main、httpapi/bots、bots/repository 及本树/index，auth/devices 只读；准确五主文件与必要相关测试，不改迁移／契约／其他模块。
- 历史 实施交接：`891287cf143e4d300c353e24233979077e84e64b`，基线 `388dd8ae…`，五主文件＋三测试（共八文件），隔离树 clean 并释放写权。Implementer 报最终全服普通、六包 race、全服 vet/build/gofmt/diff、锁版 Web ci/build PASS。自测曾发现 HTTP/2 无正文误拒绝，已修正、补回归并完整重跑；原失败保留。A01 准备固定此 SHA 只读独审，完整父 BE-01 不自动完成。
- A01 结果：Agent `/root/http_audit_a01` `CHANGES_REQUIRED`，`BE-01-HTTP-A01-001`：ServeMux 清理非规范 probe 路径在处理器前产生 307／Location／HTML，无 no-store/nosniff，query 原值会复制到响应。独立真实 SQLite＋生产路由＋TLS `/tmp` overlay 复现；全服普通、六包 race、vet/build/gofmt、锁版 Web 均 PASS。F01 只允许 router.go／instance.go 及 instance_test.go／router_test.go，提前固定拒绝会规范化为 probe 的别名路径，保持冻结 JSON／安全响应、不扩大其他路由。
- 历史 F01 派发准备：Backend Fixer，以 `891287cf143e4d300c353e24233979077e84e64b` 创建 `/workspace/RelayNest-be01-http-fix` / `codex/be01-http-f01-20261026`；独占上述精确路径及本树/index，仅修 A01-001，新提交后固定 SHA 重审。
- 历史 F01 Agent `/root/http_fix_f01` 已派发，原 A01 仍只读完成证据交接；父 BE-01 保持未完成。
- 历史 恢复 R03（2026-10-26）：Agent `/root/resume_checkpoint` 核主树 `work` / `064807f9e5e126f48c9360d4d02a40fc209f790e`，无在途测试／Git／文档写入进程。F01 树 HEAD 仍 `891287cf…`，已有 instance.go／router.go／router_test.go 三文件未提交修改（146 插入／2 删除），不可覆盖重做；六包 race 与串行全服日志含 PASS，但未留精确命令／退出码，尚无修复 SHA。重新派 Fixer `/root/http_fix_resume` 接管现有修改完成验证与本地提交，仍仅处理 A01-001；父项未完成。

- F01 收尾交接：新提交 `d8706747be5dbd98e91fe4892ab2f3f260e3b35c`（parent `891287cf143e4d300c353e24233979077e84e64b`），保留三文件 146+/2-，未扩围；树 clean，进程已结束并释放写锁。精确命令与退出码记录 `/tmp/be01-http-f01/resume/`：原审计 overlay、httpapi/cmd 普通、六包串行 race、全服普通／vet/build/gofmt/diff 与锁版 Web build 均 PASS；旧并发 cmd 1 秒进程超时 FAIL 保留，修正执行调度后串行 PASS，未改超时断言。A02 准备固定新 SHA 独审，未据 Fixer 报告直接放行。
- A02 结果：Agent `/root/http_audit_a02` `PASS`，固定 `d8706747be5dbd98e91fe4892ab2f3f260e3b35c`，关闭 A01-001，无新发现；原失败 overlay 及三协议生产 SQLite/router 540 次别名／query／方法响应矩阵 PASS，全服普通、六包串行 race、vet/build/gofmt/diff 与锁版 Web 副本构建 PASS。本轮无 FAIL，历史失败保留，受审树 clean、重型检查资源已释放。
- 历史 I01 派发准备：Backend Integrator，目标 `work` / `571dc5dca696d4dfb446a868ca58169cf488579b`，精确组合原实现 `891287cf143e4d300c353e24233979077e84e64b` 与修复 `d8706747be5dbd98e91fe4892ab2f3f260e3b35c`；在隔离集成树核双补丁与已集成附件／0008 组合后再移动目标。主调度 BACKEND 未提交元数据须保护不 stage，父 BE-01 独立收口仍未进行。
- 历史 I01 Agent `/root/http_integrate_i01` 已派发，目标业务 Git/index 与公共接线／重型检查独占至回交；主调度文档自本记录后冻结至 I01 释放。
- I01 结果：Agent `/root/http_integrate_i01` `PASS`，目标 `work` 无冲突快进 `571dc5dca696d4dfb446a868ca58169cf488579b` → `cf2c8b49b4b5338e58e4b9d6e9698e5e648bdc31`；映射 `891287cf143e4d300c353e24233979077e84e64b` → `cbbbb6e172d52391680279ebfbf163dfbc380d4c`、`d8706747be5dbd98e91fe4892ab2f3f260e3b35c` → `cf2c8b49b4b5338e58e4b9d6e9698e5e648bdc31`，range-diff 两项 `=`。锁版 Node 22.23.2／npm 10.9.8 Web／全服普通／六包串行 race／vet/build/gofmt/diff／原 A01+A02 三协议 540 矩阵及目标 build／关键路由均 PASS；BACKEND patch 逐字节保留，index 空，0001～0008 checksum 不变。业务与重型锁已释放，父 G1 与 Android 完整 call 仍 NOT_RUN；HTTP 叶验收、独审与精确集成证据齐全，父 BE-01 仍待独立组合收口。

- 旧ID：`P08-04-D01-B01-HTTP01`；类型：有限 leaf；状态：`done`（2026-10-26，主调度依据 A02 PASS 与 I01 精确组合从 integrating 归档）。
- 依赖：[已完成 BE-01-AUTH](COMPLETED.md#g5-be01-auth)、[已完成基线 P08-04-D01-B01-BOT01](COMPLETED.md#g4)、[已完成基线 P08-04-D01-B01-PLAN02](COMPLETED.md#g4)。
- 门槛：`G-GIT`、`G-API`、`G-INSTANCE-CONTRACT`；条件未实证不记通过。

允许 router.go、instance.go、httpapi/bots.go、bots/repository.go、cmd/relaynest/main.go 五个主文件及测试；auth/devices只读消费最终接口，main只接线。Bot ListPage 与 RenewRead 不得分离快照；仓储读取放入 AUTH 授权读闭包，Bot写继续同事务授权写，devices只验证共享安全入口。

公开 GET /api/v1/instance 无 query/body，拒绝 Authorization/Cookie/X-CSRF-Token/X-RelayNest-Instance-ID 的存在（含空值/大小写），不解析记录值。非法优先于限流，限流优先于读库；全局常量内存20rps/burst40，429 Retry-After:1。每次严格读取持久化ID，200仅小写UUIDv4单字段，缺失/损坏/读取失败503。成功/错误no-store、nosniff，禁止ETag/Last-Modified/Location/304/redirect；固定Error/request_id/code/message/retryable。

检查：真实HTTP+SQLite验证探测格式/错误顺序/限流/缓存/503，逐项列八个现有auth/device/admin-bot路由的绑定证据；切A→B、事务ID/代次变化、读投影不拼接。包含Web/CSRF、设备全撤/自撤、Bot内部发现、health/static非回归。运行httpapi/auth/devices/bots/storage测试及race、vet/build/必要全包检查；Web embed按锁版构建。客户端完整call5秒/1024字节由Android任务验收，不冒充本卡结果。

- 归档与限制：BE-DOC-S03 `/root/storage_archive_resume` 仅转录主调度完成决定与既有证据；完整父 G1、G-INSTANCE-RUNTIME／G-INSTANCE-COMPOSITION 仍 pending，Android 完整 call 5 秒／1024 字节及未来附件／WS 等未实现路由 NOT_RUN，不以 HTTP 叶完成替代父组合或跨模块验收。

<a id="g5-be02-bot"></a>

### G5 BE-02-BOT Bot 正式创建 HTTP

- 原 ID：`P05-02-S02`；类型：有限 leaf；状态：`done`（2026-10-26，由主调度依据 A01 PASS 与 I01 精确集成证据从 integrating 归档）。仅完成当前有限 Bot 文本入口，父 BE-02 仍未完成；管理员入口未开始，仍依赖完整 BE-01。
- 依赖：[已完成基线 P05-02-S01](#g3)、[已完成基线 P05-02-PLAN01](#g3)、[已完成基线 P08-04-D01-B01-BOT01](#g4)。
- 门槛：`G-GIT`、`G-API`；条件未实证不记通过。

- 基线：`01c74a776b0b76fcc6616f60ae637a5a53f18384`；S01 最终 `e674801f3640508327e102bda37805b687c3108d`、PLAN01 集成 `50be58eaaba6c8918dbd7642d9ef06125f0dda07` 及报告 `137799bc9f3c94b279b94a0d18b01480bb4903bd`、BOT01 最终 `4698f39dbe7e78f86cba209a0a44a750c9fdd8db` 经 R01 验证均为基线祖先。
- 短历史与范围：从未开始经 ready／running／review／integrating 完成；Implementer `/root/bot_implement` 在隔离树 `/workspace/RelayNest-be02-bot`、分支 `codex/be02-bot-20261026` 提交 `907c2a4dbf3a8774bcd438fb22b2381def7394a5`。实际五文件为 `server/internal/httpapi/messages.go`、`messages_test.go`、`router.go`、`server/cmd/relaynest/main.go`、`main_test.go`；未改 auth／bots／SQL／API。
- 最终独审：A01 `/root/bot_audit_a01` 对固定 `907c2a4dbf3a8774bcd438fb22b2381def7394a5` PASS。真实 HTTP／SQLite 及临时 overlay 补证 410 墓碑、读取 body 时撤销／降权、精确大小边界；普通五包、串行五包 race、vet/build/gofmt、锁版 Web typecheck/build PASS。
- 精确集成：I01 `/root/bot_integrate_i01` 在目标 `work` 无冲突快进 `01c74a776b0b76fcc6616f60ae637a5a53f18384` → `907c2a4dbf3a8774bcd438fb22b2381def7394a5`，源与目标准确受审 SHA 相同。全服普通、五包串行 race、vet/build/gofmt、锁版 Node 22.23.2／npm 10.9.8 Web 构建及目标路由回归 PASS；主调度 BACKEND／PROGRESS patch 哈希前后相同，index 空，未 push。
- 保留风险 `A01-R01`：默认跨包并行 race 首轮因基线 50ms busy-timeout 夹具触发 SQLITE_BUSY（messages 包本次零差异）；独立单测 race 连续 30 次、基线 20 次与五包串行 race PASS，不隐去首轮 FAIL。404 在当前禁止附件／引用的有限阶段无可达资源路径，仍 NOT_RUN；Webhook 未接线、范围外 NOT_RUN，不宣称完整引用／Webhook 验收。

允许 httpapi/messages.go、router.go、cmd/relaynest/main.go 三主文件及测试；不改auth/bots/SQL/API/管理员/Webhook分支，共享接线串行。正式 POST /api/v1/bot/messages 只用真实 API Token 与 BOT01 固定身份；启用/send/提交前凭据代次仍需验证，BotID只来自身份、不接受body指定，实例头不切身份。调用S01同事务创建，commit后201 Message+no-store。

请求≤256KiB、UUIDv4幂等头；未知/结构400、语义422、超限413、未授权401、能力403、资源域404、异摘要409 IDEMPOTENCY_CONFLICT、墓碑410 RESOURCE_EXPIRED、存储503，遵循冻结Error/request_id/retryable及默认值摘要；日志不含凭据/正文。检查真实路由+SQLite丢首响应/重启同键仍同ID且不增序号/changes/job，并发同键/异内容、commit失败全回滚、API reset新Token同作用域旧Token拒绝、跨Bot不泄漏；当前授权/能力先于查重，实例头缺失/畸形/错误不影响Bot身份。必跑httpapi/messages/idempotency/bots/cmd测试、相关race、go vet ./...；Webhook未接线须记NOT_RUN。

<a id="g5-be02-admin"></a>

### G5 BE-02-ADMIN Android 管理员正式创建 HTTP（有限文本）

- 原 ID：`P05-02-S03`；类型：有限 leaf；状态：`done`（2026-10-27，总调度依据完整 WIP 链 A04 PASS、准确组合全部必检及 I01 精确集成，从 integrating 归档）。仅 ADMIN 叶完成，父 [BE-02 四项组合验收](BACKEND.md#be-02) 尚未闭环，不据子项完成自动记父 done。
- 完整原始实现／修复链：`a97cfcee014298c64e8468a7b9eb1d007b635e37 → 8ab66c847bf1a6e0c86592f90994679b6685c7e5 → c937602975c232c62a20808f3a376a86b2d84795 → 92404a1d3fee0b2516133867af585e68a94ce772`。最初 `8ab66c8` 仅环境迁移 WIP，不作为验收；后续审计覆盖完整五文件及准确组合，未只审／集成尾提交。
- 准确组合基线：`f7e9daf415edfa4f051928820098765b04a661c9`，已包含推送 STORAGE／0009 及独立 [STORAGE-BUSY-01](#g5-storage-busy-01) 修复 `8a377840…／8d32f818…`；候选 `.worktrees/backend-admin-i01` / `integrate/backend-admin-i01-20261027@665edd785d8784ae0e3304d3d3bdfed2d474f872`。range-diff 三项均 `=`、无冲突；ADMIN 五文件与原 `92404a1` 逐字节一致，其余 server 与基线一致。明确本组合含九迁移及已审写准入，不将它与原八迁移 source 称为全树相同。

| 原提交 | 准确组合映射 |
| --- | --- |
| `8ab66c847bf1a6e0c86592f90994679b6685c7e5` | `6357b706b3ea73786e1b0deb56a7406fd8926b87` |
| `c937602975c232c62a20808f3a376a86b2d84795` | `76161d62aff9107ec0ec4fd02fd95cb2853d19c3` |
| `92404a1d3fee0b2516133867af585e68a94ce772` | `665edd785d8784ae0e3304d3d3bdfed2d474f872` |

- 额度中断证据：旧 `/tmp/relaynest-admin-i01-20261027/commands.jsonl` 已有同一 `665edd7` 六组真实 ADMIN HTTP＋SQLite PASS（exit 0／10.522 秒）、原 messages 并发／排队 race 五轮 PASS（exit 0／21.374 秒），准确复用而不冒称新跑。旧 full-test 仅部分包输出且无完整结束／退出码，记 **NOT_RUN（未形成完整结果）**；full-race／vet／build 无完整记录同记 NOT_RUN。额度中断不是开发 FAIL，不覆盖或删除该部分日志。
- 恢复 Integrator `/root/admin_i01_resume` 在同一固定 `665edd7` 新开 `/tmp/relaynest-admin-i01-resume-20261027-062641/`，实际补齐 **PASS**：`go test -p 1 -count=1 ./...`（exit 0／123.983 秒）；`go test -race -p 1 -count=1 ./...`（exit 0／509.553 秒）；全服 `go vet ./...`（0.732 秒）、`go build ./...`（1.337 秒）；gofmt／diff／来源保护。锁版 Go 1.27.3、Node 22.23.2、npm 10.9.8 实测 PASS；同版 Web tree 与三文件真实 embed SHA256 核验 PASS，复用实际产物而不重复声称新构建。每命令起止、准确 SHA／环境／退出码见新目录 `commands.jsonl`，汇总 `result.json`，差异与保护证明 `source-checks.json`、`inputs.json`；原日志未覆盖。
- 最终独审 **ADMIN-A04 PASS**：不同 Auditor `/root/admin_a04_resume` 固定 `665edd7`，核完整原 WIP／修复链、三项精确映射及已审 gate 基础；独立六组真实 HTTP＋SQLite PASS（exit 0／8.335 秒），并独立核完整全服普通／race／vet／build 退出 0。`ADMIN-A03-B01` 因准确组合全服 race 通过而关闭；旧 `ADMIN-A01-B01` 监听阻塞、`ADMIN-A02-01` 过期观察者夹具修正的失败与复验事实保留。独立证据 `/tmp/relaynest-admin-a04-resume-20261027-063856/{inputs.json,commands.jsonl,admin-http-independent.log,post.json}`。
- I01 精确集成 **PASS**：获总调度正式授权后，Integrator `/root/admin_i01_resume` 再核 root 仅未跟踪 `.worktrees/`、候选 clean／同 SHA／同实际构建输入，将权威 `resume/backend-20261026` 从 `f7e9daf…` 安全 ff 到 `665edd785d8784ae0e3304d3d3bdfed2d474f872`。ff 后 root tree 与受审候选精确一致，原迁移／0009／gate 保留，root `go build ./...`、diff 检查 PASS，main ref 仍 `01c74a776b0b76fcc6616f60ae637a5a53f18384`；未重复 cherry-pick、未改业务逻辑。收尾证据 `/tmp/relaynest-admin-i01-final-20261027-064139/{pre.json,commands.jsonl,post.json,target-fast-forward.log,target-build.log}`。
- 范围与限制：仅 Android AdminBearer 的 plain／markdown 无附件、引用、按钮消息创建；固定 administrator sender、真实 B01 同事务实例／会话／代次复核、当前 Bot receive 权限先于重放、提交后 201／no-store 原回执。回执仅代表持久化，不等于应用 ACK／已读／业务成功。父 BE-02 的 Bot＋Admin 四项组合 R01／独审／精确组合仍另行收口；未来 reset HTTP、完整 CreateMessageRequest／Webhook、附件及 Android／跨端实例门槛不据此完成。Bot Hub 不执行业务命令或 MCP。未 push、部署或创建 `.codex-task-done`。

#### 历史实施、失败与原验收（保留当时结论）

以下 OPEN／NOT_RUN／未集成字样均为当时状态，最终关闭与归档以上方 A04／I01 为准；保留原验收和所有失败，不将历史失败改写为从未发生。

- 调度（2026-10-26）：owner Backend Orchestrator，固定基线 `a97cfcee014298c64e8468a7b9eb1d007b635e37` / 目标 `work`；BE-02-BOT `907c2a4dbf3a8774bcd438fb22b2381def7394a5` 与 PLAN01 精确集成 `50be58eaaba6c8918dbd7642d9ef06125f0dda07` 已验证祖先，完整 BE-01 既定现有后端父范围经 R01／A01／I01 归档，完成快照 `a97cfcee…`，业务组合 `cf2c8b49…`。
- 派发准备：Backend Implementer，隔离 `/workspace/RelayNest-be02-admin` / `codex/be02-admin-20261026`；仅 httpapi/messages.go／router.go／cmd main.go／必要 messages/create.go 四主文件与相关指定测试，独占本树/index、messages HTTP/router/main 接线，不改 auth／SQL／API／Bot 分支。可与只改新 devices／0009 的推送集成并行；重型检查须等待推送 I01 释放。
- Agent `/root/admin_message_implement` 已派发，开始隔离准备及实施，尚无新增独审／集成结论。
- 环境迁移备份：用户已授权保存代码，现有五文件已独立 WIP 提交 `8ab66c847bf1a6e0c86592f90994679b6685c7e5`，远端入口 `recovery/backend-20261026-admin-wip`；不是功能完成提交，无新测试／审计结果。原 worker 不再运行，从现有代码继续，见本文件开头交接。

- Debian 恢复实施：Implementer `/root/admin_implementer` 从 `8ab66c847bf1a6e0c86592f90994679b6685c7e5` 接续，在上述 ADMIN 隔离树提交 checkpoint `c937602975c232c62a20808f3a376a86b2d84795`，树 clean 并释放写权；未重写生产 WIP，仅修正 TLS 丢响应夹具、204 空响应断言并补八项授权失效回归。完整待验收链为 `a97cfcee014298c64e8468a7b9eb1d007b635e37 → 8ab66c8… → c937602…`，五文件整体纳入审计，不能只审尾提交。
- 权限解除前检查：`go test -p 1 -run '^$' ./...`、全服 vet/build、gofmt/diff、messages/idempotency 普通 PASS；真实 cmd ADMIN 定向测试 FAIL，`go test -p 1 -race -count=1 ./...` 退出 1（240.8 秒），cmd/auth/httpapi 因 socket 权限失败，attachments/bots/changes/devices/health/idempotency/messages/storage/webui race PASS，未见 DATA RACE 报告但不等于全服 race PASS。新增 ADMIN 业务断言及完整真实 HTTP＋SQLite验收 NOT_RUN；尚无正式实施验收交接／精确集成。实际命令与退出码见 `/tmp/relaynest-admin-implementation/final-commands.txt`、`cmd-http.log`、`race-all.log`，这些为本轮临时证据，不冒称随 Git 保存。
- ADMIN-A01：独立 Auditor `/root/admin_auditor` 固定 `c937602975c232c62a20808f3a376a86b2d84795`，审查上述完整五文件链，结论 **BLOCKED**。稳定阻塞 **ADMIN-A01-B01**：`server/cmd/relaynest/main_test.go:354、1757` 的真实 TLS 服务监听报 `operation not permitted`，业务断言未运行；静态未发现确定业务缺陷不等于 PASS。复验要求：同一固定 SHA 在可监听环境运行全部 ADMIN HTTP＋SQLite、相关普通／race及全服必检，再续独审、必要编号修复／重审和精确集成。
- 权限解除后 A02／有限修复（2026-10-27，UTC+8）：新 Implementer `/root/admin_retry` 从 `c937602975c232c62a20808f3a376a86b2d84795` 接续；独立 Auditor `/root/admin_a02_retry` 对该 SHA 给出 **CHANGES_REQUIRED**，稳定问题 **ADMIN-A02-01** 为测试夹具使用过期实例观察。仅 `server/cmd/relaynest/main_test.go` 修复后提交 `92404a1d3fee0b2516133867af585e68a94ce772`（parent `c937602…`），未改生产代码／安全策略／冻结接口。完整候选链为 `a97cfcee014298c64e8468a7b9eb1d007b635e37 → 8ab66c847bf1a6e0c86592f90994679b6685c7e5 → c937602975c232c62a20808f3a376a86b2d84795 → 92404a1d3fee0b2516133867af585e68a94ce772`，不得只审或集成尾提交。
- 本轮实施验证：真实六组 ADMIN HTTP＋SQLite、全服普通、vet/build、gofmt/diff **PASS**；全服串行 race **FAIL**（exit 1，454.120 秒），唯一失败为 `messages/TestIdempotentConcurrentSameKey` 在 `server/internal/messages/create_test.go:130` 报 `SQLITE_BUSY`（SQLite 3.53.4），其他全部包 PASS。无 DATA RACE 报告不等于全服 race PASS，不改既有断言／50ms 超时以制造通过。精确命令／退出码与实际新日志为 `/tmp/relaynest-admin-retry-20261027/commands.txt`、`full-race.log`；旧 socket／夹具失败仍保留。
- A03 续独审：`/root/admin_a02_retry` 固定 `92404a1…` 审查上述完整链，静态无新增业务发现，独立六组真实 ClientMessage HTTP＋SQLite **PASS**，关闭 **ADMIN-A02-01** 与旧环境 **ADMIN-A01-B01**；但必检阻塞 **ADMIN-A03-B01** 仍 **OPEN**，因上述同键并发测试 SQLITE_BUSY，A03 最终结论为 **BLOCKED**，不能进入精确集成或标 done。独立证据为 `/tmp/relaynest-admin-a03-20261027/evidence.txt`、`client-message-independent.log`。
- B01 原断言对照：Researcher `/root/permission_verifier` 在 Go 1.27.3／SQLite 3.53.4 下，保留原 50ms BusyTimeout、12 goroutine／双 Store／全部断言，候选 `92404a1…` 五轮 race **4 FAIL／1 PASS**、基线 `15831444e0b616b0b9dadf0cad470741a0ac3eea` **3 FAIL／2 PASS**；基线 stage-only 诊断 overlay 五轮 **4 FAIL／1 PASS**，四次均明确为 **BEGIN IMMEDIATE → SQLITE_BUSY**，不是业务回调或 COMMIT 失败。三组命令均 exit 1，不能将偶发 PASS 算稳定通过；候选／基线相关输入 hash 一致，仅诊断返回错误补阶段名，无 SQL／重试／超时／断言改动。
- 原同数据库双 Store 写锁根因已由独立 [STORAGE-BUSY-01 修复叶](#g5-storage-busy-01) 完成：完整 `c806fd9… → e0c3631…` 经 A02 PASS、I01 精确合入原根 `8d32f818536e7a237d366afe0be321c69a7700e7`。保留上方原 50ms／12 并发基线失败与 BEGIN 定位事实；本修复完成不自动关闭 **ADMIN-A03-B01**，准确 ADMIN＋fix 全服 race 仍 **NOT_RUN**，需续完整组合验收／独审后决定 ADMIN 状态。
- 对照实际命令与证据为 `/tmp/relaynest-admin-b01-research-20261027/commands.txt`、`candidate-inputs.json`、`baseline-inputs.json`、`overlay-inputs.json`、`candidate-count5.log`、`baseline-count5.log`、`baseline-diagnostic-count5.log`、`overlay.json`；均为本环境临时证据，不冒称随 Git 保存。尚无全部必检通过的正式 ADMIN 实施验收交接／精确集成，目标集成仓库尚未合入 ADMIN，完整候选 `92404a1…` 保存在既有独立树；BE-10-01-HTTP 隔离准备已完成、阶段 1 实施见该叶当前登记，router/main 锁继续保留给 ADMIN，STORAGE 已完成结论保持。
- A03 时的历史状态：旧ID `P05-02-S03`；类型：有限 leaf；状态：`running`（2026-10-27，总调度继续保留实施步骤；当前固定候选 `92404a1…` 的真实六组及静态检查已通过，但全服 race 必检 `ADMIN-A03-B01` 未关闭，未进入已通过审计的 integrating，不标 done）。
- 依赖：[已完成 BE-02-BOT](#g5-be02-bot)、[已完成基线 P05-02-PLAN01](#g3)、[已完成 BE-01 现有后端父范围](#g5-be01)。
- 门槛：`G-GIT`、`G-API`；条件未实证不记通过。

允许 httpapi/messages.go、router.go、cmd/relaynest/main.go、必要 messages/create.go 四主文件及测试；auth只消费 BE-01 已集成接口。正式 POST /api/v1/client/bots/{id}/messages 仅 Android AdminBearer，固定sender=administrator、Bot启用/receive。S01幂等/消息/投递/changes/待办必须处于 B01 WithAuthorizedWrite 的同一受控事务，先ID再读Bearer，提交前复核ID/会话/代次；不能以M01/BOT01/桩middleware代替完整绑定。

commit后201 ClientMessageReceipt含原message/delivery_id，no-store；同有效管理员换会话仍同作用域，重放前核当前授权。错误同Bot分支并保实例错误，接受G1瞬切字节窗口。检查真实HTTP+SQLite丢响应重试/重新登录同键不增消息/投递/不可变事件/序号/changes/job，异内容409；任一步/commit失败全回滚；缺/重复/畸形/错ID无副作用，事务中ID/会话/代次失效拒绝，probe A切B不认证/不执行；Web/Bot误用及管理/设备/health非回归。必跑httpapi/messages/idempotency/auth/cmd测试、相关race、go vet ./...、go test -race -count=1 ./...。

<a id="g5-be04-read-storage"></a>

### BE-04-01-READ-STORAGE 共享已读事务仓储与原子初始化

- 类型：有限子叶；状态：`done`（2026-10-28，总调度依据完整源检查、不同A01 PASS、准确新并集I01 PASS及root安全ff批准完成；running→review→integrating历史保留）。原Implementer `/root/be04_read_implementer`，恢复检查及提交 `/root/read_resume_checks`，准备角色 `/root/admin_i01_resume`。固定产品基线 `d4604184a2d3e2616146d63b8b902d052a723237`，实际私树 `.worktrees/backend-be04-read`／`impl/backend-be04-read-20261028` 从 `8b3c0c9e27bad2ea1522790972e8d4ac9f61cd91` 到单提交 `efdae35c250e6062d2a30d127dfe4c8d7fbdba8c`，准确十文件、工作树clean。依赖[BE-02限定父组合](COMPLETED.md#g5-be02-compose)、[HISTORY-STORAGE](COMPLETED.md#g5-be04-history-storage)及既有[B01／Bot事务与认证基础](COMPLETED.md#g1)均已集成；不依赖REFERENCE-CREATE或其他WIP。148个输入文件hash与既有检查一致；new-tests的sync／bots **PASS**（1.126／0.118秒）、whole-normal全服普通 **PASS**（140.823秒，覆盖相关八包普通）、related-race **PASS**（358.782秒）、全服vet／build **PASS**（2.623／1.562秒），五命令均exit0且日志hash匹配，gofmt／diff／迁移／真实embed检查PASS；没有本叶测试FAIL，首次Git提交因identity缺失exit128、使用命令级身份重试成功。证据 `/tmp/relaynest-be04-read-validation-20261028/{commands.jsonl,final-post.json,static-checks.json}`。独立A01 `/root/read_storage_auditor` 固定完整源链 **PASS**，无实施发现；独立new-tests exit0／5.742秒、148输入前后相同，证据 `/tmp/relaynest-be04-read-a01-20261028/{static-evidence.json,independent-tests.json,independent-tests.log}`，日志SHA256 `5c0a3001fde542626e0e80899260f0715d18283af3dbb4112af7b040cc5d0a99`。元数据BE04-READ-A01-01已由36ed24d纠正，不是实施缺陷。准确I01及目标快进已完成，详见下方。
- 单目标：仅共享已读可信事务仓储及初始化，最多四个主要文件：新增 `server/internal/sync/read_models.go`、`server/internal/sync/read_repository.go`、`server/migrations/0010_bot_read_states.sql`，仅修改 `server/internal/bots/repository.go` 的 CreateWithWrite 原子初始化；必要同包新 read 测试／Bot 创建测试、`server/internal/attachments/schema_test.go`和 devices push 迁移计数测试。SQL旧1～9／auth／storage／changes／messages／API／router／main／Web及其他任务树只读；不引入 HTTP／DTO／外部错误码或改契约。
- 迁移：`0010_bot_read_states.sql` 已正式永久占用，READ实施／集成写锁已释放；原SQL1～9逐字节／checksums不变，0009仍永久归push，后续DDL必须另核号。分配前扫描证据 `/tmp/relaynest-be04-read-plan-20261027-172206/migration-scan.json` 保留，不将早期库／refs计数作为当前计数。
- 冻结语义：消费受信调用方 `*sql.Tx`，拒 nil，不自开／提交事务，不造仅凭公开 Actor 字段的授权包装；同 tx验证 Bot及状态，缺行／损坏 fail closed，不 lazy repair。已读输入上界用 `bots.bot_seq`，不以现存消息 MAX替代；disabled Bot可读。max合并仅实际前进时更新 read_seq／version／非空UTC时间并同事务追加 `changes.ReadUpdated`；旧／相同水位不改时间版本、不写事件。版本溢出及腐坏值失败且不发布部分结果；未读只统计现存 application 且 bot_seq>read_seq，不计管理员／callback／编辑／过期。
- 初始化：旧 Bot迁移为 read_seq=0／version=1／非空UTC（复用现有 strftime UTC约定），新 Bot在 CreateWithWrite同事务初始化；初建仅原 BotUpsert，不额外 ReadUpdated。PRD4.2／Tech4与6.3／OAS ReadState已有version≥1和UTC必填约束，此为既有合同实现细节，不增加外部接口。原Bot凭据／BotUpsert／readrow成功和回滚原子，不发布未提交明文或状态。
- 验收：真实 SQLite 9→10迁移，原1～9 checksums不变；旧／新Bot初始化稳定、reopen不改时间版本、FK／删除级联；真实RequestSecurity多设备和B01授权、伪actor／撤销／实例代次否定；两个Store并发max、事件数恰等于实际前进次数，旧设备不倒退。最高消息删除后曾用序号仍可作有效上界；SQL／changes／COMMIT／重新授权／取消失败全回滚、无部分结果或凭据发布；错误／溢出／缺row不隐式修复。保留原50ms／12并发及10ms门槛，不改断言或超时掩盖问题。
- 正式检查门槛：锁版 Go1.27.3／Node22.23.2／npm10.9.8，真实 Web embed先核同版输入；实际相关 sync／bots／messages／auth／storage／changes／attachments／devices 普通与受影响 race、全服普通／vet／build、gofmt／diff及迁移检查，重型串行。完整本地提交后不同Agent固定SHA审计，再准确并集I01核收；所有命令须记录PASS／FAIL／NOT_RUN及退出码，旧计数失败轮未运行；首次正式检查前仅将迁移计数9→10，首次正式new-tests／whole-normal直接PASS，保留原1～9 checksums及原断言。本次仅更正误述元数据，不是测试复验，不删除其他任务真实FAIL证据。
- **BE-04-01-READ-STORAGE-I01 PASS**：Integrator `/root/backend_restore_retry` 从准确目标 `resume/backend-20261026@8fddeaf3a015dd46237d653f8fa4a6c8df7e5ab1` 在 `.worktrees/backend-be04-read-i01`／`integrate/backend-be04-read-i01-20261028` 仅一次无冲突重放完整源 `efdae35c250e6062d2a30d127dfe4c8d7fbdba8c → 0a93d6ea7508ae7a62eb0007e32bc0237b39ba5d`，range-diff单项 `=`；纯元数据 `57f63628cbd1cfe1b33cd652418b084d2c3f1640 → 43e9d634d5fd25ebd9012b3369b140bb409d39d3` 后冻结最终候选43e。早候选57f的所有Go检查确为NOT_RUN，原inputs.json／plan.json保留；仅最终43e实际执行：真实9→10迁移＋新Bot原子初始化 **PASS**（4.823秒）、全服普通 **PASS**（156.003秒）、十包race **PASS**（648.445秒，sync／bots／messages／auth／storage／changes／devices／attachments／cmd／httpapi）、全服vet／build **PASS**（2.651／1.561秒）、gofmt／diff **PASS**，全部exit0。cmd/relaynest race实际267.830秒在原300秒超时内完成，没有改超时或旧断言。十路径逐字节同受审源，其余target业务／REF／BE10HTTP／旧SQL／Web／依赖完整保留，无上传／F01／PENDING源码。147产品输入＋三真实embed在检查前后及root ff后hash完全一致，安全ff root `8fddeaf… → 43e9d63…`，main `01c74a7…` 不变。证据 `/tmp/relaynest-be04-read-i01-20261028/{inputs-refrozen.json,plan-refrozen.json,range-diff.txt,fixed-commands.jsonl,fixed-post.json,fixed-target-fast-forward.log,fixed-*.log}`；Web／API未改，真实embed核hash复用，本轮npm／validator **NOT_RUN**，不冒新实测。
- 父与后包：仅存储／初始化完成，父BE-04-01仍running；历史／共享已读HTTP、同步投影、推送已读策略过滤、附件投影、父最终组合及全局／跨端门槛均未由此完成。上传／F01／PENDING源码未纳入本叶受测43e，分别按其I01／完整验收推进；后续可消费本叶已集成仓储，但不推定HTTP／提醒过滤已完成。未push／部署／发布／创建完成标记。

<a id="g5-be04-reference-create"></a>

### BE-04-01-REFERENCE-CREATE 既有双入口文本创建接通同 Bot 回复引用

- 类型：有限子叶；状态：`done`（2026-10-28，总调度依据完整实施、不同 A01 PASS 与准确 I01 新并集及真实目标集成批准完成；原 running → review → integrating 历史保留）。Implementer `/root/be04_reference_implementer`，固定实施基线 `dbdb1c9fa13a8053bd039d4910c18dd2cdffd060`，完整单提交 `65488a571fe241bee116309df96aaee351099fc6` 准确七路径，原 WIP 全包含，源 `.worktrees/backend-be04-reference` clean冻结。依赖[BE-02限定父组合](COMPLETED.md#g5-be02-compose)及[HISTORY-STORAGE](COMPLETED.md#g5-be04-history-storage)均已完成。Integrator `/root/restore_backend_1028` 从准确目标 `resume/backend-20261026@bd95d01462d967b822e6d25bcca365cad387aaa3` 在 `.worktrees/backend-be04-reference-i01`／`integrate/backend-be04-reference-i01-20261028` 完整重放。
- 源证据：`/tmp/relaynest-be04-reference-validation-20261028` 固定输入全服普通143.565秒／全服race581.710秒／vet0.749秒／build1.354秒完整exit0。独立A01 `/root/admin_i01_resume` 固定完整源链 **PASS**、无发现；真实cmd七组/messages四组独立检查18.362秒exit0，证据 `/tmp/relaynest-be04-reference-a01-20261027-173519/{inputs.json,commands.jsonl,independent.log,post.json}`。A01确认真实HTTP同DDL503／全回滚与仓储callback成功后真实延迟FK COMMIT失败配对足够；同一次HTTP事务内部直接COMMIT观测仍 **NOT_RUN**，不得冒称已直接观测。上述源检查不冒充I01新并集实测。
- 单目标：既有 Bot API／Android 文本创建支持同 Bot 回复引用，无新路由。首次请求在当前授权、基本校验／原摘要、幂等查询之后验证引用；命中旧 key 原样重放，源删除后旧成功 key 仍返回原 receipt／delivery 且无新增。首次引用在同事务序号分配前校验，双引用列同时写入，message.created 保留 original 引用 ID；两 HTTP 回执复用原 originalID。跨 Bot／合法不存在引用统一 404 RESOURCE_NOT_FOUND，管理员错误白名单须覆盖；新增 UUID 结构／格式按既有400／422规则，不创设新合同。
- 允许范围：四主文件 `server/internal/messages/{create.go,repository.go,pending.go}` 和 `server/internal/httpapi/messages.go`；必要同包测试、`server/internal/httpapi/messages_test.go`、仅新 `server/cmd/relaynest/message_references_test.go`。SQL／API／依赖／已集成 history及reference／auth／storage／changes／router／main生产与旧 main_test 全部只读；公共 router／main 写锁由总调度另行排他协调，本叶不得触及其生产文件或其他任务树。
- 验收：原幂等语义、真实 B01 授权、Bot API＋Android 双入口真实 HTTP／SQLite及原子回滚；同 Bot 引用成功、跨 Bot／不存在一致拒绝、源删后幂等重试无新增，回执／delivery／message.created 的 original 引用一致；失败不得写序号／消息／投递／changes／幂等或泄露正文。仅 Bot API／Android 新引用，Webhook identity 新引用仍不支持，不扩大附件／按钮或重写 BE-02 历史完成范围。
- 限制与后包：仅既有 Bot API／Android 文本引用完成，父 BE-04-01 继续 running，READ-STORAGE／历史 HTTP／最终父组合另包；共享已读存储已另行分包并实施、尚未完成。引用叶无需迁移、不占0010，0009永久保留给 push。原准备未运行 Go／npm／validator；实施检查按总调度的全局重型窗口排队，不与其他 Actor 并跑。

- `BE-04-01-REFERENCE-CREATE-I01` **PASS**：完整映射 `65488a571fe241bee116309df96aaee351099fc6 → b5fa149f3b1505508be4703c3e6be45c923e22e4`，range-diff 一项 `=`，仅该七路径与受审源相同，目标已有推送 HTTP 九路径、HISTORY、Web／依赖／九 SQL／真实 embed及最新元数据保留。另有纯元数据提交 `74e07f65a8b70bd91d129920573a779783c1a746`，即最终准确受测候选。此 HTTP＋引用新并集实际全服普通 **PASS**（exit 0／153.066秒）、相关七包 race **PASS**（554.827秒）、全服 vet／build（0.813／1.398秒）、gofmt／diff 全 PASS；没有把旧 source 的全服结果冒充新组合。证据 `/tmp/relaynest-be04-reference-i01-20261028/{inputs.json,commands.jsonl,complete.json,post.json,target-fast-forward.log,*log}`；已安全 ff root `bd95d01… → 74e07f6…`，受测输入／文件 hash一致，main `01c74a7…` 不变。
- 验收覆盖双入口实际 TLS／HTTP＋SQLite、12并发同异key、先授权再重放、B01提交前会话／实例失效、删源后的旧回执／delivery不变、跨Bot及不存在404／UUID400422、重开／丢响应／故障全回滚；Webhook引用继续关闭，附件／按钮不开放。HTTP jobs=0，不据此宣称非零正式推送；仓储故障非零待办检查另有实际日志。源码和测试未改旧 main_test 或超时／并发断言；无生产测试 FAIL，首次本地 commit 因 Git identity未配置未创建对象，仅命令级指定身份后成功，不改共享配置。新 npm／API机验 **NOT_RUN**，Web／API未改，真实 embed按hash复用。
- 限制：上述 COMMIT配对经 A01判断充分，同一次 HTTP内部直接 COMMIT观测仍 **NOT_RUN**。父 BE-04-01 原历史分页／共享已读／最终组合，以及 READ-STORAGE、历史／已读HTTP和附件投影后包仍按各自门槛执行；本叶无迁移、未占0010，未 push／部署／创建完成标记。

<a id="g5-be04-history-storage"></a>

### BE-04-01-HISTORY-STORAGE 历史／详情与同 Bot 引用只读仓储准备

- 类型：有限子叶；状态：`done`（2026-10-27，Implementer `/root/be04_history_implementer` 完整源链经不同 A01 PASS 与 I01 精确集成；父 BE-04-01 原验收整体仍未完成）。准备角色 `/root/admin_i01_resume`；实际固定实施基线 `a6f4e7da58d198c6fe0744878406e0522aa56e27`；隔离树 `.worktrees/backend-be04-history`／分支 `impl/backend-be04-history-20261027`，本树／index 和本叶 messages 新文件写锁归 Implementer。只读仓储源链 `a6f4e7d… → c15b66e47f3c789877f8842ce74768a5fdf77a1e → 504d74ba91a0a9152a90b417efb6f826a2eac04c → a88ac3a743954385b413b773230a608db3d088d2`，仅六个新增 messages 文件（三主＋三测试），无迁移。不同 A01 与 I01 均 **PASS**，见下方准确证据。
- 实际检查与历史：准确 `a88ac3a…` messages 普通 **PASS**（exit 0，包耗时3.504秒），证据 `/tmp/relaynest-be04-history-short-20261027-202815/{inputs.json,result.json,messages-test.log,post.json}`；`504d74b…` 新授权夹具曾因 Unix 秒／纳秒、旧 Store 实例保护和取消错误分类假设 **FAIL**，仅新 `history_auth_test.go` 修正后复验 PASS，保留旧日志。auth／storage／changes 普通也已完整 exit 0 **PASS**，证据 `/tmp/relaynest-be04-history-validation-20261027-203716/commands.jsonl`；四包 race、相关 vet、全服 build、gofmt／diff／clean 后均完整 exit 0 **PASS**，准确 `a88ac3a…` 日志与输入／post hash 均已核验；不外推为全服普通或新 HTTP 通过。
- 单目标与允许写范围：仅 `server/internal/messages/{history.go,history_models.go,reference.go}` 及对应 `history_test.go`／`reference_test.go`／`history_auth_test.go`，优先复用模型减少文件；实现历史／详情和同 Bot 引用域的只读事务仓储准备。全部消费受信调用方 `*sql.Tx`，拒绝 nil，不自开／提交事务，不造仅凭 Actor 公开字段的授权包装。
- 原验收中本叶覆盖：`bot_seq DESC`、排他内部边界、有界 limit＋1；同 tx 核 Bot 存在及读取，停用 Bot 历史可读；同 Bot 存在引用接受，跨 Bot／不存在统一不可用；源删不泄正文且保留 originalID。真实 SQLite 覆盖多 Bot 分页、并发新增删改不改变排序、边界／limit、错误无部分结果、引用 FK、真实 RequestSecurity＋B01 授权及伪 Actor／会话／实例代次、取消及最终失败不发布。无 route 不声称 Web／Bot HTTP 完成。
- 排他边界：`create.go`／`repository.go`／`pending.go` 显式拒引用保持；auth／changes／storage／SQL／API／router／main 与推送 HTTP WIP 全部只读。public DTO、cursor codec、HTTP、创建引用放开及共享 read 表另包；后包初始 readstate version≥1／time 非空规则待具体设计，本叶不实现、不设新外部错误码。
- 迁移：Planner 已只读核六注册树和三独立 Git 仓库 refs，最高 0009，无 0010＋或未提交迁移；原 0006 已具双引用列、同 Bot 触发器／FK SET NULL，本叶无需迁移、不占 0010。0009 永久由 push 占用；未来 READ-STORAGE 派发时重新核号分配，不以本次扫描预占。

- 独立 `BE-04-01-HISTORY-STORAGE-A01`：固定完整源链至 `a88ac3a743954385b413b773230a608db3d088d2` **PASS**，无待修发现；独立八组测试 exit 0／0.870 秒，前后源码／输入不变。证据 `/tmp/relaynest-be04-history-a01-20261027-2047/` 的 pre／post／command／exit／log hash；stdout SHA256 `fc88544345412ba8374effae5d030031320400442b7780f7af3da2cdcc731b9f`。
- `BE-04-01-HISTORY-STORAGE-I01` **PASS**：准确目标 `resume/backend-20261026` 从 `a9429ef5cb7c47f72db3a55139b7e26e76da6e9f` 安全 ff 至 `6e5ca4b7b7ca8b11d8a550bcf35627499132c1e3`，候选树 `.worktrees/backend-be04-history-i01`／`integrate/backend-be04-history-i01-20261027`。完整映射 `c15b66e47f3c789877f8842ce74768a5fdf77a1e → f37ffeee487bd26ba878aa317c703aef5d981d8a`、`504d74ba91a0a9152a90b417efb6f826a2eac04c → ff116b7a969f5cdf7c25dde6a8ca8e13ca80b301`、`a88ac3a743954385b413b773230a608db3d088d2 → 6e5ca4b7b7ca8b11d8a550bcf35627499132c1e3`，range-diff 三项 `=`，无冲突／无业务变化；候选 server 与源 a88 tree 完全一致，Web／go.mod／go.sum／九 SQL／真实 embed 与受测输入字节相同，目标文档保留最新元数据。准确复用源必检及 A01，**本 I01 未新跑 Go／npm／validator／build**，不冒新实测；记录 `/tmp/relaynest-be04-history-i01-20261027-124833/{checks.json,post.json}`。main ref 不变，旧 50ms／12 并发断言及其他旧源码未动，无 HTTP 推送产品代码混入。
- 完成仅为调用方持有事务的历史／详情／引用域存储准备；仍不开放创建引用，无共享已读／附件投影／公开 cursor／HTTP。REFERENCE-CREATE、READ-STORAGE、HTTP 和父原验收另包；初始共享已读策略仍待具体设计。source 504 新夹具失败与 a88 复验事实保留，不记生产失败；未占 0010，未 push／部署／创建完成标记。

<a id="g5-be02-compose"></a>

### G5 BE-02 正式幂等创建与回执有限父组合

- 原 ID：`P05-02`；类型：有限父收口；状态：`done`（2026-10-27，总调度依据 R01 实际双入口组合、不同 A01 PASS 与 I01 等值核收 PASS，从 review 经 integrating 完成归档）。原 Bot／ADMIN 两叶早于本父完成，其历史记录中的“父未闭环”保留为当时状态，不靠子项 done 自动完成父项。
- 四项原验收均由真实生产 Bot＋管理员 HTTP／SQLite 独立组合收口：**提交后才回执；丢失响应重试不重复；同键异内容 409；Token 重置不更换稳定身份作用域**。本次没有新增产品源码／迁移或拼入未审推送 HTTP `2c7038a…`。
- R01 Researcher `/root/be02_compose_prep` 的固定源码为 `15aa3cd7e64db235102c051f78ec7e4801ddb764`，证据快照 `f915c86504a1036e145c57757810116c95c6a8b6`；六条普通／相关 race 均完整 exit 0，生产装配、计数与范围详见下方保留的原 R01 快照。只在 `/tmp` 追加测试，原 77924 字节测试前缀保留，overlay SHA256 `e5a00a42acdd0876e023bf2c3f428cd4576a87467aa07166c62c8fd2ce688935`；原 v1 私有 DTO 编译失败／v2 修测试 struct 后通过与额度阶段均保留，不将其冒称生产失败或重跑过的 whole 矩阵。
- 独立 **BE-02-COMPOSE-A01 PASS**：Auditor `/root/be02_compose_audit` 固定上述 `f915c865…` 证据快照，核 source15aa／overlay e5a／R01 六条命令，独立只复跑 `TestBE02ComposeProduction／TestBE02ComposeFaults`，exit 0／5.739 秒。前后源码仍 15aa、树 clean／所有输入与 overlay 同一；独立日志 SHA256 `bed72dc83481be11c4cd301bc697777c42d1d99f20b92501f20232472e19cf52`。证据 `/tmp/relaynest-be02-compose-a01-20261027-113300/{inputs.json,independent.log,post.json}`。
- **COMMIT 方法与限制**：A01 结合真实源码提交／回执顺序、延迟 FK companion 事务内部到达 COMMIT 的直接证据，以及同 DDL 另一生产 HTTP 503／完整回滚的配对证据，判断足以覆盖原“提交后才回执”验收，无需新增生产 hook。**未直接观测同一次 HTTP 事务内部已到达 COMMIT，该直接内部观测仍为 NOT_RUN；companion 不等于 HTTP 内部观测，本次不把方法判断改写为直接实测。**
- **BE-02-COMPOSE-I01 PASS**：Integrator `/root/admin_i01_resume` 独占 root 文档／index，仅核收既有精确组合。归档前 root `resume/backend-20261026@f915c865…` 与受测 source15aa／完整 whole 已测 `665edd785d8784ae0e3304d3d3bdfed2d474f872` 的 server tree `032e99f18fe47e10c8c7418138b567a45413d1e8`、Web tree `a93c7865a737912c8833d05df5ebf407b296c378` 相同；go.mod／sum、九迁移和真实 embed 逐文件 SHA256 一致，全部既定依赖与 Bot／ADMIN 子叶已为目标祖先。复核 R01 六完整 exit 0、原测试前缀与日志 hash、A01 映射／退出码正确；没有新产品提交、冲突或需重放的 feature，未重新运行 Go／npm／whole／build，而准确复用同业务源码既有 665 全服普通／race／vet／build 及 I01 root build 证据。实际核收 `/tmp/relaynest-be02-compose-i01-20261027-113727/checks.json`。
- 完成范围仍仅 plain／markdown、无附件／引用／按钮，回执仅持久化而非 ACK／已读／业务成功。Bot／Admin 计数来自各自独立 fixture，不是同库顺序累计；HTTP `jobs=0`，不声称生产 HTTP 非零正式推送待办已验收。正式 reset HTTP、Webhook、附件／引用／按钮、后续 BE-04 等未实现能力及 Android／跨端实例门槛均不自动完成；未分配新迁移，0009 继续原占用。Bot Hub 不执行业务命令或 MCP。
- 本次只将原父证据移入本页并修相应完成依赖链接；其他完成叶不重做，后继任务仍待各自规划／派发。未 push、部署或创建 `.codex-task-done`，本父 done 不代表全后端完成。

#### 原 R01 证据快照与原验收（f915 时，保留当时 review 状态）

以下“待 A01／I01”属于历史快照；最终闭环以上方 A01／I01 为准。观测限制、独立 fixture 计数和原验收原样保留，不将配对证据改写为同次 HTTP 内部直接观测。

- 子项证据：[Bot 有限文本 HTTP](#g5-be02-bot) 与 [ADMIN 有限文本 HTTP](#g5-be02-admin) 均已完成独审及精确集成；父项原四项真实组合验收仍未闭环。
- 父组合 R01 证据已交付，状态由 `running` 转为 `review`（2026-10-27，总调度依据 Backend Researcher `/root/be02_compose_prep` 的实际结果，仅提交证据快照待不同 A01；尚未独审／I01，不标 done）。固定源码／独立验证树为 `15aa3cd7e64db235102c051f78ec7e4801ddb764` / `.worktrees/backend-be02-compose-r01` / `verify/be02-compose-r01-20261027`，不消费未集成 HTTP WIP；本次快照仅元数据，源码与该固定对象及已测 `665edd7…` 的 server／Web 一致。
- R01 使用生产 `authenticationDependencies → httpapi.New → 真实 TLS／HTTP → SQLite` 双入口，补齐此前 Bot 单路由 mux 四项证据与生产装配仅 smoke 的父组合缺口，不重做或撤销已完成子叶。Researcher 报告四项外部可观察断言 **PASS**：201／no-store 回执与已持久化消息／幂等对应，管理员 delivery／不可变 event／payload 序号一致；首响应完全不读取，确认数据库已提交后断连，再做原键重试。Bot 重开生产装配、真实仓储 Rotate／旧 token 拒绝，管理员真实 logout／login，同键异内容 409 IDEMPOTENCY_CONFLICT、当前降权 403 均在动作后的基准快照上验证回放零增量；轮换／登录／降权自身的合法变更不混称回放无副作用。仓储 Rotate 不代表正式 reset HTTP 已实现。
- 计数向量为 `[sum(bot_seq), message, delivery, distinct event, changes, jobs, idempotency, changehigh]`：Bot、Admin **各自独立入口／fixture** 的初始基线均为 `[0,0,0,0,1,0,0,1]`；Bot 首提交为 `[1,1,0,0,2,0,1,2]`，Admin 首提交为 `[1,1,1,1,3,0,1,3]`。这些不是同库顺序累计；动作后重试前后完整状态／匿名事件 hash 相同。HTTP fixture `jobs=0`，不能宣称生产 HTTP 非零推送待办已验收；非零 job 原子性只准确引用本轮已实际通过的 `messages/TestIdempotentFaultsAreAtomic` 等真实仓储检查。
- 故障补证：11 类真实 HTTP 故障均得到 503／no-store、无成功回执、完整状态不变。**COMMIT 观测限制：companion 真实仓储事务中直接观测消息／幂等／延迟违规行各 1、业务 callback 返回 nil，随后真实延迟 FK COMMIT 失败；另一条生产 HTTP 在相同故障 DDL 下得到 503 与全回滚。未直接观测同一次 HTTP 事务内部已达 COMMIT，该直接内部观测仍为 NOT_RUN。不得把 companion 称为 HTTP 内部观测，配对证据是否充分由不同 A01 决定，不能据此自行豁免验收门槛。**
- 实际执行 Go 1.27.3 普通三条及相关 race 三条全部 exit 0：cmd 普通／race `13.837／46.252` 秒，httpapi `2.241／6.366` 秒，messages `1.589／10.286` 秒。固定证据 `/tmp/relaynest-be02-compose-r01/{evidence.json,final-inputs.json,commands.jsonl,main_test.go,overlay.json}`；最终追加测试副本 SHA256 `e5a00a42acdd0876e023bf2c3f428cd4576a87467aa07166c62c8fd2ce688935`，原 `main_test.go` 的 77924 字节前缀逐字节保留（原 SHA256 `272d02e3064dd92c1f0cb4606eb79bb5bb338a4d8fc4c3051318284b21325cf7`）。日志 hash 已独立核对，只摘要计数、状态及匿名 hash，原始正文／token／credential 不入库。
- 保留历史：R01 v1 仅 `/tmp` overlay 误用私有 DTO，编译 exit 1；改为本地 decode struct 的 v2 后上述六命令 PASS，原 `ordinary-cmd.log`、`main_test.v1.go.txt`、`final-inputs.v1.json` 均保留，不能算生产源码失败。额度中断期间未产生本快照或后续 A01／I01 结果，不把已完成六命令改记失败，也不冒称已审完。准确复用已测 `665edd7…` 全服普通／race／vet／build：相同 server／Web tree、go.mod／sum、九迁移及真实 embed 输入 hash 见 `final-inputs.json` 引用的准备证据，whole 矩阵不是本 R01 新跑。
- R01 未改变生产源码、SQL、原超时或断言；范围仍仅 plain／markdown、无附件／引用／按钮／Webhook／ACK。父四项原验收与依赖保留，正式 reset HTTP、HTTP 非零推送与跨端门槛均不据此自动通过。

- 旧ID：`P05-02`；类型：父收口，不整组派发；状态：`review`（R01 已交付，待不同 A01 审证据及 I01 准确组合；父收口未完成，不标 done）。
- 依赖：[已完成基线 P05-01](#g2)、[已完成基线 P05-02-PLAN01](#g3)、[已完成基线 P05-02-S01](#g3)、[已完成 BE-02-BOT](#g5-be02-bot)、[已完成 BE-02-ADMIN](#g5-be02-admin)。
- 允许范围：`server/internal/messages/**`、`server/internal/idempotency/**`、`server/migrations/**`，及对应测试。
- 门槛：`G-GIT`；条件未实证不记通过。

父卡保留四项原验收：提交后才回执；丢失响应重试不重复；同键异内容409；Token重置不更换稳定身份作用域。存储基础虽已完成，必须组合真实 Bot 与管理员 HTTP+SQLite 复验。

当前有限阶段仅 plain/markdown 文本且无附件/引用/按钮；其他输入显式拒绝并记录未实现，不能吞字段或声称完整 CreateMessageRequest 通过。后续引用、附件、按钮、标准Webhook的阶段门槛分别由 BE-04/05 保留。回执仅代表持久化，不代表应用ACK、已读或业务成功。

<a id="g5-be05-http"></a>

### BE-05-01-HTTP 真实上传接线与组合验收

- 类型：有限叶；状态：`done`（2026-10-28，总调度依据完整源检查、不同A01 PASS、准确新组合I01 PASS及root安全ff批准，保留review→integrating过程）。Implementer `/root/be05_upload_implementer`，完整单提交源链 `bd95d01462d967b822e6d25bcca365cad387aaa3 → 53427226cf5c3f40ac997d710efc7ba0f7893f3b`；准确五路径 `server/cmd/relaynest/main.go`、`server/internal/httpapi/router.go`、`server/internal/httpapi/attachments.go`及cmd／httpapi两份attachments_test.go。依赖已完成[STORAGE](COMPLETED.md#g5-be05-storage)及[BE-01现有后端父范围](COMPLETED.md#g5-be01)，本叶无新迁移，router／main实施与集成写锁已释放。
- 完成范围：冻结query＋原始二进制流上传／201／no-store及错误合同，真实HTTP＋SQLite＋files／生产main接线。真实Bot-first发现先于读流，Matched拒绝不回退、Unavailable失败关闭，仅NoMatch进入Android同快照门禁；提交前重验凭据、权限、会话及实例代次。Web Cookie／Webhook／receive-only无上传权，缺目标／无权限先拒绝再读流。覆盖幂等、实际可配置正int64限额、分段流及截断、流中撤销／模式／实例变化、SQL／COMMIT失败回滚和秘密边界；不能把默认限额当不可配置上限或宣称设置HTTP已完成。
- 源检查：targeted-03原12顶层Test **PASS**，全服普通 **PASS**（75.797秒）、六相关包race **PASS**（247.718秒）、全服vet／build **PASS**（0.807／1.458秒）、gofmt／diff／clean PASS。证据 `/tmp/relaynest-be05-http-20261028-2my4nzjp/{complete.json,matrix-commands.jsonl,targeted-03.log,ordinary-01.log,race-01.log,vet-01.log,build-01.log}`，源码／输入与日志hash匹配。targeted-01新测试字段／MIME fixture FAIL、targeted-02旧Store实例变化预期409而实际失败关闭503的fixture FAIL保留，只修新增tests，未弱化生产断言；首次commit缺默认identity失败、命令级身份重试成功，不冒充测试失败。
- 不同 **BE05-HTTP-A01 PASS**：固定完整534源链，原12顶层及新增授权场景真实TLS／SQLite／files独立复验exit0／8.473秒，源码前后不变。新增实际是2个顶层Test，StreamAuthorityChanges下admin-generation／bot-send两子例，加AndroidMissingTargetBeforeRead一例，共3场景；不是新增3个顶层。证据 `/tmp/relaynest-be05-http-a01-evidence/{command.json,independent.log,independent_test.go,before.json,after.json}`，独立日志SHA256 `859d10d01d57f60548b54095a4668abbede954e4791d4be52a11b58180369fec`。
- **BE-05-01-HTTP-I01 PASS**：Integrator `/root/backend_restore_retry` 从准确root `resume/backend-20261026@c3bbd66b696089ac6d580ddd0699f72075b48759` 在 `.worktrees/backend-be05-http-i01`／`integrate/backend-be05-http-i01-20261028` 仅一次无冲突重放 `53427226cf5c3f40ac997d710efc7ba0f7893f3b → a9962d4b278d6a191ddfbb7d579e31043c98b102`，range-diff单项 `=`；纯两doc提交后固定准确候选 `a647df879c607d524f58ab058c2537816f372671`。五路径逐字节同受审源，其余目标READ／F01／HISTORY／REF／HTTP、SQL1～10、Web／依赖保留，无PENDING／JOB源码或SQL11。原READ／REFERENCE受测候选未含本上传源码，不将旧结果当本次新组合检查。
- a647新组合实际：原12顶层＋A01新增2顶层承载3场景，共14顶层全 **PASS**（11.410秒）；全服普通 **PASS**（163.768秒），六包race **PASS**（519.789秒；httpapi／cmd／attachments／auth／bots／storage），全服vet／build **PASS**（2.734／1.566秒），gofmt／diff **PASS**，全部exit0。cmd race用时280.551秒，源测试默认10分钟超时原样保留，未改断言／超时。固定SHA／版本／start-end／exit／log hash、151产品输入及三真实embed／overlay前后和ff后全部一致；root已安全ff `c3bbd66… → a647df8…`，main `01c74a7…` 不变。证据 `/tmp/relaynest-be05-http-i01-20261028/{inputs.json,plan.json,range-diff.txt,commands.jsonl,post.json,target-fast-forward.log,*log}`。
- 计数核验历史：I01最后只读汇总脚本误assert“15个顶层”而发生一次AssertionError；实际按日志名称与父子例复核为原12＋新增2顶层、其中3新增授权场景全部PASS，post.json保留错误计数及正确复核事实。这是汇总计数FAIL，不是Go／产品FAIL；仅纠正汇总断言，未改源码、产品测试断言或重跑已PASS矩阵。
- 精确复用及限制：API六tracked输入／Web24输入与三真实dist hash、既有validator源日志证据准确复用；source及I01均未改API／Web，本轮npm／Python validator **NOT_RUN**，不冒新实测。父BE-05-01仍running，尚待外URL不抓取计数证明及R01→不同Auditor→准确父I01；下载／关联绑定／后台回收／设置HTTP／部署及全局门槛未由本叶完成。未push／部署／发布／创建完成标记。

<a id="g5-be05-storage"></a>

### G5 BE-05-01-STORAGE 流式持久化状态机准备

- 原任务：`BE-05-01-STORAGE`；类型：有限 leaf；状态：`done`（2026-10-26，由主调度依据 A01 PASS 与 I01 精确集成证据从 integrating 归档）；owner Backend Orchestrator；Implementer `/root/attachment_storage_implement`；依赖 P05-01 已集成 `ed488e543d892f2353fbc95f72a2f0af98ba9a8d`；固定基线 `388dd8ae1f3641bdfd26a7a421e4171b8da16971` / 目标 `work`。
- 历史派发准备：Backend Implementer，隔离 `/workspace/RelayNest-be05-storage` / `codex/be05-storage-20261026`；独占 attachments 包、本树/index 与迁移编号 **0008**，直到本包精确集成释放。其他任务不得预占或写该编号；不占 HTTP/auth/router/main 锁。
- 只允许五主文件：`server/internal/attachments/{models.go,repository.go,files.go,upload.go}`、`server/migrations/0008_attachments.sql`；测试只允许同包 `{repository_test.go,files_test.go,upload_test.go,schema_test.go}`。不修改通用消息幂等接口／旧迁移／消息关联／投递保护／恢复扫描／HTTP／配置路由。
- 目标：真实 SQLite＋附件卷的 staging/ready、稳定身份作用域上传去重、活动尝试 fencing；短事务接口可消费 Bot／Android 真实授权闭包，流式过程不持写事务，ready 结果仅提交成功后发布。ready 查询仅返回 ready；原名只显示、安全随机物理键、拒绝路径越界／symlink、不抓 URL。
- 验收：有界缓冲及实际字节限制、正 int64 可配置大小及最大值无溢出，hash/fsync/同卷 rename/目录 fsync；同键重试必须实际有界读流核字节摘要，同元数据不能直接重放，异内容冲突；重开/并发/取消/过期租约/迟到旧尝试/半文件/rename后DB失败/真实COMMIT失败无虚假成功。保留 staging/文件恢复证据，不越界实施 BE-05-04。
- 必跑附件／storage 普通与 race、vet/gofmt、旧库升级及旧迁移 checksum 验证；结果区分 PASS/FAIL/NOT_RUN。真实 HTTP 授权、实例门禁、Bot-first 读流顺序及全局设置路由尚未接线，全部明确 NOT_RUN；本包独审集成不等于 BE-05-01 完成。
- 历史实施交接：固定 `064807f9e5e126f48c9360d4d02a40fc209f790e`，准确五主文件＋四测试，树 clean、实施停止；迁移锁 0008 保留。Implementer 报附件/storage 普通、race、vet/build/gofmt、旧库升级及 0001～0007 字节/checksum PASS；额外全服务 build 因缺生成 Web dist FAIL，真实 HTTP 等范围仍 NOT_RUN。交独立 A01 固定 SHA 审查，不据局部结果标父任务完成。
- A01 结果：Agent `/root/attachment_audit_a01` `PASS`，固定 `064807f9e5e126f48c9360d4d02a40fc209f790e` 准确九文件，无需修复编号。独立附件/storage 普通／race／vet/build、gofmt/diff、旧库升级及旧迁移校验 PASS；临时归档补证续租 SQL 失败关闭 pipe、过期流／新 ready 后旧流 fencing race PASS。全服务 build 再现缺 Web dist FAIL；真实 HTTP／G1 等范围继续 NOT_RUN。
- 历史 I01 派发准备：Backend Integrator，准确受审提交由目标 `work` / `388dd8ae…` 可快进时保留 SHA；仅附件九文件／0008 迁移，迁移锁保持至组合检查通过。主树未提交 BACKEND 调度元数据由主调度所有，须保存并保护、不 stage；HTTP F01 在另一树只改 router/instance，不消费其未审代码。
- I01 结果：Agent `/root/attachment_integrate_i01` `PASS`，目标 `work` 无冲突快进 `388dd8ae1f3641bdfd26a7a421e4171b8da16971` → 原受审 `064807f9e5e126f48c9360d4d02a40fc209f790e`，准确九文件、无额外业务修改。附件/storage 普通／race、旧库升级、旧迁移字节/hash、全服普通／vet/build/gofmt 与两树锁版 Node 22.23.2／npm 10.9.8 Web 构建 PASS；目标回归/build PASS。原 BACKEND patch 哈希前后一致，index 空。Git/index/附件/0008 写锁释放，0008 永久占用；HTTP／G1／恢复扫描等 NOT_RUN。生成 Web dist 后全服 build PASS，关闭原缺 dist 构建失败，保留上述历史 FAIL。验收与精确集成证据齐全，仅本 STORAGE 子包完成，父 BE-05-01 不完成。
- 归档与限制：BE-DOC-S02 `/root/storage_archive_resume` 仅转录主调度完成决定及既有证据。HTTP 授权／Bot-first 实际读流顺序／Android／完整 G1／设置路由／恢复扫描均 NOT_RUN；准备完成不等于真实上传完成。0008 永久占用、写锁已释放；下一迁移 0009 派发前须另核编号及排他锁。

<a id="g5-be10-preflight-f01"></a>

### BE10-PREFLIGHT-F01 非零待办会话期限单位修复及A01编号关闭

- 类型：有限修复叶；状态：`done`（2026-10-28，总调度依据完整源检查、不同A02 PASS、新组合I01 PASS及root准确ff批准；原review→integrating过程保留）。仅处理 **BE10-PREFLIGHT-A01-01**，编号已由独立A02验证关闭并完成准确集成；不完成BE-10-02父调度或真实生产HTTP非零接线。
- 完整源链：固定产品基线 `1d1d666eae984768e7fe30685246fe709b7f3682 → b30504e2ace54282bc42d44f816df1d0b1678375`，Fixer `/root/push_expiry_fixer`，三路径 `server/internal/messages/pending.go`、`server/internal/messages/repository_test.go`、`server/internal/auth/message_push_expiry_test.go`。只将待办会话有效期比较与认证真实UnixNano存储对齐，必要fixture数据同单位，保留原断言、50ms／12并发及超时；未改SQL／HTTP接线／发送逻辑。
- 源检查：真实六例PASS（3.876秒）、messages普通PASS（5.924秒）、auth／devices／storage／changes普通PASS（42.071秒）、五包race PASS（122.212秒）、相关vet／全服build PASS（0.592／1.583秒）、gofmt／diff PASS；230跟踪输入及三源文件前后hash一致。证据 `/tmp/relaynest-be10-preflight-f01-20261028/{commands.jsonl,commit-verification.json,source-inputs.before.json,source-inputs.after.json,tracked-inputs.before.json,tracked-inputs.after.json,*log}`。该源命令记录有elapsed／exit／log hash但未记录start／end，不补造时间；首次commit缺默认identity失败后命令级身份重试成功，不算测试失败。
- 不同 **BE10-PREFLIGHT-A02 PASS**：固定b305新源，使用原A01同六例overlay（SHA256 `00077c663aac1b0cd14d42f7ad6b21ed51d8fec7710f50d45ea3523895634720`），独立exit0／4.001秒，两有效控制与四过期否定均PASS，关闭编号-01。证据 `/tmp/relaynest-be10-preflight-a02-76ui1pvy/{commands.jsonl,independent-six.log,tracked-inputs.before.json,tracked-inputs.after.json}`；日志SHA256 `26e2f8eb36c70102abf3d3c0d457536ff8d5e61ea4f4e6fe905907c91b280282`。
- **BE10-PREFLIGHT-F01-I01 PASS**：Integrator `/root/backend_restore_retry` 从准确root `resume/backend-20261026@34b21671a26bcccbe1154b167947dfb4f0ed23fa` 在 `.worktrees/backend-be10-preflight-f01-i01`／`integrate/backend-be10-preflight-f01-i01-20261028` 仅一次无冲突重放 `b30504e2ace54282bc42d44f816df1d0b1678375 → e42e7f9df12a16950928380e766bdce0559b8be7`，range-diff单项 `=`，纯两doc提交后准确受测候选 `20f5691aa1c17f181af87b8bcf51f417b2bc7acb`。三路径逐字节同受审源，READ／HISTORY／REF／HTTP、SQL1～10、Web／依赖及其余目标完整保留，未加入UPLOAD／PENDING／JOB源码或SQL11。
- 仅最终20f实际I01：原六例真实0／31／91天×双入口 **PASS**（4.913秒；2有效各job1，4过期ErrPushTarget、空回执及九项零增量），全服普通 **PASS**（159.039秒，覆盖五包普通及READ／0010启动兼容），相关八包race **PASS**（374.494秒；messages／auth／devices／storage／changes／sync／bots／attachments），全服vet／build **PASS**（2.465／1.618秒）、gofmt／diff **PASS**。全部exit0，固定SHA／版本／start-end／log hash及148产品输入、三真实embed、overlay前后及ff后均一致；此前中断时I01全部NOT_RUN事实保留，未将源／A02日志当新组合实测。已安全ff root `34b2167… → 20f5691…`，main `01c74a7…` 不变。证据 `/tmp/relaynest-be10-preflight-f01-i01-20261028/{inputs.json,plan.json,range-diff.txt,commands.jsonl,post.json,target-fast-forward.log,*log}`；Web／API未改，真实embed核hash复用，本轮npm／validator NOT_RUN。
- 范围：当前两正式消息HTTP入口仍为nil targets，本修复仅真实仓储非零目标期限，不证明生产HTTP非零接线或平台外发。后包生产接线、待办调度／策略过滤／adapter、全局及跨端门槛继续未完成；历史已完成PASS原范围不改。无新迁移，0009／0010永久占用，0011仍仅JOB私树独占；未push／部署／发布／创建完成标记。

#### 原BE10-PREFLIGHT-A01真实失败与修复前证据（历史保留）

- 当时状态：`changes_requested`；独立审计结论 **CHANGES_REQUIRED**（2026-10-28），固定基线 `1d1d666eae984768e7fe30685246fe709b7f3682`，发现 **BE10-PREFLIGHT-A01-01**：`server/internal/messages/pending.go:69` 以Unix秒比较认证真实写入的Unix纳秒期限，过期会话仍可生成非零待办。触发为合法SessionStore创建后恢复当前时钟、真实过期会话作为非零PushTargets，覆盖application／idempotent两仓储入口；不是平台发送验证。
- 六例实际结果：0／31／91天 × 两入口，两个有效会话PASS，四个过期会话FAIL；auth Inspect已拒绝过期，但消息入口返回err=nil、job=1且业务状态增加。期望为ErrPushTarget、空回执、零状态增量。日志子例标签31ns／91ns是显示错误，`age_days`与时钟构造实际按天。证据 `/tmp/relaynest-be10-preflight-a01-4qf9ln45/{baseline-test.log,commands.jsonl}`；最终overlay SHA256 `00077c663aac1b0cd14d42f7ad6b21ed51d8fec7710f50d45ea3523895634720`。前两轮缺embed及unused import编译失败分别保留于setup-failed.log／overlay-build-failed.log，不冒充业务结果。
- 影响边界：当前两条正式消息HTTP入口仍传nil targets，devices现有逻辑已按纳秒过滤；历史已完成PASS保留原范围，不追认其覆盖此次真实非零过期场景。该限制现由上述F01／A02／准确I01闭环，但不扩大本段当时验证范围。

<a id="g5-be10-compose"></a>

### BE-10-01 Hub 推送注册与设备通知偏好

- 完成追加依赖（不改变上述开工依赖）：[BE-01](BACKEND.md#be-01)；基础准备可先进行，真实全范围收口必须满足。

- P01 有限规划（2026-10-26）：owner Backend Orchestrator，Agent `/root/push_registration_plan`，固定基线 `571dc5dca696d4dfb446a868ca58169cf488579b` / `work`，只读细化当前 leaf 的准确文件／事务边界与准备／真实接线拆分，不修改代码、状态或分配迁移。状态仍未开始；新迁移候选 0009 须另核锁，完整 BE-01 门槛未满足，不据规划放行真实注册验收。
- P01 已返回 STORAGE／HTTP 两有限子包建议；主调度采用并保留原完整验收。R04 `/root/resume_checkpoint` 核实际基线 `571dc5dca696d4dfb446a868ca58169cf488579b`，P04-05 精确集成 `d78711faa56dd01b7e10cf4eec3b70490f2d04e9`／最终 `005994a049635163e879c069ecf02526c8ddbe1f`、P03-07 集成 `981692f49663075aa553ce45aa645e75197b15ea`／最终 `375a270c74080d9eb3ea8e975d8c79814afc091f` 均为祖先；九个 worktree 均无 0009 或迁移修改，当前无迁移 writer。由调度将 0009 排他分配下列 STORAGE 子包至精确集成，不与未来迁移并写。

- 证据与交接：STORAGE 与五端点 HTTP 各已独审／精确集成后，本父又经独立 R01／A01／I01 核收原 T9.3 服务器注册版本／时间、CAS、并发模式切换、撤销重登旧回调及条件解绑不误删和实例绑定；不是仅依子叶 done 推定完成。平台发送／真机／全局实例门禁仍不在本父有限服务器范围。

- `BE10-COMPOSE-R01`：独立验证树 `.worktrees/backend-be10-compose-r01`／`verify/be10-compose-r01-20261028`，固定 `d4604184a2d3e2616146d63b8b902d052a723237`，两组临时 overlay 在 `/tmp/relaynest-be10-compose-r01-20261028` 补齐 E01并发模式切换、E02撤销／重登后旧回调隔离。首轮 ordinary因 overlay八处误用 fixture类型编译 **FAIL**（exit1／2.155秒），仅临时 overlay的 f→f.store 修正，原日志及v1保留，不改生产；v2 ordinary **PASS**（exit0／3.997秒）、race **PASS**（13.248秒）。最终 overlay源 SHA256 `f6599d886a56496e11dc9e97263a30ac87472c4dcc6ed9f19439efdf2b1ed0bf`，实际 commands／源文件／输入可核，不因准备材料存在标PASS。

- 旧ID：`P09-01`；类型：有限分拆收口；状态：`done`（2026-10-28，总调度依据 R01补证、不同 A01 PASS及准确 I01真组合／目标集成批准；只含既定服务器注册／偏好／条件解绑范围）。
- 依赖：[已完成基线 P04-05](COMPLETED.md#g1)、[已完成基线 P03-07](COMPLETED.md#e1)。
- 允许范围：`server/internal/devices/**`、`server/migrations/**`，及对应测试。
- 门槛：`G-API`、`G-GIT`；条件未实证不记通过。

按T9.3冻结通道集合及逐provider注册版本/时间实现，仅当前设备会话可登记；部分上报不清空其他通道。全局/Bot/DND/预览配置CAS独立于注册代次，SDK注册成功不改用户提醒模式，vendor激活须满足冻结资格门槛。target_invalid仅设备/会话/provider/发送注册版本匹配才条件解绑对应注册，保留设备/会话/其他通道/偏好；会话撤销使全部通道失效，token与秘密不回显/日志。覆盖同版异值、并发模式切换、撤销后迟到登记及旧代次失败不删新注册。Android写入口另须BE-01实例绑定。

- `BE10-COMPOSE-A01`：不同 Auditor固定 `d4604184a2d3e2616146d63b8b902d052a723237`（server／Web／依赖／真实 embed与已集成 `539f0b8…` 相同），审原父完整验收和 R01实际证据，**PASS**、无发现。独立普通 exit0／5.905秒、race exit0／13.227秒，overlay保持上述源hash。证据 `/tmp/relaynest-be10-compose-a01-20261028/{inputs.json,commands.jsonl,post.json,independent-ordinary.log,independent-race.log,prior-log-verification.json}`。E02为真实注册HTTP＋真实仓储 ApplyPushResult＋HTTP读回，不是外部平台发送。
- `BE10-COMPOSE-I01` **PASS**：在包含已审引用与推送HTTP的新准确候选 `74e07f65a8b70bd91d129920573a779783c1a746` 复用完全相同 overlay源（只映射审查树路径），实际定向普通 exit0／4.017秒、race exit0／9.723秒，证据 `/tmp/relaynest-be10-compose-i01-20261028/{inputs.json,commands.jsonl,post.json,ordinary.log,race.log}`。同SHA全服普通153.066秒／七包race554.827秒／vet／build／gofmt／diff，准确复用本轮 REFERENCE-I01 已完整新组合检查，不重复跑、不冒新实测。root安全 ff `bd95d01… → 74e07f6…`、受测输入不变；本父无产品源提交，原STORAGE／HTTP完成档案及四映射不重写。
- 范围与保留限制：通道注册／偏好CAS／目标版本条件解绑／会话撤销与迟到结果／实例门禁已服务器组合验证；vendor资格未装配时失败关闭，不代表OPPO资格通过。发送调度、FCM／OPPO adapter、厂商审核／真机、Android正式通知以及全局 G-INSTANCE-RUNTIME／G-INSTANCE-COMPOSITION继续未验收。HTTP-I01旧harness目录O_EXCL冲突FAIL及修正完整复验历史保持原[完成档案](COMPLETED.md#g5-be10-http)，R01临时类型编译FAIL保留；不删断言／改超时、不以准备或批准代验。后继BE-10-02仅此前置完成，共享已读运行时尚在实施须规划真实依赖，不自动ready整包。未 push／部署／发布／创建完成标记。

<a id="g5-be10-http"></a>

### BE-10-01-HTTP 当前设备注册与偏好真实接线

- 类型：有限 leaf；状态：`done`（2026-10-28，总调度依据完整源链、A02 PASS 和准确 I01 新组合检查／精确集成批准有限叶完成）。依赖[已完成 STORAGE](COMPLETED.md#g5-be10-storage)、[已完成 BE-01 现有后端父范围](COMPLETED.md#g5-be01)；写准入修复已[独审及精确集成](COMPLETED.md#g5-storage-busy-01)。
- 阶段 1 原 Implementer `/root/push_http_implementer` 从 `3aec54e04945c9e1a8442f7ca996d2ded15f56e7` 开始；恢复 owner `/root/push_http_resume` 已保存 checkpoint `1291ea355c3c3f837df56f5d5e1f08d6bc641059`（parent `3aec54e…`）。阶段 2 已由总调度正式放行：先以 `backup/push-http-phase1-20261027` 保留原 checkpoint，再从真实 ADMIN 归档基线 `5d6a57df45f9bbf8185da8957e83ace08ec0ef27` 无冲突重放为 `3471a3bbc659580f14c627e5dc6c7cbbc9e7b680`（parent `5d6a57d…`），range-diff 一项 `=`；原三文件与 `1291ea3` 一致、其他 server 与 `5d6a57d` 一致，不重新实现。隔离树／分支仍为 `.worktrees/backend-push-http` / `impl/backend-push-http-20261027`，该树保留源可达性，本叶实施／集成写锁已释放。
- 阶段 2 checkpoint `2c7038a2196be27927e5ba474f93cfbcfef56533`（parent `3471a3b…`，真实基线仍 `5d6a57d…`）已保留，累计四主实现＋三测试；旧 `1291ea3…` 备份和 `3471a3b…` 一项等价重放历史保持，不重复重放代码。阶段 1 六组 handler 普通在准确 `3471a3b…` **PASS**（exit 0／6.090 秒），证据 `/tmp/relaynest-push-http-phase1-20261027-iq565fyx/commands.jsonl`。
- 阶段 2 短普通已有完整 **PASS**：同 `2c7038a…`、Go 1.27.3 的 `router-exact` 与 `assembled-push` 均有开始／结束／exit 0（1.716／10.810 秒），五组生产 TLS＋SQLite、真实 main 子进程／日志／事务／授权用例通过；证据 `/tmp/relaynest-push-http-phase2-20261027-2dqjpoxx/{inputs.json,commands.jsonl,router-exact.log,assembled-push.log}`。不将额度中断误记成这些已完成短检查失败或未跑。同 `2c7038a…` 全服普通另已完整 PASS（exit 0／137.877 秒），vet／build（7.625／1.591 秒）及 gofmt／diff／clean 检查 PASS，实际证据 `/tmp/relaynest-push-http-ordinary-20261027-bjpjyeum/{inputs.json,commands.jsonl,*log}`；准确复用而非本 I01 重跑。同 `2c7038a…` 相关六包 race 已完整 **PASS**（exit 0／566.727 秒），证据 `/tmp/relaynest-push-http-race-20261027-ltqvy615/commands.jsonl`。A01 正式 **CHANGES_REQUIRED**：`BE10-HTTP-A01-01` 为偏好 PUT 409 schema 缺公共 INSTANCE_MISMATCH；`BE10-HTTP-A01-02` 为仅合成 Bearer 不足真实 Bot 误用验证。此为 A01 历史结论，后经下方 A02 独立关闭两个编号及 I01 精确集成，不删除原失败事实。
- 仅允许 `server/internal/devices/{push_http.go,http.go}`、`server/internal/httpapi/router.go`、`server/cmd/relaynest/main.go` 四主文件及必要对应测试；其他生产源码、auth／仓储／迁移／API 契约只读。各检查仍按总调度排队，一次仅一个 full／cmd／race 窗口。上述 `5d6a57d` 为本叶已授实际产品基线，canonical 纯元数据推进不要求它再次重放。
- 编号修复进展：`BE10-HTTP-A01-02` 已仅两测试提交 `8cf74db212762719315be9108a8a1b32a2f87083`（parent `2c7038a…`），真实管理 HTTP 签发 Bot 凭据、Bot 消息 201 正对照及五个拒绝 401／no-store／完整 hash 不变；普通／race 完整 PASS（exit 0／3.874／11.075 秒），实际 `/tmp/relaynest-be10-http-f01-02-20261027-2ozs655w/commands.jsonl`，后经不同 A02 独审 PASS。`BE10-HTTP-A01-01` 用户已明确同意最小合同修正，Fixer `/root/push_http_resume` 获准修改 `server/api/openapi.yaml` 的该 409 oneOf：保留 settings 并补公共 Error 嵌套 `error.code=INSTANCE_MISMATCH`；必要的 `server/api/validate.py` 最小合同验证工具及对应测试补证也获授权，仅限本 operation 例子／真实响应接入，不弱化通用 schema、不改其他 operations／依赖，runtime 四文件不改。已形成干净提交 `df160ff33c4db6d6818678f11463773be0a5be40`（parent `8cf74db…`），完整源链仍为 `5d6a57d… → 3471a3b… → 2c7038a… → 8cf74db… → df160ff…`。四 runtime Go 文件／依赖锁与 `2c7038a…` 字节一致，复用临时锁版工具 venv，不再记待用户答复。原完整 validator **PASS**（118 schemas／1537 正例／918 负例／34 变异／40 operations）；真实三种409普通／race完整 **PASS**（exit 0／3.711／9.949秒），schema 正例及20负例 PASS，旧 schema 拒实例响应反证保留，实际证据 `/tmp/relaynest-be10-http-f02-01-20261027-zm24osyv/verification-0d_lyl6p/commands.jsonl`。最终 `df160ff…` 全服普通131.427秒／相关六包 race430.116秒／vet0.446秒／build1.369秒／gofmt／diff／clean 均完整 exit 0 PASS，独立证据 `/tmp/relaynest-push-http-final-df160ff-20261027-xI26hj/{commands.jsonl,handoff.txt,tracked-inputs.sha256,embed-inputs.sha256}`；未将 `2c7038a…` 历史冒充新 SHA 实测。A01 独立两组 5.241 秒仅有工具输出、无持久 stdout／ledger，不伪造日志路径；后续 A02 已提供准确新证据，root 产品代码已于 I01 精确合入。
- 原有限 HTTP 验收已通过最终真实 router＋main＋SQLite、不同固定 SHA 审计及精确集成；不是仅 handler／编译验收，父 BE-10-01／平台发送／跨端门槛不自动通过。
- 按冻结五 operations 接线 GET/PUT current/push、DELETE current/push/{provider}、GET/PUT current/notifications；真实当前 Android Bearer 与同事务实例门禁，拒 Web/Bot／目标选择器／客户端时间，262144 字节／严格 schema／no-store／无 token。原设备及认证／Bot 路由非回归，真实 router＋main＋SQLite 组合，不以直接 handler 测试冒充。
- 原 BE-10-01 全部验收及资格边界保留，STORAGE 准备不替代 H01／G1；后端发送适配器及平台实测仍分别归后续任务。

- `BE10-HTTP-A02`：不同 Auditor 固定完整 `5d6a57d… → 3471a3b… → 2c7038a… → 8cf74db… → df160ff33c4db6d6818678f11463773be0a5be40` **PASS**，关闭 `BE10-HTTP-A01-01`／`BE10-HTTP-A01-02`，无新增发现。独立真实 HTTP exit 0／6.010秒、标准合同 exit 0／4.060秒；真实三响应各恰好命中一个分支、20负例拒绝，前后输入不变。证据 `/tmp/relaynest-be10-http-a02-20261027-mJkise/{commands.jsonl,before.json,after.json,independent-http.log,independent-contract.log,responses}`。
- `BE-10-01-HTTP-I01` **PASS**：从准确 root `dbdb1c9fa13a8053bd039d4910c18dd2cdffd060` 保留 HISTORY 六文件和元数据，以隔离候选 `.worktrees/backend-push-http-i01`／`integrate/backend-push-http-i01-20261027` 完整映射 `3471a3bbc659580f14c627e5dc6c7cbbc9e7b680 → 956ae54e7830ab3657ba67be7839bc9a709a9cbf`、`2c7038a2196be27927e5ba474f93cfbcfef56533 → 452bfb6d45351655876ac434e6cd97da52f8b15b`、`8cf74db212762719315be9108a8a1b32a2f87083 → 18328d039db74bec9dcf739e2ff792081061b146`、`df160ff33c4db6d6818678f11463773be0a5be40 → 539f0b89263775658bde56d6d07af88493ec8e0d`，range-diff 四项 `=`、无冲突。HTTP 九路径与受审 df 逐字节同，其余 server／docs／Web／go.mod／go.sum／九 SQL 与 targetbase 同；新并集含 HISTORY，server whole tree与 df 不同，不用旧分支 PASS 代验新组合。实际同版真实 embed 复制核 hash，工具锁 Go1.27.3／Node22.23.2／npm10.9.8。
- 准确 `539f0b8…` 新组合实际检查：关键真实 HTTP **PASS**（exit 0／12.944秒）、全服普通复验 **PASS**（134.209秒）、相关六包 race **PASS**（438.697秒）、全服 vet／build（0.752／1.270秒）、gofmt／diff 和标准 API validator（4.008秒）均完整 exit 0；validator 消费本候选关键 HTTP 捕获三响应。全服 race（区别于要求的六包 race）、平台真机、FCM／OPPO发送和全局 G-INSTANCE 门禁未由本叶验证。证据 `/tmp/relaynest-push-http-i01-20261027-130400/{inputs.json,commands.jsonl,complete.json,integration-pre.json,integration-post.json,target-fast-forward.log}`。
- 历史 `BE10-I01-ENV-01`：本轮 harness 给关键 HTTP／whole 共用 capture 目录，后者 O_EXCL 写 `settings-conflict.json` 遇 file exists，原全服 ordinary exit 1／140.255秒。该 harness 目录冲突不记为已证业务失败；总调度批准仅临时 runner 每命令独立目录，原输出／run.v1.py／日志保留，不改受审源码／断言／超时，随后完整复验 PASS。恢复后核日志 hash／源码／真实 embed输入而未重跑矩阵，于2026-10-28安全 ff root `dbdb1c9… → 539f0b8…`，target tree `7410f9b615751cf2ae33edbe83d56144603d0260` 等于受测候选，main ref 不动、无迁移分配。
- 完成仅上述五个当前设备注册／偏好 operations；父 BE-10-01 尚须按原验收进行存储＋HTTP 的独立父组合与资格边界核收。发送调度、FCM／OPPO adapter及真机、Android正式通知、全局两实例 runtime／composition继续按各自任务，不因本叶完成通过；未 push／部署／发布／创建完成标记。

<a id="g5-be10-storage"></a>

### G5 BE-10-01-STORAGE 注册与通知偏好存储准备

- 类型：有限 leaf；状态：`done`（2026-10-27，总调度依据 A02 PASS、精确映射及权限解除后 I01 全部必要检查 PASS，从 integrating 归档）；owner Backend Orchestrator；Implementer `/root/push_storage_implement`；固定基线 `571dc5dca696d4dfb446a868ca58169cf488579b` / 目标 `work`。只消费已集成 AUTH／设备／契约基础，不消费尚在独审的 HTTP；父项完整完成门槛不作本准备包前置。
- 派发准备：Backend Implementer，隔离 `/workspace/RelayNest-be10-registration` / `codex/be10-registration-20261026`；独占本树/index、三个 devices 新文件与迁移编号 **0009**。仅允许四主文件 `server/internal/devices/{push.go,push_repository.go,notifications.go}`、`server/migrations/0009_device_push.sql`，以及同包 `{push_test.go,push_repository_test.go,notifications_test.go}`；auth／changes／storage／router／main／旧迁移均只读。
- 注册按 Session.id＋provider，偏好按 device_id 共享，独立 int64 版本按冻结 schema；公开状态不含 token，内部发送目标与公开 DTO 分离。两 provider 共存，缺失／墓碑、同版同值重放先于 CAS、异值冲突、删除／明确失效推进屏障及耗尽，迟到结果必须匹配设备／会话／provider／发送版本，绝不删会话／新 token／其他通道／偏好。
- 偏好默认读不落库，不同会话同设备共享 CAS；完整替换校验按冻结合同，同值也推进版本／服务端时间，与 typed `DeviceNotificationsUpdated` change 同事务；失败／COMMIT 故障无部分结果。vendor 激活只能消费可信服务端资格状态及当前会话有效 opush 注册，默认未就绪；普通 FCM 准备不等待 OPPO 资格。
- 撤销／退出／重置／离线删会话须持久化使所有该会话注册失效，目标读取仍核有效 Android 会话、期限和认证代次；不靠提交后 hook 或版本耗尽阻断安全撤销。迁移内受控清理及 FK 可作为内部方案，禁止改既定 auth 安全策略。
- 验收：真实 SQLite 双 provider／双会话共享偏好、重开／并发 CAS／代次耗尽、迟到登记和 target_invalid 不删新注册、撤销／重置／离线删除、changes／语句／COMMIT 回滚及脱敏边界；devices/auth/storage/changes 普通与串行 race、vet/build/gofmt、旧库升级与 0001～0008 checksum 不变。真实 HTTP／实例错误顺序／完整 G1／推送发送为 NOT_RUN，不等于父 BE-10-01 完成。
- 实施交接：固定 `20b9fffbe8063e25cee60456dc96e53f594866cb`，仅获配七路径，树 clean 并释放实施／重检查写权，0009 迁移锁保留。四包普通／串行 race、全服 vet/build/gofmt PASS；全服普通唯一 FAIL 为范围外 `attachments/schema_test.go` 固定要求迁移总数 8，实际新增后为 9。未越范围改断言，交独立 A01 编号审查，再有限修复；真实注册 HTTP 等仍 NOT_RUN。
- A01 Agent `/root/push_storage_audit_a01`，固定 `20b9fff…` 只读审查；先轻量／静态，重型检查等待 BE-01-COMPOSE 释放，不并发跑 full／cmd／race。
- A01 结果：`CHANGES_REQUIRED`，仅 `BE-10-01-STORAGE-A01-01`：新增 0009 后附件七迁移旧库升级计数断言仍为 8，实际为 9；独立单测复现 FAIL，临时 overlay 仅改 8→9 且保留旧七 checksum 断言后 PASS。四包普通／串行 race、全服 vet/build/gofmt、锁版 Web、旧 0001～0008 校验与真实升级 PASS；未确认其他业务缺陷，全服普通唯一此 FAIL，真实 HTTP 等仍 NOT_RUN。
- F01 派发准备：以 `20b9fffbe8063e25cee60456dc96e53f594866cb` 创建 `/workspace/RelayNest-be10-registration-fix` / `codex/be10-registration-f01-20261026`，仅允许 `server/internal/attachments/schema_test.go` 按 A01-01 更新完整数量为 9，旧七 checksum／原升级断言必须保留。不改推送生产代码、迁移或其他测试；重检查等待 COMPOSE A01 释放，修复新 SHA 后重审。
- F01 Agent `/root/push_storage_fix_f01` 已派发；只准备及轻量单测，full／cmd／race 等待资源释放，0009 排他迁移锁仍保留至本包精确集成。
- F01 交接：`003c990c08ef606b838ae2715bf1d33d19e7fa00`（parent `20b9fffbe8063e25cee60456dc96e53f594866cb`），仅 `attachments/schema_test.go` 一行计数 8→9，旧七 checksum 和其他升级断言原样；源树 clean 并释放写权。定向升级、附件/storage 普通／race、全服普通、受影响 vet／全服 build／gofmt/diff PASS；基线计数 FAIL 和首次缺 Web dist FAIL 均保留，生成锁版产物后全服 PASS。A02 准备固定新 SHA 重审，不据 Fixer 报告放行。
- A02 Agent `/root/push_storage_audit_a01` 已派发，固定 `003c990c…` 完整链重审，重型资源独占，尚无结论。
- A02 结果：`PASS`，固定 `003c990c08ef606b838ae2715bf1d33d19e7fa00`，关闭 A01-01；仅数量一行变化，七推送文件／生产代码／旧迁移无差异。独立升级单测、全服普通、附件/storage race、锁版离线 Web 与 gofmt/diff PASS；A01 同生产代码四包 race／vet/build 证据明确复用，未冒称新跑，历史 FAIL 保留，源树 clean、重型锁已释放。
- I01 派发准备：Backend Integrator，目标 `work` / `a97cfcee014298c64e8468a7b9eb1d007b635e37`，精确组合 `20b9fffbe8063e25cee60456dc96e53f594866cb`＋`003c990c08ef606b838ae2715bf1d33d19e7fa00`；仅推送七文件／0009 与已编号附件单测，保护 BACKEND 元数据，不消费并行实施的 ADMIN。受影响检查／精确集成后才释放 0009 排他锁，父 BE-10-01 真实接线仍未完成。
- I01 Agent `/root/push_storage_integrate` 已派发，独占目标业务 Git/index、推送与 0009 写锁及重型检查；主调度自本行记录后冻结文档至其释放，ADMIN 只实施／轻量定向等待。
- 迁移断点实核：目标仍 `a97cfcee014298c64e8468a7b9eb1d007b635e37`，候选 `47c08a7a3fd7712379d2ef4ba6e86ad897daacb4`／父 `77025cfe0a3317a0d0df5a2b97ff1be786a3104b`，两提交已在隔离树但未快进目标；所有仓库测试／Git 进程已退出。旧 I01 冻结随会话中断解除，本次仅备份不补造 I01 PASS；0009 编号仍为此包预留，恢复入口 `recovery/backend-20261026-push-i01`。

- Debian I01 恢复：Integrator `/root/push_integrator` 在上述独立本地集成仓库核共同父 `a97cfcee…` 与恢复快照 `bde0193a…` 仅差 BACKEND／PROGRESS 元数据；无冲突精确重放候选，`bde0193a5e1f561cb0b3d30a08bea691fcd499a8 → 17aefdc7ad0a328356ebbbc1ebe326ef807ead76 → af84503f62083b3d3bfe704694a8807d3928d678`。映射 `20b9fff… → 77025cf… → 17aefdc…`、`003c990c… → 47c08a7… → af84503…`，两轮 range-diff 均 `=`；server 与原候选逐字节一致、docs 与恢复快照逐字节一致，0001～0008 未改，无新业务逻辑或冲突需重审，历史 A02 PASS 仍有效。
- 权限解除前 I01 检查：`go test ./internal/devices ./internal/auth ./internal/storage ./internal/changes ./internal/attachments` 及相同五包 `go test -race -p 1` 实际运行；devices/storage/changes/attachments 普通与 race PASS（含真实 SQLite 旧八迁移升级及 checksum／并发／回滚），auth 普通与 race FAIL（`TestHTTPLifecycle` socket 禁止）。`go test ./...` 实际 FAIL，cmd/auth/httpapi 均受 IPv4／IPv6 socket 限制，其余包 PASS；全服 `go vet ./...`、`go build ./...`、gofmt/diff、锁版 Web build PASS。Go 使用 `GOTOOLCHAIN=local GOCACHE=/tmp/relaynest-i01/go-cache GOPROXY=off`；实际新日志 `/tmp/relaynest-i01/logs/{affected-test,affected-race,full-test,vet,build,web-ci,web-ci-ignore-scripts,web-build}.log`。首次 npm／旧历史失败均保留，不将未执行 HTTP／平台发送／跨端 gate 视为通过。
- 权限解除前 I01 **BLOCKED（环境）**：源码精确合入只存在本地独立仓库 `resume/backend-20261026@af84503…`，原主树未更新；工作树 clean、检查进程已结束并释放业务／重型锁。迁移 **0009** 已实际存在并永久占用，I01 验收闭环前仍保留其排他分配，后续编号须重新实核；不得重复分配 0009、重复 cherry-pick 或将候选存在视为完成。环境解除后补齐失败的真实组合检查，再由总调度决定完成归档。

- I01 权限解除后复验（2026-10-27，UTC+8）：新 Integrator `/root/push_i01_retry` 先核 IPv4／IPv6 loopback 真实监听 PASS，锁版 Go 1.27.3／Node 22.23.2／npm 10.9.8 PASS；受测目标为独立本地仓库 `resume/backend-20261026@c4c4603e93fd22132bff59c49e40a1acc2b4625b`（源码组合仍为 `af84503f62083b3d3bfe704694a8807d3928d678`，后续仅交接元数据）。不再 cherry-pick、不改业务／断言／超时；复核两源提交 range-diff 均 `=`、server 与 `47c08a7…` 一致、0001～0008 未改，原 main 未更新。
- 本轮实际检查全部 **PASS**：原 `npm run build`（typecheck＋Vite）；`go test -p 1 -count=1 ./internal/devices ./internal/auth ./internal/storage ./internal/changes ./internal/attachments`；`go test -p 1 -count=1 ./...`；相同五包 `go test -race -p 1 -count=1`；全服 `go vet ./...`、`go build ./...`；七 Go 文件 gofmt、diff/status、精确来源比对。五包普通及 race 包含真实 8→9 升级、checksum、并发及回滚；auth 完整 race PASS（114.516 秒），此前 auth／cmd／httpapi 的 socket FAIL 已复验消除，旧失败记录保留。全部检查串行，未并跑重型矩阵。
- 新运行精确命令／退出码：`/tmp/relaynest-i01-retry-20261027/commands.tsv`，各 `affected-test.log`、`full-test.log`、`affected-race.log`、`vet.log`、`build.log`、`web-build.log` 及 `integration-evidence.log`；均为本环境临时证据，不冒称随 Git 保存。Go 环境为 `GOTOOLCHAIN=local GOPROXY=off GOCACHE=/tmp/relaynest-i01/go-cache`。本轮没有业务修改或新增源码提交，无新增逻辑冲突需要重审；A02 PASS 仍适用于精确组合。
- I01 最终 **PASS**：总调度批准本 STORAGE 叶完成；业务／Git/index／重型锁均释放，迁移 **0009 永久占用**，后续编号仍须实核。仅存储准备闭环；全服 race（非本叶必检）、推送真实注册 HTTP、完整 G1、平台发送及跨端验收本轮 **NOT_RUN**，父 BE-10-01／HTTP 及全局 G-INSTANCE-RUNTIME／COMPOSITION 均不因本叶完成而通过。未 push、部署、发布或创建完成标记。

<a id="g5-storage-busy-01"></a>

### G5 STORAGE-BUSY-01 同库共享可取消写入准入修复

- 稳定发现 **STORAGE-BUSY-01**（原发现，用户已授权有限修复）：位置 `server/internal/storage/storage.go:117–118、136–139` 的 Store.Write／BeginTx 写准入；触发为同数据库双 Store、原同键并发测试在 50ms 写锁期限下竞争，证据定位 `server/internal/messages/create_test.go:105–130` 及上述 baseline diagnostic。影响为已有基线也可在业务回调之前返回 SQLITE_BUSY，阻断 ADMIN 全服 race 必检闭环，不能凭 ADMIN 六组 HTTP 通过关闭。Researcher 与独审建议按数据库身份共享可取消的写准入 gate，以符合既定单写目标；这涉及跨 Store 生命周期／调度，超出 ADMIN 四主文件范围且属于重要架构变更，原依 AGENTS 第 3 条提交用户决定；用户本轮已明确“同意”，该确认门槛已解除，修复仍须限定范围、独立审计与准确组合验证。
- 已授权修复子叶 **STORAGE-BUSY-01**：状态 `done`（2026-10-27，总调度依据完整源链 A02 PASS 及精确 I01 PASS，从 integrating 归档）；角色 Backend Fixer，owner `/root/storage_busy_fixer`，固定权威基线 `57b27f44453e338e094c87c8c8a7c74f76230e1c` / `resume/backend-20261026`。隔离树 `.worktrees/backend-storage-busy` / `fix/storage-busy-01-20261027`，由 Fixer 排他准备／写入该树与 index；文档／目标 index 由独立文档执行者维护，需精确集成时保留后续元数据。历史 clone 不再推进。
- 修复范围仅为 `server/internal/storage/storage.go`、新增 `write_gate.go`、必要 `offline.go` 及对应测试；实现按同数据库身份共享、可取消的写入准入，涵盖跨 Store 生命周期／调度，不改变业务或冻结外部接口。原 messages 测试的 **50ms BusyTimeout／12 goroutine／双 Store／全部断言保持不变**；禁止调高超时／吞掉 busy／重放业务 callback 或 COMMIT，不扩为其他架构重写。
- 修复实施 checkpoint：`c806fd9ba5baddd97b5e3e325f7ee5c9da808338`（parent `57b27f44453e338e094c87c8c8a7c74f76230e1c`），受控代码树 clean；实际三个主实现 `storage.go`／`write_gate.go`／`offline.go`，加 `storage/write_gate_test.go`，及获确认的 `messages/repository_test.go` 单子例适配（旧 Background 跨 Store 写锁等待自身未释放事务，改为新的有界 context 观察取消）。原 `messages/create_test.go`／50ms BusyTimeout／12 goroutine／全部断言未改，无业务接口或迁移变化。
- 该 SHA 实际检查 **PASS**：storage 普通、原 messages 同键／排队取消子例 20 轮 race（71.048 秒）、storage/messages/auth 相关 race、锁版 Web build、vet/build/gofmt。全服普通 **FAIL**（exit 1，201.761 秒），唯一失败为 changes 旧跨 Store Background 自等待导致 SIGQUIT，其余包 PASS；不能用局部 PASS 替代全包结果，也未改超时掩盖失败。实施新日志 `/tmp/relaynest-storage-busy-01/commands.jsonl`、`storage-test.log`、`messages-target-race20.log`、`related-race.log`、`full-test.log`、`web-build.log`、`vet.log`、`build.log`。
- 历史独审 **STORAGE-BUSY-01-A01：CHANGES_REQUIRED**，当时尚无 PASS。**STORAGE-BUSY-01-A01-01**：`server/internal/changes/changes_test.go:298–313` 的旧 fixture 在已持同库写入准入时，用 Background 的另一个 Store 写入并等待自身尚未释放的事务；与全服普通 SIGQUIT 对应。仅该子例可改用新的有界 context，断言 `DeadlineExceeded`、`entered=false`，保留原 10ms／预取消验证及最终 `changes=1、entities=0`；复验该子例和全服普通不得再自等待。
- **STORAGE-BUSY-01-A01-02**：`server/internal/storage/storage.go:98–105`、`offline.go:87–119` 在成功发布 Store 前缺请求路径／gate 规范文件身份复验。Auditor 原生连接 hook 对 Open／OpenOffline × symlink／物理替换四例，实际均返回 Store＋nil，后续 Read 才报 `ErrInstanceUnavailable`；原／另一库 marker `101／202` 均未变，**未证实错误数据库副作用**。要求成功发布前复验所请求及 gate 规范文件身份，失败时完整清理，不承诺消除任意外部替换 TOCTOU。复验原四例应拒绝发布无效 Store，并保正常 Open／OpenOffline 与清理路径。独立实证为 `/tmp/relaynest-storage-busy-a01/initialization-evidence.txt`、`initialization-fault.log`、`inputs.json`、`overlay.json`，该 overlay 命令 exit 1，四子例均 FAIL。
- 历史总调度精确尾修复范围：不同 Fixer `/root/storage_busy_fixer` 从同树 `c806fd9…` 仅修 A01-01／A01-02，当时新 SHA 尚未知；主实现仍三个文件，必要测试额外允许 `changes/changes_test.go` **仅上述子例**，`messages/create_test.go`／原 50ms／12 并发与全部断言只读。不扩修未列问题（包括有界 GC 引用尾槽），不提前关闭发现。后续固定新 SHA 独立重审，再精确集成并验证真实 ADMIN＋fix 组合；该组合全服 race 仍 **NOT_RUN**。**ADMIN-A03-B01 保持 OPEN、ADMIN-A03 保持 BLOCKED**，未标 done，原失败均保留；临时日志不冒称随 Git 保存。

- F02 正式交接：`e0c3631df03b34d2e717761e0ce2c78f707be5b1`（parent `c806fd9ba5baddd97b5e3e325f7ee5c9da808338`），完整链 `57b27f44453e338e094c87c8c8a7c74f76230e1c → c806fd9… → e0c3631…`；实际六路径为三个 storage 主实现、`storage/write_gate_test.go`、messages／changes 两个精确取消子例。尾修复仅处理 A01-01／A01-02，保原 messages 50ms／12 并发与所有断言、changes 原 10ms／预取消／最终 changes=1 entities=0；没有业务 callback／COMMIT 重放、延长 busy_timeout 或未编号扩修。
- F02 该 SHA 实际 **PASS**：storage／changes 普通、消息并发／取消五轮 race（17.937 秒）、storage/messages/auth/changes 四包串行 race、全服普通（124.995 秒）、vet/build/gofmt。证据 `/tmp/relaynest-storage-busy-f02/commands.jsonl` 及 `initialization-test.log`、`storage-changes-test.log`、`messages-target-race5.log`、`related-race.log`、`full-test.log`、`vet.log`、`build.log`；早期 c806 二十轮 race 与 e0 五轮各按实际 SHA 保留，未混称同轮。旧全服 SIGQUIT 失败已复验通过，历史记录不删除。
- 最终独审 **STORAGE-BUSY-01-A02：PASS**：Auditor `/root/admin_a02_retry` 固定 `e0c3631…` 审查上述完整源链，关闭 A01-01／A01-02；原 A01 四例原生故障 hook 加正式新四例初始化回归全部 PASS，均拒绝发布无效 Store，marker101／202 未变；独立消息／changes 并发取消五轮 race PASS。证据 `/tmp/relaynest-storage-busy-a02/commands.jsonl`、`initialization-independent.log`、`concurrency-cancellation-race5.log`、`inputs.json`、`overlay.json`；不扩大为彻底消除外部文件替换 TOCTOU。
- I01 精确集成 **PASS**：Integrator `/root/push_i01_retry` 在原仓库隔离 `.worktrees/backend-storage-busy-i01` / `integrate/storage-busy-01-20261027`，目标 `resume/backend-20261026` 从 `5cc5dcbce95818dc40d4cf6b3962b02c5544fc09` 安全快进至 `8d32f818536e7a237d366afe0be321c69a7700e7`；映射 `c806fd9… → 8a3778408c844512bc5146951162062481f08b0c`、`e0c3631… → 8d32f818…`，range-diff 两项 `=`、无冲突。server 与受审 e0 逐字节一致，docs 保留目标元数据，旧迁移／0009／原 messages/create_test 未改，main ref 仍 `01c74a7…`。
- I01 新跑 **PASS**：Go1.27.3／Node22.23.2／npm10.9.8、storage／changes 普通、原 messages 并发及排队取消五轮 race（22.121 秒命令耗时）、全服 vet/build、快进后原根 build、gofmt/diff。受控 Web 输入同版，真实 dist 按 SHA256 来源清单复制；F02 同 e0 的全服普通／四包 race 和 A02 独立初始化／取消检查证据准确复用，未冒称新跑。证据 `/tmp/relaynest-storage-busy-i01-20261027/commands.jsonl`、`integration-evidence.log`、`web-dist-provenance.json`、`root-web-dist-hashes.json`、`target-fast-forward.log` 及各命令日志；这些为本环境临时证据，不冒称随 Git 保存。
- 归档与边界：总调度批准仅此独立修复叶 done，业务／候选／目标 index／重型锁已释放；不代表 ADMIN 完成。**ADMIN-A03-B01 保持 OPEN**，准确 ADMIN＋fix 全服 race 仍 **NOT_RUN**，须对完整 ADMIN 三提交与已审修复的真实组合再验证、续独审及精确集成。未 push、部署、发布或创建完成标记；0009 仍永久占用，未新增迁移。

## Web 管理端

<a id="w1"></a>

### W1 登录壳层与设备会话管理

- 原 ID：`P07-01`、`P07-06` 及准备/修复/PLAN/A/I；工程 `P03-02` 归 E1。
- 已完成登录/错误壳层、设备分页、精确撤销、自撤退出及迟到响应隔离。消息正文、附件和历史浏览仍不属于 Web 管理端。
- 最终组合实际通过 npm/build、客户端 8/8、五个 Go 相关包及真实生产 embed 服务＋本地 HTTPS＋Chromium 双会话；缺失/错误 CSRF 均真实403且无撤销副作用，撤销隔离/401/自撤/重新登录有实际断言。
- opaque 分页、恶意元数据文本化、错误/重试、延迟响应等 fixture 独立证明客户端处理；本地证书和忽略证书错误不等于公网 TLS 部署验收。两卡必跑矩阵无遗留 NOT_RUN；其余管理页面和正式部署另行完成。

## MoviePilot

<a id="mp1"></a>

### MP1 最小 PoC 用户验收例外

- 原 ID：`P01-01～P01-06`。P1 PASS/G-MP pass 来自 2026-10-21 用户缩小验收范围并接受剩余缺口，未执行独立审计/独立集成，不补造 A/I SHA。
- v3.1.5 主进程固定文本入站、原生定向回复、媒体/种子合成候选；临时 HTTP 桩收发、消息 ID、手动更新、图片/文件夹具；方案 A 后两组命名空间身份的按钮导航、错误身份/失效请求等28项通过。均不代表正式 Hub/Android 端到端或真实业务操作通过。
- 已批准方案 A 核心文件补丁＋助手显式应用入口，仅固定 v3.1.5 指纹；完整上下文恢复、自动编辑为可选。v3.0.0 最低目标/拟议 `>=3.0.0` 声明未经对应运行验证，不能声称全版本兼容。
- 接受但仍 NOT_RUN/限定验证：完整配置绑定与主动广播/用户路由、重启/重建/升级/重放恢复、浏览器实际点击、独立实例及原独审要求；正式插件、AC-07、P10/P11、正式发布均未通过。
- 正式插件须可信 Bot/user/request 绑定；跨 Bot 隔离证据依赖命名空间 userid，未证明宿主按 source 原生隔离。业务权限与危险确认归 MP，Hub 只传输/存储/路由，不执行命令或 MCP 工具。
- 现网恢复快照镜像可能与 Compose 声明不同，重建/升级可覆盖补丁；实际镜像/RepoDigest/恢复仍需补证。原 `/netdisk` 挂载曾经用户授权移除，不擅自恢复，相关功能另配有效路径。

## 工程、契约与已批准决定

<a id="e1"></a>

### E1 工程、CI 与 API 契约

- 原 ID：`P03-01～P03-12` 及所有拆分/修复/A/I。完成 Go/健康入口、Web/内嵌、Android 原生最小工程、固定工具链、SQLite 连接/迁移、Go/Web/Android/插件静态检查 CI 配置。
- G-API 原冻结为39 operations/115 schemas/1229 cases；新 G1 扩展为40/118/2455（见 E2）。契约覆盖认证/Bot、消息/附件、投递/失败管理、同步/bootstrap/已读/WS、设备及多通道推送、全局配置/健康投影。
- 配置与局部实际检查完成不推定远端 CI 成功、正式插件构建、APK 签名、镜像发布或安装升级通过；契约通过不代表运行时或跨端端到端。

<a id="e2"></a>

### E2 必须沿用的用户决定与冻结边界

- 原 ID：`PUSH-PLAN-01/PUSH-DECISION-01`、`P03-07-S03-S01-POLICY01`、`P08-04-D01-PLAN01/CONTRACT-PLAN01/C01` 及 MIXED/审计/集成。以下已经批准，不因归档删旧报告而重新提问。
- 全局设置：正整数天/正整数字节，0不代表立即清理/禁用/无限；默认30天、图片10MiB、文件50MiB，无新增业务上限。三配置字段采用正 SQLite int64 规范十进制字符串（1～9223372036854775807）；专用全局 CAS 版本为 JSON integer 1～9007199254740991（2^53−1），不得与字符串配置或公共 Version/设备版本混淆。匹配且版本耗尽拒写、不回绕；年龄比较防溢出，后续清理保留保护引用/快照等契约边界。
- 推送：OPPO 必填使用固定通用 title/body；显式可选 vendor，超时/未知结果不自动跨通道 fallback FCM，切换只影响未来事件；用户接受在途残余及离线 DND/已读/退出撤回限制。资格、签名、分类额度、错误码/真机 PoC 仍未验证，G-OPPO pending。
- G1 是诚实替换/误连的有限保证；用户接受探测后瞬间替换时旧 Bearer 字节仍可能到达的窗口，不承诺 G2 零泄露。正常重启/原库无回滚停机迁移保留 UUID；恢复/克隆须人工停服换 ID并撤销全部管理员 session/推送映射，清库新 ID，已初始化 ID 缺失/损坏失败关闭。
- 已批准无凭据公开 `GET /api/v1/instance` 只返回随机 UUID；Android 记录期望实例 ID并使用专用 header/会话字段；旧密文记录重登、旧 Android 拒绝、旧服务不降级。每个受保护请求先额外探测，5秒超时，共享20rps/突发40，失败关闭；Web 仍按原规则。运行时和组合尚未通过。
- 两附件 upload/download 是狭窄 Bot-first 例外：摘要匹配 Bot 后任何授权/代次拒绝不得 fallback Admin；依赖故障503；仅确定 NoMatch 后检查实例 ID，再走 Android 管理员认证。无效凭据时也接受缺 ID400/错 ID409优先于原401的代价，不扩其它端点；资源权限/文件打开仍须后续真正路由验收。

<a id="e3"></a>

### E3 准备与过程记录

- 原 ID：`P00-01/P00-02`、`P00-03/05` 已完成的 A/I、WORKSPACE*、SCHEDULER-SNAPSHOT*；只说明需求/Git/环境资料核对、工作区和调度证据，不将仍缺 NAS/OPPO 等条件的父卡算完成。
- 用户已授权持续实现、测试、独审、修复、集成及交付准备；外部条件不足只阻相关分支。现网部署、MP重建升级、破坏性操作、公开发布、远端推送和 Release 仍待具体结果准备后的最后确认。
- G-GIT/G-API/G-MP/G-FCM/G-INSTANCE-CONTRACT 已按上述范围通过；G-OPPO/G-INSTANCE-RUNTIME/G-INSTANCE-COMPOSITION/G-CORE/G-DEPLOY/G-SHELF/G-SHELF-API 均 pending。部署交付和 RelayShelf 尚无正式完成功能，RelayShelf 后置门槛不因准备报告自动放行。

## 关键提交索引

下列为最终实施或证据报告源、独审源、精确集成组合与最终集成报告提交（部分纯收口卡的“实施源”仅为报告）。未写的过程 SHA 从基线台账恢复；`—` 表示该字段未记录或不适用，不推定已通过。历史报告路径均相对 `docs/tasks/reports/`，通过前述 `git show` 查阅。

| 原任务 | 实现源 | 独审源 | 集成组合 | 最终 I 报告提交 |
| --- | --- | --- | --- | --- |
| P00-01 | `522dbeb3cb96c645a1351e05452a01df54100093` | `52635d825cbe00e0eb025ab1875de045b9211409` | `b0b614723d8055247d77fbeaa9c6dd88214d3c27` | `b0b614723d8055247d77fbeaa9c6dd88214d3c27` |
| P00-02 | `a2f3c3dc6cff6bcfe2265118c7e1a7019f01365c` | `376ee351453668348f75c93501fc5081ea5c4f4e` | `1f0ea95004062e9ef3b88ea9ce6d4cbb30695484` | `1f0ea95004062e9ef3b88ea9ce6d4cbb30695484` |
| P00-04 | `7cf1a3148ffebfd05cc8f4d6d5a6ae9b5ed6fadc` | `7812dccb9801c53ef844360c6ecddc6a0e5811f9` | `fefad6f1ced17aa7a2dd328f8683d1ae8397f1fe` | `fefad6f1ced17aa7a2dd328f8683d1ae8397f1fe` |
| P02-01 | `c49acb359e9170fc9263c2649322d62261092d95` | `4048045c66526f0fccbc3efdbdb334f0e3a5cc91` | `dd6d6fc15eb1f5a71b0cf39752907751ed97198a` | `dd6d6fc15eb1f5a71b0cf39752907751ed97198a` |
| P02-02 | `bcc42505b8a0335d328d1ac42f1f647c5e7759c4` | `d3187f8dd6ec3d3338d19fb438ec4ea9fdee08ae` | `6bdd6ef540494321ed6e22015212f9a93bfd6bdc` | `6bdd6ef540494321ed6e22015212f9a93bfd6bdc` |
| P02-03 | `f65efaed77a3f772c2bd6e12e0f03680b7a3f945` | `977c1977703737b4295b0dc6fd7794188c1f7baf` | `95f50154daa92597574c6ce63d163ab0b07cd194` | `95f50154daa92597574c6ce63d163ab0b07cd194` |
| P02-04 | `879aaebffb1e312db78caac1978e4fea23b80d4d` | `1cfc7516187f9a3b7c0c42f792f745d699576ec8` | `0eab5acb1e93ba0915bc0dcca9271ab4a124d959` | `0eab5acb1e93ba0915bc0dcca9271ab4a124d959` |
| P03-01 | `47d33bb7f5ca356947f4107d81429b9ace66dac1` | `eb56c4acc36b0a3986bd17858632923c5c798261` | `f3dbe794f8afe32573435a94e363eb7a3761924f` | `f3dbe794f8afe32573435a94e363eb7a3761924f` |
| P03-02 | `6e9171946099457aad4f7696700927bd5751e77d` | `cca49567239bf034a1f65209019303a602797811` | `6d4ac84dd8d1b083b927505a9a97c975aa3e9d44` | `6d4ac84dd8d1b083b927505a9a97c975aa3e9d44` |
| P03-03 | `e002339fb085a6f5f8ac349c6b077d5751b3708f` | `8239b4d609703f917ba81c71d07f0b5eff9b80e1` | `1e16cffa3c6d15d8f01ab8e8bc501a0c0232aef6` | `1e16cffa3c6d15d8f01ab8e8bc501a0c0232aef6` |
| P03-04 | `6eac6bfdd0f2e27e899269e07b4a7746dc0d8958` | `2e9e093444d50a214861426e5c0310787b53403a` | `30efad4f16234c07f42a2dd6e746d9d980b7dadf` | `30efad4f16234c07f42a2dd6e746d9d980b7dadf` |
| P03-05 | `5d170b84ef77cb6560d594341fa35bb0dab5bd7c` | `b7b02d023da5c25d0031d39bc833ec265a77b761` | `38d52c3bae74010167d4defe0e681e843d497fc8` | `38d52c3bae74010167d4defe0e681e843d497fc8` |
| P03-06 | `fd0f3cb9526637fa25d472ad801f995d88f0d426` | `2d2c633366742011a96e9cac5297c7b4bb30eafd` | `57ff401f95d9f747ec85938cbb521190463ae3c2` | `57ff401f95d9f747ec85938cbb521190463ae3c2` |
| P03-07 | `7ec2ae3494088b4819830b46c23fd88d7e81560b` | `34425b492601045ffcd109a88246a73a1094542f` | `981692f49663075aa553ce45aa645e75197b15ea` | `375a270c74080d9eb3ea8e975d8c79814afc091f` |
| P03-08 | `e2d56baedad2d28c6d4b8fda0d018df49c274062` | `9b3cef6ec9cc4bfc3895fe7474090fad8ab1f731` | `abd45c273b4d6feb0cd1c701ce742036dd04add3` | `abd45c273b4d6feb0cd1c701ce742036dd04add3` |
| P03-09 | `ae32f8753a5f4cff109f1fb28b739857b3655e62` | `282152c29994b67aa31e3ec046e0df0768d1a8f7` | `4cf1304839d7dfdb55dd211f7f4deabb13846349` | `4cf1304839d7dfdb55dd211f7f4deabb13846349` |
| P03-10 | `500c6e2259ffb0a99163114870b2bc0484b91547` | `0ece340fce0dcdad19a13eccf642fd6de2401e7c` | `701bbc60c5665d05d6b6ebbf167038e291a39096` | `701bbc60c5665d05d6b6ebbf167038e291a39096` |
| P03-11 | `8469c2f3caa5ff403ca61bce3116babfe633bf90` | `31a451964020b7688b65531a27ebcce0b8800c31` | `8536dfddfbff2bc6fb3e126b66f380ba7ef88a7b` | `8536dfddfbff2bc6fb3e126b66f380ba7ef88a7b` |
| P03-12 | `e3fa7343369a7af6599146e134efbbdd5a4a393a` | `ab6530f93c300ca870d6aeb8cb4bdd79c226b951` | `8e09e85e25995b011ba147f9a44761e89148e772` | `8e09e85e25995b011ba147f9a44761e89148e772` |
| P04-01 | `9af6a70c96b106b4a8f8d48e0913a80aa5bed68d` | `f4767d6cc640d3a5859ad0371860c8dcadcf8081` | `3ba2b496ae21737d15c8442a1fd56429513da1d4` | `d3acec6779b6ce5377737575cdf51f550d41b719` |
| P04-02 | `6cec43b707b88908712561718988599bc5404988` | `8ead8349afad11e1278dcb0506cf90f3a512466b` | `2f425e0d2c3edcf0f18a279f56b8993b6dab0a58` | `e831cf9d9810206f96b21ea16f2dabe0b3df3afe` |
| P04-03 | `ab43c24b784bd968a6bf4927625c3182432c4276` | `38b14f8163dc5076cf8f9546d461fea6ac3e9caa` | `04831d870582a928ce7f8f5d7e4827ae6e9134e4` | `a4df025ae42e29ec45a825a025ade77aef41d96a` |
| P04-04 | `c5be4acaf1442de7e9eabf298dd71bc217d5e708` | `8b3ff1cfa0325d9e4ec36a57e3ecb22d03404f76` | `9b65b73b2a499add367c613ae3e318138b111a3f` | `932d4dddbdee3216ed1b13b7f708f8481c205509` |
| P04-05 | `a950f489dd3b7d5e2f2a81c74cb0b1b379f05129` | `15649ac7fb03f24bb013888fe467c76428ca7f0d` | `d78711faa56dd01b7e10cf4eec3b70490f2d04e9` | `005994a049635163e879c069ecf02526c8ddbe1f` |
| P04-06 | `30affdd6ecb7559da55bbf9aead80e10c3e4debd` | `fdd735b9dd243d1631efcb6147546dc55d1d14b0` | `2a39fec92342aceee82ba02e0377ca366de8dad8` | `cab32c3a1d33c6ae93ce8fbd9a38b8bb55f40985` |
| P04-07 | `1dfac3f222d46083684a0f5b333dded4f7baee0d` | `c8ad4b7dd7234181f287dd386e9e7f93e544a6cd` | `e6de9e5e039a85e57ec8020ee03d7f24c6f59563` | `d2b58adb83ce69311e94fe9ebafd2843ec390c49` |
| P05-01 | `9720ddc2550c2b142588ed33baf5481fef05c206` | `45046b5d4095b629e2b645caaf4a52fe423487f7` | `b09a19d910cb84001987cf6afda6da3738f7badc` | `ed488e543d892f2353fbc95f72a2f0af98ba9a8d` |
| P05-02-S01 | `f9c7b7a823051f5bd1f9882310d0e460ff96dd98` | `324294257fc1f26404a629b042512ef5925bfc07` | `b7e59b64116fdd4c4f7b06009473341df38471e9` | `e674801f3640508327e102bda37805b687c3108d` |
| P07-01 | `20ba9f2bf5b5fa656bc89a6efa6a465b1f10b533` | `41b93d23352b99acd685fa63e592e1af612e885e` | `9ada1b5ecc4a0776a47ce996b22a2ad8d4035c00` | `22a3e47f49524f60fc1ae4477080af1912701b27` |
| P07-06 | `5625a6b38f22aa529919d6dba944dc2b71e49b2b` | `a09084e40f1f815d15c61003d0400673f3722e12` | `1117a5cb0fedd9187e1be71b42038cc59540559a` | `02d0aa6744b8162a911f4d709245c70d6d2266d9` |
| P08-04-S01 | `24f0ba44833fa7feba674bf58bfac1ee64dcbcb5` | `604e43160cbb63b65f3ce619df121114a6b2f146` | `e7e55c3c1c7441483bdeff6ba66672a61d72e549` | `319f262cc920af1aef276f9f475ad9e5d294e126` |
| P08-04-S02 | `12d4e735e9c5147e70c49b7ea25d00ac6f80ce90` | `386c954c79d4dd434cb83b4d5448e5d4c077a2f1` | `0262f117d4e0bdc948e192e160c4e3881607a2b6` | `06b0bf09f0f661bf4787e8af2349634efa84d24a` |
| P08-04-S03 | `b59c19b779a666ac24811ecc8538d07339e7a4c5` | `e0c5cab7e4a087ed964bdf622d95fcd060680477` | `e1849386510d2a060b909af1bd19be5881eb0d61` | `ae8f6056c63e163179dac8f90093da1ba056b549` |
| P08-04-D01-C01 | `09deddba6ace42261c227b57254f8ae9ebbd1a2a` | `8d012c9941f771a64236e38c735d09d379936259` | `f72a11c147269b4c81f0e319998292a3228a83f9` | `20d86d7699d2a9878c81a56ed5801debb0c625e8` |
| P08-04-D01-S01 | `f546060db41aabc4d4ff1fbfb1e7c91787d8ea19` | `85609afaf027bad1e930bf491b02128ac2d6a5ae` | `eea5094237fdc24fd967948795a3cdf1a0d26888` | `07779de822d1aa28ceeb324c1225499fcf20c9c4` |
| P08-04-D01-M01 | `4a4ba2343a4e95baf77af62b60e56e1e28ffcf9f` | `a0df018110094a180c431e5ab456624a260edce4` | `173c43f1ede7cf121f6eb603767f5704811bbf68` | `71b6e5ea3fd08b86f7af6848068a0dd975150161` |
| P08-04-D01-B01-BOT01 | `63b673a4ba3fe66bac68b05909b60e856c1a8d52` | `020ff797ec189bbfa7e4657ea8fb0dc1963a7945` | `529efe0814660553796da9d3f55f7b3e62e6e719` | `4698f39dbe7e78f86cba209a0a44a750c9fdd8db` |
| PUSH-PLAN-01 | `14d66a2f4872c893ff97e7d83a1df2cb0077c158` | `43969969a12c7618be4c693ba44491a893a97b22` | `428014852a00275c27362e9caa5e748e1e8338f1` | `428014852a00275c27362e9caa5e748e1e8338f1` |
| PUSH-DECISION-01 | `489adb8855c4af8f717e7da3fc9e21418fe3d34c` | `488580cab57e8504719eb4733de9ea88731bf3db` | `f9d48955de030a735ac1c3e0602678fdae0cde75` | `f9d48955de030a735ac1c3e0602678fdae0cde75` |
| P03-07-S03-S01-POLICY01 | `7c2d995c8f045ecb31f87065b1a7e90eaf5f0e86` | `5b850f156785805f25d07e4d28e04cb853b05715` | `b86ad2eba8ca5c9afc76ba65159de974ed968339` | `b86ad2eba8ca5c9afc76ba65159de974ed968339` |

P1仅用户验收：原六个技术源提交按序为 `61d9dcc6b45b20b755cab6dc188b88b5fe1fd1ad`、`8e8afa6b6bbf66462aac9f4df11a1c22d08137cf`、`213f83288d4784473c0c5f5dacf431517165f6ee`、`498d5b7b846cac466e765b1d18e6dd935d62a2d4`、`061c0b8de92d78936ff90afc3b278ab1466cc46a`、`5d31b6fcebd0a02f28a9b9792ebdd6b54696abc4`；用户验收文档提交 `b058474ce4e7e0b4321fe8dec174c23601ce30a7`；未独审、无独立集成结果。

## 历史未闭环元数据

`RECOVERY-01`、`WORKSPACE-01`、`P01-01-PLAN01` 及 `P03-03-A01`、`P03-05-A01`、`P03-08-A01` 为原台账6条未闭环过程元数据，不计入上述331条完成记录，也不生成新功能待办或改记验证通过。后三项旧审计结论已有后续修复／独审／集成链收口覆盖；仍有效的真机、构建与环境限制已交相应模块及交付验收。历史状态按原基线查阅。
