# P03-07-S01-S03 WS提示与同步域联合收口实施

日期：2026-10-22；角色：developer；结果：implemented（契约范围，待不同agent固定SHA审计/精确集成）。

基线：`3cdb5e228b7d124890d45250728b260b2091a6db`；工作树 `/home/lijx/.codex/worktrees/api-ws-contract/RelayNest`，分支 `codex/api-ws-contract`，开始状态干净且HEAD精确匹配。实现SHA提交后交接，不在提交内预写自身SHA。

## 变更与冻结

仅修改 server/api/openapi.yaml、examples.json、validate.py、README.md 和本报告；两锁只读。新增 GET `/client/ws` / `connectClientWS`，当前Android管理员Bearer/Origin/升级前错误/HTTP101无JSON正文，新增六个严格frame Schema。hello携epoch、最新change_seq及协议主次版本，changes_available只触发REST；session_revoked停止/关闭/清理会话。未知可选事件用无载荷严格envelope，已知事件不能借optional绕过Schema。主版本1兼容，其他主版本结构可合法但处理须明确失败。控制ping/pong与JSON frame分开。

x-ws-policy固定25秒ping/75秒无响应、合并慢端提示且无正文、抖动指数退避≤60秒、后台关闭/前台立即同步、两种无缺口连接窗口与提示不推进游标。合成ws_state分别检查握手、事件、连接窗口、传输和恢复。联合恢复直接复用未改变的sync/bootstrap辅助函数，覆盖epoch/floor→410、真正sync-result与bootstrap-seed隔离、generation切换保留outbox/上传和提示不能提交游标。

当前27操作/83 Schema/637例（295 valid + 342 invalid），相对基线仅增加1操作、6 Schema、109例。原26操作及全部旧paths、所有components分组的旧项（含77 Schema）、顶层元数据逐值不变；528个旧例原值和顺序不变；两个requirements锁逐字节不变。除main加入WS校验外，旧顶层辅助函数AST全部不变。H/epoch/floor/游标来源与错误优先级均未修改；前卡F01与A02关闭BS-A01-01的语义完整保留。

## 子卡验收

| 子卡项 | 判定与证据 |
| --- | --- |
| 1 握手/独立frame/版本事件 | PASS（契约）：/client/ws的security、parameters、x-error-codes、101无content；WSProtocolVersion/WSHello/WSChangesAvailable/WSSessionRevoked/WSUnknownOptional/WSFrame；ws_http101_*、ws_http_error_*、ws_permission_*、ws_state_*握手与事件例；HTTP响应与frame独立校验 |
| 2 心跳/退避/慢端/前后台/严格frame | PASS（声明/合成）：x-ws-policy精确比较，ws_state_transport及八类反例；各已知事件additionalProperties=false、WSFrame oneOf；不含任意object载荷 |
| 3 正反例与no-store | PASS（契约）：ws_bad_seq_*、ws_bad_version_*、ws_bad_minor_*、ws_leak_*、ws_unknown_rejected_*、ws_future_major_structurally_valid、ws_bad_frame_*；Web/Bot/Webhook拒绝握手；101与全部握手错误no-store正反例；42项隔离破坏探针 |
| 4 连接窗口与联合恢复 | PASS（合成）：ws_state_connect-first_*、ws_state_sync-first_*、ws_state_recovery_epoch/floor/sync-result/bootstrap-seed；ws_forbidden_*四例；前两卡全部恢复例重跑；seed来源受服务端完整性保护，不是客户端source字段 |
| 5 父映射/keys/剩余分支 | PASS（契约）：README新增父八项映射、全同步keys/错误/时间索引；本报告下表；原S02/S03所属设置类型尚待后卡显式扩展，非完整G-API |

## 父S01八项联合判定

| 父原验收 | 判定与覆盖位置 |
| --- | --- |
| 1 四操作/权限/握手 | PASS：前卡sync/read/bootstrap + 本卡connectClientWS；全操作security断言、权限例、101与握手错误独立验证 |
| 2 固定H/epoch/floor/分页 | PASS：冻结SyncPage/SyncEpoch、x-sync-envelope/x-sync-policy、sync_state cursor/batch；WS联合epoch/floor恢复复用同一判定 |
| 3 当前投影/变更类型/版本/无旧正文 | PASS：冻结六种SyncChange、Message/Bot/DeliveryMetadata、projection/apply状态；原S02设备通知设置和原S03全局设置变更仍属后卡，未用占位伪称全量 |
| 4 bootstrap/令牌/分页/版本/generation | PASS：四Bootstrap Schema、x-bootstrap-policy、两个结构扩展，token/page/verify/generation/catchup全例；WS联合seed/result、generation原子切换与保留任务门槛 |
| 5 read max/上界/未读 | PASS：UpdateReadRequest/Response、ReadState、x-read-policy、read/unread合成例，全部旧例重新执行 |
| 6 WS事件/生命周期/窗口 | PASS：六WS Schema及本卡109例，x-ws-policy、ws_state五类；25/75/60秒与无正文/前后台/未知可选/主版本均有声明及探针 |
| 7 数字/版本/令牌隔离/恢复/权限 | PASS：637例完整校验；SQLite64及非法版本、跨身份、重复分页/删除、旧revision、epoch/floor、Web/Bot越权、未知事件；42隔离探针确认关键声明破坏被拒绝 |
| 8 时间/keys/错误/锁/no-store | PASS：README两前卡及本卡完整索引；所有旧组件/例子/操作/两锁非回归；HTTP成功/错误统一no-store；frame单独校验，不以客户端时间排序 |

