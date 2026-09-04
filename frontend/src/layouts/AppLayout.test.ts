// Covers the authenticated shell that wraps every /app/* route:
// - it renders the Sidebar and a RouterView outlet for the page content
// - the mobile hamburger button opens the off-canvas sidebar, and clicking the backdrop closes it again
// - the desktop "collapse to icon rail" toggle is suppressed while on a mobile-width viewport
//   (this guards against a real bug: collapsing on desktop used to leave the sidebar stuck in
//   icon-only mode after resizing to mobile, since both states shared one `collapsed` flag)
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import AppLayout from './AppLayout.vue'
import Sidebar from '../components/Sidebar.vue'

function stubMatchMedia(initialMatches: boolean) {
  const listeners: Array<(e: { matches: boolean }) => void> = []
  const mql = {
    matches: initialMatches,
    media: '(max-width: 768px)',
    addEventListener: (_: string, listener: (e: { matches: boolean }) => void) => listeners.push(listener),
    removeEventListener: vi.fn(),
  }
  window.matchMedia = vi.fn().mockReturnValue(mql)
  // A real MediaQueryList's `.matches` is live and read directly by the component on every
  // 'change' event, so the stub must update it before notifying listeners, same as a browser would.
  return {
    emitChange: (matches: boolean) => {
      mql.matches = matches
      listeners.forEach((l) => l({ matches }))
    },
  }
}

const stubs = { RouterView: true, RouterLink: { template: '<a><slot /></a>' } }

describe('AppLayout', () => {
  beforeEach(() => setActivePinia(createPinia()))

  it('renders the sidebar and the routed page content outlet', () => {
    stubMatchMedia(false)
    const wrapper = mount(AppLayout, { global: { stubs } })
    expect(wrapper.findComponent(Sidebar).exists()).toBe(true)
    expect(wrapper.find('router-view-stub').exists()).toBe(true)
  })

  it('opens the sidebar on mobile-hamburger click and closes it via the backdrop', async () => {
    stubMatchMedia(true)
    const wrapper = mount(AppLayout, { global: { stubs } })
    expect(wrapper.find('.backdrop').exists()).toBe(false)
    await wrapper.find('.hamburger').trigger('click')
    expect(wrapper.find('.backdrop').exists()).toBe(true)
    await wrapper.find('.backdrop').trigger('click')
    expect(wrapper.find('.backdrop').exists()).toBe(false)
  })

  it('keeps the sidebar expanded on mobile even after the desktop collapse toggle fires', async () => {
    const { emitChange } = stubMatchMedia(false)
    const wrapper = mount(AppLayout, { global: { stubs } })
    const sidebar = wrapper.findComponent(Sidebar)
    await sidebar.vm.$emit('toggle-collapse')
    expect(sidebar.props('collapsed')).toBe(true)

    emitChange(true)
    await wrapper.vm.$nextTick()
    expect(sidebar.props('collapsed')).toBe(false)
  })
})
