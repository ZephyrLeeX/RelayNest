# P03-07-S01-S02-F01 实施报告

- 角色：developer / 修复 BS-A01-01；交付状态：implemented，待不同 agent A02 复审，不自行关闭发现。
- 基线：`ab5fe9a1c4f4821877d677095d8ca251a7d93e27`；分支：`codex/api-bootstrap`；工作树：`/home/lijx/.codex/worktrees/api-bootstrap/RelayNest`，开始干净且HEAD精确匹配。
- 提交范围：上述基线至本报告所在的单个修复提交；最终提交SHA在提交后交接，不预写自身SHA。
- 仅修改API四文件与本报告；未改requirements、卡片、旧报告、registry、运行时、迁移、技术设计；无新增依赖、子agent、push或部署。

## 来源语义与兼容性

bootstrap专属 `x-bootstrap-policy.cursor_provenance` 规定：服务端验证不透明游标完整性保护内的签发用途，bootstrap-seed永不充当最终证明；仅实际sync返回且已完成轮次的结果可复核。客户端不能解码、构造或自报用途。`verified_cursor_purpose` 是夹具中的服务端已验证状态，不是新增HTTP参数或响应字段。此处没有实现令牌签发/密码学验证运行时。

seed原身份/epoch、position=H、完成位=true全部保留，因此已有sync规则仍在下次请求捕获当前新H；新增11会被追赶，不把seed固定成旧H未完成轮次。真实无变化sync完成position=H可通过；不要求最终位置严格大于H或等于verify时最新水位。旧sync/read公开字段、游标绑定、轮次、签发声明及validator函数均未修改，不触发扩界方案；错误优先级仍为receipt/分页→bootstrap重启→sync非法。

## 发现、验收与场景映射

| 验收 | 例名/断言与结果 |
| --- | --- |
| F01-01 / BS-A01-01 | 新增 `bootstrap_verify_seed_current_10`、`bootstrap_verify_seed_current_11`：同身份/epoch、完整性=true、位置H=10、完成位=true，current分别10/11，服务端已验证用途bootstrap-seed，均400 INVALID_SYNC_CURSOR。current仅用于标记对照，verify本身不要求等于实时水位。PASS |
| F01-02 | 新增 `bootstrap_verify_zero_change_sync` 与 `bootstrap_catchup_zero_change`：同样身份/epoch/H/完成位，实际sync结果用途；空轮次返回has_more=false/position10→verify200。PASS |
| F01-03 | 新增 `bootstrap_catchup_new_event`：seed完成位经冻结sync规则捕获11，批次返回事件11、位置11后verify200；`bootstrap_catchup_unfinished` 捕获12、limit1，返回11/has_more=true后verify400。沿用 `bootstrap_creation_after_H` 验证创建11不进入H10 bootstrap页面。PASS |
| F01-04 | 既有9个 `bootstrap_verify_*`（valid、receipt、all_pages_complete、bootstrap_valid、sync_integrity、sync_identity、sync_epoch、cycle_complete、position）全部保留；新增 `bootstrap_verify_reset_before_invalid_sync`、`bootstrap_verify_receipt_before_reset` 验证错误优先级。沿用expiry_boundary/epoch_change/floor_cross_h/token_floor_equal_h、跨身份/篡改/错误位置和generation反例；错误用途receipt及缺参由冻结query_rules/token_purpose及validator精确policy比较核对，未冒充HTTP执行。PASS（契约/合成范围） |
| F01-05 | 两个基线程序化逐值比较、原例顺序、两锁字节与sync validator AST比较均PASS，详见下文。 |
| F01-06 | 新增10项隔离破坏探针及独立重跑原20项探针均非零退出且无Traceback；没有污染源码。PASS |

## 冻结与有意差异

冻结基线 `33cc7585e873a5debaddc5b93c8657b60d5a73a8`：25操作、所有components分组的每项（含73 Schema、安全/参数/响应）、446例及次序逐值不变；两锁逐字节不变。

派发基线26操作/77 Schema/520例：操作数、Schema数不变，当前528例（240 valid + 288 invalid）。520个原例及顺序全部保留；下列为全部有意差异，没有静默删例：

