import { request } from './client'

export interface MetaData {
  site_name: string
  site_url: string
  description: string
  languages: string[]
  default_lang: string
  theme: string
  translate_enabled: boolean
  translate_source_lang: string
  translate_target_lang: string
}

export const fetchMeta = () => request<MetaData>('/meta')
