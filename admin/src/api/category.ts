import { request } from './client'

export interface CategoryItem {
  id: number
  name: string
  slug: string
  description: string
  content_count: number
}

export const listCategories = () => request<{ items: CategoryItem[] }>('/categories')
export const createCategory = (body: { name: string; slug: string; description?: string }) =>
  request<{ category: CategoryItem }>('/categories', { method: 'POST', body })
export const updateCategory = (id: number, body: { name: string; slug: string; description?: string }) =>
  request<{ category: CategoryItem }>(`/categories/${id}`, { method: 'PUT', body })
export const deleteCategory = (id: number) => request<{ deleted: number }>(`/categories/${id}`, { method: 'DELETE' })
