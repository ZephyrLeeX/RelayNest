/** 只显示本地允许的文案；服务端 message、原始响应和凭据不进入界面或日志。 */
export class RequestError extends Error {
  constructor(readonly status: number, readonly code: string) {
    super('请求未完成');
  }
}

export function errorMessage(error: unknown, login = false): string {
  if (!(error instanceof RequestError)) return '网络连接失败，请检查连接后重试。';
  if (error.status === 401) return login ? '用户名或密码错误。' : '会话已过期或已撤销，请重新登录。';
  if (error.status === 403) return '安全校验失败，请重新检查会话后重试；请从同源管理地址访问。';
  if (error.status === 429) return '请求过于频繁，请稍后重试。';
  if (error.status === 503) return '服务暂时不可用，请稍后重试。';
  if (error.status === 400 || error.status === 422) return '输入无效，请检查后重试。';
  return '请求未完成，请稍后重试。';
}
