# 总调度恢复 checkpoint

更新：2026-10-22。本文件只记录恢复信息；唯一任务台账为 `registry.json`。不是全部完成报告。

## 授权与用户决定

- 持续推进P3～P15及必要P0/P2收口。P13～15仍待核心实际完成及G-SHELF实质门槛，P12-06准备报告本身不足。
- P1按用户最小PoC例外PASS，G-MP保持；方案A仅既有显式助手补丁，只有v3.1.5有运行证据。旧NOT_RUN、最低目标v3.0.0与拟议>=3.0.0声明分别保留，不冒充AC-07/P10/P11正式通过。
- MP恢复镜像与Compose声明不同；重建/升级可覆盖补丁，/netdisk曾经用户授权移除，不自动恢复或重建。
- 用户已批准固定通用title+content、显式可选vendor、超时不跨FCM自动补响、切换仅未来新事件、在途残余及离线DND/已读/退出撤回限制。厂商资格/API/SDK/签名/分类额度/错误码仍须PoC；不代注册/身份提交/平台授权。
- 用户明确“真机运行留到最后验收的时候”：P03-03初始化不因真机E01阻塞，但安装/冷启动/深浅色/系统栏NOT_RUN须移交P09/P11/P12正式验收。真实正式签名成功路径也仍NOT_RUN。
- 现网部署、MP重建升级、破坏性数据、公开发布、远端推送/Release最后一步须具体结果审查确认。未创建新聊天、自动化或定时监控。

## 最新接续补记（覆盖下方旧快照状态）

额度恢复后已重新核验：主树 `main@d927abed945379e00548b01e7cecf7bbef59bac2`，仅用户原有及调度叠加的registry未提交，`.directory`未跟踪，均未覆盖/暂存。下方旧快照为历史，实际唯一台账仍registry。

- P03-07-S01细分规划已独立审计集成`8e85887934d37777ec7f5cd2c9fa340f36c5f3ed`。三个串行子卡：sync+read→bootstrap→WS/父验收联合收口。
- 首子卡P03-07-S01-S01已done：实现`966066831a6b1b75720cae131b2e5d17c22e6e35`、独立审计`d51bceebcf4a81637920efd9c60b802a36943ee1` PASS、main集成`33cc7585e873a5debaddc5b93c8657b60d5a73a8`。冻结25操作/73Schema/446例，原23/61/369保留。
- bootstrap实现`9cb95c41488fdf29895900b8288b7c15aacc5abe`、负面审计`532ea26e791b9774bcd6c6246769499a40823b26`，CHANGES_REQUIRED：BS-A01-01 P2初始seed与实际sync追赶完成来源未区分，verify不能区分。原实现尚未入main，不得沿用旧检查放行。
- bootstrap修复规划`7922f0f6704a6b52df7f2b290b4595cf6d50fc68`仅新增F01卡和规划报告，API未改；`/root/bootstrap_fix_plan_audit`当前独占`/home/lijx/.codex/worktrees/api-bootstrap/RelayNest`、`codex/api-bootstrap`做独立规划审计。通过后取实际HEAD派F01修复，再由不同agent进行原卡A02，PASS后精确集成全部准确组合。零变化实际sync完成position=H必须合法；不允许初始seed冒充，不改已审sync/read；确需改前卡则停止该部分并报影响/重审。
- 原P03-07-S02设备域已预规划并独立审计集成至当前main`d927abe`：规划`1122c739a692835657807e9c3cd2d3f59810d8a5`、审计`a16ea8a663861ff6a607c7a1689198a6c9dc90bd`。拆为会话→完整注册生命周期/版本屏障→通知CAS/union/发送快照联合收口，三子卡已登记todo，仍须等待S01父完整收口；真正实施读取最终main/S01摘要，不拿25/73/446冒充未来基数。
- 用户最新确认NAS/反代可信只读入口、独立测试环境与HTTPS Origin尚未准备，部署验收前提供；P00-05继续blocked，不重复索取、不阻本地开发。OPPO账号/权限未准备及真机留最终验收决定保持。
- 实现/审计旧树及Git对象全部保留；S01-S01源树`api-sync-read`在`d51bcee`、S02规划树`api-device-plan`在`a16ea8a`，无在途写入者。集成agent`/root/integrate_next`最近任务完成、主树写入权已释放，后续派发前核对实际状态。
- 下一步：F01规划审计→F01实现→A02→原bootstrap精确集成→WS子卡→S01父收口→设备三子卡→全局设置/health及G-API总收口。P4以后仍按真正DAG，不绕过G-API；本轮远未全部完成。

