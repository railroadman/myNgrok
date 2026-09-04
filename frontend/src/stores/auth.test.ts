// Covers the auth Pinia store end to end:
// - login() on success stores accessToken/user; on failure leaves state untouched and rethrows
// - register() posts registration then chains into login() to establish a session
// - restore() (called by the router guard on load) restores a session from the refresh cookie,
//   and silently clears state instead of throwing when there is no valid session to restore
// - logout() calls the API and always clears local state
// - the `authenticated` getter requires both a token and a user
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useAuthStore } from './auth'

vi.mock('../api/client', () => ({ api: vi.fn() }))
import { api } from '../api/client'

describe('auth store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.mocked(api).mockReset()
  })

  it('is unauthenticated until both a token and a user are present', () => {
    const auth = useAuthStore()
    expect(auth.authenticated).toBe(false)
    auth.accessToken = 'tok'
    expect(auth.authenticated).toBe(false)
    auth.user = { id: 'u1', email: 'a@b.test' }
    expect(auth.authenticated).toBe(true)
  })

  it('login stores the access token and user on success', async () => {
    vi.mocked(api).mockResolvedValue({ accessToken: 'tok', user: { id: 'u1', email: 'a@b.test' } })
    const auth = useAuthStore()
    await auth.login('a@b.test', 'password123')
    expect(auth.accessToken).toBe('tok')
    expect(auth.user).toEqual({ id: 'u1', email: 'a@b.test' })
    expect(api).toHaveBeenCalledWith('/api/v1/auth/login', { method: 'POST', body: JSON.stringify({ email: 'a@b.test', password: 'password123' }) })
  })

  it('login leaves state untouched and rethrows on failure', async () => {
    vi.mocked(api).mockRejectedValue({ code: 'UNAUTHORIZED', message: 'bad credentials' })
    const auth = useAuthStore()
    await expect(auth.login('a@b.test', 'wrong')).rejects.toEqual({ code: 'UNAUTHORIZED', message: 'bad credentials' })
    expect(auth.accessToken).toBe('')
    expect(auth.user).toBeNull()
  })

  it('register calls the register endpoint then logs in with the same credentials', async () => {
    vi.mocked(api).mockResolvedValueOnce(undefined).mockResolvedValueOnce({ accessToken: 'tok', user: { id: 'u1', email: 'a@b.test' } })
    const auth = useAuthStore()
    await auth.register('a@b.test', 'password123')
    expect(api).toHaveBeenNthCalledWith(1, '/api/v1/auth/register', { method: 'POST', body: JSON.stringify({ email: 'a@b.test', password: 'password123' }) })
    expect(api).toHaveBeenNthCalledWith(2, '/api/v1/auth/login', { method: 'POST', body: JSON.stringify({ email: 'a@b.test', password: 'password123' }) })
    expect(auth.authenticated).toBe(true)
  })

  it('restore establishes a session from a valid refresh cookie', async () => {
    vi.mocked(api).mockResolvedValue({ accessToken: 'tok', user: { id: 'u1', email: 'a@b.test' } })
    const auth = useAuthStore()
    await auth.restore()
    expect(auth.authenticated).toBe(true)
  })

  it('restore clears state instead of throwing when there is no valid session', async () => {
    vi.mocked(api).mockRejectedValue({ code: 'UNAUTHORIZED', message: 'no session' })
    const auth = useAuthStore()
    auth.accessToken = 'stale'
    auth.user = { id: 'u1', email: 'a@b.test' }
    await expect(auth.restore()).resolves.toBeUndefined()
    expect(auth.accessToken).toBe('')
    expect(auth.user).toBeNull()
  })

  it('logout calls the API and clears local state', async () => {
    vi.mocked(api).mockResolvedValue(undefined)
    const auth = useAuthStore()
    auth.accessToken = 'tok'
    auth.user = { id: 'u1', email: 'a@b.test' }
    await auth.logout()
    expect(api).toHaveBeenCalledWith('/api/v1/auth/logout', { method: 'POST', body: '{}' })
    expect(auth.authenticated).toBe(false)
  })
})
