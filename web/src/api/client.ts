import createClient, { type ClientOptions, type Middleware } from 'openapi-fetch'
import { APIError, CSRF_HEADER } from '../auth/sessionApi'
import { getCsrf } from '../auth/sessionMemory'
import type { paths } from './openapi'

export type { paths }
export type { components, operations } from './openapi'

export function defaultBaseUrl(): string {
  if (typeof window !== 'undefined' && window.location?.origin) {
    return window.location.origin
  }
  return 'http://127.0.0.1:8080'
}

export const csrfMiddleware: Middleware = {
  async onRequest({ request }) {
    const method = request.method.toUpperCase()
    if (method !== 'GET' && method !== 'HEAD' && method !== 'OPTIONS') {
      const csrf = getCsrf()
      if (csrf !== '') {
        request.headers.set(CSRF_HEADER, csrf)
      }
    }
    return request
  },
}

export function createLabdnsClient(options?: ClientOptions) {
  const client = createClient<paths>({
    baseUrl: defaultBaseUrl(),
    ...options,
    credentials: 'include',
  })
  client.use(csrfMiddleware)
  return client
}

export const client = createLabdnsClient()

export function throwOnError<T>(result: {
  data?: T
  error?: unknown
  response: Response
}): T {
  if (result.response.ok) {
    return result.data as T
  }
  let code = ''
  let detail = result.response.statusText
  if (result.error && typeof result.error === 'object') {
    const problem = result.error as { code?: unknown; detail?: unknown }
    if (typeof problem.code === 'string') {
      code = problem.code
    }
    if (typeof problem.detail === 'string') {
      detail = problem.detail
    }
  }
  throw new APIError(result.response.status, code, detail)
}
