<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { listContent, type ContentEntry } from '../api/content'

const props = defineProps<{ typeName: string; visible: boolean }>()
const emit = defineEmits<{ 'update:visible': [boolean]; select: [content_id: string] }>()

const items = ref<ContentEntry[]>([])
const loading = ref(false)
const keyword = ref('')

async function load() {
  if (!props.visible) return
  loading.value = true
  try {
    const r = await listContent({ type: props.typeName, lang: '', page: 1, perPage: 100 })
    items.value = r.items
  } finally {
    loading.value = false
  }
}

const filteredItems = computed(() => {
  const k = keyword.value.trim().toLowerCase()
  if (!k) return items.value
  return items.value.filter((it) => it.content.title.toLowerCase().includes(k))
})

function pick(entry: ContentEntry) {
  emit('select', entry.content.content_id)
  emit('update:visible', false)
}

watch(() => props.visible, (v) => { if (v) load() })
</script>

<template>
  <el-dialog :model-value="visible" title="选择内容" width="640px" @update:model-value="$emit('update:visible', $event)" @open="load">
    <el-input v-model="keyword" placeholder="搜索标题" clearable style="margin-bottom: 12px" />
    <el-table :data="filteredItems" height="360" @row-click="pick">
      <el-table-column prop="content.title" label="标题" />
      <el-table-column prop="content.slug" label="Slug" width="180" />
      <el-table-column prop="content.lang" label="语言" width="80" />
    </el-table>
    <template #footer>
      <el-button @click="$emit('update:visible', false)">取消</el-button>
    </template>
  </el-dialog>
</template>
