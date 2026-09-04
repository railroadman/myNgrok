// Covers the registration form:
// - a password/confirm mismatch is caught client-side and never reaches the auth store
// - a successful submit registers and navigates to /app
// - a failed submit (e.g. email already taken) shows the message inline instead of navigating
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import RegisterView from './RegisterView.vue'
import { useAuthStore } from '../stores/auth'

const push = vi.fn()
vi.mock('vue-router', () => ({ useRouter: () => ({ push }) }))

describe('RegisterView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    push.mockReset()
  })

  it('rejects a password/confirm mismatch without calling the auth store', async () => {
    const wrapper = mount(RegisterView, { global: { stubs: { RouterLink: true } } })
    const auth = useAuthStore()
    const register = vi.spyOn(auth, 'register').mockResolvedValue()
    await wrapper.find('input[type="email"]').setValue('a@b.test')
    const passwords = wrapper.findAll('input[type="password"]')
    await passwords[0].setValue('password123')
    await passwords[1].setValue('different123')
    await wrapper.find('form').trigger('submit.prevent')
    await wrapper.vm.$nextTick()
    expect(wrapper.text()).toContain('Passwords do not match')
    expect(register).not.toHaveBeenCalled()
    expect(push).not.toHaveBeenCalled()
  })

  it('registers and navigates to /app when the passwords match', async () => {
    const wrapper = mount(RegisterView, { global: { stubs: { RouterLink: true } } })
    const auth = useAuthStore()
    vi.spyOn(auth, 'register').mockResolvedValue()
    await wrapper.find('input[type="email"]').setValue('a@b.test')
    const passwords = wrapper.findAll('input[type="password"]')
    await passwords[0].setValue('password123')
    await passwords[1].setValue('password123')
    await wrapper.find('form').trigger('submit.prevent')
    await wrapper.vm.$nextTick()
    expect(auth.register).toHaveBeenCalledWith('a@b.test', 'password123')
    expect(push).toHaveBeenCalledWith('/app')
  })

  it('shows the error message and does not navigate on a failed registration', async () => {
    const wrapper = mount(RegisterView, { global: { stubs: { RouterLink: true } } })
    const auth = useAuthStore()
    vi.spyOn(auth, 'register').mockRejectedValue({ message: 'Email already registered' })
    await wrapper.find('input[type="email"]').setValue('a@b.test')
    const passwords = wrapper.findAll('input[type="password"]')
    await passwords[0].setValue('password123')
    await passwords[1].setValue('password123')
    await wrapper.find('form').trigger('submit.prevent')
    await wrapper.vm.$nextTick()
    expect(wrapper.text()).toContain('Email already registered')
    expect(push).not.toHaveBeenCalled()
  })
})
