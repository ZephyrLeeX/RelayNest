# Go 后端剩余任务

## 环境迁移交接（2026-10-26）

用户已明确授权将代码和状态文档备份到 GitHub，随后更换执行环境。本节记录恢复入口与断点；任务状态仍以下方条目及 COMPLETED 为准，不以备份提交代替实施、审计或集成验收。

### 恢复入口

仓库：`https://github.com/ZephyrLeeX/RelayNest`。只推送下列恢复命名空间，不覆盖远端 main/work、不 force、不部署或 Release。实际上传成功以会话最终逐 ref 远端 SHA 核验为准。

| GitHub 恢复分支 | 保存内容／固定代码 |
| --- | --- |
| `recovery/backend-20261026-state` | 从原目标 `a97cfcee014298c64e8468a7b9eb1d007b635e37` 保存的本次交接快照；本提交含 BACKEND／PROGRESS 更新，分支远端 HEAD 即恢复入口 |
| `recovery/backend-20261026-admin-wip` | `8ab66c847bf1a6e0c86592f90994679b6685c7e5`；管理员消息创建的五文件 WIP，未完成完整验证、独审和集成 |
| `recovery/backend-20261026-push-i01` | `47c08a7a3fd7712379d2ef4ba6e86ad897daacb4`；推送存储隔离集成候选，尚未快进原目标或取得 I01 最终 PASS |

原目标功能代码仍为已闭环的现有后端实例绑定等结果，新增迁移只到 0008。候选包含 `77025cfe0a3317a0d0df5a2b97ff1be786a3104b` 和 `47c08a7a3fd7712379d2ef4ba6e86ad897daacb4`；两者分别来自已 A02 PASS 的源 `20b9fffbe8063e25cee60456dc96e53f594866cb` 和 `003c990c08ef606b838ae2715bf1d33d19e7fa00`。源提交身份也由下表恢复分支保留，不能只保存 cherry-pick 后 SHA。

### 新环境接续

在干净的新克隆中获取恢复分支，切到交接快照，并分别恢复两个独立工作树；不要直接在远端默认 main 上重新开发。

```sh
git fetch origin 'refs/heads/recovery/*:refs/remotes/origin/recovery/*'
git switch -c resume/backend-20261026 origin/recovery/backend-20261026-state
git worktree add -b resume/admin-wip ../RelayNest-admin-wip origin/recovery/backend-20261026-admin-wip
git worktree add -b resume/push-i01 ../RelayNest-push-i01 origin/recovery/backend-20261026-push-i01
```

先读取 AGENTS、WORKFLOW 和本文件，核实真实 HEAD、工作树、源提交及排他锁。旧 Agent ID 只作历史证据，当前没有在途执行承诺。既有已完成范围见 COMPLETED，不能重复实现。

1. 优先恢复 BE-10-01-STORAGE 的精确集成：源链已 A02 PASS；候选已无冲突应用两提交，但原目标仍在 a97cfcee014298c64e8468a7b9eb1d007b635e37。中断前仅明确报告锁版 Web 和受影响普通检查 PASS，race／后续组合及目标复核未取得最终结果；新环境必须完成实际必要检查，不得依据候选存在标 done。交接快照是新增元数据提交，与候选分叉；先核范围并保护交接文档，不盲目快进或重复 cherry-pick。
2. BE-02-ADMIN 从已保存 WIP 继续补实现／验证，不从头重做。五文件为 `server/cmd/relaynest/{main.go,main_test.go}`、`server/internal/httpapi/{messages.go,router.go}`、`server/internal/messages/create.go`，合计 376 插入／6 删除；历史仅报告轻量 messages/idempotency PASS，httpapi 曾缺 embed 资源失败后补环境，完整检查无最终交接。全服 race、独立固定 SHA 审计、必要修复和精确集成都未完成。WIP 提交只为保存代码，不能因有提交直接标 review/done；后续审计须覆盖整个 WIP 到最终实现链。
3. 0009 已排他分配给推送存储包，并在源及候选中存在；主恢复基线尚未合入它。恢复闭环前保留该编号，不给其他任务重复分配；后续 0010 仍须实际核号。公共 auth/router/main 和迁移继续串行协调。
4. 固定 Go 1.27.3、Node 22.23.2／npm 10.9.8；旧 `/tmp` 工具、依赖缓存和 Web dist 不随 Git 备份。按仓库锁版要求重新准备，先构建 Web embed 再跑服务端检查。重型 full/cmd/race 同时只跑一项，保留历史超时／SQLite_BUSY 失败及复验事实，不改原断言规避。
5. 全局 G-INSTANCE-RUNTIME／COMPOSITION 含 Android／跨端，仍 pending；BE-01 仅既定现有后端范围已完成。真实附件 HTTP／WS、真机、推送真实注册 HTTP／平台发送、部署等仍按各条目未完成边界处理。本次备份未新跑 Go 测试、不变更任何功能验收结论。

历史 `/tmp` 日志及审计 overlay 未作为源码上传；历史检查摘要和精确源码 SHA 已在本文件／COMPLETED 保存。新环境需要的验证应针对恢复后的真实组合重现，不假称本地临时路径仍存在。

### 原工作分支映射

所有现有本地 codex 分支均以完整 SHA 显式备份到下列恢复 refs，便于按原独审／实施提交追溯：

