import { describe, it, expect, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useAuthStore } from '../auth'

describe('auth store hasPerm', () => {
  beforeEach(() => { setActivePinia(createPinia()); localStorage.clear() })
  it('前缀匹配', () => {
    const s = useAuthStore()
    s.setPermissions(['content.*'])
    expect(s.hasPerm('content.read.article')).toBe(true)
    expect(s.hasPerm('content.read.page')).toBe(true)
  })
  it('admin *', () => {
    const s = useAuthStore()
    s.setPermissions(['*'])
    expect(s.hasPerm('anything')).toBe(true)
  })
  it('无匹配', () => {
    const s = useAuthStore()
    s.setPermissions(['menus.manage'])
    expect(s.hasPerm('content.write.article')).toBe(false)
  })
})
