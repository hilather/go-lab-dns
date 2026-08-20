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
  return { message: 'request failed' }
}

export function QueryError({ error }: { error: unknown }) {
  const problem = queryProblem(error)
  if (!problem) {
    return null
  }
  return <ProblemAlert error={problem} />
}
