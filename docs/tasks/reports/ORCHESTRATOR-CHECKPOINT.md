# 总调度恢复 checkpoint

更新：2026-10-23。本文件只记录恢复信息；唯一任务台账为 `registry.json`。不是全部完成报告。

## snapshot26：登录防护准备研究集成与认证启动交接

- P04-04-PREP01 研究实现 `e0299e41897885f905200ca505729ef8bd31bf98` 已在 main；独审 A01 PASS `57dfe9075e404722fbca5a85d69af5aab9065aff` 仅精确挑拣为 `5769bf2d558ae8239c50648b74c883aa12cd607b`，main 集成前 `a8cde39d73af3c47a0f271a0b2fbf0b51b1f5e23`。研究/A01 blob 均同源，文档 diff、链接与 SHA 检查通过，无源码/API/SQL/依赖变动，不跑业务测试；[I01](P04-04-PREP01-I01.md) 独立提交 `cd8f904d5f0aa0918071dbaec040617456e14359`。
- 此 PASS 只覆盖研究文档，不表示登录防护实现，不放行 P04-04；前置 P04-02 仍未完成。P04-02-S02-S01 已 done，F01 修复及最终 A02 PASS 已精确集成，保留 A01 历史；详情见 [认证 HTTP I01](P04-02-S02-S01-I01.md)。Bot S01 已 done，正式 `0004_bots.sql` 归 Bot 且已占用，不得重占。
- `/home/lijx/.codex/worktrees/auth-startup/RelayNest`、`codex/auth-startup` 基线已为 `a8cde39d73af3c47a0f271a0b2fbf0b51b1f5e23`；继承 main.go dirty SHA-256 `413893b9846ca4e44a586272b2ddf12e1d2518e6d33a35733ef40ac1ef086bf0`，由 `auth_startup_dev` 唯一写。启动关闭/全面契约与父卡验收仍待该任务，不据继承源码宣称通过。
- 旧 auth-http 树及两份原 dirty 保持原样：main.go SHA-256 同上；未跟踪 P04-02-S02-implementation.md 为 `dfd20dfe7203776717bdb854a108b1f6c0622c68ad6553703c94e71cfafe7cb9`。未纳入其他来源提交，未改卡/registry；main registry 用户 dirty 与 `.directory` 未 stage/覆盖。完成本快照及 root 授权的后续规划树准备后释放 main index；不实施登录防护，不推送/部署。

## snapshot25：认证 HTTP 首卡集成与启动卡隔离树恢复点

- P04-02-S02-S01：原实现 `117da40467ff76aa59d02b0c73b996b79c519fb8`、A01 CHANGES_REQUIRED `0c2de0d162a597808188251aebf7e7cd8b56870f`、F01 规划 `86080588cb6113eb774fc2ae946f9363c25511a7`、修复 `e57095f5ec71802cbc321de849d97b572447567c`、最终 A02 PASS `246adb19cfcf082cbcaba4364c2a1479219a3272`，均仅按序无冲突精确挑拣，保留 A01 历史。main 从 `fb770a4bec75d75c66ac72d06c472eb7714edf2a` 到固定组合 `4e74575fc1ed4c3c597ec8eee7feb3b830cd9a15`；[I01](P04-02-S02-S01-I01.md) 独立提交 `f8a2304a407c589261995f866ac04c83c12b24e4`。三源码及六份报告/卡共九 blob 与 A02 来源完全一致，无业务修正，无需重审。
- 固定组合显式 Go 1.27.3、GOTOOLCHAIN=local：auth/httpapi race、vet、三 Go 文件 gofmt、diff 全通过；复用既有真实 Web embed。本次未重复 Web 构建，未执行 cmd 接线/启动关闭、全面契约、真实代理/NAS/手机、WS/推送；原 S02 与父卡仍未完成。Bot 存储及正式 `0004_bots.sql` 已在基线中，不重占迁移号。
- app 已创建 `/home/lijx/.codex/worktrees/auth-startup/RelayNest`，分支 `codex/auth-startup`。本快照提交后先将新树快进到该精确最新 main，再按字节仅复制旧 auth-http 树 `server/cmd/relaynest/main.go` 为下一卡唯一 dirty；最终基线 SHA 和核对结果在交接登记。继承 SHA-256 应为 `413893b9846ca4e44a586272b2ddf12e1d2518e6d33a35733ef40ac1ef086bf0`，不提交 main 源码、不复制旧未跟踪 checkpoint。P04-02-S02-S02 需补真实启动关闭、cmd/router 回归、全面契约及原父卡验收矩阵，继承 dirty 不等于通过审计。
- 旧 `/home/lijx/.codex/worktrees/auth-http/RelayNest` 与 `codex/auth-http` 原样保留；上述 main.go dirty 和未跟踪 `docs/tasks/reports/P04-02-S02-implementation.md`（SHA-256 `dfd20dfe7203776717bdb854a108b1f6c0622c68ad6553703c94e71cfafe7cb9`）接管前后相同，未 stage/reset/clean。并发 PREP01 独审仅在旧树另提交自己的报告，不纳入本次五提交链。
- main 用户 `registry.json` dirty 与 `.directory` 未跟踪均不 stage/覆盖；registry 仍由 root 登记。新树核对完成后交接并释放 main index；无推送、部署、新聊天或子 agent。

