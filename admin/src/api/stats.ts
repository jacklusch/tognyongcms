import { request } from './client'
import type { ContentEntry } from './content'

export interface TypeStat {
  type_name: string
  published: number
  draft: number
}
export interface StatsData {
  by_type: TypeStat[]
  recent: ContentEntry[]
}
export const fetchStats = () => request<StatsData>('/stats')