| 路径/集合 | 原值→新值 | 理由/对应验收 |
| --- | --- | --- |
| bootstrapClient.x-bootstrap-policy.cursor_provenance | 不存在→protection/seed/catchup/accepted四项，精确值见API与复验脚本 | 服务端来源区分及新轮次，F01-01～03 |
| BootstrapPage.properties.sync_cursor.description | 不存在→服务端保护bootstrap-seed、位置H完成位及新轮次说明 | F01-01/03；ref与严格结构不变 |
| BootstrapVerified.description | 不存在→实际sync完成来源及位置可等于H说明 | F01-02；严格字段不变 |
| 上述9个bootstrap_verify状态.verified_cursor_purpose | 不存在→sync-result，原状态/预期值/例名完全保留 | 明确原测试前提，F01-04 |
| examples末尾 | 不存在→上述8个新增例名 | F01-01～04 |
| validate.py / README.md | 原verify没有来源拒绝→增加用途拒绝、catchup合成执行链与对应声明检查/说明 | F01-01～06；sync_state_valid AST不变 |

复验脚本去掉仅上述policy及两个description后，整个OpenAPI与派发基线逐值相等；逐例去掉9处新增状态后，原520例逐值相等。派发基线是532ea26祖先后续元数据链，API四文件及两锁在532ea26与ab5fe9a间逐字节相同。

## 实际检查与限制

解释器：`/tmp/relaynest-p0305-f01-locked/bin/python`，实际版本 **Python 3.13.15**；使用原锁，未安装或升级依赖。

| 实际命令 | 结果 |
| --- | --- |
| 上述绝对解释器 `--version` | PASS，Python 3.13.15 |
| 上述绝对解释器 `-m openapi_spec_validator server/api/openapi.yaml` | PASS，OK |
| 上述绝对解释器 `server/api/validate.py` | PASS，OpenAPI3.1 /77 schemas /240 valid+288 invalid /26 operations/security |
| 上述绝对解释器 `/tmp/relaynest-f01-check.py` | PASS，两基线全量逐值/顺序/锁/AST/精确差异；10项新增隔离探针 |
| 上述绝对解释器 `/tmp/relaynest-bootstrap-audit-probes.py` | PASS，两锁字节、16项文档探针+4项状态探针；本次重新执行 |
| `git diff --check`、逐文件diff与status检查 | PASS；仅授权5路径 |

首次生成examples时遇既有孤立代理字符的UTF-8编码错误；随后从本任务干净HEAD恢复该文件、保留原排版并定点新增，最终上述全量比较与检查通过。未以编码重写改变冻结例。

NOT_RUN：真实HTTP权限/缓存/令牌签发与防篡改、SQLite短事务/并发、附件竞态、Android/Room generation/崩溃恢复、真实WS及手机/OPPO/NAS端到端；没有相应运行时，契约夹具不能替代这些闭环。不放行G-API或P1/P2门槛。遗留：待A02对最终精确提交复审BS-A01-01与非回归。

## 可复验脚本

以下两段是本次实际执行脚本的完整副本；可从本报告提取，避免依赖临时文件继续存在。在本工作树执行：

```sh
/tmp/relaynest-p0305-f01-locked/bin/python - <<'EXTRACT'
from pathlib import Path
s=Path('docs/tasks/reports/P03-07-S01-S02-F01-implementation.md').read_text()
for name in ('relaynest-f01-check.py', 'relaynest-bootstrap-audit-probes.py'):
    script=s.split('<!-- '+name+' -->\n```python\n',1)[1].split('\n```',1)[0]
    Path('/tmp',name).write_text(script+'\n')
EXTRACT
/tmp/relaynest-p0305-f01-locked/bin/python /tmp/relaynest-f01-check.py
/tmp/relaynest-p0305-f01-locked/bin/python /tmp/relaynest-bootstrap-audit-probes.py
```

