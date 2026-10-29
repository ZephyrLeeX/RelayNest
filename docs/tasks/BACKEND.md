# Go 后端剩余任务

## 当前权威恢复入口（2026-10-27）

- 当前唯一推进目标为项目根 `/home/codex/projects/RelayNest` / `resume/backend-20261026`。Integrator `/root/push_i01_retry` 核实原根只有未跟踪 `.worktrees/`、无受控用户修改／路径冲突后，从既有本地克隆执行无 force 的本地 fetch，准确导入 `8a1e875730410be49ae810e9ce5a5b84932085f9`，并已安全 switch 到该 resume 分支；导入时 commit／tree 与克隆完全一致，后续本节元数据提交以实际 HEAD 为准。未从 main 重做、未重复 cherry-pick，`main` ref 仍为 `01c74a776b0b76fcc6616f60ae637a5a53f18384`。
- ADMIN 完整 WIP／修复链现已在准确 `665edd785d8784ae0e3304d3d3bdfed2d474f872` 组合取得 A04 PASS、全部必检与 I01 安全 ff，由总调度批准[有限叶完成](COMPLETED.md#g5-be02-admin)；原 `work/backend-admin-resume@92404a1…` 仅保留 source 可达性。`ADMIN-A03-B01` 已因准确组合全服 race PASS 关闭；独立 [STORAGE-BUSY-01 修复](COMPLETED.md#g5-storage-busy-01) 与推送 STORAGE 已 done 的结论不变，父 BE-02 限定范围已另经[独立 R01／A01／I01](COMPLETED.md#g5-be02-compose)完成，其他 HTTP／跨端门槛不自动通过。
- `.worktrees/backend-local@8a1e875…` 现在仅为历史恢复载体，不再并行推进第二个目标；其 Git/index 未被改写。ADMIN 与 baseline 等现有独立 worktree 路径／归属保持不动，仍按原登记的仓库和固定 SHA 检查。原根未跟踪 `.worktrees/` 不得整体 stage，也不删除或覆盖其中现场。
- 下次先在项目根核真实分支／HEAD、dirty、refs 与文档中的未闭环证据；不要再次切回默认 main、克隆新目标或重复应用已有代码。需要的源码实现／审计／修复／精确集成仍使用独立 Agent 和排他工作树；用户本轮已明确“同意”同库共享、可取消的写准入修复，有限子叶 `STORAGE-BUSY-01` 已完成两编号修复、A02 PASS 与 I01 精确合入；ADMIN 已完成准确组合检查、A04 续独审及 I01，父 BE-02 原四项已另经 R01／A01／I01 限定收口；未来能力仍按各自任务执行。未 push、部署或创建 `.codex-task-done`；下方旧环境记录保留作历史证据。

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


## Debian 本轮恢复（2026-10-26）

- Recovery `/root/backend_recovery` 核实真实 origin 为 `ZephyrLeeX/RelayNest`，原工作树 `/home/codex/projects/RelayNest` 为干净的 `main@01c74a776b0b76fcc6616f60ae637a5a53f18384`，三个恢复 refs 与交接 SHA 一致；准确恢复基线为 `bde0193a5e1f561cb0b3d30a08bea691fcd499a8`，未从 main 重做。
- 原仓库 `.git` 被执行环境限制为只读，创建 worktree 时无法写 ref。总调度批准在允许写入目录建立完整独立本地克隆 `/home/codex/projects/RelayNest/.worktrees/backend-local`（`--no-hardlinks`，无 alternates），目标分支 `resume/backend-20261026`；ADMIN 隔离树为 `/home/codex/projects/RelayNest/.worktrees/backend-admin` / `work/backend-admin-resume`。下述本地合入仅发生在该独立仓库，原 main 未更新，原受控文件未改，仅出现未跟踪 `.worktrees/`；不得将其整体 stage。
- 先前因原仓库只读，恢复曾以 `.worktrees/backend-local` 为目标并保留 `.worktrees/backend-admin` / `work/backend-admin-resume@92404a1d3fee0b2516133867af585e68a94ce772`；权限解除后已按本文件开头“当前权威恢复入口”将目标迁回项目根。ADMIN 隔离现场继续保留，不再从 main 复制、不重复克隆或 cherry-pick，旧克隆不再作为第二个推进目标。
- 实测工具版本 Go **1.27.3**、Node **22.23.2**、npm **10.9.8**。旧 `/tmp` 证据仍仅作历史；本轮日志为下列实际新运行结果。mise 默认缓存／信任目录只读失败后使用可写临时缓存或已安装锁版二进制，未升级版本。
- 锁版 Web 初次离线 `npm ci` 因 Node `spawnSync EPERM` 失败（独立 `/bin/true` 也复现）；`npm ci --offline --ignore-scripts` 后原 `npm run build`（typecheck＋Vite）PASS，未改锁文件或源码。ADMIN 核对 Web 受控输入同版后复用该真实 dist；生成物与依赖未提交。
- 权限解除前硬阻塞为本地 IPv4／IPv6 loopback socket 监听均 `operation not permitted`；不删除断言、不改超时，不以编译／静态检查替代真实 HTTP＋SQLite。BE-02 父组合等待 ADMIN 通过并精确集成；BE-05-01-HTTP、BE-10-01-HTTP 的真实组合同受 socket 门槛限制，后者还等待 STORAGE I01。暂不新增更多未验证 HTTP WIP，其他下游仍按原依赖与外部门槛执行。
- 原恢复条件：运行环境允许 loopback 监听后，针对各固定 SHA 重跑真实 HTTP＋SQLite、相关普通／race 与必跑组合；取得独立审计 PASS 后才精确集成 ADMIN，STORAGE I01 全部检查通过后才可完成。若要更新原主仓库，仍须其 `.git` 可写；本轮未 push、部署或发布，未创建 `.codex-task-done`。
- 续跑核验（2026-10-26T15:45:51Z，Recovery `/root/resume_check`）：原 main 仍为 `01c74a7…`（仅未跟踪 `.worktrees/`），本地集成树 `1fd63700e75810d4c3c99010c44de0b6a31358b4` 与 ADMIN 树 `c937602975c232c62a20808f3a376a86b2d84795` 均 clean、无 Git lock 文件；锁版二进制版本仍为 Go 1.27.3／Node 22.23.2／npm 10.9.8。最小 Python socket bind/listen 实测 `127.0.0.1` 和 `::1` 均 FAIL（errno 1，`Operation not permitted`），故代码及 I01／ADMIN-A01-B01 阻塞结论未变，未重复实施、集成或重跑必然受阻的重型矩阵，不新增 PASS；三个工作目录均无完成标记。

- 权限解除后进展（2026-10-27，UTC+8）：用户放开权限后，新 Agent 真实 loopback／推送 STORAGE I01 全部通过并完成；ADMIN 原完整 WIP／夹具修复及 SQLITE_BUSY 根因研究均保留在[完成证据](COMPLETED.md#g5-be02-admin)。独立写准入修复已 A02／I01 PASS；额度中断的准确 ADMIN 组合部分日志不冒充 whole PASS，恢复后固定 `665edd7` 全服普通／race／vet／build 完整 PASS，A04 独立六组 PASS、I01 已精确 ff，ADMIN 叶及 BE-02 原四项有限父组合各自独立闭环。0009 永久占用；BE-10-01-HTTP 阶段 2 正在实施验证，未因本父完成提前通过，未 push 或创建完成标记。

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

已完成限定父范围：[BE-02 原四项生产 Bot＋Admin HTTP／SQLite 组合的 R01／A01／I01 及方法限制](COMPLETED.md#g5-be02-compose)。只完成当前 plain／markdown 创建与持久化回执；正式 reset HTTP、Webhook、附件／引用／按钮及跨端门槛仍按后续条目执行，不能将未实现能力标 done。

<a id="be-02-bot"></a>

已完成子项：[BE-02-BOT Bot 正式创建 HTTP 的范围、验收与证据](COMPLETED.md#g5-be02-bot)。

<a id="be-02-admin"></a>

已完成子项：[BE-02-ADMIN 有限文本正式 HTTP 的完整 WIP、A04／I01 与范围证据](COMPLETED.md#g5-be02-admin)。父 [BE-02 限定四项组合](COMPLETED.md#g5-be02-compose) 已另经 R01／A01／I01 完成，不扩大子叶或未来功能范围。

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

- 证据与交接：HISTORY-STORAGE、REFERENCE-CREATE、READ-STORAGE 已各独审／精确集成，见下方完成链接；[HISTORY-HTTP](COMPLETED.md#g5-be04-history-http) 已完成既定历史／详情两GET；[READ-HTTP](#be-04-01-read-http)独审PASS后integrating；[ATTACHMENT-READ](#be-04-01-attachment-read)已派running，父最终组合仍另包，父原目标和验收不变。

- 旧ID：`P05-03`；类型：有限 leaf；状态：`running`（2026-10-27，首叶已完成，父原验收未完成）。
- 依赖：[已完成 BE-02 限定父组合](COMPLETED.md#g5-be02-compose)。
- 允许范围：`server/internal/messages/**`、`server/internal/sync/read*`、`server/migrations/**`，及对应测试。
- 门槛：`G-GIT`；条件未实证不记通过。

目标：按bot_seq分页、引用校验、共享已读取max和未读计数；自己的消息与编辑不增未读，旧设备不能倒退。

验收：越 Bot 引用拒绝。；旧设备不倒退。；自己的消息与编辑不增加未读。 运行上述允许包的共同检查。

<a id="be-04-01-history-storage"></a>

#### BE-04-01-HISTORY-STORAGE 历史／详情与同 Bot 引用只读仓储准备

已完成有限子叶：[完整 source 链、实际检查、A01／I01 映射及范围限制](COMPLETED.md#g5-be04-history-storage)。仅仓储准备已集成；父 BE-04-01 仍 `running`、未完成原历史／引用／共享已读验收，REFERENCE-CREATE 已另独审／精确集成，READ-STORAGE已另完成，[HISTORY-HTTP](COMPLETED.md#g5-be04-history-http)已完成；本仓储叶不增加共享已读／附件投影／公开 cursor／HTTP。0009 保留，0010 已另专属 READ-STORAGE，本 HISTORY 子叶未分配新号。

<a id="be-04-01-reference-create"></a>

#### BE-04-01-REFERENCE-CREATE 既有双入口文本创建接通同 Bot 回复引用

已完成有限叶：[完整源链、A01／精确I01映射、新HTTP并集实际检查及COMMIT配对限制](COMPLETED.md#g5-be04-reference-create)。只完成既有 Bot API／Android 文本引用；父 BE-04-01 仍 running，READ-STORAGE已另完成，[HISTORY-HTTP](COMPLETED.md#g5-be04-history-http)已完成，共享已读HTTP、附件投影和父最终组合未完成，Webhook引用／附件／按钮不开放。

<a id="be-04-01-read-storage"></a>

#### BE-04-01-READ-STORAGE 共享已读事务仓储与原子初始化

已完成有限叶：[完整源链、A01及准确新并集I01、迁移占用与范围限制](COMPLETED.md#g5-be04-read-storage)。仅可信事务仓储／初始化完成，0010正式永久占用、实施／集成锁已释放；父BE-04-01仍running，[HISTORY-HTTP](COMPLETED.md#g5-be04-history-http)已完成，共享已读HTTP、同步投影／推送过滤及最终组合未完成。

<a id="be-04-01-history-http"></a>

#### BE-04-01-HISTORY-HTTP Android 历史分页与详情真实 HTTP

已完成有限两GET叶：[完整源链、A01真实FAIL／F01／不同A02关闭-01及准确3e2并集I01](COMPLETED.md#g5-be04-history-http)。原fixture／race FAIL与NOT_RECORDED、动态UUID核对异常、临时原审计fixture格式差异及裁定完整保留；6路径同受审6fa、SQL1～11及其他target保留。父BE04仍running；共享已读PUT由下方READ-HTTP另integrating，附件关系读投影另running，最终组合未完成。

<a id="be-04-01-read-http"></a>

#### BE-04-01-READ-HTTP Android 共享已读真实HTTP

- 类型：有限子叶；状态：`integrating`（2026-10-29，原ready→running→review，固定SHA不同A01 PASS后进入准确I01）。固定canonical产品基线 `3e2b13a22bfad40efa9b9290ba3d22f4b3e79248`；owner Backend Orchestrator，Implementer `/root/read_http_implementer` 已派，隔离私树 `.worktrees/backend-be04-read-http`／`impl/backend-be04-read-http-20261029` 从固定3e2起步，范围仍为三main／两新增测试。main／router／read.go排他实现锁仅授本Implementer。首次最小普通检查有3个坏数据fixture被既有CHECK阻止注入的FAIL，历史保留；仅本新增测试在注入时临时ignore_check_constraints后立即恢复，生产代码未因该fixture调整。复验6顶层／64全例 **PASS**（13.844秒），证据 `/tmp/relaynest-be04-read-http-min-20261029/targeted-second.json`，log SHA256 `3a06d704275e7fe707769634323f5cbce67ac58e09962c566b500dbc638d6795`，输入前后同值。完整source链 `3e2b13a22bfad40efa9b9290ba3d22f4b3e79248 → 7b55e6a2883f83f37fcb12f59cebc2fb32682432`，单commit五路径／工作树clean；source全服普通334顶层／1501全例 **PASS**（191.897秒）、八包race263顶层／1265全例 **PASS**（863.510秒）、vet／build／gofmt／diff **PASS**。真实TLS七响应合同 **PASS**（3.065秒），read_seq字符串／version整数／UTC／无unread；新增64例及旧测试保留，146源码输入＋3embed同值，Web／API核hash复用、npm／validator本轮NOT_RUN。交接 `/tmp/relaynest-be04-read-http-full-20261029/handoff.json`；首次commit因Git identity缺失exit128，单命令 `-c` 继承既有Codex身份后重试成功，未改Git config。不同 `/root/read_http_auditor` 固定7b55完成A01 **PASS**、无发现，独立7顶层／67全例18.966秒exit0（含既有6／64及新增MAX版本noop最终撤销／延迟FK提交失败两子例，全snapshot不变）；证据 `/tmp/relaynest-be04-read-http-a01-20261029/{handoff.json,targeted.json,targeted.log}`，log SHA256 `8392b0fa3b0363685176cbf83a79e211e069725c07203dec7ca9a02ac0d0c1ae`。源CI与独审分别记证，未done。SYNC仍todo排main；附件读叶可在无重叠路径的私树并行准备，Go检查严格串行。无需额外用户批准。
- 准确I01：不同Integrator `/root/read_http_planner` 从实际target `resume/backend-20261026@c8046318f20ba347d436ba3d31b13ae0f1c0de1b` 建 `.worktrees/backend-be04-read-http-i01`／`integrate/backend-be04-read-http-i01-20261029`，一次无冲突重放完整source `7b55e6a2883f83f37fcb12f59cebc2fb32682432 → 79b5c1b`，range-diff单项 `=`、五路径blob同受审源。target相3e2仅三docs；候选全产品与固定7b55逐字节等值，146源码／24 Web／6 API／3真实embed核hash同值，SQL0001～0011及PENDING／JOB／HISTORY等既有产品保留，未合附件私树SQL0012。独立核原CI及A01实际argv／start-end／exit／log hash／完整test identities及七个TLS合同响应SHA256 `ed2873566c6398dd368c71cf6fd5fe2c3596e5b476518b54833fbf0621ebd3ab`；**精确产品等值复用PASS，本I01未新跑Go**，npm／validator亦NOT_RUN。五source文件产品gofmt／diff PASS；原Auditor临时fixture非gofmt单列且逐字节保留，不篡改、不计产品格式FAIL。冻结／核验证据 `/tmp/relaynest-be04-read-http-i01-20261029/`；最终候选提交、root安全ff及总调度归档按实际交接确认，不代父BE04／附件／同步或global完成。
- 单目标与允许路径：仅新增 `server/internal/httpapi/read.go`、最小扩展 `server/internal/httpapi/router.go` 和 `server/cmd/relaynest/main.go` 三主文件，必要新 `server/internal/httpapi/read_test.go`／`server/cmd/relaynest/read_test.go`。消费已完成 `sync.AdvanceReadTx`／`GetReadStateTx`，不改storage／sync生产仓储、SQL、冻结API、依赖或其他模块。无DDL，0009～0011永久保留，不占0012。
- 冻结入口：`PUT /api/v1/client/bots/{id}/read`，仅Android真实client write授权／实例当前会话，在同一可信授权Tx最终复核。公开请求唯一字段 `through_seq`，规范非负int64十进制字符串；严格单object，拒重复／额外字段、尾随内容、错类型，body上限262144字节。200固定no-store，结构 `{read:{bot_id,read_seq字符串,version JSON整数,updated_at UTC},server_time}`，不暴露UnreadCount；不新增合同外路由／错误码／外部接口。
- 事务与边界：共享max单调，same／old noop不改read版本／时间／事件；上界取bots.bot_seq（含已删除最高消息），超界422 `READ_SEQ_OUT_OF_RANGE`，缺Bot404、停用Bot可推进已读；缺状态行／损坏及前进版本溢出503，MAX版本same／old仍合法noop。所有响应投影及JSON可序列化验证在授权Tx成功前完成；错误／COMMIT／最终权限或绑定失效不续期、不改状态／事件、不部分发布，不跨事务拼授权或暴露秘密。
- 真实验收及必跑检查：TLS生产router／main＋SQLite、双device／sessions并发max、旧值／边界／停用／删最高／损坏／MAX版本、SQL／changes／COMMIT故障、撤销／期限／代次／实例和最终权限绑定原子；完整响应类型／UTC／no-store／秘密边界。受影响httpapi／cmd／sync／messages／auth／storage／changes／bots相关普通与race，全服普通／vet／build、gofmt／diff及冻结响应合同核验；保原50ms／12并发／10ms／timeout与旧断言。Web／API未改核hash及真实embed准确复用，不冒称新跑npm／validator。实施单独本地commit后不同Agent固定SHA独审、编号修复重审及准确I01，全部收口后由总调度决定done。
- 父与后包：仅共享已读PUT接线，不实现附件关系读投影／同步投影／推送READ过滤、CAS／Buttons／worker／发送，也不完成BE04父或global。[ATTACHMENT-READ](#be-04-01-attachment-read)已按既定TECH的message_attachments关系另running并专属分配0012；无需等未来CAS／Buttons／完整BE05绑定写，不裁父原验收。本READ-HTTP自身无DDL、不消费未验收附件私树。

<a id="be-04-01-attachment-read"></a>

#### BE-04-01-ATTACHMENT-READ 附件关系与历史／详情读投影

- 类型：有限后包；状态：`running`（2026-10-29，原ready经总调度正式派发）。owner Backend Orchestrator；Planner／纯文档Integrator `/root/read_http_planner`，准备树 `.worktrees/backend-be04-attachment-read-docs`／`docs/backend-be04-attachment-read-ready-20261029`。固定canonical实施基线 `a50972648157eb929c270937125372e8aed4a1a3`（纯文档后继，产品同已验收 `3e2b13a22bfad40efa9b9290ba3d22f4b3e79248`）；原 `/root/attachment_read_implementer` 两次Selected model is at capacity，未写草稿、写权已撤，不计开发FAIL。实际Implementer `/root/backend_restore_retry` 已接手 `.worktrees/backend-be04-attachment-read`／`impl/backend-be04-attachment-read-20261029@a50972648157eb929c270937125372e8aed4a1a3`，实核clean／无其他writer起步，按既定五main／四count及本叶新增测试实施，0012号锁不变。现13路径（五main含SQL0012／四新增测试／四处精确count 11→12）已准备，静态gofmt／diff／旧SQL0001～0011逐字节保留检查 **PASS**；首次最小普通01有新增fixture误写既有表名idempotency_keys的FAIL，仅本新测试改为真实idempotency；原FAIL保留，未改生产。02三顶层／14全例复验PASS（8.060秒），与01六个未改顶层PASS组合覆盖九顶层／25全例，不冒称单次九项全PASS；证据 `/tmp/relaynest-be04-attachment-read-min-20261029/handoff.json`。准备期21个tracked Web＋3真实dist的计数口径及原预期tracked24断言异常保留，不计Go失败。全服普通／相关race／vet／build矩阵刚开始、尚无终态，不预判PASS。尚未commit，独审／I01 **NOT_RUN**；未来Auditor须不同于实际Implementer，不得自审。已有持续授权内有限分包，无新增需求／外部接口或架构决定。
- 已集成依赖：[HISTORY-STORAGE](COMPLETED.md#g5-be04-history-storage)、[REFERENCE-CREATE](COMPLETED.md#g5-be04-reference-create)、[HISTORY-HTTP](COMPLETED.md#g5-be04-history-http)、[附件STORAGE](COMPLETED.md#g5-be05-storage)及[既定上传父组合](COMPLETED.md#g5-be05-compose)，并复用既有真实Android授权／实例绑定和READ仓储。无需等待READ-HTTP、未来CAS／Buttons／完整BE05绑定写HTTP／下载／保护引用；可信关系夹具不代表这些生产写能力已验收。
- 五主文件与单目标：新增 `server/migrations/0012_message_attachments.sql`、`server/internal/messages/message_attachment_refs.go`，仅扩展 `server/internal/messages/models.go`、`server/internal/messages/history.go` 和 `server/internal/httpapi/message_history.go`。实现TECH既定message_attachments当前关系（消息／附件ID、引用kind、必要稳定顺序、复合唯一约束及显式外键），可信read Tx内读取及同Bot校验，历史列表／详情投影冻结AttachmentReference。复用现有内部AttachmentReference类型，不修改create.go／messages.go共享DTO、main／router或新增生产绑定入口；不添delivery_attachment_refs／投递保护、下载、GC、命令执行或其他schema需求。若需超允许文件或既定TECH模型，先报告证据协调锁／范围。
- 迁移号与路径锁：只读核查全部71个common库refs及注册worktrees，最高0011且无0012；总调度本轮正式独占分配 `0012_message_attachments.sql` 给本叶，其他任务不得占用。0009～0011永久原占用、SQL0001～0011逐字节及ledger checksums不改；本次核其同154f，旧11文件checksum manifest SHA256 `e24d884b01410e38deb52ee546ba99a2c37ca928f94c46a76f55ede97e1f295c`，开工／I01须再次核对。0012目录条目、五主文件及下述四个count fixture实现写锁仅授本叶实际Implementer `/root/backend_restore_retry`；本次metadata不写SQL或源码，不把号锁分配或实施在途视为迁移已验收。
- 必要测试与精确fixture例外：新增 `server/internal/messages/message_attachment_refs_test.go`、`server/internal/messages/message_attachment_migration_test.go`、`server/internal/httpapi/message_history_attachments_test.go`、`server/cmd/relaynest/message_history_attachments_test.go`。仅额外允许 `server/internal/attachments/schema_test.go:92`、`server/internal/sync/read_migration_test.go:79`、`server/internal/devices/push_repository_test.go:307`、`server/internal/push/job_repository_test.go:311` 四处最新升级总数11→12；定位以基线行为为准，不机械全文替换。job旧十迁移构造len(checks)=10、故障rollback ledger=10及所有旧断言／timeout／50ms／12并发／10ms保持原样；storage动态计数不改。既有共享测试helper只读，确需修改先取得具体文件排他锁。
- 冻结读语义：只在原已授权read Tx读取消息和关联，不跨事务补查／自开事务；所有附件仅输出 `{id,kind}`，无附件为 `[]`，稳定顺序且不泄露存储键／物理路径／摘要或秘密。跨Bot／坏kind／损坏关系整页失败关闭，不返回部分正文；不能用ReadyTx的ready过滤悄悄删引用，缺物理文件仍保留原引用，下载404与健康／恢复行为由对应后包验收。授权／实例最终失效零正文、零续期，维持no-store及事务内可序列化验证。
- 真实验收：生产TLS／router＋SQLite先真实上传出ready文件、既有文本创建HTTP生成消息，再以可信Tx测试夹具建立关联，真实Android列表／详情HTTP读出非空附件；覆盖同Bot多附件稳定序、分页／原回复／空附件混合、disabled Bot、删消息、缺物理文件、跨Bot及坏kind关系、权限／实例最终失效和秘密边界。关系夹具仅证明读side，不冒充生产绑定写HTTP。真实旧11→12迁移保已有上传、消息、read、legacy及完整push数据，验证显式FK、故障回滚及旧checksums；不重写旧SQL。
- 四类门槛与调度：①固定基线／范围／合同／文件及迁移号锁；②真实HTTP及迁移验收、相关messages／httpapi／cmd／attachments／sync／devices／push／auth／storage／changes普通与race、全服普通／vet／build、gofmt／diff，保旧门槛；③独立本地source提交后不同Agent固定SHA独审，编号修复重审；④准确canonical新并集I01及证据齐全后总调度决定done。Web／API未改核hash与真实embed复用，不冒称新跑npm／validator。READ-HTTP优先关键路径，两叶源码可在独立树无重叠路径并行，Go严格按唯一窗口串行；本叶不抢READ-HTTP的main／router／read.go，SYNC仍todo排main。
- 父范围保留：只补当前关系读投影；父BE04全部原验收及最终组合不裁，CAS／Buttons／BE05生产绑定写／保护引用／下载／GC、同步／推送READ过滤和global门槛仍按原任务独立验收，未实现不记通过。不push／部署／创建registry、报告或完成marker。

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
- 依赖：[已完成 BE-02 限定父组合](COMPLETED.md#g5-be02-compose)、`BE-05-02`。
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

已完成既定限定上传父范围：[原验收映射、外URL不抓取R01、不同A01及准确Parent-I01](COMPLETED.md#g5-be05-compose)。STORAGE／HTTP及父组合分别闭环；消息关联、下载、后台恢复／回收、设置API、真机部署及global门槛仍未由此验收，不作为本父循环下游依赖。

<a id="be-05-01-storage"></a>

### BE-05-01-STORAGE 流式持久化状态机准备

已完成子项：[范围、完整验收与独审／精确集成证据](COMPLETED.md#g5-be05-storage)。仅存储准备完成，父BE-05-01限定上传范围也已另行完成；0008 永久占用、写锁已释放，下一编号 0009 须另核。

<a id="be-05-01-http"></a>

### BE-05-01-HTTP 真实上传接线与组合验收

已完成有限叶：[完整源链、不同A01、准确a647新组合I01、真实测试计数与范围限制](COMPLETED.md#g5-be05-http)。只完成真实上传HTTP接线及其组合，[父BE-05-01限定上传组合已完成](COMPLETED.md#g5-be05-compose)，不含下载、绑定、回收、设置HTTP、部署或全局门槛。

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

已完成限定服务器父范围：[原完整验收、R01失败／修正／补证、不同A01及准确新组合I01](COMPLETED.md#g5-be10-compose)。只含服务器注册／偏好／条件解绑及实例边界，发送调度、FCM／OPPO资格与真机、Android正式通知和全局实例门禁未由此完成。

<a id="be-10-01-storage"></a>

### BE-10-01-STORAGE 注册与通知偏好存储准备

已完成子项：[范围、原验收、A01／F01／A02 及 I01 精确组合复验](COMPLETED.md#g5-be10-storage)。存储准备与下方有限 HTTP 各已完成，限定服务器父范围也已另经独立组合完成；0009 永久占用，本包写锁已释放，后续迁移须重新核号。

<a id="be-10-01-http"></a>

### BE-10-01-HTTP 当前设备注册与偏好真实接线

已完成有限叶：[完整源链、A01 两编号／修复／A02 PASS、四提交映射、准确 I01 新组合与精确集成证据](COMPLETED.md#g5-be10-http)。只完成当前设备五 operations，其限定父组合已另行完成；不含发送适配器、FCM／OPPO真机、Android正式通知及全局门禁；原父验收保留，其他任务不自动 done。

<a id="be-10-02"></a>

### BE-10-02 Hub 推送待办合并与调度

- 证据与交接：总调度采纳有限拆分，首叶 [PENDING-READ](COMPLETED.md#g5-be10-pending-read) 已done；[JOB-STORAGE](COMPLETED.md#g5-be10-job-storage) 兼容模型／内部预留亦已done、0011永久占用；[SYNC-CREATE-HTTP](#be-10-02-sync-create-http)仅todo／待main锁、暂不开工；完整调度／发送等父范围仍未完成。父原完整范围、依赖和验收保留。首叶已完成源检查／不同独审及准确新组合I01，不将父包或未来发送能力标完成。

- 旧ID：`P09-02`；类型：有限分拆收口（不整组派发）；状态：`running`（首叶PENDING-READ有限legacy只读已done，其余父验收仍未完成，父完整运行时尚未完成）。
- 依赖：[已完成 BE-10-01 限定服务器父组合](COMPLETED.md#g5-be10-compose)、[已完成基线 P05-01](COMPLETED.md#g2)。[READ-STORAGE事务仓储已独审／精确集成完成](COMPLETED.md#g5-be04-read-storage)，实际共享已读HTTP／策略过滤仍未完成，后包依赖须按真实范围具体核实；此前置完成不自动使整包 ready。
- 允许范围：`server/internal/push/**`，及对应测试。
- 门槛：`G-GIT`；条件未实证不记通过。

消费消息事务待办，按设备合并水位、退避/期限/共享已读过滤；用途sync/alert分开、job记录会话/注册/配置版本、稳定提醒键和期限，水位合并不使旧提醒复活。重试前复核当前DND/已读/配置/注册/期限，模式或注册改变隔离旧job，同设备租约及条件状态防双worker/迟到结果污染。平台受理不是展示或已读ACK，超时为结果不确定，不自动vendor→FCM补响。

先用假adapter验证框架/FCM路径，OPPO外部审核不阻该有限任务；假结果不算厂商真机通过。PUSH-D2/D3已批准切换仅未来新事件、外发在途残余和离线展示限制；本地fencing不能撤回已外发，不以批准替代P02-05验证。发送失败不回滚消息，推送耗尽不进业务failed、不保护普通历史，DND/全部已读任务不复活。

<a id="be-10-02-pending-read"></a>

#### BE-10-02-PENDING-READ 现有同步待办可信事务只读投影与水位归并

已完成有限legacy只读叶：[完整源链、不同A01及准确745d新组合I01、历史失败与范围限制](COMPLETED.md#g5-be10-pending-read)。仅可信Tx内legacy同步hint归并，输出有界、扫描未界定；不含完整snapshot／claim／策略／生产HTTP非零targets或发送，不完成父BE-10-02。未来JOB0011 mixed legacy＋sync＋alert兼容仍待准确I01实际验证。

<a id="be-10-02-job-storage"></a>

#### BE-10-02-JOB-STORAGE 兼容持久模型与原子内部租约预留

已完成有限兼容持久模型＋内部租约叶：[完整34b→74f→c145、A01需修／F01／不同A02关闭编号及准确0e98混合I01](COMPLETED.md#g5-be10-job-storage)。0011首次准确canonical集成并永久占用，旧SQL1～10不变、实施／集成锁已释放。内部预留非最终发送资格，不完成生产HTTP非零接线、策略／worker／平台或父BE-10-02；真实mixed legacy＋sync＋alert已按实际API组合验证。

<a id="be-10-02-sync-create-http"></a>

#### BE-10-02-SYNC-CREATE-HTTP 既有双入口首次创建事务生成完整sync待办

- 类型：有限后包；状态：`todo`／待共享锁（2026-10-29，总调度采纳只读Planner `/root/backend_restore_retry` 建议，**暂不开工、非ready**）。`efe4226512f214bffd6200f4bff7b6d777751d08` 仅规划参考，不是已冻结实施基线。产品依赖已具备：[服务器注册／偏好](COMPLETED.md#g5-be10-compose)、既有消息／幂等授权与引用、[期限F01](COMPLETED.md#g5-be10-preflight-f01)、[READ](COMPLETED.md#g5-be04-read-storage)、[PENDING](COMPLETED.md#g5-be10-pending-read)、[JOB／0011](COMPLETED.md#g5-be10-job-storage)均已完成；不存在需求层HISTORY功能依赖。HISTORY已准确集成于3e2，下一公共main／router锁由总调度保留给READ-HTTP；本叶仍todo排队，执行须待共享main锁移交，届时核真实canonical重新固定最终实施SHA、私树／owner及契约锁，不能拿规划参考SHA直接开工。尚未派Implementer，源码／Go／独审／I01全部NOT_RUN。
- 单目标与四主文件：仅 `server/internal/messages/repository.go`（初始化时可信首次创建Tx回调配置）、`server/internal/messages/create.go`（幂等首次成功创建内调用）、新增 `server/internal/push/sync_enqueue.go`（最小事件输入／同Tx选目标与完整sync快照）、`server/cmd/relaynest/main.go`（composition-root接线）；必要新同包测试限messages首次创建／push快照／cmd真实HTTP组合。main与messages内部回调契约须调度排他登记；不改router、SQL、API、auth／devices实现或其他叶源码。main桥接Created到最小事件，messages／httpapi不直接import push，避免push包内测试→messages／auth→httpapi的测试导入环；不是新增服务分层或外部接口。
- 冻结事务顺序：保当前入口认证／事务授权→基本校验和原幂等摘要→receipt查重；hit原样复用，**不选择目标、不写／刷新job**。仅Bot API／Android管理员首次文本业务及引用成功，bot_seq／message／changes完成、管理员delivery已追加后，在同一可信Tx回调使用最终 `Created.LastChangeSeq`，不误用较早message-created水位；随后保存原receipt并提交，提交成功才响应。回调不自开／提交Tx、不网络发送；任一选择／第N个job／幂等／SQL／COMMIT失败全回滚，无部分job或回执。不得在回调前跨事务枚举目标，不把当前注册／配置补到旧事件或幂等重放；不扩Webhook、附件、按钮或编辑生产者。两HTTP现有nil PushTargets可保留，不借此走legacy hint旧链。
- 快照与目标：同Tx从真实有效Android session/device、未撤销／idle及absolute纳秒期限、管理员代次和有效FCM注册选目标，同device多session隔离。固定purpose=sync／provider=fcm／alert_eligible=false；用原事件messageID／BotID／BotSeq／最终水位／创建时间，真实deployment、session代次、registration_version和settings_version（缺配置行虚拟0），按TECH9.1既有24小时默认期限校验无溢出，调用已完成CreateJobTx复核。无有效FCM注册正常消息成功但零完整job，不补造版本或恢复旧事件，数据仍前台／周期补偿；none／vendor／DND不变为本叶alert资格策略，sync不提醒。不得持久化token、正文、附件URL／callback或秘密，不在响应／日志／原receipt加内部资格数据。分页仅控制内存，不套legacy100上限静默丢目标，需验证目标覆盖及事务耗时。
- 验收与检查：真实生产双入口TLS／HTTP＋SQLite＋当前注册接口产生非零完整sync job；覆盖无／仅OPPO注册、settings0／版本快照、多session、幂等／12并发同键／丢响应／删源重放不多写、轮换后旧键不补job、错误／撤销／期限／代次／实例／秘密边界全回滚；原legacy／完整sync／alert隔离保持。相关push／messages／auth／devices／storage／changes普通与race、全服普通／vet／build、gofmt／diff及响应合同核验须准确新组合执行，保原50ms／12／10ms／超时和断言；Web／API不变核hash准确复用，不机械重跑npm／Python。完整本地提交后不同Agent固定SHA独审及精确I01，不能用native仓储或桩代真实HTTP证明。
- 范围与迁移：无新DDL，**不分配／预占0012**，0009～0011永久占用不改；属既有授权内普通内部实现，未新增外部接口／需求／架构变更，无额外用户批准步骤。当前仅规划登记，后包开工等待共享锁而非询问用户。提醒原事件资格／DND-READ策略、attempt／retry／late-result、worker／adapter／平台发送、FCM／OPPO资格／真机／全局门槛及父BE-10-02仍未完成。

<a id="be10-preflight-a01"></a>

#### BE10-PREFLIGHT-A01 非零同步待办的真实会话期限预检

原CHANGES_REQUIRED及发现 **BE10-PREFLIGHT-A01-01** 已经F01、不同A02及准确I01闭环，编号关闭；[原四过期FAIL、harness失败、真实六例控制与完整关闭证据](COMPLETED.md#g5-be10-preflight-f01)保留。HTTP仍nil targets，无平台外发验收。

<a id="be10-preflight-f01"></a>

#### BE10-PREFLIGHT-F01 修复非零待办的会话期限单位

已完成有限修复叶：[完整源链、A02与准确20f新组合I01、编号关闭及范围限制](COMPLETED.md#g5-be10-preflight-f01)。root已准确集成，后包可消费修复；不完成生产HTTP非零接线、BE-10-02完整调度或平台发送。

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
