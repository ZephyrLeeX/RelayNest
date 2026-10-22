# P03-07-S02-S02 注册生命周期实施报告

- role: developer；outcome: implemented（仅契约，待独立审计与精确集成）。
- 基线：`876935a219c3a1bf828aeaabb2e8a82ca2bb5d4a`。
- 工作树：`/home/lijx/.codex/worktrees/api-push-registration/RelayNest`；分支：`codex/api-push-registration`。
- 恢复前实际遗留：openapi.yaml +281行、validate.py +34/-1；无实施提交/测试结论。保留这些注册路径和Schema骨架，补齐事务模型、夹具、文档与验证。最终实现SHA由提交后的交接提供，不在本提交自指。
- 仅修改四个API文件和本报告；两requirements锁只读；没有修改运行时、registry、其他卡、RelayShelf或厂商端。

## 验收与接口

| 子卡验收 | 结果与证据 |
| --- | --- |
| 1 完整注册/恢复/删除 | PASS（契约）：PUT/GET `/devices/current/push`，DELETE `/devices/current/push/{provider}`；`updateCurrentPush/getCurrentPush/deleteCurrentPush`，全部仅当前Android AdminBearer，完整状态/Schema和no-store。 |
| 2 独立版本/幂等/局部写 | PASS（合成）：两provider并存、FCM与OPPO独立更新及其他会话保持；同版同token先于CAS幂等且不改时间，同版异值/旧版/CAS不匹配409。 |
| 3 初始CAS/墓碑/耗尽 | PASS（合成）：初始0；删除增屏障留墓碑，未注册匹配删除无副作用；旧意图无法越过墓碑，更高版本配合GET恢复后重建；上限拒绝修改且无部分删除/回绕。 |
| 4 条件解绑/未知结果 | PASS（合成）：精确Session.id/provider/registration_version且有效会话，明确unregistered或合法载荷确认后的invalid_registration才解绑并递增；旧结果/网络/未知/非法载荷不变注册、会话或偏好。无后台HTTP接口。 |
| 5 正反/敏感/代次夹具 | PASS（结构+合成）：78条新增夹具，含完整快照及错误快照、Schema/权限/错误响应；token不回显，旧回调本地来源隔离，撤销重登不复用Session.id，同device_id可多Session。 |
| 6 冻结清单及后卡边界 | PASS（文档）：README本卡段及本报告交接枚举；不实现通知模式/偏好CAS、同步设置union或完整发送快照，不宣称S02父完成。 |

新增7 Schema：`PushProvider/PushBarrierVersion/PushRegistrationVersion/PushRegistrationUpdate/PushRegistrationDelete/PushRegistration/PushRegistrationState`。PUT keys `provider/token/registration_version/expected_version`；DELETE仅`expected_version`且provider在路径。GET/成功返回`session_id/registrations`，每条`provider/registration_version/registered/updated_at`，恰含fcm、opush；0仅从未注册，updated_at=null；其他版本为服务端UTC RFC3339时间。其他未知字段均拒绝。客户端来源Session.id检查是本地逻辑，不能冒充HTTP字段或服务端已实现的拦截。

错误：公共400/401 AUTHENTICATION_REQUIRED/403 FORBIDDEN/429/503；写另有413/422、409 REGISTRATION_CONFLICT或REGISTRATION_VERSION_EXHAUSTED。沿用Error（无token、版本额外字段、厂商原错或内部job数据）；409与未知请求结果先GET协调，不能由无token投影推断token内容。删除max有效注册报耗尽，同值重放仍200；max写入无法推进时409拒绝，不创建旧Session身份绕过屏障。

## 实际检查

原`/tmp/relaynest-p0305-f01-locked/bin/python`不存在。使用本机`/home/lijx/.local/share/mise/installs/python/3.13.15/bin/python3.13 -m venv /tmp/relaynest-p0305-f01-locked`重建，随后`/tmp/relaynest-p0305-f01-locked/bin/python -m pip install --require-hashes -r server/api/requirements.txt`成功；未升级任何锁内包。解释器路径与版本分别为`/tmp/relaynest-p0305-f01-locked/bin/python`、`Python 3.13.15`。

| 检查 | 实际结果 |
| --- | --- |
| `python -m openapi_spec_validator server/api/openapi.yaml`（上述解释器） | PASS，openapi.yaml: OK |
| `python server/api/validate.py` | PASS：91 schemas；371 valid + 394 invalid =765 examples；32 operations/security |
| 基线逐项比较（下附脚本） | PASS：原29操作结构全等、84旧Schema及所有100旧组件全等、原687例逐项全等、两锁字节全等；有意仅增加3操作/7Schema/78例 |
| 临时副本19破坏探针 | 全部非零退出，原文件无污染，详下表与可复现脚本 |
| `git diff --check` | PASS |

状态夹具中的state_valid=false表示Schema合法、事务快照不合法；因此CLI合法/非法总数是Schema/HTTP分类，不等于状态正反数。模型仅解释合成序列，不执行鉴权中间件、持久事务、并发或厂商调用。

