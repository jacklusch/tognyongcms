import { request } from './client'

export interface UserItem {
  id: number
  username: string
  role_id: number
}
export interface RoleItem {
  id: number
  name: string
  permissions: string
}

export const listUsers = () => request<{ items: UserItem[] }>('/users')
export const createUser = (body: { username: string; password: string; role_id: number }) =>
  request<{ user: UserItem }>('/users', { method: 'POST', body })
export const deleteUser = (id: number) => request<{ deleted: number }>(`/users/${id}`, { method: 'DELETE' })
export const updateUserRole = (id: number, role_id: number) =>
  request<{ id: number }>(`/users/${id}/role`, { method: 'PUT', body: { role_id } })
export const updateUserPassword = (id: number, password: string, old_password?: string) =>
  request<{ id: number }>(`/users/${id}/password`, { method: 'PUT', body: { password, old_password } })
