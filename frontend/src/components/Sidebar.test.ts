// Covers the sidebar navigation:
// - it renders every group/item from `nav.ts` (Overview/Dashboard, Connectivity/Tunnels+Agents, Access/Tokens)
// - the collapse toggle button emits `toggle-collapse` (state lives in the parent AppLayout)
// - collapsed mode hides item/group labels (only icons remain) via the `collapsed` prop
// - clicking a nav item emits `close-mobile` (so AppLayout can close the off-canvas sidebar on mobile)
// - clicking "Sign out" calls the auth store's logout()
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import Sidebar from './Sidebar.vue'
import { useAuthStore } from '../stores/auth'

const routerLinkStub = { template: '<a @click="$emit(\'click\')"><slot /></a>', props: ['to'], emits: ['click'] }

describe('Sidebar', () => {
  beforeEach(() => setActivePinia(createPinia()))

  it('renders every configured nav group and item', () => {
    const wrapper = mount(Sidebar, { props: { collapsed: false, mobileOpen: false }, global: { stubs: { RouterLink: routerLinkStub } } })
    expect(wrapper.text()).toContain('Overview')
    expect(wrapper.text()).toContain('Dashboard')
    expect(wrapper.text()).toContain('Connectivity')
    expect(wrapper.text()).toContain('Tunnels')
    expect(wrapper.text()).toContain('Agents')
    expect(wrapper.text()).toContain('Access')
    expect(wrapper.text()).toContain('Tokens')
  })

  it('emits toggle-collapse when the collapse button is clicked', async () => {
    const wrapper = mount(Sidebar, { props: { collapsed: false, mobileOpen: false }, global: { stubs: { RouterLink: routerLinkStub } } })
    await wrapper.find('.collapse-btn').trigger('click')
    expect(wrapper.emitted('toggle-collapse')).toHaveLength(1)
  })

  it('hides labels but keeps icons when collapsed', () => {
    const wrapper = mount(Sidebar, { props: { collapsed: true, mobileOpen: false }, global: { stubs: { RouterLink: routerLinkStub } } })
    expect(wrapper.text()).not.toContain('Dashboard')
    expect(wrapper.findAll('svg').length).toBeGreaterThan(0)
  })

  it('emits close-mobile when a nav item is clicked', async () => {
    const wrapper = mount(Sidebar, { props: { collapsed: false, mobileOpen: true }, global: { stubs: { RouterLink: routerLinkStub } } })
    await wrapper.find('.sidebar-item').trigger('click')
    expect(wrapper.emitted('close-mobile')).toHaveLength(1)
  })

  it('logs out when "Sign out" is clicked', async () => {
    const wrapper = mount(Sidebar, { props: { collapsed: false, mobileOpen: false }, global: { stubs: { RouterLink: routerLinkStub } } })
    const auth = useAuthStore()
    const logout = vi.spyOn(auth, 'logout').mockResolvedValue()
    await wrapper.find('.signout-btn').trigger('click')
    expect(logout).toHaveBeenCalled()
  })
})
