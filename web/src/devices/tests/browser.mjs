// 真实 Chromium + 临时 SQLite + 生产服务；不连接现网。
// PLAYWRIGHT_MODULE 指向现成模块，RELAYNEST_BINARY 指向当前树编译的二进制。
import assert from 'node:assert/strict';
import ts from 'typescript';
import https from 'node:https';
import http from 'node:http';
import net from 'node:net';
import { spawn, spawnSync } from 'node:child_process';
import { mkdtemp, readFile, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { randomBytes } from 'node:crypto';

const dir = await mkdtemp(join(tmpdir(), 'relaynest-browser-'));
const db = join(dir, 'test.sqlite');
const password = randomBytes(24).toString('base64url');
const binary = process.env.RELAYNEST_BINARY;
let browser, backend;
const proxy = https.createServer();
try {
  assert.equal(spawnSync('openssl', ['req', '-x509', '-newkey', 'rsa:2048', '-nodes', '-days', '1',
    '-subj', '/CN=127.0.0.1', '-keyout', join(dir, 'key'), '-out', join(dir, 'cert')], { stdio: 'ignore' }).status, 0);
  proxy.setSecureContext({ key: await readFile(join(dir, 'key')), cert: await readFile(join(dir, 'cert')) });
  const reserve = net.createServer();
  await new Promise(r => reserve.listen(0, '127.0.0.1', r));
  const port = reserve.address().port;
  await new Promise(r => reserve.close(r));
  await new Promise(r => proxy.listen(0, '127.0.0.1', r));
  const origin = `https://127.0.0.1:${proxy.address().port}`;
  const init = spawnSync(binary, ['admin', 'init', '--db', db, '--username', 'test-admin'], { input: `${password}\n` });
  assert.equal(init.status, 0, '临时管理员初始化成功');
  backend = spawn(binary, ['-listen', `127.0.0.1:${port}`, '-db', db, '-external-origin', origin, '-trusted-proxy', '127.0.0.1'], { stdio: 'ignore' });
  for (let i = 0; i < 100; i++) {
    try { if ((await fetch(`http://127.0.0.1:${port}/health/live`)).ok) break; } catch {}
    if (i === 99) throw new Error('临时服务未就绪');
    await new Promise(r => setTimeout(r, 50));
  }
  const writes = [];
  proxy.on('request', (req, res) => {
    {
      if (req.method === 'POST') writes.push({ path: req.url, origin: req.headers.origin, csrf: !!req.headers['x-csrf-token'] });
      const upstream = http.request({ hostname: '127.0.0.1', port, path: req.url, method: req.method,
        headers: { ...req.headers, 'x-forwarded-for': '127.0.0.1', 'x-forwarded-proto': 'https', 'x-forwarded-host': new URL(origin).host } }, result => {
        res.writeHead(result.statusCode, result.headers); result.pipe(res);
      });
      upstream.on('error', () => { res.writeHead(502).end(); });
      req.pipe(upstream);
    }
  });
  const { chromium } = await import(process.env.PLAYWRIGHT_MODULE);
  browser = await chromium.launch({ executablePath: process.env.CHROMIUM_EXECUTABLE, headless: true, args: ['--no-sandbox'] });
  const contexts = await Promise.all([browser.newContext({ ignoreHTTPSErrors: true }), browser.newContext({ ignoreHTTPSErrors: true })]);
  const [a, b] = await Promise.all(contexts.map(c => c.newPage()));
  const errors = [];
  for (const page of [a,b]) page.on('pageerror', e => errors.push(e.message));
  async function login(page, name) {
    await page.goto(origin);
    await page.waitForFunction(() => document.querySelector('#app').getAttribute('aria-busy') === 'false');
    await page.getByLabel('管理员用户名').fill('test-admin');
    await page.getByLabel('密码', { exact: true }).fill(password);
    await page.getByLabel('设备名称').fill(name);
    await page.getByRole('button', { name: '登录', exact: true }).click();
    await page.locator('[data-session]').waitFor(); await ready(page);
  }
  async function ready(page) { await page.waitForFunction(() => document.querySelector('[data-devices]')?.getAttribute('aria-busy') === 'false' || (document.querySelector('form') && document.querySelector('#app').getAttribute('aria-busy') === 'false')); }
  async function refresh(page) { await page.getByRole('button', { name: '刷新设备列表', exact: true }).click(); await ready(page); }
  async function session(page) { return page.evaluate(() => fetch('/api/v1/auth/session').then(async r => ({ status: r.status, value: r.ok ? (await r.json()).session : null }))); }
  async function list(page) { return page.evaluate(() => fetch('/api/v1/devices').then(async r => ({ status: r.status, value: r.ok ? await r.json() : null }))); }
  await login(a, 'Browser A'); await login(b, 'Browser B');
  const aid = (await session(a)).value.id, bid = (await session(b)).value.id;
  for (const mode of ['missing', 'wrong']) {
    const status = await a.evaluate(async ({ id, mode }) => (await fetch(`/api/v1/devices/${id}/session`, {
      method: 'DELETE', credentials: 'same-origin', headers: mode === 'wrong' ? { 'X-CSRF-Token': 'invalid-test-only' } : {},
    })).status, { id: bid, mode });
    assert.equal(status, 403);
    assert.equal((await session(b)).status, 200); await refresh(b);
    const rows = await list(b); assert.equal(rows.status, 200); assert.ok(rows.value.items.some(i => i.id === bid)); assert.ok(rows.value.items.some(i => i.id === aid));
    assert.equal((await session(a)).status, 200); await refresh(a); assert.equal(await a.locator('[data-session]').count(), 1);
    console.log(`PASS real device DELETE ${mode} CSRF: 403; A authenticated/refreshed; B authenticated/refreshed; both exact sessions remain`);
  }
  // 仅在本地临时服务创建同安装的两个真实Android会话，不保留其凭据。
  const seeded = await b.evaluate(async password => {
    const sessions = [];
    for (const name of ['Seed One', 'Seed Two']) {
      const r = await fetch('/api/v1/auth/login', { method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username: 'test-admin', password, client_type: 'android', device_name: name, device_id: '11111111-1111-4111-8111-111111111111' }) });
      if (r.status !== 200) throw Error('本地会话种子失败');
      sessions.push((await r.json()).session.id);
    }
    return sessions;
  }, password);
  await refresh(b);
  assert.equal(await b.locator('[data-devices] li').filter({ hasText: 'Seed ' }).count(), 2);
  const metadata = (await list(b)).value;
  assert.ok(metadata.items.every(i => Object.keys(i).sort().join(',') === 'absolute_expires_at,administrator,client_type,device_id,device_name,id,idle_expires_at'));
  assert.doesNotMatch(await b.locator('[data-devices]').innerText(), new RegExp(`${aid}|${bid}|access_token|refresh_token|csrf_token`));
  async function revoke(page, name) {
    const response = page.waitForResponse(r => r.request().method() === 'DELETE' && r.url().includes('/devices/'));
    await page.locator('[data-devices] li').filter({ hasText: name }).getByRole('button').click();
    assert.equal((await response).status(), 204);
  }
  await revoke(b, 'Seed One'); await ready(b);
  const surviving = (await list(b)).value.items;
  assert.ok(!surviving.some(i => i.id === seeded[0]) && surviving.some(i => i.id === seeded[1]));
  console.log('PASS real same-install seed: two distinct sessions rendered; exact revoke removes only selected session');
  await revoke(b, 'Browser A'); await ready(b);
  const revoked = await a.evaluate(() => fetch('/api/v1/devices').then(r => r.status)); assert.equal(revoked, 401);
  await refresh(a); await a.getByRole('button', { name: '登录', exact: true }).waitFor();
  await refresh(b); assert.equal((await session(b)).status, 200);
  await revoke(b, 'Browser B'); await b.getByRole('button', { name: '登录', exact: true }).waitFor();
  assert.equal((await contexts[1].cookies()).some(c => c.name === 'relaynest_session'), false);
  // 同两个真实context重新登录，交换撤销者以复验非自撤及自撤分支。
  await login(a, 'Browser A2'); await login(b, 'Browser B2'); await refresh(a);
  await revoke(a, 'Browser B2'); await ready(a); await refresh(b);
  await b.getByRole('button', { name: '登录', exact: true }).waitFor();
  await revoke(a, 'Browser A2'); await a.getByRole('button', { name: '登录', exact: true }).waitFor();
  assert.equal((await contexts[0].cookies()).some(c => c.name === 'relaynest_session'), false);
  console.log('PASS real browser production embed: two independent contexts, both revoke directions, exact ID, 204/self-cookie-clear/401, metadata whitelist and no full IDs in DOM');

  // 独立协议fixture，仅证明分页、UI失败及生命周期，不作真实后端证据。
  const date = new Date(Date.now() + 600000).toISOString();
  const item = (id, name) => ({ id, administrator: { username: '<img src=x onerror=alert(1)>' }, client_type: 'android', device_id: 'same-install-secret', device_name: name, idle_expires_at: date, absolute_expires_at: date, access_token: 'sensitive-fixture' });
  let mode = 'normal', requests = [], release;
  await a.route('**/api/v1/auth/*', route => route.fulfill({ status: route.request().url().endsWith('/logout') ? 204 : 200,
    ...(route.request().url().endsWith('/logout') ? {} : { json: { session: { ...item('current', 'fixture'), client_type: 'web' }, csrf_token: 'fixture-csrf' } }) }));
  await a.route('**/api/v1/devices**', async route => {
    requests.push(route.request().url());
    if (mode === 'network') { await route.abort(); return; }
    if (mode === 'delay') await new Promise(r => { release = r; });
    if (route.request().method() === 'DELETE') {
      await route.fulfill({ status: Number(mode) || 204, ...(Number(mode) ? { json: { error: { message: 'sensitive-fixture' } } } : {}) }); return;
    }
    const next = new URL(route.request().url()).searchParams.get('cursor');
    await route.fulfill({ json: mode === 'invalid' ? {} : mode === 'empty' ? { items: [], next_cursor: null, has_more: false } : {
      items: next ? [item('two', 'second')] : [item('one', 'first')], next_cursor: next ? null : 'opaque/+ =中', has_more: !next,
    } });
  });
  await a.reload(); await ready(a);
  await a.getByRole('button', { name: '下一页', exact: true }).click(); await ready(a);
  assert.equal(await a.locator('[data-devices] li').count(), 2);
  assert.equal(new URL(requests.at(-1)).searchParams.get('cursor'), 'opaque/+ =中');
  assert.equal(await a.locator('[data-devices] img').count(), 0);
  for (const [status, text] of [['404','目标会话已不存在'], ['403','安全校验失败'], ['429','请求过于频繁'], ['503','服务暂时不可用']]) {
    mode = status; await a.locator('[data-devices] li').first().getByRole('button').click();
    await a.getByRole('status').filter({ hasText: text }).waitFor();
    assert.equal(await a.locator('[data-devices] li').count(), 2);
  }
  mode = '401'; await a.locator('[data-devices] li').first().getByRole('button').click();
  await a.getByRole('button', { name: '登录', exact: true }).waitFor();
  mode = 'normal'; await a.getByRole('button', { name: '重新检查会话' }).click(); await a.locator('[data-devices]').waitFor(); await ready(a);
  mode = 'invalid'; await refresh(a); await a.getByRole('status').filter({ hasText: '请求未完成' }).waitFor();
  mode = 'network'; await refresh(a); await a.getByRole('status').filter({ hasText: '网络连接失败' }).waitFor();
  mode = 'empty'; await refresh(a); await a.getByText('暂无设备会话。', { exact: true }).waitFor();
  mode = 'normal'; await refresh(a);
  assert.equal((await a.locator('body').innerText()).includes('sensitive-fixture'), false);
  assert.equal(await a.locator('[data-devices] a').count(), 0);
  mode = 'delay'; await a.getByRole('button', { name: '刷新设备列表', exact: true }).click();
  await a.waitForTimeout(50); await a.getByRole('button', { name: '退出登录', exact: true }).click();
  await a.getByRole('button', { name: '登录', exact: true }).waitFor(); release(); await a.waitForTimeout(100);
  assert.equal(await a.locator('[data-devices]').count(), 0);
  // 直接调用真实模块dispose，延迟响应不得重画已经卸载的根节点。
  const modules = new Map();
  for (const path of ['devices/page', 'shared/errors']) {
    modules.set('/fixture/' + path + '.js', ts.transpileModule(await readFile(new URL('../../' + path + '.ts', import.meta.url), 'utf8'), {
      compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ESNext },
    }).outputText);
  }
  await a.route('**/fixture/**', route => route.fulfill({ contentType: 'text/javascript', body: modules.get(new URL(route.request().url()).pathname) }));
  await a.evaluate(async () => {
    const { mountDevices } = await import('/fixture/devices/page.js');
    const root = document.createElement('section'); document.body.append(root);
    let resolve;
    const dispose = mountDevices(root, { list: () => new Promise(r => { resolve = r; }) }, 'current', () => { throw Error('unexpected invalidation'); });
    dispose(); resolve({ items: [], next_cursor: null, has_more: false });
    await new Promise(r => setTimeout(r, 20));
    if (root.childNodes.length) throw Error('dispose后发生迟到渲染');
    root.remove();
  });
  mode = 'delay'; await a.getByRole('button', { name: '重新检查会话' }).click();
  await a.locator('[data-devices]').waitFor(); await a.waitForTimeout(50);
  // 页面导航销毁模块，旧请求不能把设备视图带到新文档。
  await a.goto('about:blank'); release(); await a.waitForTimeout(50); assert.equal(await a.locator('[data-devices]').count(), 0);
  assert.deepEqual(errors, []);
  console.log('PASS protocol fixtures: opaque pagination/same install sessions/textContent/404/403/429/503/invalid DTO/empty/retry/delayed logout/state switch/unload');
} finally {
  await browser?.close(); backend?.kill('SIGTERM'); await new Promise(r => proxy.close(r));
  if (backend && backend.exitCode === null) await new Promise(r => backend.once('exit', r));
  await rm(dir, { recursive: true, force: true });
}