父S01八项在API契约、Schema、合成状态及声明检查范围内有完整覆盖；本实施者不自行登记父卡done，不替代独立审计/精确集成，更不放行G-API或P1/P2。

## 实际验证

环境 Linux，绝对解释器 `/tmp/relaynest-p0305-f01-locked/bin/python`，实际 `Python 3.13.15`；已存在的锁环境直接复用，未安装/升级依赖。

- `.../python --version`：PASS，3.13.15。
- `.../python -m openapi_spec_validator server/api/openapi.yaml`：PASS，OK。
- `.../python server/api/validate.py`：PASS，83 schemas；295 valid + 342 invalid；27 operations/security。包含全部前两卡及F01旧例。
- `.../python /tmp/relaynest-ws-check.py`：PASS，旧26操作/77 Schema/528例/全部components/元数据/辅助函数AST/两锁非回归；42项独立临时副本破坏探针全部非零退出且无Traceback，源码不受污染。复验脚本见附录。
- 42项探针：身份3、升级/缓存/正文/frame引用/参数/错误7、WS policy全部18项、frame Schema5、五类状态预期5、父seed/floor/generation与WS提示提交门槛4。首轮提示提交游标放宽探针暴露缺乏明确负例，已补ws_forbidden事件/窗口/恢复提交及seed切换四例，最终42项全通过。
- `git diff --check`：PASS；提交前检查差异及显式暂存路径，只含本卡五文件。

真实HTTP/WS握手及Accept计算、Origin部署配置、服务端Token签发/防篡改、心跳实际计时/慢端网络/重连退避、SQLite事务/并发、Android/Room/SyncCoordinator、手机/OPPO/NAS/外部服务/现网均 **NOT_RUN**。结构、权限或合成声明不能证明这些行为已实现/实测。未push、部署、发布、改运行时/技术设计/registry/旧报告或派agent。

读取流程偏差：初始约定/规约/父子卡和摘要读取过于集中，出现超过单次1800tokens约束的输出及截断，没有严格保持精简材料包预算；不声称预算精确达标。后续改用keys/局部读取，未重复整读API/设计/例子/旧报告；该偏差已上报调度，不删减验收或伪造技术检查。

## ≤800字下一卡交接

四操作：syncClientChanges GET /client/sync（cursor/limit→SyncPage）、bootstrapClient GET /client/bootstrap（phase/token/sync_cursor/limit→BootstrapResponse）、updateClientRead PUT /client/bots/{id}/read（through_seq→UpdateReadResponse）、connectClientWS GET /client/ws（Android Bearer/Origin、101无JSON、升级前400/401/403/429/503）。均仅Android客户端权限，HTTP响应no-store。完整keys见README“全同步域keys”，同步域新增Schema共22个：前卡12+4、本卡6；SyncChange仍只有message.upsert/deleted、bot.upsert/deleted、read.updated、delivery.upsert。Sequence为SQLite64非负字符串，Version为>=1整数；epoch/cursor不透明。保留400 INVALID_SYNC_CURSOR/INVALID_BOOTSTRAP_TOKEN、410 SYNC_RESET_REQUIRED/BOOTSTRAP_RESET_REQUIRED、404 RESOURCE_NOT_FOUND、422 READ_SEQ_OUT_OF_RANGE。时间为各响应server_time、bootstrap issued_at/expires_at、read.updated_at和实体/删除时间；WS无时间字段。hello/提示不推进游标；受保护bootstrap-seed不能冒充真正完成sync-result；复核epoch/floor后才切generation。原S02补设备/FCM/每设备通知配置及所属设置变更，原S03补全局配置/健康及全局设置变更；不能以任意object填分支，父S01契约覆盖不等于全G-API。实际WS/事务/Android/手机/OPPO/NAS仍NOT_RUN。最终提交后干净释放，交不同agent审计。

## 附录：本轮实际非回归与42项破坏脚本

在本工作树根目录以锁解释器执行下列脚本，临时目录自动清理；不依赖提交外业务代码。

