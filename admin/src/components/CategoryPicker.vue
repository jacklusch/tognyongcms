<script setup lang="ts">
import { ref } from 'vue'
import { listCategories, type CategoryItem } from '../api/category'

const visible = defineModel<boolean>('visible', { default: false })
const selected = defineModel<string>('selected', { default: '' })
const items = ref<CategoryItem[]>([])
const loading = ref(false)

async function load() {
  if (!visible.value) return
  loading.value = true
  try {
    const { items: list } = await listCategories()
    items.value = list
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <el-dialog v-model="visible" title="选择分类" width="480px" @open="load">
    <div class="cat-list" v-loading="loading">
      <div
        v-for="c in items"
        :key="c.id"
        class="cat-item"
        :class="{ active: selected === String(c.id) }"
        @click="selected = String(c.id)"
      >
        <div class="cat-name">{{ c.name }}</div>
        <div class="cat-slug">{{ c.slug }}</div>
      </div>
      <el-empty v-if="!loading && items.length === 0" description="暂无分类" />
    </div>
    <template #footer>
      <el-button @click="visible = false">取消</el-button>
      <el-button type="primary" @click="visible = false">确定</el-button>
    </template>
  </el-dialog>
</template>

<style scoped>
.cat-list { max-height: 360px; overflow-y: auto; }
.cat-item { display: flex; justify-content: space-between; padding: 8px 12px; border: 1px solid #eee; border-radius: 4px; margin-bottom: 8px; cursor: pointer; }
.cat-item.active { border-color: var(--el-color-primary); }
.cat-slug { color: var(--el-text-color-secondary); font-size: 12px; }
</style>
