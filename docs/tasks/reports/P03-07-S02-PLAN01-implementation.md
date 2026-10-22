# P03-07-S02-PLAN01 文档预规划交付

## 范围与结果

- 角色：规划；基线 `33cc7585e873a5debaddc5b93c8657b60d5a73a8`，工作树 `/home/lijx/.codex/worktrees/api-device-plan/RelayNest`，分支 `codex/api-device-plan`；初始工作区干净。
- 仅改父卡 [P03-07-S02](../cards/P03-07-S02.md)，新增 [S02-S01](../cards/P03-07-S02-S01.md)、[S02-S02](../cards/P03-07-S02-S02.md)、[S02-S03](../cards/P03-07-S02-S03.md) 及本报告。没有实现API，也没有修改技术设计、其他卡、旧报告或registry。
- 按事务边界拆为会话、provider注册生命周期、通知偏好与联合收口三张串行子卡。注册写入/恢复读/条件删除/CAS/删除后持久版本屏障同卡；通知CAS/设置变更/恢复/发送快照同卡。原父8验收逐条原文保留，并给出映射。
- 每卡包含目标、冻结输入、keys/行段、路径白名单、验收、检查、串行依赖与≤800中文字符交接。目标输入≤8k、硬≤12k，共同API禁止并发写；材料预算为估算，未声称精确token统计。
- D1～D3已批准内容原样继承：固定通用文案，显式可选vendor、未来新事件切换、外发/在途残余、超时不跨通道补响、配置收敛允许漏提醒与数据补偿；仅vendor接受离线展示/撤回限制，FCM原验收不减。账号未准备不阻通道无关契约，不编造外部接口或代注册。

## 冻结与实施门槛

本规划未冻结新增接口字段。25操作/73 Schema/446例仅本规划基线的sync/read历史核验点；另一工作树的bootstrap增量及后续WS尚需S01最终审计集成。真正实施必须等待S01父任务完整收口，从最终main实际SHA派发，重验操作/组件/例子基数与最终≤800字keys/错误/时间/剩余分支摘要。不把本规划SHA或中间bootstrap计数当未来基线，不降低Sequence/Version/权限/no-store规则。

## 验证记录

本次只改任务文档，无需业务测试；未运行OpenAPI/业务/SDK/真机检查，不能据此放行G-API/G-OPPO。已执行：

| 检查 | 实际命令/方法 | 结果 |
| --- | --- | --- |
| 文档相对链接 | Perl逐个解析父卡和三子卡Markdown相对链接并检查目标存在 | PASS |
| 子卡必需字段 | Perl检查目标/依赖/冻结输入/最小上下文/路径/验收/检查/交接/预算/最终main/无父done依赖 | PASS，三卡齐全 |
| 原父验收保留 | `git show 33cc7585e873a5debaddc5b93c8657b60d5a73a8:docs/tasks/cards/P03-07-S02.md`，提取8条checkbox与当前父卡逐字比较 | PASS，8条原文相同 |
| API六文件不变 | `git diff --exit-code 33cc7585e873a5debaddc5b93c8657b60d5a73a8 -- server/api/openapi.yaml server/api/examples.json server/api/validate.py server/api/README.md server/api/requirements.in server/api/requirements.txt` | PASS，无差异 |
| diff及范围 | `git diff --check`，显式stage五文件后检查cached diff/状态 | PASS；仅本任务五文档 |

环境没有 `python`/`python3` 可执行项，两次尝试执行文档编辑脚本均在解释器启动前退出127、没有写入；随后使用系统Perl和shell完成编辑与文档检查。未安装/升级依赖。文档验证脚本位于工作树外临时路径，不提交；本次未改API，未来实施仍须按各卡锁环境执行正式校验。

## registry建议（仅建议，未写入）

| ID | 类型/归属建议 | 派发依赖与限制 |
| --- | --- | --- |
| P03-07-S02-PLAN01 | planning，subject P03-07-S02 | 固定本实现提交给不同agent审计，再精确集成/登记；不代表S02实现完成 |
| P03-07-S02-S01 | implementation，parent P03-07-S02 | S01父任务完整审计集成+本PLAN01审计集成/登记；未满足前保持未派发 |
| P03-07-S02-S02 | implementation，parent P03-07-S02 | 直接前卡S02-S01审计PASS且精确集成；继承共同前置 |
| P03-07-S02-S03 | implementation，parent P03-07-S02 | 直接前卡S02-S02审计PASS且精确集成；继承共同前置；汇总父8验收 |

父S02保留原依赖S01完整收口；不再整卡派开发。子卡不依赖父S02 done，集成依赖审计PASS而非原卡done，避免环。原P03-07-S03仍等待S02父收口；调度汇总各子卡实现/审计/集成SHA和父全部验收后才登记S02完成。registry实际字段/状态沿现有格式由调度填写，本报告不代改状态台账。

## 后卡精简交接（≤800中文字）

S02仅文档预规划，拆成S02-S01会话列表/撤销、S02-S02 provider注册/恢复读/条件删除/CAS/持久版本屏障、S02-S03通知CAS/恢复/设备设置变更/模式与发送快照联合收口；三卡串行开发→不同agent固定SHA审计→PASS精确集成。父8验收逐字保留，每卡目标≤8k输入、硬≤12k并给局部keys/路径/检查/≤800字交接。不抢S01 bootstrap/WS字段，25/73/446只作当前历史点；首卡实施必须等S01父完整审计集成，以最终main与最终摘要重验全部基数/权限/错误/版本/时间。D1～D3保持，FCM豁免不扩大；OPPO未准备只阻外部实测/激活，不代注册。API六文件不变，文档链接/字段/原验收/diff检查PASS；业务/真机NOT_RUN。待本规划不同agent审计和精确集成/registry登记，未实施S02，不放行G-API/G-OPPO。提交后真实SHA由交接返回，不在本报告预写自身SHA。
