import { request } from './client'

export const fetchSettings = () => request<{ settings: Record<string, string> }>('/settings')
export const updateSettings = (data: Record<string, string>) =>
  request<{ settings: Record<string, string> }>('/settings', { method: 'PUT', body: data })