## snapshot24：Bot 存储实际集成恢复点

- P04-06-S01 来源 `/home/lijx/.codex/worktrees/bot-store/RelayNest`、`codex/bot-store`，base `5e5ae142f4988aff9c89eaedfbf3d109838306a2`；实现 `56fa387e91378a85f0fbaf6a34d51c228a57f196`、独审 PASS `32161b51abe1ea408d19a53823267e528e67c263` 已仅按序无冲突精确挑拣。main 从 `e0299e41897885f905200ca505729ef8bd31bf98` 经 `bdeaffe` 到组合 `1fa1d6c10b964613513844656931f5d210e399db`，I01 `efc602d14bf47edb4c45fbe692c955af19267fad`，见 [I01](P04-06-S01-I01.md)。七实现路径及审计报告共八个 blob 同源，无业务修改，无需重审。
- bots/storage/changes 非缓存测试与 race，完整 Go test/vet/build、gofmt/diff 均通过；现有 Web embed 可用，无需 npm 构建，未改依赖或提交产物。API、storage 实现、原 0001～0003 未变；storage_test 动态真实迁移夹具保留失败回滚/锁/checksum 等断言并增加真实 Bot 升级。正式 `0004_bots.sql` 现唯一归 Bot 且已占用，后续迁移另行串行分配。
- 消费者使用真实 `bots.New`、Create/Get/List/Patch、Authenticate 与 AuthorizeTx；创建才返回三键明文，查询只公开元数据，List 尚非 HTTP 分页。错误 ErrInvalid/ErrNotFound/ErrConflict/ErrUnauthorized/ErrForbidden，底层错误保留，由 HTTP 脱敏映射。Create/Patch 须先完成管理员授权。P05/P06 必须在实际业务 Store.Write 中调用 AuthorizeTx 并传播错误，以数据库当前绑定/代次/启用/能力和认证快照交集核准提交；Webhook 仅 Send，Bot 身份不能管理。沿用公共 changes.Append，同事务成功后才承诺序号；不得用请求开始快照代替，不跨网络持有事务。
- S01 仅此边界闭环，HTTP/NAS/手机/正式 AC 仍 NOT_RUN。S02 仍需 P04-02 与完整 P04-03，不准备 Bot HTTP 树；P04-04 的只读 PREP01 未审计，不算父完成。认证 F01 最终 `e570` 尚在独审，待证据后另行集成；认证树与后续卡未动。
- 当前仅保留调度既有 dirty `docs/tasks/registry.json` 与 untracked `.directory`，均未 stage/覆盖；registry 唯一由 root 更新。I01 与本 snapshot 独立提交，最终核验后释放 main index 供认证集成。未 push/部署/创建聊天。旧 snapshot 为历史，恢复优先本 snapshot24，P4～P15 远未全部完成。

## snapshot23：全局变更事务共同基础实际集成恢复点

