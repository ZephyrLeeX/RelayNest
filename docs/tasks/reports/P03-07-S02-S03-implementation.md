# P03-07-S02-S03 通知设置与父S02联合收口实施

日期：2026-10-22。结果：implemented（契约及合成状态）；交不同agent固定SHA审计，未代调度登记父卡完成。

基线 `82ebd62634ecf9025670a75aa956a45667c55ea7`，工作树 `/home/lijx/.codex/worktrees/api-notification-settings/RelayNest`，分支 `codex/api-notification-settings`；开始HEAD精确匹配且干净。只改四API文件及本报告，锁、卡、registry、旧报告、运行时均未改。实现SHA提交后交接，不在提交内自指。

## 归属、实现与原子边界

T6.3明确“通知偏好始终属于设备”，T9.3将注册限定当前认证设备/会话。沿既有Session模型，设置键取当前认证 `Session.device_id`，注册仍是不可复用 `Session.id+provider`。同device_id多Session共享设备偏好，不能借用另一Session的注册；撤销仅目标Session，不误删设备偏好，不改变登录或单活规则。配置全状态序列明确包含同设备s1/s2及其他设备s3：s2无本会话opush注册时vendor写入失败，s1写入后s2读同一偏好，s3保持，撤销s1后s2仍读完整偏好。独立错误预期检查跨设备污染/旧版倒退/撤销误删均被拒绝。

新增GET/PUT current/notifications，完整配置替换而非patch；enabled、Bot覆盖、IANA时区跨午夜DND、preview、channel/provider独立表达。默认fcm/fcm不要求有效注册，注册可用不激活vendor；首次继续FCM或数据补偿。虚拟默认版本0、时间null，无变更事件，首次CAS expected=0。每次成功PUT含同值都以服务端版本+1、服务端时间和typed同步事件同事务提交，冲突返回最新版本和固定GET恢复路径。max=9223372036854775807，不回绕，不部分提交配置/时间/事件/注册。用户不能提交平台资格、设备/会话选择器或敏感字段。vendor写入需本会话有效opush注册及服务器已核对应平台前置，缺失422，绝不自动回退。

有意扩展旧 `SyncChange`：新增严格 `SyncDeviceNotifications` 分支及说明，事件配置版本必须大于0；当前GET允许虚拟版本0。按device_id过滤、仅较高settings_version应用；其他设备事件忽略投影而沿原规则推进批次游标，旧恢复响应不倒退。其余旧Schema/操作/安全规则和同步H/epoch/floor/seed/bootstrap/WS语义不变。bootstrap后另恢复当前通知配置再决定提醒，GET不冒充完成sync-result。原S03继续负责全局历史/附件设置、健康及全局设置分支；本卡不预造object分支。

job模型只验证契约。Session.id为会话代次；固定purpose(sync/alert)、目标水位、provider、注册/配置版本、稳定提醒键、Bot已读水位及expiry/alert_eligible。发送前重验身份/注册/配置、租约/同设备串行和pending完整快照；alert另查当前模式、设备/Bot开关、DND、未读、目标有效与永久资格，vendor再查平台。发送后以整个快照和当前会话/注册/配置及活租约CAS；旧结果不改新状态。unknown_delivery等不解绑、不跨FCM补响，accepted不作ACK。明确无效注册仍按前卡条件递增墓碑，max原子拒绝；全current对比包括其他provider/Session/配置。仅同会话/注册/配置代次与相同原job可合并sync水位max，不改提醒键/版本/expiry，不复活已丢资格。

D1固定title「RelayNest 有新消息」、content「打开 RelayNest 同步查看」。D2显式vendor、仅未来新事件；旧job停止新发送、隔离外发/在途/未知结果；客户端以最新确认配置决定FCM自绘，收敛允许漏提醒但数据补偿继续。D3仅vendor接受平台离线积压恢复时DND/已读/删除/退出/切换后的撤回限制；FCM保留T9.2自绘、去重/共享已读、preview、DND和系统权限检查。DND按设备时区[start,end)，跨午夜；命中丢提醒资格，结束不补响。未编造厂商SDK/REST/签名/资格/错误码或代注册。

