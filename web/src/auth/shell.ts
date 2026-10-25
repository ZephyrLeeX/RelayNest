import { DeviceClient } from '../devices/client.js';
import { mountDevices } from '../devices/page.js';
import { AuthClient, type WebSession } from './client.js';
import { errorMessage, RequestError } from '../shared/errors.js';

export function mountAuth(root: HTMLElement, client = new AuthClient()): () => void {
  let generation = 0;
  let disposeDevices: (() => void) | undefined;
  let session: WebSession | undefined;
  let pending = false;
  let disposed = false;
  let timer: ReturnType<typeof setTimeout> | undefined;
  let message = '';

  function expiry(): number {
    return session ? Math.min(Date.parse(session.idle_expires_at), Date.parse(session.absolute_expires_at)) : Infinity;
  }

  function expire(): void {
    generation++;
    pending = false;
    disposeDevices?.();
    session = undefined;
    client.clear();
    message = '会话已过期或已撤销，请重新登录。';
    clearTimeout(timer);
    render();
  }

  function checkExpiry(): void {
    if (disposed || !session) return;
    if (Date.now() >= expiry()) { expire(); return; }
    clearTimeout(timer);
    // 定时器有整数上限；长时间休眠后通过 focus/visibilitychange 再检查。
    timer = setTimeout(checkExpiry, Math.min(expiry() - Date.now(), 2_147_483_647));
  }

  function controls(): void {
    root.querySelectorAll<HTMLInputElement | HTMLButtonElement>('input, button').forEach(control => {
      if (!control.closest('[data-devices]')) control.disabled = pending;
    });
    root.setAttribute('aria-busy', String(pending));
  }

  async function run(action: () => Promise<WebSession | void>, kind: 'login' | 'restore' | 'logout'): Promise<void> {
    if (pending || disposed) return;
    // 切换认证状态前卸载设备页；旧请求只可结束自身，不能重画新会话。
    const ticket = ++generation;
    disposeDevices?.();
    pending = true;
    message = '';
    root.querySelector('[role="alert"]')?.replaceChildren();
    controls();
    try {
      const result = await action();
      if (disposed || ticket !== generation) return;
      session = result || undefined;
      message = kind === 'logout' ? '已安全退出。' : '';
    } catch (error) {
      if (disposed || ticket !== generation) return;
      if (error instanceof RequestError && error.status === 401 && kind !== 'login') {
        session = undefined;
        // 初次打开没有 Cookie 是正常未登录；已登录会话失效应明确提示。
        if (kind === 'restore' && !root.querySelector('[data-session]')) message = '';
        else message = errorMessage(error);
      } else message = errorMessage(error, kind === 'login');
    } finally {
      if (!disposed && ticket === generation) { pending = false; render(); checkExpiry(); }
    }
  }

  function render(): void {
    if (disposed) return;
    disposeDevices?.();
    disposeDevices = undefined;
    root.innerHTML = `<h1>RelayNest 管理</h1><p role="alert" aria-live="polite"></p>`;
    root.querySelector('[role="alert"]')!.textContent = message;
    if (session) {
      const section = document.createElement('section');
      section.dataset.session = '';
      const identity = document.createElement('p');
      identity.textContent = `当前管理员：${session.administrator.username}（${session.device_name}）`;
      const note = document.createElement('p');
      note.textContent = '已登录管理入口。本页不提供消息正文、附件或聊天历史浏览。';
      const logout = document.createElement('button');
      logout.type = 'button';
      logout.textContent = '退出登录';
      logout.addEventListener('click', () => { void run(() => client.logout(), 'logout'); });
      section.append(identity, note, logout);
      const devices = document.createElement('section');
      devices.dataset.devices = '';
      section.append(devices);
      root.append(section);
      if (!pending) disposeDevices = mountDevices(devices, new DeviceClient(client), session.id, expire);
    } else {
      const form = document.createElement('form');
      form.innerHTML = `<p><label>管理员用户名 <input name="username" autocomplete="username" required></label></p>
        <p><label>密码 <input name="password" type="password" autocomplete="off" required></label></p>
        <p><label>设备名称 <input name="device_name" value="Web 管理浏览器" autocomplete="off" required></label></p>
        <button type="submit">登录</button>`;
      form.addEventListener('submit', event => {
        event.preventDefault();
        if (pending || !form.reportValidity()) return;
        const username = form.elements.namedItem('username') as HTMLInputElement;
        const password = form.elements.namedItem('password') as HTMLInputElement;
        const deviceName = form.elements.namedItem('device_name') as HTMLInputElement;
        // 只交给当前请求；提交后立刻清空 DOM，不持久化、不重试密码。
        void run(() => {
          const request = client.login(username.value, password.value, deviceName.value);
          password.value = '';
          return request;
        }, 'login');
      });
      root.append(form);
    }
    const refresh = document.createElement('button');
    refresh.type = 'button';
    refresh.textContent = '重新检查会话';
    refresh.addEventListener('click', () => { void run(() => client.restore(), 'restore'); });
    root.append(refresh);
    controls();
  }

  function clearPassword(): void {
    const password = root.querySelector<HTMLInputElement>('input[type="password"]');
    if (password) password.value = '';
  }
  function onVisibility(): void {
    if (document.visibilityState === 'hidden') clearPassword();
    checkExpiry();
  }
  render();
  void run(() => client.restore(), 'restore');
  window.addEventListener('focus', checkExpiry);
  window.addEventListener('pagehide', clearPassword);
  document.addEventListener('visibilitychange', onVisibility);
  return () => {
    disposed = true;
    generation++;
    disposeDevices?.();
    clearTimeout(timer);
    clearPassword();
    client.clear();
    window.removeEventListener('focus', checkExpiry);
    window.removeEventListener('pagehide', clearPassword);
    document.removeEventListener('visibilitychange', onVisibility);
    root.replaceChildren();
  };
}
