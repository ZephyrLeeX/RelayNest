# 总调度恢复 checkpoint

更新：2026-10-22。本文件只记录恢复信息；唯一任务台账为 `registry.json`。不是全部完成报告。

## 授权与用户决定

- 持续推进P3～P15及必要P0/P2收口。P13～15仍待核心实际完成及G-SHELF实质门槛，P12-06准备报告本身不足。
- P1按用户最小PoC例外PASS，G-MP保持；方案A仅既有显式助手补丁，只有v3.1.5有运行证据。旧NOT_RUN、最低目标v3.0.0与拟议>=3.0.0声明分别保留，不冒充AC-07/P10/P11正式通过。
- MP恢复镜像与Compose声明不同；重建/升级可覆盖补丁，/netdisk曾经用户授权移除，不自动恢复或重建。
- 用户已批准固定通用title+content、显式可选vendor、超时不跨FCM自动补响、切换仅未来新事件、在途残余及离线DND/已读/退出撤回限制。厂商资格/API/SDK/签名/分类额度/错误码仍须PoC；不代注册/身份提交/平台授权。
- 用户明确“真机运行留到最后验收的时候”：P03-03初始化不因真机E01阻塞，但安装/冷启动/深浅色/系统栏NOT_RUN须移交P09/P11/P12正式验收。真实正式签名成功路径也仍NOT_RUN。
- 现网部署、MP重建升级、破坏性数据、公开发布、远端推送/Release最后一步须具体结果审查确认。未创建新聊天、自动化或定时监控。

## 已集成基线

主目录 `/home/lijx/Projects/RelayNest`，`main@38d52c3bae74010167d4defe0e681e843d497fc8`。

| 原任务 | 状态 | 实现/最终实现 | 集成报告提交 |
| --- | --- | --- | --- |
| P00-01 | done | `522dbeb3cb96c645a1351e05452a01df54100093` | `b0b614723d8055247d77fbeaa9c6dd88214d3c27` |
| P00-02 | done | `a2f3c3dc6cff6bcfe2265118c7e1a7019f01365c` | `1f0ea95004062e9ef3b88ea9ce6d4cbb30695484` |
| P00-04 | done | `7cf1a3148ffebfd05cc8f4d6d5a6ae9b5ed6fadc` | `fefad6f1ced17aa7a2dd328f8683d1ae8397f1fe` |
| P02-01 | done | `c49acb359e9170fc9263c2649322d62261092d95` | `dd6d6fc15eb1f5a71b0cf39752907751ed97198a` |
| P02-02 | done | `bcc42505b8a0335d328d1ac42f1f647c5e7759c4` | `6bdd6ef540494321ed6e22015212f9a93bfd6bdc` |
| P03-01 | done | `47d33bb7f5ca356947f4107d81429b9ace66dac1` | `f3dbe794f8afe32573435a94e363eb7a3761924f` |
| P03-02 | done | `6e9171946099457aad4f7696700927bd5751e77d` | `6d4ac84dd8d1b083b927505a9a97c975aa3e9d44` |
| P03-03 | done | `e002339fb085a6f5f8ac349c6b077d5751b3708f` | `1e16cffa3c6d15d8f01ab8e8bc501a0c0232aef6` |
| P03-04 | done | `6eac6bfdd0f2e27e899269e07b4a7746dc0d8958` | `30efad4f16234c07f42a2dd6e746d9d980b7dadf` |
| P03-05 | done | `5d170b84ef77cb6560d594341fa35bb0dab5bd7c` | `38d52c3bae74010167d4defe0e681e843d497fc8` |
| P03-08 | done | `e2d56baedad2d28c6d4b8fda0d018df49c274062` | `abd45c273b4d6feb0cd1c701ce742036dd04add3` |

P03-03含签名修复F01经A02 PASS；P03-08含WAL并发初始化修复F01经A02 PASS。历史负面审计不改写。PUSH-PLAN-01、PUSH-DECISION-01及P03-03/P03-08/P03-05修复规划均已精确审计集成；不代表未实现功能通过。

