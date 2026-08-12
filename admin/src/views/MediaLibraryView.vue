<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { listMedia, uploadMedia, deleteMedia, type MediaItem } from '../api/media'

const items = ref<MediaItem[]>([])
const loading = ref(false)
const uploading = ref(false)

async function load() {
  loading.value = true
  try {
    const r = await listMedia()
    items.value = r.items ?? []
  } catch (e: any) {
    items.value = []
    ElMessage.error(e.message ?? '加载媒体列表失败')
  } finally {
    loading.value = false
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
  await load()
}

onMounted(load)
</script>

<template>
  <div>
    <h2>媒体库</h2>
    <el-upload
      :show-file-list="false"
      :before-upload="(f: File) => { onUpload(f); return false }"
      :disabled="uploading"
    >
      <el-button type="primary" :loading="uploading">上传媒体</el-button>
    </el-upload>
    <div class="grid" v-loading="loading">
      <div v-for="m in items" :key="m.id" class="item">
        <img v-if="m.mime.startsWith('image/')" :src="m.url" :alt="m.filename" />
        <div v-else class="file-icon">{{ m.filename }}</div>
        <div class="meta">
          <div class="name">{{ m.filename }}</div>
          <el-button link type="danger" @click="remove(m)">删除</el-button>
        </div>
      </div>
      <el-empty v-if="!loading && items.length === 0" description="暂无媒体" style="grid-column: 1/-1" />
    </div>
  </div>
</template>

<style scoped>
.grid { display: grid; grid-template-columns: repeat(5, 1fr); gap: 12px; margin-top: 16px; }
.item { border: 1px solid #eee; border-radius: 4px; overflow: hidden; }
.item img { width: 100%; height: 120px; object-fit: cover; display: block; }
.file-icon { height: 120px; display: flex; align-items: center; justify-content: center; background: #fafafa; }
.meta { padding: 8px; display: flex; justify-content: space-between; align-items: center; }
.name { font-size: 12px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
</style>
