import { request } from './client'
import type { ContentEntry } from './content'

export const searchContent = (params: { type: string; lang: string; q: string; category?: number; page?: number }) =>
  request<{ items: ContentEntry[]; total: number }>(
    `/search?type=${params.type}&lang=${params.lang}&q=${encodeURIComponent(params.q)}${params.category ? `&category=${params.category}` : ''}&page=${params.page ?? 1}`
  )
