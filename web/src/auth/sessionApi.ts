import { clear, getCsrf, setCsrf } from './sessionMemory'

export const CSRF_HEADER = 'X-LabDNS-CSRF'

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

export async function getSession(): Promise<SessionResponse | null> {
  const res = await fetch('/v1/session', { method: 'GET', credentials: 'include' })
  if (res.status === 401) {
    clear()
    return null
  }
  if (!res.ok) {
    throw await readProblem(res)
  }
  return parseSession(res)
}

export async function createSession(bearer?: string): Promise<SessionResponse> {
  const headers = new Headers()
  if (bearer) {
    headers.set('Authorization', `Bearer ${bearer}`)
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
