<script setup lang="ts">
import { ref } from 'vue'
import MediaPicker from '../../components/MediaPicker.vue'
import type { SchemaField } from '../types'

const props = defineProps<{ field: SchemaField; modelValue: string }>()
const emit = defineEmits<{ 'update:modelValue': [string] }>()
const pickerVisible = ref(false)
const selectedUrl = ref('')

function openPicker() {
  selectedUrl.value = props.modelValue ?? ''
  pickerVisible.value = true
}
</script>

<template>
  <div class="img-control">
    <img v-if="modelValue" :src="modelValue" class="preview" />
    <el-button @click="openPicker">选择图片</el-button>
    <MediaPicker
      v-model:visible="pickerVisible"
      v-model:selected="selectedUrl"
      @update:selected="emit('update:modelValue', $event)"
    />
  </div>
</template>

<style scoped>
.preview { max-width: 200px; max-height: 120px; display: block; margin-bottom: 8px; }
</style>
