"""校验结构、示例及权限声明；字节/Range 扩展仅验证契约，不模拟 HTTP 服务。"""

import copy
from datetime import datetime
from zoneinfo import ZoneInfo, ZoneInfoNotFoundError
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


def notification_dnd(validator, enabled, instance, schema):
    """时区必须能解析；跨午夜保留方向，不能把相等时段误当全天。"""
    if not enabled or not isinstance(instance, dict):
        return
    if not isinstance(instance.get('timezone'), str):
        return
    try:
        ZoneInfo(instance.get('timezone', ''))
    except (ZoneInfoNotFoundError, ValueError):
        yield ValidationError('免打扰时区不是可用IANA时区')
    if instance.get('start') == instance.get('end'):
        yield ValidationError('免打扰起止不能相同')


def unique_bot_overrides(validator, enabled, instance, schema):
    if enabled and isinstance(instance, list):
        ids = [item['bot_id'] for item in instance
               if isinstance(item, dict) and isinstance(item.get('bot_id'), str)]
        if len(ids) != len(set(ids)):
            yield ValidationError('同一设备Bot开关不能重复')


def notification_in_dnd(config, instant):
    dnd = config['dnd']
    if not dnd['enabled']:
        return False
    local = datetime.fromisoformat(instant.replace('Z', '+00:00')).astimezone(ZoneInfo(dnd['timezone']))
    clock = local.strftime('%H:%M')
    start, end = dnd['start'], dnd['end']
    return start <= clock < end if start < end else clock >= start or clock < end


def notification_settings_state_valid(state, schemas):
    """合成配置事务/恢复模型：每步核对全状态，不是数据库或HTTP实现。"""
    data = copy.deepcopy(state['initial'])
    wrapper = {'$ref': '#/components/schemas/NotificationSettingsUpdate', 'components': {'schemas': schemas}}
    validator = ContractValidator(wrapper, format_checker=FormatChecker())
    for step in state['steps']:
        status, code, projection = 200, None, None
        session = data['sessions'].get(step['session_id'])
        if not session or not session['active']:
            status, code = 401, 'AUTHENTICATION_REQUIRED'
        elif step['action'] in ('get', 'put'):
            device = session['device_id']
            config = data['settings'][device]
            if step['action'] == 'get':
                projection = {'settings': copy.deepcopy(config), 'server_time': step['now']}
            else:
                request = step['request']
                provider = request.get('notify_provider')
                registration = session['registrations'].get(provider)
                if not validator.is_valid(request):
                    status, code = 422, 'VALIDATION_FAILED'
                elif request['expected_version'] != config['settings_version']:
                    status, code = 409, 'SETTINGS_CONFLICT'
                elif config['settings_version'] == 9223372036854775807:
                    status, code = 409, 'SETTINGS_VERSION_EXHAUSTED'
                elif any(item['bot_id'] not in data['authorized_bots'] for item in request['bot_overrides']):
                    status, code = 404, 'RESOURCE_NOT_FOUND'
                elif (request['notify_channel'] == 'vendor'
                      and (not registration or not registration['registered']
                           or provider not in data['verified_platforms'])):
                    status, code = 422, 'PROVIDER_NOT_READY'
                else:
                    updated = {k: copy.deepcopy(v) for k, v in request.items() if k != 'expected_version'}
                    updated.update(device_id=device, settings_version=config['settings_version'] + 1,
                                   updated_at=step['now'])
                    data['settings'][device] = updated
                    data['events'].append({'type': 'device.notifications.updated', 'settings': copy.deepcopy(updated)})
                    projection = {'settings': copy.deepcopy(updated), 'server_time': step['now']}
                if status == 409:
                    projection = {'latest_settings_version': config['settings_version'],
                                  'recovery_path': '/devices/current/notifications'}
        elif step['action'] == 'apply':
            # 跨设备事件不能污染当前投影；无论是否应用，原envelope游标规则不变。
            candidate = step['settings']
            local = data['local']
            if (candidate['device_id'] == session['device_id']
                    and candidate['settings_version'] > local['settings_version']):
                data['local'] = copy.deepcopy(candidate)
        elif step['action'] == 'revoke':
            session['active'] = False
        else:
            return False
        if {'status': status, 'code': code, 'projection': projection, 'data': data} != step['expected']:
            return False
    return True


def notification_job_state_valid(state):
    """合成租约和快照条件；平台结果不视为ACK，旧结果不修改新注册/配置。"""
    snapshot = state['snapshot']
    current = state['current']
    config = current['settings']
    now = datetime.fromisoformat(state['now'].replace('Z', '+00:00'))
    parse = lambda value: datetime.fromisoformat(value.replace('Z', '+00:00'))
    exact = (current['active'] and snapshot['session_id'] == current['session_id']
             and snapshot['device_id'] == current['device_id'] == config['device_id']
             and snapshot['provider'] == current['registration']['provider']
             and current['registration']['registered']
             and snapshot['registration_version'] == current['registration']['registration_version']
             and snapshot['settings_version'] == config['settings_version'])
    lease = (state['worker'] == current['lease_owner'] and now < parse(current['lease_expires_at'])
             and current['device_serial_owner'] == state['worker'])
    ready = exact and lease and now < parse(snapshot['expires_at'])
    if snapshot['purpose'] == 'sync':
        # none/vendor均允许FCM静默同步，不以提醒模式阻断数据补偿。
        ready = ready and snapshot['provider'] == 'fcm'
    else:
        enabled = config['enabled'] and next((x['enabled'] for x in config['bot_overrides']
                                               if x['bot_id'] == snapshot['bot_id']), True)
        ready = (ready and snapshot['alert_eligible'] and current['alert_eligible'] and current['target_exists']
                 and int(snapshot['bot_seq']) > int(current['read_seq']) and enabled
                 and config['notify_channel'] != 'none' and config['notify_provider'] == snapshot['provider']
                 and not notification_in_dnd(config, state['now']))
        if snapshot['provider'] == 'opush':
            ready = ready and current['platform_verified']
    after = copy.deepcopy(current)
    if state['action'] == 'before':
        ready = ready and snapshot == current['job_snapshot'] and current['job_status'] == 'pending'
        ready = ready and snapshot == current['job_snapshot'] and current['job_status'] == 'pending'
        outcome = 'send' if ready else 'drop'
        if ready:
            after['job_status'] = 'sending'
        if ready:
            after['job_status'] = 'sending'
        if (snapshot['purpose'] == 'alert' and notification_in_dnd(config, state['now'])):
            after['alert_eligible'] = False
    elif state['action'] == 'result':
        # 已读/DND/过期可能在外发后变化：仅同一快照/租约能记录归一化结果，不能补响。
        same_job = snapshot == current['job_snapshot'] and current['job_status'] == 'sending'
        if exact and lease and same_job:
            outcome = 'record'
            after['job_status'] = state['result']
            if state['result'] == 'unregistered' or (state['result'] == 'invalid_registration' and state['payload_valid']):
                if after['registration']['registration_version'] == 9223372036854775807:
                    outcome, after = 'exhausted', copy.deepcopy(current)
                else:
                    after['registration']['registration_version'] += 1
                    after['registration']['registered'] = False
        else:
            outcome = 'stale'
    elif state['action'] == 'merge':
        # 合并只推进同步水位；保留原job及资格、版本、稳定键与expiry。
        if exact and snapshot == current['job_snapshot']:
            outcome = 'merged'
            after['sync_target_watermark'] = str(max(int(current['sync_target_watermark']), int(state['new_watermark'])))
        else:
            outcome = 'stale'
    else:
        return False
    return {'outcome': outcome, 'current': after} == state['expected']

