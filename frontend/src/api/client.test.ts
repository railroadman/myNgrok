// Covers the shared `api()` fetch wrapper used by every store/view:
// - a successful response unwraps and returns `body.data`
// - request options (method, headers, body) are forwarded to fetch as-is, merged with the default JSON header
// - a non-2xx response throws the server's `body.error`, or a generic REQUEST_FAILED error if none was sent
// - a response with an unparsable body (empty/non-JSON) is treated as `{}` instead of throwing on `.json()`
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { api } from './client'

describe('api client', () => {
  beforeEach(() => {
    vi.stubGlobal('fetch', vi.fn())
  })
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('returns the unwrapped data on a successful response', async () => {
    vi.mocked(fetch).mockResolvedValue(new Response(JSON.stringify({ data: { id: '1' } }), { status: 200 }))
    await expect(api('/api/v1/tunnels')).resolves.toEqual({ id: '1' })
  })

  it('forwards method, headers and body to fetch, defaulting Content-Type only when the caller sets no headers of its own', async () => {
    vi.mocked(fetch).mockResolvedValue(new Response(JSON.stringify({ data: null }), { status: 200 }))
    await api('/api/v1/tunnels', { method: 'POST', body: '{"name":"x"}' })
    expect(fetch).toHaveBeenLastCalledWith('/api/v1/tunnels', {
      credentials: 'include',
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: '{"name":"x"}',
    })

    // Note: `init` is spread after the default `headers`, so a caller-supplied `headers` object
    // (e.g. Authorization) replaces the default entirely rather than merging with it.
    await api('/api/v1/agent-tokens', { method: 'POST', headers: { Authorization: 'Bearer tok' }, body: '{"name":"x"}' })
    expect(fetch).toHaveBeenLastCalledWith('/api/v1/agent-tokens', {
      credentials: 'include',
      method: 'POST',
      headers: { Authorization: 'Bearer tok' },
      body: '{"name":"x"}',
    })
  })

  it('throws the server-provided error on a non-2xx response', async () => {
    vi.mocked(fetch).mockResolvedValue(new Response(JSON.stringify({ error: { code: 'UNAUTHORIZED', message: 'nope' } }), { status: 401 }))
    await expect(api('/api/v1/tunnels')).rejects.toEqual({ code: 'UNAUTHORIZED', message: 'nope' })
  })

  it('falls back to a generic error when a failed response carries no error payload', async () => {
    vi.mocked(fetch).mockResolvedValue(new Response('not json', { status: 500 }))
    await expect(api('/api/v1/tunnels')).rejects.toEqual({ code: 'REQUEST_FAILED', message: 'Request failed' })
  })
})
