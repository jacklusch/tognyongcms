import { request } from './client'

export interface CategoryItem {
  id: number
  parent_id: number
  name: string
  name_en: string
  slug: string
  description: string
  content_count: number
  total_content_count: number
  children: CategoryItem[]
}

export interface CategoryPath { id: number; path: string }

export interface CategoryCascaderOption {
  value: number
  label: string
}

export const listCategories = (lang?: string) =>
  request<{ items: CategoryItem[]; all: CategoryPath[] }>(`/categories${lang ? `?lang=${lang}` : ''}`)
export const createCategory = (body: { name: string; name_en?: string; slug?: string; description?: string; parent_id?: number }) =>
  request<{ category: CategoryItem }>('/categories', { method: 'POST', body })
export const updateCategory = (id: number, body: { name: string; name_en?: string; slug: string; description?: string; parent_id?: number }) =>
  request<{ category: CategoryItem }>(`/categories/${id}`, { method: 'PUT', body })
export const deleteCategory = (id: number) => request<{ deleted: number }>(`/categories/${id}`, { method: 'DELETE' })

export function buildCategoryOptions(all: CategoryPath[]): CategoryCascaderOption[] {
  return all.map((p) => ({ value: p.id, label: p.path }))
}

export function findParentPathId(all: CategoryPath[], id: number): number | undefined {
  const cur = all.find((p) => p.id === id)
  if (!cur) return undefined
  const sep = cur.path.lastIndexOf('/')
  if (sep <= 0) return undefined
  const parentPath = cur.path.slice(0, sep)
  return all.find((p) => p.path === parentPath)?.id
}
