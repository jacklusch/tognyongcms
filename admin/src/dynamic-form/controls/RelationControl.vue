<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue'
import type { SchemaField } from '../types'
import ContentPicker from '../../components/ContentPicker.vue'
import CategoryPicker from '../../components/CategoryPicker.vue'

const props = defineProps<{ field: SchemaField; modelValue: string }>()
const emit = defineEmits<{ 'update:modelValue': [string] }>()

const pickerVisible = ref(false)
const isCategory = computed(() => props.field.relation_type === 'category')
// 回显：分类 id → 名称（CategoryPicker 打开时加载）
const categoryName = ref('')
const candidates = ref<{ content_id: string; title: string }[]>([])

const currentTitle = computed(() => {
  if (!props.modelValue) return ''
  if (isCategory.value) return categoryName.value || `${props.modelValue}（未匹配，可重新选择）`
  const hit = candidates.value.find((c) => c.content_id === props.modelValue)
  return hit ? hit.title : `${props.modelValue}（未匹配候选，可重新选择）`
})

async function loadCandidates() {
  if (isCategory.value) {
    // 加载分类并匹配当前值
    const { listCategories } = await import('../../api/category')
    const r = await listCategories()
    const hit = r.items.find((c) => String(c.id) === props.modelValue)
    categoryName.value = hit?.name ?? ''
    // T5：弹层高亮与已保存值同步（watch 同值跳过，不触发转发/关闭）
    selectedCat.value = props.modelValue
    return
  }
  const { listContent } = await import('../../api/content')
  const r = await listContent({ type: props.field.relation_type ?? '', lang: '', page: 1, perPage: 100 })
  candidates.value = r.items.map((it) => ({ content_id: it.content.content_id, title: it.content.title }))
}

function onSelect(content_id: string) {
  emit('update:modelValue', content_id)
  pickerVisible.value = false
}

// 反模式规避：CategoryPicker 选中值先落本地 ref，再转发给父级
const selectedCat = ref('')
function onSelectCat(v: string) {
  emit('update:modelValue', v)
  pickerVisible.value = false
}
// 同值（程序化同步）跳过，避免把弹层高亮同步误当成用户选择而关闭弹层
watch(selectedCat, (v) => { if (v !== props.modelValue) onSelectCat(v) })

watch(pickerVisible, (v) => { if (v) loadCandidates() })
// I1：首次渲染即回显分类名（或内容标题）
onMounted(() => { if (props.modelValue) loadCandidates() })
</script>

<template>
  <div class="relation-control">
    <el-input :model-value="currentTitle" readonly :placeholder="field.label" @click="pickerVisible = true">
      <template #append>
        <el-button @click="pickerVisible = true">选择</el-button>
      </template>
    </el-input>
    <el-button v-if="modelValue" link type="danger" @click="$emit('update:modelValue', '')">清除</el-button>
    <CategoryPicker v-if="isCategory" v-model:visible="pickerVisible" v-model:selected="selectedCat" @update:selected="onSelectCat" />
    <ContentPicker v-else :type-name="field.relation_type ?? ''" :visible="pickerVisible" @update:visible="pickerVisible = $event" @select="onSelect" />
  </div>
</template>
