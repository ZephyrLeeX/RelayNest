# P03-07-S01-S02 实施报告

- 角色：developer；结果：implemented（待不同 agent 独立审计）。
- 基线：`33cc7585e873a5debaddc5b93c8657b60d5a73a8`，前卡实现/审计已集成。
- 独占目录：`/home/lijx/.codex/worktrees/api-bootstrap/RelayNest`；分支：`codex/api-bootstrap`。初始工作区干净。
- 实现 SHA：本报告随实现提交，不预写自身 SHA；精确 SHA 在提交后交接。
- 仅修改：`server/api/{openapi.yaml,examples.json,validate.py,README.md}` 和本报告。未修改锁、运行时、迁移、registry、设计或旧报告，未派 agent、push、部署。

## 契约与逐项验收

| 子卡验收 | 结果与证据 |
| --- | --- |
| 首次短读事务固定 H/epoch/Bot/共享已读；创建≤H、不可变位置、逐页当前投影/删除/附件复核，无长读事务 | PASS（契约声明）：`bootstrapClient.x-bootstrap-policy`；`BootstrapMessage.created_change_seq`；`x-bootstrap-envelope`；`bootstrap_creation_delete_attachment_current`、创建越界/旧投影/已删除正文/附件消失反例。排序为Bot UUID文本升序、Bot内bot_seq升序，位置按扫描末键推进。 |
| 严格 Schema、身份/no-store、15分钟身份/H/位置令牌、过期/epoch/floor恢复；跨身份/位置/边界/重复页/删除/附件例 | PASS（结构/声明）：4个新增严格Schema；AdminBearer-only及3种越权例；全部响应no-store统一检查；令牌固定900秒、非滑动到期；expiry_boundary/epoch_change/floor_cross_h/cross_identity/wrong_position/tampered/repeat_same_position等夹具；floor=H正例。 |
| 高revision不倒退、Bot按版本合并、独立generation保留outbox/上传，全页+追赶+再检+原子切换并删除旧缓存 | PASS（契约/合成状态）：generation声明及安全/阻断/错误切换例；新增verify阶段与`BootstrapVerified`的floor≤H≤catchup_position扩展，未完成分页/同步、过期/换epoch/floor越H禁止切换；旧消息/Bot版本不覆盖；unsafe_hint_advances_cursor反例。 |
| 联合sync恢复检查及前卡sync/read非回归，不用提示推进游标，不宣称Android实现 | PASS（声明/非回归）：复用原完成轮次位置H的sync游标，追赶后验证末页receipt及已完成游标；verify身份/epoch/未完成/位置不足例；全部旧446例原值及次序保留，全部旧25操作/组件不变。README明确运行时、Android及WS未实现。 |

新增操作：GET `/client/bootstrap`，operationId=`bootstrapClient`。`phase=page`默认；`phase=verify`用于切换前复核，属于本卡恢复闭环局部协议命名。新增Schema：BootstrapMessage、BootstrapPage、BootstrapVerified、BootstrapResponse；新增74例，总520例。未扩展旧SyncChange类型；WS留S01-S03、设备通知/设置留原S02/S03。

## 冻结 keys、错误与时间位置

- 页：`kind=page,epoch,server_high_watermark,retention_floor,bots,read_states,messages,page_token,next_token,has_more,sync_cursor,issued_at,expires_at,server_time`，全部必填；messages项为`created_change_seq,message`，最多200。
- 复核：`kind=verified,epoch,server_high_watermark,retention_floor,catchup_position,server_time`，全部必填。server_high_watermark始终为原H；retention_floor为响应复核的当前floor；catchup_position来自已完成同步游标。
- 所有序号复用非负SQLite64字符串Sequence，Message.revision/Bot.version/ReadState.version复用Version>=1整数；无任意object占位或未解析引用。
- 令牌绑定管理员/epoch/H/扫描位置(bot_id,bot_seq)/snapshot_id/issued_at/expires_at/purpose/all_pages_complete。next_token用于resume；page_token为本页完成后receipt，只有末页receipt可verify。两者用途不可互换；重复续页重新读取当前投影，不承诺字节相同。
- `400 INVALID_BOOTSTRAP_TOKEN`：格式、完整性、身份、位置、用途及参数组合错误；`400 INVALID_SYNC_CURSOR`：最终复核sync游标非法/跨身份/epoch/未完成/位置不足；先检查bootstrap令牌及其重启条件。`410 BOOTSTRAP_RESET_REQUIRED`：now>=expires_at、epoch变化或floor>H，重新开始并提示仅部分历史保留；floor=H可继续。
- issued_at为本轮服务端签发时间，expires_at=issued_at+900秒且不续期；server_time为响应时间。实体时间语义不变，时间不替代序号/版本排序。
- sync_cursor绑定原H/身份/epoch的完成轮次，所有页结束后追赶(H,current]；verify不冻结后续写入，切换后继续原sync游标追赶。

