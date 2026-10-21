"""校验 OpenAPI 结构、Schema 示例及当前卡的权限声明；不模拟运行时鉴权。"""

import json
from pathlib import Path

import yaml
from jsonschema import Draft202012Validator, FormatChecker
from openapi_spec_validator import validate_spec

ROOT = Path(__file__).resolve().parent


def main():
    document = yaml.safe_load((ROOT / 'openapi.yaml').read_text(encoding='utf-8'))
    validate_spec(document)
    schemas = document['components']['schemas']
    for schema in schemas.values():
        Draft202012Validator.check_schema(schema)
    cases = json.loads((ROOT / 'examples.json').read_text(encoding='utf-8'))
    failures = []
    for case in cases:
        # 将 components 放到本地根，所有 $ref 保持 OpenAPI 内部绝对片段，不进行远端抓取。
        schema = {'$ref': '#/components/schemas/' + case['schema'],
                  'components': {'schemas': schemas}}
        validator = Draft202012Validator(schema, format_checker=FormatChecker())
        actual = validator.is_valid(case['value'])
        if actual != case['valid']:
            failures.append(case['name'])
    if failures:
        raise SystemExit('示例校验与预期不符：' + '、'.join(failures))

    operations = 0
    for path, item in document['paths'].items():
        for method, operation in item.items():
            if method == 'parameters':
                continue
            operations += 1
            if operation['operationId'] == 'login':
                expected = []
            elif method == 'get':
                expected = [{'AdminBearer': []}, {'WebCookie': []}]
            else:
                expected = [{'AdminBearer': []}, {'WebCookie': [], 'CSRF': []}]
            if operation['security'] != expected:
                raise SystemExit(f'管理员权限声明错误：{method} {path}')
            # 独立 scheme 的同为 Bearer 语法不代表 Token 可以互换；运行时须校验身份/用途。
            for response in operation['responses'].values():
                if '$ref' not in response:
                    header = response['headers']['Cache-Control']['schema']
                    if header != {'type': 'string', 'const': 'no-store'}:
                        raise SystemExit(f'响应缺少 no-store：{method} {path}')
    schemes = document['components']['securitySchemes']
    for name in ('AdminBearer', 'BotAPIToken', 'WebhookKey'):
        if schemes[name]['type'] != 'http' or schemes[name]['scheme'] != 'bearer':
            raise SystemExit('Bearer scheme 声明错误：' + name)
    # 共享正例和反例必须同时存在，否则不能用删光反例获得通过。
    valid = sum(case['valid'] for case in cases)
    invalid = len(cases) - valid
    if not valid or not invalid:
        raise SystemExit('必须同时包含合法与非法示例')
    print(f'PASS: OpenAPI 3.1; {len(schemas)} schemas; '
          f'{valid} valid + {invalid} invalid examples; {operations} operations/security')


if __name__ == '__main__':
    main()
