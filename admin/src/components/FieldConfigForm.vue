<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { ElMessage } from 'element-plus'
import type { SchemaField, FieldType } from '../dynamic-form/types'
import { listContentTypes } from '../api/content-types'

const props = defineProps<{ visible: boolean; field: SchemaField | null; types: { name: string }[] }>()
const emit = defineEmits<{ 'update:visible': [boolean]; save: [SchemaField] }>()

const ALL_TYPES: FieldType[] = ['text','textarea','richtext','number','boolean','date','datetime','select','multiselect','image','file','slug','relation','repeat']

const form = ref<SchemaField>({ name: '', label: '', type: 'text' })
const optionsText = ref('')
const subFieldsText = ref('')
const editingName = ref('')

watch(() => props.visible, (v) => {
  if (!v) return
  if (props.field) {
    form.value = { ...props.field }
    editingName.value = props.field.name
    optionsText.value = (props.field.options ?? []).join('\n')
    subFieldsText.value = props.field.sub_fields ? JSON.stringify(props.field.sub_fields, null, 2) : ''
  } else {
    form.value = { name: '', label: '', type: 'text' }
    editingName.value = ''
    optionsText.value = ''
    subFieldsText.value = ''
  }
})

const needOptions = computed(() => form.value.type === 'select' || form.value.type === 'multiselect')
const needRelation = computed(() => form.value.type === 'relation')
const needRepeat = computed(() => form.value.type === 'repeat')
const needIndex = computed(() => form.value.type === 'text' || form.value.type === 'textarea')

function onSave() {
  if (!/^[a-zA-Z][a-zA-Z0-9_]*$/.test(form.value.name)) { ElMessage.warning('字段名不合法'); return }
  const out: SchemaField = { ...form.value }
  if (needOptions.value) out.options = optionsText.value.split('\n').map(s => s.trim()).filter(Boolean)
  else delete out.options
  if (needRepeat.value) {
    try {
      const arr = JSON.parse(subFieldsText.value || '[]')
      if (!Array.isArray(arr)) throw new Error()
      out.sub_fields = arr as SchemaField[]
    } catch {
      ElMessage.warning('子字段配置 JSON 格式错误')
      return
    }
  } else {
    delete out.sub_fields
  }
  if (!needRelation.value) delete out.relation_type
  if (!needIndex.value) delete out.indexed
  emit('save', out)
  emit('update:visible', false)
}
</script>

<template>
  <el-dialog :model-value="visible" :title="editingName ? '编辑字段' : '添加字段'" width="560px" @update:model-value="$emit('update:visible', $event)">
    <el-form label-width="90px">
      <el-form-item label="类型">
        <el-select v-model="form.type" style="width: 100%">
          <el-option v-for="t in ALL_TYPES" :key="t" :label="t" :value="t" />
        </el-select>
      </el-form-item>
      <el-form-item label="字段名">
        <el-input v-model="form.name" placeholder="如 title" :disabled="!!editingName" />
      </el-form-item>
      <el-form-item label="标签">
        <el-input v-model="form.label" placeholder="如 标题" />
      </el-form-item>
      <el-form-item>
        <el-checkbox v-model="form.required">必填</el-checkbox>
        <el-checkbox v-model="form.translatable">可翻译</el-checkbox>
        <el-checkbox v-if="needIndex" v-model="form.indexed">索引列</el-checkbox>
      </el-form-item>
      <el-form-item v-if="needOptions" label="选项">
        <el-input v-model="optionsText" type="textarea" :rows="4" placeholder="每行一个选项" />
      </el-form-item>
      <el-form-item v-if="needRelation" label="目标类型">
        <el-select v-model="form.relation_type" style="width: 100%">
          <el-option label="category" value="category" />
          <el-option v-for="t in types.filter(t => t.name !== 'category' && t.name !== form.name)" :key="t.name" :label="t.name" :value="t.name" />
        </el-select>
      </el-form-item>
      <el-form-item v-if="needRepeat" label="子字段">
        <div>子字段配置为简化表单（每行一个子字段定义 JSON），实现时提供简易编辑器或文本输入</div>
        <el-input v-model="subFieldsText" type="textarea" :rows="4" placeholder='[{"name":"title","label":"标题","type":"text","required":true}]' />
      </el-form-item>
      <el-form-item label="默认值">
        <el-input v-model="form.default" />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="$emit('update:visible', false)">取消</el-button>
      <el-button type="primary" @click="onSave">确定</el-button>
    </template>
  </el-dialog>
</template>
