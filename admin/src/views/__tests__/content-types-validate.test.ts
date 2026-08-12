import { describe, it, expect } from 'vitest'
import { validateContentType } from '../content-types-validate'

describe('validateContentType', () => {
  it('合法类型通过', () => {
    expect(validateContentType({ name: 'article', label: '文章', fields: [{ name: 'title', label: '标题', type: 'text' }] })).toBeNull()
  })
  it('名称格式', () => {
    expect(validateContentType({ name: '1bad', label: 'x', fields: [] })).toBeTruthy()
  })
  it('字段重名', () => {
    expect(validateContentType({ name: 'a', label: 'x', fields: [{ name: 't', label: '', type: 'text' }, { name: 't', label: '', type: 'text' }] })).toBeTruthy()
  })
  it('relation 缺目标', () => {
    expect(validateContentType({ name: 'a', label: 'x', fields: [{ name: 'r', label: '', type: 'relation' }] })).toBeTruthy()
  })
  it('repeat 缺子字段', () => {
    expect(validateContentType({ name: 'a', label: 'x', fields: [{ name: 'r', label: '', type: 'repeat' }] })).toBeTruthy()
  })
})
