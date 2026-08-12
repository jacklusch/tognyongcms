<script setup lang="ts">
import { computed } from 'vue'
import type { SchemaField, FormValues } from '../types'
import DynamicForm from '../DynamicForm.vue'

const props = defineProps<{ field: SchemaField; modelValue: unknown }>()
const emit = defineEmits<{ 'update:modelValue': [unknown] }>()

const rows = computed<FormValues[]>(() => Array.isArray(props.modelValue) ? props.modelValue as FormValues[] : [])

function addRow() {
  const next = [...rows.value]
  next.push(defaultRow())
  emit('update:modelValue', next)
}

function removeRow(i: number) {
  const next = rows.value.filter((_, idx) => idx !== i)
  emit('update:modelValue', next)
}

function updateRow(i: number, row: FormValues) {
  const next = [...rows.value]
  next[i] = row
  emit('update:modelValue', next)
}

function defaultRow(): FormValues {
  const out: FormValues = {}
  for (const f of props.field.sub_fields ?? []) {
    if (f.default !== undefined) out[f.name] = f.default
    else out[f.name] = ''
  }
  return out
}
</script>

<template>
  <div class="repeat-control">
    <div v-for="(row, i) in rows" :key="i" class="repeat-row">
      <DynamicForm :model-value="row" :fields="field.sub_fields ?? []" @update:model-value="updateRow(i, $event)" />
      <el-button link type="danger" @click="removeRow(i)">删除行</el-button>
    </div>
    <el-button size="small" @click="addRow">添加行</el-button>
  </div>
</template>

<style scoped>
.repeat-row { border: 1px dashed #dcdfe6; border-radius: 4px; padding: 8px 12px; margin-bottom: 8px; position: relative; }
</style>
