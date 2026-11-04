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

已完成既定后端父组合：[BE-04-01 历史分页引用和共享已读](COMPLETED.md#g5-be04-compose)。子项、原目标／验收、父R01首轮fixture FAIL与修正、不同A01及准确e8产品适用性I01完整归档；Android应用／WS／global及其他独立后包仍未据此完成。

<a id="be-04-01-history-storage"></a>
<a id="be-04-01-reference-create"></a>
<a id="be-04-01-read-storage"></a>
<a id="be-04-01-history-http"></a>
<a id="be-04-01-read-http"></a>
<a id="be-04-01-attachment-read"></a>

已完成子项：[历史仓储](COMPLETED.md#g5-be04-history-storage)、[文本回复引用](COMPLETED.md#g5-be04-reference-create)、[共享已读仓储](COMPLETED.md#g5-be04-read-storage)、[历史两GET](COMPLETED.md#g5-be04-history-http)、[共享已读PUT](COMPLETED.md#g5-be04-read-http)、[附件关系读投影](COMPLETED.md#g5-be04-attachment-read)。仅保留稳定链接，原完成证据和各自范围不重复维护。

<a id="be-04-02"></a>

### BE-04-02 应用消息 CAS 编辑

- 证据与交接：父[BE-04-01既定后端组合](COMPLETED.md#g5-be04-compose)已done，仅满足该依赖，本CAS仍todo／未ready。Planner已提出完整替换reply_to合同与original_reply SQL immutable约束可能冲突，尚待独立核实及必要语义决策；本轮不修改冻结接口／SQL、不据父完成自动开工。尚无本步骤新增实施／独审／集成证据，派发前仍须固定实际owner／基线／路径锁。 [CAS-TEXT-BUTTONS-STORAGE](#be-04-02-cas-text-buttons-storage)仅可信Tx内部文本／按钮CAS已另进入review／独审需修，完整HTTP与reply语义不由此放行。

- 旧ID：`P05-04`；类型：有限 leaf；状态：`todo`（父依赖已满足，完整编辑HTTP语义未闭环，非ready）。
- 依赖：[已完成BE-04-01既定后端父组合](COMPLETED.md#g5-be04-compose)。
- 允许范围：`server/internal/messages/**`、`server/migrations/**`，及对应测试。
- 门槛：`G-GIT`；条件未实证不记通过。

目标：只允许应用编辑自身应用消息；保原bot_seq/创建时间/已读语义，revision CAS；正文/按钮与changes同事务，附件引用在BE-05-02组合。Android及Webhook不可编辑。

验收：旧revision冲突。；Android及Webhook不可编辑。；文本/按钮与变更同事务；附件引用集成由 P05-06 收口验收。 运行上述允许包的共同检查。

<a id="be-04-02-cas-text-buttons-storage"></a>

#### BE-04-02-CAS-TEXT-BUTTONS-STORAGE 可信事务文本／按钮CAS原语

- 类型：有限子叶；状态：`integrating`／A03 **PASS**（2026-11-05，完整0bd3→0e23→c45→8a08固定链独审通过，A01／A02四旧编号均CLOSED；精确I01检查尚未运行，不能done；原ready→running／review及两轮CHANGES_REQUIRED历史保留）。owner Backend Orchestrator；固定已完成父的canonical产品基线 `2e1a62c6615148c80cb6e8e1c0b1a3cc77bd3225`（产品同e8／ad6ce），父依赖见[完成档案](COMPLETED.md#g5-be04-compose)。Implementer `/root/cas_storage_implementer` 从准确 `0bd3b543141792e8ca68b8dcf243f159ccf98a83` 在 `.worktrees/backend-be04-cas-storage`／`impl/backend-be04-cas-storage-20261030` 正式提交完整单链 **0bd3→0e23cde530b74791acb5aa2e7cacd77590439871**，恰六新文件（原三main＋三edit_*_test），私树clean且实施／Go锁已释放。179产品＋3真实embed与受测及commit逐字节一致；min01完整10顶层／64全例 **PASS**（5.882秒），log SHA256 `57d0d725efc27d125d7a63aead1fffb2b94643f3629ed286e2df15288c3e15aa`；全服普通353／1590 **PASS**（208.145秒，log `d23a0c3a6c4c909751b84b679df65afdd6d42e60ee716c66f6c3602db2f3f343`），五包messages／bots／changes／auth／storage race163／634 **PASS**（463.045秒，log `fee4d7022dc247ed2f0cbe051f2e2cc0ea3eda5b886486b40b53ef128ea4bc5a`），vet0.886秒／build26.040秒／产品fmt／diff均 **PASS**，本叶source首轮无FAIL。完整实施交接 `/tmp/relaynest-be04-cas-storage-20261030/handoff.json` SHA256 `d8409d580df27931d18c2016fbe20bce29585761d9e0ca88af3ff904d8dd59e4`；原freeze `draft-freeze.json` SHA256 `2a108670364861468b1b695517f069b6648208287a63ab4da981037e67dc12f2` 保留。上述仅source检查，不含后继SYNC精确并集。
- 独立A01：不同Agent `/root/cas_storage_auditor` 固定完整 **0bd3→0e23** 六路径审查，已回 **CHANGES_REQUIRED**（两项P2；独立13顶层／67全例中11／65 PASS、2项FAIL）；`BE04-CAS-STORAGE-A01-01`（`edit_tx.go:70–74`）：changes的BEFORE INSERT触发器`RAISE(IGNORE)`导致零插入，Edit却成功提交正文而无新change；`BE04-CAS-STORAGE-A01-02`（`edit_tx.go:44–48、84–85`）：持久original_reply为非法UUID时仍成功编辑，未fail closed。正式审计报告 `/tmp/relaynest-be04-cas-storage-a01-20261030/handoff.json` SHA256 `6180f9aa1c0cf0bfd78bf0bdcb24eb66a17a9ddd9702211d348ab1d30e6463ab`；独立Go已退出。原A01两编号及真实FAIL保留，原source全部PASS事实不删除；不同Fixer F1已从0e23处理并提交如下，原两编号经不同A02原字节复验关闭，但A02新发现仍需修复，不以F1复验代独审。原独立overlay须原字节重验，不改旧断言或以正常DDL阻挡坏库夹具代替复验。A01不是PASS，准确I01仍 **NOT_RUN**；尚未集成、未创建完成标记。
- 有限F1：不同Fixer `/root/cas_storage_fixer_f1` 在 `.worktrees/backend-be04-cas-storage-f1`／`fix/backend-be04-cas-storage-f1-20261030` 完成 **0e23→c45d634834f06532de7839c83674fb789ebfc031**，完整待审链为 **0bd3→0e23→c45d634**；仅修改 `edit_tx.go` 并新增 `edit_regression_test.go` 两路径，不改共享Append／SQL／API。180产品＋3真实embed与受测／提交逐字节一致；原A01 overlay原字节两FAIL在F1最小15顶层／76全例 **PASS**（6.461秒）中转PASS，原source64例保留。全服普通358／1602 **PASS**（210.411秒）、五包race168／646 **PASS**（471.191秒）、vet1.464秒／build2.222秒／产品fmt／diff均 **PASS**；原全矩阵在额度中断后自然完成，恢复只核日志／输入，没有重跑冒新实测。F1正式交接 `/tmp/relaynest-be04-cas-storage-f1-20261030/handoff.json` SHA256 `d41b6731f742b2aa7b598dc40f4ad3a9ba793b51f9e7f7b77748e6d397a67c44`；Go／私树写锁已释放。不同Auditor `/root/cas_storage_auditor_a02` 完整审查0bd3→0e23→c45，已回 **CHANGES_REQUIRED**：原15顶层／76全例PASS并关闭原A01两编号，新增2顶层／4全例真实FAIL。新P2 `BE04-CAS-STORAGE-A02-01`（`edit_tx.go:38–52`）为持久bot_seq／created_change_seq数字BLOB仍被接受；`BE04-CAS-STORAGE-A02-02`（`edit_tx.go:75–102`）为Append后changes触发器改写messages.text，返回输入候选却与持久内容不一致。正式A02报告 `/tmp/relaynest-be04-cas-storage-a02-20261104/handoff.json` SHA256 `4f45e6de248025428783db519156cec8c01990b96086e275292cd46eb40adb7d`，原A01两编号CLOSED、新增两编号仍待后续修复／重审。Fixer `/root/cas_storage_fixer_f1` 已接有限F2，从固定c45在 `.worktrees/backend-be04-cas-storage-f2`／`fix/backend-be04-cas-storage-f2-20261104` 静态准备，限本叶至多两主文件与必要新测试；此为F2派发时历史阶段；后续正式F2／A03及I01状态见下，不能据原阶段或本地PASS标done。
- 有限F2与最终A03：Fixer仅在 `edit_tx.go` 增加本消息／关联原始SQLite类型与编辑结果精确快照检查，并新增 `edit_snapshot_regression_test.go`；固定 **c45→8a08cd7bb9b6fb839b1775d348b637dc66019309**，完整本叶八文件链 **0bd3→0e23→c45→8a08**。min20／109 PASS8.276秒、全服普通363／1635 PASS209.373秒、原五包race173／679 PASS499.330秒、vet／build／fmt／diff全部PASS，181产品＋3真实embed与受测／提交一致，两原审计overlay逐字节保留，原76及新增109身份均通过；F2交接 `/tmp/relaynest-be04-cas-storage-f2-20261104/handoff.json` SHA256 `dea98a4351e7cd7e7ee6d0b95e882a4257308cf881dd4865481f583101111362`。不同Auditor完整A03 **PASS**、无新发现，独立22顶层／113全例PASS8.165秒并核原全矩阵；报告 `/tmp/relaynest-be04-cas-storage-a03-20261104/handoff.json` SHA256 `d7cc2a2614fbe554c9101e4943d40ae9fd9597678c8a74332635d9d6a246036d`；原A01／A02真实FAIL不改写。
- 准确I01：Integrator `/root/cas_storage_integrator` 从真实canonical **a4e81695de0c7af042d416e0ab2b53a99140dcce** 在 `.worktrees/backend-be04-cas-storage-i01`／`integrate/backend-be04-cas-storage-i01-20261104` 一次无冲突准确应用三源提交，映射0e23→**e20893e7f0f7c0e53c2a04ab5f635677132d4b4b**、c45→**70d227233727cf6fb4668f450dd1f4fb8e72fe04**、8a08→**98a4b36a126e1f4737a5eb0d5012e632975ea996**；八业务文件与最终受审源逐字节相同。目标额外含已done SYNC-CREATE七路径，不能冒称旧sourceCI已测最终并集；新TLS入队／nativeCAS／已读／不可变receipt-job-delivery组合、全服普通、原五包race、新cmd组合定向race、vet／build／格式当前全部 **NOT_RUN**，等待唯一Go窗口。完整HTTP／reply语义、未来附件绑定与BE04-02父仍未完成；不新DDL／迁移号，不push／部署／marker。
- 三个新主文件：仅 `server/internal/messages/edit_models.go`、`server/internal/messages/edit_validation.go`、`server/internal/messages/edit_tx.go`，必要新测试限 `server/internal/messages/edit_*_test.go`。只提供消费已有可信 `*sql.Tx` 的native文本／按钮CAS，不自开／提交事务、不持有Tx、不增加公开HTTP或外部错误码。既有messages模型／其他仓储／main／router／create.go／repository.go／auth／bots／changes／SQL／API／依赖均只读；不DDL、不占迁移号、不改reply或附件关系，必要额外路径先报告证据协调。
- 授权与事务：消费既有bots `Edit` 能力、真实身份及同Tx当前Bot／凭据代次／启用复核，只允许应用编辑本Bot自身application消息；Android／Webhook身份及管理员消息在本native入口拒绝，不冒称未开放的HTTP已验证。expected revision比较、正文／按钮既定限制校验、revision／updated_at和既有changes `MessageUpsert` 追加在同一可信Tx；提交前复核授权，错误完整传播由实际Tx拥有者回滚，候选只在最终提交成功后可发布。具体内部输入／结果与错误分类属实现细节，不在本叶擅定完整替换reply的HTTP语义。
- 不变量：只修改本叶文本／按钮相关持久内容及编辑版本／时间；保持bot_seq、created_at、原回复／original引用、附件关系、共享read状态／未读含义、原幂等回执及不可变delivery快照。不得触发首次create回调、首次推送待办、sync job、callback事件或网络发送，不执行业务命令。revision冲突、溢出、损坏／SQL／changes／COMMIT或最终授权失败都不部分写入／发事件，不回绕、不吞错误或重放业务callback／COMMIT。
- 必要验收：真实SQLite可信Tx成功文本／按钮CAS、旧revision冲突、越Bot／非application／Android／Webhook与失效凭据／最终授权拒绝；两Store同revision竞争仅一次成功，序号／创建时间／共享已读／计数及所有不可变引用／回执／delivery不变。覆盖版本边界及耗尽、校验失败、SQL与changes故障、延迟FK提交失败／重开观察全回滚；失败无部分候选发布。使用现有真实身份／事务能力，不凭可伪造公开Actor字段认授权，不用HTTP桩代native事务证明，也不声称完整CAS HTTP或未来回复／附件替换已验收。
- 四类门槛与锁：①固定父done后基线、三个新文件／自有tests及内部Tx契约；②实际相关messages／bots／changes／auth／storage普通与race、全服普通／vet／build、gofmt／diff和冻结输入核验，保原50ms／12并发／10ms／timeout与旧断言；③单独本地source提交后不同Agent固定完整SHA独审，发现编号修复重审；④准确canonical并集I01及证据齐全再由总调度决定本叶done。Web／API未改时核hash及真实embed精确复用，不冒称新npm／validator。仅这三个新文件和自有tests未来写锁留本叶，不抢SYNC的main／create／repository／push及回调契约；Go按全局唯一窗口串行。
- 原父边界：完整BE-04-02仍 `todo`／非ready，完整替换reply_to与original_reply SQL immutable语义待独立核实及必要用户决定；原目标、依赖、范围、旧revision冲突／Android及Webhook不可编辑／正文按钮与changes同事务和BE-05-02附件组合验收完整保留。本子叶不代父／HTTP／附件／投递保护／callback／Android应用／WS／平台或global完成；无push／部署／marker。

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

- 证据与交接：总调度采纳有限拆分，首叶 [PENDING-READ](COMPLETED.md#g5-be10-pending-read) 已done；[JOB-STORAGE](COMPLETED.md#g5-be10-job-storage) 兼容模型／内部预留亦已done、0011永久占用；[SYNC-CREATE-HTTP](COMPLETED.md#g5-be10-sync-create-http)真实生产者亦已done；[SYNC-LEASE-CHECK](#be-10-02-sync-lease-check)可信Tx只读复核已running；完整调度／发送等父范围仍未完成。父原完整范围、依赖和验收保留。首叶已完成源检查／不同独审及准确新组合I01，不将父包或未来发送能力标完成。

- 旧ID：`P09-02`；类型：有限分拆收口（不整组派发）；状态：`running`（首叶PENDING-READ有限legacy只读已done，其余父验收仍未完成，父完整运行时尚未完成）。
- 依赖：[已完成 BE-10-01 限定服务器父组合](COMPLETED.md#g5-be10-compose)、[已完成基线 P05-01](COMPLETED.md#g2)。[READ-STORAGE事务仓储已独审／精确集成完成](COMPLETED.md#g5-be04-read-storage)，实际共享已读HTTP已在[BE04父组合](COMPLETED.md#g5-be04-compose)完成；推送策略过滤仍未完成，后包依赖须按真实范围具体核实；此前置完成不自动使整包 ready。
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

已完成有限生产者叶：[完整f2→35e、不同A01及准确cfa新组合I01、全部失败／复验与范围](COMPLETED.md#g5-be10-sync-create-http)。两入口真实TLS／注册HTTP首次创建同事务形成完整sync快照，幂等命中不补写；0012新并集定向／全服普通／十一包race／vet／build全PASS。仅此叶done，策略／租约发送前复核／worker／adapter／平台与父BE10-02仍未完成，无DDL／迁移号占用。

<a id="be-10-02-sync-lease-check"></a>

#### BE-10-02-SYNC-LEASE-CHECK 已预留完整sync租约的可信事务只读复核

- 类型：有限子叶；状态：`review`／A01 **CHANGES_REQUIRED**（2026-11-04，原ready→running后完整source已提交，F1已派准备、修复尚未提交）。owner Backend Orchestrator；固定完成canonical产品基线 `cfa65a4d8eff8a99219ef7489d7e649c03f1ac2c`，后继归档仅docs、产品相同；Implementer `/root/sync_lease_implementer` 从准确 **5c4059b8db03793d32184d2e1f6369e1aa7e29e0** 在 `.worktrees/backend-be10-sync-lease-check`／`impl/backend-be10-sync-lease-check-20261030` 实施。依赖[JOB内部预留](COMPLETED.md#g5-be10-job-storage)、[真实双入口非零sync生产者](COMPLETED.md#g5-be10-sync-create-http)、现有认证／注册与期限修复均已准确集成。恰六新文件（原三main＋同包二test＋新cmd test），183产品＋3真实embed草稿冻结；min01／min02新夹具 **FAIL**及原字节／日志均保留，仅修新夹具后min03完整9顶层／35全例 **PASS**（5.108秒），log SHA256 `ee27570a136074845f299ca342e8d3da470786988757f0db16b3359a6b2f092a`；freeze `/tmp/relaynest-be10-sync-lease-check-20261030/draft-freeze.json` SHA256 `39132f3c2350d959a09d5d9723c60243707ed997053e7f99ba6eb8e4d7ef362c`。完整source **5c4059b→d0fc6a808b6c2ec635ac4511559a4c0f81e6e89c** 已单独提交，六新路径、私树clean；183产品＋3真实embed与受测／提交逐字节一致。全服普通359／1586 **PASS**（215.071秒），push／auth／devices／storage四包完整race144／503 **PASS**（329.551秒），仅新cmd `TestAssembledSyncLeaseCheckProductionSnapshots` 定向race1／5 **PASS**（16.480秒），vet0.883秒／build1.419秒／产品fmt／diff均 **PASS**。本轮没有新全cmd race，旧5c前置宽矩阵证据不冒新实测。正式source交接 `/tmp/relaynest-be10-sync-lease-check-20261030/handoff.json` SHA256 `554769a77d113201f312efe0a2364d09e54a373b1f72820d587d5e3e2455d95b`；Go及source锁已释放。独审需修见下，准确I01 **NOT_RUN**；本叶不代父策略／worker／平台发送或global。
- 独立A01／待修F1：不同Auditor `/root/sync_lease_auditor` 固定完整 **5c4059b→d0fc6a8** 六路径，结论 **CHANGES_REQUIRED**；唯一P2 `BE10-SYNC-LEASE-A01-01` 位于 `sync_lease_snapshot.go:27–35、42–52`，当前FCM注册／设置版本被损坏为数字BLOB后仍强转成整数并返回有效候选，违反坏数据失败关闭。原9顶层／35全例独立PASS；新增1顶层／2子例真实FAIL（合计1／3失败），不以source矩阵PASS代独审。正式报告 `/tmp/relaynest-be10-sync-lease-a01-20261104/handoff.json` SHA256 `cda19b4f50f0b818b18d4699f092dfc506e078bdc48853b1b43713e00d0f5edf`；原overlay须原字节复验。不同Fixer `/root/sync_lease_fixer_f1` 已从固定d0fc6a8在 `.worktrees/backend-be10-sync-lease-f1`／`fix/backend-be10-sync-lease-f1-20261104` 准备，只处理此编号，限snapshot实现与必要新测试；修复尚未提交，F1最小11顶层／45全例 **PASS**（9.045秒），原A01两子例原字节复验转PASS，184产品＋3真实embed前后同值；full已获授权在途、尚无终态，仍须正式fix提交后完整A02及准确I01，不能done。旧min01／02新夹具FAIL及min03／source矩阵PASS全部保留，不改历史SQL、既有虚拟settings版本0或过期／轮换正常拒绝语义。
- 单目标与三个新主文件：仅 `server/internal/push/sync_lease_check_models.go`、`server/internal/push/sync_lease_check.go`、`server/internal/push/sync_lease_snapshot.go`，必要同包新tests及 `server/cmd/relaynest/sync_lease_check_test.go` 另计（只新增test复用已集成TLS夹具，不改main／旧helper）；消费可信Tx与现有ReserveJobsTx产出的完整sync租约，仅读当前job／lease与auth代际／有效FCM本session注册版本／设置版本（虚拟0）／deployment／期限，失败关闭、不给过时lease新资格。只读复核不是最终发送资格；不返回token、原lease秘密、正文、URL或callback，不把DND／READ／删除擅自当sync禁令。
- 不变量与锁：legacy与alert不得混入，只读前后全表digest相同；不领／续租／释放、不写attempt／结果、不发送、不增加worker／adapter／HTTP/main/router／DDL，不分配0013。三个新文件与必要自有tests排他，其他push／auth／devices／storage／messages／迁移及依赖全只读。Go须总调度唯一窗口串行，保原50ms／12并发／10ms／timeout与旧断言。
- 原验收：真实生产TLS非零job→真实ReserveJobsTx→可信事务复核；双Store接管／释放后旧lease、到期、注册轮换／撤销、认证代次、实例变更、版本0、reopen、mixed legacy＋完整sync＋alert均真实验证，无token／秘密泄漏／任何表写入。完成相关普通／race／全服普通／vet／build／gofmt／diff、锁版及冻结输入核验，正式本地source后不同Agent固定全链独审、编号修复／重审和准确新组合I01全部满足，才能由总调度决定done；不代BE10父剩余完整策略／worker／平台与global门槛。

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
