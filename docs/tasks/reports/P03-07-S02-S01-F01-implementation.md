# P03-07-S02-S01-F01 实施报告

- 日期：2026-10-22；角色：修复 developer；范围：DS-A01-01 / P1。结果：implemented，等待不同 agent 对最终提交 A02 复审，本报告不关闭发现。
- 基线：`6928f4cd79d461a199abfae997ec1ed9e69ee642`；工作树 `/home/lijx/.codex/worktrees/api-device-session/RelayNest`，分支 `codex/api-device-session`，开始时干净。只改四 API 文件及本报告，不动冻结操作、任何 components、锁、运行时、卡片、旧报告或 registry。

## 有意差异

`GET /devices` 与 `DELETE /devices/{id}/session` 的描述、参数说明及 `x-device-session-policy.identity` 统一为 `Session.id-exact-target-and-generation`。路径不变、无新增必填参数；`device_id` 仅元数据，可同时存在多个有效 Session。事务只撤销所给 Session.id 全部 provider/WS，提交前复核归属/有效性；未知、无权或失效404，无副作用，不回退同设备新会话。登录新身份、原 logout 范围、权限、no-store、错误和服务端UTC规则保持。

原16个单会话状态声明替换为逐步合成序列，原39个增量场景均保留并加强，原操作前有效回调与失效当前会话回调拒绝也由序列独立核对；新增11例，总687例。每一步比较状态码、全部失效资源、回调接纳结果及完整会话快照（身份、设备、归属、有效性、资源、结果）；登录只追加会话，回调只可写有效且身份一致的目标。Bot 凭据保持的声明也参与比较。这是契约模型，不是服务实现。合成 S1 为 `11111111-1111-4111-8111-111111111111`，S2 为 `33333333-3333-4333-8333-333333333333`，同 device_id `22222222-2222-4222-8222-222222222222`；替代选择键和未知ID也使用合法 UUID，避免混淆格式400与目标404。

## 验收映射

E编号为 examples.json 的1起始序号；范围覆盖原39例 E638～E676。

| 修复卡验收顺序 | 结果与证据 |
| --- | --- |
| 1 精确目标及合法多会话 | PASS；两操作、README、DEVICE_POLICY 一致；E677～E681 的列表接受同设备两会话，状态序列实际选择并比较其他会话保留，而非仅证明Schema接受两行。 |
| 2 双会话及404无副作用 | PASS；E660～E666 保留权限/未知/他人目标；E660撤S1、E677撤S2、E681分别撤二者；E678以合法device_id替代请求404；每步快照保留同设备另一会话、他设备、其他管理员会话及Bot凭据。 |
| 3 交错、迟到及回调 | PASS；E668～E670/E672～E673原迟到/跨代声明加强；E679/E680覆盖登录S2先于/后于撤销S1、两次迟到DELETE(S1)均404、旧回调/在途结果拒绝复活S1或写入S2，S2自身新结果可写；E682目标已失效无回退。 |
| 4 撤销范围与logout | PASS；E671仅logout当前认证会话；E674漏provider/E675扩大logout负例保留；E684～E687新增错误快照负例（误选S2、漏WS、误撤同设备其他会话、误撤Bot）；M06～M08/M14～M16验证相关断言。 |
| 5 权限/敏感字段/缓存/时间 | PASS；E638/E639列表，E640～E646字段泄漏/UTC反例，E647～E656两操作Bearer/Cookie/Bot/Webhook权限，E657～E659稳定错误，E660～E667状态权限，E676顶层凭据拒绝；E683补Webhook状态拒绝。M09～M13验证Cookie/CSRF/同源/no-store/脱敏。 |
| 6 锁、基线及diff | PASS；见实际检查；旧27操作、83旧Schema及其余旧components等值，F01基线全部components（含DeviceSessionPage）等值；637旧例对象原文及两锁字节一致。 |
| 7 破坏探针 | PASS；16个独立临时副本均由实际校验非零拒绝，无Traceback代替断言，详见下表。 |

