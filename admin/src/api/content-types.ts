import { request } from './client'
import type { SchemaField } from '../dynamic-form/types'

export interface ContentTypeItem {
  id: number
  name: string
  label: string
  fields: SchemaField[]
}
export const listContentTypes = () => request<{ items: ContentTypeItem[] }>('/content-types')
export const createContentType = (body: { name: string; label: string; fields: SchemaField[] }) =>
  request<{ content_type: ContentTypeItem }>('/content-types', { method: 'POST', body })
export const updateContentType = (id: number, body: { label: string; fields: SchemaField[] }) =>
  request<{ content_type: ContentTypeItem }>(`/content-types/${id}`, { method: 'PUT', body })
export const deleteContentType = (name: string) => request<{ deleted: string }>(`/content-types/${name}`, { method: 'DELETE' })
