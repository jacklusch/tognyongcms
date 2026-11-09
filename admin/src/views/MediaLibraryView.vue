<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { listMedia, uploadMedia, deleteMedia, batchDeleteMedia, type MediaItem } from '../api/media'

const items = ref<MediaItem[]>([])
const loading = ref(false)
const uploading = ref(false)
const page = ref(1)
const perPage = ref(20)
const total = ref(0)
const selectedIds = ref<number[]>([])

async function load() {
  loading.value = true
  try {
    const r = await listMedia(page.value, perPage.value)
    items.value = r.items ?? []
    total.value = r.total ?? 0
  } catch (e: any) {
    items.value = []
    total.value = 0
    ElMessage.error(e.message ?? '加载媒体列表失败')
  } finally {
    loading.value = false
  }
}

function onPageChange(p: number) {
  page.value = p
  selectedIds.value = []
  load()
}

const allSelected = computed(() => items.value.length > 0 && items.value.every((m) => selectedIds.value.includes(m.id)))
const someSelected = computed(() => selectedIds.value.length > 0 && !allSelected.value)

function toggleAll(checked: unknown) {
  selectedIds.value = checked ? items.value.map((m) => m.id) : []
}

function toggleOne(id: number, checked: unknown) {
  if (checked) {
    if (!selectedIds.value.includes(id)) selectedIds.value = [...selectedIds.value, id]
  } else {
    selectedIds.value = selectedIds.value.filter((x) => x !== id)
  }
}

async function onUpload(file: File) {
  uploading.value = true
  try {
    await uploadMedia(file)
    ElMessage.success('上传成功')
    await load()
  } catch (e: any) {
    ElMessage.error(e.message ?? '上传失败')
  } finally {
    uploading.value = false
  }
}

async function remove(m: MediaItem) {
  try { await ElMessageBox.confirm(`确认删除 ${m.filename}？`, '提示') } catch { return }
  await deleteMedia(m.id)
  ElMessage.success('已删除')
  selectedIds.value = selectedIds.value.filter((x) => x !== m.id)
  await load()
}

async function removeSelected() {
  const ids = [...selectedIds.value]
  if (!ids.length) return
  try { await ElMessageBox.confirm(`确认删除选中的 ${ids.length} 个媒体？`, '提示') } catch { return }
  try {
    const r = await batchDeleteMedia(ids)
    ElMessage.success(`已删除 ${r.deleted} 个`)
    selectedIds.value = []
    await load()
  } catch (e: any) {
    ElMessage.error(e.message ?? '批量删除失败')
  }
}

onMounted(load)
</script>

<template>
  <div>
    <h2>媒体库</h2>
    <div class="bar">
      <el-upload
        :show-file-list="false"
        :before-upload="(f: File) => { onUpload(f); return false }"
        :disabled="uploading"
      >
        <el-button type="primary" :loading="uploading">上传媒体</el-button>
      </el-upload>
      <el-checkbox :model-value="allSelected" :indeterminate="someSelected" @change="toggleAll">全选</el-checkbox>
      <el-button type="danger" :disabled="selectedIds.length === 0" @click="removeSelected">删除选中({{ selectedIds.length }})</el-button>
    </div>
    <div class="grid" v-loading="loading">
      <div v-for="m in items" :key="m.id" class="item" :class="{ active: selectedIds.includes(m.id) }">
        <el-checkbox class="check" :model-value="selectedIds.includes(m.id)" @change="(v: unknown) => toggleOne(m.id, v)" />
        <img v-if="m.mime.startsWith('image/')" :src="m.url" :alt="m.filename" />
        <div v-else class="file-icon">{{ m.filename }}</div>
        <div class="meta">
          <div class="name">{{ m.filename }}</div>
          <el-button link type="danger" @click="remove(m)">删除</el-button>
        </div>
      </div>
      <el-empty v-if="!loading && items.length === 0" description="暂无媒体" style="grid-column: 1/-1" />
    </div>
    <el-pagination
      class="pager"
      background
      layout="prev, pager, next, total"
      :current-page="page"
      :page-size="perPage"
      :total="total"
      @current-change="onPageChange"
    />
  </div>
</template>

<style scoped>
.bar { display: flex; align-items: center; gap: 16px; margin-bottom: 16px; }
.grid { display: grid; grid-template-columns: repeat(5, 1fr); gap: 12px; }
.item { position: relative; border: 1px solid #eee; border-radius: 4px; overflow: hidden; }
.item.active { border-color: var(--el-color-primary); }
.item .check { position: absolute; top: 4px; left: 4px; z-index: 1; background: rgba(255, 255, 255, .85); border-radius: 3px; padding: 0 3px; margin: 0; }
.item img { width: 100%; height: 120px; object-fit: cover; display: block; }
.file-icon { height: 120px; display: flex; align-items: center; justify-content: center; background: #fafafa; }
.meta { padding: 8px; display: flex; justify-content: space-between; align-items: center; }
.name { font-size: 12px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.pager { margin-top: 16px; justify-content: center; }
</style>
