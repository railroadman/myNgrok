import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import TunnelsView from './TunnelsView.vue'

vi.mock('../api/client', () => ({ api: vi.fn() }))
import { api } from '../api/client'

describe('TunnelsView', () => {
  beforeEach(() => { setActivePinia(createPinia()); vi.mocked(api).mockResolvedValue([{ id: 'tun_1', subdomain: 'demo123', localAddress: '127.0.0.1:8080', status: 'open', agentId: 'agent_1' }]) })
  it('shows tunnel returned by API', async () => {
    const wrapper = mount(TunnelsView, { global: { stubs: { RouterLink: true } } })
    await Promise.resolve(); await wrapper.vm.$nextTick()
    expect(wrapper.text()).toContain('demo123')
    expect(wrapper.text()).toContain('1 active of 1')
  })
})
