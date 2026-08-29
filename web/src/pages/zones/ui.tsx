import { APIError } from '../../auth/sessionApi'
import { ProblemAlert } from '../../components/ProblemAlert'

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

export function CursorPager({
  cursor,
  nextCursor,
  onFirst,
  onNext,
  firstLabel = 'First page',
  nextLabel = 'Next',
}: {
  cursor: string
  nextCursor: string
  onFirst: () => void
  onNext: (cursor: string) => void
  firstLabel?: string
  nextLabel?: string
}) {
  if (cursor === '' && nextCursor === '') {
    return null
  }
  return (
    <p className="cursor-pager">
      {cursor !== '' ? (
        <button type="button" onClick={onFirst}>
          {firstLabel}
        </button>
      ) : null}{' '}
      {nextCursor !== '' ? (
        <button type="button" onClick={() => onNext(nextCursor)}>
          {nextLabel}
        </button>
      ) : null}
    </p>
  )
}
