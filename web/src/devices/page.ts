import { DeviceClient, type DeviceSession } from './client.js';
import { RequestError, errorMessage } from '../shared/errors.js';

export function mountDevices(root: HTMLElement, client: DeviceClient, currentID: string, invalidate: () => void): () => void {
  let items: DeviceSession[] = [];
  let cursor: string | null = null;
  let busy = false;
  let disposed = false;
  // 生命周期代次同时隔离分页、撤销和卸载，迟到响应不能恢复旧列表。
  let generation = 0;
  let message = '';

  function text(tag: string, value: string): HTMLElement {
    const node = document.createElement(tag);
    node.textContent = value;
    return node;
  }
  function button(label: string, action: () => void): HTMLButtonElement {
    const node = document.createElement('button');
    node.type = 'button'; node.textContent = label; node.disabled = busy;
    node.addEventListener('click', action); return node;
  }
  function failure(error: unknown): void {
    if (error instanceof RequestError && error.status === 401) { invalidate(); return; }
    message = error instanceof RequestError && error.status === 404
      ? '目标会话已不存在，请刷新设备列表。' : errorMessage(error);
  }
  function render(): void {
    if (disposed) return;
    root.replaceChildren(text('h2', '设备会话'));
    root.setAttribute('aria-busy', String(busy));
    const alert = text('p', message); alert.setAttribute('role', 'status'); alert.setAttribute('aria-live', 'polite');
    root.append(alert, button('刷新设备列表', () => { void load(); }));
    const list = document.createElement('ul');
    for (const item of items) {
      const row = document.createElement('li');
      // 完整标识只用于内存定位，避免通过 DOM 属性或显示文本泄露。
      row.append(text('p', `${item.administrator.username} · ${item.device_name} · ${item.client_type}${item.id === currentID ? '（当前会话）' : ''}`),
        text('p', `设备：${item.device_id ? (item.device_id.length > 4 ? '…' + item.device_id.slice(-4) : '已隐藏') : '浏览器'}；空闲到期：${item.idle_expires_at}；绝对到期：${item.absolute_expires_at}`),
        button('撤销此会话', () => { void revoke(item.id); }));
      list.append(row);
    }
    root.append(list);
    if (!busy && !items.length) root.append(text('p', '暂无设备会话。'));
    if (cursor) root.append(button('下一页', () => { void load(cursor!); }));
    else if (!busy && items.length) root.append(text('p', '已到最后一页。'));
  }
  async function load(next = ''): Promise<void> {
    if (disposed || busy) return;
    busy = true; message = ''; const ticket = ++generation;
    if (!next) { items = []; cursor = null; }
    render();
    try {
      const page = await client.list(next);
      if (disposed || ticket !== generation) return;
      // 分页按 Session.id 合并，绝不以安装标识去重。
      items = next ? [...items, ...page.items.filter(item => !items.some(old => old.id === item.id))] : page.items;
      cursor = page.has_more ? page.next_cursor : null;
    } catch (error) { if (!disposed && ticket === generation) failure(error); }
    finally { if (!disposed && ticket === generation) { busy = false; render(); } }
  }
  async function revoke(id: string): Promise<void> {
    if (disposed || busy) return;
    busy = true; message = ''; const ticket = ++generation; render();
    try {
      await client.revoke(id);
      if (disposed || ticket !== generation) return;
      if (id === currentID) { invalidate(); return; }
      busy = false;
      await load();
    } catch (error) { if (!disposed && ticket === generation) failure(error); }
    finally { if (!disposed && ticket === generation) { busy = false; render(); } }
  }
  void load();
  return () => { disposed = true; generation++; items = []; cursor = null; root.replaceChildren(); };
}
