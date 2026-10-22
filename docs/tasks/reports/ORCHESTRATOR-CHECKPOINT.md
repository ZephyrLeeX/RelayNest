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

最近确认主树 `main@876935a219c3a1bf828aeaabb2e8a82ca2bb5d4a`。唯一台账仍registry，恢复时先核实际HEAD/agent/目录及未提交内容，不用本快照覆盖新状态。本轮持续执行，远未完成P3～P15。

- P00-01/02/04、P02-01～04、P03-01～06及08～12均已独立审计精确集成。G-GIT/G-MP/G-FCM=pass（历史PoC限定），G-API/CORE/DEPLOY/SHELF/SHELF-API/OPPO=pending。
- P03-07父running；S01同步域已done，三子卡及八项父契约验收均通过独立审计和精确集成。S02设备域running；S03全局配置/健康/正式G-API收口尚未实施。

| S01子卡 | 最终实现 / 独立审计 | main集成 |
| --- | --- | --- |
| sync/read | `966066831a6b1b75720cae131b2e5d17c22e6e35` / `d51bceebcf4a81637920efd9c60b802a36943ee1` | `33cc7585e873a5debaddc5b93c8657b60d5a73a8` |
| bootstrap含F01 | `a379d9620c9a0b616c575d749fd05030376437c3` / A02 `0993016dd18b120ce4c300c7b86087474e5eee9e` | `3cdb5e228b7d124890d45250728b260b2091a6db` |
| WS及父联合收口 | `169fd8043e48e37762f11e845f00f56ce8024933` / `91215226a724679ca163be5279c049cb6bba3ad7` | `5cdaf15f7e01b663dd0f2b68fc044d9e875aaa79` |

bootstrap原实现9cb95c4经A01 532ea26发现BS-A01-01（seed可冒充实际追赶完成）；F01规划/独立审计/分支集成后修复，A02独立重现旧问题并验证来源对照/零变化追赶/新事件链，最终准确七提交链入main，原负面报告保留。问题仅在契约范围关闭，运行时NOT_RUN。WS开发/审计读取输出预算偏差如实记载，不假称流程完全合规。不得因这些已闭环工作被旧快照描述为未完成而重跑。

## 在途工作区与下一步

