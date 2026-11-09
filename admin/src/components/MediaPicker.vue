<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { listMedia } from '../api/media'
import type { MediaItem } from '../api/media'

const visible = defineModel<boolean>('visible', { default: false })
const selected = defineModel<string>('selected', { default: '' })
const items = ref<MediaItem[]>([])
const loading = ref(false)
const page = ref(1)
const perPage = ref(15)
const total = ref(0)

async function load() {
  if (!visible.value) return
  loading.value = true
  try {
    const { items: list, total: t } = await listMedia(page.value, perPage.value)
    items.value = list ?? []
    total.value = t ?? 0
  } finally {
    loading.value = false
  }
}

function open() {
  page.value = 1
  load()
}

function onPageChange(p: number) {
  page.value = p
  load()
}

onMounted(load)
</script>

<template>
  <el-dialog v-model="visible" title="选择媒体" width="920px" @open="open">
    <div class="grid" v-loading="loading">
      <div
        v-for="m in items"
        :key="m.id"
        class="item"
        :class="{ active: selected === m.url }"
        @click="selected = m.url"
      >
        <img v-if="m.mime.startsWith('image/')" :src="m.url" :alt="m.filename" />
        <div v-else class="file-icon">{{ m.filename }}</div>
        <div class="name">{{ m.filename }}</div>
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
    <template #footer>
      <el-button @click="visible = false">取消</el-button>
      <el-button type="primary" @click="visible = false">确定</el-button>
    </template>
  </el-dialog>
</template>

<style scoped>
.grid { display: grid; grid-template-columns: repeat(3, 1fr); gap: 12px; min-height: 120px; }
.item { border: 1px solid #eee; padding: 8px; cursor: pointer; border-radius: 4px; }
.item.active { border-color: var(--el-color-primary); }
.item img { width: 100%; height: 90px; object-fit: cover; }
.file-icon { height: 90px; display: flex; align-items: center; justify-content: center; background: #fafafa; font-size: 12px; }
.name { font-size: 12px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.pager { margin-top: 16px; justify-content: center; }
</style>