<!-- relaynest-f01-check.py -->
```python
import ast,copy,json,subprocess,tempfile
from pathlib import Path
import yaml
root=Path('/home/lijx/.codex/worktrees/api-bootstrap/RelayNest');api=root/'server/api'
py='/tmp/relaynest-p0305-f01-locked/bin/python'
def at(sha,name):return subprocess.check_output(['git','show',sha+':server/api/'+name],cwd=root)
doc=yaml.safe_load((api/'openapi.yaml').read_text());cases=json.loads((api/'examples.json').read_text());src=(api/'validate.py').read_text()
for sha,nops,nschemas,ncases in [('33cc7585e873a5debaddc5b93c8657b60d5a73a8',25,73,446),('ab5fe9a1c4f4821877d677095d8ca251a7d93e27',26,77,520)]:
 old=yaml.safe_load(at(sha,'openapi.yaml'));prior=json.loads(at(sha,'examples.json'))
 assert len(prior)==ncases and len(old['components']['schemas'])==nschemas
 assert sum(m in {'get','post','put','patch','delete'} for i in old['paths'].values() for m in i)==nops
 changes=[]
 for path,item in old['paths'].items():
  if item!=doc['paths'][path]:changes.append(path)
 assert changes==([] if nops==25 else ['/client/bootstrap']),changes
 for group,items in old['components'].items():
  changed=[k for k,v in items.items() if doc['components'][group][k]!=v]
  assert changed==(['BootstrapPage','BootstrapVerified'] if group=='schemas' and nschemas==77 else []),changed
 for i,c in enumerate(prior):
  current=copy.deepcopy(cases[i]);assert c['name']==current['name']
  if c.get('bootstrap_state',{}).get('kind')=='verify':
   assert current['bootstrap_state'].pop('verified_cursor_purpose')=='sync-result'
  assert current==c,c['name']
 for name in ('requirements.in','requirements.txt'):assert at(sha,name)==(api/name).read_bytes()
 # 同时冻结旧sync/read验证函数，避免局部实现改变旧语义。
 def funcs(s):return {n.name:ast.dump(n) for n in ast.parse(s).body if isinstance(n,ast.FunctionDef)}
 assert funcs(src)['sync_state_valid']==funcs(at(sha,'validate.py').decode())['sync_state_valid']
 print('BASELINE PASS',sha,nops,nschemas,ncases,'all components/examples/order/locks; sync validator unchanged')
# 对当前包仅允许精确新增policy与两个description，防止白名单吞掉无关变化。
current_prior=yaml.safe_load(at('ab5fe9a','openapi.yaml'))
normalized=copy.deepcopy(doc)
assert len(cases)==528 and len(doc['components']['schemas'])==77
assert normalized['paths']['/client/bootstrap']['get']['x-bootstrap-policy'].pop('cursor_provenance')=={
 'protection':'server-verified-integrity-bound-issuance-purpose-not-client-input',
 'seed':'bootstrap-seed-rejected-by-verify-even-if-complete-at-H',
 'catchup':'seed-starts-new-sync-cycle-capturing-current-high-watermark',
 'accepted':'actual-sync-result-and-completed-cycle-at-or-after-H'}
normalized['components']['schemas']['BootstrapPage']['properties']['sync_cursor'].pop('description')
normalized['components']['schemas']['BootstrapVerified'].pop('description')
assert normalized==current_prior
# 派发元数据祖先、四API与锁在前置审计/精确基线间未变。
assert subprocess.run(['git','merge-base','--is-ancestor','532ea26e791b9774bcd6c6246769499a40823b26','ab5fe9a1c4f4821877d677095d8ca251a7d93e27'],cwd=root).returncode==0
for n in ('openapi.yaml','examples.json','validate.py','README.md','requirements.in','requirements.txt'):assert at('532ea26',n)==at('ab5fe9a',n)
mutations=[
 ('remove-source-rejection',src.replace("or state.get('verified_cursor_purpose') != 'sync-result' or not state['cycle_complete']","or not state['cycle_complete']"),None,None),
 ('seed-treated-as-sync-result',src.replace("state.get('verified_cursor_purpose') != 'sync-result'","state.get('verified_cursor_purpose') not in ('sync-result', 'bootstrap-seed')"),None,None),
 ('remove-cycle-check',src.replace("or not state['cycle_complete']",''),None,None),
 ('require-position-greater-H',src.replace("int(state['position']) < int(state['h'])","int(state['position']) <= int(state['h'])"),None,None),
 ('seed-fixed-old-H',src.replace("'complete': state['seed_complete']","'complete': False"),None,None),
]
# 上述变化分别由原seed对照、未完成及零变化正例、catchup新事件链检出。
for label,key,val in [('seed-policy-unfinished','initial_sync_cursor','unfinished-cycle-at-H'),('source-not-protected','cursor_provenance',{})]:
 d=copy.deepcopy(doc);d['paths']['/client/bootstrap']['get']['x-bootstrap-policy'][key]=val;mutations.append((label,src,d,None))
for schema in ('BootstrapPage','BootstrapVerified','BootstrapMessage'):
 d=copy.deepcopy(doc);d['components']['schemas'][schema]['additionalProperties']=True;mutations.append(('strict-'+schema,src,d,None))
with tempfile.TemporaryDirectory(prefix='relaynest-f01-check-') as tmp:
 p=Path(tmp)
 for label,code,d,c in mutations:
  assert code!=src or d is not None or c is not None,label
  (p/'validate.py').write_text(code);(p/'openapi.yaml').write_text(yaml.safe_dump(d or doc));(p/'examples.json').write_text(json.dumps(c or cases))
  result=subprocess.run([py,str(p/'validate.py')],capture_output=True,text=True)
  assert result.returncode and 'Traceback' not in result.stderr,(label,result.stdout,result.stderr)
  print('PROBE PASS',label)
print('F01 10 isolated mutation probes PASS; 528 cases including unchanged ordered 520 coverage')
```