- 当前实现为`P03-07-S02-S02`完整provider注册生命周期，owner `/root/push_registration_resume`；独占`/home/lijx/.codex/worktrees/api-push-registration/RelayNest`、`codex/api-push-registration`，精确提交基线`876935a219c3a1bf828aeaabb2e8a82ca2bb5d4a`。前worker额度耗尽，用户明确恢复后继续；实际agent列表无旧写入者。恢复检查发现openapi.yaml、validate.py两文件本卡未提交修改（当时314增1删），index为空，已交新worker接续保留；这些修改尚无测试/提交结论，不能丢弃或冒充已验证。运行中后续文件变化以实际状态为准。
- 前设备卡P03-07-S02-S01已done：原实现`12d9cee328eda93e16a689fa4638c0eba6b06482`、负面A01 `de2a6f915e20354e16738f3d17736d50f16894c8`发现DS-A01-01 P1。规划/独立审计/分支集成后F01最终`a63bd59c8f50dcd83353e279b044f25f4383780c`经A02 `c05a5b93b64b5348bb98270ade4856fc23061fac` PASS，七提交组合main集成`876935a219c3a1bf828aeaabb2e8a82ca2bb5d4a`。DS-A01-01仅在契约范围关闭，负面审计保留，不重跑此闭环。
- 已冻结设备会话语义：撤销路径{id}使用Session.id，device_id只是元数据，同设备可多Session；精确撤销只作用指定代次全部provider/WS，迟到旧S1不回退选新S2。未新增单活/登录替换规则，所有components/旧27操作/637例和锁保留。后注册/通知不得假定device_id唯一Session，归属按已确认T9.3另核。
- 当前注册卡必须整体完成写入/恢复读/条件删除/初始CAS/同版幂等与冲突/删除持久版本屏障/耗尽/迟到结果隔离，通知模式和完整发送快照留下一卡。提交后不同agent固定SHA独立审计，修复后重审，再精确main集成，才派S02-S03；本卡尚未done。
- 当前共同契约冻结：29操作/84Schema/687例、全部旧组件/两requirements锁、Sequence SQLite64字符串/Version>=1、权限/no-store、H/epoch/floor/seed与真实sync完成来源、WS仅提示不可推进游标。后卡按实际基线重验，不依赖历史计数猜当前状态。
- S02文档规划已独立审计集成main`d927abed945379e00548b01e7cecf7bbef59bac2`，三子卡：S02-S01会话→S02-S02完整provider注册生命周期/CAS/条件删除/恢复/持久版本屏障→S02-S03通知CAS/恢复/设置union/发送快照联合收口。两后卡todo；父S02保持原八验收。
- S03文档规划已独立审计集成main`e6b0565ec3be85997d9b7daa66bce9ad7cec2ab8`，三子卡：S03-S01全局settings/CAS/changes→S03-S02根health与管理投影→S03-S03全T3/T8/父验收/G-API证据补缺收口。全部todo，实施等设备父完整收口，不提前冻结设备字段。
- 每卡实现独立提交→不同agent精确SHA审计→编号修复及重审→集成agent准确组合验证→root登记done→下一卡。S03最终和父P03-07实质验收后才评估G-API，之后P4～P10按真实DAG；不提前启动服务端实现绕过契约门槛。
- 集成agent `/root/integrate_next` 最近WS集成已完成、主树释放；旧集成agent会话已不在实际列表；本checkpoint拟由新`/root/integration_resume`作为SCHEDULER-SNAPSHOT-06单独提交。恢复时再核实际在途状态，不能根据名字猜运行中agent。

## 外部条件、环境与文件保护

- P00-03环境报告/独立审计已集成main`bf1421383abd46bfaa1e803c1ecfdf63e80a9c46`，完整环境仍BLOCKED：缺RepoDigest/重建证据/独立实例/当前现场，不撤销P1用户例外。
- P00-05报告/独立审计已集成main`99db13e5a0426f8e19407a43500b17ebb8390058`，用户最新确认可信NAS只读入口、独立测试资源及HTTPS Origin尚未准备，部署验收前提供。不要重复索取/猜连接/自动登录；不阻本地开发。TLS/WS/上传/LongPolling现场实测NOT_RUN。
- 用户确认OPPO账号/应用Push权限尚未准备，P02-05/G-OPPO受阻，仅阻厂商实测/激活；真机运行留正式验收，正式签名成功/安装界面/端到端NOT_RUN保留。不能将Schema/合成状态或阶段放行冒充HTTP/SQLite/Room/WS/通知实际运行验收。
- 锁Python可先核`/tmp/relaynest-p0305-f01-locked/bin/python`，缺失依hash锁恢复不升级。Android/mise版本沿原锁；干净Go构建前先Web npm ci/build生成gitignored embed资源，不靠旧dist。SQLite已占0001，下一迁移0002；共享API/依赖/迁移串行。
- 主树registry.json含用户原有修改及调度更新，`.directory`为用户未跟踪文件，均不stage/覆盖/丢弃。本摘要由root创建，Git提交必须交集成agent显式路径处理，禁止git add .。
- 各已审源树及分支保留，旧/tmp可能消失但Git对象/管理引用不prune/remove/clean；恢复先核未提交内容与实际锁，不自动重置。当前仅provider注册树具有API写入权。
- 本文件不是全部完成报告，不创建新聊天/定时任务/自动化，也不授权push、Release、真实部署或MP重建升级。P13～15虽已授权，仍等核心实际完成/G-SHELF实质门槛，不改RelayShelf后端/数据库。
