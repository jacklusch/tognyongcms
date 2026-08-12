<script setup lang="ts">
import type { SchemaField } from '../types'
defineProps<{ field: SchemaField; modelValue: string }>()
defineEmits<{ 'update:modelValue': [string] }>()
</script>
<template>
  <el-date-picker :model-value="modelValue || null" @update:model-value="$emit('update:modelValue', $event ?? '')" type="datetime" value-format="YYYY-MM-DDTHH:mm:ssZ" />
</template>
