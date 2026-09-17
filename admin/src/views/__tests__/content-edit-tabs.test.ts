import { describe, it, expect } from 'vitest'
import { resolveSwitchTab, orderLangTabs, defaultEditorLang } from '../content-edit-tabs'

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

describe('orderLangTabs', () => {
  it('zh 排在 en 前', () => {
    expect(orderLangTabs(['en', 'zh'])).toEqual(['zh', 'en'])
  })

  it('其余语言保持配置中的相对顺序追加在后', () => {
    expect(orderLangTabs(['en', 'ja', 'zh'])).toEqual(['zh', 'en', 'ja'])
  })
})

describe('defaultEditorLang', () => {
  it('优先返回编辑顺序中第一个已有翻译的语言', () => {
    expect(defaultEditorLang(['zh', 'en'], ['en', 'zh'], 'en')).toBe('zh')
    expect(defaultEditorLang(['zh', 'en'], ['en'], 'en')).toBe('en')
  })

  it('无任何已有翻译时回退 fallback', () => {
    expect(defaultEditorLang(['zh', 'en'], [], 'en')).toBe('en')
  })
})
