<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { listMedia } from '../api/media'
import type { MediaItem } from '../api/media'

const visible = defineModel<boolean>('visible', { default: false })
const selected = defineModel<string>('selected', { default: '' })
const items = ref<MediaItem[]>([])
const loading = ref(false)

async function load() {
  if (!visible.value) return
  loading.value = true
  try {
    const { items: list } = await listMedia()
    items.value = list
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

<template>
  <el-dialog v-model="visible" title="选择媒体" width="720px" @open="load">
    <div class="grid">
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
      <el-empty v-if="!loading && items.length === 0" description="暂无媒体" />
    </div>
    <template #footer>
      <el-button @click="visible = false">取消</el-button>
      <el-button type="primary" @click="visible = false">确定</el-button>
    </template>
  </el-dialog>
</template>

<style scoped>
.grid { display: grid; grid-template-columns: repeat(4, 1fr); gap: 12px; }
.item { border: 1px solid #eee; padding: 8px; cursor: pointer; border-radius: 4px; }
.item.active { border-color: var(--el-color-primary); }
.item img { width: 100%; height: 80px; object-fit: cover; }
.name { font-size: 12px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
</style>
