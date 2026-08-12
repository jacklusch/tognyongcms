<script setup lang="ts">
import type { SchemaField } from '../types'
defineProps<{ field: SchemaField; modelValue: boolean }>()
defineEmits<{ 'update:modelValue': [boolean] }>()
</script>
<template>
  <el-switch :model-value="modelValue" @update:model-value="$emit('update:modelValue', $event)" />
</template>
