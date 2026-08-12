import { describe, it, expect } from 'vitest'

// CategoryPicker 纯逻辑：selected 存分类 id 字符串
describe('category picker logic', () => {
  it('分类 id 转字符串用于选中', () => {
    const id = 5
    expect(String(id)).toBe('5')
  })
})
