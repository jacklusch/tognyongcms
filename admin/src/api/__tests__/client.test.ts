import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { request } from '../client'
import { useAuthStore } from '../../stores/auth'

describe('client request', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
  })

  it('带 token 并解析 data', async () => {
    const auth = useAuthStore()
    auth.setToken('tok-1')
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ code: 200, data: { ok: 1 } }),
    }) as any
    const data = await request<{ ok: number }>('/x')
    expect(data.ok).toBe(1)
    const [, init] = (globalThis.fetch as any).mock.calls[0]
    expect(init.headers.Authorization).toBe('Bearer tok-1')
  })

  it('401 清除 token', async () => {
    const auth = useAuthStore()
    auth.setToken('tok-1')
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 401,
      json: () => Promise.resolve({ code: 401, message: '未登录' }),
    }) as any
    await expect(request('/x')).rejects.toMatchObject({ code: 401 })
    expect(auth.token).toBeNull()
  })
})
