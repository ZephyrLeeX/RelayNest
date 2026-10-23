import { RequestError } from '../shared/errors.js';

export interface WebSession {
  id: string;
  administrator: { username: string };
  client_type: 'web';
  device_name: string;
  idle_expires_at: string;
  absolute_expires_at: string;
}

/** Cookie 完全交由浏览器；CSRF 只从 Web DTO 获取并存于内存。 */
export class AuthClient {
  private csrf = '';
  constructor(private readonly transport: typeof fetch = fetch.bind(globalThis)) {}

  clear(): void { this.csrf = ''; }

  private async request(path: string, method: 'GET' | 'POST', body?: string): Promise<unknown> {
    const headers: Record<string, string> = {};
    if (body !== undefined) headers['Content-Type'] = 'application/json';
    if (path === 'logout') headers['X-CSRF-Token'] = this.csrf;
    const response = await this.transport(`/api/v1/auth/${path}`, {
      method, headers, credentials: 'same-origin', cache: 'no-store', redirect: 'error',
      ...(body === undefined ? {} : { body }),
    });
    if (!response.ok) {
      if (response.status === 401) this.clear();
      let code = '';
      try {
        const dto = await response.json();
        if (typeof dto?.error?.code === 'string') code = dto.error.code;
      } catch { /* 非 JSON 错误仍由 HTTP 状态映射安全文案。 */ }
      throw new RequestError(response.status, code);
    }
    if (path === 'logout' && response.status === 204) { this.clear(); return; }
    if (path === 'logout' || response.status !== 200) throw new RequestError(502, 'INVALID_RESPONSE');
    try { return await response.json(); }
    catch { throw new RequestError(502, 'INVALID_RESPONSE'); }
  }

  private accept(value: unknown): WebSession {
    const dto = value as { session?: WebSession; csrf_token?: string } | null;
    const session = dto?.session;
    if (!session || session.client_type !== 'web' || typeof session.id !== 'string' ||
        typeof session.administrator?.username !== 'string' || typeof session.device_name !== 'string' ||
        typeof session.idle_expires_at !== 'string' || typeof session.absolute_expires_at !== 'string' ||
        !Number.isFinite(Date.parse(session.idle_expires_at)) ||
        !Number.isFinite(Date.parse(session.absolute_expires_at)) ||
        typeof dto?.csrf_token !== 'string' || !dto.csrf_token) {
      this.clear();
      throw new RequestError(502, 'INVALID_RESPONSE');
    }
    this.csrf = dto.csrf_token;
    // 只保留本页需要的元数据，不传播未知响应字段。
    return { id: session.id, administrator: { username: session.administrator.username },
      client_type: 'web', device_name: session.device_name,
      idle_expires_at: session.idle_expires_at, absolute_expires_at: session.absolute_expires_at };
  }

  async restore(): Promise<WebSession> { return this.accept(await this.request('session', 'GET')); }
  async login(username: string, password: string, deviceName: string): Promise<WebSession> {
    return this.accept(await this.request('login', 'POST', JSON.stringify({
      username, password, client_type: 'web', device_name: deviceName,
    })));
  }
  async logout(): Promise<void> { await this.request('logout', 'POST'); }
}
