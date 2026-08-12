import { describe, it, expect } from 'vitest'
import { buildCategoryOptions, type CategoryPath } from '../../api/category'

describe('category picker logic', () => {
  it('分类 id 转字符串用于选中', () => {
    const id = 5
    expect(String(id)).toBe('5')
  })

  it('级联数据由 all（含 path）生成；选中返回 id 字符串', () => {
    const all: CategoryPath[] = [
      { id: 1, path: '产品中心' },
      { id: 2, path: '产品中心/斩拌机' },
      { id: 3, path: '产品中心/斩拌机/刀片' },
    ]
    const options = buildCategoryOptions(all)
    expect(options).toEqual([
      { value: 1, label: '产品中心' },
      { value: 2, label: '产品中心/斩拌机' },
      { value: 3, label: '产品中心/斩拌机/刀片' },
    ])
    const leaf = options.find((o) => o.value === 3)!
    expect(String(leaf.value)).toBe('3')
  })
})
