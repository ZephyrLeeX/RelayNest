# P03-05-F01 Content-Range 修复实施报告

```yaml
task_id: P03-05-F01
subject_id: P03-05
role: fixer
outcome: implemented
finding_ids: [P03-05-A01-F01]
base_sha: e1aa7a3aba162ade6c689ab0cdc09e07fbc3dad7
original_implementation_sha: b1777a0290cac38b95f00e349bae676702cf7f17
original_audit_sha: 1b4445891621aa41ffa4899664121574b762b851
worktree: /tmp/relaynest-run3/api-fix
branch: codex/api-fix-run3
report: docs/tasks/reports/P03-05-F01-implementation.md
```

开始时 HEAD 精确等于派发基线且工作树干净。本报告随修复提交，不预写自身 SHA；最终实现 SHA、提交范围由提交后交接提供。仅交付 implemented 待不同 agent 的 A02 复审，未关闭发现或父任务，未改 registry、历史审计或规划。

## 修改与验收

- 共享 ContentRange、下载206/416头部共三处 pattern 使用 `(?![\s\S])` 断言真正结尾；这是可用于 JSON Schema 的负向前瞻与字符类，未使用 Python 私有锚点。仅共同 Schema 增加说明注释。
- `content_range` 完整匹配 ASCII 单区间或星号形式；无匹配时显式产生 ValidationError，不 trim。保持 `0 <= start <= end < size`，星号的0和100大小仍合法。
- 保留原219示例并追加19反例：审计三基串分别附LF/CR/CRLF/额外字符共12例，另7例覆盖缺字段、空值、非ASCII数字、前导空格和 end > size。12个响应案例分别校验共同 Schema 和具体206/416头部；原正例及形态互换案例继续运行。
- 每个 ContentRange 示例额外以仅含 type 与 x-content-range 的 Schema 执行，独立验证扩展识别与数字语义，防止 pattern 掩盖扩展回归。

以上验收均 PASS。实际238例为92正例、146反例；未修改原 valid/schema_valid 预期或删除 Range 矩阵。独立探针确认仅三处模式解析后不同，其他契约解析后完全相等；原219例逐项保留，前卡9操作、28 Schema及所有公共组件保持，两个锁依赖文件字节不变。无运行时实现、接口权限、需求、架构或依赖变化。

## 实际环境与命令

Linux，Python 3.13.15、uv 0.12.13。旧 `/tmp/relaynest-p0304-locked/bin/python` 在本轮环境不存在，原绝对路径的两条检查为 NOT_RUN；依据本轮派发授权，从既有 mise 工具创建独立新环境并严格按原哈希锁重建，未升级锁版本。下列命令从工作树根执行，均退出0；同步18个锁包成功，跨文件系统硬链接警告自动退回复制，无验证失败。

```sh
/home/lijx/.local/share/mise/installs/uv/0.12.13/uv-x86_64-unknown-linux-gnu/uv venv --python /home/lijx/.local/share/mise/installs/python/3.13.15/bin/python3.13 /tmp/relaynest-p0305-f01-locked
/home/lijx/.local/share/mise/installs/uv/0.12.13/uv-x86_64-unknown-linux-gnu/uv pip sync --python /tmp/relaynest-p0305-f01-locked/bin/python --require-hashes server/api/requirements.txt
/tmp/relaynest-p0305-f01-locked/bin/python -m openapi_spec_validator server/api/openapi.yaml
/tmp/relaynest-p0305-f01-locked/bin/python server/api/validate.py
PYTHONDONTWRITEBYTECODE=1 /tmp/relaynest-p0305-f01-locked/bin/python /tmp/relaynest-p0305-f01-probe.py
git diff --check
```

