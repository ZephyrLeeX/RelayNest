# P01-06 补充检查复现记录

日期：2026-10-21。以下脚本是研究检查材料，不是正式插件实现。检查对象为提交 `061c0b8` 的探针和现有 v3.1.5 容器。脚本只使用标准库及容器已有依赖。

## 本地协议桩负面检查

将脚本保存为 `/tmp/relaynest-p106-negative.py`，在仓库根运行 `PYTHONDONTWRITEBYTECODE=1 python /tmp/relaynest-p106-negative.py`。结果：success=true，11项通过；不加载MP。

```python
"""补充协议桩负面测试；不加载 MP，不运行消息业务。"""
import copy
import importlib.util
import json
from pathlib import Path
from types import SimpleNamespace
import threading
import urllib.error
import urllib.request

path = Path('/home/lijx/Projects/RelayNest/integrations/moviepilot/poc/relaynestprobe/transport.py')
spec = importlib.util.spec_from_file_location('p106_transport', path)
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)
checks = []

def rejected(url, data, label):
    try:
        module.request(url, data)
    except urllib.error.HTTPError as error:
        assert error.code == 400
    else:
        raise AssertionError(label)
    checks.append(label)

with module.receiver() as active:
    active.update(run='p106-local', sent=[])
    sender = module.TransportProbeMixin()
    sender._transport = active
    message = SimpleNamespace(source='RelayNest-P1Probe:A', userid='RelayNest-P1Probe:A:user',
                              text='relaynest-p1probe:synthetic', title='RelayNest P1 probe',
                              image=None, file_path=None)
    mid = sender._transport_send('direct', message)
    payload = active['sent'][0]['payload']
    row = active['state']['messages'][mid]
    changed = copy.deepcopy(payload)
    changed['text'] = 'changed-payload'
    rejected(active['url'] + '/messages', changed, 'same_key_conflicting_payload_400')
    assert active['state']['creates'] == 1 and row['text'] == message.text
    bad = dict(payload, source='OtherBot')
    rejected(active['url'] + '/messages', bad, 'foreign_source_400')
    bad = dict(payload, userid='real-user')
    rejected(active['url'] + '/messages', bad, 'foreign_user_400')
    edit = dict(source='RelayNest-P1Probe:B', userid='RelayNest-P1Probe:B:user', message_id=mid, text='wrong')
    rejected(active['url'] + '/edit', edit, 'cross_bot_edit_400')
    edit.update(source=message.source, userid=message.userid, message_id='not-owned')
    rejected(active['url'] + '/edit', edit, 'unknown_message_edit_400')
    corrupt = copy.deepcopy(payload)
    corrupt.update(key='corrupt-file', attachments=[dict(kind='file', bytes='YnJva2Vu', sha256='wrong')])
    rejected(active['url'] + '/messages', corrupt, 'corrupt_attachment_400')
    assert active['state']['creates'] == 1 and active['state']['edits'] == 0 and row['revision'] == 1
    checks.append('rejections_do_not_mutate_rows')
    edit.update(message_id=mid, text='relaynest-p1probe:edited')
    module.request(active['url'] + '/edit', edit)
    replay = module.request(active['url'] + '/messages', payload)
    assert replay['message_id'] == mid and row['revision'] == 2 and row['text'] == edit['text']
    checks.append('create_replay_after_edit_does_not_revert')
    Path(active['file']).unlink()
    message.file_path = active['file']
    try:
        sender._transport_send('direct', message)
    except FileNotFoundError:
        pass
    else:
        raise AssertionError('missing_file')
    assert active['state']['creates'] == 1
    checks.append('missing_fixture_file_not_sent')
first_url = active['url']
assert not Path(active['file']).exists()
assert not any(t.name == 'relaynest-p105-receiver' for t in threading.enumerate())
try:
    module.request(first_url + '/messages', payload)
except urllib.error.URLError:
    checks.append('closed_receiver_unreachable')
else:
    raise AssertionError('receiver_remains_open')
try:
    with module.receiver() as failed:
        raise RuntimeError('fixed-test-failure')
except RuntimeError:
    pass
assert not Path(failed['file']).exists()
assert not any(t.name == 'relaynest-p105-receiver' for t in threading.enumerate())
checks.append('exception_cleanup_no_thread_or_fixture')
print(json.dumps({'kind': 'local_protocol_stub_negative_checks', 'count': len(checks), 'checks': checks, 'success': True}))
```

## 容器消息模型与渠道能力

