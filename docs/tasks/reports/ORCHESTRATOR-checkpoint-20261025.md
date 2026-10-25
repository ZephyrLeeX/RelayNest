# 总调度恢复点 2026-10-25

这是进行中恢复点，不是全部交付。root只维护调度；源码/测试/审计/集成交子agent。用户P3–15持续授权仍有效，4槽含root。

## 当前主分支与保护

main HEAD 592ea2d39b377838f1476dacf6343e42c691b275；index已释放。registry.json为用户原修改与root持续调度合成dirty，禁止整体stage/提交/覆盖；.directory原未跟踪不动。本文为root未提交调度摘要。独立原生worktree根/home/lijx/Projects/RelayNest-recovery-20261024/，禁止reset/clean/prune。app managed tree消失故障历史保留所有refs/artifacts。

## 本轮新闭环（不要重做）

- P07-06 Web设备页：实现5625a6b38f22aa529919d6dba944dc2b71e49b2b；A01 a09084e40f1f815d15c61003d0400673f3722e12 PASS；main组合1117a5cb0fedd9187e1be71b42038cc59540559a；I报告02d0aa6744b8162a911f4d709245c70d6d2266d9。真实HTTPS Chromium双context/CSRF两403/精确撤销/自撤/同install隔离及原auth回归通过。
- P08-04-D01-S01实例存储：实现f546060db41aabc4d4ff1fbfb1e7c91787d8ea19；A01 85609afaf027bad1e930bf491b02128ac2d6a5ae PASS；main组合eea5094237fdc24fd967948795a3cdf1a0d26888；I报告07779de822d1aa28ceeb324c1225499fcf20c9c4。0005_instance_identity已集成，4包race/真实双进程等通过，仅storage局部。
- P05-01-PLAN02两卡规划：8bcf4c6/A494c043/组合ae65b4d/I1aaaf779均真实，S02/S03已登记，父P05-01未完成。
- P05-01-S02 Schema：实现6d37fb8d021d1a03dd7d5e8416ba4fc2e28e8e09；A01 1b1edddf2cb5effadc87d2c586e27ae77372f8b7 PASS；main组合23b9e08e472a5e39f8fdcb17b998be6b57898cdf；I报告4f4cfd0ea4af1b9c71919af5dd9cba0d365d4111。0006_messages包含messages/deliveries/push_jobs，8包组合测试/vet/build与真实迁移通过，迁移锁released无新占号。S03必须实际同Tx回填created_change_seq及完整快照，不能提前验收。
- S02-SCOPE01只增instance_test.go两处迁移夹具授权，原断言不变；I fad32a126a4c0b95dde20c5cdc7b358f8a0dd188。
- B01-SCOPE01范围补充：实现e988ea3ccaac43a9c3fc54b5b312a03102752cd1/A8602f91/Ia346e033da54f7f302fcf2819c90d68737a9d5a7；BOT01新卡ready后已派实现。B01保持M01依赖并增加BOT01闭环；当前无附件真实route。
- M01-SCOPE01：实现455429e9aeab71795502ac9385c7445229e03272/A3c9953d78f23b3de22dc38714d43b6dcfe48d5a0 PASS；组合2c5c3e8d2657875a5664eb0a4144ae38a6474f87/I592ea2d39b377838f1476dacf6343e42c691b275。只授权新增storage/offline.go OpenOffline无DDL无迁移只完整现存DB；保原Store.Write/三个TxAPI；shared/exclusive flock仅本地协调，人工全部停服/隔离前置不变。

## 在途（必须先实际list_agents/status核对）