| 破坏项 | exit | 拒绝证据（最后一行） |
| --- | --- | --- |
| android-permission | 1 | 示例校验与预期不符：注册 getCurrentPush权限 AdminBearer 权限声明、注册 getCurrentPush权限 WebCookie 权限声明 |
| no-store | 1 | KeyError: 'Cache-Control' |
| delete-conflict-status | 1 | KeyError: '409' |
| token-response | 1 | 示例校验与预期不符：注册 投影拒绝 token |
| initial-CAS-policy | 1 | 注册路径/投影/事务声明不符：getCurrentPush |
| delete-barrier-policy | 1 | 注册路径/投影/事务声明不符：getCurrentPush |
| unknown-result-policy | 1 | 注册路径/投影/事务声明不符：getCurrentPush |
| max-version-schema | 1 | 示例校验与预期不符：注册 拒绝字段 registration_version 9223372036854775808 |
| initial-CAS-model | 1 | 示例校验与预期不符：注册 状态 初始CAS和多通道局部保持 注册事务状态声明、注册 状态 删除墓碑与恢复重建 注册事务状态声明 |
| replay-before-CAS | 1 | 示例校验与预期不符：注册 状态 初始CAS和多通道局部保持 注册事务状态声明、注册 状态 耗尽拒绝无部分提交 注册事务状态声明 |
| replay-time-unchanged | 1 | 示例校验与预期不符：注册 状态 初始CAS和多通道局部保持 注册事务状态声明、注册 状态 耗尽拒绝无部分提交 注册事务状态声明 |
| delete-barrier-model | 1 | 示例校验与预期不符：注册 状态 删除墓碑与恢复重建 注册事务状态声明 |
| old-result-exact-version | 1 | 示例校验与预期不符：注册 状态 旧结果未知结果不解绑 注册事务状态声明 |
| unknown-result-preserved | 1 | 示例校验与预期不符：注册 状态 旧结果未知结果不解绑 注册事务状态声明 |
| valid-payload-required | 1 | 示例校验与预期不符：注册 状态 旧结果未知结果不解绑 注册事务状态声明 |
| late-callback-source | 1 | 示例校验与预期不符：注册 状态 撤销重登迟到来源隔离 注册事务状态声明 |
| exhaustion-atomic | 1 | 示例校验与预期不符：注册 状态 耗尽拒绝无部分提交 注册事务状态声明 |
| revocation-auth | 1 | 示例校验与预期不符：注册 状态 撤销重登迟到来源隔离 注册事务状态声明 |
| lost-barrier-fixture | 1 | 示例校验与预期不符：注册 状态 删除墓碑与恢复重建 注册事务状态声明 |

## 未执行及限制

真实Go HTTP/数据库CAS与原子提交、多worker、外发与撤回、WS断连、Android本地回调实现、SDK、OPPO账号/资格/签名/分类/厂商错误码、平台展示、手机/NAS现场均NOT_RUN。OPPO缺账号仅保留厂商实测/激活门槛，未代注册，未放行G-OPPO或其他正式验收。注册与偏好归属沿T9.3当前会话，不引入单活/登录替换。没有重要需求变更或待用户决定；后续运行时必须实现本契约，结构PASS不证明运行时。

## 后卡交接（≤800字）

基线876935a；本卡新增GET/PUT /devices/current/push和DELETE /devices/current/push/{provider}，仅当前有效Android AdminBearer，禁止设备/会话选择器；提交前重验不可复用Session.id，同device_id多会话独立。PUT provider/token/registration_version/expected_version，DELETE expected_version；fcm/opush局部独立。首次屏障0，新版本严格增且CAS匹配；有效同版同token重放先于CAS且保留时间。删除有效注册增屏障留墓碑，匹配未注册无副作用，旧删除/迟到写冲突；耗尽拒绝推进、不回绕/部分提交。恢复仅session_id及两通道provider/version/registered/服务端UTC updated_at，绝不回token。409冲突/耗尽后GET协调；公共Error/no-store沿用。内部解绑仅精确有效Session.id+provider+registration_version且明确无效注册，未知/网络/旧结果不动新注册/其他通道/偏好；旧来源回调本地丢弃，不能换新凭据重放。32ops/91schemas/765例及19破坏探针PASS，旧29/84/687/全部组件/两锁不变。真实worker/SDK/平台/手机/NAS NOT_RUN。后卡补偏好CAS/模式/设置union/完整发送快照；本卡未完成父S02。实现SHA见提交后交接，尚待异agent审计/集成。

## 非回归与破坏检查可复现脚本

在仓库根目录将下列脚本保存到临时文件后，用上述锁解释器执行。脚本只在临时目录变更副本，校验器仍为被审版本；退出非零才算成功拒绝，临时目录自动清理。