| 原本地分支 | 完整 SHA | GitHub 恢复分支 |
| --- | --- | --- |
| `codex/be01-auth-f01-20261026` | `39529d3d3f0f5b070af2237d6361025d436de20e` | `recovery/backend-20261026-local/codex/be01-auth-f01-20261026` |
| `codex/be01-auth-i01-20261026` | `1aa0aa9e0c390a46a7293cae56410e0baae8ce71` | `recovery/backend-20261026-local/codex/be01-auth-i01-20261026` |
| `codex/be01-http-20261026` | `891287cf143e4d300c353e24233979077e84e64b` | `recovery/backend-20261026-local/codex/be01-http-20261026` |
| `codex/be01-http-f01-20261026` | `d8706747be5dbd98e91fe4892ab2f3f260e3b35c` | `recovery/backend-20261026-local/codex/be01-http-f01-20261026` |
| `codex/be01-http-i01-20261026` | `cf2c8b49b4b5338e58e4b9d6e9698e5e648bdc31` | `recovery/backend-20261026-local/codex/be01-http-i01-20261026` |
| `codex/be02-admin-20261026` | `8ab66c847bf1a6e0c86592f90994679b6685c7e5` | `recovery/backend-20261026-local/codex/be02-admin-20261026` |
| `codex/be02-bot-20261026` | `907c2a4dbf3a8774bcd438fb22b2381def7394a5` | `recovery/backend-20261026-local/codex/be02-bot-20261026` |
| `codex/be05-storage-20261026` | `064807f9e5e126f48c9360d4d02a40fc209f790e` | `recovery/backend-20261026-local/codex/be05-storage-20261026` |
| `codex/be10-registration-20261026` | `20b9fffbe8063e25cee60456dc96e53f594866cb` | `recovery/backend-20261026-local/codex/be10-registration-20261026` |
| `codex/be10-registration-f01-20261026` | `003c990c08ef606b838ae2715bf1d33d19e7fa00` | `recovery/backend-20261026-local/codex/be10-registration-f01-20261026` |
| `codex/be10-storage-i01-20261026` | `47c08a7a3fd7712379d2ef4ba6e86ad897daacb4` | `recovery/backend-20261026-local/codex/be10-storage-i01-20261026` |

历史 AUTH `d894f14175249bbd17609a0eb3521462577151c8` 同时由 AUTH F01 的祖先链保留；其余所有 detached 代码均由上述提交或恢复入口可达。只保存源码、必要测试和版本化文档；不上传依赖、构建产物、临时数据库、密钥或原始敏感日志。


