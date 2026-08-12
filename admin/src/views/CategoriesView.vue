<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { listCategories, createCategory, updateCategory, deleteCategory, type CategoryItem } from '../api/category'

const items = ref<CategoryItem[]>([])
const dialogVisible = ref(false)
const editing = ref<CategoryItem | null>(null)
const form = ref({ name: '', slug: '', description: '' })

function slugify(name: string): string {
  return name.trim().toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '')
}

async function load() {
  const r = await listCategories()
  items.value = r.items
}

onMounted(load)

function openCreate() {
  editing.value = null
  form.value = { name: '', slug: '', description: '' }
  dialogVisible.value = true
}

function openEdit(c: CategoryItem) {
  editing.value = c
  form.value = { name: c.name, slug: c.slug, description: c.description }
  dialogVisible.value = true
}

function onNameInput() {
  if (!editing.value && !form.value.slug) {
    form.value.slug = slugify(form.value.name)
  }
}

async function save() {
  if (!form.value.name || !form.value.slug) { ElMessage.warning('请填写名称和 slug'); return }
  try {
    if (editing.value) await updateCategory(editing.value.id, form.value)
    else await createCategory(form.value)
    ElMessage.success('已保存')
    dialogVisible.value = false
    await load()
  } catch (e: any) { ElMessage.error(e.message ?? '保存失败') }
}

async function remove(c: CategoryItem) {
  if (c.content_count > 0) {
    ElMessage.warning('该分类下仍有内容，请先移除')
    return
  }
  try { await ElMessageBox.confirm(`确认删除分类 ${c.name}？`, '提示') } catch { return }
  try {
    await deleteCategory(c.id)
    ElMessage.success('已删除')
    await load()
  } catch (e: any) { ElMessage.error(e.message ?? '删除失败') }
}
</script>

<template>
  <div>
    <h2>分类管理</h2>
    <el-button type="primary" @click="openCreate">新建分类</el-button>
    <el-table :data="items" style="margin-top: 16px">
      <el-table-column prop="name" label="名称" width="140" />
      <el-table-column prop="slug" label="Slug" width="160" />
      <el-table-column prop="description" label="描述" />
      <el-table-column prop="content_count" label="内容数" width="90" />
      <el-table-column label="操作" width="160">
        <template #default="{ row }">
          <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
          <el-button link type="danger" @click="remove(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="dialogVisible" :title="editing ? '编辑分类' : '新建分类'" width="440px">
      <el-form label-width="70px">
        <el-form-item label="名称"><el-input v-model="form.name" @input="onNameInput" /></el-form-item>
        <el-form-item label="Slug"><el-input v-model="form.slug" placeholder="小写字母数字连字符" /></el-form-item>
        <el-form-item label="描述"><el-input v-model="form.description" type="textarea" :rows="2" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="save">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>
