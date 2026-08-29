export type ApiError = { code: string; message: string }

export async function api<T>(path: string, init: RequestInit = {}): Promise<T> {
  const response = await fetch(path, {
    credentials: 'include',
    headers: { 'Content-Type': 'application/json', ...init.headers },
    ...init,
  })
  const body = await response.json().catch(() => ({}))
  if (!response.ok) throw (body.error ?? { code: 'REQUEST_FAILED', message: 'Request failed' }) as ApiError
  return body.data as T
}
