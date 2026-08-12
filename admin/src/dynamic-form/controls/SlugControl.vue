<script setup lang="ts">
import type { SchemaField } from '../types'
defineProps<{ field: SchemaField; modelValue: string }>()
defineEmits<{ 'update:modelValue': [string] }>()
</script>
<template>
  <el-input :model-value="modelValue" @update:model-value="$emit('update:modelValue', $event)" placeholder="小写字母数字横线" />
</template>