- P05-01-S01 来源树 `/home/lijx/.codex/worktrees/changes-foundation/RelayNest`、`codex/changes-foundation`，base `85305dfd3e4b4e15e78a208b5d75e8fd331deea0`。实现 `9041072b5a86610d4e08440732f06b9492918011`、独审 PASS `3314661e6343e36acfa702538f9d7fe35ab7e64f` 已按序无冲突精确集成；main 从 `5e85e9f8b1e1567c4bd7a96cc8ae58dcf738aeab` 经 `6de2970ab1c18e97463b789cd39c2c68e96356d2` 到组合 `b245192604c09445ed21156b1832fcd9e9a1c785`，I01 `c8e782d264226e9204e61494e311ede640ca3bc1`。四文件 blob 同源，API/依赖/SQL/storage 未变；非缓存 changes/storage 测试、changes race、相关 vet、gofmt/diff 均通过。详见 [I01](P05-01-S01-I01.md)。
- Bot 实际调用 `changes.Append(ctx, tx, Metadata)` / `Epoch(ctx, tx)`；业务与日志在同一 Store.Write 中，错误向外传播，Write 成功后才承诺序号。Sequence 线缆为十进制字符串，Revision 为整数；Type 八种冻结事件仅映射投影键。四种 ErrInvalid* 支持 errors.Is，底层错误保留，epoch 缺行保留 sql.ErrNoRows。消费者复用真实接口，不复制私有追加逻辑。
- root 后续登记共同基础 done 并实际派发 Bot 存储；本快照提交后从精确最新 main 用 app 准备 `bot-store` 隔离树，路径/base/branch 由交接返回。本轮只准备，不实现、不预写 SQL；`0004` 仍未分配，由 root 实际派发时串行预留。主 registry 由 root 唯一写，既有 dirty 与 `.directory` 未改/stage/提交；完成树交接后释放 main index。
- 认证 HTTP 首卡仍在 `auth-http` 隔离树开发 handler/router，主树 main.go 留待次卡；本次未触碰其源码或 index。Web、真实 NAS/手机与正式 E2E 未运行，父卡及其消费者组合验收不因本次基础通过而完成。未 push、部署、创建聊天或子 agent。

## snapshot22：认证 HTTP 续作规划与保护工作树恢复点

- P04-02-S02-PLAN01：来源实施 `3022e333def87c51795db442d623f79584633491`、独审 PASS `b138739f7a134a60750b56235a59b4535e38c08e`；main 从 `85305dfd3e4b4e15e78a208b5d75e8fd331deea0` 精确集成 `d6403d2` → `2f31ed8`，I01 `1cf5fa1`。四新文档 blob 同源，原 S02 七项/父五项验收保留，三相对链接存在、相关19节点依赖闭包无环、diff通过；无源码/API/SQL/依赖变更，未跑Go/Web业务测试。
- 两子卡 `P04-02-S02-S01` → `P04-02-S02-S02`，先独审集成再派下一卡；root登记规划done及子卡后才派发。首卡接管 handler/测试/router；次卡负责 main/入口配置/关闭与全验收矩阵。底层session/origin/管理员只读；原S02/P04-02组合验收、G-API证据和真实NAS/手机/正式AC门槛不因规划通过而完成。
- 保留 `/home/lijx/.codex/worktrees/auth-http/RelayNest`、`codex/auth-http` 原树；源码祖先 `b60f9ae09878bc6f3af06861ad19a90a73f088e1`，原规划/独审造成非FF。只精确cherry-pick最新main相对祖先八份文档提交与I01，无dirty路径冲突、无代码改写；来源准备HEAD `f1c739b9c9aaf84b2fb6a1eaad6edc9a88b69dda` 与main提交树相同。此快照提交后仅同步该文档提交，最终HEAD由交接登记，作为首卡可恢复基线。
- 五原dirty：`server/cmd/relaynest/main.go`、`server/internal/httpapi/router.go`、`server/internal/auth/http.go`、`server/internal/auth/http_test.go`、`docs/tasks/reports/P04-02-S02-implementation.md`；已逐文件SHA-256核保留，不stage/reset/clean。首卡不得提交main和原checkpoint；保留dirty并不等于源码审计通过。
- changes-foundation继续独立开发，本次未纳入；0004未由本卡分配。主registry仍由root唯一写，既有dirty与`.directory`未改/stage/提交。完成最终哈希/index复核并交接后释放main index；未push/部署/创建聊天。