## 父原8验收逐项联合证据

| 原项 | 三卡实际证据及判断 |
| --- | --- |
| 1 会话/撤销 | S02-S01+F01最终源a63bd59、A02 c05a5b9 PASS，main40de39f/f17b94f，集成876935a。`listDeviceSessions/revokeDeviceSession`、Session/DeviceSessionPage及旧会话状态/权限例保留。DS-A01-01已关闭：Session.id精确定位，不依赖device_id唯一，不影响其他会话/Bot凭据。Cookie撤销CSRF/同源声明保留；真实即时断连NOT_RUN。 |
| 2 注册/恢复/CAS | S02-S02源6901bc0、A01 14081bc PASS，main02c2386/28ae9c9，集成82ebd62。PUT/GET push及7注册Schema全部不变，初始0、有效同版同token保留时间幂等、每provider局部更新、多会话独立、旧版/同版异值冲突的765例基段全保留。 |
| 3 条件删除/旧结果 | S02-S02 DELETE push/{provider}及push_registration_state全保留；删除递增屏障留墓碑，未知/网络结果不删，新Session不承接旧回调。本卡job结果联合验证旧注册/配置/租约/完整快照不符、明确无效注册、未知结果与max耗尽不部分提交。 |
| 4 配置/恢复 | 本卡GET/PUT current/notifications、NotificationSettingsUpdate/State/Conflict；模式合法/非法、缺本会话注册/平台、CAS冲突恢复、同值递增、虚拟0及max、全字段替换均有真实合成正反例。 |
| 5 设置变更/版本时间 | SyncDeviceNotifications和当前GET；跨设备污染、同设备旧版本倒退/重复、同device_id多Session共享与撤销不删、耗尽配置/时间/事件原子性。原sync/bootstrap/WS夹具保留，设置版本及服务端时间不作客户端排序时间。 |
| 6 D1～D3 | 两操作冻结x-notification-job-policy与README完整边界；vendor固定通用文案、首次FCM、切换只未来事件/旧job失效、在途残余、未知结果不跨响、收敛可漏但补偿数据。FCM不扩豁免，DND命中及结束不补响夹具；平台展示NOT_RUN。 |
| 7 联合状态/安全 | 134新增例，配置事务逐步全状态、job before/result/merge全current比较；包含租约/同设备串行、目标已读/删除、none静默同步、无效组合、来源版本/旧结果、DND时区边界、敏感字段禁止。旧765例全保留，含前两卡多通道/幂等/局部更新/越权/墓碑非回归；23隔离破坏均被拒绝。 |
| 8 路径/交接/限制 | 下表及README完整列三卡5路径7操作、全部新增恢复读和条件删除；两锁/旧操作/旧组件/旧例比较PASS。原S03剩余全局分支显式保留；真实HTTP/数据库CAS、多worker、SDK、平台、真机DND、手机/NAS均NOT_RUN。未放行完整G-API/G-OPPO。 |

## 三卡路径、权限、状态及keys

| 路径/operationId | 权限 | 状态与Schema |
| --- | --- | --- |
| GET /devices，listDeviceSessions | AdminBearer或WebCookie | 200 DeviceSessionPage(items:Session)；400/401/403/429/503 |
| DELETE /devices/{id}/session，revokeDeviceSession | AdminBearer或WebCookie+CSRF，Cookie同源 | 204；400/401/403/404/429/503；id为Session.id，无body |
| GET /devices/current/push，getCurrentPush | 当前有效Android AdminBearer | 200 PushRegistrationState；400/401/403/429/503 |
| PUT /devices/current/push，updateCurrentPush | 同上 | PushRegistrationUpdate→200 PushRegistrationState；400/401/403/409/413/422/429/503 |
| DELETE /devices/current/push/{provider}，deleteCurrentPush | 同上 | PushRegistrationDelete→200 PushRegistrationState；400/401/403/409/413/422/429/503 |
| GET /devices/current/notifications，getCurrentNotifications | 同上 | 200 NotificationSettingsState；400/401/403/429/503 |
| PUT /devices/current/notifications，updateCurrentNotifications | 同上 | NotificationSettingsUpdate→200 NotificationSettingsState；400/401/403/404/409 NotificationSettingsConflict/413/422/429/503 |

