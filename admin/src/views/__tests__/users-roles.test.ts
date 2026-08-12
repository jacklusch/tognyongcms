import { describe, it, expect } from 'vitest'
// 纯逻辑测试（权限解析），避免复杂组件挂载
function parsePerms(raw: string): string[] {
  try { const arr = JSON.parse(raw); return Array.isArray(arr) ? arr : [] } catch { return [] }
}

describe('roles perms parsing', () => {
  it('解析权限 JSON', () => {
    expect(parsePerms('["content.read","content.write"]')).toEqual(['content.read', 'content.write'])
  })
  it('非法 JSON 返回空', () => {
    expect(parsePerms('nope')).toEqual([])
  })
})
