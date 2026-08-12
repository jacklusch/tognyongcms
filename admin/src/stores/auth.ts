import { defineStore } from 'pinia'

interface AuthState {
  token: string | null
  username: string | null
  role: string | null
  permissions: string[]
}

export const useAuthStore = defineStore('auth', {
  state: (): AuthState => ({
    token: localStorage.getItem('dlz_token'),
    username: null,
    role: null,
    permissions: [] as string[],
  }),
  getters: {
    isLoggedIn: (s) => !!s.token,
    isAdmin: (s) => s.role === 'admin',
    hasPerm: (s) => (p: string) => {
      if (s.permissions.includes('*')) return true
      if (s.permissions.includes(p)) return true
      const parts = p.split('.')
      for (let i = parts.length - 1; i >= 1; i--) {
        if (s.permissions.includes(parts.slice(0, i).join('.') + '.*')) return true
      }
      return false
    },
  },
  actions: {
    setToken(t: string) {
      this.token = t
      localStorage.setItem('dlz_token', t)
    },
    setUser(u: { username: string; role: string }) {
      this.username = u.username
      this.role = u.role
    },
    setPermissions(p: string[]) {
      this.permissions = p
    },
    clear() {
      this.token = null
      this.username = null
      this.role = null
      this.permissions = []
      localStorage.removeItem('dlz_token')
    },
  },
})