<!-- relaynest-bootstrap-audit-probes.py -->
```python
import copy, json, subprocess, tempfile
from pathlib import Path
import yaml
root=Path('/home/lijx/.codex/worktrees/api-bootstrap/RelayNest/server/api')
py='/tmp/relaynest-p0305-f01-locked/bin/python'
doc=yaml.safe_load((root/'openapi.yaml').read_text());cases=json.loads((root/'examples.json').read_text())
for name in ('requirements.in','requirements.txt'):
 assert (root/name).read_bytes()==subprocess.check_output(['git','show','33cc7585:server/api/'+name],cwd=root)
print('two requirements byte comparisons PASS')
mutations=[('web permission',lambda d:d['paths']['/client/bootstrap']['get'].update(security=[{'AdminCookie':[]}])),('bot permission',lambda d:d['paths']['/client/bootstrap']['get'].update(security=[{'BotAPIToken':[]}])),('cache',lambda d:d['paths']['/client/bootstrap']['get']['responses']['200']['headers']['Cache-Control']['schema'].update(const='public'))]
policy_mutations={'ttl_seconds':901,'cross_request_transaction':True,'message_boundary':'created-change_seq <= current','position':'offset','projection':'stale','generation':[],'token_binding':['high_watermark'],'query_rules':{},'hints':'advance','sliding_expiry':True}
for key,val in policy_mutations.items():
 mutations.append((key,lambda d,key=key,val=val:d['paths']['/client/bootstrap']['get']['x-bootstrap-policy'].__setitem__(key,val)))
mutations.extend([('created type',lambda d:d['components']['schemas']['BootstrapMessage']['properties'].__setitem__('created_change_seq',{'type':'integer'})),('page envelope',lambda d:d['components']['schemas']['BootstrapPage'].__setitem__('x-bootstrap-envelope',False)),('verified envelope',lambda d:d['components']['schemas']['BootstrapVerified'].__setitem__('x-bootstrap-verified',False))])
with tempfile.TemporaryDirectory(prefix='relaynest-audit-') as tmp:
 p=Path(tmp);(p/'validate.py').write_bytes((root/'validate.py').read_bytes())
 for name,mutate in mutations:
  d=copy.deepcopy(doc);mutate(d);(p/'openapi.yaml').write_text(yaml.safe_dump(d));(p/'examples.json').write_text(json.dumps(cases))
  r=subprocess.run([py,str(p/'validate.py')],capture_output=True,text=True)
  assert r.returncode!=0 and 'Traceback' not in r.stderr,(name,r.returncode,r.stdout,r.stderr)
 print('isolated document probes',len(mutations),'PASS')
 # 各恢复状态的错误预期破坏，确保声明分支实际被执行。
 state_probes=[]
 for kind in ['token','verify','page','generation']:
  c=next(x for x in cases if x.get('bootstrap_state',{}).get('kind')==kind)
  state_probes.append((kind,c['name']))
 for kind,name in state_probes:
  e=copy.deepcopy(cases);s=next(x['bootstrap_state'] for x in e if x['name']==name)
  if 'expected_status' in s:s['expected_status']=499
  else:s['expected']=not s['expected']
  (p/'openapi.yaml').write_text(yaml.safe_dump(doc));(p/'examples.json').write_text(json.dumps(e))
  r=subprocess.run([py,str(p/'validate.py')],capture_output=True,text=True)
  assert r.returncode!=0 and 'Traceback' not in r.stderr,(kind,r.stdout,r.stderr)
 print('isolated state probes',len(state_probes),'PASS')
print('TOTAL 20 mutation probes PASS')
```
