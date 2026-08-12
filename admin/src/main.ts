import { createApp } from 'vue'
import { createPinia } from 'pinia'
import ElementPlus from 'element-plus'
import 'element-plus/dist/index.css'
import App from './App.vue'
import router from './router'
import { useAuthStore } from './stores/auth'
import { me } from './api/auth'

async function bootstrap() {
  const pinia = createPinia()
  const app = createApp(App)
  app.use(pinia)
  app.use(router)
  app.use(ElementPlus)
  const auth = useAuthStore()
  if (auth.token && !auth.role) {
    try {
      const { user, permissions } = await me()
      auth.setUser({ username: user.username, role: user.role })
      auth.setPermissions(permissions)
    } catch { /* 401 由 client 处理跳登录 */ }
  }
  app.mount('#app')
}

bootstrap()
