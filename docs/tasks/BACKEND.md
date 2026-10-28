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

- 证据与交接：HISTORY-STORAGE、REFERENCE-CREATE、READ-STORAGE 已各独审／精确集成，见下方完成链接；[HISTORY-HTTP](#be-04-01-history-http) 已正式 running，负责既定历史／详情两 GET；共享已读 PUT HTTP、附件关联投影及父最终组合仍另包，父原目标和验收不变。

- 旧ID：`P05-03`；类型：有限 leaf；状态：`running`（2026-10-27，首叶已完成，父原验收未完成）。
- 依赖：[已完成 BE-02 限定父组合](COMPLETED.md#g5-be02-compose)。
- 允许范围：`server/internal/messages/**`、`server/internal/sync/read*`、`server/migrations/**`，及对应测试。
- 门槛：`G-GIT`；条件未实证不记通过。

目标：按bot_seq分页、引用校验、共享已读取max和未读计数；自己的消息与编辑不增未读，旧设备不能倒退。

验收：越 Bot 引用拒绝。；旧设备不倒退。；自己的消息与编辑不增加未读。 运行上述允许包的共同检查。

<a id="be-04-01-history-storage"></a>

#### BE-04-01-HISTORY-STORAGE 历史／详情与同 Bot 引用只读仓储准备

已完成有限子叶：[完整 source 链、实际检查、A01／I01 映射及范围限制](COMPLETED.md#g5-be04-history-storage)。仅仓储准备已集成；父 BE-04-01 仍 `running`、未完成原历史／引用／共享已读验收，REFERENCE-CREATE 已另独审／精确集成，READ-STORAGE已另完成，[HISTORY-HTTP](#be-04-01-history-http) 已另 running；本仓储叶不增加共享已读／附件投影／公开 cursor／HTTP。0009 保留，0010 已另专属 READ-STORAGE，本 HISTORY 子叶未分配新号。

<a id="be-04-01-reference-create"></a>

#### BE-04-01-REFERENCE-CREATE 既有双入口文本创建接通同 Bot 回复引用

已完成有限叶：[完整源链、A01／精确I01映射、新HTTP并集实际检查及COMMIT配对限制](COMPLETED.md#g5-be04-reference-create)。只完成既有 Bot API／Android 文本引用；父 BE-04-01 仍 running，READ-STORAGE已另完成，[HISTORY-HTTP](#be-04-01-history-http) 已另 running，共享已读HTTP、附件投影和父最终组合未完成，Webhook引用／附件／按钮不开放。

<a id="be-04-01-read-storage"></a>

#### BE-04-01-READ-STORAGE 共享已读事务仓储与原子初始化

已完成有限叶：[完整源链、A01及准确新并集I01、迁移占用与范围限制](COMPLETED.md#g5-be04-read-storage)。仅可信事务仓储／初始化完成，0010正式永久占用、实施／集成锁已释放；父BE-04-01仍running，[HISTORY-HTTP](#be-04-01-history-http) 已另 running，共享已读HTTP、同步投影／推送过滤及最终组合未完成。

<a id="be-04-01-history-http"></a>

#### BE-04-01-HISTORY-HTTP Android 历史分页与详情真实 HTTP

- 类型：有限子叶；状态：`running`（2026-10-28，原ready → running完成源提交后，经不同A01 CHANGES_REQUIRED退回有限F01修复；父 BE-04-01 仍 `running`，原验收不变）。owner Backend Orchestrator；准备角色 `/root/history_http_prepare`（Backend Planner／文档 Integrator），本轮仅元数据登记；Implementer `/root/history_http_implementer`，真实隔离树 `.worktrees/backend-be04-history-http`／`impl/backend-be04-history-http-20261028` 从下述 canonical 基线 clean 起步。固定 canonical 产品基线 `613e79e838539154a91df1625ed4ab479095feba`，目标 `resume/backend-20261026`。准备树 `.worktrees/backend-be04-history-http-docs`／`docs/backend-be04-history-http-ready-20261028`；实际实施基线同为 canonical `613e79e…`，root `356ff7801285d3aaae4767e02fb775e70b956ab4` 仅为其纯文档后继，不把文档提交冒充产品变更。
- 恢复实测：首次最小检查 `/tmp/relaynest-history-http-targeted-first.log`（SHA256 `6d32733a09bdf16548c0ebdd1d84058dc41cc22054aed81353f65f3f14654864`）中cmd两组QueryAndCredentials／AuthorizationInvalidation **FAIL**（包8.980秒），httpapi **PASS**（0.007秒）、messages **PASS**（0.743秒）；原日志保留，首次完整命令／start-end／退出码及受测输入pre/post未记录部分为 **NOT_RECORDED**，不补造。Implementer在最小授权窗口仅改新增cmd fixture，对照既有门禁独立保留missing头400 `INSTANCE_REQUIRED`／显式空头400 `INVALID_REQUEST`；instance／before既有503并独立重开Store观察，final仍保留409／无正文／不续期断言。Go1.27.3最小复验 **PASS**（exit0，13顶层：cmd4含24失效子项／httpapi2／messages7），实际命令 `go test ./cmd/relaynest ./internal/httpapi ./internal/messages -run '^(TestAssembledHistory|TestHistory)' -count=1 -v`；start `2026-10-28T20:27:43.695457688+08`、end `2026-10-28T20:27:54.330840421+08`，证据 `/tmp/relaynest-history-http-retest-20261028/` 下command-argv.json、pre/post、hash及log，log SHA256 `73398032f852efd5abf0de636bec12eec3d36412242830e6726b68186974e536`。该最小复验时其余source／runtime未改、六路径WIP未提交，全服／race／vet／build／commit／A01／I01当时 **NOT_RUN**；后续源提交／矩阵及A01需修见下，不能用最小PASS代替正式验收。
- 源提交／A01／有限F01：完整源链 `613e79e838539154a91df1625ed4ab479095feba → b5cfe8b90b1e1f03d7b6b8e4ea250e5750e89fac` 全CI **PASS**，证据 `/tmp/relaynest-history-http-full-20261028/handoff.json` SHA256 `2cee6629d527f719bdcd59917548027615b82a41483f3902d8a24aa1c5888978`；不同HISTORY-A01实际 **CHANGES_REQUIRED**（P2），编号 `BE04-HISTORY-A01-01`，位置 `server/internal/httpapi/message_history.go:193`及65／90：DB created_at=`9999-12-31T23:59:59-08:00`转UTC后year10000，事务内未验证JSON可序列化，事务后botJSON忽略Encode错误，真实TLS／nativeRead复现两GET返回200空body／no-store且idle_expires_at增加（exit1、两子例FAIL）。证据 `/tmp/relaynest-history-http-a01/{stage.json,run.log}`，log SHA256 `d8665da56046fdc53e42bf3aa68a3b48ebc5cf25795ed1ed11bea6237d554b81`。预期503 `SERVICE_UNAVAILABLE`、整页／详情不发布且续期回滚。不同Fixer `/root/history_http_fixer` 从固定b5cfe8新建 `.worktrees/backend-be04-history-http-f01`，仅BE04-HISTORY-F01修此编号的HTTP投影及必要httpapi／cmd history测试，不动全局bots.go／auth／main／router／SQL／API；三路径补丁尚未提交。最小9顶层／62全例 **PASS**（exit0／13.938秒，含原Auditor原样overlay复验、两字段UTC上下边界／整页坏行／idle零变化），246输入前后同；证据 `/tmp/relaynest-history-http-f01-min-20261028/stage.json`，log SHA256 `009fd803e1fd82bb12330494899fef480643610aa8585d94102b6ec8fdc535de`。F01全矩阵／commit／A02／I01 **NOT_RUN**；原source全CI PASS及首次fixture FAIL历史保留，本叶仍running，修复后须不同A02及准确I01。
- 已集成依赖：[BE-01](COMPLETED.md#g5-be01)、[BE-02限定父组合](COMPLETED.md#g5-be02-compose)、[HISTORY-STORAGE](COMPLETED.md#g5-be04-history-storage)（最终映射 `6e5ca4b…`）、[REFERENCE-CREATE](COMPLETED.md#g5-be04-reference-create)（`b5fa149…`／准确组合 `74e07f6…`）。[READ-STORAGE](COMPLETED.md#g5-be04-read-storage)（`0a93d6e…`／准确组合 `43e9d63…`）亦已集成，但本历史叶不依赖共享已读 HTTP；上传 HTTP 已准确集成于 canonical 基线，不消费 PENDING／JOB 私树 WIP。
- 冻结合同：以 `server/api/openapi.yaml` 的 `listClientMessages`、`getClientMessage`、`MessagePage`、`Message` 及 `x-instance-binding` 为准，结合 PRD §3.4／§4.1／§4.2、技术设计 §3／§6。仅实现 `GET /client/bots/{id}/messages` 与 `GET /client/messages/{id}`；不含共享已读 PUT、同步／bootstrap、WS、消息编辑或附件绑定。Android Bearer＋实例头及会话私有绑定；Web Cookie、Bot API Token、Webhook Key 均不可获得正文。停用 Bot 的仍存历史可读；不存在／过期／无权统一既定404，不新设路由或错误码。
- 允许主文件（4～5）：新增 `server/internal/httpapi/message_history.go`，可选新增 `server/internal/httpapi/message_history_cursor.go`；仅扩展 `server/internal/messages/history.go` 的可信 read tx 详情 helper，按 messageID 在同一事务解析 Bot 后复用 `GetHistoryMessageTx`；`server/internal/httpapi/router.go`；`server/cmd/relaynest/main.go` 最小装配。必要测试限新增 `server/internal/httpapi/message_history*_test.go`、`server/cmd/relaynest/message_history_test.go` 和 `server/internal/messages/history_http_tx_test.go`；不改旧 main_test、auth 生产代码、SQL、API、依赖或其他模块。实际实现如需超出此范围，先报告证据重新分包。
- 必要授权接线：canonical 的 `clientMessageAuthorization` 当前忽略 read/write、固定 `RequestWrite` 且仅提供 Write 闭包；本叶须最小支持真实 `RequestRead → WithAuthorizedRead`，保留 Android 类型和实例安全入口及原 Write 语义。不得复用允许 Web 的管理端授权作为 Android 历史入口。列表、详情全部在私有真实会话绑定的可信读事务内获取，不能先跨事务查 Bot／正文再拼接授权；详情只接受 messageID，不加公开 bot 选择器。候选响应仅在授权读闭包整体成功后发布，撤销／过期／认证代次／实例变化或最终失败不得泄漏正文、续期或产生业务副作用。
- 分页与投影验收：`bot_seq DESC`、排他边界、有界 limit＋1、不用 offset；cursor 不透明并绑定 Bot，错 Bot 拒绝，末页 `next_cursor=null` 且 `has_more=false`，编辑不改排序。cursor 编码和不超过仓储200的实际页限为内部实现细节，不引入新签名 key、TTL、同步 H／epoch 或 bootstrap 令牌。冻结 Message 必填字段和类型完整投影，保留已有 `ButtonsJSON`；引用用 originalID，源删除仍保留原 ID且不返回源正文，已删详情404。附件关联投影尚未实现，不能冒称本叶完成实际关联附件显示或擅定新显示策略；如现有模型不足以兑现当前合同，先报告冲突。
- 真实验收：生产 TLS／HTTP＋SQLite 接线、既有创建及同 Bot 引用后列表／详情；跨 Bot cursor、空页／页限／末页／删除分页边界后继续、并发新增及编辑不乱序、原引用保留／源删详情404；Android／Web／Bot／Webhook、停用 Bot及当前授权失效矩阵。受保护响应固定 `no-store`，实际 JSON 对照冻结 OAS（含序列／版本字符串、UTC时间、必填数组及 null）；错误和日志不得泄漏正文、会话／实例秘密或 SQL。
- 锁与检查：准备仅写 BACKEND／PROGRESS及准备树/index，root/index 排他锁在本登记完成后释放；公共 main／router 的实现写锁已由总调度仅授予 `/root/history_http_implementer`，连同其实现私树／index及本叶允许路径排他持有。无需迁移；0009／0010 永久原占用，0011 专属 JOB 保留。Go1.27.3 的 messages／httpapi／cmd／auth／storage／changes／bots 相关普通与 race、全服普通／vet／build、gofmt／diff、真实响应合同及精确 main／router 新并集检查须实施后执行；旧测试断言和超时不变，原Implementer已完成源提交／全CI；首次最小FAIL／局部PASS与fixture修正后复验PASS均如上保留；当前仅不同Fixer持有限F01三路径补丁锁，main／router不在F01写范围，F01完整矩阵按总调度窗口执行，不抢跑。Web／API未改时核锁版输入及真实 embed hash复用，不机械重跑 npm／validator；源单独本地提交后由不同 Agent 固定 SHA 独审及准确 I01，记录命令／hash／退出码，区分 PASS／FAIL／NOT_RUN。
- 本轮限制：只核 Git／文档／冻结合同与既有最小授权接线，无源码修改、Go／npm／validator或新技术 PASS；不创建 registry／cards／reports、不 push／部署／发布／创建完成标记。历史 HTTP、共享已读 HTTP、附件关联投影和父最终组合在真实完成前均保持未完成。

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

- 证据与交接：总调度采纳有限拆分，首叶 [PENDING-READ](COMPLETED.md#g5-be10-pending-read) 已done；新增 [JOB-STORAGE](#be-10-02-job-storage) 已running、0011正式独占分配，独立实施中。父原完整范围、依赖和验收保留。首叶已完成源检查／不同独审及准确新组合I01，不将父包或未来发送能力标完成。

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

- 类型：有限子叶；状态：`integrating`（2026-10-28，原review经A01需修退running，现F01源提交／不同A02 PASS，转准确I01，原 `/root/job_storage_implementer` 及fresh接替者因模型capacity失败已撤写权，原 `/root/read_storage_auditor` 仅任本叶Implementer、不得审自己JOB，固定真实起步 `34b21671a26bcccbe1154b167947dfb4f0ed23fa`）。原产品基线 `d4d942e615736bb3aaed0c4f4d14a661f4231994` 后仅元数据推进；仅四主文件、允许tests及三处计数fixture，0011锁归本叶；同原私树已准备四主文件＋三新测试＋三处计数fixture，旧1～10字节不变，gofmt／diff静态PASS、已正式源提交。最小Job tests实际 **PASS**（exit0／3.450秒，12顶层＋2迁移子例，157输入前后不变），证据 `/tmp/relaynest-be10-job-resume-20261028/{minimal-01.json,minimal-01.log}`；完整源链 `34b21671a26bcccbe1154b167947dfb4f0ed23fa → 74f69ea872425cd863fe466132b9239103d225f0` 单提交十路径（四主／三测试／三计数fixture），工作树clean、旧SQL1～10字节不变。157输入当前与受测前后hash一致，源全服普通 **PASS**（165.197秒）、九包race **PASS**（515.921秒）、全服vet／build **PASS**（2.701／1.560秒），均exit0；handoff `/tmp/relaynest-be10-job-resume-20261028/handoff.json` SHA256 `643bfb6c4e6285465787b8e3f015dc840f2ff478891c4b8ea1767bb663d34b38`。独立JOB-A01对固定完整源链判定 **CHANGES_REQUIRED**，发现 `BE10-JOB-A01-01`；原源全检查PASS保留但不等于当时独审PASS；现F01及A02如下，精确I01 **NOT_RUN**，0011锁继续保留，不整派父BE-10-02。
- A01／有限F01：`BE10-JOB-A01-01` 位于 `0011_push_schedule.sql:64–75`，无条件immutable触发器强化legacy的id／device／session／target旧列约束。独立Python SQLite3.46.1以真实0006／0011 DDL及deferFK事务更新device_id对照：旧表接受，升级后在读取前报 `immutable push job snapshot`，使已审PENDING `bc71696224caf8aabb3c879f9d589abec5b79f7f` 的 `TestPendingBindingMismatchSameTransaction` 提前失败，违反保留旧列约束语义。证据 `/tmp/relaynest-be10-job-a01-20261028/legacy-trigger-repro.json`；被审树固定74f69且clean，独审未跑Go。不同Fixer `/root/job_storage_fixer` 从固定74f69准备新私树，仅授权 `BE10-JOB-F01` 处理此编号，范围限0011及必要job测试；完整job不可变与跨类别禁令不得弱化，0001～0010不改，0011仍本叶独占且未canonical可修，不分配新号。F01正式源提交 `c1459e72ce2fef3be32ab5b12beb903d3edef9e9`，完整链 `34b21671… → 74f69ea… → c1459e7…`，补丁仅两路径。新源全服普通164.554秒／九包race517.166秒／vet2.738秒／build1.616秒及已验收canonical四文件overlay push普通5.070秒／race31.453秒均 **PASS**；handoff `/tmp/relaynest-be10-job-f01-20261028/handoff.json` SHA256 `497f85f8fe7b3988a78961e6a82e98c627c12634caf684bb4c3a783a11f60122`。不同JOB-A02对完整链 **PASS**、关闭编号-01：旧schema→原0011→修后SQL对照accepted→error→accepted；Job14独立3.430秒／Pending6独立5.850秒PASS，完整sync／alert冻结及双向类转换均有原触发器真实断言；handoff `/tmp/relaynest-be10-job-a02-20261028/handoff.json` SHA256 `7eadcd5ca2661c19b04efdfda15b0aab52bcaf230b2cd6139afc45702e5ab6ff`。原A01 FAIL完整保留；现Integrator `/root/backend_restore_retry` 从准确root `89e72c5c5a2a152338a5336b291108130f201edf` 在 `.worktrees/backend-be10-job-storage-i01`／`integrate/backend-be10-job-storage-i01-20261028` 无冲突完整应用74f及c145两源，最终十路径同c145，其余target含已验收PENDING／READ／F01／UPLOAD／REF及原SQL1～10／Web／API／依赖保留，无HISTORY私树源码。0011本叶首次进入候选、未另号。仅临时overlay新增真实同会话legacy＋完整sync＋alert并存，实际ReadPendingTx仅legacy、CreateJobTx／Reserve仅完整快照及旧五列writer继续可用的兼容检查；连同原Job14／Pending6、真实旧10→11迁移／故障回滚reopen、全服普通、九包race／vet／build／格式均待冻结后执行，全部PASS前不ff root／不done。证据 `/tmp/relaynest-be10-job-storage-i01-20261028`；0011排他锁保留。
- 已集成依赖：[BE-10-01限定服务器父组合](COMPLETED.md#g5-be10-compose)、[P05-01消息事务与待办仓储](COMPLETED.md#g2)、[认证／设备基础](COMPLETED.md#g1)、[READ-STORAGE及0010准确I01](COMPLETED.md#g5-be04-read-storage)和既有storage。root已包含0010，不得回到缺0010的旧基线写0011。沿现表兼容扩展、保留旧hint且禁止补造提醒资格，属于落实既有冻结合同的内部实施；不新增公开接口或架构分层。
- **迁移排他锁**：总调度2026-10-28正式独占分配 **`0011_push_schedule.sql` → BE-10-02-JOB-STORAGE**，从本登记起持续至准确独审／精确I01核收；其他writer不得占用该号。分配前只读实核root为上述d4基线，根common库51 refs／29注册树、backend-local common库6 refs／3注册树，共57 refs／32树；refs最高分别0010／0009，无0011＋，全部迁移目录无dirty。随后仅新建本叶docs隔离树。0009、0010永久占用；不得修改0001～0010或因旧恢复记录重新分配它们。
- 单目标与四主文件：只允许新增 `server/migrations/0011_push_schedule.sql`、`server/internal/push/job_models.go`、`server/internal/push/job_repository.go`、`server/internal/push/job_claim.go`。沿既有push_jobs兼容扩展／重建持久字段，提供有界、确定顺序的原子内部租约预留、条件释放及过期接管；预留不是最终发送／提醒资格，不实现实际发送次数扣减、退避或发送结果完整状态机。
- 必要测试允许路径：新建 `server/internal/push/job_models_test.go`、`server/internal/push/job_repository_test.go`、`server/internal/push/job_claim_test.go`；旧迁移总数fixture仅允许最小更新 `server/internal/attachments/schema_test.go`、`server/internal/devices/push_repository_test.go`、`server/internal/sync/read_migration_test.go` 当前最新总数10→11，保留旧八／九迁移构造、checksum与全部原断言。storage动态计数fixture无需修改；不得据计数授权扩大实现范围。
- legacy与状态隔离：保留旧五列INSERT、原列名／类型语义及session/device级联；旧行和旧writer新增行继续默认 `state='pending'`，pending专属快照未齐的legacy hint，仅可只读、不得发送型预留。完整sync／alert仅使用queued／leased等独立状态集合；新CHECK及不可跨类别转换约束绑定状态和快照完整性，重试／释放／接管不能回pending，完整job不能降回legacy，legacy不能读取当前配置／注册／水位补造原事件资格。不得清理历史或新增伴生表。
- 冻结字段与期限：完整快照遵循OpenAPI `x-notification-job-policy`／`NotificationJobSnapshot`及TECH §9.1／9.3，保存session/device、用途、目标水位、provider、注册／配置版本、deployment/message稳定提醒键、bot_id/bot_seq、expiry及alert资格；sync也保留关联稳定键且provider=fcm。settings_version允许虚拟默认0。原创建时间／expiry／稳定键和原事件资格不可因合并、重领或reopen重置；旧行缺可信时间保持legacy，不以升级时间冒充。target watermark不等于message.created序号，管理员路径可能为delivery追加后的最终change水位。
- 租约和秘密：同device跨session互斥、租约状态／摘要持久，原始租约秘密不持久化；短写事务条件预留，过期接管后旧lease释放／更新必须CAS no-op。nil／context取消／SQL／COMMIT失败不发布部分结果或留下部分修改。job不得持久化注册token、正文、附件URL、callback或凭据；租约秘密仅内部可信句柄使用，不能进入JSON、格式化日志或普通回执，网络等待不得持写事务。内部预留不得绕过后包发送前的会话／注册／配置／DND／READ／期限复核。
- PENDING兼容和符号锁：可只读消费canonical已验收[PENDING-READ](COMPLETED.md#g5-be10-pending-read)，不修改 `pending_models.go`／`pending_read.go`／`pending_read_test.go`。新生产符号及测试helper使用Job／job前缀，避开现有ErrPendingInput、ErrPendingData、MaxPendingSessions、PendingSession、PendingDevice、ReadPendingTx及pending测试helper。PENDING实际查询仅筛state=pending，故上述数据库状态隔离保证旧只读投影不吞新alert；无需等待PENDING完成才独立实施。其已准确集成于745d，JOB最终I01仍必须实测同会话legacy＋完整sync＋完整alert共存时旧投影仅归并legacy、旧五列INSERT仍可投影，不提前宣称组合通过。
- 来源风险及禁止范围：[F01](COMPLETED.md#g5-be10-preflight-f01)已独立A02／准确I01完成并集成root20f，后续真实生产者验收须消费该修复；JOB当前固定34b私树仍不含F01，不能以DDL代替真实组合验收。禁止修改producer、auth／devices／storage实现、HTTP/router/main、API／依赖／Web；不实现worker、策略过滤、adapter、平台发送、fallback、动态插件、清理任务，不触RelayShelf／MoviePilot。PENDING有限legacy只读叶已done，JOB及父调度保留其未完成阶段与真实FAIL，不因前置叶完成而自动done。
- 必须真实验证：旧库升级及故障原子回滚、0001～0010 checksum不变、旧五列INSERT／legacy不可领取、完整快照约束／版本0／不可变字段；双Store竞争、同device跨session互斥、不同device独立预留、到期接管／reopen恢复、旧lease CAS；nil／取消／SQL／COMMIT失败无部分发布及秘密边界。完整快照仓储fixture不冒称生产消息HTTP来源；PENDING已准确集成，实际混合投影组合仍待JOB准确I01。
- 必跑检查及收口：固定Go1.27.3，push／messages／devices／auth／storage／changes／sync／attachments相关普通与race、受影响vet、全服build、gofmt／diff；保留既有50ms／12并发及10ms门槛。所有Go按总调度授予重型窗口执行，PENDING源窗口已释放、F01-I01已完成，JOB完整源矩阵已PASS并释放窗口，HISTORY最小复验已PASS；JOB有限F01源完整矩阵及不同A02均已PASS，精确I01按总调度排队。Web／API未改时核锁版与真实embed准确复用，不机械重跑npm／validator。本地独立实现提交后交不同Agent固定SHA独审及准确组合I01，记录实际命令／hash／退出码并区分PASS／FAIL／NOT_RUN。本叶source已正式提交，原A01需修已由F01／A02关闭、准确I01未完成、父BE-10-02仍running且未ready／done，不完成生产接线、共享已读策略、BE-10-03或真机／跨端门槛；不push／部署／创建完成标记。

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