## 实际检查

解释器：`/tmp/relaynest-p0305-f01-locked/bin/python`，实际 `Python 3.13.15`；环境存在，未重建/升级依赖。

| 命令 | 结果 |
| --- | --- |
| `/tmp/relaynest-p0305-f01-locked/bin/python --version` | PASS：3.13.15 |
| `/tmp/relaynest-p0305-f01-locked/bin/python -m openapi_spec_validator server/api/openapi.yaml` | PASS：OK |
| `/tmp/relaynest-p0305-f01-locked/bin/python server/api/validate.py` | PASS：77 schemas；232 valid + 288 invalid examples；26 operations/security |
| `/tmp/relaynest-p0305-f01-locked/bin/python /tmp/relaynest-bootstrap-checks.py`（本轮临时脚本） | PASS：对精确基线逐值比较全部25操作、所有旧components（73Schema及安全/公共响应/参数）；旧446例内容及次序不变；两锁逐字节相同 |
| 同临时脚本22项隔离副本破坏探针 | PASS：全部exit=1，且无异常堆栈；源码未受污染 |
| `git diff --check`，差异与状态检查 | PASS；仅授权路径 |

破坏探针：Web/Bot权限、成功缓存、TTL、长事务、创建边界、分页位置、当前投影、generation、令牌绑定、查询规则、提示推进游标、创建序号数字类型、页扩展、复核扩展；以及过期边界/位置/创建越H/附件消失/消息旧版本/floor越H/未完成sync的夹具错误预期。每项独立深拷贝YAML和例子到临时目录运行原validator，确认非零退出。

锁SHA256：requirements.in=`4d56f1903c85ca444dde0966a7fd2702f15242b2cbc5f35eb1fcd273a0317d7c`；requirements.txt=`4e1992c8e79a37d1671671afd5e24f686837f719078ac7bdab06b5919231f184`。

开发中曾遇到新增YAML节点插入层级及旧非法Unicode夹具序列化错误；已恢复基线原文本并改为仅插入新节点/追加新例。最终冻结集合逐值比较、OpenAPI及全部例验证通过；未弱化旧非法Unicode检查。

NOT_RUN：真实HTTP鉴权/令牌/响应缓存、SQLite短事务/并发/文件竞态、Android/Room generation/崩溃恢复、WS、手机/OPPO/NAS；合成声明及结构例不代表这些已通过。不放行G-API或P1/P2，用户设备输入留正式验收。无请求用户决策或已知需求阻塞。

## WS 后卡交接（≤800中文字）

已冻结GET /client/bootstrap：默认phase=page，首次固定H/epoch/Bot/read快照，每页短读当前消息，created_change_seq≤H，按bot_id文本/bot_seq升序扫描，位置按扫描末键推进；附件消失保留引用、下载404。bots/read_states每页重复固定快照，按版本合并不压倒高版本。page_token为完成页receipt，next_token为resume，绑定管理员/epoch/H/位置/snapshot/签发到期/purpose/末页标记，900秒不续期。末页next_token=null；从sync_cursor（完成轮次H）追赶(H,current]，再phase=verify携末页receipt+完成sync游标复核epoch/floor。非法bootstrap令牌/用途/参数400 INVALID_BOOTSTRAP_TOKEN；复核游标非法/未完成/位置<H为400 INVALID_SYNC_CURSOR；过期/epoch变/floor>H为410 BOOTSTRAP_RESET_REQUIRED，floor=H可用。独立generation保留outbox/上传，高revision与版本不倒退，全页+追赶+再检后原子切换/删除旧缓存。提示不推进游标。页issued_at/expires_at为固定会话时间，server_time响应时间；H为原快照，catchup_position为完成追赶位置。现26操作/77Schema/520例与两锁冻结。WS仍待S01-S03，设备通知/设置留原S02/S03；真实HTTP/SQLite/客户端/WS/真机均NOT_RUN。
