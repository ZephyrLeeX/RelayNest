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
