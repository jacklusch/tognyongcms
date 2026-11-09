import type { CategoryItem } from '../api/category'

export function categoryDeleteGuard(c: CategoryItem): string | null {
  if (c.total_content_count > 0) return '该分类下仍有内容，请先移除'
  if (c.children?.length) return '请先删除子分类'
  return null
}
