import { describe, it, expect } from 'vitest'
import { resolveSwitchTab } from '../content-edit-tabs'

const byLang = {
  en: { fields: { title: 'Hello', body: 'world' } },
}
const shared = { slug: 'hello', type: 'article' }

describe('resolveSwitchTab', () => {
  it('切到未翻译语言置 needCreate=true，表单从空白开始（不带当前语言内容）', () => {
    const r = resolveSwitchTab(byLang, 'zh', shared)
    expect(r.needCreate).toBe(true)
    expect(r.form).toEqual({ ...shared })
  })

  it('切回已翻译语言复位 needCreate=false 并载入该语言字段', () => {
    const r = resolveSwitchTab(byLang, 'en', shared)
    expect(r.needCreate).toBe(false)
    expect(r.form).toEqual({ ...shared, title: 'Hello', body: 'world' })
  })
})