将脚本保存为 `/tmp/relaynest-p106-models.py`；通过 `ssh -F /home/lijx/.ssh/config fnos 'docker exec -i -e PYTHONDONTWRITEBYTECODE=1 -w /app moviepilot-v2 /opt/venv/bin/python -B -' < /tmp/relaynest-p106-models.py` 运行。结果：3个模型接受None、拒绝RelayNest字符串；6项能力均false；运行态scope不序列化。

```python
"""只读模型回归；不导入或调用消息业务链。"""
import json
from pydantic import ValidationError
from app.schemas.types import NotificationChannel
from app.schemas.message import IncomingMessage, Message, MessageResponse
from app.schemas.notification import ChannelCapabilityManager
rows = []
for model in (IncomingMessage, Message, MessageResponse):
    for channel in (None, 'RelayNest'):
        try:
            value = model(channel=channel, source='RelayNest-P1Probe:A')
            row = {'model': model.__name__, 'channel': channel, 'accepted': True,
                   'source_preserved': value.source == 'RelayNest-P1Probe:A'}
        except ValidationError as error:
            row = {'model': model.__name__, 'channel': channel, 'accepted': False,
                   'errors': [{'loc': list(e['loc']), 'type': e['type']}
                              for e in error.errors(include_input=False, include_url=False)]}
        assert row['accepted'] == (channel is None)
        rows.append(row)
capabilities = {str(channel): {'buttons': ChannelCapabilityManager.supports_buttons(channel),
                              'callbacks': ChannelCapabilityManager.supports_callbacks(channel),
                              'editing': ChannelCapabilityManager.supports_editing(channel)}
                for channel in (None, 'RelayNest')}
assert all(not flag for row in capabilities.values() for flag in row.values())
message = Message(channel=None, source='RelayNest-P1Probe:A', save_history=False)
message.set_notification_route_scope('user')
assert message.notification_route_scope == 'user'
assert 'notification_route_scope' not in message.model_dump()
assert '_notification_route_scope' not in message.model_dump()
print(json.dumps({'kind': 'container_separate_process_model_checks', 'models': rows,
                  'capabilities': capabilities, 'route_scope_serialized': False,
                  'relaynest_channel_registered': any(c.value == 'RelayNest' for c in NotificationChannel)}, ensure_ascii=False))
```

## 容器自动编辑gate组件检查

同上将stdin脚本换为 `/tmp/relaynest-p106-edit-gate.py`。结果：edit_provider_calls=0、post_fallback_calls=1、provided_context_preserved_in_fallback=true、real_delivery=false、success=true。使用真实宿主函数和记录链，不调用真实业务发送。

```python
"""容器独立进程组件检查；chain 是记录桩，不进行真实发送。"""
import json
from app.application.messaging.interaction import update_or_post_message

class RecordingChain:
    def __init__(self):
        self.edits = []
        self.posts = []
    def edit_message(self, **kwargs):
        self.edits.append(kwargs)
        return True
    def post_message(self, message):
        self.posts.append(message)

chain = RecordingChain()
update_or_post_message(chain=chain, channel=None, source='RelayNest-P1Probe:A',
                       userid='RelayNest-P1Probe:A:user', username='RelayNest probe',
                       title='RelayNest P1 probe', text='relaynest-p1probe:synthetic',
                       original_message_id='p106-owned-message', original_chat_id='p106-owned-chat')
assert len(chain.edits) == 0 and len(chain.posts) == 1
message = chain.posts[0]
assert message.original_message_id == 'p106-owned-message'
assert message.original_chat_id == 'p106-owned-chat'
assert message.channel is None and message.save_history is False
print(json.dumps({'kind':'container_component_with_recording_chain',
                  'edit_provider_calls':len(chain.edits), 'post_fallback_calls':len(chain.posts),
                  'provided_context_preserved_in_fallback':True, 'real_delivery':False,
                  'success':True}))
```

## 其它实际检查

- 既有 `verify_probe.py` 通过，未修改。
- 容器inspect及源码SHA-256见[门槛报告](P01-06-implementation.md)。
- 通过既有只读清单脚本访问 `/api/v1/plugin/installed`：HTTP200，8个ID与P01-05一致。
- 既有 `probe_api.py status`：HTTP404、退出码1为预期接口已移除；不是模型脚本失败。
- 探针目录不存在，`find /tmp -maxdepth 1 -name relaynest-p105-\*` 无输出；健康及StartedAt未变。
- 检查脚本仅保存在本地/tmp及本Markdown，无远端落盘源码。