| 检查 | 实际结果 |
| --- | --- |
| OpenAPI结构 | PASS：`server/api/openapi.yaml: OK` |
| 示例/声明主流程 | PASS：`OpenAPI 3.1; 44 schemas; 92 valid + 146 invalid examples; 18 operations/security` |
| 独立三模式探针 | PASS：每个模式单独拒绝三基串乘四尾缀，合计36组合；合法对应形态及206/416互换边界正确 |
| 独立扩展探针 | PASS：仅type和扩展的Schema拒绝尾缀、非法完整格式、逆区间、end等于/超过size、零大小单区间；保留四个合法边界值 |
| 基线语义/例子/依赖比较 | PASS：仅三pattern变化；原219例、前卡9操作/28Schema/公共组件与锁字节保留 |
| 独立Range矩阵 | PASS：1,287组，size=0..10、端点/长度0..12；闭区间/开放结尾/suffix |
| 声明负面变异 | PASS：跨Bot引用、Webhook凭据、下载Web Cookie、上传可选幂等、callback receive、编辑CAS、404缓存、图片大小八类均非零拒绝 |
| Python编译 | PASS：输出 `/tmp/relaynest-p0305-f01-validate.pyc`，仓库外 |
| 差异/路径 | PASS：`git diff --check`；提交前检查仅三份源码及本报告，未改README、依赖或台账 |

## 独立临时探针复现

上述 `/tmp/relaynest-p0305-f01-probe.py` 的实际内容如下。它在仓库外运行并使用临时目录进行声明变异，不写回契约或例子；临时目录自动回收。可将此代码保存到同名临时文件，再执行上面的命令。

