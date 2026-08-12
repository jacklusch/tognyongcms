import { request } from './client'

export interface MenuItem {
  id: number
  name: string
  lang: string
  items: string
}
export const listMenus = (lang: string) => request<{ items: MenuItem[] }>(`/menus?lang=${lang}`)
export const createMenu = (body: { name: string; lang: string; items: unknown }) =>
  request<{ menu: MenuItem }>('/menus', { method: 'POST', body })
export const updateMenu = (id: number, body: { name?: string; lang?: string; items?: unknown }) =>
  request<{ menu: MenuItem }>(`/menus/${id}`, { method: 'PUT', body })
export const deleteMenu = (id: number) => request<{ deleted: number }>(`/menus/${id}`, { method: 'DELETE' })
