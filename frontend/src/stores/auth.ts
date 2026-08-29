import { defineStore } from 'pinia'
import { api } from '../api/client'

type User = { id: string; email: string }
type LoginResponse = { accessToken: string; user: User }

export const useAuthStore = defineStore('auth', {
  state: () => ({ accessToken: '', user: null as User | null, loading: false }),
  getters: { authenticated: (state) => Boolean(state.accessToken && state.user) },
  actions: {
    async login(email: string, password: string) {
      const result = await api<LoginResponse>('/api/v1/auth/login', { method: 'POST', body: JSON.stringify({ email, password }) })
      this.accessToken = result.accessToken; this.user = result.user
    },
    async register(email: string, password: string) {
      await api('/api/v1/auth/register', { method: 'POST', body: JSON.stringify({ email, password }) })
      await this.login(email, password)
    },
    async restore() {
      try {
        const result = await api<LoginResponse>('/api/v1/auth/refresh', { method: 'POST', body: '{}' })
        this.accessToken = result.accessToken; this.user = result.user
      } catch { this.accessToken = ''; this.user = null }
    },
    async logout() {
      await api('/api/v1/auth/logout', { method: 'POST', body: '{}' })
      this.accessToken = ''; this.user = null
    },
  },
})
