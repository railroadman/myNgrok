import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import AgentsView from './AgentsView.vue'

vi.mock('../api/client', () => ({ api: vi.fn() }))
import { api } from '../api/client'

describe('AgentsView', () => {
  beforeEach(() => { setActivePinia(createPinia()); vi.mocked(api).mockResolvedValue([{ id: 'a1', instanceID: 'home', hostname: 'Home PC', os: 'linux', arch: 'amd64', version: '0.1', connected: true }]) })
  it('shows online agent returned by API', async () => {
    const wrapper = mount(AgentsView, { global: { stubs: { RouterLink: true } } })
    await Promise.resolve(); await wrapper.vm.$nextTick()
    expect(wrapper.text()).toContain('Home PC')
    expect(wrapper.text()).toContain('1 online of 1')
  })
})