```python
import ast,copy,json,subprocess,tempfile
from pathlib import Path
import yaml
root=Path.cwd();api=root/'server/api';py='/tmp/relaynest-p0305-f01-locked/bin/python';base='3cdb5e228b7d124890d45250728b260b2091a6db'
def old(name):return subprocess.check_output(['git','show',base+':server/api/'+name])
doc=yaml.safe_load((api/'openapi.yaml').read_text());prev=yaml.safe_load(old('openapi.yaml'));cases=json.loads((api/'examples.json').read_text());old_cases=json.loads(old('examples.json'));src=(api/'validate.py').read_text()
assert len(old_cases)==528 and cases[:528]==old_cases
for k,v in prev.items():
 if k not in ['paths','components']:assert doc[k]==v,k
for k,v in prev['paths'].items():assert doc['paths'][k]==v,k
for group,values in prev['components'].items():
 for k,v in values.items():assert doc['components'][group][k]==v,(group,k)
for f in ['requirements.in','requirements.txt']:assert (api/f).read_bytes()==old(f),f
before={n.name:ast.dump(n) for n in ast.parse(old('validate.py')).body if isinstance(n,ast.FunctionDef)}
after={n.name:ast.dump(n) for n in ast.parse(src).body if isinstance(n,ast.FunctionDef)}
for n,v in before.items():
 if n!='main':assert after[n]==v,n
print('NONREG PASS: 26 operations, all component groups/77 schemas, ordered 528 cases, top metadata, old helper AST, 2 locks unchanged')
mutations=[]
def change(label,fn):
 d=copy.deepcopy(doc);fn(d);mutations.append((label,d,cases,src))
ws=lambda d:d['paths']['/client/ws']['get']
for scheme in ['WebCookie','BotAPIToken','WebhookKey']:change('permission_'+scheme,lambda d,s=scheme:ws(d).update(security=[{s:[]}]))
change('101_cache',lambda d:ws(d)['responses']['101']['headers']['Cache-Control']['schema'].update(const='public'))
change('error_cache',lambda d:d['components']['responses']['401']['headers']['Cache-Control']['schema'].update(const='public'))
change('101_json_body',lambda d:ws(d)['responses']['101'].update(content={'application/json':{'schema':{'type':'object'}}}))
change('business_write',lambda d:ws(d).update(requestBody={'content':{'application/json':{'schema':{'type':'object'}}}}))
change('frame_reference',lambda d:ws(d).__setitem__('x-ws-frame-schema',{}))
change('origin_parameter',lambda d:ws(d).__setitem__('parameters',[]))
change('handshake_errors',lambda d:ws(d).__setitem__('x-error-codes',{}))
for key in ws(doc)['x-ws-policy']:
 change('policy_'+key,lambda d,k=key:ws(d)['x-ws-policy'].__setitem__(k,None))
for key in ['WSHello','WSChangesAvailable','WSSessionRevoked','WSUnknownOptional']:
 change('schema_'+key,lambda d,k=key:d['components']['schemas'][k].update(additionalProperties=True))
change('frame_any_object',lambda d:d['components']['schemas'].__setitem__('WSFrame',{'type':'object'}))
for kind in ['handshake','event','window','recovery','transport']:
 c=copy.deepcopy(cases);st=next(x['ws_state'] for x in c if x.get('ws_state',{}).get('kind')==kind)
 key={'handshake':'expected_status','event':'expected_action','window':'expected_action','recovery':'expected_verify_status','transport':'expected'}[kind]
 st[key]=not st[key] if isinstance(st[key],bool) else 'wrong'
 mutations.append(('state_'+kind,doc,c,src))
for label,needle,replacement in [
 ('parent_seed_origin',"state.get('verified_cursor_purpose') != 'sync-result'","state.get('verified_cursor_purpose') not in ('sync-result', 'bootstrap-seed')"),
 ('parent_floor',"int(state['position']) < int(state['floor'])","False"),
 ('parent_generation',"and state['preserve_uploads'] and state['atomic']","and state['atomic']"),
 ('ws_hint_commit',"and not state['hint_committed']","and True"),
]:
 assert needle in src,label
 # 函数放宽后另加对应反例，以证明联合门槛没有被绕过。
 c=copy.deepcopy(cases)
 mutations.append((label,doc,c,src.replace(needle,replacement)))
with tempfile.TemporaryDirectory(prefix='relaynest-ws-probe-') as tmp:
 p=Path(tmp)
 for label,d,c,code in mutations:
  (p/'openapi.yaml').write_text(yaml.safe_dump(d,allow_unicode=True));(p/'examples.json').write_text(json.dumps(c));(p/'validate.py').write_text(code)
  result=subprocess.run([py,str(p/'validate.py')],capture_output=True,text=True)
  assert result.returncode!=0 and 'Traceback' not in result.stderr,(label,result.stdout,result.stderr)
 print('MUTATIONS PASS:',len(mutations),'isolated nonzero exits without traceback')
```
