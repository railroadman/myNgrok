import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import TokensView from './TokensView.vue'
import { useAuthStore } from '../stores/auth'

vi.mock('../api/client', () => ({ api: vi.fn() }))
import { api } from '../api/client'

describe('TokensView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    const auth = useAuthStore()
    auth.accessToken = 'access-token'
    auth.user = { id: 'user-1', email: 'operator@example.test' }
  })

  it('creates a token, reveals it once, and renders status', async () => {
    vi.mocked(api)
      .mockResolvedValueOnce([{ id: 'token-1', name: 'Home PC', prefix: 'tkn_home', createdAt: '', revokedAt: undefined }])
      .mockResolvedValueOnce({ id: 'token-2', name: 'Laptop', prefix: 'tkn_laptop', token: 'tkn_plaintext', createdAt: '' })
      .mockResolvedValueOnce([{ id: 'token-2', name: 'Laptop', prefix: 'tkn_laptop', createdAt: '', revokedAt: undefined }])
    const wrapper = mount(TokensView)
    await Promise.resolve(); await wrapper.vm.$nextTick()
    expect(wrapper.text()).toContain('Home PC')
    await wrapper.find('input').setValue('Laptop')
    await wrapper.find('form').trigger('submit')
    await Promise.resolve(); await wrapper.vm.$nextTick()
    expect(wrapper.text()).toContain('tkn_plaintext')
    expect(wrapper.text()).toContain('Active')
  })
})
