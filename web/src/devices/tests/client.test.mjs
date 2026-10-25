import { test } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp, readFile, writeFile, mkdir, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import ts from 'typescript';
const dir = await mkdtemp(join(tmpdir(), 'relaynest-devices-'));
await writeFile(join(dir, 'package.json'), '{"type":"module"}');
for (const path of ['shared/errors', 'auth/client', 'devices/client']) {
  await mkdir(join(dir, path.split('/')[0]), { recursive: true });
  await writeFile(join(dir, `${path}.js`), ts.transpileModule(await readFile(new URL(`../../${path}.ts`, import.meta.url), 'utf8'), {
    compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ESNext },
  }).outputText);
}
const { AuthClient } = await import(join(dir, 'auth/client.js'));
const { DeviceClient } = await import(join(dir, 'devices/client.js'));
const item = { id: 'one/+', administrator: { username: 'admin', secret: 'private' }, client_type: 'web', device_id: 'install', device_name: 'name', idle_expires_at: '2030-01-01Z', absolute_expires_at: '2030-01-02Z', token: 'private' };
const authDTO = { session: item, csrf_token: 'test-private' };

test('opaque分页、七字段、同install多会话、DELETE精确ID及204无需JSON', async () => {
  const calls = [];
  const auth = new AuthClient(async (url, options) => {
    calls.push({ url, ...options });
    if (url.includes('/auth/')) return Response.json(authDTO);
    if (options.method === 'DELETE') return { ok: true, status: 204, json() { throw Error('must not parse'); } };
    return Response.json({ items: [item, { ...item, id: 'two' }], next_cursor: url.includes('cursor=') ? null : 'opaque/+ =中', has_more: !url.includes('cursor='), token: 'private' });
  });
  await auth.restore(); const client = new DeviceClient(auth);
  const page = await client.list();
  assert.equal(page.items.length, 2);
  assert.equal(JSON.stringify(page).includes('private'), false);
  assert.equal(Object.keys(page.items[0]).length, 7);
  await client.list(page.next_cursor);
  assert.equal(new URL(calls.at(-1).url, 'http://local').searchParams.get('cursor'), page.next_cursor);
  assert.equal(calls[1].url, '/api/v1/devices?limit=50');
  await client.revoke(item.id);
  const call = calls.at(-1);
  assert.equal(call.url, '/api/v1/devices/one%2F%2B/session');
  assert.equal(call.body, undefined); assert.equal(call.headers['X-CSRF-Token'], 'test-private');
  assert.equal(call.credentials, 'same-origin'); assert.equal(call.cache, 'no-store'); assert.equal(call.redirect, 'error');
});

test('两种401均清CSRF；403/503保留；无效DTO拒绝', async () => {
  let status = 200, headers;
  const auth = new AuthClient(async (url, options) => {
    headers = options.headers;
    if (url.includes('/auth/')) return Response.json(authDTO);
    return status === 200 ? Response.json({ items: [item], next_cursor: null, has_more: false }) : Response.json({ error: { code: options.method === 'DELETE' ? 'AUTHENTICATION_REQUIRED' : 'AUTHENTICATION_FAILED', message: 'private' } }, { status });
  });
  const client = new DeviceClient(auth);
  for (const method of ['list', 'revoke']) {
    await auth.restore(); status = 401;
    await assert.rejects(client[method]('target'), e => e.status === 401 && !e.message.includes('private'));
    await assert.rejects(client.revoke('target')); assert.equal(headers['X-CSRF-Token'], '');
  }
  for (status of [403, 503]) {
    await auth.restore(); await assert.rejects(client.revoke('target'));
    assert.equal(headers['X-CSRF-Token'], 'test-private');
  }
  for (const value of [null, {}, { items: [item], next_cursor: '', has_more: true }, { items: [{ ...item, client_type: 'bad' }], next_cursor: null, has_more: false }]) {
    await assert.rejects(new DeviceClient(new AuthClient(async () => Response.json(value))).list());
  }
});

test('clear后的迟到restore不能重新写入CSRF，旧设备401不清新会话', async () => {
  let resolve;
  let late = false, sent;
  const auth = new AuthClient((url, options) => {
    sent = options.headers;
    if (late) return new Promise(r => { resolve = r; });
    return Promise.resolve(options.method === 'DELETE' ? new Response(null, { status: 204 }) : Response.json(authDTO));
  });
  late = true; const pending = auth.restore(); auth.clear(); late = false;
  resolve(Response.json(authDTO)); await assert.rejects(pending);
  late = true; const old = auth.listDevices(new URLSearchParams()); late = false;
  await auth.restore(); resolve(new Response(null, { status: 401 })); await assert.rejects(old);
  await auth.revokeDevice('target'); assert.equal(sent['X-CSRF-Token'], 'test-private');
});
test.after(() => rm(dir, { recursive: true, force: true }));
