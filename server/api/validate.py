"""校验结构、示例及权限声明；字节/Range 扩展仅验证契约，不模拟 HTTP 服务。"""

import copy
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


ContractValidator = validators.extend(Draft202012Validator, {
    'x-utf8-max-bytes': utf8_max_bytes,
    'x-byte-ranges': byte_ranges,
    'x-content-range': content_range,
})


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
        'uploadAttachment': ['AdminBearer', 'BotAPIToken'],
        'downloadAttachment': ['AdminBearer', 'BotAPIToken'],
    }
    delivery_ids = {'claimEvents', 'ackDelivery', 'renewDelivery', 'listDeliveries', 'retryDelivery'}
    new_security.update({'claimEvents': ['BotAPIToken'], 'ackDelivery': ['BotAPIToken'],
                         'renewDelivery': ['BotAPIToken'], 'listDeliveries': ['AdminBearer', 'WebCookie']})
    idempotent = {'createBotMessage', 'editBotMessage', 'createWebhookMessage',
                  'createClientMessage', 'createCallback', 'uploadAttachment', 'retryDelivery'}
    reference_ops = set(new_security) - {'listClientMessages', 'getClientMessage'} - delivery_ids
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
    if set(operation_map) != baseline_ids | set(new_security) | delivery_ids:
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