1. /root/message_store_atomic_dev，P05-01-S03，tree message-store，branch codex/message-store-20261025，base4f4cfd0ea4af1b9c71919af5dd9cba0d365d4111。只messages仓储/tests/report，无迁移/API。实现Bot授权复核+消息/seq/changes/最小pending同Store.Write原子及故障并发。未交付SHA/必跑检查未完成。
2. /root/instance_offline_maintenance_dev，P08-04-D01-M01，tree instance-maintenance，branch codex/instance-maintenance-20261025，base fad32a126a4c0b95dde20c5cdc7b358f8a0dd188（只有0005），四未跟踪本卡草稿auth helper/CLI parser+flock/tests/操作文档，原子撤销局部真实测试通过但完整CLI未完成。已正式通知scopeI592ea2d允许offline.go，继续接线。main0006的push_jobs(session_id,device_id) FK ON DELETE CASCADE；后续最终组合应补真实pending清除/回滚回归。不得自己丢草稿/刷新基线；若须刷新交集成角色。完整命令/clone/commit矩阵尚未完成，不算done。
3. /root/bot_api_identity_dev，P08-04-D01-B01-BOT01，tree bot-api-identity，branch codex/bot-api-identity-20261025，basea346e033da54f7f302fcf2819c90d68737a9d5a7。只bots内部未知BotID身份发现+tests，区分NoMatch/匹配后拒绝/存储失败，按卡真实SQLite/race/vet。无附件路由实现，不能冒称HTTP分流通过。刚派，尚无实现SHA。

三个实现完成分别新agent精确SHA独审，修复后重审，独立集成main组合检查后root done。任务与锁只以dirtyregistry为准，旧tree内registry可能todo，不误重跑历史。

## 继承用户决定与外部条件

P1/G-MP limited PoC已接受，v3.1.5运行证据/补丁指纹限定，target3.0.0与>=3.0.0市场声明不等已验证，NOT_RUN保持，MP权限/危险确认及命名空间userid隔离。P2/G-FCM历史limited真实PoC通过，不机械重跑。OPPO允许固定通用标题正文与显式可选模式，账号/app权限尚未准备，仅相关branch外部blocked。真机运行留最终验收；NAS/可信只读入口/独立部署环境/HTTPS Origin用户部署验收前提供，不反复追问。
G1具体合同/生命周期/有限保证和两附件Bot-first例外全部已批准，不重问：每protected call probe，瞬切旧Bearer字节可能抵达新部署窗口接受；backuprestore/clone人工停服换ID撤销会话/映射；两附件matched Bot拒绝不可fallback，storage503，NoMatch才IDfirst Android，其它AndroidIDfirst。
G-INSTANCE-CONTRACT pass但RUNTIME/COMPOSITION pending，P08-04/S04仍未完成。P13–15需实质核心完成与G-SHELF，不因准备报告提前释放。无现网部署/MP rebuild/破坏数据/公开发布/remote push/Release，最后一步需具体结果后用户确认。无自动化。

## 下一步

收三个在途交付并立即独审；M01最终I需0006组合；S03闭环后核父P05-01全部原验收再后续P05-02/P06依赖。BOT01闭环与M01都done后推进B01公开实例probe与Android服务端绑定；B01仍5file预算如超范围交有限拆卡。完整DAG/root当前registry权威，外部阻塞不阻独立分支。

## 再次额度恢复核对

main仍592ea2d，旧三个agent已全部不在，未产生新实现提交。新worker先保留并研究原草稿：P05-01-S03=message_store_atomic_resume（三untracked messages Go文件），M01=instance_maintenance_resume（main.go改动+offline/CLI/auth/docs草稿），BOT01=bot_identity_resume（api_identity.go草稿）。三个原worktree/branch/基线不变，新agent只续作，不以旧未完成测试当pass。root已登记真实agent_id/owner。

## 最新闭环与主HEAD更新

