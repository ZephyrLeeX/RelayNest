# P03-06 投递与失败管理 OpenAPI 实施报告

```yaml
task_id: P03-06
role: developer
outcome: implemented
base_sha: 38d52c3bae74010167d4defe0e681e843d497fc8
worktree: /home/lijx/.codex/worktrees/api-delivery/RelayNest
branch: codex/api-delivery-run3
changed_paths:
  - server/api/openapi.yaml
  - server/api/examples.json
  - server/api/validate.py
  - server/api/README.md
  - docs/tasks/reports/P03-06-implementation.md
requested_decisions: []
```

开始时 HEAD 精确匹配派发基线且工作树干净；本卡独占写入共同 OpenAPI。实现 SHA 在提交后交接，不在本报告预写自身 SHA。仅 implemented，待不同 agent 独立审计和精确集成，不改 registry/任务状态。未改 go.mod、存储/迁移、运行时业务、RelayShelf、其他卡或 P03-07 同步/设备/推送。

## 依据与接口冻结

已按局部阅读 AGENTS、CONTRIBUTING、WORKER_PROTOCOL、本卡、P03-05 实施/A02 和 F01 修复摘要、PRD §1/§3.1～3.6、ARCHITECTURE §1/§4/§5、TECHNICAL_DESIGN §2/§3/§4/§5.3～5.4/§6边界/§7.2/§8，以及 DEVELOPMENT_PLAN P3。无新依赖和外部接口推测。

在原18操作/44 Schema上新增5操作/17 Schema，共23操作/61 Schema；新增131示例，合计369（136正例、233反例）。`schema_valid` 区分结构合法与权限/状态声明不合法的夹具；它不改变原238示例的预期。

| 接口 | 冻结行为 |
| --- | --- |
| POST /bot/events/claim | Bot receive，consumer_id、wait默认25上限30、limit默认20上限100；200投递批次/超时空数组；原子有界领取、等待在事务外、唤醒及提交重新检查启用/能力/凭据代次 |
| POST /bot/deliveries/{id}/ack | consumer_id+lease_token，当前Bot/凭据代次/有效租约；200 acked原回执；同成功租约重试恢复原acked_at，不把租约自然过期误报冲突；仍先鉴权和校验Bot状态 |
| POST /bot/deliveries/{id}/renew | 同consumer/有效租约；extend_seconds 1～120，在当前lease_until上延长，领取起含初始120秒累计不超过600秒；200新期限；不承诺响应丢失的幂等恢复 |
| GET /admin/deliveries | Android管理员或Web Cookie，bot_id/state筛选及既有cursor/limit；state=failed为失败队列；独立严格DeliveryMetadata投影 |
| POST /admin/deliveries/{id}/retry | Android管理员或Web Cookie+CSRF+同源校验；必需幂等键和空JSON；仅failed→pending，代次加一、attempt归零，保留ID和载荷；同键恢复优先于新操作状态判断 |

`retry_generation` 初始字符串0，复用非负SQLite64位Sequence，覆盖超过JS安全整数及SQLite上限/溢出示例；`state_version`复用Version。ACK/retry回执严格区分状态。ACK是应用持久接收，先保存完整事件与必要附件再ACK，业务执行/失败回复和已读均另行处理。单次业务失败没有Hub NACK接口。

默认重试8次、超时才消耗实际领取尝试、退避及扫描参数按T4；停用回pending并暂停计时，API凭据重置撤销旧租约并重排，Webhook重置不影响API租约。无消费者/纯等待不耗次数；撤销有效租约不额外耗次数，下一领取可复用未消耗尝试编号。`attempt`为当前代尝试编号，未领取为0。手动重试不执行或重跑业务；停用Bot重排后仍暂停，恢复后才领取。版本/代次耗尽不得回绕，503且不提交状态。

新局部字段选择：consumer_id最长128字符、领取批次最多100是有界资源实现命名/限制；不新增产品功能。renew显式使用extend_seconds；租约不可预测，服务只存摘要，并绑定Bot、consumer、凭据代次和本次领取。ACK同成功租约重放使用已保存摘要，与泛化幂等键区分。

409稳定码为BOT_DISABLED、LEASE_CONFLICT、LEASE_LIMIT_EXCEEDED、DELIVERY_NOT_FAILED、IDEMPOTENCY_CONFLICT；跨Bot/无权资源404 RESOURCE_NOT_FOUND，墓碑410 RESOURCE_EXPIRED。403声明receive能力不足或管理权限不足，401拒绝无效/撤销身份，503不伪装空队列。`x-error-codes`将对应状态码列表纳入示例检查。所有成功/错误响应继续no-store。

## 安全与验收

