import { afterEach, expect, it, vi } from 'vitest'
import { createSession, deleteSession, getSession, CSRF_HEADER } from './sessionApi'
import { clear, getCsrf } from './sessionMemory'

function cookieServer() {
  let cookie = ''
  let serial = 0
  let holdPost = false
  let holdDelete = false
  let failureStatus: number | 'network' = 503
  let failures = 0
  let invalidBody = false
  const deliveries: (() => void)[] = []
  const sessions = new Map<string, { csrf: string; actor: { id: string; class: string } }>()
  const methods: string[] = []
  vi.stubGlobal('fetch', vi.fn((_url: string, init: RequestInit) => {
    const method = init.method ?? 'GET'
    methods.push(method)
    if (method === 'POST') {
      const id = `session-${++serial}`
      const body = { csrf: `csrf-${id}`, actor: { id, class: 'ui-session' } }
      sessions.set(id, body)
      return new Promise<Response>((resolve) => {
        const deliver = () => { cookie = id; resolve(Response.json(invalidBody ? {} : body)) }
        if (holdPost) deliveries.push(deliver)
        else deliver()
      })
    }
    const session = sessions.get(cookie)
    if (method === 'GET') return Promise.resolve(session ? Response.json(session) : new Response(null, { status: 401 }))
    if (failures-- > 0) return failureStatus === 'network' ? Promise.reject(new TypeError('network unavailable')) : Promise.resolve(new Response(null, { status: failureStatus }))
    if (!session) return Promise.resolve(new Response(null, { status: 401 }))
    expect(new Headers(init.headers).get('Authorization')).toBeNull()
    if (new Headers(init.headers).get(CSRF_HEADER) !== session.csrf) return Promise.resolve(new Response(null, { status: 403 }))
    sessions.delete(cookie)
    return new Promise<Response>((resolve) => {
      const deliver = () => { cookie = ''; resolve(new Response(null, { status: 204 })) }
      if (holdDelete) deliveries.push(deliver)
      else deliver()
    })
  }))
  return {
    methods, sessions,
    invalidBody: (value: boolean) => { invalidBody = value },
    holdPost: () => { holdPost = true }, releasePosts: () => { holdPost = false },
    holdDelete: () => { holdDelete = true }, failDeletes: (count: number, status: number | 'network' = 503) => { failures = count; failureStatus = status },
    deliver: () => { const deliver = deliveries.shift(); expect(deliver).toBeDefined(); deliver!() },
    pending: () => deliveries.length,
  }
}
afterEach(() => { clear(); vi.unstubAllGlobals() })

it('revokes a late successful login cookie before logout or recovery', async () => {
  const server = cookieServer()
  server.holdPost()
  const login = createSession('old')
  const rejected = expect(login).rejects.toMatchObject({ name: 'AbortError' })
  const logout = deleteSession()
  expect(server.methods).toEqual(['POST'])
  server.deliver()
  await rejected
  await logout
  expect(server.sessions.size).toBe(0)
  expect(await getSession()).toBeNull()
  expect(getCsrf()).toBe('')
})

it('waits for logout cookie expiration before sending a newer login', async () => {
  const server = cookieServer()
  await createSession('old')
  server.holdDelete()
  const logout = deleteSession()
  const login = createSession('new')
  expect(server.methods).toEqual(['POST', 'DELETE'])
  server.deliver()
  await logout
  const newer = await login
  expect((await getSession())?.actor.id).toBe(newer.actor.id)
})

it('blocks recovery while stale-cookie revocation is delivering', async () => {
  const server = cookieServer()
  server.holdPost()
  server.holdDelete()
  const login = createSession('old')
  const rejected = expect(login).rejects.toMatchObject({ name: 'AbortError' })
  const logout = deleteSession()
  server.deliver()
  await vi.waitFor(() => expect(server.pending()).toBe(1))
  const recovery = getSession()
  expect(server.methods).not.toContain('GET')
  server.deliver()
  await rejected
  await logout
  expect(await recovery).toBeNull()
})

it('keeps failed cleanup fail closed until an explicit new login succeeds', async () => {
  const server = cookieServer()
  server.holdPost()
  server.failDeletes(20)
  const login = createSession('old')
  const loginFailure = expect(login).rejects.toMatchObject({ status: 503 })
  const logout = deleteSession()
  const logoutFailure = expect(logout).rejects.toMatchObject({ status: 503 })
  server.deliver()
  await loginFailure
  await logoutFailure
  await expect(getSession()).rejects.toMatchObject({ status: 503 })
  expect(server.methods).not.toContain('GET')
  expect(getCsrf()).toBe('')
  server.releasePosts()
  const newer = await createSession('new')
  const afterLogin = server.methods.length
  expect((await getSession())?.actor.id).toBe(newer.actor.id)
  expect(server.methods.slice(afterLogin)).toEqual(['GET'])
})

it.each([503, 403, 'network'] as const)('does not recover an old cookie after logout failure %s', async (status) => {
  const server = cookieServer()
  await createSession('old')
  server.failDeletes(20, status)
  await expect(deleteSession()).rejects.toThrow()
  await expect(getSession()).rejects.toThrow()
  expect(server.methods).not.toContain('GET')
  const newer = await createSession('new')
  expect((await getSession())?.actor.id).toBe(newer.actor.id)
})

it('retries failed revocation before cookie-authenticated recovery', async () => {
  const server = cookieServer()
  await createSession('old')
  server.failDeletes(1)
  await expect(deleteSession()).rejects.toMatchObject({ status: 503 })
  expect(await getSession()).toBeNull()
  expect(server.sessions.size).toBe(0)
})

it('skips superseded queued logins without revoking the final login', async () => {
  const server = cookieServer()
  server.holdPost()
  const first = createSession('first')
  const firstFailure = expect(first).rejects.toMatchObject({ name: 'AbortError' })
  const skipped = createSession('skipped')
  const skippedFailure = expect(skipped).rejects.toMatchObject({ name: 'AbortError' })
  const logout = deleteSession()
  const final = createSession('final')
  server.releasePosts()
  server.deliver()
  await firstFailure
  await skippedFailure
  await logout
  const session = await final
  expect(server.methods.filter((method) => method === 'POST')).toHaveLength(2)
  expect((await getSession())?.actor.id).toBe(session.actor.id)
})


it('blocks recovery after Set-Cookie with an invalid login body', async () => {
  const server = cookieServer()
  server.invalidBody(true)
  await expect(createSession('old')).rejects.toMatchObject({ code: 'invalid_value' })
  expect(await getSession()).toBeNull()
  expect(server.methods).toEqual(['POST'])
  server.invalidBody(false)
  const newer = await createSession('new')
  expect((await getSession())?.actor.id).toBe(newer.actor.id)
})
