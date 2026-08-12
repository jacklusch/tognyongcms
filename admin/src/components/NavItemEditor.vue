<script setup lang="ts">
import { ref } from 'vue'
import type { NavItem } from '../views/menu-types'
import { emptyNavItem } from '../views/menu-types'

const props = defineProps<{ item: NavItem; types: string[] }>()
const emit = defineEmits<{
  'update:item': [NavItem]
  remove: []
}>()

function updateItem(patch: Partial<NavItem>) {
  emit('update:item', { ...props.item, ...patch })
}

function updateChild(i: number, child: NavItem) {
  const children = [...props.item.children]
  children[i] = child
  emit('update:item', { ...props.item, children })
}

function removeChild(i: number) {
  const children = props.item.children.filter((_, idx) => idx !== i)
  emit('update:item', { ...props.item, children })
}

function addChild() {
  emit('update:item', { ...props.item, children: [...props.item.children, emptyNavItem()] })
}
</script>

<template>
  <div class="nav-editor">
    <div class="nav-row">
      <el-input v-model="item.label" placeholder="显示文字" class="w-120" @update:model-value="updateItem({ label: $event })" />
      <el-select :model-value="item.type" class="w-140" @update:model-value="updateItem({ type: $event as NavItem['type'] })">
        <el-option label="首页" value="home" />
        <el-option v-for="t in types" :key="t" :label="t" :value="`type:${t}`" />
        <el-option label="自定义" value="custom" />
      </el-select>
      <el-input v-if="item.type === 'custom'" v-model="item.url" placeholder="路径如 /page/about" class="w-180" @update:model-value="updateItem({ url: $event })" />
      <el-button link type="primary" @click="addChild">+子项</el-button>
      <el-button link type="danger" @click="$emit('remove')">删</el-button>
    </div>
    <div v-if="item.children.length" class="nav-children">
      <NavItemEditor
        v-for="(child, i) in item.children"
        :key="i"
        :item="child"
        :types="types"
        @update:item="updateChild(i, $event)"
        @remove="removeChild(i)"
      />
    </div>
  </div>
</template>

<style scoped>
.nav-row { display: flex; gap: 8px; align-items: center; padding: 4px 0; }
.nav-children { margin-left: 24px; border-left: 1px dashed #dcdfe6; padding-left: 8px; }
.w-120 { width: 120px; }
.w-140 { width: 140px; }
.w-180 { width: 180px; }
</style>
