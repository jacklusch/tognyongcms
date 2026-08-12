<script setup lang="ts">
import { useAuthStore } from './stores/auth'
import { useRouter } from 'vue-router'
import { logout } from './api/auth'
import LayoutSidebar from './components/LayoutSidebar.vue'

const auth = useAuthStore()
const router = useRouter()

async function onLogout() {
  await logout().catch(() => {})   // best-effort 失效服务端会话
  auth.clear()
  router.push('/login')
}
</script>

<template>
  <el-container class="shell">
    <LayoutSidebar />
    <el-container>
      <el-header class="shell-header">
        <div class="user-info">
          <span>{{ auth.username }}</span>
          <el-button link type="primary" @click="onLogout">退出</el-button>
        </div>
      </el-header>
      <el-main><router-view /></el-main>
    </el-container>
  </el-container>
</template>

<style scoped>
.shell { min-height: 100vh; }
.shell-header { display: flex; justify-content: flex-end; align-items: center; border-bottom: 1px solid #eee; }
.user-info { display: flex; gap: 12px; align-items: center; }
</style>
