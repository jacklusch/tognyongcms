<script setup lang="ts">
import { useAuthStore } from '../stores/auth'

const auth = useAuthStore()
const menus = [
  { path: '/', label: '仪表盘', adminOnly: false },
  { path: '/content', label: '内容管理', adminOnly: false },
  { path: '/media', label: '媒体库', adminOnly: false },
  { path: '/menus', label: '菜单管理', adminOnly: false },
  { path: '/content-types', label: '内容类型', adminOnly: false, manageOnly: true },
  { path: '/settings', label: '设置', adminOnly: true },
  { path: '/categories', label: '分类管理', adminOnly: true },
  { path: '/users', label: '用户管理', adminOnly: true },
  { path: '/roles', label: '角色管理', adminOnly: true },
]
</script>

<template>
  <el-aside width="200px" class="sidebar">
    <el-menu :default-active="$route.path" router>
      <template v-for="m in menus" :key="m.path">
        <el-menu-item v-if="(!m.adminOnly || auth.isAdmin) && (!m.manageOnly || auth.role !== 'author')" :index="m.path">
          {{ m.label }}
        </el-menu-item>
      </template>
    </el-menu>
  </el-aside>
</template>

<style scoped>
.sidebar { border-right: 1px solid #eee; }
</style>
