// src/api/media.ts
import { useAuthStore } from '../stores/auth'
import { request } from './client'

const BASE = '/api'

export interface MediaItem {
  id: number
  filename: string
  url: string
  mime: string
  size: number
  created_at: string
}

export const listMedia = (page = 1, perPage = 50) =>
  request<{ items: MediaItem[] }>(`/media?page=${page}&per_page=${perPage}`)

// uploadMedia 走独立 fetch：FormData 由 fetch 自动带 multipart boundary，
// 不能走 client.ts 的 JSON 序列化逻辑。
export const uploadMedia = async (file: File): Promise<{ key: string; url: string }> => {
  const fd = new FormData()
  fd.append('file', file)
  const auth = useAuthStore()
  const headers: Record<string, string> = {}
  if (auth.token) {
    headers['Authorization'] = `Bearer ${auth.token}`
  }
  const res = await fetch(`${BASE}/media/upload`, { method: 'POST', headers, body: fd })
  const data = await res.json().catch(() => ({ code: res.status, message: '响应解析失败' }))
  if (!res.ok) {
    throw { code: data.code ?? res.status, message: data.message ?? '上传失败' }
  }
  return data.data as { key: string; url: string }
}

export const deleteMedia = (id: number) => request<{ deleted: number }>(`/media/${id}`, { method: 'DELETE' })
