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
