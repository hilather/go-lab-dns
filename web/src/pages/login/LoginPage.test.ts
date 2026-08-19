import { afterEach, describe, expect, it, vi } from 'vitest'
import { clear } from '../../auth/sessionMemory'
import { createSession } from '../../auth/sessionApi'
import { isLoopbackHost } from './LoginPage'

describe('login', () => {
  afterEach(() => {
    clear()
    vi.restoreAllMocks()
    vi.unstubAllGlobals()
  })

  it('treats loopback hostnames as local administrator eligible', () => {
    expect(isLoopbackHost('127.0.0.1')).toBe(true)
    expect(isLoopbackHost('localhost')).toBe(true)
    expect(isLoopbackHost('[::1]')).toBe(true)
    expect(isLoopbackHost('dns-mgmt.lab.example')).toBe(false)
  })

  it('discards the bearer after POST and never writes storage', async () => {
    const setItem = vi.spyOn(Storage.prototype, 'setItem')
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify({
          csrf: 'csrf-after-login',
          actor: { id: 'admin', class: 'ui-session' },
        }),
        { status: 200, headers: { 'Content-Type': 'application/json' } },
      ),
    )
    vi.stubGlobal('fetch', fetchMock)

    let bearer: string | undefined = 'paste-me-once'
    await createSession(bearer)
    bearer = undefined

    expect(bearer).toBeUndefined()
    expect(setItem).not.toHaveBeenCalled()
    expect(window.localStorage.getItem('token')).toBeNull()
    expect(window.sessionStorage.getItem('csrf')).toBeNull()
    const url = String(fetchMock.mock.calls[0]?.[0])
    expect(url).toBe('/v1/session')
  })
})