推送分支依赖说明：原 BE-10-03 父依赖字段保留；下游只做框架／FCM 的准备或验证时，执行包必须明确消费 [BE-10-03-FCM](BACKEND.md#be-10-03-fcm) 精确独审已集成子结果，OPPO 外部阻塞不传播至 FCM。全范围最终验收仍须两通道分别有证据，OPPO 缺条件保留 blocked／NOT_RUN，不能以 FCM 代验或将父项标完成。

本文件是后端唯一剩余任务入口；BE-01～BE-10 是功能导航，其中 BE-01、BE-02 和 BE-10-03 同时承担旧父任务收口，不能整组派给一次 worker。仅下列 leaf 可独立实施；依赖须完成独立审计和精确集成后才满足。未更新条目保留重组基线 `086fa51d79d7726545a9abbab8feebb25770034e` 的快照；已恢复任务以各条目最新调度证据为准，不从旧快照推定活跃 worker。

需求与接口以 [PRD](../PRD.md)、[架构](../ARCHITECTURE.md)、[技术设计](../TECHNICAL_DESIGN.md) 和 [OpenAPI](../../server/api/openapi.yaml) 为准；这里只保留实施范围和验收，不建立新合同。工程检查、独立提交与 Git 规则见 [CONTRIBUTING](../CONTRIBUTING.md)。Bot Hub 只传输、存储和路由，不执行业务命令/MCP，不增加 Web 聊天/历史或正文推送，不修改 RelayShelf 后端/数据库。

已完成基础统一见 [COMPLETED](COMPLETED.md)：认证/设备、Bot 基础事务、消息事务仓储、幂等存储、实例身份/离线维护和 Bot 内部身份发现。尤其 P05-02-S01 的最终收口为 `e674801f3640508327e102bda37805b687c3108d`，不等于正式消息 HTTP 完成。

每个 leaf 默认只读相关设计局部、列出的主要文件及对应测试；拟定路径不存在时先核对当前等价入口，不凭空新建模块。默认 3～5 主要实现文件；迁移须串行核号。router/main、共用 auth 接口与迁移不能并行覆盖。每项交付记录基线/最终 SHA、真实命令/环境/结果、未测项及遗留问题，独审与集成后才可完成；不以桩、源码推断或局部测试代替组合验收。

共同检查：代码任务在 `server` 运行受影响包 `go test`、相关 `go test -race`、`go vet` 和 gofmt；集成公共 auth/router/main 时补 server build/全包检查及受影响路由非回归。固定 Go 1.27.3，不升级工具链。下文“检查”还列各项专有必验行为；环境不足须明确 NOT_RUN/blocked。测试任务只补交叉回归，发现实现缺陷应形成有限修复项，不能借测试整组重写。

依赖写作 已完成依赖链接 的项均指 [已完成证据锚](COMPLETED.md)，不重新实施旧任务。BE-01/BE-02/BE-10-03 仅在其 leaf 和本节父验收全部满足后供消费者依赖，其他 BE 组号只导航。

## BE-01 完整实例绑定

<a id="be-01"></a>

已完成父范围：[BE-01 现有后端 probe＋八路由的完整验收、G1 窗口与未来路由约束](COMPLETED.md#g5-be01)。全局 G-INSTANCE-RUNTIME／G-INSTANCE-COMPOSITION 仍 pending；Android／真机／未来附件与 WS／代理验收不在本次完成范围。

<a id="be-01-auth"></a>

已完成子项：[BE-01-AUTH Android 实例入口与事务绑定的范围、验收与证据](COMPLETED.md#g5-be01-auth)。

<a id="be-01-http"></a>

### BE-01-HTTP 实例探测与真实路由同快照组合

已完成子项：[范围、完整验收、修复历史与独审／精确集成证据](COMPLETED.md#g5-be01-http)。既定现有后端父范围已完成，见 [BE-01](COMPLETED.md#g5-be01)；全局跨端门槛仍 pending。

<a id="be-01-compose"></a>

已完成子项：[BE-01-COMPOSE 完整验收及 R01／A01／I01 证据](COMPLETED.md#g5-be01-compose)。

## BE-02 正式幂等消息创建与回执

<a id="be-02"></a>

### BE-02 消息幂等创建和接收回执

- 子项证据：[Bot 有限文本 HTTP 已完成独审与精确集成](COMPLETED.md#g5-be02-bot)；ADMIN 尚未开始，父项原组合验收仍未闭环。

- 旧ID：`P05-02`；类型：父收口，不整组派发；状态：进行中（父收口未完成）。
- 依赖：[已完成基线 P05-01](COMPLETED.md#g2)、[已完成基线 P05-02-PLAN01](COMPLETED.md#g3)、[已完成基线 P05-02-S01](COMPLETED.md#g3)、[已完成 BE-02-BOT](COMPLETED.md#g5-be02-bot)、`BE-02-ADMIN`。
- 允许范围：`server/internal/messages/**`、`server/internal/idempotency/**`、`server/migrations/**`，及对应测试。
- 门槛：`G-GIT`；条件未实证不记通过。

父卡保留四项原验收：提交后才回执；丢失响应重试不重复；同键异内容409；Token重置不更换稳定身份作用域。存储基础虽已完成，必须组合真实 Bot 与管理员 HTTP+SQLite 复验。

当前有限阶段仅 plain/markdown 文本且无附件/引用/按钮；其他输入显式拒绝并记录未实现，不能吞字段或声称完整 CreateMessageRequest 通过。后续引用、附件、按钮、标准Webhook的阶段门槛分别由 BE-04/05 保留。回执仅代表持久化，不代表应用ACK、已读或业务成功。

<a id="be-02-bot"></a>

已完成子项：[BE-02-BOT Bot 正式创建 HTTP 的范围、验收与证据](COMPLETED.md#g5-be02-bot)。

<a id="be-02-admin"></a>

### BE-02-ADMIN Android管理员正式创建HTTP

- 调度（2026-10-26）：owner Backend Orchestrator，固定基线 `a97cfcee014298c64e8468a7b9eb1d007b635e37` / 目标 `work`；BE-02-BOT `907c2a4dbf3a8774bcd438fb22b2381def7394a5` 与 PLAN01 精确集成 `50be58eaaba6c8918dbd7642d9ef06125f0dda07` 已验证祖先，完整 BE-01 既定现有后端父范围经 R01／A01／I01 归档，完成快照 `a97cfcee…`，业务组合 `cf2c8b49…`。
- 派发准备：Backend Implementer，隔离 `/workspace/RelayNest-be02-admin` / `codex/be02-admin-20261026`；仅 httpapi/messages.go／router.go／cmd main.go／必要 messages/create.go 四主文件与相关指定测试，独占本树/index、messages HTTP/router/main 接线，不改 auth／SQL／API／Bot 分支。可与只改新 devices／0009 的推送集成并行；重型检查须等待推送 I01 释放。
- Agent `/root/admin_message_implement` 已派发，开始隔离准备及实施，尚无新增独审／集成结论。
- 环境迁移备份：用户已授权保存代码，现有五文件已独立 WIP 提交 `8ab66c847bf1a6e0c86592f90994679b6685c7e5`，远端入口 `recovery/backend-20261026-admin-wip`；不是功能完成提交，无新测试／审计结果。原 worker 不再运行，从现有代码继续，见本文件开头交接。

- 旧ID：`P05-02-S03`；类型：有限 leaf；状态：`blocked`（环境迁移暂停，原 running；WIP 已保存但未完整验证／独审／集成，恢复后接续 running）。
- 依赖：[已完成 BE-02-BOT](COMPLETED.md#g5-be02-bot)、[已完成基线 P05-02-PLAN01](COMPLETED.md#g3)、[已完成 BE-01 现有后端父范围](COMPLETED.md#g5-be01)。
- 门槛：`G-GIT`、`G-API`；条件未实证不记通过。

允许 httpapi/messages.go、router.go、cmd/relaynest/main.go、必要 messages/create.go 四主文件及测试；auth只消费 BE-01 已集成接口。正式 POST /api/v1/client/bots/{id}/messages 仅 Android AdminBearer，固定sender=administrator、Bot启用/receive。S01幂等/消息/投递/changes/待办必须处于 B01 WithAuthorizedWrite 的同一受控事务，先ID再读Bearer，提交前复核ID/会话/代次；不能以M01/BOT01/桩middleware代替完整绑定。

commit后201 ClientMessageReceipt含原message/delivery_id，no-store；同有效管理员换会话仍同作用域，重放前核当前授权。错误同Bot分支并保实例错误，接受G1瞬切字节窗口。检查真实HTTP+SQLite丢响应重试/重新登录同键不增消息/投递/不可变事件/序号/changes/job，异内容409；任一步/commit失败全回滚；缺/重复/畸形/错ID无副作用，事务中ID/会话/代次失效拒绝，probe A切B不认证/不执行；Web/Bot误用及管理/设备/health非回归。必跑httpapi/messages/idempotency/auth/cmd测试、相关race、go vet ./...、go test -race -count=1 ./...。

## BE-03 Bot生命周期和身份权限收口

<a id="be-03-02"></a>

### BE-03-02 身份权限矩阵回归

- 完成追加依赖（不改变上述开工依赖）：[BE-03-01](BACKEND.md#be-03-01)、[BE-03-03](BACKEND.md#be-03-03)；基础准备可先进行，真实全范围收口必须满足。

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

- 旧ID：`P04-08`；类型：有限 leaf；状态：未开始。
- 依赖：[已完成基线 P04-03](COMPLETED.md#g1)、[已完成基线 P04-04](COMPLETED.md#g1)、[已完成基线 P04-05](COMPLETED.md#g1)、[已完成基线 P04-07](COMPLETED.md#g1)。
- 允许范围：`server/internal/auth/*test*`、`server/internal/bots/*test*`、`server/internal/devices/*test*`，及对应测试。
- 门槛：`G-GIT`；条件未实证不记通过。

只补管理员/Bot/Webhook/send/receive权限、重置/撤销矩阵和AC-02证据。旧前置均为已完成基础；正式reset/delete、租约和队列组合验收还需BE-03-01与BE-03-03真实完成。只改auth/bots/devices对应测试；发现实现缺陷另列有限修复项，不能扩成业务实现。

<a id="be-03-03"></a>

### BE-03-03 启停重置删除的队列集成

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

- 旧ID：`P06-07`；类型：有限 leaf；状态：未开始。
- 依赖：`BE-06-04`、`BE-07-02`、[已完成基线 P04-07](COMPLETED.md#g1)。
- 允许范围：`server/internal/bots/**`、`server/internal/delivery/**`、`server/internal/retention/**`，及对应测试。
- 门槛：`G-GIT`；条件未实证不记通过。

目标：启停暂停/恢复租约与去重；API reset旧代次取消轮询/撤销租约/重排，Webhook reset不影响API租约；旧轮询/ACK拒绝。永久删除级联队列/保护并留tombstone，共享附件不得误删其他Bot引用；同事务和重启故障回滚。

验收：旧轮询/ACK 拒绝。；恢复去重。；永久删除取消保护且不误删其他 Bot 附件。 运行上述允许包的共同检查。

<a id="be-03-01"></a>

### BE-03-01 生命周期完整HTTP

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

- 旧ID：`P04-07-S03`；类型：有限 leaf；状态：未开始。
- 依赖：[已完成基线 P04-07](COMPLETED.md#g1)、`BE-03-03`、[已完成基线 P04-07-PLAN01](COMPLETED.md#g1)、[已完成基线 P04-05-S03](COMPLETED.md#g1)。

正式接入两个凭据reset与DELETE，并验证PATCH启停走完整事务；只改httpapi/bots.go、router.go、cmd/relaynest/main.go及测试，仓储/SQL/auth只读。复用publicRequestSecurity、BotAuthorize、WithAuthorizedWrite，认证/目标/业务/changes/renew同事务；Cookie维持CSRF/同源，Bot凭据无管理权。reset无expected_version请求体，返回对应IssuedAPIToken/IssuedWebhookKey且no-store；普通查询不泄密。DELETE仅冻结confirm_name/expected_version，成功204，取消确认不发请求。

检查：真实路由覆盖Android Bearer、Cookie+CSRF、Bot拒绝、缺失/撤销/跨Bot与事务内失效，失败不续期/不写changes；API reset撤销旧租约/取消轮询并重排，Webhook reset不影响API租约，消息/failed保留。DELETE错名/旧版本无副作用、正确确认完整级联并保tombstone，共享附件不误删的证据来自BE-03-03。运行httpapi/cmd/bots/真实delivery/retention包测试、httpapi/cmd vet及gofmt；缺真实队列/故障组合不得用桩替代。reset内部耗尽等未冻结对外错误映射，实施前核对现有通则；若需新增状态/code/结构，先具体提案确认并串行更新合同，不自行猜。G1用实际已集成契约/接口。

## BE-04 消息读取编辑事件及故障边界

<a id="be-04-01"></a>

### BE-04-01 历史分页引用和共享已读

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

- 旧ID：`P05-03`；类型：有限 leaf；状态：未开始。
- 依赖：`BE-02`。
- 允许范围：`server/internal/messages/**`、`server/internal/sync/read*`、`server/migrations/**`，及对应测试。
- 门槛：`G-GIT`；条件未实证不记通过。

目标：按bot_seq分页、引用校验、共享已读取max和未读计数；自己的消息与编辑不增未读，旧设备不能倒退。

验收：越 Bot 引用拒绝。；旧设备不倒退。；自己的消息与编辑不增加未读。 运行上述允许包的共同检查。

<a id="be-04-02"></a>

### BE-04-02 应用消息 CAS 编辑

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

- 旧ID：`P05-04`；类型：有限 leaf；状态：未开始。
- 依赖：`BE-04-01`。
- 允许范围：`server/internal/messages/**`、`server/migrations/**`，及对应测试。
- 门槛：`G-GIT`；条件未实证不记通过。

目标：只允许应用编辑自身应用消息；保原bot_seq/创建时间/已读语义，revision CAS；正文/按钮与changes同事务，附件引用在BE-05-02组合。Android及Webhook不可编辑。

验收：旧revision冲突。；Android及Webhook不可编辑。；文本/按钮与变更同事务；附件引用集成由 P05-06 收口验收。 运行上述允许包的共同检查。

<a id="be-04-03"></a>

### BE-04-03 标准 JSON Webhook 接入

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

- 旧ID：`P05-09`；类型：有限 leaf；状态：未开始。
- 依赖：`BE-02`、`BE-05-02`。
- 允许范围：`server/internal/httpapi/webhook*`、`server/internal/messages/**`，及对应测试。
- 门槛：`G-GIT`；条件未实证不记通过。

目标：1. 复用消息创建事务接入独立只发送密钥。；2. 不增加映射/脚本/出站功能。

验收：标准 JSON/未知字段/幂等/超限/停用。；Webhook 不能编辑或下载。 运行上述允许包的共同检查。

<a id="be-04-04"></a>

### BE-04-04 消息附件故障边界回归

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

- 旧ID：`P05-10`；类型：有限 leaf；状态：未开始。
- 依赖：`BE-04-02`、`BE-05-04`、`BE-04-03`、`BE-04-05`。
- 允许范围：`server/internal/messages/*test*`、`server/internal/attachments/*test*`，及对应测试。
- 门槛：`G-GIT`；条件未实证不记通过。

目标：提交/附件恢复/编辑冲突交叉故障：所有已宣称持久成功的数据能恢复，无受保护文件被误回收。

验收：所有声明持久化成功的记录可恢复。；无受保护文件误回收。 运行上述允许包的共同检查。

<a id="be-04-05"></a>

### BE-04-05 按钮回调独立事件与投递

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

- 旧ID：`P05-11`；类型：有限 leaf；状态：未开始。
- 依赖：`BE-04-02`。
- 允许范围：`server/internal/messages/callback*`、`server/internal/delivery/event*`、`server/migrations/**`，及对应测试。
- 门槛：`G-GIT`；条件未实证不记通过。

目标：callback为独立持久快照/事件/投递，父消息和按钮版本校验；编辑不改已提交callback快照。旧版本/失效按钮409 BUTTON_STALE，callback_data原样；一次点击网络重试复用键，再次点击是新事件。

验收：旧版本/失效按钮409。；callback_data原样。；重复点击网络重试复用键。 运行上述允许包的共同检查。

## BE-05 附件完整生命周期

<a id="be-05-01"></a>

### BE-05-01 附件流式上传状态机

- 完成追加依赖（不改变上述开工依赖）：[已完成 BE-01 现有后端父范围](COMPLETED.md#g5-be01)；基础准备可先进行，真实全范围收口必须满足。

- P01 有限规划（2026-10-26）：owner Backend Orchestrator，Agent `/root/backend_dependency_plan`，基线 `388dd8ae1f3641bdfd26a7a421e4171b8da16971` / `work` 只读；只细化本 leaf 的准确 3～5 主文件工作包及迁移／接口边界，不写文件或预占迁移号。状态仍未开始；BE-01 完整完成追加门槛未满足，不据准备规划放行真实上传验收。
- P01 已返回有限拆分；R02 `/root/backend_recovery` 验证 P05-01 最终 `ed488e543d892f2353fbc95f72a2f0af98ba9a8d` 为基线祖先，六个 worktree 均无 0008 文件或迁移改动，唯一 HTTP 写入者不改迁移。由调度拆成下列两个执行子包，原完整验收不变；状态转准备中，只有两包和完整 BE-01 门槛全部满足才可收口。

- 子项证据：[STORAGE 已完成独审与精确集成](COMPLETED.md#g5-be05-storage)，源与最终 `064807f9e5e126f48c9360d4d02a40fc209f790e`；[HTTP 真实上传接线](#be-05-01-http) 尚未开始；[BE-01 现有后端依赖已完成](COMPLETED.md#g5-be01)，父原上传验收仍待满足。

- 旧ID：`P05-05`；类型：有限分拆收口（不整组派发）；状态：准备中（STORAGE 已完成，HTTP 及父验收待闭环，BE-01 现有后端依赖已完成）。
- 依赖：[已完成基线 P05-01](COMPLETED.md#g2)。
- 允许范围：`server/internal/attachments/**`、`server/migrations/**`，及对应测试。
- 门槛：`G-GIT`；条件未实证不记通过。

补充检查：ready前不能关联；流式内存有界，随机物理路径、字节限制/hash/fsync/rename及幂等一致，URL仅保存不抓取。默认图片10MiB/文件50MiB按已确认配置；不能把默认值当不可配置硬上限。真实上传接线必须依BE-01 runtime及Bot-first附件矩阵，上传流在正确授权后才读取。

<a id="be-05-01-storage"></a>

### BE-05-01-STORAGE 流式持久化状态机准备

已完成子项：[范围、完整验收与独审／精确集成证据](COMPLETED.md#g5-be05-storage)。仅存储准备完成，父 BE-05-01 仍未完成；0008 永久占用、写锁已释放，下一编号 0009 须另核。

<a id="be-05-01-http"></a>

### BE-05-01-HTTP 真实上传接线与组合验收

- 类型：有限 leaf；状态：`todo`；依赖 [已完成 BE-05-01-STORAGE](COMPLETED.md#g5-be05-storage)、[已完成 BE-01 现有后端父范围](COMPLETED.md#g5-be01)，必须分别独审和集成。准备路径为 `server/internal/httpapi/attachments.go`、`router.go`、`server/cmd/relaynest/main.go` 及相关测试，派发前固定真实基线及排他锁。
- 沿用原上传完整验收及冻结 query＋原始二进制流／201/no-store／错误合同；真实 Bot-first 格式及身份发现先于读流，Matched 拒绝不回退、Unavailable 失败关闭、仅 NoMatch 进入 Android 同快照门禁，最终提交重验代次／会话。Web Cookie／Webhook／receive-only 无上传权。
- 必须真实 HTTP＋SQLite＋文件组合，配置使用实际可注入正 int64 接口，不能默认值写死或宣称未实现的设置路由已生效；STORAGE 测试不能替代真实读流顺序、事务和安全错误验收。完成后父 BE-05-01 仍须核原完整验收，无范围丢失。

<a id="be-05-02"></a>

### BE-05-02 附件关联权限与保护引用

- 完成追加依赖（不改变上述开工依赖）：[已完成 BE-01 现有后端父范围](COMPLETED.md#g5-be01)；基础准备可先进行，真实全范围收口必须满足。

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

- 旧ID：`P05-06`；类型：有限 leaf；状态：未开始。
- 依赖：`BE-04-02`、`BE-05-01`、`BE-04-05`。
- 允许范围：`server/internal/attachments/**`、`server/internal/messages/**`、`server/migrations/**`，及对应测试。
- 门槛：`G-GIT`；条件未实证不记通过。

关联与消息/编辑/投递更新同事务；覆盖ready→关联竞态、引用增减和pending/leased/failed保护。receive-only只能下载已领取事件必需附件，不能任意浏览本Bot附件；跨Bot绑定拒绝。真实HTTP授权消费BE-01完整绑定/BOT01发现，匹配后拒绝不降级，存储故障503；未实现路由不得提前写已验收。

<a id="be-05-03"></a>

### BE-05-03 附件下载 Range 和安全响应

- 完成追加依赖（不改变上述开工依赖）：[已完成 BE-01 现有后端父范围](COMPLETED.md#g5-be01)；基础准备可先进行，真实全范围收口必须满足。

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

- 旧ID：`P05-07`；类型：有限 leaf；状态：未开始。
- 依赖：`BE-05-02`。
- 允许范围：`server/internal/attachments/**`，及对应测试。
- 门槛：`G-GIT`；条件未实证不记通过。

下载先授权资源域再打开文件；覆盖Range/ETag/下载租约、内容类型/安全文件名，路径穿越、symlink和物理路径泄漏拒绝。仅两附件端点按BE-01 Bot-first分流；NoMatch后Android先ID/会话同快照，Matched后停用/能力/资源域/代次拒绝不回退；不得先读Range或附件求BotID。真实HTTP+SQLite+文件组合必须验证资源/下载并发及保护租约释放，不能只做文件单测。

<a id="be-05-04"></a>

### BE-05-04 附件启动恢复与孤儿回收

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

- 旧ID：`P05-08`；类型：有限 leaf；状态：未开始。
- 依赖：`BE-05-03`。
- 允许范围：`server/internal/attachments/**`，及对应测试。
- 门槛：`G-GIT`；条件未实证不记通过。

目标：staging/deleting启动恢复与孤儿扫描；rename成功DB失败、进程退出、缺文件、活动上传保护可恢复且不伪造文件。本项不做消息历史清理。

验收：rename 成功 DB 失败/进程退出/缺文件/活动上传保护可恢复。；无伪造文件。 运行上述允许包的共同检查。

## BE-06 投递租约与失败管理

<a id="be-06-01"></a>

### BE-06-01 原子领取与 Long Polling

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

- 旧ID：`P06-01`；类型：有限 leaf；状态：未开始。
- 依赖：`BE-04-02`、`BE-04-05`。
- 允许范围：`server/internal/delivery/**`，及对应测试。
- 门槛：`G-GIT`；条件未实证不记通过。

目标：原子pending领取、不可预测lease token、多消费者互斥有效租约；Long Poll等待不持写事务，唤醒丢失/断链可取消恢复重领。

验收：多消费者不重复占有效租约。；等待不持写事务。；断链后可重领。 运行上述允许包的共同检查。

<a id="be-06-02"></a>

### BE-06-02 ACK 幂等与受限续约

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

- 旧ID：`P06-02`；类型：有限 leaf；状态：未开始。
- 依赖：`BE-06-01`、`BE-05-02`。
- 允许范围：`server/internal/delivery/**`，及对应测试。
- 门槛：`G-GIT`；条件未实证不记通过。

目标：有效租约ACK/受限renew与完成租约幂等重试，释放保护引用；旧租约/撤销/过期拒绝，同ACK200，续约限额验证；ACK只表接收而非业务成功。

验收：旧租约/撤销/过期拒绝。；同 ACK 200。；续约限额。；ACK 不代表业务成功。 运行上述允许包的共同检查。

<a id="be-06-03"></a>

### BE-06-03 退避失败与重启扫描

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

- 旧ID：`P06-03`；类型：有限 leaf；状态：未开始。
- 依赖：`BE-06-02`。
- 允许范围：`server/internal/delivery/**`，及对应测试。
- 门槛：`G-GIT`；条件未实证不记通过。

目标：仅实际领取消耗尝试，8次实际领取后第8次超时转failed；指数退避/抖动、租约超时扫描、可控时钟与重启恢复，内存唤醒丢失不丢持久任务。

验收：无消费者不耗次数。；第8次超时转失败。；时钟可控。；内存唤醒丢失仍恢复。 运行上述允许包的共同检查。

<a id="be-06-04"></a>

### BE-06-04 失败元数据列表和手动重试

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

- 旧ID：`P06-04`；类型：有限 leaf；状态：未开始。
- 依赖：`BE-06-03`。
- 允许范围：`server/internal/admin/deliver*`、`server/internal/delivery/**`，及对应测试。
- 门槛：`G-GIT`；条件未实证不记通过。

目标：失败列表仅脱敏元数据，禁止正文/附件URL/callback/原始错误；手动重试幂等并推进代次但event_id不变，不能重跑业务或改原持久快照。

验收：不含正文/附件 URL/callback/原始错误。；event_id 不变。；不重跑业务。 运行上述允许包的共同检查。

## BE-07 历史与墓碑清理

<a id="be-07-01"></a>

### BE-07-01 普通历史清理与投递保护

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

- 旧ID：`P06-05`；类型：有限 leaf；状态：未开始。
- 依赖：`BE-06-03`、`BE-05-04`。
- 允许范围：`server/internal/retention/**`、`server/internal/messages/**`，及对应测试。
- 门槛：`G-GIT`；条件未实证不记通过。

目标：默认30天、可控时钟和小批次期限清理；消息tombstone，pending/leased/failed、父消息及附件引用保护，未ACK/failed不删，离线设备不阻塞清理，引用过期可显示。

验收：30天可控时钟。；未 ACK/失败不删除。；离线设备不阻塞。；引用过期可显示。 运行上述允许包的共同检查。

<a id="be-07-02"></a>

### BE-07-02 变更 floor 与幂等墓碑回收

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

- 旧ID：`P06-06`；类型：有限 leaf；状态：未开始。
- 依赖：`BE-07-01`。
- 允许范围：`server/internal/retention/**`、`server/internal/idempotency/**`、`server/internal/sync/cursor*`，及对应测试。
- 门槛：`G-GIT`；条件未实证不记通过。

目标：事件保留/floor单调与最小幂等墓碑回收，受保护键不删，已过期资源410不复活，epoch/floor边界一致；不保留旧正文。

验收：受保护键不删。；过期资源410不复活。；epoch/floor 边界一致。 运行上述允许包的共同检查。

## BE-08 配置健康及并发综合回归

<a id="be-08-01"></a>

### BE-08-01 配置资源控制和系统健康投影

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

- 旧ID：`P06-08`；类型：有限 leaf；状态：未开始。
- 依赖：`BE-03-03`、[已完成基线 P04-04](COMPLETED.md#g1)。
- 允许范围：`server/internal/admin/**`、`server/internal/health/**`、`server/internal/httpapi/**`，及对应测试。
- 门槛：`G-GIT`；条件未实证不记通过。

全局设置retention_days/image_max_bytes/file_max_bytes为1～9223372036854775807规范十进制字符串，0非法；默认30天/10485760/52428800。SQLite int64序号在协议中保字符串，不擅改公共Sequence/Attachment.size。专用设置version/expected_version为JSON整数且1～9007199254740991，与设备版本/注册版本分开；CAS、同值递增、updated_at、settings.updated同事务。输入结构400/语义422、旧版409 GLOBAL_SETTINGS_CONFLICT；版本耗尽409 GLOBAL_SETTINGS_VERSION_EXHAUSTED，先输入再CAS再耗尽，禁止回绕/写入/发变更，GET仍可用。缩短保留期限的影响明确；磁盘满/写压力不得成功，检查DB和附件卷，FCM失败不使Hub整体unready。合同已冻结，具体schema/错误以OpenAPI为准。

<a id="be-08-02"></a>

### BE-08-02 投递与清理并发故障回归

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

- 旧ID：`P06-09`；类型：有限 leaf；状态：未开始。
- 依赖：`BE-08-01`、`BE-04-04`。
- 允许范围：`server/internal/delivery/*test*`、`server/internal/retention/*test*`、`server/internal/bots/*test*`，及对应测试。
- 门槛：`G-GIT`；条件未实证不记通过。

目标：租约/清理/下载/编辑/删除交叉并发及重启故障注入，AC-04/05关键状态证据；不得半提交、越权或误回收保护引用。

验收：AC-04/05 关键状态有证据。；无半提交/越权/保护竞态。 运行上述允许包的共同检查。

## BE-09 Hub同步bootstrap及WS

<a id="be-09-01"></a>

### BE-09-01 Hub 增量同步服务端

- 完成追加依赖（不改变上述开工依赖）：[已完成 BE-01 现有后端父范围](COMPLETED.md#g5-be01)；基础准备可先进行，真实全范围收口必须满足。

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

- 旧ID：`P08-01`；类型：有限 leaf；状态：未开始。
- 依赖：`BE-07-02`。
- 允许范围：`server/internal/sync/**`，及对应测试。
- 门槛：`G-GIT`；条件未实证不记通过。

当前投影与事件在同一读事务，固定高水位分页，删除事件不导致整批失败，重复消费幂等；所有序号为字符串，epoch/floor和410边界一致。新增Android保护路由必须接入BE-01真实绑定/事务接口后才验收，不以入口单次读ID替代。

<a id="be-09-02"></a>

### BE-09-02 Hub bootstrap 重建协议

- 完成追加依赖（不改变上述开工依赖）：[已完成 BE-01 现有后端父范围](COMPLETED.md#g5-be01)；基础准备可先进行，真实全范围收口必须满足。

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

- 旧ID：`P08-02`；类型：有限 leaf；状态：未开始。
- 依赖：`BE-09-01`、`BE-04-01`。
- 允许范围：`server/internal/sync/**`，及对应测试。
- 门槛：`G-GIT`；条件未实证不记通过。

固定H及创建序列限定，分页令牌和过期按冻结协议，不持跨请求长事务；并发编辑/删除/新建、floor超越均可重建，版本不回退。路由实例绑定消费BE-01同快照接口。

<a id="be-09-03"></a>

### BE-09-03 Hub WS 提示与连接撤销

- 完成追加依赖（不改变上述开工依赖）：[已完成 BE-01 现有后端父范围](COMPLETED.md#g5-be01)；基础准备可先进行，真实全范围收口必须满足。

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

- 旧ID：`P08-03`；类型：有限 leaf；状态：未开始。
- 依赖：`BE-09-01`、[已完成基线 P04-05](COMPLETED.md#g1)、`BE-03-03`。
- 允许范围：`server/internal/realtime/**`、`server/internal/httpapi/ws*`，及对应测试。
- 门槛：`G-GIT`；条件未实证不记通过。

仅Bearer WS，hello/提示合并/心跳/session_revoked，凭据不进URL、消息写仍REST、Web管理不建聊天WS。握手升级发布前验证实例/会话，连接窗口可由增量补齐，慢客户端有界，撤销关闭；消费BE-01完整runtime门禁，存活连接不跨部署搬迁。

## BE-10 正式推送服务端

<a id="be-10-01"></a>

### BE-10-01 Hub 推送注册与设备通知偏好

- 完成追加依赖（不改变上述开工依赖）：[BE-01](BACKEND.md#be-01)；基础准备可先进行，真实全范围收口必须满足。

- P01 有限规划（2026-10-26）：owner Backend Orchestrator，Agent `/root/push_registration_plan`，固定基线 `571dc5dca696d4dfb446a868ca58169cf488579b` / `work`，只读细化当前 leaf 的准确文件／事务边界与准备／真实接线拆分，不修改代码、状态或分配迁移。状态仍未开始；新迁移候选 0009 须另核锁，完整 BE-01 门槛未满足，不据规划放行真实注册验收。
- P01 已返回 STORAGE／HTTP 两有限子包建议；主调度采用并保留原完整验收。R04 `/root/resume_checkpoint` 核实际基线 `571dc5dca696d4dfb446a868ca58169cf488579b`，P04-05 精确集成 `d78711faa56dd01b7e10cf4eec3b70490f2d04e9`／最终 `005994a049635163e879c069ecf02526c8ddbe1f`、P03-07 集成 `981692f49663075aa553ce45aa645e75197b15ea`／最终 `375a270c74080d9eb3ea8e975d8c79814afc091f` 均为祖先；九个 worktree 均无 0009 或迁移修改，当前无迁移 writer。由调度将 0009 排他分配下列 STORAGE 子包至精确集成，不与未来迁移并写。

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

- 旧ID：`P09-01`；类型：有限分拆收口；状态：准备中（STORAGE／HTTP 两包及完整 BE-01 门槛未闭环）。
- 依赖：[已完成基线 P04-05](COMPLETED.md#g1)、[已完成基线 P03-07](COMPLETED.md#e1)。
- 允许范围：`server/internal/devices/**`、`server/migrations/**`，及对应测试。
- 门槛：`G-API`、`G-GIT`；条件未实证不记通过。

按T9.3冻结通道集合及逐provider注册版本/时间实现，仅当前设备会话可登记；部分上报不清空其他通道。全局/Bot/DND/预览配置CAS独立于注册代次，SDK注册成功不改用户提醒模式，vendor激活须满足冻结资格门槛。target_invalid仅设备/会话/provider/发送注册版本匹配才条件解绑对应注册，保留设备/会话/其他通道/偏好；会话撤销使全部通道失效，token与秘密不回显/日志。覆盖同版异值、并发模式切换、撤销后迟到登记及旧代次失败不删新注册。Android写入口另须BE-01实例绑定。

<a id="be-10-01-storage"></a>

### BE-10-01-STORAGE 注册与通知偏好存储准备

- 类型：有限 leaf；状态：`blocked`（2026-10-26，环境迁移中断 I01，恢复后回 integrating）；owner Backend Orchestrator；Implementer `/root/push_storage_implement`；固定基线 `571dc5dca696d4dfb446a868ca58169cf488579b` / 目标 `work`。只消费已集成 AUTH／设备／契约基础，不消费尚在独审的 HTTP；父项完整完成门槛不作本准备包前置。
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

<a id="be-10-01-http"></a>

### BE-10-01-HTTP 当前设备注册与偏好真实接线

- 类型：有限 leaf；状态：`todo`；依赖已独审集成 `BE-10-01-STORAGE` 与 [已完成 BE-01 现有后端父范围](COMPLETED.md#g5-be01)。主调度明确后包允许 `server/internal/devices/{push_http.go,http.go}`、`server/internal/httpapi/router.go`、`server/cmd/relaynest/main.go` 及必要对应测试，实际派发前再固定 SHA／排他锁；其他生产文件只读。
- 按冻结五 operations 接线 GET/PUT current/push、DELETE current/push/{provider}、GET/PUT current/notifications；真实当前 Android Bearer 与同事务实例门禁，拒 Web/Bot／目标选择器／客户端时间，262144 字节／严格 schema／no-store／无 token。原设备及认证／Bot 路由非回归，真实 router＋main＋SQLite 组合，不以直接 handler 测试冒充。
- 原 BE-10-01 全部验收及资格边界保留，STORAGE 准备不替代 H01／G1；后端发送适配器及平台实测仍分别归后续任务。

<a id="be-10-02"></a>

### BE-10-02 Hub 推送待办合并与调度

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

- 旧ID：`P09-02`；类型：有限 leaf；状态：未开始。
- 依赖：`BE-10-01`、[已完成基线 P05-01](COMPLETED.md#g2)。
- 允许范围：`server/internal/push/**`，及对应测试。
- 门槛：`G-GIT`；条件未实证不记通过。

消费消息事务待办，按设备合并水位、退避/期限/共享已读过滤；用途sync/alert分开、job记录会话/注册/配置版本、稳定提醒键和期限，水位合并不使旧提醒复活。重试前复核当前DND/已读/配置/注册/期限，模式或注册改变隔离旧job，同设备租约及条件状态防双worker/迟到结果污染。平台受理不是展示或已读ACK，超时为结果不确定，不自动vendor→FCM补响。

先用假adapter验证框架/FCM路径，OPPO外部审核不阻该有限任务；假结果不算厂商真机通过。PUSH-D2/D3已批准切换仅未来新事件、外发在途残余和离线展示限制；本地fencing不能撤回已外发，不以批准替代P02-05验证。发送失败不回滚消息，推送耗尽不进业务failed、不保护普通历史，DND/全部已读任务不复活。

<a id="be-10-03"></a>

### BE-10-03 推送适配父收口（FCM／OPPO）

- 完成追加依赖（不改变上述开工依赖）：[BE-10-03-FCM](BACKEND.md#be-10-03-fcm)、[BE-10-03-OPPO](BACKEND.md#be-10-03-oppo)；基础准备可先进行，真实全范围收口必须满足。

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

- 旧ID：`P09-03`；类型：父收口，不整组派发；状态：未开始。
- 依赖：`BE-10-02`、[已完成基线 P02-04](COMPLETED.md#a2)。
- 允许范围：`server/internal/push/**`，及对应测试。
- 门槛：`G-FCM`、`G-GIT`；条件未实证不记通过。

同一旧卡保留两个可分别派发的有限分支，不能把FCM通过写成双adapter完成：先做类型化插槽/静态装配与FCM；OPPO为可选、外部条件未满足时blocked的独立分支，依 [ANDROID 的AND-06](ANDROID.md) 对P02-05现行鉴权/单推/错误表及实测证据，不阻框架/FCM。分支没有新合同或自动激活资格；父项只在两分支按适用发布范围分别有证据后收口。

框架/FCM锁SDK/API并安全加载凭据，核心不含厂商请求字段；adapter只收通用提示/必要元数据，不传业务消息模型，无正文/附件URL/秘密，不把服务端秘密装进APK。OPPO按已批准PUSH-D1文案及D2/D3边界，实施前核官方当前接口，不能编造外部结果。

输出区分平台受理、明确失效、暂态、限流、配置错、结果不确定；未知/格式/鉴权错误不无条件删注册。明确失效按发送快照条件解绑单通道，绝不删除会话/设备/新token；稳定提醒键重试，超时不跨通道补响，不承诺恰好一次。桩覆盖限流/鉴权/未知/已受理后超时/轮换迟到失败；FCM与OPPO分别记录版本、环境、真实证据，OPPO无环境明确blocked。

<a id="be-10-03-fcm"></a>

### BE-10-03-FCM 类型化插槽、静态装配与FCM适配

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

- 旧ID：`P09-03` 拆分叶；类型：有限 leaf；状态：未开始。
- 开工依赖：`BE-10-02`、[已完成基线 P02-04](COMPLETED.md#a2)。
- 门槛：`G-FCM`、`G-GIT`。

server/internal/push/**：T9.3类型化插槽/静态装配、FCM最小data载荷、安全凭据加载、错误分类；核心无厂商字段。限定框架+FCM，不等待OPPO资格。

外部条件：测试服务账号安全加载；SDK/API固定版本及真实环境证据。

验收：

- [ ] 无正文/URL/服务端秘密。
- [ ] 按明确无效响应与发送快照版本条件解绑对应通道，不删设备会话/新token；暂态/未知/鉴权错误不无条件删除。
- [ ] 桩覆盖限流、鉴权、未知、受理后超时、注册轮换迟到失败；真实provider版本/环境证据单列，必需未测blocked。
- [ ] 稳定提醒键重试，超时不跨通道补响，不保证恰好一次；独审最终实现SHA及精确集成。

检查：运行受影响 push 包普通／race、vet、gofmt 与真实 HTTP/SQLite 调度组合；固定SDK/API及实际 provider 环境分别记录，缺必须条件记 blocked／NOT_RUN。沿用父项完整验收及通道边界，不用桩代替真实厂商结果。

<a id="be-10-03-oppo"></a>

### BE-10-03-OPPO OPPO适配及独立平台验收

- 完成门槛角色：`G-OPPO` 须通过后父项／消费者才能收口。

- 证据与交接：尚无本步骤新增实施／独审／集成证据；已有历史或WIP见下文。派发时由调度登记 owner、固定基线、路径／锁、工作树／分支；回交实际检查及未测限制、实现／独审／精确集成 SHA。

- 旧ID：`P09-03` 拆分叶；类型：有限 leaf；状态：受阻（缺平台资格／权限与实测证据，只阻本分支）。
- 开工依赖：`BE-10-03-FCM`、`AND-06`。
- 门槛：`G-FCM`、`G-GIT`。

server/internal/push/**：基于已查证平台接口独立OPPO适配；PUSH-D1固定通用标题正文及D2/D3显式选择、未知结果不跨通道回退等边界。缺账号/审核只阻本分支，不要求现在开户，不编造接口/资格。

外部条件：P02-05现行鉴权/单推/错误表与实测证据；获准测试/正式权限（依本次目标）。

验收：

- [ ] 无正文/URL/服务端秘密。
- [ ] 按明确无效响应与发送快照版本条件解绑对应通道，不删设备会话/新token；暂态/未知/鉴权错误不无条件删除。
- [ ] 桩覆盖限流、鉴权、未知、受理后超时、注册轮换迟到失败；真实provider版本/环境证据单列，必需未测blocked。
- [ ] 稳定提醒键重试，超时不跨通道补响，不保证恰好一次；独审最终实现SHA及精确集成。

检查：运行受影响 push 包普通／race、vet、gofmt 与真实 HTTP/SQLite 调度组合；固定SDK/API及实际 provider 环境分别记录，缺必须条件记 blocked／NOT_RUN。沿用父项完整验收及通道边界，不用桩代替真实厂商结果。