全部成功和错误no-store。旧注册keys：provider/token/registration_version/expected_version；删除仅expected_version；恢复session_id/registrations[provider,registration_version,registered,updated_at]，0仅从未注册，墓碑不能丢。配置写keys：expected_version/enabled/bot_overrides(bot_id,enabled)/dnd(enabled,timezone,start,end)/preview/notify_channel/notify_provider；恢复settings(device_id,settings_version,updated_at+全部配置)/server_time。同步type=device.notifications.updated/change_seq/settings，外层SyncPage.server_time，GET与同步按设备/版本应用。新增9 Schema：`NotificationSettingsVersion/NotificationDND/NotificationBotOverride/NotificationSettingsUpdate/NotificationSettings/NotificationSettingsState/NotificationSettingsConflict/NotificationJobSnapshot/SyncDeviceNotifications`。

新401 AUTHENTICATION_REQUIRED、403 FORBIDDEN；404 RESOURCE_NOT_FOUND；409 SETTINGS_CONFLICT/SETTINGS_VERSION_EXHAUSTED附error.latest_settings_version/recovery_path，其余错误沿公共Error，不泄token/厂商内部串；422 VALIDATION_FAILED/PROVIDER_NOT_READY。注册409 REGISTRATION_CONFLICT/REGISTRATION_VERSION_EXHAUSTED及全部既有sync/bootstrap错误不变。所有updated_at/server_time为服务端UTC；版本非客户端时间。来源Session.id隔离注册，device_id隔离配置，回调不能换新凭据重放。

## 检查记录与限制

- 解释器 `/tmp/relaynest-p0305-f01-locked/bin/python`，Python 3.13.15。两锁SHA256：requirements.in `4d56f1903c85ca444dde0966a7fd2702f15242b2cbc5f35eb1fcd273a0317d7c`；requirements.txt `4e1992c8e79a37d1671671afd5e24f686837f719078ac7bdab06b5919231f184`。逐条核对已装包版本与原锁相等，未安装/升级依赖。
- `python -m openapi_spec_validator server/api/openapi.yaml`：OK；`python server/api/validate.py`：PASS，34 operations/100 schemas/450 valid+449 invalid=899例。
- `/tmp/notification-checks.py`：32旧操作全部相等、90旧Schema相等+SyncChange唯一有意扩展、其他components与文档顶层相等、765旧例按完整结构/顺序相等、两锁字节相等及已装版本相等；新增2操作/9Schema/134例。原例中非法Unicode负例保留原转义字节，不重编码破坏。
- `git diff --check`、最终diff/status及显式stage范围检查：通过（最终提交前再执行）。
- 最初生成片段出现YAML锚名碰撞及非法Unicode负例重编码失败，改为仅新增无alias片段与保留原examples字节，再执行完整校验。首次缺状态破坏触发KeyError，已增强校验器将缺失响应状态记为明确失败，不把异常当有效探针。新增状态包含独立错误预期，不能把与实现同算生成答案当证据。
- 读取偏差：初始批读及少数后续输出超过1800tokens并截断，未精确计量总输入，不能声称严格预算达标。后续以keys/局部读取；未整灌设计/API/899例。本报告如实记录，不省验收。
- 结构/状态/声明PASS均不证明HTTP鉴权、数据库事务、多worker抢占、SDK、平台展示/撤回、真机DND、手机/NAS；上述全部NOT_RUN。OPPO账号未准备仅保留激活/实测门槛。无待用户决定或本卡外功能变更。

## 隔离破坏矩阵

