# 总调度恢复 checkpoint

更新：2026-10-22。本文件只记录恢复信息；唯一任务台账为 `registry.json`。不是全部完成报告。

## 授权与用户决定

- 持续推进P3～P15及必要P0/P2收口。P13～15仍待核心实际完成及G-SHELF实质门槛，P12-06准备报告本身不足。
- P1按用户最小PoC例外PASS，G-MP保持；方案A仅既有显式助手补丁，只有v3.1.5有运行证据。旧NOT_RUN、最低目标v3.0.0与拟议>=3.0.0声明分别保留，不冒充AC-07/P10/P11正式通过。
- MP恢复镜像与Compose声明不同；重建/升级可覆盖补丁，/netdisk曾经用户授权移除，不自动恢复或重建。
- 用户已批准固定通用title+content、显式可选vendor、超时不跨FCM自动补响、切换仅未来新事件、在途残余及离线DND/已读/退出撤回限制。厂商资格/API/SDK/签名/分类额度/错误码仍须PoC；不代注册/身份提交/平台授权。
- 用户明确“真机运行留到最后验收的时候”：P03-03初始化不因真机E01阻塞，但安装/冷启动/深浅色/系统栏NOT_RUN须移交P09/P11/P12正式验收。真实正式签名成功路径也仍NOT_RUN。
- 现网部署、MP重建升级、破坏性数据、公开发布、远端推送/Release最后一步须具体结果审查确认。未创建新聊天、自动化或定时监控。

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
