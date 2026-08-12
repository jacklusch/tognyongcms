export type FieldType =
  | 'text' | 'textarea' | 'richtext' | 'number' | 'boolean' | 'date' | 'datetime'
  | 'select' | 'multiselect' | 'image' | 'file' | 'slug'
  | 'relation' | 'repeat'

export interface SchemaField {
  name: string
  label: string
  type: FieldType
  required?: boolean
  indexed?: boolean
  translatable?: boolean
  default?: unknown
  options?: string[]
  max_length?: number
  pattern?: string
  min?: number | null
  max?: number | null
  relation_type?: string
  sub_fields?: SchemaField[]
}

export type FormValues = Record<string, unknown>
