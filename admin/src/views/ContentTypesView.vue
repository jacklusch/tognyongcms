<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { listContentTypes, createContentType, updateContentType, deleteContentType, type ContentTypeItem } from '../api/content-types'
import { validateContentType } from './content-types-validate'
import FieldConfigForm from '../components/FieldConfigForm.vue'
import type { SchemaField } from '../dynamic-form/types'

const types = ref<ContentTypeItem[]>([])
const selectedId = ref<number | null>(null)
const selectedType = ref<ContentTypeItem | null>(null)
const editingLabel = ref('')
const formVisible = ref(false)
const editingField = ref<SchemaField | null>(null)

function syncSelected() {
  selectedType.value = types.value.find(t => t.id === selectedId.value) ?? null
}

async function load() {
  const r = await listContentTypes()
  types.value = r.items
  if (selectedId.value === null && types.value.length) selectedId.value = types.value[0].id
  syncSelected()
}

onMounted(load)

function select(id: number) {
  selectedId.value = id
  syncSelected()
  editingLabel.value = selectedType.value?.label ?? ''
}

async function saveType() {
  if (!selectedType.value) return
  const draft = { name: selectedType.value.name, label: editingLabel.value, fields: selectedType.value.fields }
  const err = validateContentType(draft)
  if (err) { ElMessage.warning(err); return }
  await updateContentType(selectedType.value.id, { label: editingLabel.value, fields: selectedType.value.fields })
  ElMessage.success('已保存')
  await load()
}

async function createNew() {
  // 类型名必须是 [a-zA-Z][a-zA-Z0-9_]*，不能含连字符
  const name = `newtype${Date.now().toString(36)}`
  try {
    await createContentType({ name, label: name, fields: [] })
    await load()
    const created = types.value.find(t => t.name === name)
    if (created) select(created.id)
  } catch (e: any) {
    ElMessage.error(e.message ?? '新建内容类型失败')
  }
}

async function copyType(t: ContentTypeItem) {
  // 复制名用下划线连接（连字符会被后端拒绝）
  const newName = `${t.name}_copy`
  try {
    await createContentType({ name: newName, label: `${t.label} 副本`, fields: JSON.parse(JSON.stringify(t.fields)) })
    await load()
    ElMessage.success('已复制')
    const created = types.value.find(x => x.name === newName)
    if (created) select(created.id)
  } catch (e: any) {
    ElMessage.error(e.message ?? '复制内容类型失败')
  }
}

async function removeType(t: ContentTypeItem) {
  try { await ElMessageBox.confirm(`确认删除内容类型 ${t.name}？该类型全部内容将一并删除。`, '危险操作', { type: 'warning' }) } catch { return }
  await deleteContentType(t.name)
  ElMessage.success('已删除')
  selectedId.value = null
  await load()
}

function openAddField() { editingField.value = null; formVisible.value = true }
function openEditField(f: SchemaField) { editingField.value = f; formVisible.value = true }

function onFieldSave(field: SchemaField) {
  if (!selectedType.value) return
  const idx = selectedType.value.fields.findIndex(f => f.name === (editingField.value?.name ?? '__new__'))
  const fields = [...selectedType.value.fields]
  if (idx >= 0) fields[idx] = field
  else fields.push(field)
  selectedType.value = { ...selectedType.value, fields }
}

function removeField(name: string) {
  if (!selectedType.value) return
  selectedType.value = { ...selectedType.value, fields: selectedType.value.fields.filter(f => f.name !== name) }
}

function moveField(i: number, dir: -1 | 1) {
  if (!selectedType.value) return
  const fields = [...selectedType.value.fields]
  const j = i + dir
  if (j < 0 || j >= fields.length) return
  ;[fields[i], fields[j]] = [fields[j], fields[i]]
  selectedType.value = { ...selectedType.value, fields }
}

function copyField(f: SchemaField) {
  if (!selectedType.value) return
  selectedType.value = { ...selectedType.value, fields: [...selectedType.value.fields, { ...f, name: `${f.name}-copy` }] }
}
</script>

<template>
  <div class="builder">
    <div class="left">
      <el-card>
        <template #header>
          <div class="card-head">
            <span>内容类型</span>
            <el-button size="small" type="primary" @click="createNew">新建</el-button>
          </div>
        </template>
        <el-menu :default-active="String(selectedId)" @select="(i: string) => select(Number(i))">
          <el-menu-item v-for="t in types" :key="t.id" :index="String(t.id)">
            <span class="type-name">{{ t.name }}</span>
            <span class="type-label">{{ t.label }}</span>
          </el-menu-item>
        </el-menu>
      </el-card>
    </div>
    <div class="right">
      <el-card v-if="selectedType">
        <template #header>
          <div class="card-head">
            <span>字段配置 · {{ selectedType.name }}</span>
            <div>
              <el-button size="small" @click="saveType">保存</el-button>
              <el-button size="small" @click="copyType(selectedType)">复制</el-button>
              <el-button size="small" type="danger" @click="removeType(selectedType)">删除</el-button>
            </div>
          </div>
        </template>
        <el-form label-width="70px" style="max-width: 420px">
          <el-form-item label="标签">
            <el-input v-model="editingLabel" />
          </el-form-item>
        </el-form>
        <el-button size="small" type="primary" @click="openAddField">添加字段</el-button>
        <el-table :data="selectedType.fields" size="small" style="margin-top: 12px">
          <el-table-column prop="name" label="名称" width="140" />
          <el-table-column prop="label" label="标签" />
          <el-table-column prop="type" label="类型" width="110" />
          <el-table-column label="标记" width="140">
            <template #default="{ row }">
              <el-tag v-if="row.required" size="small">必填</el-tag>
              <el-tag v-if="row.translatable" size="small" type="success">翻译</el-tag>
              <el-tag v-if="row.indexed" size="small" type="info">索引</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="180">
            <template #default="{ row, $index }">
              <el-button link type="primary" @click="openEditField(row)">编辑</el-button>
              <el-button link @click="moveField($index, -1)">↑</el-button>
              <el-button link @click="moveField($index, 1)">↓</el-button>
              <el-button link @click="copyField(row)">复制</el-button>
              <el-button link type="danger" @click="removeField(row.name)">删</el-button>
            </template>
          </el-table-column>
        </el-table>
      </el-card>
      <el-empty v-else description="请选择或新建内容类型" />
    </div>
    <FieldConfigForm :visible="formVisible" :field="editingField" :types="types" @update:visible="formVisible = $event" @save="onFieldSave" />
  </div>
</template>

<style scoped>
.builder { display: flex; gap: 16px; align-items: flex-start; }
.left { width: 280px; flex-shrink: 0; }
.right { flex: 1; }
.card-head { display: flex; justify-content: space-between; align-items: center; }
.type-label { color: var(--el-text-color-secondary); font-size: 12px; margin-left: 8px; }
</style>
