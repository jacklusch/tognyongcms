export type NavType = 'home' | 'custom' | `type:${string}`

export interface NavItem {
  label: string
  type: NavType
  url: string
  children: NavItem[]
}

export function emptyNavItem(): NavItem {
  return { label: '', type: 'home', url: '', children: [] }
}

// normalizeNavItem 归一化阶段 5 旧格式 {label,url}（缺 type/children）及任意嵌套项。
export function normalizeNavItem(it: any): NavItem {
  return {
    label: it.label ?? '',
    type: it.type ?? 'custom',
    url: it.url ?? '',
    children: Array.isArray(it.children) ? it.children.map(normalizeNavItem) : [],
  }
}