```python
import contextlib, copy, importlib.util, io, json, py_compile, subprocess, tempfile
from pathlib import Path
import yaml

BASE = 'e1aa7a3aba162ade6c689ab0cdc09e07fbc3dad7'
def old(path, sha=BASE):
    return subprocess.check_output(['git', 'show', f'{sha}:{path}'])
spec = importlib.util.spec_from_file_location('contract', 'server/api/validate.py')
m = importlib.util.module_from_spec(spec)
spec.loader.exec_module(m)
d = yaml.safe_load(Path('server/api/openapi.yaml').read_text())
headers = d['paths']['/attachments/{id}']['get']['responses']
patterns = [d['components']['schemas']['ContentRange']['pattern']]
patterns += [headers[s]['headers']['Content-Range']['schema']['allOf'][1]['pattern'] for s in ('206','416')]
bases = ['bytes 9-0/1', 'bytes 0-1/100', 'bytes */100']
for pattern in patterns:
    v = m.Draft202012Validator({'type':'string', 'pattern':pattern})
    for base in bases:
        for suffix in ('\n','\r','\r\n','x'):
            assert not v.is_valid(base+suffix), (pattern, repr(base+suffix))
for pattern, yes, no in [(patterns[0], ['bytes 0-0/1','bytes */0'], []),
                         (patterns[1], ['bytes 0-99/100'], ['bytes */100']),
                         (patterns[2], ['bytes */0','bytes */100'], ['bytes 0-99/100'])]:
    v=m.Draft202012Validator({'pattern':pattern})
    assert all(v.is_valid(x) for x in yes)
    assert all(not v.is_valid(x) for x in no)
v=m.ContractValidator({'type':'string','x-content-range':True})
for value in ['bytes 0-0/1','bytes 0-99/100','bytes */0','bytes */100']:
    assert v.is_valid(value), value
bad=['','bytes 0-1/','bytes */','bytes ０-１/１００','bytes */１００',' bytes */100',
     'bytes 9-0/1','bytes 0-100/100','bytes 0-101/100','bytes 0-0/0']
bad += [base+suffix for base in bases for suffix in ('\n','\r','\r\n','x')]
assert all(not v.is_valid(value) for value in bad)
print('PASS: three patterns independently reject 36 suffix values; standalone extension format/semantic edges')
before=yaml.safe_load(old('server/api/openapi.yaml'))
normal=copy.deepcopy(d)
normal['components']['schemas']['ContentRange']['pattern']=before['components']['schemas']['ContentRange']['pattern']
for s in ('206','416'):
    normal['paths']['/attachments/{id}']['get']['responses'][s]['headers']['Content-Range']['schema']['allOf'][1]['pattern']=before['paths']['/attachments/{id}']['get']['responses'][s]['headers']['Content-Range']['schema']['allOf'][1]['pattern']
assert normal==before
cases=json.loads(Path('server/api/examples.json').read_text())
assert cases[:219]==json.loads(old('server/api/examples.json'))
for name in ('requirements.in','requirements.txt'):
    assert Path('server/api',name).read_bytes()==old('server/api/'+name)
prior=yaml.safe_load(old('server/api/openapi.yaml','30efad4f16234c07f42a2dd6e746d9d980b7dadf'))
for group, entries in prior['components'].items():
    for key, value in entries.items():
        assert d['components'][group][key]==value
for path, item in prior['paths'].items():
    assert d['paths'][path]==item
print('PASS: only three parsed schema patterns differ; 219 examples, prior 9 operations/28 schemas and lock bytes preserved')
count=0
for size in range(11):
    ranges=[(f'bytes={a}-{b}',a,b) for a in range(13) for b in range(a,13)]
    ranges += [(f'bytes={a}-',a,size-1) for a in range(13)]
    ranges += [(f'bytes=-{n}',max(0,size-n),size-1) for n in range(13)]
    for value,left,right in ranges:
        expected=[[left,min(right,size-1)]] if left<=min(right,size-1) else []
        assert m.range_intervals(value,size)==expected, (value,size)
        count+=1
assert count==1287
print('PASS: independent Range matrix',count)
ops={op['operationId']:op for item in d['paths'].values() for method,op in item.items() if method!='parameters'}
mutations=[]
x=copy.deepcopy(d); x['paths']['/bot/messages']['post'].pop('x-reference-policy'); mutations.append(x)
x=copy.deepcopy(d); x['paths']['/webhooks/{bot_id}']['post']['security']=[{'BotAPIToken':[]}]; mutations.append(x)
x=copy.deepcopy(d); x['paths']['/attachments/{id}']['get']['security'].append({'WebCookie':[]}); mutations.append(x)
x=copy.deepcopy(d)
for p in x['paths']['/attachments']['post']['parameters']:
    if m.resolve(x,p)['name']=='Idempotency-Key':
        p.clear(); p.update(copy.deepcopy(next(m.resolve(d,q) for q in ops['uploadAttachment']['parameters'] if m.resolve(d,q)['name']=='Idempotency-Key'))); p['required']=False
mutations.append(x)
x=copy.deepcopy(d); x['paths']['/client/bots/{id}/callbacks']['post'].pop('x-target-bot-capability'); mutations.append(x)
x=copy.deepcopy(d); x['components']['schemas']['EditMessageRequest']['required'].remove('expected_revision'); mutations.append(x)
x=copy.deepcopy(d); r=m.resolve(x,x['paths']['/bot/messages']['post']['responses']['404']); r['headers']['Cache-Control']['schema']['const']='public'; mutations.append(x)
x=copy.deepcopy(d); x['paths']['/attachments']['post']['requestBody']['x-default-max-bytes']['image']=1; mutations.append(x)
with tempfile.TemporaryDirectory(prefix='relaynest-f01-') as td:
    root=Path(td); (root/'examples.json').write_text(Path('server/api/examples.json').read_text())
    original=m.ROOT; m.ROOT=root
    try:
        for index,x in enumerate(mutations):
            (root/'openapi.yaml').write_text(yaml.safe_dump(x,allow_unicode=True))
            try:
                with contextlib.redirect_stdout(io.StringIO()): m.main()
            except SystemExit as error:
                assert error.code, index
            else: raise AssertionError(('mutation accepted',index))
    finally: m.ROOT=original
print('PASS: 8 declaration mutations rejected (authorization/idempotency/CAS/cache/size)')
py_compile.compile('server/api/validate.py',cfile='/tmp/relaynest-p0305-f01-validate.pyc',doraise=True)
print('PASS: py_compile output outside repository')
```

## 限制与交接

NOT_RUN：真实HTTP身份/跨Bot授权/凭据撤销、幂等墓碑与CAS并发、callback快照事务、上传流限制与fsync/rename/恢复/清理竞态、真实Range多段编码及ETag/条件响应、Android/Web客户端、真机、RelayShelf、部署。现有交付仍是契约与校验器，以上声明测试不证明运行时安全或文件服务行为。

无新增需求/架构决策、无其他未解决实现事项；唯一发现的关闭由不同 agent 的精确SHA A02复审判断。未读取凭据、派额外agent、推送、发布或部署。
