# P03-07-S02-S03-I01 通知联合契约精确集成

日期：2026-10-22；角色：integrator；结果：implemented。父 S02 三卡契约可交调度收口，不代表 G-API/G-OPPO 或运行时验收通过。

## 精确组合

目标 `/home/lijx/Projects/RelayNest`、`main`；集成前 HEAD `e5dd0c94b3f5280dfa890a6f4c5c0a619000e412`。源通知树检查干净。源实现父提交精确为 `82ebd62634ecf9025670a75aa956a45667c55ea7`，源审计父提交为该实现；源基线至集成前 main 仅恢复快照变化，API 未改。

| 源提交 | main 目标提交 | 职责 |
| --- | --- | --- |
| `692e11964ea74cb84cf62470a08e4c3a1780a01f` | `3a2378e1899ff9ec37282bd9dfef3c3e98a3f2a4` | 通知偏好、设置同步与发送快照 |
| `36a05a2523a968f384e82de4eb599a10f31226eb` | `6d9396104a73e7ee68faf33f5efd247e826c26a4` | 独立 PASS 审计及父八项联合复核 |

按序 cherry-pick，无冲突或 API 调整。四个 API 文件、实施与审计报告六路径均与审计源 blob 逐字节一致，不需内容重审。本报告独立提交，最终 main SHA 提交后交接。

## 实际验证

Linux，`/tmp/relaynest-p0305-f01-locked/bin/python`，实测 Python 3.13.15，本轮未安装或升级依赖。

- `.../python -m openapi_spec_validator server/api/openapi.yaml`：PASS，OK。
- `.../python server/api/validate.py`：PASS，34 operations/security、100 schemas、450 valid +449 invalid =899 examples。
- 锁解释器只读脚本：六路径源 blob 一致；旧32操作完整结构、90个未变旧 Schema、其他全部 components、顶层元数据、765例值及顺序、两 requirements 锁字节及已安装锁内版本均保持。唯一旧 Schema SyncChange 仅增加 SyncDeviceNotifications 分支与相应描述；逆向移除该分支并恢复描述后与旧值全等。最终34/100/899。
- 前卡 main 集成节点确认为当前祖先；设备负面 A01、最终 A02、注册 A01 与各自源报告 blob 一致，负面历史保留。
- `git diff --check e5dd0c9 HEAD`：PASS；本报告提交前另查 status、cached diff 和限定暂存路径。
- registry.json 与 `.directory` 不覆盖、不暂存、不提交；前后核对哈希，主树 index 在任务结束为空。

未重复审计的23项破坏探针；其独立复验记录保留在审计报告。本轮真实 HTTP/数据库事务/并发、多worker租约、Android回调、SDK、FCM/OPPO网络、系统展示/撤回、手机/NAS、崩溃窗口均 NOT_RUN。未 push、部署、发布或派额外 agent。

## 父 S02 三链及收口边界

| 子卡 | 精确实现和审计 | 已集成 main 节点 |
| --- | --- | --- |
| 会话 S01 | F01 `a63bd59c8f50dcd83353e279b044f25f4383780c`；A02 `c05a5b93b64b5348bb98270ade4856fc23061fac` PASS | `876935a219c3a1bf828aeaabb2e8a82ca2bb5d4a` |
| 注册 S02 | `6901bc09de53995435102d11f2dfad9bd3f46059`；A01 `14081bcb11a5ce8c7ce7b700b6b1ca6e10bdf1e1` PASS | `82ebd62634ecf9025670a75aa956a45667c55ea7` |
| 通知 S03 | `692e11964ea74cb84cf62470a08e4c3a1780a01f`；A01 `36a05a2523a968f384e82de4eb599a10f31226eb` PASS | `6d9396104a73e7ee68faf33f5efd247e826c26a4`（本报告之前） |

前两卡集成报告及当前独立审计均已核对；当前审计明确逐项覆盖父原八验收：会话/撤销、注册/CAS、条件删除/迟到结果、通知配置、设置同步/版本、D1～D3、联合夹具/发送快照、交接/限制。其判断为三卡精确集成后父 S02 契约可收口，本次组合满足该条件，交调度登记；不由本报告修改台账。

DS-A01-01 仅契约范围关闭，原 CHANGES_REQUIRED 报告不改；Session.id 精确撤销、同设备多 Session、设备共享偏好及 Session/provider 隔离保持。当前审计关于读取截断与上下文预算执行偏差的记录原样保留，不编造预算合规或独立运行时证据。

G-API/G-OPPO 与真实 worker/平台/真机验收继续未通过。OPPO账号资格仍缺，不能用合成状态代替现场。全局 settings/CAS/changes 属下一卡。

## 后续与释放

提交本报告后释放主树写入权，从最终 main 精确 SHA 准备 P03-07-S03-S01 独占 managed worktree 与 `codex/` 分支，实际干净路径、分支、基线另行交接；不实施下一卡。源通知树及全部审计历史保留。
