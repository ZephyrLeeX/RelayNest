"""校验结构、示例及权限声明；字节/Range 扩展仅验证契约，不模拟 HTTP 服务。"""

import copy
from datetime import datetime
import json
import re
from pathlib import Path

import yaml
from jsonschema import Draft202012Validator, FormatChecker, ValidationError, validators
from openapi_spec_validator import validate_spec

ROOT = Path(__file__).resolve().parent
RANGES = re.compile(r'bytes=(?:[0-9]+-[0-9]*|-[0-9]+)(?:, *(?:[0-9]+-[0-9]*|-[0-9]+))*')


def utf8_max_bytes(validator, limit, instance, schema):
    if isinstance(instance, str):
        try:
            size = len(instance.encode('utf-8'))
        except UnicodeEncodeError:
            yield ValidationError('字符串不能编码为合法 UTF-8')
            return
        if size > limit:
            yield ValidationError('UTF-8 字节数超过契约限制')


def byte_ranges(validator, enabled, instance, schema):
    if not enabled or not isinstance(instance, str):
        return
    if not RANGES.fullmatch(instance):
        yield ValidationError('Range 字节语法错误')
        return
    for part in instance[6:].split(','):
        start, end = part.strip().split('-')
        if start and end and int(start) > int(end):
            yield ValidationError('Range 起始位置大于结束位置')


def content_range(validator, enabled, instance, schema):
    if not enabled or not isinstance(instance, str):
        return
    match = re.fullmatch(r'bytes ([0-9]+)-([0-9]+)/([0-9]+)', instance)
    if match:
        start, end, size = map(int, match.groups())
        if not 0 <= start <= end < size:
            yield ValidationError('Content-Range 与实际区间大小矛盾')
        return
    # 星号形式同样要求完整匹配；解析失败必须拒绝，不能跳过语义门槛。
    if not re.fullmatch(r'bytes \*/[0-9]+', instance):
        yield ValidationError('Content-Range 字节语法错误')


def sync_envelope(validator, enabled, instance, schema):
    """只核对批次结构；固定H、读事务及权限由声明夹具覆盖，不模拟运行时。"""
    if not enabled or not isinstance(instance, dict):
        return
    changes = instance.get('changes')
    high = instance.get('server_high_watermark')
    if not isinstance(changes, list) or not isinstance(high, str) or not high.isdecimal():
        return
    positions = [item.get('change_seq') for item in changes if isinstance(item, dict)]
    if any(not isinstance(pos, str) or not pos.isdecimal() for pos in positions):
        return
    positions = [int(pos) for pos in positions]
    if any(pos <= 0 or pos > int(high) for pos in positions):
        yield ValidationError('变更必须在正序号至H之间')
    if any(left >= right for left, right in zip(positions, positions[1:])):
        yield ValidationError('变更序号必须严格递增')
    if instance.get('has_more') and (not positions or positions[-1] >= int(high)):
        yield ValidationError('非末页必须非空且最后位置小于H')


def sync_state_valid(state):
    """核对合成状态声明的预期；不是HTTP/SQLite/并发或客户端实测。"""
    kind = state['kind']
    if kind == 'cursor':
        if state['identity'] != state['cursor_identity'] or not state.get('integrity', True):
            status = 400
        elif state['epoch'] != state['cursor_epoch'] or int(state['position']) < int(state['floor']):
            status = 410
        else:
            status = 200
        high = state['server_h'] if state['complete'] else state['h']
        return status == state['expected_status'] and (status != 200 or high == state['expected_h'])
    if kind == 'projection':
        valid = state['current_revision'] >= state['event_revision']
        valid = valid and state['result_type'] == ('message.deleted' if state['deleted'] else 'message.upsert')
        valid = valid and state['contains_body'] != state['deleted']
        return valid == state['expected']
    if kind == 'apply':
        return (state['incoming_version'] > state['local_version']) == state['expected_apply']
    if kind == 'read':
        old, through, upper = (int(state[key]) for key in ('old', 'through', 'upper'))
        status = 422 if through > upper else 200
        return status == state['expected_status'] and (status != 200 or (
            str(max(old, through)) == state['expected_read']
            and (through > old) == state['expected_advance']))
    if kind == 'batch':
        # 比较已知有序事件区间，防止夹具用next_cursor掩盖遗漏事件。
        eligible = [pos for pos in state['events']
                    if int(state['position']) < int(pos) <= int(state['h'])]
        expected = eligible[:state['limit']]
        more = len(eligible) > state['limit']
        next_position = expected[-1] if more else state['h']
        valid = (state['returned'] == expected and state['has_more'] == more
                 and state['next_position'] == next_position)
        return valid == state['expected']
    if kind == 'unread':
        count = sum(message['exists'] and message['sender'] == 'application'
                    and int(message['bot_seq']) > int(state['read']) for message in state['messages'])
        return count == state['expected_unread']
    return False


