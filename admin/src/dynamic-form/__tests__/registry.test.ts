import { describe, it, expect } from 'vitest'
import { validateText, validateNumber, validateSlug, validateForm } from '../registry'
import type { SchemaField } from '../types'

const f = (name: string, type: any, extra: Partial<SchemaField> = {}): SchemaField => ({
  name, label: name, type, ...extra,
})

describe('registry validators', () => {
  it('必填', () => {
    expect(validateText('', f('t', 'text', { required: true }))).toBeTruthy()
    expect(validateText('x', f('t', 'text', { required: true }))).toBeNull()
  })
  it('max_length', () => {
    expect(validateText('abc', f('t', 'text', { max_length: 2 }))).toBeTruthy()
  })
  it('number 范围', () => {
    expect(validateNumber(5, f('n', 'number', { min: 0, max: 10 }))).toBeNull()
    expect(validateNumber(-1, f('n', 'number', { min: 0 }))).toBeTruthy()
    expect(validateNumber('abc', f('n', 'number'))).toBeTruthy()
  })
  it('slug 格式', () => {
    expect(validateSlug('hello-world', f('s', 'slug'))).toBeNull()
    expect(validateSlug('Hello World', f('s', 'slug'))).toBeTruthy()
  })
  it('validateForm 汇总', () => {
    const fields = [f('a', 'text', { required: true }), f('b', 'number')]
    const errors = validateForm(fields, { a: '', b: 1 })
    expect(errors.a).toBeTruthy()
    expect(errors.b).toBeUndefined()
  })
})
