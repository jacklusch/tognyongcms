<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { fetchSettings, updateSettings } from '../api/settings'
import { fetchMeta, type MetaData } from '../api/meta'

const meta = ref<MetaData | null>(null)
const settings = ref<Record<string, string>>({})
const loading = ref(false)
const saving = ref(false)

async function load() {
  meta.value = await fetchMeta()
  const r = await fetchSettings()
  settings.value = { ...r.settings }
  if (!settings.value.site_name) settings.value.site_name = meta.value.site_name
  if (!settings.value.description) settings.value.description = meta.value.description
  if (!settings.value.theme) settings.value.theme = meta.value.theme
}

async function save() {
  saving.value = true
  try {
    await updateSettings(settings.value)
    ElMessage.success('已保存')
  } catch (e: any) {
    ElMessage.error(e.message ?? '保存失败')
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>

<template>
  <div v-if="meta">
    <h2>站点设置</h2>
    <el-form label-width="100px" style="max-width: 520px">
      <el-form-item label="站点名称">
        <el-input v-model="settings.site_name" />
      </el-form-item>
      <el-form-item label="站点描述">
        <el-input v-model="settings.description" type="textarea" :rows="3" />
      </el-form-item>
      <el-form-item label="主题">
        <el-input v-model="settings.theme" placeholder="default" />
      </el-form-item>
      <el-form-item label="语言">
        <el-tag v-for="l in meta.languages" :key="l" style="margin-right: 4px">{{ l }}</el-tag>
      </el-form-item>
      <el-form-item>
        <el-button type="primary" :loading="saving" @click="save">保存</el-button>
      </el-form-item>
    </el-form>
  </div>
</template>
