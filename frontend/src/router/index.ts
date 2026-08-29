import { createRouter, createWebHistory } from 'vue-router'
import LoginView from '../views/LoginView.vue'
import RegisterView from '../views/RegisterView.vue'
import DashboardView from '../views/DashboardView.vue'
import TokensView from '../views/TokensView.vue'
import AgentsView from '../views/AgentsView.vue'
import TunnelsView from '../views/TunnelsView.vue'
import { useAuthStore } from '../stores/auth'

const router = createRouter({ history: createWebHistory(), routes: [
  { path: '/', redirect: '/app' },
  { path: '/login', component: LoginView },
  { path: '/register', component: RegisterView },
  { path: '/app', component: DashboardView, meta: { requiresAuth: true } },
  { path: '/app/tokens', component: TokensView, meta: { requiresAuth: true } },
  { path: '/app/agents', component: AgentsView, meta: { requiresAuth: true } },
  { path: '/app/tunnels', component: TunnelsView, meta: { requiresAuth: true } },
] })
router.beforeEach(async (to) => {
  const auth = useAuthStore()
  if (!auth.accessToken) await auth.restore()
  if (to.meta.requiresAuth && !auth.authenticated) return '/login'
  if ((to.path === '/login' || to.path === '/register') && auth.authenticated) return '/app'
})
export default router