以下每次均将三份契约/夹具文件复制到独立临时目录，仅破坏一项，执行同一validate.py，全部得到非零契约/状态错误；不以SyntaxError/Traceback计成功。精确复现脚本附后。

| 破坏项 | 退出 | 拒绝证据 |
| --- | --- | --- |
| web-permission | 1 | 示例校验与预期不符：通知 权限 updateCurrentNotifications{'AdminBearer': []} 权限声明、通知 权限 updateCurrentNotifications{'WebCookie': []} 权限声明 |
| no-store | 1 | 响应缺少 no-store：get /devices/current/notifications |
| missing-409 | 1 | 示例校验与预期不符：通知 冲突恢复响应 操作响应状态缺失、通知 耗尽响应 操作响应状态缺失 |
| missing-recovery-version | 1 | 示例校验与预期不符：通知 冲突必须最新版本 |
| mode-provider-combo | 1 | 示例校验与预期不符：通知 非法组合 fcmopush、通知 非法组合 fcmNone、通知 非法组合 vendorfcm、通知 非法组合 vendorNone、通知 非法组合 nonefcm、通知 非法组合 noneopush、通知 非法模式事务 通知配置状态 |
| union-missing | 1 | 示例校验与预期不符：通知 typed同步分支、通知 完整union批次 |
| weak-job-policy | 1 | 通知路径/权限/状态/事务声明不符：getCurrentNotifications |
| weak-settings-owner | 1 | 通知路径/权限/状态/事务声明不符：updateCurrentNotifications |
| sensitive-snapshot | 1 | 示例校验与预期不符：通知 快照禁止 token、通知 快照禁止 body、通知 快照禁止 attachment_url、通知 快照禁止 callback、通知 快照禁止 credentials |
| DND-disabled | 1 | 示例校验与预期不符：通知 DND命中跨午夜上海23点 通知发送状态、通知 DND命中午夜后上海0点 通知发送状态、通知 DND开始包含22点 通知发送状态 |
| cross-device-apply | 1 | 示例校验与预期不符：通知 配置CAS恢复跨设备同设备多Session撤销 通知配置状态 |
| old-version-applied | 1 | 示例校验与预期不符：通知 配置CAS恢复跨设备同设备多Session撤销 通知配置状态 |
| CAS-bypassed | 1 | 示例校验与预期不符：通知 配置CAS恢复跨设备同设备多Session撤销 通知配置状态、通知 同值PUT递增并恢复未知请求结果 通知配置状态 |
| vendor-gate-bypassed | 1 | 示例校验与预期不符：通知 配置CAS恢复跨设备同设备多Session撤销 通知配置状态、通知 无平台前置 通知配置状态 |
| settings-exhaustion-bypassed | 1 | 示例校验与预期不符：通知 耗尽 通知配置状态 |
| lease-owner-bypassed | 1 | 示例校验与预期不符：通知 失租约旧worker结果 通知发送状态 |
| serial-owner-bypassed | 1 | 示例校验与预期不符：通知 同设备串行锁 通知发送状态 |
| old-config-result | 1 | 示例校验与预期不符：通知 旧配置结果不能写新状态 通知发送状态、通知 旧配置水位合并不能改新代次 通知发送状态 |
| old-registration-result | 1 | 示例校验与预期不符：通知 注册轮换 通知发送状态、通知 旧注册结果不能删新注册 通知发送状态 |
| full-snapshot-result-bypassed | 1 | 示例校验与预期不符：通知 迟到结果快照不符 purpose 通知发送状态、通知 迟到结果快照不符 target_watermark 通知发送状态、通知 迟到结果快照不符 settings_version 通知发送状态、通知 迟到结果快照不符 registration_version 通知发送状态、通知 迟到结果快照不符 expires_at 通知发送状态、通 |
| unknown-result-clears | 1 | 示例校验与预期不符：通知 结果 unknown_delivery 通知发送状态 |
| merge-revives | 1 | 示例校验与预期不符：通知 合并水位不复活资格 通知发送状态、通知 合并错误复活 通知发送状态 |
| initial-settings-mutation | 1 | 示例校验与预期不符：通知 配置CAS恢复跨设备同设备多Session撤销 通知配置状态、通知 同值PUT递增并恢复未知请求结果 通知配置状态 |

