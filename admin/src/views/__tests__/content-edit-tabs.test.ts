import { describe, it, expect } from 'vitest'
import { resolveSwitchTab } from '../content-edit-tabs'

const byLang = {
  en: { fields: { title: 'Hello', body: 'world' } },
}
const shared = { slug: 'hello', type: 'article' }

describe('resolveSwitchTab', () => {
  it('切到未翻译语言置 needCreate=true 并保留当前语言可翻译字段', () => {
    const current = { title: '中文草稿', slug: 'unused' }
    const r = resolveSwitchTab(byLang, 'zh', shared, current)
    expect(r.needCreate).toBe(true)
    expect(r.form).toEqual({ ...shared, title: '中文草稿', slug: 'unused' })
  })

  it('切回已翻译语言复位 needCreate=false 并载入该语言字段', () => {
    const current = { title: '中文草稿' }
    const r = resolveSwitchTab(byLang, 'en', shared, current)
    expect(r.needCreate).toBe(false)
    expect(r.form).toEqual({ ...shared, title: 'Hello', body: 'world' })
  })
})
