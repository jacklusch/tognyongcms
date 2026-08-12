import { request } from './client'

export interface ContentEntry {
  content: {
    id: number
    content_type_id: number
    content_id: string
    lang: string
    slug: string
    title: string
    status: string
    created_by: number
    created_at: string
    updated_at: string
    published_at: string | null
    payload: string
  }
  type_name: string
  fields: Record<string, unknown>
}

export const listContent = (params: { type: string; lang: string; status?: string; page?: number; perPage?: number }) =>
  request<{ items: ContentEntry[]; total: number }>(`/content?type=${params.type}&lang=${params.lang}${params.status ? `&status=${params.status}` : ''}&page=${params.page ?? 1}&per_page=${params.perPage ?? 20}`)

export const getContent = (id: number) => request<{ content: ContentEntry }>(`/content/${id}`)
export const createContent = (type: string, lang: string, data: Record<string, unknown>) =>
  request<{ content: ContentEntry }>('/content', { method: 'POST', body: { type, lang, data } })
export const updateContent = (id: number, data: Record<string, unknown>) =>
  request<{ content: ContentEntry }>(`/content/${id}`, { method: 'PUT', body: { data } })
export const deleteContent = (id: number) => request<{ deleted: number }>(`/content/${id}`, { method: 'DELETE' })
export const publishContent = (id: number) => request<{ id: number; status: string }>(`/content/${id}/publish`, { method: 'POST' })
export const unpublishContent = (id: number) => request<{ id: number; status: string }>(`/content/${id}/unpublish`, { method: 'POST' })
export const listTranslations = (id: number) => request<{ items: ContentEntry[] }>(`/content/${id}/translations`)
export const createTranslation = (id: number, lang: string, data: Record<string, unknown>, typeName: string) =>
  request<{ content: ContentEntry }>(`/content/${id}/translate`, { method: 'POST', body: { type: typeName, lang, data } })
