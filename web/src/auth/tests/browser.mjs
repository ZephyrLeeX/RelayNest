// 真实 Chromium + 临时 SQLite + 生产服务；不连接现网。
// PLAYWRIGHT_MODULE 指向现成模块，RELAYNEST_BINARY 指向当前树编译的二进制。
import assert from 'node:assert/strict';
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
    if (req.url.startsWith('/api/')) {
      if (req.method === 'POST') writes.push({ path: req.url, origin: req.headers.origin, csrf: !!req.headers['x-csrf-token'] });
      const upstream = http.request({ hostname: '127.0.0.1', port, path: req.url, method: req.method,
        headers: { ...req.headers, 'x-forwarded-for': '127.0.0.1', 'x-forwarded-proto': 'https', 'x-forwarded-host': new URL(origin).host } }, result => {
        res.writeHead(result.statusCode, result.headers); result.pipe(res);
      });
      upstream.on('error', () => { res.writeHead(502).end(); });
      req.pipe(upstream);
    } else {
      const path = req.url === '/' ? 'index.html' : req.url.slice(1);
      if (!/^(index\.html|assets\/[A-Za-z0-9_.-]+)$/.test(path)) { res.writeHead(404).end(); return; }
      readFile(resolve('server/internal/webui/dist', path)).then(data => {
        res.setHeader('Content-Type', path.endsWith('.js') ? 'text/javascript' : path.endsWith('.css') ? 'text/css' : 'text/html'); res.end(data);
      }).catch(() => res.writeHead(404).end());
    }
  });
  const request = (path, method = 'GET', body, cookie, csrf) => new Promise((resolve, reject) => {
    const req = https.request(origin + '/api/v1/auth/' + path, { method, rejectUnauthorized: false,
      headers: { Origin: origin, ...(body ? { 'Content-Type': 'application/json' } : {}),
        ...(cookie ? { Cookie: cookie } : {}), ...(csrf ? { 'X-CSRF-Token': csrf } : {}) } }, res => {
      let text = ''; res.on('data', chunk => { text += chunk; });
      res.on('end', () => resolve({ status: res.statusCode, headers: res.headers, text }));
    }); req.on('error', reject); req.end(body);
  });
  const body = JSON.stringify({ username: 'test-admin', password, client_type: 'web', device_name: 'test-browser' });
  const loggedIn = await request('login', 'POST', body);
  assert.equal(loggedIn.status, 200);
  const rawCookie = loggedIn.headers['set-cookie'][0];
  assert.match(rawCookie, /HttpOnly/); assert.match(rawCookie, /Secure/); assert.match(rawCookie, /SameSite=Lax/);
  const jarCookie = rawCookie.split(';')[0];
  const csrfToken = JSON.parse(loggedIn.text).csrf_token;
  assert.equal((await request('session', 'GET', undefined, jarCookie)).status, 200);
  assert.equal((await request('logout', 'POST', undefined, jarCookie)).status, 403);
  assert.equal((await request('logout', 'POST', undefined, jarCookie, csrfToken)).status, 204);
  assert.equal((await request('session', 'GET', undefined, jarCookie)).status, 401);
  console.log('PASS real HTTPS requests with manual Cookie jar: login/session/CSRF403/logout204/revocation401/Cookie headers; not browser evidence');
  if (!process.env.REAL_REQUEST_ONLY) {
  const { chromium } = await import(process.env.PLAYWRIGHT_MODULE);
  browser = await chromium.launch({ executablePath: process.env.CHROMIUM_EXECUTABLE, headless: true, args: ['--no-sandbox'] });
  const context = await browser.newContext({ ignoreHTTPSErrors: true });
  const page = await context.newPage();
  const errors = [];
  page.on('pageerror', error => errors.push(error.message));
  await page.goto(origin);
  await page.waitForFunction(() => document.querySelector('#app').getAttribute('aria-busy') === 'false');
  const login = async value => {
    await page.getByLabel('管理员用户名').fill('test-admin');
    await page.getByLabel('密码', { exact: true }).fill(value);
    await page.getByRole('button', { name: '登录', exact: true }).click();
  };
  await login('incorrect-test-input');
  await page.getByRole('alert').filter({ hasText: '用户名或密码错误' }).waitFor();
  assert.equal(await page.getByLabel('密码', { exact: true }).inputValue(), '');
  await login(password);
  await page.locator('[data-session]').waitFor();
  const cookie = (await context.cookies()).find(c => c.name === 'relaynest_session');
  assert.ok(cookie?.httpOnly && cookie.secure && cookie.sameSite === 'Lax');
  assert.equal(await page.evaluate(() => document.cookie), '');
  assert.deepEqual(await page.evaluate(() => [localStorage.length, sessionStorage.length]), [0, 0]);
  await page.reload();
  await page.locator('[data-session]').waitFor();
  const forbidden = await page.evaluate(() => fetch('/api/v1/auth/logout', { method: 'POST', credentials: 'same-origin' }).then(r => r.status));
  assert.equal(forbidden, 403);
  await page.getByRole('button', { name: '退出登录', exact: true }).click();
  await page.getByRole('alert').filter({ hasText: '已安全退出' }).waitFor();
  assert.equal((await context.cookies()).some(c => c.name === 'relaynest_session'), false);
  assert.equal(writes.at(-1).origin, origin);
  assert.equal(writes.at(-1).csrf, true);
  console.log('PASS real browser: login failure/password clearing/login/reload/session/Secure+HttpOnly+SameSite/storage/CSRF403/logout204/browser Origin');

  // 以下显式为协议桩交互：隔离401、403、429及短到期窗口，不能冒称服务端E2E。
  let mode = 'session';
  const synthetic = () => ({ session: { id: 'synthetic', administrator: { username: '<script>private</script>' },
    client_type: 'web', device_name: 'test', idle_expires_at: new Date(Date.now() + 1200).toISOString(),
    absolute_expires_at: new Date(Date.now() + 60000).toISOString() }, csrf_token: 'synthetic-csrf' });
  await page.route('**/api/v1/auth/*', async route => {
    const path = new URL(route.request().url()).pathname;
    const status = mode === '429' ? 429 : path.endsWith('/logout') ? 403 : mode === '401' ? 401 : 200;
    await route.fulfill({ status, json: status === 200 ? synthetic() : { error: { code: 'FORBIDDEN', message: 'sensitive-server-data' } } });
  });
  await page.getByRole('button', { name: '重新检查会话' }).click();
  await page.locator('[data-session]').waitFor();
  assert.equal(await page.locator('script').count(), 1);
  await page.getByRole('button', { name: '退出登录', exact: true }).click();
  await page.getByRole('alert').filter({ hasText: '安全校验失败' }).waitFor();
  mode = '401';
  await page.getByRole('button', { name: '重新检查会话' }).click();
  await page.getByRole('alert').filter({ hasText: '会话已过期' }).waitFor();
  mode = '429';
  await login(password);
  await page.getByRole('alert').filter({ hasText: '请求过于频繁' }).waitFor();
  mode = 'session';
  await page.getByRole('button', { name: '重新检查会话' }).click();
  await page.locator('[data-session]').waitFor();
  await page.getByRole('alert').filter({ hasText: '会话已过期' }).waitFor();
  assert.equal((await page.locator('body').innerText()).includes('sensitive-server-data'), false);
  assert.deepEqual(errors, []);
  console.log('PASS browser protocol fixtures: 403 safe shell/401 expiry/429/idle expiry/HTML text escaping');
  }
} finally {
  await browser?.close();
  backend?.kill('SIGTERM');
  await new Promise(r => proxy.close(r));
  if (backend && backend.exitCode === null) await new Promise(r => backend.once('exit', r));
  await rm(dir, { recursive: true, force: true });
}
