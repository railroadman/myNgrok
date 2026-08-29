import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import DashboardView from './DashboardView.vue'
import { useAuthStore } from '../stores/auth'

vi.mock('vue-router', () => ({ useRouter: () => ({ push: vi.fn() }) }))

describe('DashboardView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    const auth = useAuthStore()
    auth.accessToken = 'access-token'
    auth.user = { id: 'user-1', email: 'operator@example.test' }
  })

  it('shows operator actions for tunnels, agents and tokens', () => {
    const wrapper = mount(DashboardView, { global: { stubs: { RouterLink: { template: '<a><slot /></a>' } } } })
    expect(wrapper.text()).toContain('Route local')
    expect(wrapper.text()).toContain('Inspect tunnels')
    expect(wrapper.text()).toContain('Monitor agents')
    expect(wrapper.text()).toContain('Create a token')
  })
})
