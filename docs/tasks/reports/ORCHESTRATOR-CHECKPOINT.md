# 总调度恢复 checkpoint

更新：2026-10-23。本文件只记录恢复信息；唯一任务台账为 `registry.json`。不是全部完成报告。

## 授权与用户决定

- 持续推进P3～P15及必要P0/P2收口。P13～15仍待核心实际完成及G-SHELF实质门槛，P12-06准备报告本身不足。
- P1按用户最小PoC例外PASS，G-MP保持；方案A仅既有显式助手补丁，只有v3.1.5有运行证据。旧NOT_RUN、最低目标v3.0.0与拟议>=3.0.0声明分别保留，不冒充AC-07/P10/P11正式通过。
- MP恢复镜像与Compose声明不同；重建/升级可覆盖补丁，/netdisk曾经用户授权移除，不自动恢复或重建。
- 用户已批准固定通用title+content、显式可选vendor、超时不跨FCM自动补响、切换仅未来新事件、在途残余及离线DND/已读/退出撤回限制。厂商资格/API/SDK/签名/分类额度/错误码仍须PoC；不代注册/身份提交/平台授权。
- 用户明确“真机运行留到最后验收的时候”：P03-03初始化不因真机E01阻塞，但安装/冷启动/深浅色/系统栏NOT_RUN须移交P09/P11/P12正式验收。真实正式签名成功路径也仍NOT_RUN。
- 现网部署、MP重建升级、破坏性数据、公开发布、远端推送/Release最后一步须具体结果审查确认。未创建新聊天、自动化或定时监控。

## 当前恢复点（2026-10-23）

最近确认主树 `main@b86ad2eba8ca5c9afc76ba65159de974ed968339`。唯一台账仍registry，恢复时先核实际HEAD/agent/目录及未提交内容，不用本快照覆盖新状态。本轮持续执行，远未完成P3～P15。

- P00-01/02/04、P02-01～04、P03-01～06及08～12均已独立审计精确集成。G-GIT/G-MP/G-FCM=pass（历史PoC限定），G-API/CORE/DEPLOY/SHELF/SHELF-API/OPPO=pending。
- P03-07父running；S01同步域已done，三子卡及八项父契约验收均通过独立审计和精确集成。S02设备域已契约done；S03全局配置/健康/正式G-API收口running，首子卡在途。

| S01子卡 | 最终实现 / 独立审计 | main集成 |
| --- | --- | --- |
| sync/read | `966066831a6b1b75720cae131b2e5d17c22e6e35` / `d51bceebcf4a81637920efd9c60b802a36943ee1` | `33cc7585e873a5debaddc5b93c8657b60d5a73a8` |
| bootstrap含F01 | `a379d9620c9a0b616c575d749fd05030376437c3` / A02 `0993016dd18b120ce4c300c7b86087474e5eee9e` | `3cdb5e228b7d124890d45250728b260b2091a6db` |
| WS及父联合收口 | `169fd8043e48e37762f11e845f00f56ce8024933` / `91215226a724679ca163be5279c049cb6bba3ad7` | `5cdaf15f7e01b663dd0f2b68fc044d9e875aaa79` |

bootstrap原实现9cb95c4经A01 532ea26发现BS-A01-01（seed可冒充实际追赶完成）；F01规划/独立审计/分支集成后修复，A02独立重现旧问题并验证来源对照/零变化追赶/新事件链，最终准确七提交链入main，原负面报告保留。问题仅在契约范围关闭，运行时NOT_RUN。WS开发/审计读取输出预算偏差如实记载，不假称流程完全合规。不得因这些已闭环工作被旧快照描述为未完成而重跑。

## 在途工作区与下一步

