export interface TabEntry {
  fields: Record<string, unknown>
}

export interface TabSwitchResult {
  form: Record<string, unknown>
  needCreate: boolean
}

export function resolveSwitchTab(
  byLang: Record<string, TabEntry>,
  lang: string,
  shared: Record<string, unknown>,
): TabSwitchResult {
  const entry = byLang[lang]
  if (entry) {
    return { form: { ...shared, ...entry.fields }, needCreate: false }
  }
  // 目标语言尚无翻译：表单从共享字段（非可翻译）开始，不带当前语言已填内容
  return { form: { ...shared }, needCreate: true }
}

// orderLangTabs 后台内容编辑页语言标签的显示顺序：zh 排在 en 前，其余语言保持配置中的相对顺序追加在后。
export function orderLangTabs(languages: string[]): string[] {
  const rank = (l: string) => (l === 'zh' ? 0 : l === 'en' ? 1 : 2)
  return [...languages].sort((a, b) => rank(a) - rank(b))
}

// defaultEditorLang 编辑内容时的默认语言：优先编辑顺序中第一个已有翻译的语言，都没有则回退 fallback。
export function defaultEditorLang(langOrder: string[], available: string[], fallback: string): string {
  return langOrder.find((l) => available.includes(l)) ?? fallback
}