当前门槛：G-GIT=pass；G-API=pending；G-MP=pass；G-FCM=pending；G-CORE=pending；G-DEPLOY=pending；G-SHELF=pending；G-SHELF-API=pending；G-OPPO=pending。

## 在途工作区

恢复时先重新核对实际agent/worktree。当前两个在途agent及下一张已准备卡如下；总调度正在仅提交本checkpoint，不会提交registry用户差异。

| 任务/owner | 工作树 | 精确基线 | 状态/下一步 |
| --- | --- | --- | --- |
| P03-09 / `/root/go_ci_dev_r3` | `/home/lijx/.codex/worktrees/go-ci/RelayNest` | `6d4ac84dd8d1b083b927505a9a97c975aa3e9d44` | running；GoCI开发，同job先Web构建再Go检查；远端GH NOT_RUN |
| P03-11-A01 / `/root/android_ci_audit_r3` | `/tmp/relaynest-run3/android-ci` | `491fc873657006f3f037b57175b35783a2ea388f` | running；Android debug CI独立审计，远端GH NOT_RUN |
| P03-06 / `None` | `/home/lijx/.codex/worktrees/api-delivery/RelayNest` | `38d52c3bae74010167d4defe0e681e843d497fc8` | ready；待派新worker，投递与失败管理OpenAPI；不得越过依赖提前冻结同步设备契约 |

## 下一批与阻塞

- P03-05最终实现5d170b84ef77cb6560d594341fa35bb0dab5bd7c经A02 b7b02d023da5c25d0031d39bc833ec265a77b761 PASS后精确集成，F01关闭；旧负面审计保留。契约不等于运行时HTTP/事务通过。
- P03-06依P03-05最终集成，P03-07后才评估G-API。P4～P6不越过契约冻结门槛。
- Web交接确认Go构建必须先npm --prefix web ci --no-audit --no-fund，再npm --prefix web run build生成server/internal/webui/dist；P03-09/10实现须待P03-02审计/集成组合，不能靠旧dist缓存假通过。
- SQLite迁移已分配0001_initial.sql，下一编号0002；共同契约/依赖/迁移串行。
- P02-03/04仍review，仅缺必要独立历史证据审计/集成，不重跑真机矩阵。旧实现分别f65efaed77a3f772c2bd6e12e0f03680b7a3f945、879aaebffb1e312db78caac1978e4fea23b80d4d；G-FCM仍pending，未测项保留移交正式验收。
- P02-05/G-OPPO缺资格、应用与Push权限、受控配置安全引用及实际能力证据，只阻OPPO分支。P00-03/05环境收口、后续CI和插件检查等仍按DAG派发。

## 文件保护与环境恢复

- 主目录registry.json启动前已有用户未提交修改，本次总调度元数据叠加保存；不整文件擅自提交。用户.directory未跟踪，不提交。此checkpoint为总调度创建，交SCHEDULER-SNAPSHOT-01集成agent单独提交；不得git add .。
- 旧/tmp/relaynest-run2及更早工作树已实际缺失，旧分支/Git对象/管理引用全部保留，未prune/remove/clean。当前新审计树在/tmp/relaynest-run3；GoCI与P03-06已使用create_worktree持久受管目录，路径见表。
- 旧CI分支codex/ci-run2无新增提交、无暂存差异、stash空，旧目录缺失无法恢复未提交内容；新CI卡按真实现场重做，不能伪称此前完成。
- 当前任何在途目录可能因环境切换消失；先核对Git对象/agent/未提交内容再恢复，不能覆盖工作或重跑已完成PoC。必要工具先核查既有mise绝对路径，不凭历史报告声称当前可用。
- 历史辅助RECOVERY-01/WORKSPACE-01尚未单独归档证据，不凭聊天伪填审计或实现SHA；已核对的P0正式任务闭环不受影响。
