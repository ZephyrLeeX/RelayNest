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
  private generation = 0;
  constructor(private readonly transport: typeof fetch = fetch.bind(globalThis)) {}

  clear(): void { this.csrf = ''; this.generation++; }

  private async request(path: string, method: 'GET' | 'POST' | 'DELETE', body?: string): Promise<unknown> {
    const generation = this.generation;
    const headers: Record<string, string> = {};
    if (body !== undefined) headers['Content-Type'] = 'application/json';
    if (path === 'logout' || method === 'DELETE') headers['X-CSRF-Token'] = this.csrf;
    const response = await this.transport(path.startsWith('/api/v1/devices') ? path : `/api/v1/auth/${path}`, {
      method, headers, credentials: 'same-origin', cache: 'no-store', redirect: 'error',
      ...(body === undefined ? {} : { body }),
    });
    if (!response.ok) {
      if (response.status === 401 && generation === this.generation) this.clear();
      let code = '';
      try {
        const dto = await response.json();
        if (typeof dto?.error?.code === 'string') code = dto.error.code;
      } catch { /* 非 JSON 错误仍由 HTTP 状态映射安全文案。 */ }
      throw new RequestError(response.status, code);
    }
    if (response.status === 204 && (path === 'logout' || method === 'DELETE')) {
      if (path === 'logout' && generation === this.generation) this.clear();
      return;
    }
    if (path === 'logout' || method === 'DELETE' || response.status !== 200) throw new RequestError(502, 'INVALID_RESPONSE');
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
    // 新认证代次使旧设备请求的迟到401不能清除刚恢复的凭据。
    this.generation++;
    this.csrf = dto.csrf_token;
    // 只保留本页需要的元数据，不传播未知响应字段。
    return { id: session.id, administrator: { username: session.administrator.username },
      client_type: 'web', device_name: session.device_name,
      idle_expires_at: session.idle_expires_at, absolute_expires_at: session.absolute_expires_at };
  }

  async restore(): Promise<WebSession> {
    const generation = this.generation;
    const dto = await this.request('session', 'GET');
    if (generation !== this.generation) throw new RequestError(401, 'AUTHENTICATION_FAILED');
    return this.accept(dto);
  }
  async login(username: string, password: string, deviceName: string): Promise<WebSession> {
    const generation = this.generation;
    const dto = await this.request('login', 'POST', JSON.stringify({
      username, password, client_type: 'web', device_name: deviceName,
    }));
    if (generation !== this.generation) throw new RequestError(401, 'AUTHENTICATION_FAILED');
    return this.accept(dto);
  }
  /** 设备请求仅复用当前 Cookie 与私有 CSRF，不导出凭据。 */
  async listDevices(query: URLSearchParams): Promise<unknown> {
    return this.request(`/api/v1/devices?${query}`, 'GET');
  }
  async revokeDevice(id: string): Promise<void> {
    await this.request(`/api/v1/devices/${encodeURIComponent(id)}/session`, 'DELETE');
  }
  async logout(): Promise<void> { await this.request('logout', 'POST'); }
}
