<script setup lang="ts">
import { ref, computed } from 'vue'
import { listCategories, buildCategoryOptions, type CategoryPath } from '../api/category'

const visible = defineModel<boolean>('visible', { default: false })
const selected = defineModel<string>('selected', { default: '' })
const all = ref<CategoryPath[]>([])
const loading = ref(false)
const value = ref<number>()

const options = computed(() => buildCategoryOptions(all.value))
const cascaderProps = { checkStrictly: true, emitPath: false }

async function load() {
  if (!visible.value) return
  loading.value = true
  try {
    const r = await listCategories()
    all.value = r.all
    value.value = r.all.find((p) => String(p.id) === selected.value)?.id
  } finally {
    loading.value = false
  }
}

function onPick(v: unknown) {
  const id = typeof v === 'number' ? v : undefined
  value.value = id
  selected.value = id === undefined ? '' : String(id)
}
</script>

<template>
  <el-dialog v-model="visible" title="选择分类" width="480px" @open="load">
    <el-cascader
      :model-value="value"
      :options="options"
      :props="cascaderProps"
      clearable
      filterable
      placeholder="选择分类（可清空）"
      style="width: 100%"
      v-loading="loading"
      @update:model-value="onPick"
    />
    <el-empty v-if="!loading && options.length === 0" description="暂无分类" />
    <template #footer>
      <el-button @click="visible = false">取消</el-button>
      <el-button type="primary" @click="visible = false">确定</el-button>
    </template>
  </el-dialog>
</template>
