import { request } from './client'
import { type RoleItem } from './user'

export type { RoleItem }

export const listRoles = () => request<{ items: RoleItem[] }>('/roles')
export const listPerms = () => request<{ perms: string[] }>('/roles/perms')
export const createRole = (body: { name: string; permissions: string[] }) =>
  request<{ role: RoleItem }>('/roles', { method: 'POST', body })
export const updateRole = (id: number, body: { name: string; permissions: string[] }) =>
  request<{ id: number }>(`/roles/${id}`, { method: 'PUT', body })
export const deleteRole = (id: number) => request<{ deleted: number }>(`/roles/${id}`, { method: 'DELETE' })
