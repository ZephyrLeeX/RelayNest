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

最近确认主树 `main@cfc43c7f1bc63221d2738ad612b590ddd22f469b`（P04-02-S01实现/审计精确集成及I01后，snapshot17前）。唯一任务台账仍registry；本轮持续执行，远未完成P4～P15。

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
- 各已审源树及分支保留，不prune/remove/clean/reset。session-store源树已释放，HEAD7e6a8d5cfd59d6069711fd3106ebc2ec718933bb，branch codex/session-store。main实际0001/0002/0003已占，下一候选0004尚未分配；S02不提前建树，等来源安全前置最终实现独审集成。
- 本文件不是全部完成报告，不创建新聊天/定时任务/自动化，也不授权push、Release、真实部署或MP重建升级。P13～15虽已授权，仍等核心实际完成/G-SHELF实质门槛，不改RelayShelf后端/数据库。

## 当前P4派发点（恢复时以registry和实际agent为准）

- 主树snapshot13 `fd7ca5173c540796aa97e3efb9bf90a7adc94c32`，G-API契约pass已由root据最终独审/集成登记，P03-07及S03done。P04-01-PLAN01规划最终`73d4fa47c02a4478e3a81acf0f60f2740e63f252`、审计`34acc46f40d425f5bda8296a4bd8f11b1c43caeb`、集成`c69d8304277fb9f1ab9a9409e708acac5a0573a6`/I01 `18d3f3ad5c5e3ca64ed3f7bfa45546bb86ac5d9e`，已done。
- P04-01原实现9a97d7e、A01 b7fce9d、F01最终9af6a70c96b106b4a8f8d48e0913a80aa5bed68d、独立A02 PASS f4767d6cc640d3a5859ad0371860c8dcadcf8081已精确集成至main组合3ba2b496ae21737d15c8442a1fd56429513da1d4，I01 d3acec6779b6ce5377737575cdf51f550d41b719。无冲突，18路径blob同源/14源码依赖同受审修复；SIGINT termios问题A02关闭。锁Web ci/build、Go1.27.3/local三包非缓存race/vet、主树二进制真实PTY25取消与独立CLI竞争回归均PASS。root已据证据登记P04-01 done；本快照不改registry。非Linux实际运行/真机/会话正式AC等NOT_RUN保留。
- 开发交接待审：固定管理员id=1、username原样区分大小写不trim；Argon2id64MiB/3/1；generation int64从1至MaxInt64耗尽原子拒绝reset；摘要与代次同事务；提供ReadAdministrator/InitAdministrator/ResetPassword。0002本实现占用但待精确集成，下一编号0003仅集成后确认。Go1.27.3/SQLite锁保持，必要x/crypto0.58.0、x/term0.47.0、传递x/sys0.49.0为本实现依赖，须审计核对。真实CLI/SQLite与未测会话/WS/手机等分开。
- P04-02-PLAN01文档规划71682be3f1f91ea0a05b2a6237a6d68d218bd6e6、独立A01 PASS 100f9314d7137d298d049c139318b290ceed95e9已精确集成；main组合b5519a6b339303ed11fbabbf06cbfdf5990c6a4d，I01 d6a30a7eca7149101ea7ac59fc3a6ad523812788。五文档blob/链接/原五验收通过，server/API未动，无父子依赖环。建议S01依P04-01+PLAN01、S02依S01，父保留P04-01并汇合两子卡；沿父G-GIT，root登记。旧session-plan规划树保留；S01已完成最终独审精确集成，S02仍等独立来源安全前置，不编造尚不存在的接线接口。
- 干净Go检查先Web npm ci/build生成gitignored embed dist；工具沿mise锁，测试用临时DB，不连接生产。root唯一写registry，含用户原dirty内容不stage；`.directory`不动。本快照由集成角色仅显式路径提交。持续推进P4～P15，不把该恢复点称全部完成。
- S01迁移夹具范围补充已独审精确集成：实施d9c791bad99bf508beafd0a5785f0d5f74b0920f、A01 PASS4ee17de23341e5a8bda8ed63876f628788555090；main组合1620289997cba0b451441a0390777995d0e50448、I01 533f79b3b7aff9fb861e6f72204f9202f7637135。仅S01卡/实施/审计三文档同源，原验收/链接通过，源码/API未动。新增storage_test.go真实迁移夹具维护权限，旧升级/回滚/锁/checksum强度不减；不授权storage实现或额外依赖。
- P04-02-S01实现1da5e72aed508e01fb0d172b79fba0052af61a52、A01预算checkpoint349ee276e22336466b2d4a317bac88dc3e47151e、A02最终PASS7e6a8d5cfd59d6069711fd3106ebc2ec718933bb已无冲突精确集成；main组合9269876a5b57f18cc43d8100e88dc0f4a0b3b4ca、I01 cfc43c7f1bc63221d2738ad612b590ddd22f469b。A01仍BLOCKED历史checkpoint，A02组合独立证据完成PASS；root据证据登记S01 done，不代表P04-02/S02完成。
- 本轮组合：Web锁Node22.23.2/npm10.9.8 ci/build通过；Go1.27.3/local三包非缓存race(auth7.229/storage2.178/cmd2.397秒)及vet/diff通过。7路径blob同源、5实现路径同1da，API6/依赖/CLI/0001/0002不变，唯新增0003。S02须把密码校验观察代次传Create；Inspect后其它校验成功仍须Authenticate重新查库续期，expectedClient依实际凭据通道决定。真实HTTP/客户端/正式AC等NOT_RUN。
