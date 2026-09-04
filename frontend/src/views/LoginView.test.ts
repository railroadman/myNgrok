// Covers the login form: a successful submit calls the auth store and navigates to /app,
// and a failed submit (bad credentials, or any thrown error) shows the message inline instead
// of navigating anywhere.
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import LoginView from './LoginView.vue'
import { useAuthStore } from '../stores/auth'

const push = vi.fn()
vi.mock('vue-router', () => ({ useRouter: () => ({ push }) }))

describe('LoginView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    push.mockReset()
  })

  it('logs in and navigates to /app on success', async () => {
    const wrapper = mount(LoginView, { global: { stubs: { RouterLink: true } } })
    const auth = useAuthStore()
    vi.spyOn(auth, 'login').mockResolvedValue()
    await wrapper.find('input[type="email"]').setValue('a@b.test')
    await wrapper.find('input[type="password"]').setValue('password123')
    await wrapper.find('form').trigger('submit.prevent')
    await wrapper.vm.$nextTick()
    expect(auth.login).toHaveBeenCalledWith('a@b.test', 'password123')
    expect(push).toHaveBeenCalledWith('/app')
  })

  it('shows the error message and does not navigate on a failed login', async () => {
    const wrapper = mount(LoginView, { global: { stubs: { RouterLink: true } } })
    const auth = useAuthStore()
    vi.spyOn(auth, 'login').mockRejectedValue({ message: 'Invalid email or password' })
    await wrapper.find('input[type="email"]').setValue('a@b.test')
    await wrapper.find('input[type="password"]').setValue('wrong-password')
    await wrapper.find('form').trigger('submit.prevent')
    await wrapper.vm.$nextTick()
    expect(wrapper.text()).toContain('Invalid email or password')
    expect(push).not.toHaveBeenCalled()
  })
})
