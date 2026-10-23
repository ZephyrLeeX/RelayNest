import { test } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp, readFile, writeFile, mkdir, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import ts from 'typescript';

// 在临时目录转译真实源码，不修改锁文件或生成仓库构建产物。
const dir = await mkdtemp(join(tmpdir(), 'relaynest-auth-test-'));
await mkdir(join(dir, 'shared'));
await mkdir(join(dir, 'auth'));
await writeFile(join(dir, 'package.json'), '{"type":"module"}');
for (const path of ['shared/errors', 'auth/client']) {
  const source = await readFile(new URL(`../../${path}.ts`, import.meta.url), 'utf8');
  await writeFile(join(dir, `${path}.js`), ts.transpileModule(source, {
    compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ESNext },
  }).outputText);
}
const { AuthClient } = await import(join(dir, 'auth/client.js'));
const { errorMessage } = await import(join(dir, 'shared/errors.js'));
const dto = () => ({ session: { id: 'session-1', administrator: { username: 'admin' }, client_type: 'web',
  device_name: 'browser', idle_expires_at: '2030-01-01T00:00:00Z', absolute_expires_at: '2030-01-07T00:00:00Z' }, csrf_token: 'test-csrf' });

test('登录严格 DTO、同源请求；恢复仅 Cookie；退出仅 CSRF 且接受 204', async () => {
  const calls = [];
  const client = new AuthClient(async (url, options) => {
    calls.push({ url, ...options });
    return url.endsWith('logout') ? new Response(null, { status: 204 }) : Response.json(dto());
  });
  await client.login('admin', 'temporary-test-input', 'browser');
  await client.restore();
  await client.logout();
  assert.deepEqual(JSON.parse(calls[0].body), { username: 'admin', password: 'temporary-test-input', client_type: 'web', device_name: 'browser' });
  for (const call of calls) {
    assert.equal(call.credentials, 'same-origin'); assert.equal(call.cache, 'no-store');
    assert.equal(call.headers.Authorization, undefined); assert.equal(call.redirect, 'error');
  }
  assert.equal(calls[1].body, undefined); assert.deepEqual(calls[1].headers, {});
  assert.equal(calls[2].headers['X-CSRF-Token'], 'test-csrf');
  await client.logout();
  assert.equal(calls[3].headers['X-CSRF-Token'], '');
});

test('401 清 CSRF，403保留会话可显式恢复，429/503及非JSON显示安全文案', async () => {
  let status = 200;
  let sent;
  const client = new AuthClient(async (_url, options) => {
    sent = options.headers;
    return status === 200 ? Response.json(dto()) : new Response('private-password-sql', { status });
  });
  await client.restore();
  for (const code of [403, 429, 503, 401]) {
    status = code;
    await assert.rejects(client.logout(), error => {
      const message = errorMessage(error);
      assert.equal(message.includes('private-password-sql'), false);
      assert.match(message, code === 429 ? /频繁/ : code === 403 ? /安全校验/ : code === 503 ? /暂时/ : /过期/);
      return true;
    });
  }
  await assert.rejects(client.logout());
  assert.equal(sent['X-CSRF-Token'], '');
});

test('拒绝Android响应、畸形响应和非204退出；不传播未知响应字段', async () => {
  for (const value of [{ session: { client_type: 'android' }, access_token: 'secret' }, {}, null,
    { ...dto(), csrf_token: '' }]) {
    await assert.rejects(new AuthClient(async () => Response.json(value)).restore());
  }
  const value = dto(); value.session.secret = 'private';
  const client = new AuthClient(async () => Response.json(value));
  assert.equal((await client.restore()).secret, undefined);
  await assert.rejects(client.logout());
});

test('网络异常不向文案泄露内部错误', () => {
  assert.equal(errorMessage(new Error('secret')), '网络连接失败，请检查连接后重试。');
});

test.after(async () => { await rm(dir, { recursive: true, force: true }); });
