import { describe, it, expect } from 'vitest'

function slugify(name: string): string {
  return name.trim().toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '')
}

describe('category slugify', () => {
  it('中文名 → 空（无 ASCII）', () => { expect(slugify('新闻')).toBe('') })
  it('英文名 → 连字符 slug', () => { expect(slugify('Tech News')).toBe('tech-news') })
})
