import { AuthClient } from '../auth/client.js';
import { RequestError } from '../shared/errors.js';

export interface DeviceSession {
  id: string;
  administrator: { username: string };
  client_type: 'web' | 'android';
  device_id: string;
  device_name: string;
  idle_expires_at: string;
  absolute_expires_at: string;
}
export interface DevicePage { items: DeviceSession[]; next_cursor: string | null; has_more: boolean }

export class DeviceClient {
  constructor(private readonly auth: AuthClient) {}
  async list(cursor = ''): Promise<DevicePage> {
    const query = new URLSearchParams({ limit: '50' });
    if (cursor) query.set('cursor', cursor);
    const value = await this.auth.listDevices(query) as DevicePage | null;
    const invalid = () => new RequestError(502, 'INVALID_RESPONSE');
    if (!value || !Array.isArray(value.items) || typeof value.has_more !== 'boolean' ||
        (value.next_cursor !== null && typeof value.next_cursor !== 'string') ||
        (value.has_more ? !value.next_cursor || value.next_cursor === cursor : value.next_cursor !== null)) throw invalid();
    const items = value.items.map(item => {
      if (!item || typeof item.id !== 'string' || !item.id ||
          typeof item.administrator?.username !== 'string' ||
          !['web', 'android'].includes(item.client_type) || typeof item.device_id !== 'string' ||
          typeof item.device_name !== 'string' || typeof item.idle_expires_at !== 'string' ||
          typeof item.absolute_expires_at !== 'string' || !Number.isFinite(Date.parse(item.idle_expires_at)) ||
          !Number.isFinite(Date.parse(item.absolute_expires_at))) throw invalid();
      // 七字段白名单；未知字段与认证凭据不能穿过本页边界。
      return { id: item.id, administrator: { username: item.administrator.username },
        client_type: item.client_type, device_id: item.device_id, device_name: item.device_name,
        idle_expires_at: item.idle_expires_at, absolute_expires_at: item.absolute_expires_at };
    });
    if (new Set(items.map(item => item.id)).size !== items.length) throw invalid();
    return { items, next_cursor: value.next_cursor, has_more: value.has_more };
  }
  async revoke(id: string): Promise<void> { await this.auth.revokeDevice(id); }
}
