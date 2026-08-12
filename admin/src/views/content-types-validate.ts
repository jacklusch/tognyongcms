import type { SchemaField } from '../dynamic-form/types'

export interface TypeDraft {
  name: string
  label: string
  fields: SchemaField[]
}

export function validateContentType(draft: TypeDraft): string | null {
  if (!/^[a-zA-Z][a-zA-Z0-9_]*$/.test(draft.name)) return '类型名称需以字母开头且只含字母数字下划线'
  if (!draft.label) return '请输入类型标签'
  const seen = new Set<string>()
  for (const f of draft.fields) {
    if (!/^[a-zA-Z][a-zA-Z0-9_]*$/.test(f.name)) return `字段名 ${f.name} 不合法`
    if (seen.has(f.name)) return `字段名 ${f.name} 重复`
    seen.add(f.name)
    if (f.type === 'relation' && !f.relation_type) return `字段 ${f.name} 必须选择目标内容类型`
    if (f.type === 'repeat' && (!f.sub_fields || f.sub_fields.length === 0)) return `字段 ${f.name} 必须配置子字段`
  }
  return null
}
