import { request } from './client'

export interface MetaData {
  site_name: string
  site_url: string
  description: string
  languages: string[]
  default_lang: string
  theme: string
}

export const fetchMeta = () => request<MetaData>('/meta')
