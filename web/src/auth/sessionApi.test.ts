import { afterEach, describe, expect, it, vi } from 'vitest'
import { clear, getCsrf, setCsrf } from './sessionMemory'
import { createSession, CSRF_HEADER, deleteSession, getJSON, getSession } from './sessionApi'

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

describe('sessionApi storage', () => {
  afterEach(() => {
    clear()
    vi.restoreAllMocks()
    vi.unstubAllGlobals()
  })

  it('keeps CSRF in memory and never writes Web Storage', async () => {
    const setItem = vi.spyOn(Storage.prototype, 'setItem')
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(
        jsonResponse(200, {
          csrf: 'csrf-secret',
          actor: { id: 'loopback', class: 'ui-session', role: 'administrator', scopes: [] },
        }),
      ),
    )

    const sess = await createSession()
    expect(sess.csrf).toBe('csrf-secret')
    expect(getCsrf()).toBe('csrf-secret')
    expect(setItem).not.toHaveBeenCalled()
    expect(window.localStorage.length).toBe(0)
    expect(window.sessionStorage.length).toBe(0)
  })

  it('does not persist a bearer token when creating a session', async () => {
    const setItem = vi.spyOn(Storage.prototype, 'setItem')
    const fetchMock = vi.fn().mockResolvedValue(
      jsonResponse(200, {
        csrf: 'from-bearer',
        actor: { id: 'admin', class: 'ui-session', role: 'administrator' },
      }),
    )
    vi.stubGlobal('fetch', fetchMock)

    await createSession('super-secret-token')
    expect(getCsrf()).toBe('from-bearer')
    expect(setItem).not.toHaveBeenCalled()

    const req = fetchMock.mock.calls[0]?.[1] as RequestInit
    const headers = new Headers(req.headers)
    expect(headers.get('Authorization')).toBe('Bearer super-secret-token')
    const url = String(fetchMock.mock.calls[0]?.[0])
    expect(url).not.toContain('super-secret-token')
    expect(window.location.href).not.toContain('super-secret-token')
  })

  it('sends CSRF on DELETE and clears memory without storage writes', async () => {
    const setItem = vi.spyOn(Storage.prototype, 'setItem')
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(
        jsonResponse(200, { csrf: 'live-csrf', actor: { id: 'loopback', class: 'ui-session' } }),
      )
      .mockResolvedValueOnce(new Response(null, { status: 204 }))
    vi.stubGlobal('fetch', fetchMock)

    await createSession()
    await deleteSession()
    expect(getCsrf()).toBe('')
    expect(setItem).not.toHaveBeenCalled()
    const del = fetchMock.mock.calls[1]?.[1] as RequestInit
    expect(new Headers(del.headers).get(CSRF_HEADER)).toBe('live-csrf')
  })

  it('GET /v1/session 401 clears CSRF and does not touch storage', async () => {
    const setItem = vi.spyOn(Storage.prototype, 'setItem')
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(
        jsonResponse(401, { code: 'unauthenticated', detail: 'authentication required' }),
      ),
    )
    const sess = await getSession()
    expect(sess).toBeNull()
    expect(getCsrf()).toBe('')
    expect(setItem).not.toHaveBeenCalled()
  })

  it('getJSON does not write storage', async () => {
    const setItem = vi.spyOn(Storage.prototype, 'setItem')
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(200, { ready: true })))
    const body = await getJSON('/v1/status')
    expect(body).toEqual({ ready: true })
    expect(setItem).not.toHaveBeenCalled()
  })

  it('omits CSRF on first-login POST', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      jsonResponse(200, { csrf: 'first', actor: { id: 'loopback', class: 'ui-session' } }),
    )
    vi.stubGlobal('fetch', fetchMock)
    await createSession()
    const headers = new Headers((fetchMock.mock.calls[0]?.[1] as RequestInit).headers)
    expect(headers.get(CSRF_HEADER)).toBeNull()
  })

  it('sends CSRF on cookie-present POST when memory has a token', async () => {
    setCsrf('existing-csrf')
    const fetchMock = vi.fn().mockResolvedValue(
      jsonResponse(200, { csrf: 'rotated', actor: { id: 'loopback', class: 'ui-session' } }),
    )
    vi.stubGlobal('fetch', fetchMock)
    await createSession()
    const headers = new Headers((fetchMock.mock.calls[0]?.[1] as RequestInit).headers)
    expect(headers.get(CSRF_HEADER)).toBe('existing-csrf')
    expect(getCsrf()).toBe('rotated')
  })

  it('in-flight GET 401 after createSession does not clear the new CSRF', async () => {
    let resolveGet: ((value: Response) => void) | undefined
    const getPending = new Promise<Response>((resolve) => {
      resolveGet = resolve
    })
    const fetchMock = vi.fn((_url: string, init?: RequestInit) => {
      if ((init?.method ?? 'GET') === 'GET') {
        return getPending
      }
      return Promise.resolve(
        jsonResponse(200, { csrf: 'new-csrf', actor: { id: 'loopback', class: 'ui-session' } }),
      )
    })
    vi.stubGlobal('fetch', fetchMock)

    const inFlight = getSession()
    await createSession()
    expect(getCsrf()).toBe('new-csrf')
    resolveGet?.(jsonResponse(401, { code: 'unauthenticated', detail: 'authentication required' }))
    await inFlight
    expect(getCsrf()).toBe('new-csrf')
  })

  it('aborted GET does not clear CSRF', async () => {
    setCsrf('keep-me')
    const ac = new AbortController()
    vi.stubGlobal(
      'fetch',
      vi.fn((_url: string, init?: RequestInit) => {
        return new Promise<Response>((_resolve, reject) => {
          init?.signal?.addEventListener('abort', () => {
            reject(new DOMException('aborted', 'AbortError'))
          })
        })
      }),
    )
    const pending = getSession({ signal: ac.signal })
    ac.abort()
    await expect(pending).resolves.toBeNull()
    expect(getCsrf()).toBe('keep-me')
  })
})
