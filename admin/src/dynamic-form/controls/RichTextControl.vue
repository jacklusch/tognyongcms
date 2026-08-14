<script setup lang="ts">
import { onBeforeUnmount, ref, shallowRef, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { Editor, Toolbar } from '@wangeditor/editor-for-vue'
import type { IDomEditor } from '@wangeditor/editor'
import { Transforms } from 'slate'
import '@wangeditor/editor/dist/css/style.css'
import type { SchemaField } from '../types'
import MediaPicker from '../../components/MediaPicker.vue'
import { uploadMedia } from '../../api/media'

const props = defineProps<{ field: SchemaField; modelValue: string }>()
const emit = defineEmits<{ 'update:modelValue': [string] }>()

const editorRef = shallowRef<IDomEditor>()
const pickerVisible = ref(false)
const pickerTarget = ref<'image' | 'video' | 'file'>('image')
const selectedUrl = ref('')

const toolbarConfig = {}
const editorConfig = {
  placeholder: '输入内容…',
  MENU_CONF: {
    // 上传图片：走现有 /api/media/upload（进媒体库），返回 url 插入编辑器
    uploadImage: {
      async customUpload(file: File, insertFn: (url: string, alt: string, href: string) => void) {
        try {
          const { url } = await uploadMedia(file)
          insertFn(url, file.name, url)
        } catch (e: any) {
          ElMessage.error(e.message ?? '图片上传失败')
        }
      },
      allowedFileTypes: ['image/*'],
    },
    // 上传视频：同理走媒体库
    uploadVideo: {
      async customUpload(file: File, insertFn: (url: string, poster: string) => void) {
        try {
          const { url } = await uploadMedia(file)
          insertFn(url, '')
        } catch (e: any) {
          ElMessage.error(e.message ?? '视频上传失败')
        }
      },
      allowedFileTypes: ['video/*'],
    },
  } as any,
}

function handleCreated(editor: IDomEditor) {
  editorRef.value = editor
  if (props.modelValue) {
    editor.setHtml(props.modelValue)
  }
}

watch(() => props.modelValue, (v) => {
  const ed = editorRef.value
  if (ed && v !== ed.getHtml()) {
    ed.setHtml(v ?? '')
  }
})

// 打开媒体库选择器
function openPicker(kind: 'image' | 'video' | 'file') {
  pickerTarget.value = kind
  selectedUrl.value = ''
  pickerVisible.value = true
}

// 在光标位置插入 slate 节点
function insertSlateNode(node: any) {
  const ed = editorRef.value
  if (!ed) return
  ed.deselect()
  Transforms.insertNodes(ed as any, node)
  ed.focus()
}

// 媒体选中后（MediaPicker 的 v-model:selected 变化时）插入
watch(selectedUrl, (url) => {
  if (!url || !pickerVisible.value) return
  pickerVisible.value = false
  const ed = editorRef.value
  if (!ed) return
  const filename = decodeURIComponent(url.split('/').pop() ?? '')
  try {
    if (pickerTarget.value === 'image') {
      insertSlateNode({ type: 'image', src: url, alt: filename, href: '', children: [{ text: '' }] })
    } else if (pickerTarget.value === 'video') {
      insertSlateNode({ type: 'video', src: url, children: [{ text: '' }] })
    } else {
      insertSlateNode({ type: 'link', url, children: [{ text: filename }] })
    }
  } catch (e: any) {
    ElMessage.error(e.message ?? '插入媒体失败')
  }
})
</script>

<template>
  <div class="richtext">
    <div class="richtext-toolbar-row">
      <Toolbar :editor="editorRef" :default-config="toolbarConfig" mode="default" />
      <el-button size="small" @click="openPicker('image')">选图片</el-button>
      <el-button size="small" @click="openPicker('video')">选视频</el-button>
      <el-button size="small" @click="openPicker('file')">选附件</el-button>
    </div>
    <Editor :default-config="editorConfig" :model-value="modelValue" @update:model-value="(v) => emit('update:modelValue', v)" mode="default" @on-created="handleCreated" style="height: 300px; overflow-y: hidden;" />
    <MediaPicker v-model:visible="pickerVisible" v-model:selected="selectedUrl" />
  </div>
</template>

<style scoped>
.richtext-toolbar-row { display: flex; align-items: center; gap: 8px; }
.richtext-toolbar-row .el-button { flex-shrink: 0; }
</style>