主HEAD现ed488e543d892f2353fbc95f72a2f0af98ba9a8d，main index释放。rootregistry、.directory、本文原保护不变。
- S03实现8d2442f1fcf23b5f592b90b399253ead2cf6e123/A2278782492c861cd086f3d6c448346d5a6366de8/组合9720ddc2550c2b142588ed33baf5481fef05c206/Ia47bdf4b44f6eacf7c9aadecbe7a20159095a0e5，done。
- 父P05-01独立A01 45046b5d4095b629e2b645caaf4a52fe423487f7 PASS，最终父I组合b09a19d910cb84001987cf6afda6da3738f7badc/I ed488e543d892f2353fbc95f72a2f0af98ba9a8d，已root登记done，原验收全部闭环但P5阶段/HTTP/G1/幂等/引用附件/P06provider未完成。
- BOT01实现63b673a4ba3fe66bac68b05909b60e856c1a8d52/A020ff797ec189bbfa7e4657ea8fb0dc1963a7945 PASS/组合529efe0814660553796da9d3f55f7b3e62e6e719/I4698f39dbe7e78f86cba209a0a44a750c9fdd8db，done。readTx matched COMMIT定点注入NOT_RUN保持，真实附件routes/资源域/fileopen未来。
- M01实现4a4ba2343a4e95baf77af62b60e56e1e28ffcf9f/Aa0df018110094a180c431e5ab456624a260edce4 PASS固定旧基线；最终main真实0006+S03+BOT01强制六迁移与pending=1的临时overlay普通+race通过，restore/clone清零、move保留、随机/UPDATE/DELETE/COMMIT/busy/expected/exhausted整体回滚，完整alltests五包race/vet/build PASS；组合173c43f1ede7cf121f6eb603767f5704811bbf68/I71b6e5ea3fd08b86f7af6848068a0dd975150161，done。B01开发树已native准备instance-server-bind/codex/instance-server-bind-20261025 at71b6e5，尚未派实施。

现之前三个developer任务全部已完成审集，勿恢复旧草稿。当前在途仅规划/审计（必须list_agents核真）：
1. instance_binding_scope_preflight：P08-04-D01-B01-PLAN02，tree instance-binding-plan/branchcodex/instance-binding-plan-20261025/base4698f39，docs-only有限实际routes/范围/预算核对。已发现原B01漏request_security.go/Bot列表需repo事务读快照，拟两直接实施卡AUTH01（authhttp/request_security/session/device_sessions/instancebinding，5主file）和HTTP01（router/probe/httpapiBots/botsrepo/main，5主file）。原父全部验收不变，AUTH01不得单独放runtime，最终M01/BOT01 done输入明确。尚未交implSHA。
2. message_create_plan_audit：P05-02-PLAN01-A01，在message-create-plan，base4698f39，固定实现09a551722a144fafa3fb8994caaa41b8302ae238(5docs父追加/S01幂等同Tx/S02BotHTTP/S03AndroidHTTP/报告)，独立审计进行中。S03强依赖B01最终I；新迁移串行核号不预占0007；父P05-01刚正式done可解依赖，规划A/I仍需完成才实施。

下一步收两规划A/I，先放P05-02幂等基础独立分支与B01AUTH01串行公共认证分支；各自worktree由集成准备，迁移号由集成核main与在途锁唯一分配。原B01prepared树基线71b6e5需在计划精确I后由集成刷新（空树），不自行覆盖已有内容。

## 当前执行包（取代此前在途段）

main HEAD a128b20449993b8d854faa4f498a2ca4f7dcd607，main index已释放，rootdirtyregistry/.directory/本文保护不变。
- P05-02-PLAN01：09a551722a144fafa3fb8994caaa41b8302ae238/Aa54e6e075bcb1e223a0886a4fd8b2f08201805d3/组合50be58eaaba6c8918dbd7642d9ef06125f0dda07/I137799bc9f3c94b279b94a0d18b01480bb4903bd，done。S01/S02/S03三有限实施卡已登记；S03强依赖B01，parent P05-02未done。
- 唯一0007_idempotency.sql由集成核35个实际trees/main/inflight分配，ALLOC报告8a70a31c6195ef02a5fc3dc43dc64db8be0dd330在idempotent-create树。迁移锁allocated owner idempotent_create_dev到S01最终I。
- 幂等fixture scope：实现d442840edaa4793f4c1a9e6f3fc25c630459ae4e/A2fd53182a6e574c48613831458c2e7e27eb55617/组合17514e9c90b854c90bb9572e7b13a13c7a2f296c/Ia128b20449993b8d854faa4f498a2ca4f7dcd607，done，root已正式通知developer允许storage_test.go目标版本过滤/旧ledger比较/完整真实迁移证明，不改instance_test/production/原断言。
- B01 PLAN02：c2cd6a5175d29e2450ef09aaf6837a7007c1e19c/A38c70ad933bdb1f3a7596a29a0fc0889766352f1/组合5c92be29ac7ae64acc8f58bfc59b0715a15188e2/I5519ebfee27e3ad74145466eb27051945804f07e，done。AUTH01→HTTP01两有限卡已登记，B01原所有依赖/验收不变，AUTH01不单独放runtime。