## snapshot21：全局变更事务共同基础规划集成恢复点

- P05-01-PLAN01：实施 `3ced9e53d581451dc6ee81e74ba7965a505056a7`、独审 PASS `856566263cb026ecbeeca055f9aabaddb03b43d0`；main 从 `928e031fe259d418c55272ebc19fd7b6b9a28e53` 按序精确集成 `d828a924bfdfa0cfbe96890548d6eb9a30e25c77` → 组合 `2b6e5aeb236c6a86cff6522366872e94319f85b9`；I01 `0f72e3e2e6fb4995288aa20674b5b2d1eb07db50`。无冲突/修正，四文档 blob 同源，父卡原文逐行保留，21相对链接存在、相关26节点依赖闭包无环、diff通过；无源码/API/SQL变更，未跑Go/Web业务测试。
- 规划通过后由root登记done；实际 P05-01-S01 等规划、P03-07、P03-08独审集成done及G-GIT/G-API。父P05-01保留P04-07/P03-08，等待共同基础实际复用与原全部验收收口；Bot S01 必须等待真实基础实现独审集成和接口交接。A/I依subject_id/review/精确SHA与PASS触发，不依subject done。
- root已将Bot规划done及S01/S02登记；认证HTTP预算退出，未提交，后续调度续作。G-API契约pass、P04-01/会话S01/来源S01done；真实NAS/手机/正式AC仍NOT_RUN。
- snapshot21提交后从精确最新main用app准备 `changes-foundation` 隔离树，仅供P05-01-S01后续派发，路径/分支/base交root登记；准备不等于实现。0004仍未分配，此卡禁止迁移。主index串行占用在工作树交接后释放。
- root是registry唯一写者；本集成角色未改/stage/提交dirty registry，`.directory`保持用户未跟踪；未push/部署/创建聊天。本快照只提交checkpoint。

## 授权与用户决定

- 持续推进P3～P15及必要P0/P2收口。P13～15仍待核心实际完成及G-SHELF实质门槛，P12-06准备报告本身不足。
- P1按用户最小PoC例外PASS，G-MP保持；方案A仅既有显式助手补丁，只有v3.1.5有运行证据。旧NOT_RUN、最低目标v3.0.0与拟议>=3.0.0声明分别保留，不冒充AC-07/P10/P11正式通过。
- MP恢复镜像与Compose声明不同；重建/升级可覆盖补丁，/netdisk曾经用户授权移除，不自动恢复或重建。
- 用户已批准固定通用title+content、显式可选vendor、超时不跨FCM自动补响、切换仅未来新事件、在途残余及离线DND/已读/退出撤回限制。厂商资格/API/SDK/签名/分类额度/错误码仍须PoC；不代注册/身份提交/平台授权。
- 用户明确“真机运行留到最后验收的时候”：P03-03初始化不因真机E01阻塞，但安装/冷启动/深浅色/系统栏NOT_RUN须移交P09/P11/P12正式验收。真实正式签名成功路径也仍NOT_RUN。
- 现网部署、MP重建升级、破坏性数据、公开发布、远端推送/Release最后一步须具体结果审查确认。未创建新聊天、自动化或定时监控。

## 当前恢复点（2026-10-23）

最近确认主树 `main@0f72e3e2e6fb4995288aa20674b5b2d1eb07db50`（全局变更基础规划独审精确集成/I01 后，snapshot21 前）。唯一任务台账仍registry；本轮持续执行，远未完成P4～P15。

- P00-01/02/04、P02-01～04、P03-01～06及08～12均已独立审计精确集成。G-GIT/G-MP/G-FCM/G-API=pass（MP/FCM为历史PoC限定，API仅契约）；CORE/DEPLOY/SHELF/SHELF-API/OPPO=pending。
- P03-07同步/设备/全局配置/健康契约及最终综合证据均已独审精确集成。最终卡实施7ec2ae3、独审PASS34425b4、main组合981692f、I01 375a270；父五项/S03八项证据齐全。root已据最终独审及精确集成证据登记P03-07/S03/最终卡done与G-API契约pass，无需新用户授权。运行时/正式AC未验。

