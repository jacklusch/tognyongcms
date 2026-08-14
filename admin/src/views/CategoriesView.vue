<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { listCategories, createCategory, updateCategory, deleteCategory, buildCategoryOptions, findParentPathId, type CategoryItem, type CategoryPath } from '../api/category'
import { fetchMeta, type MetaData } from '../api/meta'
import { categoryDeleteGuard } from './categories-logic'

const meta = ref<MetaData | null>(null)
const langFilter = ref('')
const items = ref<CategoryItem[]>([])
const all = ref<CategoryPath[]>([])
const dialogVisible = ref(false)
const editing = ref<CategoryItem | null>(null)
const form = ref<{ name: string; slug: string; description: string; parent_id?: number }>({ name: '', slug: '', description: '' })

const cascadeOptions = computed(() => buildCategoryOptions(all.value))
const cascadeProps = { checkStrictly: true, emitPath: false }

function slugify(name: string): string {
  return name.trim().toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '')
}

async function load() {
  const r = await listCategories(langFilter.value || undefined)
  items.value = r.items
  all.value = r.all
}

onMounted(async () => {
  meta.value = await fetchMeta()
  langFilter.value = meta.value.default_lang
  await load()
})

function openCreate(row?: CategoryItem) {
  editing.value = null
  form.value = { name: '', slug: '', description: '', parent_id: row?.id }
  dialogVisible.value = true
}

function openEdit(c: CategoryItem) {
  editing.value = c
  form.value = {
    name: c.name,
    slug: c.slug,
    description: c.description,
    parent_id: findParentPathId(all.value, c.id),
  }
  dialogVisible.value = true
}

function onNameInput() {
  if (!editing.value && !form.value.slug) {
    form.value.slug = slugify(form.value.name)
  }
}

function onParentChange(v: unknown) {
  form.value.parent_id = typeof v === 'number' ? v : undefined
}

async function save() {
  if (!form.value.name || !form.value.slug) { ElMessage.warning('请填写名称和 slug'); return }
  try {
    const body = {
      name: form.value.name,
      slug: form.value.slug,
      description: form.value.description,
      parent_id: form.value.parent_id,
    }
    if (editing.value) await updateCategory(editing.value.id, body)
    else await createCategory(body)
    ElMessage.success('已保存')
    dialogVisible.value = false
    await load()
  } catch (e: any) { ElMessage.error(e.message ?? '保存失败') }
}

async function remove(c: CategoryItem) {
  const guard = categoryDeleteGuard(c)
  if (guard) { ElMessage.warning(guard); return }
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
    <el-form inline style="margin-top: 8px">
      <el-form-item label="语言">
        <el-select v-model="langFilter" style="width: 120px" @change="load">
          <el-option v-for="l in meta?.languages ?? []" :key="l" :label="l" :value="l" />
        </el-select>
      </el-form-item>
    </el-form>
    <el-button type="primary" @click="openCreate()">新建分类</el-button>
    <el-table :data="items" row-key="id" :tree-props="{ children: 'children' }" style="margin-top: 16px">
      <el-table-column prop="name" label="名称" width="200" />
      <el-table-column prop="slug" label="Slug" width="160" />
      <el-table-column prop="description" label="描述" />
      <el-table-column prop="content_count" label="内容数" width="90" />
      <el-table-column prop="total_content_count" label="总内容数" width="90" />
      <el-table-column label="操作" width="240">
        <template #default="{ row }">
          <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
          <el-button link type="primary" @click="openCreate(row)">添加子分类</el-button>
          <el-button link type="danger" @click="remove(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="dialogVisible" :title="editing ? '编辑分类' : '新建分类'" width="480px">
      <el-form label-width="70px">
        <el-form-item label="上级分类">
          <el-cascader
            :model-value="form.parent_id"
            :options="cascadeOptions"
            :props="cascadeProps"
            clearable
            filterable
            placeholder="不选则为顶级分类"
            style="width: 100%"
            @update:model-value="onParentChange"
          />
        </el-form-item>
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