ContractValidator = validators.extend(Draft202012Validator, {
    'x-notification-dnd': notification_dnd,
    'x-unique-bot-overrides': unique_bot_overrides,
    'x-notification-dnd': notification_dnd,
    'x-unique-bot-overrides': unique_bot_overrides,
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


def push_registration_state_valid(state):
    """注册契约的合成事务模型；快照比较不能替代真实数据库/SDK并发测试。"""
    sessions = copy.deepcopy(state['sessions'])
    ceiling = 9223372036854775807
    for step in state['steps']:
        action = step['action']
        status, code, projection = None, None, None
        current = step.get('session_id')
        session = sessions.get(current)
        if action == 'login':
            # 同device_id允许多个Session；已撤销身份也不能复用。
            new = step['session']
            if new['id'] in sessions:
                return False
            sessions[new['id']] = copy.deepcopy(new)
        elif action == 'revoke':
            if session:
                session['active'] = False
                for registration in session['providers'].values():
                    registration['token'] = None
        elif action == 'callback' and step['source_session_id'] != current:
            # 这是客户端本地来源检查，source_session_id不是HTTP请求字段。
            status = 'discard'
        elif action == 'internal':
            # 内部失败分类已归一化，不接收厂商错误串；未知/网络失败没有解绑权。
            registration = (session['providers'][step['provider']] if session else None)
            exact = (session and session['active'] and registration['token'] is not None
                     and registration['version'] == step['snapshot_version'])
            invalid = (step['result'] == 'unregistered'
                       or (step['result'] == 'invalid_registration' and step['payload_valid']))
            if exact and invalid:
                if registration['version'] == ceiling:
                    code = 'REGISTRATION_VERSION_EXHAUSTED'
                else:
                    registration.update(version=registration['version'] + 1,
                                        token=None, updated_at=step['server_time'])
        elif not session or not session['active']:
            status, code = 401, 'AUTHENTICATION_REQUIRED'
        elif step.get('credential', 'AdminBearer') != 'AdminBearer':
            status, code = 403, 'FORBIDDEN'
        elif action == 'get':
            status = 200
            projection = {'session_id': current, 'registrations': [
                {'provider': provider, 'registration_version': record['version'],
                 'registered': record['token'] is not None, 'updated_at': record['updated_at']}
                for provider, record in session['providers'].items()]}
        elif action in ('put', 'callback', 'delete'):
            record = session['providers'][step['provider']]
            version, expected = record['version'], step['expected_version']
            replay = (action != 'delete' and record['token'] is not None
                      and step['registration_version'] == version and step['token'] == record['token'])
            # 重放保留原时间；删除后的墓碑不具备“同值重放”资格。
            if replay:
                status = 200
            elif expected != version or (action != 'delete' and step['registration_version'] <= version):
                status, code = 409, 'REGISTRATION_CONFLICT'
            elif action == 'delete' and record['token'] is None:
                status = 200
            elif version == ceiling:
                status, code = 409, 'REGISTRATION_VERSION_EXHAUSTED'
            else:
                record.update(version=version + 1 if action == 'delete' else step['registration_version'],
                              token=None if action == 'delete' else step['token'],
                              updated_at=step['server_time'])
                status = 200
        else:
            return False
        if ({'status': status, 'code': code, 'projection': projection, 'sessions': sessions}
                != step['expected']):
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


# 本卡冻结的事务约束和脱敏Schema，防止单端点删除或敏感字段回显。
PUSH_POLICY = {'identity': 'authenticated-current-android-Session.id-never-reused',
 'key': 'Session.id+provider-not-device_id',
 'transaction': 'reauthenticate-and-CAS-and-write-or-delete-and-server-time-atomic',
 'initial': 'absent-version-0-no-token-updated_at-null',
 'put': 'same-version-same-token-replay-before-CAS-otherwise-strictly-increase-and-match-barrier',
 'delete': 'match-barrier-increment-and-clear-token-retain-tombstone-unregistered-match-noop',
 'internal_result': 'unregistered-or-confirmed-valid-payload-invalid-registration-only-exact-active-session-provider-registration-version-increment-barrier',
 'unknown_result': 'preserve-registration-no-cross-provider-or-session-or-preference-change',
 'exhaustion': 'reject-mutation-no-wrap-no-partial-commit',
 'recovery': 'GET-current-push-no-token-all-providers-including-tombstones'}
PUSH_SCHEMAS = {'PushProvider': {'type': 'string', 'enum': ['fcm', 'opush']},
 'PushBarrierVersion': {'type': 'integer',
                        'minimum': 0,
                        'maximum': 9223372036854775807,
                        'description': '当前会话+provider持久版本屏障；0仅从未注册，删除不归零。'},
 'PushRegistrationVersion': {'type': 'integer', 'minimum': 1, 'maximum': 9223372036854775807},
 'PushRegistrationUpdate': {'type': 'object',
                            'properties': {'provider': {'$ref': '#/components/schemas/PushProvider'},
                                           'token': {'type': 'string',
                                                     'minLength': 1,
                                                     'maxLength': 4096,
                                                     'writeOnly': True,
                                                     'description': '不回显、不写日志；合成示例不是真实厂商凭据。'},
                                           'registration_version': {'$ref': '#/components/schemas/PushRegistrationVersion'},
                                           'expected_version': {'$ref': '#/components/schemas/PushBarrierVersion'}},
                            'required': ['provider',
                                         'token',
                                         'registration_version',
                                         'expected_version'],
                            'additionalProperties': False},
 'PushRegistrationDelete': {'type': 'object',
                            'properties': {'expected_version': {'$ref': '#/components/schemas/PushBarrierVersion'}},
                            'required': ['expected_version'],
                            'additionalProperties': False},
 'PushRegistration': {'type': 'object',
                      'properties': {'provider': {'$ref': '#/components/schemas/PushProvider'},
                                     'registration_version': {'$ref': '#/components/schemas/PushBarrierVersion'},
                                     'registered': {'type': 'boolean'},
                                     'updated_at': {'oneOf': [{'$ref': '#/components/schemas/UTCDateTime'},
                                                              {'type': 'null'}]}},
                      'required': ['provider', 'registration_version', 'registered', 'updated_at'],
                      'additionalProperties': False,
                      'allOf': [{'if': {'properties': {'registration_version': {'const': 0}}},
                                 'then': {'properties': {'registered': {'const': False},
                                                         'updated_at': {'type': 'null'}}},
                                 'else': {'properties': {'updated_at': {'$ref': '#/components/schemas/UTCDateTime'}}}}]},
 'PushRegistrationState': {'type': 'object',
                           'properties': {'session_id': {'$ref': '#/components/schemas/ID'},
                                          'registrations': {'type': 'array',
                                                            'minItems': 2,
                                                            'maxItems': 2,
                                                            'items': {'$ref': '#/components/schemas/PushRegistration'},
                                                            'allOf': [{'contains': {'properties': {'provider': {'const': 'fcm'}},
                                                                                    'required': ['provider']},
                                                                       'minContains': 1,
                                                                       'maxContains': 1},
                                                                      {'contains': {'properties': {'provider': {'const': 'opush'}},
                                                                                    'required': ['provider']},
                                                                       'minContains': 1,
                                                                       'maxContains': 1}]}},
                           'required': ['session_id', 'registrations'],
                           'additionalProperties': False}}



NOTIFICATION_POLICY = {'owner': 'authenticated-Session.device_id-shared-across-sessions-never-assume-unique-device-session',
 'registration': 'Session.id+provider-independent-never-activate-mode',
 'initial': 'virtual-version-0-enabled-fcm-preview-false-no-event-time-null',
 'transaction': 'reauthenticate-current-session-check-bots-platform-registration-CAS-device-config-version-time-and-sync-change-atomic',
 'put': 'full-replacement-match-expected-increment-even-same-value-no-client-time',
 'conflict': '409-latest_settings_version-recovery_path-GET-before-retry',
 'exhaustion': '409-no-wrap-no-config-time-event-or-registration-partial-commit',
 'vendor': 'explicit-opush-valid-current-session-registration-and-server-verified-platform-otherwise-422-no-fallback',
 'sync': 'typed-device.notifications.updated-device_id-filter-strictly-newer-version-GET-current-recovery-no-H-epoch-floor-seed-change'}
NOTIFICATION_JOB_POLICY = {'snapshot': 'session_id-device_id-purpose-target_watermark-provider-registration_version-settings_version-reminder_key-bot_id-bot_seq-expires_at-alert_eligible',
 'before': 'active-exact-session-device-registration-settings-and-mode-lease-device-serial-expiry-then-alert-enabled-bot-DND-unread-existing',
 'after': 'same-entire-snapshot-current-registration-config-active-session-and-live-lease-CAS-old-result-noop',
 'merge': 'sync-watermark-max-only-alert-eligibility-and-original-version-key-expiry-never-revive',
 'switch': 'future-new-events-only-stop-old-version-new-sends-isolate-inflight-no-cross-provider-timeout-retry',
 'dnd': 'drop-alert-eligibility-during-device-timezone-window-no-end-replay-sync-continues',
 'result': 'accepted-is-not-ACK-unknown-preserves-registration-no-cross-channel-resound',
 'vendor_text': {'title': 'RelayNest 有新消息', 'content': '打开 RelayNest 同步查看'},
 'fcm': 'data-only-hint-self-render-after-authoritative-sync-dedup-deployment-message-read-preview-DND-no-vendor-exemptions',
 'vendor_limit': 'only-vendor-accepted-offline-DND-read-delete-logout-switch-retraction-limit',
 'convergence': 'latest-confirmed-config-no-stale-fcm-resound-allowed-missed-alert-data-compensates'}
NOTIFICATION_SCHEMAS = {'NotificationSettingsVersion': {'type': 'integer',
                                 'minimum': 0,
                                 'maximum': 9223372036854775807,
                                 'description': '服务端设备配置版本；0仅虚拟默认，非客户端时间，首次CAS从0开始。'},
 'NotificationDND': {'type': 'object',
                     'properties': {'enabled': {'type': 'boolean'},
                                    'timezone': {'type': 'string',
                                                 'minLength': 1,
                                                 'maxLength': 128},
                                    'start': {'type': 'string',
                                              'pattern': '^(?:[01][0-9]|2[0-3]):[0-5][0-9]$'},
                                    'end': {'type': 'string',
                                            'pattern': '^(?:[01][0-9]|2[0-3]):[0-5][0-9]$'}},
                     'required': ['enabled', 'timezone', 'start', 'end'],
                     'additionalProperties': False,
                     'x-notification-dnd': True,
                     'description': 'IANA时区；start包含/end不包含，每日时段，start>end跨午夜，禁止相等。按该时区当时实际本地时间计算，包含DST重复时刻；缺失时刻自然无匹配。禁用仍保存有效配置。'},
 'NotificationBotOverride': {'type': 'object',
                             'properties': {'bot_id': {'$ref': '#/components/schemas/ID'},
                                            'enabled': {'type': 'boolean'}},
                             'required': ['bot_id', 'enabled'],
                             'additionalProperties': False},
 'NotificationSettingsUpdate': {'type': 'object',
                                'properties': {'expected_version': {'$ref': '#/components/schemas/NotificationSettingsVersion'},
                                               'enabled': {'type': 'boolean'},
                                               'bot_overrides': {'type': 'array',
                                                                 'maxItems': 1000,
                                                                 'items': {'$ref': '#/components/schemas/NotificationBotOverride'},
                                                                 'x-unique-bot-overrides': True,
                                                                 'description': '完整替换的设备局部Bot覆盖；未列出Bot默认启用，与设备总开关取AND。仅引用当前身份有权的未删除Bot，否则404。'},
                                               'dnd': {'$ref': '#/components/schemas/NotificationDND'},
                                               'preview': {'type': 'boolean'},
                                               'notify_channel': {'type': 'string',
                                                                  'enum': ['fcm',
                                                                           'vendor',
                                                                           'none']},
                                               'notify_provider': {'type': ['string', 'null'],
                                                                   'enum': ['fcm', 'opush', None]}},
                                'required': ['expected_version',
                                             'enabled',
                                             'bot_overrides',
                                             'dnd',
                                             'preview',
                                             'notify_channel',
                                             'notify_provider'],
                                'additionalProperties': False,
                                'oneOf': [{'properties': {'notify_channel': {'const': 'fcm'},
                                                          'notify_provider': {'const': 'fcm'}}},
                                          {'properties': {'notify_channel': {'const': 'vendor'},
                                                          'notify_provider': {'const': 'opush'}}},
                                          {'properties': {'notify_channel': {'const': 'none'},
                                                          'notify_provider': {'const': None}}}],
                                'description': '完整配置替换而非局部patch；不接受device_id/session_id/时间/版本或平台资格，设备归属从当前认证Session取得。'},
 'NotificationSettings': {'type': 'object',
                          'properties': {'device_id': {'$ref': '#/components/schemas/ID'},
                                         'settings_version': {'$ref': '#/components/schemas/NotificationSettingsVersion'},
                                         'updated_at': {'oneOf': [{'$ref': '#/components/schemas/UTCDateTime'},
                                                                  {'type': 'null'}]},
                                         'enabled': {'type': 'boolean'},
                                         'bot_overrides': {'type': 'array',
                                                           'maxItems': 1000,
                                                           'items': {'$ref': '#/components/schemas/NotificationBotOverride'},
                                                           'x-unique-bot-overrides': True,
                                                           'description': '完整替换的设备局部Bot覆盖；未列出Bot默认启用，与设备总开关取AND。仅引用当前身份有权的未删除Bot，否则404。'},
                                         'dnd': {'$ref': '#/components/schemas/NotificationDND'},
                                         'preview': {'type': 'boolean'},
                                         'notify_channel': {'type': 'string',
                                                            'enum': ['fcm', 'vendor', 'none']},
                                         'notify_provider': {'type': ['string', 'null'],
                                                             'enum': ['fcm', 'opush', None]}},
                          'required': ['device_id',
                                       'settings_version',
                                       'updated_at',
                                       'enabled',
                                       'bot_overrides',
                                       'dnd',
                                       'preview',
                                       'notify_channel',
                                       'notify_provider'],
                          'additionalProperties': False,
                          'oneOf': [{'properties': {'notify_channel': {'const': 'fcm'},
                                                    'notify_provider': {'const': 'fcm'}}},
                                    {'properties': {'notify_channel': {'const': 'vendor'},
                                                    'notify_provider': {'const': 'opush'}}},
                                    {'properties': {'notify_channel': {'const': 'none'},
                                                    'notify_provider': {'const': None}}}],
                          'allOf': [{'if': {'properties': {'settings_version': {'const': 0}}},
                                     'then': {'properties': {'updated_at': {'type': 'null'},
                                                             'enabled': {'const': True},
                                                             'bot_overrides': {'const': []},
                                                             'dnd': {'const': {'enabled': False,
                                                                               'timezone': 'UTC',
                                                                               'start': '22:00',
                                                                               'end': '07:00'}},
                                                             'preview': {'const': False},
                                                             'notify_channel': {'const': 'fcm'},
                                                             'notify_provider': {'const': 'fcm'}}},
                                     'else': {'properties': {'updated_at': {'$ref': '#/components/schemas/UTCDateTime'}}}}]},
 'NotificationSettingsState': {'type': 'object',
                               'properties': {'settings': {'$ref': '#/components/schemas/NotificationSettings'},
                                              'server_time': {'$ref': '#/components/schemas/UTCDateTime'}},
                               'required': ['settings', 'server_time'],
                               'additionalProperties': False},
 'NotificationSettingsConflict': {'type': 'object',
                                  'properties': {'error': {'type': 'object',
                                                           'properties': {'code': {'type': 'string',
                                                                                   'pattern': '^[A-Z][A-Z0-9_]*$',
                                                                                   'description': '稳定机器码；按 '
                                                                                                  'code '
                                                                                                  '分支，不匹配中文 '
                                                                                                  'message。后续卡可增加 '
                                                                                                  'code。',
                                                                                   'enum': ['SETTINGS_CONFLICT',
                                                                                            'SETTINGS_VERSION_EXHAUSTED']},
                                                                          'message': {'type': 'string'},
                                                                          'request_id': {'type': 'string',
                                                                                         'minLength': 1},
                                                                          'retryable': {'type': 'boolean'},
                                                                          'latest_settings_version': {'$ref': '#/components/schemas/NotificationSettingsVersion'},
                                                                          'recovery_path': {'const': '/devices/current/notifications'}},
                                                           'required': ['code',
                                                                        'message',
                                                                        'request_id',
                                                                        'retryable',
                                                                        'latest_settings_version',
                                                                        'recovery_path'],
                                                           'additionalProperties': False}},
                                  'required': ['error'],
                                  'additionalProperties': False},
 'SyncDeviceNotifications': {'type': 'object',
                             'properties': {'type': {'const': 'device.notifications.updated'},
                                            'change_seq': {'$ref': '#/components/schemas/Sequence'},
                                            'settings': {'allOf': [{'$ref': '#/components/schemas/NotificationSettings'},
                                                                   {'properties': {'settings_version': {'minimum': 1}}}]}},
                             'required': ['type', 'change_seq', 'settings'],
                             'additionalProperties': False,
                             'description': '仅认证设备device_id应用；其他设备事件忽略投影但仍按原批次提交游标，同设备仅较高settings_version替换。GET恢复先校验当前认证设备，再按版本应用，旧响应不得倒退；同device_id多Session共享偏好，不共享注册。'},
 'NotificationJobSnapshot': {'type': 'object',
                             'properties': {'session_id': {'$ref': '#/components/schemas/ID'},
                                            'device_id': {'$ref': '#/components/schemas/ID'},
                                            'purpose': {'enum': ['sync', 'alert']},
                                            'target_watermark': {'$ref': '#/components/schemas/Sequence'},
                                            'provider': {'$ref': '#/components/schemas/PushProvider'},
                                            'registration_version': {'$ref': '#/components/schemas/PushRegistrationVersion'},
                                            'settings_version': {'$ref': '#/components/schemas/NotificationSettingsVersion'},
                                            'reminder_key': {'type': 'object',
                                                             'properties': {'deployment_id': {'$ref': '#/components/schemas/ID'},
                                                                            'message_id': {'$ref': '#/components/schemas/ID'}},
                                                             'required': ['deployment_id',
                                                                          'message_id'],
                                                             'additionalProperties': False},
                                            'bot_id': {'$ref': '#/components/schemas/ID'},
                                            'bot_seq': {'$ref': '#/components/schemas/Sequence'},
                                            'expires_at': {'$ref': '#/components/schemas/UTCDateTime'},
                                            'alert_eligible': {'type': 'boolean'}},
                             'required': ['session_id',
                                          'device_id',
                                          'purpose',
                                          'target_watermark',
                                          'provider',
                                          'registration_version',
                                          'settings_version',
                                          'reminder_key',
                                          'bot_id',
                                          'bot_seq',
                                          'expires_at',
                                          'alert_eligible'],
                             'additionalProperties': False,
                             'description': '仅内部契约夹具，不是HTTP队列或厂商payload。sync也携带关联稳定键但不产生提醒；派生提醒须保留原事件资格/版本，不可由目标水位合并复活。',
                             'allOf': [{'if': {'properties': {'purpose': {'const': 'sync'}}},
                                        'then': {'properties': {'provider': {'const': 'fcm'}}}}]}}


# T8公开探针及管理投影固定声明；旧组件/操作继续走原校验。
HEALTH_SCHEMAS = {'HealthOK': {'type': 'string', 'const': 'ok\n'},
 'HealthUnavailable': {'type': 'string', 'const': 'unavailable\n'},
 'StatusCount': {'type': 'integer',
                 'minimum': 0,
                 'maximum': 9007199254740991,
                 'description': '汇总计数的JSON安全整数上限，不是队列容量限制；超出时不得返回失真值。'},
 'QueueStatus': {'type': 'object',
                 'properties': {'pending': {'$ref': '#/components/schemas/StatusCount'},
                                'leased': {'$ref': '#/components/schemas/StatusCount'},
                                'retry_wait': {'$ref': '#/components/schemas/StatusCount'},
                                'failed': {'$ref': '#/components/schemas/StatusCount'},
                                'oldest_waiting_at': {'oneOf': [{'$ref': '#/components/schemas/UTCDateTime'},
                                                                {'type': 'null'}]}},
                 'required': ['pending', 'leased', 'retry_wait', 'failed', 'oldest_waiting_at'],
                 'additionalProperties': False,
                 'allOf': [{'if': {'properties': {'pending': {'const': 0},
                                                  'retry_wait': {'const': 0}}},
                            'then': {'properties': {'oldest_waiting_at': {'type': 'null'}}},
                            'else': {'properties': {'oldest_waiting_at': {'$ref': '#/components/schemas/UTCDateTime'}}}}],
                 'description': '应用投递全局汇总，含停用Bot的保留队列；failed计失败投递，不计业务命令结果。最旧等待取pending/retry_wait最早入队时刻，未等待为null，leased不计等待。'},
 'ConnectionStatus': {'type': 'object',
                      'properties': {'ws_online_sessions': {'$ref': '#/components/schemas/StatusCount'},
                                     'recent_poll_bots': {'$ref': '#/components/schemas/StatusCount'},
                                     'unknown_bots': {'$ref': '#/components/schemas/StatusCount'},
                                     'webhook_state': {'const': 'unknown'}},
                      'required': ['ws_online_sessions',
                                   'recent_poll_bots',
                                   'unknown_bots',
                                   'webhook_state'],
                      'additionalProperties': False,
                      'description': 'WS计当前有效管理员会话连接；recent_poll/unknown按应用Bot汇总，遵循既有Bot.connection判定，不新增近期阈值。Webhook-only计入unknown，webhook_state永远unknown；WS在线不代表应用消费者或Webhook持续在线。'},
 'AttachmentRecoveryStatus': {'type': 'object',
                              'properties': {'anomaly_count': {'$ref': '#/components/schemas/StatusCount'},
                                             'last_error_code': {'enum': [None,
                                                                          'ATTACHMENT_RECOVERY_FAILED',
                                                                          'ATTACHMENT_STORAGE_UNAVAILABLE',
                                                                          'ATTACHMENT_OTHER']}},
                              'required': ['anomaly_count', 'last_error_code'],
                              'additionalProperties': False},
 'DatabaseStatus': {'type': 'object',
                    'properties': {'state': {'enum': ['available', 'unavailable', 'unknown']},
                                   'last_error_code': {'enum': [None,
                                                                'DB_UNAVAILABLE',
                                                                'DB_BUSY',
                                                                'DB_IO_ERROR',
                                                                'DB_OTHER']}},
                    'required': ['state', 'last_error_code'],
                    'additionalProperties': False},
 'PushStatus': {'type': 'object',
                'properties': {'state': {'enum': ['available', 'degraded', 'unknown']},
                               'last_error_code': {'enum': [None,
                                                            'PUSH_UNAVAILABLE',
                                                            'PUSH_REJECTED',
                                                            'PUSH_OTHER']}},
                'required': ['state', 'last_error_code'],
                'additionalProperties': False},
 'AdminSystemStatus': {'type': 'object',
                       'properties': {'observed_at': {'$ref': '#/components/schemas/UTCDateTime'},
                                      'queue': {'$ref': '#/components/schemas/QueueStatus'},
                                      'connections': {'$ref': '#/components/schemas/ConnectionStatus'},
                                      'attachment_recovery': {'$ref': '#/components/schemas/AttachmentRecoveryStatus'},
                                      'database': {'$ref': '#/components/schemas/DatabaseStatus'},
                                      'push': {'$ref': '#/components/schemas/PushStatus'}},
                       'required': ['observed_at',
                                    'queue',
                                    'connections',
                                    'attachment_recovery',
                                    'database',
                                    'push'],
                       'additionalProperties': False,
                       'description': '管理员只读系统元数据允许列表；不含消息正文、callback_data、附件URL/路径、原始异常、SQL、凭据或FCM注册。错误仅映射稳定分类码，无自由文本；null表示无可展示错误记录，unknown表示尚无观测。不是settings或配置版本投影。'}}
HEALTH_PATHS = {'/health/live': {'get': {'operationId': 'getHealthLive',
                          'summary': '公开根健康存活探针',
                          'description': '根路径，覆盖全局/api/v1；已存在Go处理器。ready仅由数据库/挂载Probe决定；未接入、失败、超时或停止为503，FCM不可用仅降级。此声明不证明存储Probe已接入。',
                          'servers': [{'url': '/'}],
                          'security': [],
                          'x-health-policy': {'live': 'process-only',
                                              'ready': 'database-and-mount-probe',
                                              'unready': ['probe-absent',
                                                          'probe-failed',
                                                          'probe-timeout',
                                                          'stopping'],
                                              'push_failure': 'degraded-not-unready',
                                              'public_detail': 'fixed-text-only'},
                          'responses': {'200': {'description': '进程存活/已就绪',
                                                'headers': {'Cache-Control': {'schema': {'type': 'string',
                                                                                         'const': 'no-store'}},
                                                            'X-Content-Type-Options': {'schema': {'type': 'string',
                                                                                                  'const': 'nosniff'}}},
                                                'content': {'text/plain; charset=utf-8': {'schema': {'$ref': '#/components/schemas/HealthOK'},
                                                                                          'example': 'ok\n'}}}}}},
 '/health/ready': {'get': {'operationId': 'getHealthReady',
                           'summary': '公开根健康就绪探针',
                           'description': '根路径，覆盖全局/api/v1；已存在Go处理器。ready仅由数据库/挂载Probe决定；未接入、失败、超时或停止为503，FCM不可用仅降级。此声明不证明存储Probe已接入。',
                           'servers': [{'url': '/'}],
                           'security': [],
                           'x-health-policy': {'live': 'process-only',
                                               'ready': 'database-and-mount-probe',
                                               'unready': ['probe-absent',
                                                           'probe-failed',
                                                           'probe-timeout',
                                                           'stopping'],
                                               'push_failure': 'degraded-not-unready',
                                               'public_detail': 'fixed-text-only'},
                           'responses': {'200': {'description': '进程存活/已就绪',
                                                 'headers': {'Cache-Control': {'schema': {'type': 'string',
                                                                                          'const': 'no-store'}},
                                                             'X-Content-Type-Options': {'schema': {'type': 'string',
                                                                                                   'const': 'nosniff'}}},
                                                 'content': {'text/plain; charset=utf-8': {'schema': {'$ref': '#/components/schemas/HealthOK'},
                                                                                           'example': 'ok\n'}}},
                                         '503': {'description': '未就绪，不公开探针细节',
                                                 'headers': {'Cache-Control': {'schema': {'type': 'string',
                                                                                          'const': 'no-store'}},
                                                             'X-Content-Type-Options': {'schema': {'type': 'string',
                                                                                                   'const': 'nosniff'}}},
                                                 'content': {'text/plain; charset=utf-8': {'schema': {'$ref': '#/components/schemas/HealthUnavailable'},
                                                                                           'example': 'unavailable\n'}}}}}},
 '/admin/status': {'get': {'operationId': 'getAdminStatus',
                           'summary': '管理员系统状态脱敏汇总',
                           'description': '仅唯一管理员Bearer或Web Cookie，Bot '
                                          'Token/Webhook/匿名拒绝；只读不需CSRF。只返回汇总允许列表，不提供正文或任意错误串。此路由为待实现契约；汇总读取失败返回受保护JSON '
                                          'Error 503，FCM降级仍可返回200。',
                           'security': [{'AdminBearer': []}, {'WebCookie': []}],
                           'x-management-projection': 'system-status-metadata-only',
                           'responses': {'200': {'description': '脱敏状态汇总',
                                                 'headers': {'Cache-Control': {'schema': {'type': 'string',
                                                                                          'const': 'no-store'}}},
                                                 'content': {'application/json': {'schema': {'$ref': '#/components/schemas/AdminSystemStatus'}}}},
                                         '401': {'$ref': '#/components/responses/401'},
                                         '403': {'$ref': '#/components/responses/403'},
                                         '429': {'$ref': '#/components/responses/429'},
                                         '503': {'$ref': '#/components/responses/503'}}}}}


def health_state_valid(state):
    """合成声明模型，不调用Probe/HTTP；推送可用性不参与就绪判定。"""
    ready = 200 if state['probe'] == 'ok' and not state['stopping'] else 503
    return state['live_status'] == 200 and state['ready_status'] == ready


def health_response_valid(document, state):
    """验证固定公开响应信封；不将此夹具称为真实HTTP测试。"""
    path = {'getHealthLive': '/health/live', 'getHealthReady': '/health/ready'}.get(state['operationId'])
    if path is None:
        return False
    response = document['paths'][path]['get']['responses'].get(str(state['status']))
    if response is None or state['content_type'] != 'text/plain; charset=utf-8':
        return False
    expected_headers = {name: header['schema']['const'] for name, header in response['headers'].items()}
    return (state['headers'] == expected_headers
            and state['body'] == response['content'][state['content_type']]['example'])


def response_body_schema(path, method, operation, response, case):
    """媒体由固定操作决定；夹具不能换媒体或借公开操作绕过 JSON 校验。"""
    public_bodies = {
        ('/health/live', 'get', 'getHealthLive', '200'): 'HealthOK',
        ('/health/ready', 'get', 'getHealthReady', '200'): 'HealthOK',
        ('/health/ready', 'get', 'getHealthReady', '503'): 'HealthUnavailable',
    }
    public_schema = public_bodies.get((path, method, operation['operationId'], str(case['status'])))
    expected_media = 'text/plain; charset=utf-8' if public_schema else 'application/json'
    if (case.get('media_type', 'application/json') != expected_media
            or (public_schema and case['schema'] != public_schema)):
        raise ValueError('正文夹具媒体/公开Schema错误')
    if expected_media not in response.get('content', {}):
        raise ValueError('正文响应媒体缺失')
    return response['content'][expected_media]['schema']



def global_settings_state_valid(state, schemas):
    """仅核对合成契约声明；未执行HTTP、数据库事务或真实清理。"""
    def matches(name, value):
        schema = {'$ref': '#/components/schemas/' + name,
                  'components': {'schemas': schemas}}
        return ContractValidator(schema, format_checker=FormatChecker()).is_valid(value)

    if state['kind'] == 'cas':
        current = copy.deepcopy(state['before'])
        if not matches('GlobalSettings', current):
            return False
        events = []
        results = []
        for step in state['steps']:
            request = step['request']
            auth = step.get('auth', 'bearer')
            if auth == 'anonymous':
                status, code = 401, 'AUTHENTICATION_REQUIRED'
            elif auth not in ('bearer', 'cookie') or (auth == 'cookie' and not (
                    step.get('csrf') and step.get('same_origin'))):
                status, code = 403, 'FORBIDDEN'
            elif (not isinstance(request, dict)
                  or set(request) != {'retention_days', 'image_max_bytes',
                                      'file_max_bytes', 'expected_version'}
                  or any(not isinstance(request[key], str) for key in (
                      'retention_days', 'image_max_bytes', 'file_max_bytes'))
                  or isinstance(request['expected_version'], bool)
                  or not isinstance(request['expected_version'], (int, float))):
                status, code = 400, 'INVALID_REQUEST'
            elif not matches('GlobalSettingsUpdate', request):
                status, code = 422, 'VALIDATION_FAILED'
            elif request['expected_version'] != current['version']:
                status, code = 409, 'GLOBAL_SETTINGS_CONFLICT'
            elif current['version'] == 9007199254740991:
                status, code = 409, 'GLOBAL_SETTINGS_VERSION_EXHAUSTED'
            else:
                if not matches('UTCDateTime', step['server_time']):
                    return False
                current = {key: request[key] for key in (
                    'retention_days', 'image_max_bytes', 'file_max_bytes')}
                current.update(version=request['expected_version'] + 1,
                               updated_at=step['server_time'])
                events.append({'type': 'settings.updated',
                               'settings_version': current['version'],
                               'recovery_path': '/admin/settings'})
                status, code = 200, None
            result = {'status': status, 'code': code}
            if status == 409:
                result.update(latest_version=current['version'], retryable=False,
                              recovery_path='/admin/settings')
            results.append(result)
        return (results == state['results'] and current == state['after']
                and events == state['events'])
    if state['kind'] == 'cleanup':
        if not matches('GlobalSettingsValue', state['retention_days']):
            return False
        if not all(matches('UTCDateTime', state[key]) for key in ('now', 'created_at')):
            return False
        # timedelta.days 是差值的整日部分，无巨量天数乘法或远古cutoff。
        age = (datetime.fromisoformat(state['now'])
               - datetime.fromisoformat(state['created_at'])).days
        protected = (state['delivery'] in ('pending', 'leased', 'retry_wait', 'failed')
                     or state['unacknowledged'] or state['necessary_parent'])
        delete_message = age >= int(state['retention_days']) and not protected
        delete_attachment = (delete_message and not state['other_attachment_reference']
                             and not state['active_download'])
        return (state['delete_message'] == delete_message
                and state['delete_attachment'] == delete_attachment)
    if state['kind'] == 'sync':
        if not matches('SyncGlobalSettings', state['event']):
            return False
        incoming = state['event']['settings_version']
        if not matches('GlobalSettingsVersion', state['local_version']):
            return False
        # 事件无管理正文；重读始终经过管理权限，且异步GET不得倒退。
        fetch = incoming > state['local_version']
        body_allowed = state['auth'] in ('bearer', 'cookie')
        apply = (fetch and body_allowed and matches('GlobalSettings', state['response'])
                 and state['response']['version'] >= incoming
                 and state['response']['version'] > state['local_version'])
        return (state['fetch'] == fetch and state['body_allowed'] == body_allowed
                and state['apply'] == apply and state['cursor_committed'] is True)
    return False


GLOBAL_SETTINGS_SCHEMAS = {'GlobalSettingsValue': {'allOf': [{'$ref': '#/components/schemas/Sequence'},
                                   {'not': {'const': '0'}}],
                         'description': '正 int64 '
                                        '规范十进制字符串，1～9223372036854775807；禁止0、前导零、符号、小数、指数和JSON数字。仅为表示/存储技术上限，无额外业务上限。'},
 'GlobalSettingsVersion': {'allOf': [{'$ref': '#/components/schemas/Version'},
                                     {'maximum': 9007199254740991}],
                           'description': '全局设置专用JSON整数版本，1～9007199254740991；不改变公共或设备Version。'},
 'GlobalSettings': {'type': 'object',
                    'properties': {'retention_days': {'$ref': '#/components/schemas/GlobalSettingsValue',
                                                      'default': '30'},
                                   'image_max_bytes': {'$ref': '#/components/schemas/GlobalSettingsValue',
                                                       'default': '10485760'},
                                   'file_max_bytes': {'$ref': '#/components/schemas/GlobalSettingsValue',
                                                      'default': '52428800'},
                                   'version': {'$ref': '#/components/schemas/GlobalSettingsVersion'},
                                   'updated_at': {'$ref': '#/components/schemas/UTCDateTime'}},
                    'required': ['retention_days',
                                 'image_max_bytes',
                                 'file_max_bytes',
                                 'version',
                                 'updated_at'],
                    'additionalProperties': False,
                    'description': '完整全局设置；初始持久化版本1，updated_at由服务端生成。默认30天、10MiB、50MiB均可调。'},
 'GlobalSettingsUpdate': {'type': 'object',
                          'properties': {'retention_days': {'$ref': '#/components/schemas/GlobalSettingsValue',
                                                            'default': '30'},
                                         'image_max_bytes': {'$ref': '#/components/schemas/GlobalSettingsValue',
                                                             'default': '10485760'},
                                         'file_max_bytes': {'$ref': '#/components/schemas/GlobalSettingsValue',
                                                            'default': '52428800'},
                                         'expected_version': {'$ref': '#/components/schemas/GlobalSettingsVersion'}},
                          'required': ['retention_days',
                                       'image_max_bytes',
                                       'file_max_bytes',
                                       'expected_version'],
                          'additionalProperties': False},
 'GlobalSettingsConflict': {'oneOf': [{'type': 'object',
                                       'properties': {'error': {'type': 'object',
                                                                'properties': {'code': {'const': 'GLOBAL_SETTINGS_CONFLICT'},
                                                                               'message': {'type': 'string'},
                                                                               'request_id': {'type': 'string',
                                                                                              'minLength': 1},
                                                                               'retryable': {'const': False},
                                                                               'latest_version': {'$ref': '#/components/schemas/GlobalSettingsVersion'},
                                                                               'recovery_path': {'const': '/admin/settings'}},
                                                                'required': ['code',
                                                                             'message',
                                                                             'request_id',
                                                                             'retryable',
                                                                             'latest_version',
                                                                             'recovery_path'],
                                                                'additionalProperties': False}},
                                       'required': ['error'],
                                       'additionalProperties': False},
                                      {'type': 'object',
                                       'properties': {'error': {'type': 'object',
                                                                'properties': {'code': {'const': 'GLOBAL_SETTINGS_VERSION_EXHAUSTED'},
                                                                               'message': {'type': 'string'},
                                                                               'request_id': {'type': 'string',
                                                                                              'minLength': 1},
                                                                               'retryable': {'const': False},
                                                                               'latest_version': {'allOf': [{'$ref': '#/components/schemas/GlobalSettingsVersion'}, {'const': 9007199254740991}]},
                                                                               'recovery_path': {'const': '/admin/settings'}},
                                                                'required': ['code',
                                                                             'message',
                                                                             'request_id',
                                                                             'retryable',
                                                                             'latest_version',
                                                                             'recovery_path'],
                                                                'additionalProperties': False}},
                                       'required': ['error'],
                                       'additionalProperties': False}],
                            'description': '先CAS再耗尽：过期即CONFLICT；匹配且达到上限为VERSION_EXHAUSTED。重读最新配置；禁止盲重放。耗尽不可重试，GET仍可用，恢复写入需未来明确迁移。'},
 'SyncGlobalSettings': {'type': 'object',
                        'properties': {'type': {'const': 'settings.updated'},
                                       'change_seq': {'$ref': '#/components/schemas/Sequence'},
                                       'settings_version': {'$ref': '#/components/schemas/GlobalSettingsVersion'},
                                       'recovery_path': {'const': '/admin/settings'}},
                        'required': ['type', 'change_seq', 'settings_version', 'recovery_path'],
                        'additionalProperties': False,
                        'description': '仅传全局配置版本及管理GET关联，不携带配置正文；正文仍需管理员权限。仅更高版本触发重读，重复/旧版本忽略投影但正常提交同步游标；GET旧响应不得覆盖较新版本。与device.notifications.updated独立。'}}
GLOBAL_SETTINGS_PATH = {'get': {'operationId': 'getGlobalSettings',
         'summary': '读取全局设置',
         'tags': ['Admin'],
         'security': [{'AdminBearer': []}, {'WebCookie': []}],
         'description': '仅唯一管理员。Cookie写入须CSRF及同源Origin/Referer。三配置为正int64规范十进制字符串，0不表示立即清理、禁用或无限；不新增业务上限。缩短期限只影响后续清理批次；保护未完成/未ACK/failed及必要附件、父消息和有效引用，不承诺永久恢复。',
         'responses': {'200': {'description': '完整配置、版本及服务端时间。',
                               'headers': {'Cache-Control': {'schema': {'type': 'string',
                                                                        'const': 'no-store'}}},
                               'content': {'application/json': {'schema': {'$ref': '#/components/schemas/GlobalSettings'}}}},
                       '400': {'$ref': '#/components/responses/400'},
                       '401': {'$ref': '#/components/responses/401'},
                       '403': {'$ref': '#/components/responses/403'},
                       '429': {'$ref': '#/components/responses/429'},
                       '503': {'$ref': '#/components/responses/503'}},
         'x-global-settings-policy': {'validation_order': ['authentication',
                                                           'structure-400',
                                                           'field-semantics-422',
                                                           'expected-version-CAS',
                                                           'version-exhaustion',
                                                           'atomic-commit'],
                                      'cookie_write': 'csrf-and-same-origin-origin-or-referer',
                                      'commit': 'all-values-version-updated_at-and-one-settings.updated-in-one-transaction',
                                      'same_values': 'increment-version-and-emit-one-change',
                                      'conflict': '409-GLOBAL_SETTINGS_CONFLICT-latest-version-and-reread',
                                      'exhaustion': '409-GLOBAL_SETTINGS_VERSION_EXHAUSTED-retryable-false-no-write-no-event-no-wrap',
                                      'recovery': 'GET-admin-settings-then-reapply-intent-no-blind-replay'},
         'x-cleanup-policy': {'age': 'UTC-whole-days-since-created_at-safe-day-second-decomposition-no-duration-multiplication-no-cutoff-subtraction-future-never-expired',
                              'schedule': 'shortening-affects-subsequent-hourly-small-batches-no-immediate-completion',
                              'transaction': 'recheck-protection-in-delete-transaction',
                              'protect': ['pending',
                                          'leased',
                                          'retry_wait',
                                          'failed',
                                          'unacknowledged',
                                          'necessary-parent-message-and-attachment',
                                          'other-message-attachment-reference',
                                          'active-download-reference'],
                              'ordinary_reference': 'does-not-permanently-protect-parent-message',
                              'no_ttl': 'protected-delivery-or-failure-never-silently-dropped-under-pressure',
                              'history': 'offline-or-unread-does-not-protect-history-no-permanent-recovery-guarantee'}},
 'put': {'operationId': 'updateGlobalSettings',
         'summary': 'CAS替换全局设置',
         'tags': ['Admin'],
         'security': [{'AdminBearer': []}, {'WebCookie': [], 'CSRF': []}],
         'description': '仅唯一管理员。Cookie写入须CSRF及同源Origin/Referer。三配置为正int64规范十进制字符串，0不表示立即清理、禁用或无限；不新增业务上限。缩短期限只影响后续清理批次；保护未完成/未ACK/failed及必要附件、父消息和有效引用，不承诺永久恢复。',
         'responses': {'200': {'description': '完整配置、版本及服务端时间。',
                               'headers': {'Cache-Control': {'schema': {'type': 'string',
                                                                        'const': 'no-store'}}},
                               'content': {'application/json': {'schema': {'$ref': '#/components/schemas/GlobalSettings'}}}},
                       '400': {'$ref': '#/components/responses/400'},
                       '401': {'$ref': '#/components/responses/401'},
                       '403': {'$ref': '#/components/responses/403'},
                       '429': {'$ref': '#/components/responses/429'},
                       '503': {'$ref': '#/components/responses/503'},
                       '413': {'$ref': '#/components/responses/413'},
                       '422': {'$ref': '#/components/responses/422'},
                       '409': {'description': '过期CAS优先；匹配但耗尽不可重试，无部分写入或事件。',
                               'headers': {'Cache-Control': {'schema': {'type': 'string',
                                                                        'const': 'no-store'}}},
                               'content': {'application/json': {'schema': {'$ref': '#/components/schemas/GlobalSettingsConflict'}}}}},
         'x-global-settings-policy': {'validation_order': ['authentication',
                                                           'structure-400',
                                                           'field-semantics-422',
                                                           'expected-version-CAS',
                                                           'version-exhaustion',
                                                           'atomic-commit'],
                                      'cookie_write': 'csrf-and-same-origin-origin-or-referer',
                                      'commit': 'all-values-version-updated_at-and-one-settings.updated-in-one-transaction',
                                      'same_values': 'increment-version-and-emit-one-change',
                                      'conflict': '409-GLOBAL_SETTINGS_CONFLICT-latest-version-and-reread',
                                      'exhaustion': '409-GLOBAL_SETTINGS_VERSION_EXHAUSTED-retryable-false-no-write-no-event-no-wrap',
                                      'recovery': 'GET-admin-settings-then-reapply-intent-no-blind-replay'},
         'x-cleanup-policy': {'age': 'UTC-whole-days-since-created_at-safe-day-second-decomposition-no-duration-multiplication-no-cutoff-subtraction-future-never-expired',
                              'schedule': 'shortening-affects-subsequent-hourly-small-batches-no-immediate-completion',
                              'transaction': 'recheck-protection-in-delete-transaction',
                              'protect': ['pending',
                                          'leased',
                                          'retry_wait',
                                          'failed',
                                          'unacknowledged',
                                          'necessary-parent-message-and-attachment',
                                          'other-message-attachment-reference',
                                          'active-download-reference'],
                              'ordinary_reference': 'does-not-permanently-protect-parent-message',
                              'no_ttl': 'protected-delivery-or-failure-never-silently-dropped-under-pressure',
                              'history': 'offline-or-unread-does-not-protect-history-no-permanent-recovery-guarantee'},
         'requestBody': {'required': True,
                         'x-max-bytes': 262144,
                         'content': {'application/json': {'schema': {'$ref': '#/components/schemas/GlobalSettingsUpdate'}}}},
         'x-error-codes': {'400': ['INVALID_REQUEST'],
                           '401': ['AUTHENTICATION_REQUIRED'],
                           '403': ['FORBIDDEN'],
                           '409': ['GLOBAL_SETTINGS_CONFLICT', 'GLOBAL_SETTINGS_VERSION_EXHAUSTED'],
                           '413': ['REQUEST_TOO_LARGE'],
                           '422': ['VALIDATION_FAILED']}}}

def main():
    document = yaml.safe_load((ROOT / 'openapi.yaml').read_text(encoding='utf-8'))
    validate_spec(document)
    schemas = document['components']['schemas']
    for schema in schemas.values():
        Draft202012Validator.check_schema(schema)
    # health根servers/公开权限/纯文本及管理JSON分别固定，不能扩大公开例外。
    for path, expected in HEALTH_PATHS.items():
        if document['paths'].get(path) != expected:
            raise SystemExit('健康/管理操作声明错误：' + path)
    if document['servers'] != [{'url': '/api/v1'}]:
        raise SystemExit('管理API前缀不得被公开根探针改写')
    for name, expected in HEALTH_SCHEMAS.items():
        if schemas.get(name) != expected:
            raise SystemExit('健康/管理Schema错误：' + name)
    if document['paths'].get('/admin/settings') != GLOBAL_SETTINGS_PATH:
        raise SystemExit('全局配置操作声明错误')
    for name, expected in GLOBAL_SETTINGS_SCHEMAS.items():
        if schemas.get(name) != expected:
            raise SystemExit('全局配置Schema错误：' + name)
    cases = json.loads((ROOT / 'examples.json').read_text(encoding='utf-8'))
    failures = []
    operation_map = {op['operationId']: op for item in document['paths'].values()
                     for method, op in item.items() if method != 'parameters'}
    operation_locations = {op['operationId']: (path, method)
                           for path, item in document['paths'].items()
                           for method, op in item.items() if method != 'parameters'}
    # JSON 正文不得增加另一媒体供夹具改选；附件流及无正文响应仍走原路径。
    for operation in operation_map.values():
        for declared_response in operation['responses'].values():
            content = resolve(document, declared_response).get('content', {})
            if 'application/json' in content and set(content) != {'application/json'}:
                raise SystemExit('JSON响应媒体声明错误：' + operation['operationId'])
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
        if ('global_settings_state' in case
                and global_settings_state_valid(case['global_settings_state'], schemas)
                != case.get('state_valid', True)):
            failures.append(case['name'] + ' 全局配置合成状态')
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
        if ('notification_settings_state' in case
                and notification_settings_state_valid(case['notification_settings_state'], schemas) != case.get('state_valid', True)):
            failures.append(case['name'] + ' 通知配置状态')
        if ('notification_job_state' in case
                and notification_job_state_valid(case['notification_job_state']) != case.get('state_valid', True)):
            failures.append(case['name'] + ' 通知发送状态')
        if ('push_registration_state' in case
                and push_registration_state_valid(case['push_registration_state']) != case.get('state_valid', True)):
            failures.append(case['name'] + ' 注册事务状态声明')
        if 'sync_state' in case and not sync_state_valid(case['sync_state']):
            failures.append(case['name'] + ' 同步状态声明')
        if ('health_state' in case
                and health_state_valid(case['health_state']) != case.get('state_valid', True)):
            failures.append(case['name'] + ' 健康状态声明')
        if ('health_response' in case
                and health_response_valid(document, case['health_response']) != case['valid']):
            failures.append(case['name'] + ' 公开纯文本信封')
        if 'declaration' in case:
            declaration = case['declaration']
            declared_security = operation_map[declaration['operationId']]['security']
            allowed = (declaration['security'] in declared_security
                       or (declaration['operationId'] in ('getHealthLive', 'getHealthReady')
                           and declared_security == [] and declaration['security'] == {}))
            if allowed != case['valid']:
                failures.append(case['name'] + ' 权限声明')
        if 'operationId' in case:
            declared_response = operation_map[case['operationId']]['responses'].get(str(case['status']))
            if declared_response is None:
                failures.append(case['name'] + ' 操作响应状态缺失')
                continue
            response = resolve(document, declared_response)
            if 'header' in case:
                response_schema = response['headers'][case['header']]['schema']
            else:
                path, method = operation_locations[case['operationId']]
                try:
                    response_schema = response_body_schema(
                        path, method, operation_map[case['operationId']], response, case)
                except ValueError as error:
                    failures.append(case['name'] + ' ' + str(error))
                    continue
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
        'getCurrentNotifications': ['AdminBearer'], 'updateCurrentNotifications': ['AdminBearer'],
        'getCurrentPush': ['AdminBearer'], 'updateCurrentPush': ['AdminBearer'],
        'deleteCurrentPush': ['AdminBearer'],
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
                                         'syncClientChanges', 'updateClientRead', 'bootstrapClient', 'connectClientWS', 'getCurrentPush', 'updateCurrentPush', 'deleteCurrentPush', 'getCurrentNotifications', 'updateCurrentNotifications'} - delivery_ids
    for path, item in document['paths'].items():
        for method, operation in item.items():
            if method == 'parameters':
                continue
            oid = operation['operationId']
            if oid in ('login', 'getHealthLive', 'getHealthReady'):
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
    # 注册事务、恢复投影与删除屏障是一个整体，不能按端点拆散或削弱。
    for name, expected in PUSH_SCHEMAS.items():
        if schemas.get(name) != expected:
            raise SystemExit('注册Schema冻结声明不符：' + name)
    for path, method, oid, body in (
            ('/devices/current/push', 'get', 'getCurrentPush', None),
            ('/devices/current/push', 'put', 'updateCurrentPush', 'PushRegistrationUpdate'),
            ('/devices/current/push/{provider}', 'delete', 'deleteCurrentPush', 'PushRegistrationDelete')):
        op = document['paths'][path][method]
        statuses = {'200', '400', '401', '403', '429', '503'}
        if body:
            statuses |= {'409', '413', '422'}
        if (op['operationId'] != oid or set(op['responses']) != statuses
                or op.get('x-push-registration-policy') != PUSH_POLICY
                or op['responses']['200']['content']['application/json']['schema'] != {
                    '$ref': '#/components/schemas/PushRegistrationState'}
                or op.get('parameters', []) != ([{'name': 'provider', 'in': 'path',
                    'required': True, 'schema': {'$ref': '#/components/schemas/PushProvider'}}]
                    if method == 'delete' else [])):
            raise SystemExit('注册路径/投影/事务声明不符：' + oid)
        expected_body = ({'required': True, 'x-max-bytes': 262144, 'content': {'application/json': {'schema': {
            '$ref': '#/components/schemas/' + body}}}} if body else None)
        expected_codes = {'401': ['AUTHENTICATION_REQUIRED'], '403': ['FORBIDDEN']}
        if body:
            expected_codes['409'] = ['REGISTRATION_CONFLICT', 'REGISTRATION_VERSION_EXHAUSTED']
        if op.get('requestBody') != expected_body or op.get('x-error-codes') != expected_codes:
            raise SystemExit('注册请求/错误声明不符：' + oid)
    for name, expected in NOTIFICATION_SCHEMAS.items():
        if schemas.get(name) != expected:
            raise SystemExit('通知Schema冻结声明不符：' + name)
    for method, oid in (('get', 'getCurrentNotifications'), ('put', 'updateCurrentNotifications')):
        op = document['paths']['/devices/current/notifications'][method]
        statuses = {'200', '400', '401', '403', '429', '503'}
        codes = {'401': ['AUTHENTICATION_REQUIRED'], '403': ['FORBIDDEN']}
        if method == 'put':
            statuses |= {'404', '409', '413', '422'}
            codes.update({'404': ['RESOURCE_NOT_FOUND'], '409': ['SETTINGS_CONFLICT', 'SETTINGS_VERSION_EXHAUSTED'],
                          '422': ['VALIDATION_FAILED', 'PROVIDER_NOT_READY']})
        expected_body = ({'required': True, 'x-max-bytes': 262144, 'content': {'application/json': {'schema': {
            '$ref': '#/components/schemas/NotificationSettingsUpdate'}}}} if method == 'put' else None)
        if (op['operationId'] != oid or set(op['responses']) != statuses
                or op.get('x-client-type') != 'android' or op.get('parameters', []) != []
                or op.get('requestBody') != expected_body or op.get('x-error-codes') != codes
                or op.get('x-notification-settings-policy') != NOTIFICATION_POLICY
                or op.get('x-notification-job-policy') != NOTIFICATION_JOB_POLICY
                or op['responses']['200']['content']['application/json']['schema'] != {
                    '$ref': '#/components/schemas/NotificationSettingsState'}):
            raise SystemExit('通知路径/权限/状态/事务声明不符：' + oid)
        if method == 'put' and op['responses']['409']['content']['application/json']['schema'] != {
                '$ref': '#/components/schemas/NotificationSettingsConflict'}:
            raise SystemExit('通知冲突缺少最新版本及恢复路径')
    expected_union = ['SyncMessageUpsert', 'SyncMessageDelete', 'SyncBotUpsert', 'SyncBotDelete',
                      'SyncReadUpdate', 'SyncDeliveryUpsert', 'SyncDeviceNotifications', 'SyncGlobalSettings']
    if schemas['SyncChange']['oneOf'] != [{'$ref': '#/components/schemas/' + name} for name in expected_union]:
        raise SystemExit('同步union全局与设备配置分支不完整')
    if set(operation_map) != baseline_ids | set(new_security) | delivery_ids | {'listDeviceSessions', 'revokeDeviceSession', 'getHealthLive', 'getHealthReady', 'getAdminStatus', 'getGlobalSettings', 'updateGlobalSettings'}:
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
