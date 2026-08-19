import { afterEach, describe, expect, it, vi } from 'vitest'
import { CSRF_HEADER } from '../auth/sessionApi'
import { clear, setCsrf } from '../auth/sessionMemory'
import { createLabdnsClient } from './client'

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

function requestOf(call: unknown[] | undefined): Request {
  const first = call?.[0]
  if (first instanceof Request) {
    return first
  }
  return new Request(String(first), (call?.[1] ?? {}) as RequestInit)
}

describe('openapi client', () => {
  afterEach(() => {
    clear()
    vi.restoreAllMocks()
  })

  it('sends credentials include and CSRF on mutating requests', async () => {
    setCsrf('csrf-secret')
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse(200, {}))
    const api = createLabdnsClient({ fetch: fetchMock })
    await api.POST('/v1/state:validate', { body: {} })
    expect(fetchMock).toHaveBeenCalled()
    const req = requestOf(fetchMock.mock.calls[0] as unknown[])
    expect(req.credentials).toBe('include')
    expect(req.headers.get(CSRF_HEADER)).toBe('csrf-secret')
    expect(req.method).toBe('POST')
  })

  it('omits CSRF on GET', async () => {
    setCsrf('csrf-secret')
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse(200, { ready: true }))
    const api = createLabdnsClient({ fetch: fetchMock })
    await api.GET('/v1/status')
    const req = requestOf(fetchMock.mock.calls[0] as unknown[])
    expect(req.credentials).toBe('include')
    expect(req.headers.get(CSRF_HEADER)).toBeNull()
    expect(req.method).toBe('GET')
  })

  it('does not write CSRF to Web Storage', async () => {
    const setItem = vi.spyOn(Storage.prototype, 'setItem')
    setCsrf('csrf-secret')
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse(200, {}))
    const api = createLabdnsClient({ fetch: fetchMock })
    await api.POST('/v1/cache:flush', { body: {} })
    expect(setItem).not.toHaveBeenCalled()
  })
})
