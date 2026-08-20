import type { ReactNode } from 'react'
import { APIError } from '../../auth/sessionApi'
import { ProblemAlert } from '../../components/ProblemAlert'
import { MUTATIONS_UI003 } from './view'

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

export function MutationsPending({ children }: { children: ReactNode }) {
  return (
    <p className="mutations-pending">
      {children} <span>{MUTATIONS_UI003}</span>
    </p>
  )
}
