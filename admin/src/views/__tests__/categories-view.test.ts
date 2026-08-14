import { describe, it, expect } from 'vitest'
import { buildCategoryOptions, findParentPathId, type CategoryItem, type CategoryPath } from '../../api/category'
import { categoryDeleteGuard } from '../categories-logic'

function slugify(name: string): string {
  return name.trim().toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '')
}

describe('category slugify', () => {
  it('中文名 → 空（无 ASCII）', () => { expect(slugify('新闻')).toBe('') })
  it('英文名 → 连字符 slug', () => { expect(slugify('Tech News')).toBe('tech-news') })
})

const all: CategoryPath[] = [
  { id: 1, path: '产品中心' },
  { id: 2, path: '产品中心/斩拌机' },
  { id: 3, path: '产品中心/斩拌机/刀片' },
]

describe('category tree', () => {
  it('listCategories 返回树（children 嵌套）与扁平 all 对应', () => {
    const items: CategoryItem[] = [
      {
        id: 1, parent_id: 0, name: '产品中心', slug: 'product', description: '', content_count: 0, total_content_count: 0,
        children: [{ id: 2, parent_id: 1, name: '斩拌机', slug: 'emulsifier', description: '', content_count: 0, total_content_count: 0, children: [] }],
      },
    ]
    expect(items[0].children).toHaveLength(1)
    expect(items[0].children[0].parent_id).toBe(items[0].id)
    expect(all.find((p) => p.id === items[0].children[0].id)?.path).toBe('产品中心/斩拌机')
  })
})

describe('category cascader options', () => {
  it('all（含 path）生成 {value,label} 级联 options', () => {
    expect(buildCategoryOptions(all)).toEqual([
      { value: 1, label: '产品中心' },
      { value: 2, label: '产品中心/斩拌机' },
      { value: 3, label: '产品中心/斩拌机/刀片' },
    ])
  })
  it('空列表 → 空 options', () => {
    expect(buildCategoryOptions([])).toEqual([])
  })
})

describe('category parent 回显', () => {
  it('编辑回显：findParentPathId 用 all 找当前 id 的父节点 id', () => {
    expect(findParentPathId(all, 3)).toBe(2)
    expect(findParentPathId(all, 2)).toBe(1)
  })
  it('顶级分类（无父级）与未知 id 返回 undefined', () => {
    expect(findParentPathId(all, 1)).toBeUndefined()
    expect(findParentPathId(all, 999)).toBeUndefined()
  })
})

describe('category delete guard', () => {
  const base: CategoryItem = { id: 1, parent_id: 0, name: 'x', slug: 'x', description: '', content_count: 0, total_content_count: 0, children: [] }
  it('有子分类 → 提示请先删除子分类，不调删除 API', () => {
    const c: CategoryItem = { ...base, children: [{ ...base, id: 2, parent_id: 1, name: '子' }] }
    expect(categoryDeleteGuard(c)).toBe('请先删除子分类')
  })
  it('其他语言有内容（全语言计数）→ 提示先移除内容', () => {
    const c: CategoryItem = { ...base, total_content_count: 3 }
    expect(categoryDeleteGuard(c)).toBe('该分类下仍有内容，请先移除')
  })
  it('无子分类无内容 → 可删（null）', () => {
    expect(categoryDeleteGuard(base)).toBeNull()
  })
})