| S01子卡 | 最终实现 / 独立审计 | main集成 |
| --- | --- | --- |
| sync/read | `966066831a6b1b75720cae131b2e5d17c22e6e35` / `d51bceebcf4a81637920efd9c60b802a36943ee1` | `33cc7585e873a5debaddc5b93c8657b60d5a73a8` |
| bootstrap含F01 | `a379d9620c9a0b616c575d749fd05030376437c3` / A02 `0993016dd18b120ce4c300c7b86087474e5eee9e` | `3cdb5e228b7d124890d45250728b260b2091a6db` |
| WS及父联合收口 | `169fd8043e48e37762f11e845f00f56ce8024933` / `91215226a724679ca163be5279c049cb6bba3ad7` | `5cdaf15f7e01b663dd0f2b68fc044d9e875aaa79` |

bootstrap原实现9cb95c4经A01 532ea26发现BS-A01-01（seed可冒充实际追赶完成）；F01规划/独立审计/分支集成后修复，A02独立重现旧问题并验证来源对照/零变化追赶/新事件链，最终准确七提交链入main，原负面报告保留。问题仅在契约范围关闭，运行时NOT_RUN。WS开发/审计读取输出预算偏差如实记载，不假称流程完全合规。不得因这些已闭环工作被旧快照描述为未完成而重跑。

## 在途工作区与下一步

