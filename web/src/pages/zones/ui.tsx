import type { ReactNode } from 'react'
import { APIError } from '../../auth/sessionApi'
import { ProblemAlert } from '../../components/ProblemAlert'
import { MUTATIONS_UI003 } from './zones'

export function queryProblem(err: unknown): { code?: string; detail?: string } | null {
  if (!err) {
    return null
  }
  if (err instanceof APIError) {
    return { code: err.code, detail: err.detail }
  }
  if (err instanceof Error) {
    return { detail: err.message }
  }
  return { detail: 'request failed' }
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

export function CursorPager({
  cursor,
  nextCursor,
  onFirst,
  onNext,
}: {
  cursor: string
  nextCursor: string
  onFirst: () => void
  onNext: (cursor: string) => void
}) {
  if (cursor === '' && nextCursor === '') {
    return null
  }
  return (
    <p className="cursor-pager">
      {cursor !== '' ? (
        <button type="button" onClick={onFirst}>
          First page
        </button>
      ) : null}{' '}
      {nextCursor !== '' ? (
        <button type="button" onClick={() => onNext(nextCursor)}>
          Next
        </button>
      ) : null}
    </p>
  )
}
