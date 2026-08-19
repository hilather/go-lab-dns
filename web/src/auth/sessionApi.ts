import { clear, getCsrf, setCsrf } from './sessionMemory'

export const CSRF_HEADER = 'X-LabDNS-CSRF'

// Bumped on createSession so a stale GET 401 cannot clear() a newer CSRF.
let sessionGen = 0

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
  setCsrf(body.csrf)
  return body
}

export type GetSessionOpts = {
  signal?: AbortSignal
}

export async function getSession(opts?: GetSessionOpts): Promise<SessionResponse | null> {
  const gen = sessionGen
  const signal = opts?.signal
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
  return parseSession(res)
}

export async function createSession(bearer?: string): Promise<SessionResponse> {
  sessionGen += 1
  const headers = new Headers()
  if (bearer) {
    headers.set('Authorization', `Bearer ${bearer}`)
  }
  const csrf = getCsrf()
  if (csrf !== '') {
    headers.set(CSRF_HEADER, csrf)
  }
  const res = await fetch('/v1/session', {
    method: 'POST',
    credentials: 'include',
    headers,
  })
  if (!res.ok) {
    throw await readProblem(res)
  }
  return parseSession(res)
}

export async function deleteSession(): Promise<void> {
  const headers = new Headers()
  const csrf = getCsrf()
  if (csrf !== '') {
    headers.set(CSRF_HEADER, csrf)
  }
  const res = await fetch('/v1/session', {
    method: 'DELETE',
    credentials: 'include',
    headers,
  })
  clear()
  if (res.status === 204 || res.status === 401) {
    return
  }
  if (!res.ok) {
    throw await readProblem(res)
  }
}

export async function getJSON(path: string): Promise<unknown> {
  const res = await fetch(path, { method: 'GET', credentials: 'include' })
  if (res.status === 401) {
    clear()
  }
  if (!res.ok) {
    throw await readProblem(res)
  }
  return res.json() as Promise<unknown>
}