def bootstrap_envelope(validator, enabled, instance, schema):
    """仅检查重建响应的跨字段边界，事务与客户端切换仍由实现任务验证。"""
    if not enabled or not isinstance(instance, dict):
        return
    try:
        high = int(instance['server_high_watermark'])
        floor = int(instance['retention_floor'])
        keys = [(item['message']['bot_id'], int(item['message']['bot_seq']))
                for item in instance['messages']]
        if floor > high or any(int(item['created_change_seq']) > high
                               for item in instance['messages']):
            yield ValidationError('bootstrap记录创建位置或floor超过H')
        if keys != sorted(set(keys)):
            yield ValidationError('bootstrap消息必须按不可变键严格递增且无重复')
        if instance['has_more'] != (instance['next_token'] is not None):
            yield ValidationError('bootstrap末页token边界错误')
        issued = datetime.fromisoformat(instance['issued_at'].replace('Z', '+00:00'))
        expires = datetime.fromisoformat(instance['expires_at'].replace('Z', '+00:00'))
        now = datetime.fromisoformat(instance['server_time'].replace('Z', '+00:00'))
        if (expires - issued).total_seconds() != 900 or not issued <= now < expires:
            yield ValidationError('bootstrap固定有效期必须为15分钟且响应未过期')
        bot_ids = [bot['id'] for bot in instance['bots']]
        read_ids = [read['bot_id'] for read in instance['read_states']]
        if len(set(bot_ids)) != len(bot_ids) or set(read_ids) != set(bot_ids) or len(read_ids) != len(bot_ids):
            yield ValidationError('Bot及共享已读快照必须逐Bot一一对应')
        if any(key[0] not in bot_ids for key in keys):
            yield ValidationError('bootstrap消息必须属于冻结Bot集合')
    except (KeyError, TypeError, ValueError):
        # 结构和字段格式交由标准Schema拒绝，避免扩展对非法夹具抛异常。
        return


def bootstrap_verified(validator, enabled, instance, schema):
    if not enabled or not isinstance(instance, dict):
        return
    try:
        high, floor, position = (int(instance[key]) for key in
                                ('server_high_watermark', 'retention_floor', 'catchup_position'))
        if not floor <= high <= position:
            yield ValidationError('复核要求floor≤原H≤已完成追赶位置')
    except (KeyError, TypeError, ValueError):
        return


def bootstrap_state_valid(state):
    """合成恢复声明，显式区分预期拒绝与真实服务端/Room验证。"""
    kind = state['kind']
    if kind == 'token':
        invalid = (not state['integrity'] or state['identity'] != state['token_identity']
                   or state['position'] != state['token_position'])
        restart = (state['now'] >= state['expires'] or state['epoch'] != state['token_epoch']
                   or int(state['floor']) > int(state['h']))
        status = 400 if invalid else 410 if restart else 200
        return status == state['expected_status']
    if kind == 'verify':
        if not state['receipt'] or not state['all_pages_complete']:
            status = 400
        elif not state['bootstrap_valid']:
            status = 410
        elif (not state['sync_integrity'] or state['identity'] != state['sync_identity']
              or state['epoch'] != state['sync_epoch']
              or state.get('verified_cursor_purpose') != 'sync-result' or not state['cycle_complete']
              or int(state['position']) < int(state['h'])):
            status = 400
        else:
            status = 200
        code = ('INVALID_BOOTSTRAP_TOKEN' if not state['receipt'] or not state['all_pages_complete']
                else 'BOOTSTRAP_RESET_REQUIRED' if status == 410
                else 'INVALID_SYNC_CURSOR' if status == 400 else None)
        return status == state['expected_status'] and code == state['expected_code']
    if kind == 'catchup':
        # 合成服务端执行链：seed的完成位必须让冻结sync规则捕获新H；
        # 实际批次产出的完成位/位置进入verify，来源不是客户端请求字段。
        cursor = {'kind': 'cursor', 'identity': 'a1', 'cursor_identity': 'a1',
                  'epoch': 'e1', 'cursor_epoch': 'e1', 'position': state['h'],
                  'floor': state['h'], 'complete': state['seed_complete'],
                  'h': state['h'], 'server_h': state['current'],
                  'expected_h': state['current'], 'expected_status': 200}
        batch = {'kind': 'batch', 'position': state['h'], 'h': state['current'],
                 'events': state['events'], 'limit': state['limit'],
                 'returned': state['returned'], 'has_more': state['has_more'],
                 'next_position': state['next_position'], 'expected': True}
        proof = dict(state['verify'], verified_cursor_purpose='sync-result',
                     cycle_complete=not state['has_more'], position=state['next_position'])
        return (sync_state_valid(cursor) and sync_state_valid(batch)
                and bootstrap_state_valid(proof))
    if kind == 'page':
        # 当前投影扫描；删除可使页变空，附件消失不伪造文件可用。
        rows = sorted(state['rows'], key=lambda row: (row['bot_id'], int(row['bot_seq'])))
        eligible = [row for row in rows if int(row['created']) <= int(state['h'])
                    and (row['bot_id'], int(row['bot_seq'])) >
                        (state['position'][0], int(state['position'][1]))]
        scanned = eligible[:state['limit']]
        expected = [{'id':row['id'], 'revision':row['revision'],
                     'attachment_exists':row['attachment_exists']}
                    for row in scanned if not row['deleted']]
        position = [scanned[-1]['bot_id'], scanned[-1]['bot_seq']] if scanned else state['position']
        return ((state['returned'] == expected and state['next_position'] == position
                 and state['has_more'] == (len(eligible) > state['limit'])) == state['expected'])
    if kind == 'generation':
        permitted = (state['all_pages'] and state['sync_complete']
                     and int(state['catchup_position']) >= int(state['h'])
                     and state['epoch'] == state['initial_epoch']
                     and int(state['floor']) <= int(state['h']) and state['token_valid'])
        valid = (state['switch'] == permitted and state['preserve_outbox']
                 and state['preserve_uploads'] and state['atomic']
                 and (not state['switch'] or state['remove_old_cache'])
                 and state['message_result'] == max(state['message_local'], state['message_incoming'])
                 and state['bot_result'] == max(state['bot_local'], state['bot_incoming'])
                 and not state['hint_advances_cursor'])
        return valid == state['expected']
    return False


ContractValidator = validators.extend(Draft202012Validator, {
    'x-utf8-max-bytes': utf8_max_bytes,
    'x-byte-ranges': byte_ranges,
    'x-content-range': content_range,
    'x-sync-envelope': sync_envelope,
    'x-bootstrap-envelope': bootstrap_envelope,
    'x-bootstrap-verified': bootstrap_verified,
})