| 卡内条目 | 结果及证据 |
| --- | --- |
| ACK与业务结果分开 | PASS（契约）：AckReceipt只含ID、acked状态和时间；注入business_success被拒；README和ACK说明先持久接收再本地业务 |
| 无正文管理投影 | PASS（契约）：独立严格允许列表；正文/callback_data/附件信息/URL/Token/consumer/原始异常等反例拒绝；列表和重试不引用Message/ClaimResponse |
| 同租约重试明确 | PASS（契约）：同成功ACK保留原acked_at；旧/过期/其他consumer非成功租约409；声明及破坏变异检查；renew重复可能再延长，仍受累计上限 |
| 接收与管理员权限 | PASS（声明）：25个组合夹具覆盖Bot/管理员/Webhook/Web Cookie及Cookie+CSRF，receive标志、管理投影和幂等声明检查；不是实际身份请求 |
| 受影响校验与非回归 | PASS：以下命令和探针；原全部组件/18操作/44 Schema/238例与锁字节保留，三处严格ContentRange模式保留 |
| 独立提交及后续审计 | 本报告随实现提交；精确SHA交接，不标父卡done |

管理error_code只收服务映射稳定码，不能拼接外部错误串。Schema拒绝结构泄漏，不可能仅凭正则保证一个上游字符串真正经过脱敏；必须在后续服务实现测试映射。lease_until/acked_at/next_attempt_at仅分别在leased/acked/retry_wait非空，状态相关时间也有Schema正反例。

## callback 关联边界及后续交接

遵循T2.3最小不可变callback字段：message_id/message_revision/button_id/callback_data、来源管理员/设备与时间；event_id位于领取条目。`message.created`事件含sender=administrator的Message不可变快照。没有为callback增加父消息正文/历史读取权限；Hub不解释参数。按调度确认，不凭未来接入假设增加载荷字段。

T4.2/T5.4规定未ACK/failed投递、必要父消息及附件受保护；管理重试不能解除保护，receive-only只能下载所属领取事件所需附件。后续P05-06/P05-11及本卡独立审计需核对“callback不可变快照→父消息/必要附件引用保护→领取事件附件授权”的实现闭环，尤其不可把一个合法UUID当下载授权。当前OpenAPI没有实现或实测这些关联，也未提供新的Bot历史枚举接口。

## 实际检查与环境

Linux，复用既有 `/tmp/relaynest-p0305-f01-locked`，Python 3.13.15；未重建/升级锁依赖。系统无通用python3命令，所有最终命令均使用该明确解释器。生成阶段曾被结构校验拒绝重复YAML锚点，已改为只对新增节点无别名输出；旧节点原文不重序列化，最终结构及语义非回归均通过。

在工作树根执行以下命令，最终均退出0：

```sh
/tmp/relaynest-p0305-f01-locked/bin/python --version
/tmp/relaynest-p0305-f01-locked/bin/python -m openapi_spec_validator server/api/openapi.yaml
/tmp/relaynest-p0305-f01-locked/bin/python server/api/validate.py
/tmp/relaynest-p0305-f01-locked/bin/python /tmp/relaynest-p0306-probe.py
git diff --check
```

| 检查 | 实际结果 |
| --- | --- |
| OpenAPI结构及Schema | `server/api/openapi.yaml: OK` |
| 主流程 | `PASS: OpenAPI 3.1; 61 schemas; 136 valid + 233 invalid examples; 23 operations/security` |
| 前卡非回归 | 原全部路径、Schema、公共响应/securitySchemes/parameters解析后完全相等；原238例逐项保留，两锁文件字节不变 |
| 声明破坏变异 | 14/14非零拒绝，且无异常traceback；覆盖Webhook领取、管理员ACK、send续约、retry缺CSRF、正文投影、未知字段、幂等键可选、缺重放、累计续约无上限、缺ACK重放、领取无上界、代次数字、缺状态错误码及管理响应引用正文载荷 |
| Python编译 | PASS，pyc输出到`/tmp/relaynest-p0306-validate.pyc`，不入库 |
| diff和范围 | PASS，只有四份API文件及本报告；未改锁依赖/台账/业务 |

锁文件SHA-256：requirements.in=`4d56f1903c85ca444dde0966a7fd2702f15242b2cbc5f35eb1fcd273a0317d7c`；requirements.txt=`4e1992c8e79a37d1671671afd5e24f686837f719078ac7bdab06b5919231f184`。

## 未执行与限制

没有运行时路由/存储实现，真实Long Polling、并发claim CAS、ACK/renew与停用/重置竞态、ACK丢回复恢复、失败计时、手动重试事务/并发、附件引用保护和管理实际响应脱敏均NOT_RUN；本卡验收仅限契约，不声称投递状态机实测通过。未执行真机/外部服务请求、推送、发布或部署。无待用户决定项；以上运行时验证交后续实现任务，不用契约声明替代。

