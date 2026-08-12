<script setup lang="ts">
import type { SchemaField } from '../types'
defineProps<{ field: SchemaField; modelValue: number | null }>()
defineEmits<{ 'update:modelValue': [number | null] }>()
</script>
<template>
  <el-input-number :model-value="modelValue ?? undefined" @update:model-value="$emit('update:modelValue', $event)" />
</template>