- 当前中断卡为`P03-07-S03-S02`公开健康/管理员状态契约，旧owner `/root/health_contract_dev`已不在实际agent列表。工作树`/home/lijx/.codex/worktrees/api-health-contract/RelayNest`、branch `codex/api-health-contract`、HEAD/基线`2fc964dc2c967f6ac53b1d2512d0e0d3ff2734e7`。保留四API文件未提交修改及本卡未跟踪implementation报告；没有最终实现SHA或独立审计。恢复先由集成角色核对归属，之后新开发agent继续，不能丢弃/重置。待完成明文响应/错误例、变异验证和旧34操作/100Schema/899例与依赖锁冻结比较，所有已有测试自述须重新确认。
- ORDER01文档调整已独立审计集成`2fc964dc2c967f6ac53b1d2512d0e0d3ff2734e7`，健康卡可先行，但共享四API文件仍严格串行；最终S03-S03依赖全局设置和健康两卡均done。
- 用户已接受全局设置正整数政策，POLICY01最终文档`7c2d995c8f045ecb31f87065b1a7e90eaf5f0e86`、审计`5b850f156785805f25d07e4d28e04cb853b05715`已集成main `b86ad2eba8ca5c9afc76ba65159de974ed968339`。正数天/字节以SQLite64规范十进制字符串表示；0不支持；默认30天/10MiB/50MiB；全局设置专用CAS保留JSON整数，上限2^53−1，公共Version不变；到龄判断避免超大天数乘法溢出，匹配但耗尽409原子拒绝。技术方案以主树PRD/设计/卡和POLICY01-finalization报告为准，不再询问同一政策。
- 全局设置实现卡S03-S01仍todo：原开发只留下checkpoint，尚未实现API。旧`api-global-settings`树保存POLICY01证据，不重置。健康卡提交→独立审计→精确集成后，由集成角色从实际最新main准备新的全局设置实现工作树，按finalization精简读取配方派全新worker。
- 设备父P03-07-S02已done，三卡精确集成链/父八项契约验收经独立审计核对完整。设备会话最终F01`a63bd59c8f50dcd83353e279b044f25f4383780c`、A02`c05a5b93b64b5348bb98270ade4856fc23061fac`、main`876935a219c3a1bf828aeaabb2e8a82ca2bb5d4a`；注册实现`6901bc09de53995435102d11f2dfad9bd3f46059`、审计`14081bcb11a5ce8c7ce7b700b6b1ca6e10bdf1e1`、main`82ebd62634ecf9025670a75aa956a45667c55ea7`；通知联合实现`692e11964ea74cb84cf62470a08e4c3a1780a01f`、审计`36a05a2523a968f384e82de4eb599a10f31226eb`、main`42cd918c01c34bb7baa918420dad0daa2b4d6a98`。DS-A01-01契约关闭，原负面审计保留；读取输出/预算偏差如实保留，不声称流程全合规。不得按旧快照重跑已完成卡。
- 设备冻结交接：撤销按Session.id，device_id不唯一Session；通知偏好按设备共享，注册按Session.id/provider隔离，初始CAS0、删除递增持久屏障，旧未知结果不改新注册，token不回显。GET/PUT current/notifications、配置CAS/恢复/vendor服务器前置、device.notifications.updated、DND/前后job快照CAS/租约串行仅契约及合成状态，真实worker/平台NOT_RUN。D1～D3/FCM边界保持，当前全局设置卡只补剩余全局变更，不弱化设备域。
- 当前共同契约冻结：34操作/100Schema/899例、全部旧组件/两requirements锁、Sequence SQLite64字符串/Version>=1、权限/no-store、H/epoch/floor/seed与真实sync完成来源、WS仅提示不可推进游标。后卡按实际基线重验，不依赖历史计数猜当前状态。
- S02文档规划已独立审计集成main`d927abed945379e00548b01e7cecf7bbef59bac2`，三子卡：S02-S01会话→S02-S02完整provider注册生命周期/CAS/条件删除/恢复/持久版本屏障→S02-S03通知CAS/恢复/设置union/发送快照联合收口。三卡已done；父S02原八项契约验收完整收口，运行时NOT_RUN。
- S03文档规划已独立审计集成main`e6b0565ec3be85997d9b7daa66bce9ad7cec2ab8`，三子卡涵盖全局settings/CAS/changes、根health/管理投影以及最终证据补缺。ORDER01已改为健康先行，S03-S02中断恢复，S03-S01等待共享API窗口；S03-S03须两者done，按精确实际基线串行。
- 每卡实现独立提交→不同agent精确SHA审计→编号修复及重审→集成agent准确组合验证→root登记done→下一卡。S03最终和父P03-07实质验收后才评估G-API，之后P4～P10按真实DAG；不提前启动服务端实现绕过契约门槛。
- 2026-10-23恢复时实际agent列表仅root；旧名字不代表运行。恢复核查角色`/root/recovery_verify`已派出，后续实际owner/锁以registry为准。

## 外部条件、环境与文件保护

- P00-03环境报告/独立审计已集成main`bf1421383abd46bfaa1e803c1ecfdf63e80a9c46`，完整环境仍BLOCKED：缺RepoDigest/重建证据/独立实例/当前现场，不撤销P1用户例外。
- P00-05报告/独立审计已集成main`99db13e5a0426f8e19407a43500b17ebb8390058`，用户最新确认可信NAS只读入口、独立测试资源及HTTPS Origin尚未准备，部署验收前提供。不要重复索取/猜连接/自动登录；不阻本地开发。TLS/WS/上传/LongPolling现场实测NOT_RUN。
- 用户确认OPPO账号/应用Push权限尚未准备，P02-05/G-OPPO受阻，仅阻厂商实测/激活；真机运行留正式验收，正式签名成功/安装界面/端到端NOT_RUN保留。不能将Schema/合成状态或阶段放行冒充HTTP/SQLite/Room/WS/通知实际运行验收。
- 锁Python可先核`/tmp/relaynest-p0305-f01-locked/bin/python`，缺失依hash锁恢复不升级。Android/mise版本沿原锁；干净Go构建前先Web npm ci/build生成gitignored embed资源，不靠旧dist。SQLite已占0001，下一迁移0002；共享API/依赖/迁移串行。
- 主树registry.json含用户原有修改及调度更新，`.directory`为用户未跟踪文件，均不stage/覆盖/丢弃。本摘要由root创建，Git提交必须交集成agent显式路径处理，禁止git add .。
- 各已审源树及分支保留，旧/tmp可能消失但Git对象/管理引用不prune/remove/clean；恢复先核未提交内容与实际锁，不自动重置。当前API写入锁仅健康契约树，恢复核查后由新worker接续。
- 本文件不是全部完成报告，不创建新聊天/定时任务/自动化，也不授权push、Release、真实部署或MP重建升级。P13～15虽已授权，仍等核心实际完成/G-SHELF实质门槛，不改RelayShelf后端/数据库。
