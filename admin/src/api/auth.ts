import { request } from './client'

export interface MeResponse {
  user: { id: number; username: string; role: string }
  permissions: string[]
}

export const login = (username: string, password: string) =>
  request<{ token: string }>('/auth/login', { method: 'POST', body: { username, password } })
export const logout = () => request<{ logged_out: boolean }>('/auth/logout', { method: 'POST' })
export const me = () => request<MeResponse>('/auth/me')
