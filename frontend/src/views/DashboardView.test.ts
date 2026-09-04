import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import DashboardView from './DashboardView.vue'
import { useAuthStore } from '../stores/auth'

vi.mock('../api/client', () => ({ api: vi.fn() }))
import { api } from '../api/client'

describe('DashboardView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    const auth = useAuthStore()
    auth.accessToken = 'access-token'
    auth.user = { id: 'user-1', email: 'operator@example.test' }
    vi.mocked(api).mockImplementation(async (path: string) => {
      if (path === '/api/v1/tunnels') return [{ id: 't1', status: 'open' }]
      if (path === '/api/v1/agents') return [{ id: 'a1', connected: true }]
      if (path === '/api/v1/agent-tokens') return [{ id: 'k1' }]
      if (path === '/api/v1/traffic') return { requestsTotal: 3, requestBytes: 1024, responseBytes: 2048 }
      return []
    })
  })

  it('shows live counts for tunnels, agents, tokens and traffic', async () => {
    const wrapper = mount(DashboardView, { global: { stubs: { RouterLink: { template: '<a><slot /></a>' } } } })
    await Promise.resolve(); await Promise.resolve(); await wrapper.vm.$nextTick()
    expect(wrapper.text()).toContain('Dashboard')
    expect(wrapper.text()).toContain('1 active')
    expect(wrapper.text()).toContain('1 online')
    expect(wrapper.text()).toContain('1 active')
    expect(wrapper.text()).toContain('3.0 KB transferred')
  })
})
