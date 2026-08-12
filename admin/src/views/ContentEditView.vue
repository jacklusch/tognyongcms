<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { getContent, createContent, updateContent, publishContent, unpublishContent, listTranslations, createTranslation, type ContentEntry } from '../api/content'
import { listContentTypes, type ContentTypeItem } from '../api/content-types'
import { fetchMeta, type MetaData } from '../api/meta'
import DynamicForm from '../dynamic-form/DynamicForm.vue'
import { validateForm } from '../dynamic-form/registry'
import type { FormValues } from '../dynamic-form/types'
import { resolveSwitchTab } from './content-edit-tabs'
import { useAuthStore } from '../stores/auth'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const meta = ref<MetaData | null>(null)
const types = ref<ContentTypeItem[]>([])
const id = computed(() => route.params.id ? Number(route.params.id) : null)
const isNew = computed(() => !id.value)

const currentType = ref<ContentTypeItem | null>(null)
const currentLang = ref('')
const activeTab = ref('')
const translations = ref<ContentEntry[]>([])
const form = ref<FormValues>({})
const loading = ref(false)
const saving = ref(false)

const langTabs = computed(() => meta.value?.languages ?? [])
const fields = computed(() => currentType.value?.fields ?? [])

// 非 translatable 字段跨语言共享：从首个已有翻译取共享值
const sharedValues = computed(() => {
  const first = translations.value[0]
  if (!first) return {}
  const out: FormValues = {}
  for (const f of fields.value) {
    if (!f.translatable) out[f.name] = first.fields[f.name]
  }
  return out
})

async function loadType() {
  const tr = await listContentTypes()
  types.value = tr.items
}

async function loadTranslations(contentId: number) {
  // I1 容错：author 无类型级读权限时 listTranslations 403——吞掉返回空，避免新建成功后误报保存失败。
  try {
    const tr = await listTranslations(contentId)
    translations.value = tr.items
    const byLang: Record<string, ContentEntry> = {}
    for (const t of tr.items) byLang[t.content.lang] = t
    return byLang
  } catch {
    translations.value = []
    return {}
  }
}

async function loadEdit(contentId: number) {
  const { content } = await getContent(contentId)
  const byLang = await loadTranslations(contentId)
  currentType.value = types.value.find((t) => t.name === content.type_name) ?? null
  currentLang.value = content.content.lang
  activeTab.value = content.content.lang
  const langEntry = byLang[activeTab.value]
  form.value = { ...sharedValues.value, ...(langEntry?.fields ?? {}) }
}

async function initNew() {
  // 路由 query 带 type
  const typeName = String(route.query.type ?? types.value[0]?.name ?? '')
  currentType.value = types.value.find((t) => t.name === typeName) ?? null
  currentLang.value = meta.value?.default_lang ?? ''
  activeTab.value = currentLang.value
  form.value = {}
}

const needCreate = ref(false)

async function switchTab(lang: string) {
  if (id.value) {
    const byLang = await loadTranslations(id.value)
    const { form: next, needCreate: create } = resolveSwitchTab(byLang, lang, sharedValues.value, form.value)
    form.value = next
    needCreate.value = create
    if (create) {
      ElMessage.info(`「${lang}」暂无翻译，保存后将创建该语言版本`)
    }
  }
  activeTab.value = lang
}

async function save() {
  if (!currentType.value) return
  const errs = validateForm(fields.value, form.value)
  if (Object.keys(errs).length) {
    ElMessage.warning(Object.values(errs)[0])
    return
  }
  saving.value = true
  try {
    const payload: FormValues = { ...sharedValues.value, ...form.value }
    if (isNew.value) {
      const { content } = await createContent(currentType.value.name, activeTab.value, payload)
      router.replace(`/content/${content.content.id}`)
      ElMessage.success('已创建')
      // 刷新 id 后走编辑态
      await loadAfterCreate(content.content.id)
    } else if (id.value) {
      if (needCreate.value) {
        await createTranslation(id.value, activeTab.value, payload, currentType.value.name)
        needCreate.value = false
      } else {
        await updateContent(id.value, payload)
      }
      ElMessage.success('已保存')
      await loadEdit(id.value)
    }
  } catch (e: any) {
    ElMessage.error(e.message ?? '保存失败')
  } finally {
    saving.value = false
  }
}

async function loadAfterCreate(contentId: number) {
  await loadType()
  const byLang = await loadTranslations(contentId)
  currentType.value = types.value.find((t) => t.name === currentType.value?.name) ?? null
  activeTab.value = currentLang.value
  const entry = byLang[activeTab.value]
  form.value = { ...sharedValues.value, ...(entry?.fields ?? {}) }
}

async function togglePublish() {
  if (!id.value) return
  const entry = translations.value.find((t) => t.content.lang === activeTab.value)
  const target = entry?.content.status === 'published' ? 'unpublish' : 'publish'
  if (target === 'publish') await publishContent(id.value)
  else await unpublishContent(id.value)
  ElMessage.success(target === 'publish' ? '已发布' : '已撤回')
  await loadEdit(id.value)
}

onMounted(async () => {
  meta.value = await fetchMeta()
  await loadType()
  if (isNew.value) await initNew()
  else if (id.value) await loadEdit(id.value)
})
</script>

<template>
  <div v-if="currentType">
    <h2>{{ isNew ? '新建内容' : '编辑内容' }} · {{ currentType.label }}</h2>
    <el-tabs v-model="activeTab" @tab-change="switchTab">
      <el-tab-pane v-for="l in langTabs" :key="l" :label="l" :name="l" />
    </el-tabs>
    <DynamicForm v-model="form" :fields="fields" />
    <div class="actions">
      <el-button type="primary" :loading="saving" @click="save">保存</el-button>
      <el-button v-if="!isNew && (auth.role === 'admin' || auth.role === 'editor') && auth.hasPerm(`content.publish.${currentType?.name}`)" @click="togglePublish">发布/撤回</el-button>
      <el-button @click="router.back()">返回</el-button>
    </div>
  </div>
  <el-empty v-else description="请先创建内容类型" />
</template>

<style scoped>
.actions { margin-top: 24px; display: flex; gap: 12px; }
</style>