## 仓库外探针复现

可将以下代码保存为`/tmp/relaynest-p0306-probe.py`后执行上述命令。它只读精确基线，在临时目录破坏契约并运行验证，不改工作区源码；临时目录自动回收。

```python
import copy,hashlib,json,py_compile,subprocess,tempfile
from pathlib import Path
import yaml
BASE='38d52c3bae74010167d4defe0e681e843d497fc8'
root=Path.cwd(); api=root/'server/api'
def old(path):return subprocess.check_output(['git','show',f'{BASE}:{path}'])
before=yaml.safe_load(old('server/api/openapi.yaml'));after=yaml.safe_load((api/'openapi.yaml').read_text())
for key,item in before['paths'].items():assert after['paths'][key]==item,key
for group,items in before['components'].items():
 for key,item in items.items():assert after['components'][group][key]==item,(group,key)
assert before['servers']==after['servers']
assert json.loads((api/'examples.json').read_text())[:238]==json.loads(old('server/api/examples.json'))
for path in ('server/api/requirements.in','server/api/requirements.txt'):
 assert (root/path).read_bytes()==old(path)
 print(path,hashlib.sha256((root/path).read_bytes()).hexdigest())
print('PASS baseline: 18 operations, 44 schemas, all old components, 238 examples and both lock files unchanged')
# 每次只破坏一个声明；在临时目录运行，源契约不被改写。
mutations={
 'claim_webhook':lambda d:d['paths']['/bot/events/claim']['post'].update(security=[{'WebhookKey':[]}]),
 'ack_admin':lambda d:d['paths']['/bot/deliveries/{id}/ack']['post'].update(security=[{'AdminBearer':[]}]),
 'renew_send_only':lambda d:d['paths']['/bot/deliveries/{id}/renew']['post'].update({'x-bot-capability':'send'}),
 'retry_cookie_without_csrf':lambda d:d['paths']['/admin/deliveries/{id}/retry']['post'].update(security=[{'AdminBearer':[]},{'WebCookie':[]}]),
 'metadata_body':lambda d:d['components']['schemas']['DeliveryMetadata']['properties'].update(text={'type':'string'}),
 'metadata_unknown_fields':lambda d:d['components']['schemas']['DeliveryMetadata'].update(additionalProperties=True),
 'retry_optional_key':lambda d:d['components']['parameters']['IdempotencyKey'].update(required=False),
 'retry_missing_replay':lambda d:d['paths']['/admin/deliveries/{id}/retry']['post'].pop('x-idempotency-policy'),
 'renew_total_unbounded':lambda d:d['paths']['/bot/deliveries/{id}/renew']['post'].pop('x-max-total-lease-seconds'),
 'ack_replay_absent':lambda d:d['paths']['/bot/deliveries/{id}/ack']['post'].pop('x-ack-replay'),
 'claim_unbounded':lambda d:d['components']['schemas']['ClaimRequest']['properties']['limit'].pop('maximum'),
 'generation_number':lambda d:d['components']['schemas']['ClaimedDelivery']['properties'].update(retry_generation={'type':'integer','minimum':0}),
 'status_code_missing':lambda d:d['paths']['/bot/deliveries/{id}/renew']['post']['x-error-codes']['409'].remove('LEASE_LIMIT_EXCEEDED'),
 'admin_payload_response':lambda d:d['paths']['/admin/deliveries']['get']['responses']['200']['content']['application/json'].update(schema={'$ref':'#/components/schemas/ClaimResponse'}),
}
with tempfile.TemporaryDirectory(prefix='relaynest-p0306-') as directory:
 tmp=Path(directory)
 for filename in ('validate.py','examples.json'): (tmp/filename).write_bytes((api/filename).read_bytes())
 for name,mutate in mutations.items():
  document=copy.deepcopy(after);mutate(document)
  (tmp/'openapi.yaml').write_text(yaml.safe_dump(document,allow_unicode=True,sort_keys=False))
  result=subprocess.run(['/tmp/relaynest-p0305-f01-locked/bin/python',str(tmp/'validate.py')],capture_output=True,text=True)
  assert result.returncode!=0,(name,result.stdout,result.stderr)
  assert 'Traceback' not in result.stderr,(name,result.stderr)
  print('PASS rejected mutation:',name)
py_compile.compile(str(api/'validate.py'),cfile='/tmp/relaynest-p0306-validate.pyc',doraise=True)
print('PASS: 14 declaration mutations rejected; Python compile outside repository')

```