def ws_state_valid(state):
    """合成连接/窗口/恢复声明；不模拟真实网络、时钟或持久化。"""
    kind = state['kind']
    if kind == 'handshake':
        if state['identity'] != 'AdminBearer' or not state['session_active']:
            status = 401
        elif state['origin'] not in ('absent', 'trusted'):
            status = 403
        elif state['query_credentials'] or not state['upgrade_valid']:
            status = 400
        else:
            status = 101
        return status == state['expected_status']
    if kind == 'event':
        event = state['event']
        if state['major'] != 1:
            action = 'fail-close'
        elif event == 'session_revoked':
            action = 'stop-close-clear-session'
        elif event in ('hello', 'changes_available'):
            action = 'rest-sync'
        else:
            action = 'ignore' if state['optional'] else 'fail-close'
        # 提示、hello、未知事件和不兼容事件全部不得更新持久游标。
        return action == state['expected_action'] and not state['advance_cursor']
    if kind == 'window':
        # 两种顺序都比较缓存提示/hello；相同epoch内才比较水位。
        if state['epoch'] != state['local_epoch']:
            action = 'rest-reset-check'
        else:
            target = max(int(state['hello']), int(state['buffered_hint']))
            action = 'rest-sync' if int(state['rest_position']) < target else 'caught-up'
        return (state['strategy'] in ('connect-first', 'sync-first')
                and action == state['expected_action'] and not state['hint_committed'])
    if kind == 'recovery':
        # 复用前卡的固定H/epoch/floor与真正sync完成来源检查，不复制或放宽。
        sync = state['sync_state']
        verify = state['bootstrap_state']
        generation = state['generation_state']
        return (bootstrap_state_valid(generation)
                and generation['switch'] == (verify['expected_status'] == 200)
                and sync_state_valid(sync) and bootstrap_state_valid(verify)
                and sync['expected_status'] == 410
                and verify['expected_status'] == state['expected_verify_status']
                and not state['hint_committed'])
    if kind == 'transport':
        return (state['ping_seconds'] == 25 and state['disconnect_seconds'] == 75
                and state['jitter'] and 0 <= state['retry_delay'] <= 60
                and state['queued_hints'] <= 1 and not state['queued_body']
                and state['background_action'] == 'close'
                and state['foreground_action'] == 'immediate-rest-sync') == state['expected']
    return False


def device_session_state_valid(state):
    """执行合成序列并逐步比对完整快照；不执行HTTP、数据库或真实WS。"""
    sessions = copy.deepcopy(state['sessions'])
    for step in state['steps']:
        action = step['action']
        status, invalidated, accepted = None, [], None
        if action == 'login':
            # 登录只增加新身份，不能以device_id替换或注销已有会话。
            session = step['session']
            if session['id'] in sessions:
                return False
            sessions[session['id']] = copy.deepcopy(session)
        elif action == 'callback':
            target = sessions.get(step['id'])
            accepted = bool(target and target['active']
                            and step['source_session_id'] == target['id'])
            if accepted:
                target['results'].append(step['result'])
        else:
            # 路径只按Session.id选择；不存在/无权/失效不回退到同设备会话。
            target_id = state['current_session_id'] if action == 'logout' else step['id']
            target = sessions.get(target_id)
            caller = sessions[state['current_session_id']]
            status = (401 if not caller['active'] else 403
                      if state['actor'] != 'admin' or state['credential'] not in ('bearer', 'cookie')
                      or (state['credential'] == 'cookie'
                          and not (state['csrf'] and state['same_origin']))
                      else 404 if not target or not target['active']
                      or target['owner'] != caller['owner'] else 204)
            if status == 204:
                invalidated = [target_id + ':' + resource for resource in target['resources']]
                target['active'] = False
                target['resources'] = []
        if ({'status': status, 'invalidated': invalidated, 'callback': accepted,
             'sessions': sessions, 'bot_credential_active': True} != step['expected']):
            return False
    return True


def range_intervals(value, size):
    """将合法示例映射至预期区间；只是契约边界计算，不验证文件服务。"""
    result = []
    for part in value[6:].split(','):
        start, end = part.strip().split('-')
        if start:
            left = int(start)
            right = min(int(end), size - 1) if end else size - 1
        else:
            left = max(0, size - int(end))
            right = size - 1
        if left <= right:
            result.append([left, right])
    return result


def resolve(document, value):
    while '$ref' in value:
        target = document
        for segment in value['$ref'][2:].split('/'):
            target = target[segment]
        value = target
    return value


DEVICE_POLICY = {'identity': 'Session.id-exact-target-and-generation',
 'generation': 'new-session-id-every-login-never-reuse',
 'scope': 'current-administrator-active-sessions-only',
 'cookie_write': 'csrf-and-same-origin-origin-or-referer',
 'revoke': 'atomic-entire-target-session-all-providers-and-ws',
 'late_callback': 'reject-revoked-or-mismatched-session-id-before-commit',
 'logout': 'current-session-only-other-devices-and-bot-credentials-unchanged'}
DEVICE_PAGE = {'type': 'object',
 'properties': {'items': {'type': 'array', 'items': {'$ref': '#/components/schemas/Session'}},
                'next_cursor': {'oneOf': [{'$ref': '#/components/schemas/OpaqueCursor'},
                                          {'type': 'null'}]},
                'has_more': {'type': 'boolean'}},
 'required': ['items', 'next_cursor', 'has_more'],
 'additionalProperties': False}

