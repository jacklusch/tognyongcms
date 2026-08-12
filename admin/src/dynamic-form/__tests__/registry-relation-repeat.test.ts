import { describe, it, expect } from 'vitest'
import { validateRelation, validateRepeat } from '../registry'
import type { SchemaField } from '../types'

const f = (name: string, type: any, extra: Partial<SchemaField> = {}): SchemaField => ({
  name, label: name, type, ...extra,
})

describe('relation/repeat validators', () => {
  it('relation 必填与值', () => {
    expect(validateRelation('', f('r', 'relation', { required: true }))).toBeTruthy()
    expect(validateRelation('grp-abc', f('r', 'relation'))).toBeNull()
  })
  it('repeat 数组与逐行子校验', () => {
    const rep = f('items', 'repeat', { required: true, sub_fields: [{ name: 'title', label: '标题', type: 'text', required: true } as SchemaField] })
    expect(validateRepeat('x', rep)).toBeTruthy()
    expect(validateRepeat([], rep)).toBeNull()
    expect(validateRepeat([{ title: '' }], rep)).toBeTruthy() // 行内缺必填
    expect(validateRepeat([{ title: 'ok' }], rep)).toBeNull()
  })
})