- `P03-07-S03-S02`健康契约已精确集成：原实现ac3c725、A01 bfc92e5、F01最终实现`966ec9e57150555342d067ebfb9afcf6eb03990a`、A02 PASS `5a0c8731859cb14d20fcb49044cf41fdb5f12b55`；main组合`6f89cf633c5ecd8677ca61b53d99a12969a8fcd8`，I01报告提交`5a91ddcc70e5df4f0388f42d1953b778e4991af0`。无冲突/代码改写，S02-A01-F01由A02关闭；原负面报告与预算偏差保留，调度据实际证据登记done。
- 集成复验实际锁Python `/tmp/relaynest-health-contract-locked/bin/python`，Python3.13.15/18锁版本；OpenAPI与validator通过，37操作/109Schema/987例（470正517反）。旧34操作/100Schema/899例、全部旧组件/servers/两锁冻结；API六文件与966ec9逐字节相同。A02独立变异证据保留，真实HTTP/DB故障/FCM/手机/NAS均NOT_RUN。
- ORDER01文档调整已独立审计集成`2fc964dc2c967f6ac53b1d2512d0e0d3ff2734e7`，健康卡可先行，但共享四API文件仍严格串行；最终S03-S03依赖全局设置和健康两卡均done。
- 用户已接受全局设置正整数政策，POLICY01最终文档`7c2d995c8f045ecb31f87065b1a7e90eaf5f0e86`、审计`5b850f156785805f25d07e4d28e04cb853b05715`已集成main `b86ad2eba8ca5c9afc76ba65159de974ed968339`。正数天/字节以SQLite64规范十进制字符串表示；0不支持；默认30天/10MiB/50MiB；全局设置专用CAS保留JSON整数，上限2^53−1，公共Version不变；到龄判断避免超大天数乘法溢出，匹配但耗尽409原子拒绝。技术方案以主树PRD/设计/卡和POLICY01-finalization报告为准，不再询问同一政策。
- 全局设置S03-S01实现`27c5ac9bbdfd3c5edd09901a7ac01a5cc0ecfb82`、独立A01 PASS `8d48a71c59d7c24408f4ca5a59c08e6070211ce2`已无冲突精确集成；main组合`394d6117b75605f660663bb7c8087cb6b4410cc1`，I01报告提交`329238af18cabaf44256b0e55afa7464119f7aac`。实际锁Python3.13.15/18锁版本，OpenAPI和validator通过39操作/115Schema/1229例（568正661反）；旧37/109/987与组件/两锁冻结，仅SyncChange.oneOf+描述按批准全局分支扩展。六API文件与受审实现逐字节同。旧政策树与实现树保留，不重置；运行时NOT_RUN。
- 设备父P03-07-S02已done，三卡精确集成链/父八项契约验收经独立审计核对完整。设备会话最终F01`a63bd59c8f50dcd83353e279b044f25f4383780c`、A02`c05a5b93b64b5348bb98270ade4856fc23061fac`、main`876935a219c3a1bf828aeaabb2e8a82ca2bb5d4a`；注册实现`6901bc09de53995435102d11f2dfad9bd3f46059`、审计`14081bcb11a5ce8c7ce7b700b6b1ca6e10bdf1e1`、main`82ebd62634ecf9025670a75aa956a45667c55ea7`；通知联合实现`692e11964ea74cb84cf62470a08e4c3a1780a01f`、审计`36a05a2523a968f384e82de4eb599a10f31226eb`、main`42cd918c01c34bb7baa918420dad0daa2b4d6a98`。DS-A01-01契约关闭，原负面审计保留；读取输出/预算偏差如实保留，不声称流程全合规。不得按旧快照重跑已完成卡。
- 设备冻结交接：撤销按Session.id，device_id不唯一Session；通知偏好按设备共享，注册按Session.id/provider隔离，初始CAS0、删除递增持久屏障，旧未知结果不改新注册，token不回显。GET/PUT current/notifications、配置CAS/恢复/vendor服务器前置、device.notifications.updated、DND/前后job快照CAS/租约串行仅契约及合成状态，真实worker/平台NOT_RUN。D1～D3/FCM边界保持，当前全局设置卡只补剩余全局变更，不弱化设备域。
- 当前共同契约冻结：39操作/115Schema/1229例、全部旧组件/两requirements锁、Sequence SQLite64字符串/Version>=1、权限/no-store、H/epoch/floor/seed与真实sync完成来源、WS仅提示不可推进游标；健康纯文本媒体例外仅限两根健康精确操作/状态。后卡按实际基线重验。
- S02文档规划已独立审计集成main`d927abed945379e00548b01e7cecf7bbef59bac2`，三子卡：S02-S01会话→S02-S02完整provider注册生命周期/CAS/条件删除/恢复/持久版本屏障→S02-S03通知CAS/恢复/设置union/发送快照联合收口。三卡已done；父S02原八项契约验收完整收口，运行时NOT_RUN。
- S03最终卡已完成：实施`7ec2ae3494088b4819830b46c23fd88d7e81560b`、独审`34425b492601045ffcd109a88246a73a1094542f`精确集成至main `981692f49663075aa553ce45aa645e75197b15ea`；API六文件与base b8c3c2相同，39/115/1229组合检查通过，11个前置main点均为当前祖先；独审35组/95路径映射证据保留。
- 每卡实现独立提交→不同agent精确SHA审计→编号修复及重审→集成agent准确组合验证→root登记done→下一卡。G-API契约已实质收口，后续P4～P10按真实DAG；不以契约通过冒充运行时通过。
- 恢复核查、健康/settings精确集成由`/root/recovery_verify`完成，两源树均已释放。集成角色从snapshot11后最终main准备api-contract-final树供P03-07-S03-S03，精确路径/分支/base依工具结果交接；准备不等于实施，实际owner/锁以registry为准。

## 外部条件、环境与文件保护

