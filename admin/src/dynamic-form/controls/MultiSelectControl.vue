<script setup lang="ts">
import type { SchemaField } from '../types'
defineProps<{ field: SchemaField; modelValue: string[] }>()
defineEmits<{ 'update:modelValue': [string[]] }>()
</script>
<template>
  <el-select :model-value="modelValue" @update:model-value="$emit('update:modelValue', $event)" multiple style="width: 100%">
    <el-option v-for="o in field.options" :key="o" :label="o" :value="o" />
  </el-select>
</template>