## 实际检查

解释器 `/tmp/relaynest-p0305-f01-locked/bin/python` 实测 Python **3.13.15**；锁定18个包安装版本全部匹配 requirements.txt，含 PyYAML 6.0.2、jsonschema 4.25.1、openapi-spec-validator 0.7.2。锁SHA256：`4e1992c8e79a37d1671671afd5e24f686837f719078ac7bdab06b5919231f184`。未重新下载安装。

| 实际命令（仓库根） | 结果 |
| --- | --- |
| `/tmp/relaynest-p0305-f01-locked/bin/python --version` | Python 3.13.15 |
| 同解释器 `-m openapi_spec_validator server/api/openapi.yaml` | `server/api/openapi.yaml: OK` |
| 同解释器 `server/api/validate.py` | `PASS: OpenAPI 3.1; 84 schemas; 331 valid + 356 invalid examples; 29 operations/security` |
| 同解释器 `/tmp/relaynest-f01-checks.py` | PASS；通过git show取旧基线5cdaf15及F01基线6928，逐旧operationId比较全部操作对象、逐旧components分组/key比较、比较F01全部components；JSONDecoder定位637个旧对象原文逐字节比较；两锁原始bytes比较。 |
| 同解释器 `/tmp/relaynest-f01-probes.py` | 16/16 REJECT，均退出1；临时副本不修改工作树。 |
| `git diff --check` | PASS |

等值脚本SHA256：`afcf1f72d0a2dd566c0485cc72bca507c5b5f014f97594402c8f72b698d822e4`。探针脚本、输出分别在 `/tmp/relaynest-f01-probes.py`、`/tmp/relaynest-f01-probes.log`；探针脚本SHA256：`674102520c6828aa20fad0c39ad9e2376e3cfc29ca87c7b537f95ab621fc1249`。临时脚本不入库；复审可按上面比较步骤及下面突变重建，不能仅采信摘要。

| 探针 | 临时突变及拒绝依据 |
| --- | --- |
| M01 / M02 | identity恢复device_id / 选择器改按device_id查找；策略冻结/实际状态序列拒绝。 |
| M03 | 失效S1回退S2；两种重登顺序及目标失效序列拒绝。 |
| M04 / M05 | 回调不查有效性 / 不查来源身份；旧回调/跨代结果序列拒绝。 |
| M06 | 撤销时连带注销S2；完整快照拒绝。 |
| M07 / M08 | 失效资源列表漏oppo / 漏ws；资源列表拒绝。 |
| M09 / M10 | Cookie忽略CSRF / 忽略同源；原缺CSRF/跨源例拒绝。 |
| M11 | 仅GET /devices成功响应改public；no-store检查拒绝。 |
| M12 | 仅临时Session Schema允许未知字段；原列表token/CSRF/摘要/推送等泄漏例拒绝。 |
| M13 | 仅设备DELETE的Cookie安全声明去CSRF；原权限例拒绝。 |
| M14 / M15 | Bot凭据声明失效 / logout改撤路径目标；凭据/当前会话隔离断言拒绝。 |
| M16 | 登录使同device_id旧会话失效；两种登录/撤销顺序快照拒绝，不能用单活消除合法多会话。 |

## 限制与后卡交接

HTTP、数据库事务/并发、多worker、实际WS断连、外发撤回、SDK、真机、NAS均 **NOT_RUN**：本卡仅契约与合成模型，以上快照不证明真实运行效果。没有修改冻结login、logout、Session或权限，无新增重要决定。

后卡按 `Session.id` 精确目标和不可复用身份接入；保留所有provider/WS整会话失效，不能假定device_id唯一Session。注册及偏好归属、恢复读/条件更新、job快照隔离仍须按T9.3另核，本卡未提前冻结其字段或归属。DS-A01-01由不同agent对最终SHA复审A02后再由调度关闭，原实现与本修复尚未集成main。
