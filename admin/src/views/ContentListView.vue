<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { listContent, deleteContent, type ContentEntry } from '../api/content'
import { listContentTypes } from '../api/content-types'
import { listCategories, type CategoryItem } from '../api/category'
import { searchContent } from '../api/search'
import { fetchMeta, type MetaData } from '../api/meta'
import { useAuthStore } from '../stores/auth'

const router = useRouter()
const auth = useAuthStore()
const meta = ref<MetaData | null>(null)
const types = ref<{ name: string; label: string }[]>([])
const typeFilter = ref('')
const langFilter = ref('')
const statusFilter = ref('')
const categoryFilter = ref<number | undefined>(undefined)
const categories = ref<CategoryItem[]>([])
const keyword = ref('')

// 扁平化分类树（含子分类）为下拉选项
const categoryOptions = computed(() => {
  const out: { label: string; value: number }[] = []
  const walk = (list: CategoryItem[], prefix = '') => {
    for (const c of list) {
      const label = prefix ? `${prefix} / ${c.name}` : c.name
      out.push({ label, value: c.id })
      walk(c.children ?? [], label)
    }
  }
  walk(categories.value)
  return out
})
const items = ref<ContentEntry[]>([])
const total = ref(0)
const page = ref(1)
const perPage = 20
const loading = ref(false)
const searchTimer = ref<number | null>(null)

async function load() {
  // I1：类型为空（如 author 无可读类型）时不发请求，避免后端 400 报错，显示空态。
  if (!typeFilter.value) {
    items.value = []
    total.value = 0
    return
  }
  loading.value = true
  try {
    if (keyword.value) {
      const r = await searchContent({ type: typeFilter.value, lang: langFilter.value, q: keyword.value, category: categoryFilter.value, page: page.value })
      items.value = r.items
      total.value = r.total
    } else {
      const r = await listContent({ type: typeFilter.value, lang: langFilter.value, status: statusFilter.value || undefined, category: categoryFilter.value, page: page.value, perPage })
      items.value = r.items
      total.value = r.total
    }
  } finally {
    loading.value = false
  }
}

onMounted(async () => {
  meta.value = await fetchMeta()
  langFilter.value = meta.value.default_lang
  categories.value = (await listCategories(langFilter.value)).items
  const tr = await listContentTypes()
  types.value = tr.items.filter(t => auth.hasPerm(`content.read.${t.name}`))
  typeFilter.value = types.value[0]?.name ?? ''
  await load()
})

watch([typeFilter, langFilter, statusFilter, categoryFilter, page], () => load())
watch(langFilter, () => listCategories(langFilter.value).then(r => categories.value = r.items))

function onSearchInput() {
  if (searchTimer.value) window.clearTimeout(searchTimer.value)
  searchTimer.value = window.setTimeout(() => { page.value = 1; load() }, 400)
}

function edit(id?: number) {
  router.push(id ? `/content/${id}` : '/content/new')
}

const typeDialogVisible = ref(false)

function openNewDialog() {
  typeDialogVisible.value = true
}

function startNew(typeName: string) {
  typeDialogVisible.value = false
  router.push(`/content/new?type=${typeName}`)
}

async function remove(id: number) {
  try {
    await ElMessageBox.confirm('确认删除该内容？', '提示')
  } catch { return }
  await deleteContent(id)
  ElMessage.success('已删除')
  load()
}

function statusTag(s: string) {
  return s === 'published' ? 'success' : 'info'
}
</script>

<template>
  <div>
    <h2>内容管理</h2>
    <el-empty v-if="!types.length" description="当前账号无可读的内容类型" />
    <template v-else>
    <el-form inline>
      <el-form-item label="类型">
        <el-select v-model="typeFilter" style="width: 160px">
          <el-option v-for="t in types" :key="t.name" :label="t.label" :value="t.name" />
        </el-select>
      </el-form-item>
      <el-form-item label="语言">
        <el-select v-model="langFilter" style="width: 120px">
          <el-option v-for="l in meta?.languages ?? []" :key="l" :label="l" :value="l" />
        </el-select>
      </el-form-item>
      <el-form-item label="状态">
        <el-select v-model="statusFilter" style="width: 120px" clearable>
          <el-option label="草稿" value="draft" />
          <el-option label="已发布" value="published" />
        </el-select>
      </el-form-item>
      <el-form-item label="分类">
        <el-select v-model="categoryFilter" style="width: 180px" clearable placeholder="全部">
          <el-option v-for="o in categoryOptions" :key="o.value" :label="o.label" :value="o.value" />
        </el-select>
      </el-form-item>
      <el-form-item>
        <el-input v-model="keyword" placeholder="搜索标题/Slug" clearable @input="onSearchInput" />
      </el-form-item>
      <el-form-item>
        <el-button type="primary" @click="openNewDialog">新建</el-button>
      </el-form-item>
    </el-form>

    <el-table :data="items" v-loading="loading">
      <el-table-column prop="content.title" label="标题" />
      <el-table-column prop="content.slug" label="Slug" />
      <el-table-column prop="category_name" label="分类" width="140" />
      <el-table-column label="状态" width="100">
        <template #default="{ row }"><el-tag :type="statusTag(row.content.status)">{{ row.content.status }}</el-tag></template>
      </el-table-column>
      <el-table-column prop="content.lang" label="语言" width="80" />
      <el-table-column prop="content.updated_at" label="更新时间" width="180" />
      <el-table-column label="操作" width="140">
        <template #default="{ row }">
          <el-button link type="primary" @click="edit(row.content.id)">编辑</el-button>
          <el-button link type="danger" @click="remove(row.content.id)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-pagination
      v-model:current-page="page"
      :total="total"
      :page-size="perPage"
      layout="prev, pager, next"
      class="pager"
    />
    </template>

    <el-dialog v-model="typeDialogVisible" title="选择要添加的内容类型" width="420px">
      <div class="type-grid">
        <el-card
          v-for="t in types"
          :key="t.name"
          class="type-card"
          shadow="hover"
          @click="startNew(t.name)"
        >
          <div class="type-name">{{ t.label }}</div>
          <div class="type-code">{{ t.name }}</div>
        </el-card>
      </div>
    </el-dialog>
  </div>
</template>

<style scoped>
.pager { margin-top: 16px; justify-content: flex-end; }
.type-grid { display: grid; grid-template-columns: repeat(2, 1fr); gap: 12px; }
.type-card { cursor: pointer; text-align: center; }
.type-name { font-weight: 600; }
.type-code { color: var(--el-text-color-secondary); font-size: 12px; margin-top: 4px; }
</style>
