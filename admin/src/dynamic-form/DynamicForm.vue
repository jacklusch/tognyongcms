<script setup lang="ts">
import { computed } from 'vue'
import { registry, validateForm } from './registry'
import type { SchemaField, FormValues } from './types'

const props = defineProps<{ fields: SchemaField[]; modelValue: FormValues }>()
const emit = defineEmits<{ 'update:modelValue': [FormValues] }>()

const errors = computed(() => validateForm(props.fields, props.modelValue))

function updateField(name: string, val: unknown) {
  emit('update:modelValue', { ...props.modelValue, [name]: val })
}
</script>

<template>
  <el-form :model="modelValue" label-position="top">
    <el-form-item v-for="f in fields" :key="f.name" :label="f.label" :error="errors[f.name]">
      <component
        :is="registry[f.type]?.component"
        :field="f"
        :model-value="modelValue[f.name]"
        @update:model-value="updateField(f.name, $event)"
      />
      <div v-if="f.required" class="req-hint">*</div>
    </el-form-item>
  </el-form>
</template>

<style scoped>
.req-hint { color: var(--el-color-danger); }
</style>