## 后卡交接（≤800字）

基线82ebd62；会话a63bd59/A02c05a5b9→876935a；注册6901bc0/A0114081bc→82ebd62。本卡GET/PUT /devices/current/notifications。前卡GET /devices/current/push、条件DELETE /devices/current/push/{provider}；GET /devices、DELETE /devices/{id}/session保留。通知仅Android AdminBearer；设备偏好共享，注册按Session.id独立，不改单活。写expected_version+全配置，读settings(device_id/settings_version/updated_at+配置)/server_time，详keys见上。0默认/CAS0；同值递增，配置/时间/同步事件原子，耗尽不回绕。409最新版本/恢复路径；422缺本会话注册或平台不回退；401/403/404，全no-store。变更device.notifications.updated/change_seq/settings按设备/较高版本应用，不改H/epoch/floor/seed。快照含会话/用途/水位/provider/两版本/稳定键/expiry；前后同快照CAS、租约/同设备串行，未知结果不删注册，删除墓碑屏障保留。D1通用文本/D2仅未来、旧job失效/在途残余/不跨FCM补响/D3仅vendor例外，DND丢提醒资格不补响。34操作100Schema899例、23破坏、旧765及两锁PASS。HTTP/worker/SDK/平台/手机/NAS NOT_RUN；原S03补全局历史/附件配置、健康及全局变更。待异agent审计，未放行G-API/G-OPPO。

## 复现非回归与隔离检查

在本卡工作树，以锁Python运行下列脚本；依赖仅原锁中已装包，临时目录自动清理，不改源文件。脚本保留本次实际工作树路径，可按审计checkout调整root。