```python
import copy,json,subprocess,tempfile,shutil
from pathlib import Path
import yaml
base='876935a219c3a1bf828aeaabb2e8a82ca2bb5d4a'
root=Path.cwd(); api=root/'server/api'; py='/tmp/relaynest-p0305-f01-locked/bin/python'
def baseline(name):return subprocess.check_output(['git','show',base+':server/api/'+name])
old=yaml.safe_load(baseline('openapi.yaml')); new=yaml.safe_load((api/'openapi.yaml').read_text())
ops=lambda d:{(p,m):v for p,x in d['paths'].items() for m,v in x.items() if m != 'parameters'}
a,b=ops(old),ops(new)
assert all(b[k]==v for k,v in a.items())
component_count=0
for group,values in old['components'].items():
    for k,v in values.items(): assert new['components'][group][k]==v; component_count+=1
old_cases=json.loads(baseline('examples.json')); cases=json.loads((api/'examples.json').read_text())
assert cases[:len(old_cases)]==old_cases
for name in ['requirements.in','requirements.txt']: assert baseline(name)==(api/name).read_bytes()
print(f'NONREG PASS: {len(a)} old operations; {len(old["components"]["schemas"])} old schemas; {component_count} all old components; {len(old_cases)} old examples; 2 byte-identical locks')
probes=[]
def probe(name,mutator): probes.append((name,mutator))
def spec(field,value):
    def change(target):
        doc=copy.deepcopy(new); node=doc
        for k in field[:-1]:node=node[k]
        if value=='REMOVE': del node[field[-1]]
        else: node[field[-1]]=value
        (target/'openapi.yaml').write_text(yaml.safe_dump(doc,allow_unicode=True,sort_keys=False))
    return change
prefix=['paths','/devices/current/push','get']
probe('android-permission',spec(prefix+['security'],[{'WebCookie':[]}]))
probe('no-store',spec(prefix+['responses','200','headers','Cache-Control'],'REMOVE'))
probe('delete-conflict-status',spec(['paths','/devices/current/push/{provider}','delete','responses','409'],'REMOVE'))
probe('token-response',spec(['components','schemas','PushRegistration','properties','token'],{'type':'string'}))
probe('initial-CAS-policy',spec(prefix+['x-push-registration-policy','initial'],'reset-to-1'))
probe('delete-barrier-policy',spec(prefix+['x-push-registration-policy','delete'],'delete-row'))
probe('unknown-result-policy',spec(prefix+['x-push-registration-policy','unknown_result'],'clear-registration'))
probe('max-version-schema',spec(['components','schemas','PushRegistrationVersion','maximum'],9223372036854775808))
def model_replace(old_text,new_text):
    def change(target):
        p=target/'validate.py'; text=p.read_text(); assert old_text in text; p.write_text(text.replace(old_text,new_text,1))
    return change
probe('initial-CAS-model',model_replace("elif expected != version or", "elif False or"))
probe('replay-before-CAS',model_replace('if replay:\n                status = 200', 'if False:\n                status = 200'))
probe('replay-time-unchanged',model_replace('if replay:\n                status = 200', "if replay:\n                record['updated_at'] = step['server_time']\n                status = 200"))
probe('delete-barrier-model',model_replace("version=version + 1 if action == 'delete'", "version=0 if action == 'delete'"))
probe('old-result-exact-version',model_replace("and registration['version'] == step['snapshot_version']", 'and True'))
probe('unknown-result-preserved',model_replace("step['result'] == 'unregistered'", "step['result'] in ('unregistered', 'unknown', 'network_error')"))
probe('valid-payload-required',model_replace("and step['payload_valid']", 'and True'))
probe('late-callback-source',model_replace("step['source_session_id'] != current", 'False'))
probe('exhaustion-atomic',model_replace('elif version == ceiling:', 'elif False:'))
probe('revocation-auth',model_replace("elif not session or not session['active']:", "elif not session:"))
def bad_fixture(target):
    mutated=copy.deepcopy(cases); case=next(c for c in mutated if c['name']=='注册 状态 删除墓碑与恢复重建')
    case['push_registration_state']['steps'][2]['expected']['sessions']['s1']['providers']['fcm']['version']=0
    (target/'examples.json').write_text(json.dumps(mutated,ensure_ascii=True))
probe('lost-barrier-fixture',bad_fixture)
results=[]
with tempfile.TemporaryDirectory(prefix='relaynest-push-probes-') as tmp:
    target=Path(tmp)
    for name,mutator in probes:
        for f in ['openapi.yaml','validate.py','examples.json']:shutil.copyfile(api/f,target/f)
        mutator(target)
        run=subprocess.run([py,str(target/'validate.py')],capture_output=True,text=True)
        assert run.returncode!=0, name+' unexpectedly passed'
        results.append({'name':name,'exit':run.returncode,'error':(run.stderr or run.stdout).splitlines()[-1][:180]})
        print('PROBE REJECTED:',name)
Path('/tmp/push-check-results.json').write_text(json.dumps(results,ensure_ascii=False,indent=2))
print('PROBES PASS:',len(results),'mutations rejected')
```