- P00-03环境报告/独立审计已集成main`bf1421383abd46bfaa1e803c1ecfdf63e80a9c46`，完整环境仍BLOCKED：缺RepoDigest/重建证据/独立实例/当前现场，不撤销P1用户例外。
- P00-05报告/独立审计已集成main`99db13e5a0426f8e19407a43500b17ebb8390058`，用户最新确认可信NAS只读入口、独立测试资源及HTTPS Origin尚未准备，部署验收前提供。不要重复索取/猜连接/自动登录；不阻本地开发。TLS/WS/上传/LongPolling现场实测NOT_RUN。
- 用户确认OPPO账号/应用Push权限尚未准备，P02-05/G-OPPO受阻，仅阻厂商实测/激活；真机运行留正式验收，正式签名成功/安装界面/端到端NOT_RUN保留。不能将Schema/合成状态或阶段放行冒充HTTP/SQLite/Room/WS/通知实际运行验收。
- 锁Python可先核`/tmp/relaynest-p0305-f01-locked/bin/python`，缺失依hash锁恢复不升级。Android/mise版本沿原锁；干净Go构建前先Web npm ci/build生成gitignored embed资源，不靠旧dist。SQLite已占0001，下一迁移0002；共享API/依赖/迁移串行。
- 主树registry.json含用户原有修改及调度更新，`.directory`为用户未跟踪文件，均不stage/覆盖/丢弃。本摘要由root创建，Git提交必须交集成agent显式路径处理，禁止git add .。
- 各已审源树及分支保留，不prune/remove/clean/reset。request-source已释放，来源 S01 已由root据独审/I01登记done；auth_http_dev 正在隔离 auth-http 树（base b60f9ae）实施 P04-02-S02，尚未done。真实0001/0002/0003已占，下一0004未分配。Bot规划已精确文档集成；changefeed规划独审PASS待下一独立集成轮，不建两者源码树。
- 本文件不是全部完成报告，不创建新聊天/定时任务/自动化，也不授权push、Release、真实部署或MP重建升级。P13～15虽已授权，仍等核心实际完成/G-SHELF实质门槛，不改RelayShelf后端/数据库。

## 当前P4派发点（恢复时以registry和实际agent为准）