```python
import copy,hashlib,importlib.metadata,json,re,subprocess,tempfile,yaml
from pathlib import Path
root=Path('/home/lijx/.codex/worktrees/api-notification-settings/RelayNest');api=root/'server/api';base='82ebd62634ecf9025670a75aa956a45667c55ea7';py='/tmp/relaynest-p0305-f01-locked/bin/python'
read=lambda p:subprocess.check_output(['git','show',base+':'+p],cwd=root)
old=yaml.safe_load(read('server/api/openapi.yaml'));new=yaml.safe_load((api/'openapi.yaml').read_text());oe=json.loads(read('server/api/examples.json'));ne=json.loads((api/'examples.json').read_text())
ops=lambda d:{(p,m):o for p,i in d['paths'].items() for m,o in i.items() if m!='parameters'}
assert all(ops(new)[k]==v for k,v in ops(old).items())
assert all(new[k]==v for k,v in old.items() if k not in ('paths','components'))
assert all(new['components']['schemas'][k]==v for k,v in old['components']['schemas'].items() if k!='SyncChange')
sc=copy.deepcopy(new['components']['schemas']['SyncChange']);sc['oneOf'].pop();sc['description']=old['components']['schemas']['SyncChange']['description'];assert sc==old['components']['schemas']['SyncChange']
assert all(new['components'][k]==v for k,v in old['components'].items() if k!='schemas')
assert ne[:len(oe)]==oe
hashes={}
for file in ['requirements.in','requirements.txt']:
 raw=(api/file).read_bytes();assert raw==read('server/api/'+file);hashes[file]=hashlib.sha256(raw).hexdigest()
for name,version in re.findall(r'^([A-Za-z0-9_-]+)==([^\s\\]+)',(api/'requirements.txt').read_text(),re.M):
 assert importlib.metadata.version(name)==version,(name,version)
assert len(ops(old))==32 and len(old['components']['schemas'])==91 and len(oe)==765
print('NONREGRESSION: 32 old operations, 90 unchanged schemas + intentional SyncChange branch/description, all other components, 765 old examples, both locks+installed versions PASS')
print('LOCKS:',hashes)
cases=json.loads((api/'examples.json').read_text()); source=(api/'validate.py').read_text();results=[]
probes=[]
def decl(name,fn):probes.append((name,'declaration',fn))
def model(name,old,new):probes.append((name,'model',(old,new)))
op=lambda d:d['paths']['/devices/current/notifications']
decl('web-permission',lambda d:op(d)['put'].update(security=[{'WebCookie':[]}]))
decl('no-store',lambda d:op(d)['get']['responses']['200']['headers']['Cache-Control']['schema'].update(const='public'))
decl('missing-409',lambda d:op(d)['put']['responses'].pop('409'))
decl('missing-recovery-version',lambda d:d['components']['schemas']['NotificationSettingsConflict']['properties']['error']['required'].remove('latest_settings_version'))
decl('mode-provider-combo',lambda d:d['components']['schemas']['NotificationSettingsUpdate'].pop('oneOf'))
decl('union-missing',lambda d:d['components']['schemas']['SyncChange']['oneOf'].pop())
decl('weak-job-policy',lambda d:op(d)['get']['x-notification-job-policy'].update(after='unconditional'))
decl('weak-settings-owner',lambda d:op(d)['put']['x-notification-settings-policy'].update(owner='Session.id'))
decl('sensitive-snapshot',lambda d:d['components']['schemas']['NotificationJobSnapshot'].update(additionalProperties=True))
model('DND-disabled',"return start <= clock < end if start < end else clock >= start or clock < end","return False")
model('cross-device-apply',"candidate['device_id'] == session['device_id']","True")
model('old-version-applied',"candidate['settings_version'] > local['settings_version']","True")
model('CAS-bypassed',"request['expected_version'] != config['settings_version']","False")
model('vendor-gate-bypassed',"request['notify_channel'] == 'vendor'","False")
model('settings-exhaustion-bypassed',"config['settings_version'] == 9223372036854775807","False")
model('lease-owner-bypassed',"state['worker'] == current['lease_owner']","True")
model('serial-owner-bypassed',"current['device_serial_owner'] == state['worker']","True")
model('old-config-result',"snapshot['settings_version'] == config['settings_version']","True")
model('old-registration-result',"snapshot['registration_version'] == current['registration']['registration_version']","True")
model('full-snapshot-result-bypassed',"snapshot == current['job_snapshot'] and current['job_status'] == 'sending'","True")
model('unknown-result-clears',"state['result'] == 'unregistered' or", "state['result'] in ('unregistered', 'unknown_delivery') or")
model('merge-revives',"after['sync_target_watermark'] = str(max", "after['alert_eligible'] = True\n            after['sync_target_watermark'] = str(max")
model('initial-settings-mutation',"config['settings_version'] + 1", "config['settings_version']")
with tempfile.TemporaryDirectory(prefix='relaynest-notification-probes-') as tmp:
 target=Path(tmp)
 for name,kind,mutation in probes:
  d=copy.deepcopy(new);text=source
  if kind=='declaration':mutation(d)
  else:
   oldtext,newtext=mutation;assert oldtext in text,name;text=text.replace(oldtext,newtext,1)
  (target/'openapi.yaml').write_text(yaml.safe_dump(d,allow_unicode=True,sort_keys=False));(target/'validate.py').write_text(text);(target/'examples.json').write_bytes((api/'examples.json').read_bytes())
  result=subprocess.run([py,str(target/'validate.py')],capture_output=True,text=True)
  assert result.returncode!=0,name
  # 声明/状态失败应由校验器触发，不能把语法错误当成功探针。
  assert 'SyntaxError' not in result.stderr and 'Traceback' not in result.stderr,(name,result.stderr[-300:])
  results.append({'name':name,'returncode':result.returncode,'evidence':(result.stderr or result.stdout).splitlines()[-1][:180]})
print('PROBES:',len(results),'PASS')
Path('/tmp/notification-check-results.json').write_text(json.dumps({'baseline':base,'locks':hashes,'probes':results},ensure_ascii=False,indent=2))
```