## 当前恢复点（2026-10-22）

主目录 `/home/lijx/Projects/RelayNest`，最近已确认 main `99db13e5a0426f8e19407a43500b17ebb8390058`。后续集成进行中，恢复时先查实际 HEAD、agent 和 registry，不能以本快照覆盖更新状态。

- P00-01/02/04、P02-01～04、P03-01～06及08～12均已独立审计并集成。各精确实现/审计/集成SHA以registry及对应报告为准。
- P03-07-PLAN01：实现 `2fa5e97b8fb165ef0df3333ce9575bdacbb6167e`，独立审计 `ccbf114f4b6c82e4fcb73420281096596a7e7817` PASS，集成 `ca56c5e3a4c98f3ba88b089b2059024a6e93b324`。三个串行子卡S01同步/已读/WS、S02设备注册/通知、S03全局配置/健康投影及全契约收口已登记。父卡和G-API尚未完成。
- S01首轮只预读，API未改，预算退出checkpoint提交 `e1a0d78aaa68318a4063725f05e79272507fd993`。不能将其算实现完成。
- G-GIT=pass、G-MP=pass、G-FCM=pass（仅历史PoC限定范围）；G-API/CORE/DEPLOY/SHELF/SHELF-API/OPPO=pending。G-FCM不代表正式同步或AC验收；原E01～E07/NOT_RUN保留。
- 用户已明确OPPO账号/应用权限尚未准备，P02-05及G-OPPO受阻。仅阻相关分支，不自动注册/提交材料/读取凭据，不阻本地核心开发。

## 在途任务与目录归属

| 任务及owner | 目录/基线 | 下一步 |
| --- | --- | --- |
| P03-07-S01-PLAN01 / `/root/sync_s01_plan` | `/home/lijx/.codex/worktrees/api-delivery/RelayNest`；`ccbf114f4b6c82e4fcb73420281096596a7e7817` | 精简派发输入或保留父验收细分S01，独立审计集成后重启实现；禁止修改共同API |
| P00-03报告已集成 | main集成`bf1421383abd46bfaa1e803c1ecfdf63e80a9c46` | 报告实现`46fdca5ff0788e4e8b51750ac73a3e2bca10e022`、审计`d91e6ae0ac30d770d6621d194fb5b861d4802368`；完整环境BLOCKED |
| P00-05报告已集成`99db13e5a0426f8e19407a43500b17ebb8390058` | `/home/lijx/.codex/worktrees/p2-evidence/RelayNest`；报告`50bf88d3c133bf564415ad03ebf0220a429d81d6`、独立审计`5faf52c42725d0d0bf9ee765af6523e6a3a20dd0` | 报告PASS，环境BLOCKED；不能据此放行部署 |
| S01已释放实现树 | `/home/lijx/.codex/worktrees/api-sync-s01/RelayNest`；HEAD `e1a0d78aaa68318a4063725f05e79272507fd993` | API未改，checkpoint已提交；恢复时先确认无写入者，新实现取最终main基线 |

P00-03缺RepoDigest、完整重建证据、独立隔离实例及当前现场确认，不因此撤销P1用户例外。P00-05缺可信只读入口、独立测试资源及HTTPS Origin清单；当前无已配置SSH入口/fnos工具，未尝试猜连接或变更现网。真正TLS/WS/上传/LongPolling等继续NOT_RUN移交P11/P12。

## 继续执行与保护事项

- 先恢复实际在途agent及工作树，依次S01规划/审计/集成→S01实现/独立审计/集成→S02→S03→父卡G-API收口；之后按真实DAG启动P4～P10。局部外部阻塞不影响独立分支。
- Web静态资源为Go embed输入；干净Go构建前必须先`npm --prefix web ci --no-audit --no-fund`及`npm --prefix web run build`，不得借旧dist缓存假通过。
- SQLite已分配`0001_initial.sql`，下一迁移编号0002；API、依赖和迁移写入串行。
- 主目录registry.json包含用户既有未提交内容及调度更新，不整文件擅自stage/提交；`.directory`属于用户未跟踪文件，不动。root仅更新台账及本短摘要，Git提交交集成agent显式路径处理。
- 旧/tmp工作树可能已消失，Git对象/分支/管理引用保留，不prune/remove/clean；新工作优先持久受管worktree。不能据旧锁自动抹除工作。
- 本文件只是恢复快照，不代表全部完成、无人值守自动运行或Release授权。任何源代码变更必须重新独立审计精确SHA，集成确认后才done。
