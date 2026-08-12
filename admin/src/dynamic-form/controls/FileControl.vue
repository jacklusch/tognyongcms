<script setup lang="ts">
import { ref } from 'vue'
import MediaPicker from '../../components/MediaPicker.vue'
import type { SchemaField } from '../types'

const props = defineProps<{ field: SchemaField; modelValue: string }>()
const emit = defineEmits<{ 'update:modelValue': [string] }>()
const pickerVisible = ref(false)
const selectedUrl = ref('')

function fileDisplay() {
  const v = props.modelValue
  if (!v) return ''
  const i = v.lastIndexOf('/')
  return i >= 0 ? v.slice(i + 1) : v
}

function openPicker() {
  selectedUrl.value = props.modelValue ?? ''
  pickerVisible.value = true
}
</script>

<template>
  <div class="file-control">
    <span v-if="modelValue" class="filename">{{ fileDisplay() }}</span>
    <el-button @click="openPicker">选择文件</el-button>
    <MediaPicker
      v-model:visible="pickerVisible"
      v-model:selected="selectedUrl"
      @update:selected="emit('update:modelValue', $event)"
    />
  </div>
</template>

<style scoped>
.filename { margin-right: 8px; font-size: 13px; color: #606266; }
</style>
