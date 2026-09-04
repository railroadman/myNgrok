// Covers the global navigation guard in `beforeEach`, which gates every route:
// - a route with `meta.requiresAuth` redirects to /login when no session can be restored
// - /login and /register redirect straight to /app once a session IS restored (no re-login form for logged-in users)
// - navigating to an authenticated route succeeds (no redirect) once a session is restored
// Session state comes from `auth.restore()`, which is driven here through the mocked `api` client
// so each test can simulate "has a valid refresh cookie" vs. "does not" without a real backend.
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

vi.mock('../api/client', () => ({ api: vi.fn() }))
import { api } from '../api/client'
import router from './index'

describe('router auth guard', () => {
  beforeEach(async () => {
    setActivePinia(createPinia())
    vi.mocked(api).mockReset()
    await router.push('/')
  })

  it('redirects an unauthenticated visitor away from a protected route to /login', async () => {
    vi.mocked(api).mockRejectedValue({ code: 'UNAUTHORIZED', message: 'no session' })
    await router.push('/app')
    expect(router.currentRoute.value.path).toBe('/login')
  })

  it('lets an authenticated visitor reach a protected route', async () => {
    vi.mocked(api).mockResolvedValue({ accessToken: 'tok', user: { id: 'u1', email: 'a@b.test' } })
    await router.push('/app')
    expect(router.currentRoute.value.path).toBe('/app')
  })

  it('redirects an authenticated visitor away from /login to /app', async () => {
    vi.mocked(api).mockResolvedValue({ accessToken: 'tok', user: { id: 'u1', email: 'a@b.test' } })
    await router.push('/login')
    expect(router.currentRoute.value.path).toBe('/app')
  })

  it('lets an unauthenticated visitor reach /register', async () => {
    vi.mocked(api).mockRejectedValue({ code: 'UNAUTHORIZED', message: 'no session' })
    await router.push('/register')
    expect(router.currentRoute.value.path).toBe('/register')
  })
})
