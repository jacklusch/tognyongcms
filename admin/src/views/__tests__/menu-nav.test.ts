import { describe, it, expect } from 'vitest'
import { emptyNavItem, normalizeNavItem } from '../menu-types'

describe('menu types', () => {
  it('emptyNavItem 结构', () => {
    const n = emptyNavItem()
    expect(n).toEqual({ label: '', type: 'home', url: '', children: [] })
  })

  it('normalizeNavItem 归一化旧格式 {label,url}', () => {
    const n = normalizeNavItem({ label: '关于', url: '/page/about' })
    expect(n).toEqual({ label: '关于', type: 'custom', url: '/page/about', children: [] })
  })

  it('normalizeNavItem 递归归一化子项并回填缺字段', () => {
    const n = normalizeNavItem({ label: '父', children: [{ label: '子' }, { label: '孙', children: [{ url: '/x' }] }] })
    expect(n.type).toBe('custom')
    expect(n.children).toHaveLength(2)
    expect(n.children[0]).toEqual({ label: '子', type: 'custom', url: '', children: [] })
    expect(n.children[1].children[0]).toEqual({ label: '', type: 'custom', url: '/x', children: [] })
  })

  it('normalizeNavItem children 非数组按空处理', () => {
    expect(normalizeNavItem({ label: 'a', children: 'oops' }).children).toEqual([])
  })
})
