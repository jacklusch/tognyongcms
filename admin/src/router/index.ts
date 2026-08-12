import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const base = import.meta.env.DEV ? '/' : '/admin/'

const router = createRouter({
  history: createWebHistory(base),
  routes: [
    { path: '/login', name: 'login', component: () => import('../views/LoginView.vue') },
    {
      path: '/',
      component: () => import('../AppShell.vue'),
      children: [
        { path: '', name: 'dashboard', component: () => import('../views/DashboardView.vue'), meta: { auth: true } },
        { path: 'content', name: 'content-list', component: () => import('../views/ContentListView.vue'), meta: { auth: true } },
        { path: 'content/new', name: 'content-new', component: () => import('../views/ContentEditView.vue'), meta: { auth: true } },
        { path: 'content/:id', name: 'content-edit', component: () => import('../views/ContentEditView.vue'), meta: { auth: true } },
        { path: 'media', name: 'media', component: () => import('../views/MediaLibraryView.vue'), meta: { auth: true } },
        { path: 'menus', name: 'menus', component: () => import('../views/MenuManageView.vue'), meta: { auth: true } },
        { path: 'content-types', name: 'content-types', component: () => import('../views/ContentTypesView.vue'), meta: { auth: true, adminOnly: false } },
        { path: 'settings', name: 'settings', component: () => import('../views/SettingsView.vue'), meta: { auth: true, adminOnly: true } },
        { path: 'categories', name: 'categories', component: () => import('../views/CategoriesView.vue'), meta: { auth: true, adminOnly: true } },
        { path: 'users', name: 'users', component: () => import('../views/UsersView.vue'), meta: { auth: true, adminOnly: true } },
        { path: 'roles', name: 'roles', component: () => import('../views/RolesView.vue'), meta: { auth: true, adminOnly: true } },
      ],
    },
  ],
})

router.beforeEach((to) => {
  const auth = useAuthStore()
  if (to.meta.auth && !auth.isLoggedIn) {
    return { name: 'login' }
  }
  if (to.meta.adminOnly && !auth.isAdmin) {
    return { name: 'dashboard' }
  }
  if (to.name === 'login' && auth.isLoggedIn) {
    return { name: 'dashboard' }
  }
  return true
})

export default router
