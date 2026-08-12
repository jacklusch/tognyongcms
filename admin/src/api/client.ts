import { useAuthStore } from '../stores/auth'

const BASE = '/api'

export interface ApiError {
  code: number
  message: string
}

export async function request<T>(path: string, opts: { method?: string; body?: unknown } = {}): Promise<T> {
  const auth = useAuthStore()
  const headers: Record<string, string> = {}
  if (opts.body !== undefined) {
    headers['Content-Type'] = 'application/json'
  }
  const token = auth.token
  if (token) {
    headers['Authorization'] = `Bearer ${token}`
  }
  const res = await fetch(`${BASE}${path}`, {
    method: opts.method ?? 'GET',
    headers,
    body: opts.body !== undefined ? JSON.stringify(opts.body) : undefined,
  })
  const data = await res.json().catch(() => ({ code: res.status, message: '响应解析失败' }))
  if (!res.ok) {
    if (res.status === 401) {
      auth.clear()
      // 跳登录
      const loginPath = import.meta.env.DEV ? '/login' : '/admin/login'
      if (window.location.pathname !== loginPath) {
        window.location.href = loginPath
      }
    }
    const err: ApiError = { code: data.code ?? res.status, message: data.message ?? '请求失败' }
    throw err
  }
  return data.data as T
}