WS_POLICY = {'supported_major': 1,
 'origin': 'native-absent-allowed-present-must-pass-configured-origin-policy',
 'authentication': 'current-android-device-session-before-upgrade-no-query-credentials',
 'upgrade': 'http-101-no-json-body-frames-validated-separately',
 'events': 'hello-first-latest-committed-change_seq-rest-hints-only-no-business-writes',
 'ping_seconds': 25,
 'no_response_timeout_seconds': 75,
 'heartbeat': 'websocket-control-ping-pong-not-json-events',
 'slow_client': 'coalesce-latest-changes_available-bounded-no-body-buffer',
 'reconnect': 'exponential-with-jitter-cap-60-seconds',
 'background': 'close',
 'foreground': 'immediate-single-flight-rest-sync',
 'window': ['connect-cache-hints-then-catch-up-from-persisted-cursor',
            'sync-then-recheck-hello-high-watermark'],
 'cursor': 'only-rest-batch-and-cursor-atomic-commit-never-hello-or-hints',
 'recovery': 'epoch-mismatch-or-below-floor-rest-410-rebuild-bootstrap-verify-before-generation-switch',
 'revoked': 'stop-sync-close-clear-session-no-cached-body-display',
 'unknown_optional': 'ignore-only-unknown-optional-event-on-compatible-major',
 'incompatible_major': 'explicit-failure-close-no-frame-application'}
WS_PARAMETERS = [{'name': 'Origin',
  'in': 'header',
  'required': False,
  'schema': {'type': 'string', 'format': 'uri'}},
 {'name': 'Upgrade',
  'in': 'header',
  'required': True,
  'schema': {'type': 'string', 'const': 'websocket'}},
 {'name': 'Connection',
  'in': 'header',
  'required': True,
  'schema': {'type': 'string', 'const': 'Upgrade'}},
 {'name': 'Sec-WebSocket-Version',
  'in': 'header',
  'required': True,
  'schema': {'type': 'string', 'const': '13'}},
 {'name': 'Sec-WebSocket-Key',
  'in': 'header',
  'required': True,
  'schema': {'type': 'string', 'pattern': '^[A-Za-z0-9+/]{22}==$'}}]
WS_ERRORS = {'400': ['INVALID_WS_HANDSHAKE'], '401': ['AUTHENTICATION_REQUIRED'], '403': ['FORBIDDEN']}
WS_SCHEMAS = {'WSProtocolVersion': {'type': 'object',
                       'properties': {'major': {'type': 'integer', 'minimum': 1},
                                      'minor': {'type': 'integer', 'minimum': 0}},
                       'required': ['major', 'minor'],
                       'additionalProperties': False},
 'WSHello': {'type': 'object',
             'properties': {'type': {'const': 'hello'},
                            'epoch': {'$ref': '#/components/schemas/SyncEpoch'},
                            'change_seq': {'$ref': '#/components/schemas/Sequence'},
                            'protocol_version': {'$ref': '#/components/schemas/WSProtocolVersion'}},
             'required': ['type', 'epoch', 'change_seq', 'protocol_version'],
             'additionalProperties': False},
 'WSChangesAvailable': {'type': 'object',
                        'properties': {'type': {'const': 'changes_available'},
                                       'change_seq': {'$ref': '#/components/schemas/Sequence'}},
                        'required': ['type', 'change_seq'],
                        'additionalProperties': False},
 'WSSessionRevoked': {'type': 'object',
                      'properties': {'type': {'const': 'session_revoked'}},
                      'required': ['type'],
                      'additionalProperties': False},
 'WSUnknownOptional': {'type': 'object',
                       'properties': {'type': {'type': 'string',
                                               'pattern': '^[a-z][a-z0-9_]*$',
                                               'not': {'enum': ['hello',
                                                                'changes_available',
                                                                'session_revoked']}},
                                      'optional': {'const': True}},
                       'required': ['type', 'optional'],
                       'additionalProperties': False},
 'WSFrame': {'description': '仅服务端JSON提示frame；已知事件严格封闭。未知可选事件仅允许无载荷envelope，忽略且不推进游标；控制ping/pong不是JSON '
                            'frame。',
             'oneOf': [{'$ref': '#/components/schemas/WSHello'},
                       {'$ref': '#/components/schemas/WSChangesAvailable'},
                       {'$ref': '#/components/schemas/WSSessionRevoked'},
                       {'$ref': '#/components/schemas/WSUnknownOptional'}]}}


