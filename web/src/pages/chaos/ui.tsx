import { APIError } from '../../auth/sessionApi'
import { ProblemAlert } from '../../components/ProblemAlert'

export function queryProblem(err: unknown): { code?: string; detail?: string; message?: string } | null {
  if (!err) {
    return null
  }
  if (err instanceof APIError) {
    return { code: err.code, detail: err.detail, message: err.message }
  }
  if (err instanceof Error) {
    return { message: err.message }
  }
  if (typeof err === 'object') {
    const o = err as { code?: unknown; detail?: unknown; message?: unknown }
    const code = typeof o.code === 'string' ? o.code : undefined
    const detail = typeof o.detail === 'string' ? o.detail : undefined
    const message = typeof o.message === 'string' ? o.message : undefined
    if (code || detail || message) {
      return { code, detail, message }
    }
  }
  return { message: 'request failed' }
}

export function QueryError({ error }: { error: unknown }) {
  const problem = queryProblem(error)
  if (!problem) {
    return null
  }
  return <ProblemAlert error={problem} />
}
