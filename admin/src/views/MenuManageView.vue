<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { listMenus, createMenu, updateMenu, deleteMenu, type MenuItem } from '../api/menu'
import { fetchMeta, type MetaData } from '../api/meta'
import { listContentTypes } from '../api/content-types'
import NavItemEditor from '../components/NavItemEditor.vue'
import { emptyNavItem, normalizeNavItem, type NavItem } from './menu-types'

const meta = ref<MetaData | null>(null)
const lang = ref('zh')
const items = ref<MenuItem[]>([])
const editing = ref<MenuItem | null>(null)
const dialogVisible = ref(false)
const formName = ref('')
const navItems = ref<NavItem[]>([])
const types = ref<string[]>([])

function parseItems(raw: string): NavItem[] {
  try {
    const arr = JSON.parse(raw || '[]')
    return Array.isArray(arr) ? arr.map(normalizeNavItem) : []
  } catch {
    return []
  }
}

async function load() {
  const r = await listMenus(lang.value)
  items.value = r.items
}

onMounted(async () => {
  meta.value = await fetchMeta()
  lang.value = meta.value.default_lang
  const tr = await listContentTypes()
  types.value = tr.items.map(t => t.name)
  await load()
})

function openCreate() {
  editing.value = null
  formName.value = ''
  navItems.value = [emptyNavItem()]
  dialogVisible.value = true
}

function openEdit(m: MenuItem) {
  editing.value = m
  formName.value = m.name
  navItems.value = parseItems(m.items)
  dialogVisible.value = true
}

function updateNavItem(i: number, item: NavItem) {
  const next = [...navItems.value]
  next[i] = item
  navItems.value = next
}

function removeNavItem(i: number) {
  navItems.value = navItems.value.filter((_, idx) => idx !== i)
}

function addTopItem() {
  navItems.value = [...navItems.value, emptyNavItem()]
}

async function save() {
  if (!formName.value) { ElMessage.warning('请输入菜单名称'); return }
  const payload = { name: formName.value, lang: lang.value, items: navItems.value }
  if (editing.value) await updateMenu(editing.value.id, payload)
  else await createMenu(payload)
  ElMessage.success('已保存')
  dialogVisible.value = false
  await load()
}

async function remove(m: MenuItem) {
  try { await ElMessageBox.confirm(`确认删除菜单 ${m.name}？`, '提示') } catch { return }
  await deleteMenu(m.id)
  await load()
}
</script>

<template>
  <div>
    <h2>菜单管理</h2>
    <div class="bar">
      <el-select v-model="lang" style="width: 120px" @change="load">
        <el-option v-for="l in meta?.languages ?? []" :key="l" :label="l" :value="l" />
      </el-select>
      <el-button type="primary" @click="openCreate">新建菜单</el-button>
    </div>
    <el-table :data="items">
      <el-table-column prop="name" label="名称" />
      <el-table-column prop="lang" label="语言" width="80" />
      <el-table-column label="导航项" min-width="200">
        <template #default="{ row }">
          <el-tag v-for="(it, i) in parseItems(row.items)" :key="i" size="small" style="margin-right: 4px">{{ it.label }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="160">
        <template #default="{ row }">
          <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
          <el-button link type="danger" @click="remove(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="dialogVisible" :title="editing ? '编辑菜单' : '新建菜单'" width="640px">
      <el-form label-width="80px">
        <el-form-item label="名称"><el-input v-model="formName" /></el-form-item>
        <el-form-item label="导航项">
          <div class="nav-list">
            <NavItemEditor
              v-for="(it, i) in navItems"
              :key="i"
              :item="it"
              :types="types"
              @update:item="updateNavItem(i, $event)"
              @remove="removeNavItem(i)"
            />
            <el-button size="small" @click="addTopItem">添加导航项</el-button>
          </div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="save">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.bar { display: flex; gap: 12px; align-items: center; margin-bottom: 16px; }
</style>
