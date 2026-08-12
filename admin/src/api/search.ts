import { request } from './client'
import type { ContentEntry } from './content'

export const searchContent = (params: { type: string; lang: string; q: string; page?: number }) =>
  request<{ items: ContentEntry[]; total: number }>(`/search?type=${params.type}&lang=${params.lang}&q=${encodeURIComponent(params.q)}&page=${params.page ?? 1}`)
