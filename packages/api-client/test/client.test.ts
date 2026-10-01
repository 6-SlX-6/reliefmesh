import { describe, expect, it, vi } from 'vitest'
import { ApiError, createApiClient, isApiError, isNetworkError } from '../src/index'

function jsonResponse(status: number, body: unknown, headers: Record<string, string> = {}) {
  return new Response(body === undefined ? null : JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json', ...headers },
  })
}

describe('api client', () => {
  it('sends the CSRF token on state-changing requests only', async () => {
    const fetchMock = vi.fn(async () => jsonResponse(200, { items: [] }))
    const api = createApiClient({ fetch: fetchMock as unknown as typeof fetch, getCsrfToken: () => 'tok' })
    await api.requests.list({ open: true, q: '' })
    await api.requests.changeStatus('abc', { status: 'verified' })
    const [getUrl, getInit] = fetchMock.mock.calls[0] as unknown as [string, RequestInit]
    const [postUrl, postInit] = fetchMock.mock.calls[1] as unknown as [string, RequestInit]
    expect(getUrl).toBe('/api/v1/requests?open=true')
    expect((getInit.headers as Record<string, string>)['X-CSRF-Token']).toBeUndefined()
    expect(postUrl).toBe('/api/v1/requests/abc/status')
    expect((postInit.headers as Record<string, string>)['X-CSRF-Token']).toBe('tok')
    expect(postInit.credentials).toBe('same-origin')
    expect(postInit.body).toBe(JSON.stringify({ status: 'verified' }))
  })

  it('maps API errors with fields and codes', async () => {
    const fetchMock = vi.fn(async () =>
      jsonResponse(422, { error: { code: 'validation_failed', message: 'Some fields are invalid.', fields: { title: 'Required' } } }),
    )
    const api = createApiClient({ fetch: fetchMock as unknown as typeof fetch })
    const err = await api.requests.create({} as never).catch((e) => e)
    expect(err).toBeInstanceOf(ApiError)
    expect(isApiError(err, 'validation_failed')).toBe(true)
    expect(err.fields.title).toBe('Required')
    expect(err.status).toBe(422)
  })

  it('reports network failures as NetworkError', async () => {
    const api = createApiClient({ fetch: (async () => { throw new TypeError('Failed to fetch') }) as unknown as typeof fetch })
    const err = await api.dashboard().catch((e) => e)
    expect(isNetworkError(err)).toBe(true)
  })

  it('treats proxy gateway errors without JSON as network errors', async () => {
    const api = createApiClient({ fetch: (async () => new Response('Bad Gateway', { status: 502, headers: { 'Content-Type': 'text/html' } })) as unknown as typeof fetch })
    expect(isNetworkError(await api.dashboard().catch((e) => e))).toBe(true)
  })

  it('invokes onUnauthorized for 401 responses', async () => {
    const onUnauthorized = vi.fn()
    const api = createApiClient({
      fetch: (async () => jsonResponse(401, { error: { code: 'unauthorized', message: 'Please sign in.' } })) as unknown as typeof fetch,
      onUnauthorized,
    })
    await api.auth.session().catch(() => undefined)
    expect(onUnauthorized).toHaveBeenCalledOnce()
  })

  it('encodes path parameters', async () => {
    const fetchMock = vi.fn(async () => jsonResponse(200, {}))
    const api = createApiClient({ fetch: fetchMock as unknown as typeof fetch })
    await api.requests.get('../admin')
    expect((fetchMock.mock.calls[0] as unknown as [string])[0]).toBe('/api/v1/requests/..%2Fadmin')
  })

  it('returns undefined for 204 responses', async () => {
    const api = createApiClient({ fetch: (async () => new Response(null, { status: 204 })) as unknown as typeof fetch })
    await expect(api.auth.logout()).resolves.toBeUndefined()
  })
})