现真实在途（必须先list_agents+gitstatus）：
1. /root/idempotent_create_dev，P05-02-S01，tree /home/lijx/Projects/RelayNest-recovery-20261024/idempotent-create branchcodex/idempotent-create-20261025，devbase8a70a31c6195ef02a5fc3dc43dc64db8be0dd330/reviewbase137799bc9f3c94b279b94a0d18b01480bb4903bd。已写0007+idempotency/store+messages/create（三主）+tests草稿；定向SQLite同键/重开/reset/权限/隔离/9故障/COMMIT/overflow/tombstone等通过，真实v6→v7与checksum拒绝也通过，Webbuild完成。五包race/vet与storage夹具修后复跑仍在进行，尚未提交实现SHA/实施报告。新scope已生效勿重复问。
2. /root/instance_auth_binding_dev，P08-04-D01-B01-AUTH01，tree /home/lijx/Projects/RelayNest-recovery-20261024/instance-server-bind branchcodex/instance-server-bind-20261025，clean派发base5519ebfee27e3ad74145466eb27051945804f07e。仅auth/http.go request_security.go session.go device_sessions.go instance_binding.go五主+auth tests/report；私有绑定防伪、密码/Bearer前ID、同快照读取+受控续期写Tx/commit前ID+gen复核，保Web/Bot/device401差异。刚进实现，无SHA/未跑完整检查。router/probe/Botlist/main属于后续HTTP01，不提前宣称runtime通过。

下一步两个实现各独立新agent审计，修复后重审，集成精确组合后rootdone。S01最终I释放0007并派BotHTTP S02（共享router/main须与B01HTTP01顺序明确隔离基线/集成）；AUTH01最终I后才HTTP01公开probe+BotListPage同snapshot组合/8真实routes门控。父B01最后整体审集，继而AndroidN01等；P05-02AndroidS03等B01done。所有外部门槛与用户已批G1/OPPO/真机后置等保持原义。

## 用户要求进度文档后的真实快照

main HEAD e674801f3640508327e102bda37805b687c3108d。实际P05-02-S01-I01报告已提交，组合b7e59b64116fdd4c4f7b06009473341df38471e9，已核五包test/race、vet/build、三轮SQLite重点全部PASS，root回填S01/I done并释放0007锁。原集成agent未完成下一BotHTTP树准备（指定bot-message-http路径实际不存在），不可冒称已有实现或树。

当前collaboration.list_agents只有root，旧worker均不在。AUTH01原tree instance-server-bind/base5519ebf仍保留五auth主文件及测试草稿，尚无源码提交/实施报告/独审/I。局部auth/storage已有PASS记录，server全测旧Bot/Device fixture缺instance头收到400的FAIL仍保留，不标整体通过。恢复先查status/实际检查，不能重置草稿。

新增用户可读docs/PROGRESS.md按已完成/开发中/未开始/外部条件与后置范围列功能；docs/tasks/README.md增加入口并修正旧“本轮只有规划/没有执行结果”陈述。registry保留用户原dirty并增准确调度元数据，.directory未动，进度文档/本文未提交。用户这次请求是进度文档更新，不是阶段交付或全部任务完成。
