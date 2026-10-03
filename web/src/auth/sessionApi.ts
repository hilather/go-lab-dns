import { clear, getCsrf, setCsrf } from './sessionMemory'

export const CSRF_HEADER = 'X-LabDNS-CSRF'

// Identity transitions invalidate every earlier response, including body parsing.
let sessionGen = 0
let mutationTail: Promise<void> | null = null
// Retain the known token for failed logout or superseded login cleanup.
let pendingRevocation = ''
let recoveryBlocked = false
// CSRF of a cookie whose sign-in body was unreadable; used only so logout can revoke it.
let orphanCsrf = ''

function enqueueMutation<T>(run: () => Promise<T>): Promise<T> {
  const result = mutationTail ? mutationTail.then(run) : run()
  const tail = result.then(() => undefined, () => undefined)
  mutationTail = tail
  void tail.then(() => { if (mutationTail === tail) mutationTail = null })
  return result
}

async function deleteWithCsrf(csrf: string): Promise<void> {
  const headers = new Headers()
  if (csrf) headers.set(CSRF_HEADER, csrf)
  const res = await fetch('/v1/session', { method: 'DELETE', credentials: 'include', headers })
  if (res.status === 204 || res.status === 401) return
  if (!res.ok) throw await readProblem(res)
}

async function cookieCsrf(): Promise<string> {
  try {
    const res = await fetch('/v1/session', { method: 'GET', credentials: 'include' })
    if (!res.ok) return ''
    const body = (await res.json()) as { csrf?: unknown }
    return typeof body.csrf === 'string' ? body.csrf : ''
  } catch {
    return ''
  }
}

async function revokePendingSession(): Promise<void> {
  if (!pendingRevocation) return
  await deleteWithCsrf(pendingRevocation)
  pendingRevocation = ''
  recoveryBlocked = false
}

function isAbortError(err: unknown): boolean {
  return err instanceof DOMException
    ? err.name === 'AbortError'
    : err instanceof Error && err.name === 'AbortError'
}

export type SessionActor = {
  id: string
  class: string
  role?: string
  scopes?: string[]
  groups?: string[]
}

export type SessionResponse = {
  csrf: string
  actor: SessionActor
}

export class APIError extends Error {
  readonly status: number
  readonly code: string
  readonly detail: string

  constructor(status: number, code: string, detail: string) {
    super(detail || code || 'request failed')
    this.name = 'APIError'
    this.status = status
    this.code = code
    this.detail = detail || this.message
  }
}

async function readProblem(res: Response): Promise<APIError> {
  let code = ''
  let detail = res.statusText
  try {
    const body = (await res.json()) as { code?: unknown; detail?: unknown }
    if (typeof body.code === 'string') {
      code = body.code
    }
    if (typeof body.detail === 'string') {
      detail = body.detail
    }
  } catch {
    // problem+json is best-effort; status still stands
  }
  return new APIError(res.status, code, detail)
}

async function parseSession(res: Response): Promise<SessionResponse> {
  const body = (await res.json()) as SessionResponse
  if (typeof body.csrf !== 'string' || body.csrf === '') {
    throw new APIError(res.status, 'invalid_value', 'session response missing csrf')
  }
  return body
}

export type GetSessionOpts = {
  signal?: AbortSignal
}

export async function getSession(opts?: GetSessionOpts): Promise<SessionResponse | null> {
  const gen = sessionGen
  const signal = opts?.signal
  if (mutationTail) await mutationTail
  if (pendingRevocation) await enqueueMutation(revokePendingSession)
  if (signal?.aborted || gen !== sessionGen || recoveryBlocked) return null
  let res: Response
  try {
    res = await fetch('/v1/session', { method: 'GET', credentials: 'include', signal })
  } catch (err) {
    if (signal?.aborted || isAbortError(err)) {
      return null
    }
    throw err
  }
  if (res.status === 401) {
    if (!signal?.aborted && gen === sessionGen) {
      clear()
    }
    return null
  }
  if (!res.ok) {
    throw await readProblem(res)
  }
  const body = await parseSession(res)
  if (signal?.aborted || gen !== sessionGen) {
    return null
  }
  setCsrf(body.csrf)
  return body
}

export async function createSession(bearer?: string): Promise<SessionResponse> {
  const gen = ++sessionGen
  return enqueueMutation(async () => {
    if (pendingRevocation && !bearer) await revokePendingSession()
    if (recoveryBlocked && !bearer) throw new APIError(409, 'session_recovery_blocked', 'explicit sign-in required after an incomplete session response')
    if (gen !== sessionGen) throw new DOMException('session changed during sign-in', 'AbortError')
    const headers = new Headers()
    if (bearer) headers.set('Authorization', `Bearer ${bearer}`)
    const csrf = getCsrf()
    if (csrf) headers.set(CSRF_HEADER, csrf)
    const res = await fetch('/v1/session', { method: 'POST', credentials: 'include', headers })
    if (!res.ok) throw await readProblem(res)
    recoveryBlocked = true
    clear()
    let body: SessionResponse
    try {
      body = await parseSession(res)
    } catch (err) {
      // The cookie is set but its CSRF is only in the unreadable body; learn it
      // from a cookie GET so logout can revoke that session.
      orphanCsrf = await cookieCsrf()
      throw err
    }
    if (gen !== sessionGen) {
      pendingRevocation = body.csrf
      await revokePendingSession()
      throw new DOMException('session changed during sign-in', 'AbortError')
    }
    pendingRevocation = ''
    recoveryBlocked = false
    orphanCsrf = ''
    setCsrf(body.csrf)
    return body
  })
}

export async function deleteSession(): Promise<void> {
  ++sessionGen
  const memoryCsrf = getCsrf()
  clear()
  return enqueueMutation(async () => {
    if (pendingRevocation) await revokePendingSession()
    // Read late so a queued sign-in has finished learning an orphan cookie's CSRF;
    // it wins over the call-time snapshot because it belongs to the newest cookie.
    const csrf = orphanCsrf || memoryCsrf
    pendingRevocation = csrf
    recoveryBlocked = true
    await deleteWithCsrf(csrf)
    pendingRevocation = ''
    recoveryBlocked = false
    orphanCsrf = ''
  })
}

export async function getJSON(path: string): Promise<unknown> {
  const gen = sessionGen
  const res = await fetch(path, { method: 'GET', credentials: 'include' })
  if (res.status === 401 && gen === sessionGen) {
    clear()
  }
  if (!res.ok) {
    throw await readProblem(res)
  }
  return res.json() as Promise<unknown>
}
