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
  current: Record<string, unknown>,
): TabSwitchResult {
  const entry = byLang[lang]
  if (entry) {
    return { form: { ...shared, ...entry.fields }, needCreate: false }
  }
  return { form: { ...shared, ...current }, needCreate: true }
}