- 主树snapshot13 `fd7ca5173c540796aa97e3efb9bf90a7adc94c32`，G-API契约pass已由root据最终独审/集成登记，P03-07及S03done。P04-01-PLAN01规划最终`73d4fa47c02a4478e3a81acf0f60f2740e63f252`、审计`34acc46f40d425f5bda8296a4bd8f11b1c43caeb`、集成`c69d8304277fb9f1ab9a9409e708acac5a0573a6`/I01 `18d3f3ad5c5e3ca64ed3f7bfa45546bb86ac5d9e`，已done。
- P04-01原实现9a97d7e、A01 b7fce9d、F01最终9af6a70c96b106b4a8f8d48e0913a80aa5bed68d、独立A02 PASS f4767d6cc640d3a5859ad0371860c8dcadcf8081已精确集成至main组合3ba2b496ae21737d15c8442a1fd56429513da1d4，I01 d3acec6779b6ce5377737575cdf51f550d41b719。无冲突，18路径blob同源/14源码依赖同受审修复；SIGINT termios问题A02关闭。锁Web ci/build、Go1.27.3/local三包非缓存race/vet、主树二进制真实PTY25取消与独立CLI竞争回归均PASS。root已据证据登记P04-01 done；本快照不改registry。非Linux实际运行/真机/会话正式AC等NOT_RUN保留。
- 开发交接待审：固定管理员id=1、username原样区分大小写不trim；Argon2id64MiB/3/1；generation int64从1至MaxInt64耗尽原子拒绝reset；摘要与代次同事务；提供ReadAdministrator/InitAdministrator/ResetPassword。0002本实现占用但待精确集成，下一编号0003仅集成后确认。Go1.27.3/SQLite锁保持，必要x/crypto0.58.0、x/term0.47.0、传递x/sys0.49.0为本实现依赖，须审计核对。真实CLI/SQLite与未测会话/WS/手机等分开。
- P04-02-PLAN01文档规划71682be3f1f91ea0a05b2a6237a6d68d218bd6e6、独立A01 PASS 100f9314d7137d298d049c139318b290ceed95e9已精确集成；main组合b5519a6b339303ed11fbabbf06cbfdf5990c6a4d，I01 d6a30a7eca7149101ea7ac59fc3a6ad523812788。五文档blob/链接/原五验收通过，server/API未动，无父子依赖环。建议S01依P04-01+PLAN01、S02依S01，父保留P04-01并汇合两子卡；沿父G-GIT，root登记。旧session-plan规划树保留；S01已完成最终独审精确集成，S02仍等独立来源安全前置，不编造尚不存在的接线接口。
- 干净Go检查先Web npm ci/build生成gitignored embed dist；工具沿mise锁，测试用临时DB，不连接生产。root唯一写registry，含用户原dirty内容不stage；`.directory`不动。本快照由集成角色仅显式路径提交。持续推进P4～P15，不把该恢复点称全部完成。
- S01迁移夹具范围补充已独审精确集成：实施d9c791bad99bf508beafd0a5785f0d5f74b0920f、A01 PASS4ee17de23341e5a8bda8ed63876f628788555090；main组合1620289997cba0b451441a0390777995d0e50448、I01 533f79b3b7aff9fb861e6f72204f9202f7637135。仅S01卡/实施/审计三文档同源，原验收/链接通过，源码/API未动。新增storage_test.go真实迁移夹具维护权限，旧升级/回滚/锁/checksum强度不减；不授权storage实现或额外依赖。
- P04-02-S01实现1da5e72aed508e01fb0d172b79fba0052af61a52、A01预算checkpoint349ee276e22336466b2d4a317bac88dc3e47151e、A02最终PASS7e6a8d5cfd59d6069711fd3106ebc2ec718933bb已无冲突精确集成；main组合9269876a5b57f18cc43d8100e88dc0f4a0b3b4ca、I01 cfc43c7f1bc63221d2738ad612b590ddd22f469b。A01仍BLOCKED历史checkpoint，A02组合独立证据完成PASS；root已据证据登记S01 done，不代表P04-02/S02完成。
- 本轮组合：Web锁Node22.23.2/npm10.9.8 ci/build通过；Go1.27.3/local三包非缓存race(auth7.229/storage2.178/cmd2.397秒)及vet/diff通过。7路径blob同源、5实现路径同1da，API6/依赖/CLI/0001/0002不变，唯新增0003。S02须把密码校验观察代次传Create；Inspect后其它校验成功仍须Authenticate重新查库续期，expectedClient依实际凭据通道决定。真实HTTP/客户端/正式AC等NOT_RUN。
- P04-03-PLAN01规划8edac1a5b381fef79149a197ca11affefd129909、独立A01 PASSca448aae8cc92f30de115d5a2026ee888afcf454已精确集成；main组合8b8c503b9794d89e050b78420a1e18e7d69000de、I01 375ab7870abea1b38747d49a2e582b77c6a1c199。五文档blob/链接/原验收保留/依赖无环通过，源码API未动。
- 新来源P04-03-S01“显式来源配置与可信代理解析基础”依P03-07+PLAN01，gates沿G-GIT/G-API；认证S02保留会话S01并增来源S01，P04-03父保留P04-02并增来源S01。仅origin.go/origin_test.go或等价origin专用包及实施报告，不碰router/main/auth/存储/依赖/迁移。单HTTPSOrigin/显式peerCIDR/单层覆盖forwardheaders严格拒绝不回退，非可信peer按真实TLS/Host；不新增信任政策或NAS真实配置。新树只准备，不在本轮实施。
- 来源P04-03-S01实现bc36861e21f1f33d35ba33426df82125cb6ddfcb、独立A01 PASS1b61b1760cdafc6dcb147bdb501b778919d359de已无冲突精确集成；main组合b75b6c348d5912b14e94632096519d22c5179e7c、I01 0fa0d8d520140e978b57f942ed133271411b959d。四文件blob同受审源，Web锁ci/build/httpapi与cmd非缓存race/vet/diff通过，API6/依赖/认证/存储/CLI/迁移未变。待root登记来源S01 done，不代表P04-03父或认证组合通过。
- 认证S02交接：NewOriginPolicy显式构造；Resolve验证peer/外部HTTPS，AndroidBearer只用Resolve；ValidateLogin要求Origin，ValidateWrite仅Origin缺失可退Referer，仍须会话CSRF。错误ErrOriginConfig/ErrRequestSource/ErrBrowserOrigin由适配层按冻结契约映射，不能回显原始来源细节；不造默认域名/DB/开关。真实NAS/header覆盖/Cookie-session-CSRF组合NOT_RUN。