def main():
    document = yaml.safe_load((ROOT / 'openapi.yaml').read_text(encoding='utf-8'))
    validate_spec(document)
    schemas = document['components']['schemas']
    for schema in schemas.values():
        Draft202012Validator.check_schema(schema)
    cases = json.loads((ROOT / 'examples.json').read_text(encoding='utf-8'))
    failures = []
    operation_map = {op['operationId']: op for item in document['paths'].values()
                     for method, op in item.items() if method != 'parameters'}
    for case in cases:
        # 紧凑夹具展开后仍以真实字符串执行 Schema/UTF-8 字节校验。
        case = copy.deepcopy(case)
        for repeat in case.get('repeat', []):
            target = case['value']
            for segment in repeat['path'][:-1]:
                target = target[segment]
            target[repeat['path'][-1]] = repeat['text'] * repeat['count']
        # 仅解析内部绝对片段，不进行远端抓取；共享旧示例全部保留非回归。
        schema = {'$ref': '#/components/schemas/' + case['schema'],
                  'components': {'schemas': schemas}}
        validator = ContractValidator(schema, format_checker=FormatChecker())
        actual = validator.is_valid(case['value'])
        if actual != case.get('schema_valid', case['valid']):
            failures.append(case['name'])
        if case['schema'] == 'ContentRange':
            # 独立执行扩展，防止共享 pattern 掩盖完整格式识别的回归。
            standalone = ContractValidator({'type': 'string', 'x-content-range': True})
            if standalone.is_valid(case['value']) != case.get('schema_valid', case['valid']):
                failures.append(case['name'] + ' 独立 Content-Range 扩展')
        if 'range_size' in case:
            intervals = range_intervals(case['value'], case['range_size'])
            if intervals != case['intervals'] or (206 if intervals else 416) != case['status']:
                failures.append(case['name'] + ' 区间/状态')
        if 'bootstrap_state' in case and not bootstrap_state_valid(case['bootstrap_state']):
            failures.append(case['name'] + ' bootstrap状态声明')
        if ('ws_state' in case
                and ws_state_valid(case['ws_state']) != case.get('state_valid', True)):
            failures.append(case['name'] + ' WS状态声明')
        if ('device_session_state' in case
                and device_session_state_valid(case['device_session_state']) != case.get('state_valid', True)):
            failures.append(case['name'] + ' 设备会话状态声明')
        if 'sync_state' in case and not sync_state_valid(case['sync_state']):
            failures.append(case['name'] + ' 同步状态声明')
        if 'declaration' in case:
            declaration = case['declaration']
            allowed = declaration['security'] in operation_map[declaration['operationId']]['security']
            if allowed != case['valid']:
                failures.append(case['name'] + ' 权限声明')
        if 'operationId' in case:
            response = resolve(document, operation_map[case['operationId']]['responses'][str(case['status'])])
            response_schema = (response['headers'][case['header']]['schema'] if 'header' in case
                               else response['content']['application/json']['schema'])
            wrapped = {**response_schema, 'components': {'schemas': schemas}}
            response_valid = ContractValidator(wrapped, format_checker=FormatChecker()).is_valid(case['value'])
            code_rules = operation_map[case['operationId']].get('x-error-codes', {}).get(str(case['status']))
            if code_rules and 'error' in case['value']:
                response_valid = response_valid and case['value']['error'].get('code') in code_rules
            if response_valid != case['valid']:
                failures.append(case['name'] + ' 操作响应')
    if failures:
        raise SystemExit('示例校验与预期不符：' + '、'.join(failures))

    new_security = {
        'createBotMessage': ['BotAPIToken'], 'editBotMessage': ['BotAPIToken'],
        'createWebhookMessage': ['WebhookKey'],
        'createClientMessage': ['AdminBearer'], 'createCallback': ['AdminBearer'],
        'listClientMessages': ['AdminBearer'], 'getClientMessage': ['AdminBearer'],
        'syncClientChanges': ['AdminBearer'], 'updateClientRead': ['AdminBearer'],
        'bootstrapClient': ['AdminBearer'], 'connectClientWS': ['AdminBearer'],
        'uploadAttachment': ['AdminBearer', 'BotAPIToken'],
        'downloadAttachment': ['AdminBearer', 'BotAPIToken'],
    }
    delivery_ids = {'claimEvents', 'ackDelivery', 'renewDelivery', 'listDeliveries', 'retryDelivery'}
    new_security.update({'claimEvents': ['BotAPIToken'], 'ackDelivery': ['BotAPIToken'],
                         'renewDelivery': ['BotAPIToken'], 'listDeliveries': ['AdminBearer', 'WebCookie']})
    idempotent = {'createBotMessage', 'editBotMessage', 'createWebhookMessage',
                  'createClientMessage', 'createCallback', 'uploadAttachment', 'retryDelivery'}
    reference_ops = set(new_security) - {'listClientMessages', 'getClientMessage',
                                         'syncClientChanges', 'updateClientRead', 'bootstrapClient', 'connectClientWS'} - delivery_ids
    for path, item in document['paths'].items():
        for method, operation in item.items():
            if method == 'parameters':
                continue
            oid = operation['operationId']
            if oid == 'login':
                expected = []
            elif oid in new_security:
                expected = [{scheme: []} for scheme in new_security[oid]]
            elif method == 'get':
                expected = [{'AdminBearer': []}, {'WebCookie': []}]
            else:
                expected = [{'AdminBearer': []}, {'WebCookie': [], 'CSRF': []}]
            if operation['security'] != expected:
                raise SystemExit(f'权限声明错误：{method} {path}')
            if oid in reference_ops and operation.get('x-reference-policy') != 'same-bot-authorized-ready':
                raise SystemExit('缺少跨 Bot 引用/附件授权约束：' + oid)
            if oid in idempotent:
                parameters = [resolve(document, p) for p in operation['parameters']]
                key = next((p for p in parameters if p['name'] == 'Idempotency-Key'), None)
                if not key or not key['required'] or key['schema'] != {'$ref': '#/components/schemas/ID'}:
                    raise SystemExit('缺少幂等键：' + oid)
                for status in ('409', '410'):
                    if status not in operation['responses']:
                        raise SystemExit('缺少幂等冲突/墓碑响应：' + oid)
            # 引用的公共错误响应也必须检查，不能仅检查内联响应。
            for response in operation['responses'].values():
                resolved = resolve(document, response)
                if resolved['headers']['Cache-Control']['schema'] != {'type': 'string', 'const': 'no-store'}:
                    raise SystemExit(f'响应缺少 no-store：{method} {path}')
            body = operation.get('requestBody', {})
            if oid in new_security and 'application/json' in body.get('content', {}):
                if body.get('x-max-bytes') != 262144:
                    raise SystemExit('JSON 大小限制缺失：' + oid)
    baseline_ids = {'login', 'getSession', 'logout', 'listBots', 'createBot',
                    'patchBot', 'deleteBot', 'resetAPIToken', 'resetWebhookKey'}
    # 固定新增设备事务声明，确保权限、响应、Schema和代次策略不能被静默削弱。
    for path, method, oid, statuses in (
            ('/devices', 'get', 'listDeviceSessions', {'200', '400', '401', '403', '429', '503'}),
            ('/devices/{id}/session', 'delete', 'revokeDeviceSession',
             {'204', '400', '401', '403', '404', '429', '503'})):
        operation = document['paths'][path][method]
        if (operation['operationId'] != oid or set(operation['responses']) != statuses
                or operation.get('x-device-session-policy') != DEVICE_POLICY
                or 'requestBody' in operation):
            raise SystemExit('设备会话冻结声明不符：' + path)
    listing = operation_map['listDeviceSessions']
    revoke = operation_map['revokeDeviceSession']
    if listing['responses']['200']['content']['application/json']['schema'] != {
            '$ref': '#/components/schemas/DeviceSessionPage'}:
        raise SystemExit('设备列表缺少安全元数据投影')
    if (len(revoke.get('parameters', [])) != 1
            or {key: revoke['parameters'][0].get(key) for key in ('name', 'in', 'required', 'schema')}
            != {'name': 'id', 'in': 'path', 'required': True,
                'schema': {'$ref': '#/components/schemas/ID'}}
            or 'content' in revoke['responses']['204']
            or revoke.get('x-error-codes') != {'401': ['AUTHENTICATION_REQUIRED'],
                                               '403': ['FORBIDDEN'], '404': ['RESOURCE_NOT_FOUND']}):
        raise SystemExit('设备撤销参数/响应/错误声明不符')
    if {key: value for key, value in schemas['DeviceSessionPage'].items()
            if key != 'description'} != DEVICE_PAGE:
        raise SystemExit('设备会话分页Schema不符')
    if set(operation_map) != baseline_ids | set(new_security) | delivery_ids | {'listDeviceSessions', 'revokeDeviceSession'}:
        raise SystemExit('冻结操作集合不符')
    if any(method in document['paths']['/client/messages/{id}'] for method in ('put', 'patch')):
        raise SystemExit('不得提供 Android 编辑接口')
    if operation_map['editBotMessage'].get('x-bot-capability') != 'send':
        raise SystemExit('编辑必须 Bot send')
    if 'expected_revision' not in schemas['EditMessageRequest']['required']:
        raise SystemExit('编辑缺少 CAS 版本')
    if schemas['MessageText'].get('x-utf8-max-bytes') != 65536 or schemas['CallbackData'].get('x-utf8-max-bytes') != 4096:
        raise SystemExit('UTF-8 字节限制缺失')
    for oid in ('createClientMessage', 'createCallback'):
        if operation_map[oid].get('x-target-bot-capability') != 'receive':
            raise SystemExit('客户端仅可向 receive Bot 写入')
    for oid in ('listClientMessages', 'getClientMessage', 'createClientMessage', 'createCallback'):
        if operation_map[oid].get('x-client-type') != 'android':
            raise SystemExit('客户端接口缺少 Android 会话限制')
    upload_body = operation_map['uploadAttachment']['requestBody']
    if upload_body.get('x-default-max-bytes') != {'image': 10485760, 'file': 52428800} or not upload_body.get('x-configurable-max-bytes'):
        raise SystemExit('附件默认/可配置字节限制缺失')
    # 投递权限、租约和管理投影是契约声明，不能以此冒充运行时事务验证。
    for oid in ('claimEvents', 'ackDelivery', 'renewDelivery'):
        if operation_map[oid].get('x-bot-capability') != 'receive':
            raise SystemExit('投递必须 Bot receive：' + oid)
        if operation_map[oid].get('x-lease-policy') != 'current-bot-consumer-credential-generation':
            raise SystemExit('租约缺少consumer/凭据代次约束：' + oid)
    if operation_map['claimEvents'].get('x-claim-policy') != 'atomic-due-enqueue-order-outside-wait':
        raise SystemExit('领取事务/等待边界缺失')
    for oid, schema_name in [('claimEvents', 'ClaimResponse'), ('ackDelivery', 'AckReceipt'),
                             ('renewDelivery', 'RenewReceipt'), ('listDeliveries', 'DeliveryMetadataPage'),
                             ('retryDelivery', 'RetryDeliveryReceipt')]:
        operation = operation_map[oid]
        response = resolve(document, operation['responses']['200'])
        if response['content']['application/json']['schema'] != {'$ref': '#/components/schemas/' + schema_name}:
            raise SystemExit('投递响应类型错误：' + oid)
    for oid in ('listDeliveries', 'retryDelivery'):
        if operation_map[oid].get('x-management-projection') != 'delivery-metadata-only':
            raise SystemExit('管理投影边界缺失：' + oid)
    if operation_map['retryDelivery'].get('x-idempotency-policy') != 'authorized-replay-before-state-check':
        raise SystemExit('手动重试必须先鉴权再恢复同键结果')
    metadata = schemas['DeliveryMetadata']
    allowed_metadata = {'delivery_id', 'event_id', 'bot_id', 'state', 'state_version', 'attempt',
                        'retry_generation', 'created_at', 'updated_at', 'lease_until',
                        'next_attempt_at', 'acked_at', 'error_code'}
    if set(metadata['properties']) != allowed_metadata or metadata.get('additionalProperties') is not False:
        raise SystemExit('管理元数据允许列表被改变')
    for oid, codes in {'ackDelivery': {'LEASE_CONFLICT', 'BOT_DISABLED'},
                       'renewDelivery': {'LEASE_CONFLICT', 'BOT_DISABLED', 'LEASE_LIMIT_EXCEEDED'},
                       'retryDelivery': {'DELIVERY_NOT_FAILED', 'IDEMPOTENCY_CONFLICT'}}.items():
        if set(operation_map[oid].get('x-error-codes', {}).get('409', [])) != codes:
            raise SystemExit('状态错误码声明错误：' + oid)
    if operation_map['claimEvents'].get('x-initial-lease-seconds') != 120:
        raise SystemExit('默认领取租约错误')
    if operation_map['renewDelivery'].get('x-max-total-lease-seconds') != 600:
        raise SystemExit('累计租约上限缺失')
    if operation_map['ackDelivery'].get('x-ack-replay') != 'authorized-same-successful-lease':
        raise SystemExit('ACK缺少同成功租约幂等重放约束')
    if schemas['ClaimedDelivery']['properties']['retry_generation'] != {'$ref': '#/components/schemas/Sequence'}:
        raise SystemExit('投递代次必须保持64位十进制字符串')
    wait = schemas['ClaimRequest']['properties']['wait_seconds']
    limit = schemas['ClaimRequest']['properties']['limit']
    if (wait.get('default'), wait.get('minimum'), wait.get('maximum')) != (25, 0, 30):
        raise SystemExit('Long Polling等待边界错误')
    if (limit.get('default'), limit.get('minimum'), limit.get('maximum')) != (20, 1, 100):
        raise SystemExit('领取批次必须有界')
    if schemas['RenewRequest']['properties']['extend_seconds'].get('maximum') != 120:
        raise SystemExit('续约单次限制缺失')
    for oid in ('ackDelivery', 'renewDelivery', 'retryDelivery'):
        body = operation_map[oid]['requestBody']
        if body.get('x-max-bytes') != 262144:
            raise SystemExit('投递JSON大小限制缺失：' + oid)
    # 精确冻结游标/已读声明，避免只有文字却没有可破坏的检查。
    sync = operation_map['syncClientChanges']
    read = operation_map['updateClientRead']
    expected_sync = {'cursor_binding': ['administrator_id', 'epoch', 'high_watermark', 'position', 'cycle_complete'],
     'cursor_integrity': 'server-verified-opaque',
     'initial_position': '0',
     'initial_epoch': 'current',
     'high_watermark': 'fixed-per-cycle',
     'completed_cycle': 'capture-new-high-watermark',
     'order': 'ascending-change_seq',
     'page_limit': 200,
     'projection': 'same-read-transaction-current',
     'deleted_projection': 'body-free-tombstone',
     'gap': 'never-skip',
     'reset_status': 410,
     'reset_code': 'SYNC_RESET_REQUIRED',
     'invalid_cursor_status': 400,
     'invalid_cursor_code': 'INVALID_SYNC_CURSOR',
     'identity_mismatch_status': 400,
     'floor_comparison': 'position < floor',
     'epoch_comparison': 'cursor.epoch != current.epoch'}
    expected_read = {'input': 'actually-displayed-bot_seq',
     'upper_bound': 'bot-highest-ever-existing-bot_seq',
     'merge': 'max',
     'advance': 'same-transaction-read-version-and-shared-change',
     'unchanged': 'no-version-time-or-event-change',
     'unread': 'existing-application-with-bot_seq>read_seq',
     'exclude': ['administrator', 'callback', 'edit', 'expired'],
     'out_of_range_status': 422,
     'out_of_range_code': 'READ_SEQ_OUT_OF_RANGE'}
    if sync.get('x-sync-policy') != expected_sync or read.get('x-read-policy') != expected_read:
        raise SystemExit('同步/已读状态声明错误')
    for operation in (sync, read):
        if operation.get('x-client-type') != 'android':
            raise SystemExit('同步/已读仅允许Android管理员')
    if sync.get('x-error-codes') != {'400': ['INVALID_SYNC_CURSOR'], '410': ['SYNC_RESET_REQUIRED']}:
        raise SystemExit('同步错误码声明错误')
    if read.get('x-error-codes') != {'404': ['RESOURCE_NOT_FOUND'], '422': ['READ_SEQ_OUT_OF_RANGE']}:
        raise SystemExit('已读错误码声明错误')
    limit = next(p for p in sync['parameters'] if p['name'] == 'limit')
    if limit['schema'] != {'type': 'integer', 'minimum': 1, 'maximum': 200, 'default': 200}:
        raise SystemExit('同步有界页大小声明错误')
    if schemas['SyncPage'].get('x-sync-envelope') is not True:
        raise SystemExit('缺少同步批次语义扩展')
    if schemas['SyncPage']['properties']['changes'].get('maxItems') != 200:
        raise SystemExit('同步批次缺少上限')
    if schemas['UpdateReadRequest'] != {
            'type': 'object', 'properties': {'through_seq': {'$ref': '#/components/schemas/Sequence'}},
            'required': ['through_seq'], 'additionalProperties': False}:
        raise SystemExit('已读请求必须严格使用bot_seq序号')
    bootstrap = operation_map['bootstrapClient']
    expected_bootstrap = {'initial_snapshot': 'short-read-transaction-H-epoch-bots-shared-read',
     'cross_request_transaction': False,
     'order': ['bot_id', 'bot_seq'],
     'bot_order': 'ascending-UUID-text',
     'message_boundary': 'created-change_seq <= H',
     'position': 'last-scanned-bot_id-and-immutable-bot_seq',
     'projection': 'per-page-current-revision-deletion-and-attachment-existence-recheck',
     'deleted': 'omit-body-and-replay-tombstone-after-H',
     'missing_attachment': 'retain-message-reference-download-404-ATTACHMENT_MISSING',
     'snapshot': 'repeat-fixed-bots-and-read_states-on-every-page',
     'token_binding': ['administrator_id',
                       'epoch',
                       'high_watermark',
                       'position',
                       'snapshot_id',
                       'issued_at',
                       'expires_at',
                       'purpose',
                       'all_pages_complete'],
     'token_integrity': 'server-verified-opaque',
     'ttl_seconds': 900,
     'sliding_expiry': False,
     'repeat_page': 'same-position-recheck-current-projection-not-byte-replay',
     'invalid_token': {'status': 400, 'code': 'INVALID_BOOTSTRAP_TOKEN'},
     'restart': {'status': 410,
                 'code': 'BOOTSTRAP_RESET_REQUIRED',
                 'conditions': ['now >= expires_at', 'epoch != token.epoch', 'floor > H']},
     'initial_sync_cursor': 'completed-cycle-at-H-same-administrator-and-epoch',
     'cursor_provenance': {
         'protection': 'server-verified-integrity-bound-issuance-purpose-not-client-input',
         'seed': 'bootstrap-seed-rejected-by-verify-even-if-complete-at-H',
         'catchup': 'seed-starts-new-sync-cycle-capturing-current-high-watermark',
         'accepted': 'actual-sync-result-and-completed-cycle-at-or-after-H'},
     'verification': 'after-all-pages-and-completed-sync-cycle-at-or-after-H-recheck-epoch-floor',
     'verification_cursor_error': {'status': 400, 'code': 'INVALID_SYNC_CURSOR'},
     'generation': ['independent-staging',
                    'preserve-outbox-and-upload-tasks',
                    'merge-message-max-revision',
                    'merge-bot-and-read-by-version',
                    'apply-terminal-tombstones',
                    'finish-all-pages',
                    'catch-up-(H,current]',
                    'verify-epoch-floor',
                    'atomic-generation-switch-and-remove-old-server-cache'],
     'hints': 'never-advance-cursor',
     'page_limit': 200,
     'query_rules': {'page': 'no-sync_cursor; token-is-resume-purpose-or-absent',
                     'verify': 'require-final-page-receipt-token-and-completed-sync_cursor; '
                               'limit-ignored',
                     'invalid_query': '400-INVALID_BOOTSTRAP_TOKEN'},
     'token_purpose': {'next_token': 'resume-after-scanned-position',
                       'page_token': 'receipt-after-scanned-position-with-all_pages_complete-flag'}}
    if bootstrap.get('x-bootstrap-policy') != expected_bootstrap:
        raise SystemExit('bootstrap快照/令牌/恢复声明错误')
    if bootstrap.get('x-client-type') != 'android':
        raise SystemExit('bootstrap仅允许Android管理员')
    if bootstrap.get('x-error-codes') != {
            '400': ['INVALID_BOOTSTRAP_TOKEN', 'INVALID_SYNC_CURSOR'],
            '410': ['BOOTSTRAP_RESET_REQUIRED']}:
        raise SystemExit('bootstrap错误码声明错误')
    params = {item['name']: item for item in bootstrap['parameters']}
    if set(params) != {'phase', 'token', 'sync_cursor', 'limit'} or params['limit']['schema'] != {
            'type': 'integer', 'minimum': 1, 'maximum': 200, 'default': 200}:
        raise SystemExit('bootstrap参数与有界页大小错误')
    if params['phase']['schema'] != {'type': 'string', 'enum': ['page', 'verify'], 'default': 'page'}:
        raise SystemExit('bootstrap阶段声明错误')
    for key in ('token', 'sync_cursor'):
        if params[key]['schema'] != {'$ref': '#/components/schemas/OpaqueCursor'}:
            raise SystemExit('bootstrap令牌必须不透明：' + key)
    page = schemas['BootstrapPage']
    if page.get('x-bootstrap-envelope') is not True or page['properties']['messages'].get('maxItems') != 200:
        raise SystemExit('bootstrap缺少跨字段边界检查')
    if 'bootstrap-seed' not in page['properties']['sync_cursor'].get('description', ''):
        raise SystemExit('bootstrap种子签发用途说明缺失')
    if schemas['BootstrapVerified'].get('x-bootstrap-verified') is not True:
        raise SystemExit('bootstrap缺少最终复核边界检查')
    for name in ('BootstrapPage', 'BootstrapMessage', 'BootstrapVerified'):
        schema = schemas[name]
        if schema.get('additionalProperties') is not False or set(schema['required']) != set(schema['properties']):
            raise SystemExit('bootstrap对象必须严格且字段完整：' + name)
    for key in ('server_high_watermark', 'retention_floor'):
        if page['properties'][key] != {'$ref': '#/components/schemas/Sequence'}:
            raise SystemExit('bootstrap必须复用Sequence：' + key)
    if schemas['BootstrapMessage']['properties'] != {
            'created_change_seq': {'$ref': '#/components/schemas/Sequence'},
            'message': {'$ref': '#/components/schemas/Message'}}:
        raise SystemExit('bootstrap必须复用当前Message投影及创建序号')
    # WS frame独立于HTTP响应；精确声明只证明契约，不能替代真实连接验证。
    ws = operation_map['connectClientWS']
    if ws.get('x-ws-policy') != WS_POLICY or ws.get('x-client-type') != 'android':
        raise SystemExit('WS权限/恢复/窗口/心跳声明错误')
    if ws.get('x-ws-frame-schema') != {'$ref': '#/components/schemas/WSFrame'}:
        raise SystemExit('WS frame必须独立引用严格Schema')
    if 'content' in ws['responses']['101'] or 'requestBody' in ws:
        raise SystemExit('WS升级不得返回JSON正文或接收业务写入')
    if set(ws['responses']) != {'101', '400', '401', '403', '429', '503'}:
        raise SystemExit('WS握手错误分支缺失')
    if ws.get('parameters') != WS_PARAMETERS or ws.get('x-error-codes') != WS_ERRORS:
        raise SystemExit('WS握手参数/错误契约不符')
    for name, expected in WS_SCHEMAS.items():
        if schemas[name] != expected:
            raise SystemExit('WS frame Schema不符：' + name)
    schemes = document['components']['securitySchemes']
    for name in ('AdminBearer', 'BotAPIToken', 'WebhookKey'):
        if schemes[name]['type'] != 'http' or schemes[name]['scheme'] != 'bearer':
            raise SystemExit('Bearer scheme 声明错误：' + name)
    valid = sum(case['valid'] for case in cases)
    invalid = len(cases) - valid
    if not valid or not invalid:
        raise SystemExit('必须同时包含合法与非法示例')
    print(f'PASS: OpenAPI 3.1; {len(schemas)} schemas; '
          f'{valid} valid + {invalid} invalid examples; {len(operation_map)} operations/security')


if __name__ == '__main__':
    main()
