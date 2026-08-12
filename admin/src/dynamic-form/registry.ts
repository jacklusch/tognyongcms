import type { Component } from 'vue'
import type { SchemaField, FormValues, FieldType } from './types'
import TextControl from './controls/TextControl.vue'
import TextareaControl from './controls/TextareaControl.vue'
import NumberControl from './controls/NumberControl.vue'
import BooleanControl from './controls/BooleanControl.vue'
import DateControl from './controls/DateControl.vue'
import DateTimeControl from './controls/DateTimeControl.vue'
import SelectControl from './controls/SelectControl.vue'
import MultiSelectControl from './controls/MultiSelectControl.vue'
import SlugControl from './controls/SlugControl.vue'
import RichTextControl from './controls/RichTextControl.vue'
import ImageControl from './controls/ImageControl.vue'
import FileControl from './controls/FileControl.vue'
import RelationControl from './controls/RelationControl.vue'
import RepeatControl from './controls/RepeatControl.vue'

export interface FieldControl {
  component: Component
  validate: (v: unknown, f: SchemaField) => string | null
}

// 全部字段类型均已有专用控件
export const registry: Record<FieldType, FieldControl> = {
  text: { component: TextControl, validate: validateText },
  textarea: { component: TextareaControl, validate: validateText },
  richtext: { component: RichTextControl, validate: validateText },
  number: { component: NumberControl, validate: validateNumber },
  boolean: { component: BooleanControl, validate: () => null },
  date: { component: DateControl, validate: validateDate },
  datetime: { component: DateTimeControl, validate: validateDateTime },
  select: { component: SelectControl, validate: validateSelect },
  multiselect: { component: MultiSelectControl, validate: validateMultiSelect },
  image: { component: ImageControl, validate: validateText },
  file: { component: FileControl, validate: validateText },
  slug: { component: SlugControl, validate: validateSlug },
  relation: { component: RelationControl, validate: validateRelation },
  repeat: { component: RepeatControl, validate: validateRepeat },
}

function isEmpty(v: unknown): boolean {
  return v === undefined || v === null || v === ''
}

export function validateRequired(v: unknown, f: SchemaField): string | null {
  if (f.required && isEmpty(v)) return `请填写${f.label}`
  return null
}

export function validateText(v: unknown, f: SchemaField): string | null {
  const req = validateRequired(v, f)
  if (req) return req
  if (isEmpty(v)) return null
  const s = String(v)
  if (f.max_length && [...s].length > f.max_length) return `${f.label}超长，上限 ${f.max_length}`
  if (f.pattern) {
    const re = new RegExp(f.pattern)
    if (!re.test(s)) return `${f.label}格式不合法`
  }
  return null
}

export function validateSlug(v: unknown, f: SchemaField): string | null {
  const req = validateRequired(v, f)
  if (req) return req
  if (isEmpty(v)) return null
  if (!/^[a-z0-9-]+$/.test(String(v))) return `${f.label}必须是合法别名`
  return null
}

export function validateNumber(v: unknown, f: SchemaField): string | null {
  const req = validateRequired(v, f)
  if (req) return req
  if (isEmpty(v)) return null
  const n = Number(v)
  if (Number.isNaN(n)) return `${f.label}需要数字`
  if (f.min !== null && f.min !== undefined && n < f.min) return `${f.label}不能小于 ${f.min}`
  if (f.max !== null && f.max !== undefined && n > f.max) return `${f.label}不能大于 ${f.max}`
  return null
}

export function validateDate(v: unknown, f: SchemaField): string | null {
  const req = validateRequired(v, f)
  if (req) return req
  if (isEmpty(v)) return null
  if (!/^\d{4}-\d{2}-\d{2}$/.test(String(v))) return `${f.label}日期格式应为 YYYY-MM-DD`
  return null
}

export function validateDateTime(v: unknown, f: SchemaField): string | null {
  const req = validateRequired(v, f)
  if (req) return req
  if (isEmpty(v)) return null
  if (Number.isNaN(Date.parse(String(v)))) return `${f.label}时间格式不合法`
  return null
}

export function validateSelect(v: unknown, f: SchemaField): string | null {
  const req = validateRequired(v, f)
  if (req) return req
  if (isEmpty(v)) return null
  if (f.options && !f.options.includes(String(v))) return `${f.label}值不在选项内`
  return null
}

export function validateMultiSelect(v: unknown, f: SchemaField): string | null {
  const req = validateRequired(v, f)
  if (req) return req
  if (isEmpty(v)) return null
  if (!Array.isArray(v)) return `${f.label}需要字符串数组`
  return null
}

export function validateRelation(v: unknown, f: SchemaField): string | null {
  const req = validateRequired(v, f)
  if (req) return req
  if (isEmpty(v)) return null
  return null
}

export function validateRepeat(v: unknown, f: SchemaField): string | null {
  const req = validateRequired(v, f)
  if (req) return req
  if (isEmpty(v)) return null
  if (!Array.isArray(v)) return `${f.label}需要数组`
  const sub = f.sub_fields ?? []
  for (let i = 0; i < v.length; i++) {
    const row = v[i]
    const errs = validateForm(sub, (row ?? {}) as FormValues)
    const firstErr = Object.values(errs)[0]
    if (firstErr) return `${f.label}第 ${i + 1} 行: ${firstErr}`
  }
  return null
}

// 表单级校验：返回 {fieldName: error}
export function validateForm(fields: SchemaField[], values: FormValues): Record<string, string> {
  const errors: Record<string, string> = {}
  for (const f of fields) {
    const ctl = registry[f.type]
    if (!ctl) continue
    const err = ctl.validate(values[f.name], f)
    if (err) errors[f.name] = err
  }
  return errors
}
