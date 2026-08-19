import { APIError } from '../../auth/sessionApi'
import { ProblemAlert } from '../../components/ProblemAlert'
import { useCacheStatusQuery } from '../../query/live'
import { FlushPanel } from './FlushPanel'
import { num, parseCacheStatus, yn } from './view'

function problemOf(err: unknown): { code?: string; detail?: string; message?: string } | null {
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

export function CachePage() {
  const cacheQuery = useCacheStatusQuery()
  const summary = parseCacheStatus(cacheQuery.data)
  const known = summary !== null

  return (
    <article className="dashboard">
      <h1>Cache</h1>
      <section>
        <h2>Status</h2>
        {cacheQuery.isPending && !cacheQuery.data ? (
          <p>Loading…</p>
        ) : cacheQuery.error ? (
          <ProblemAlert error={problemOf(cacheQuery.error)} />
        ) : (
          <dl>
            <dt>Enabled</dt>
            <dd>{yn(summary?.enabled, known)}</dd>
            <dt>Entries</dt>
            <dd>
              {num(summary?.entries, known)} / {num(summary?.maxEntries, known)}
            </dd>
            <dt>Hits</dt>
            <dd>{num(summary?.hits, known)}</dd>
            <dt>Misses</dt>
            <dd>{num(summary?.misses, known)}</dd>
            <dt>Evicts</dt>
            <dd>{num(summary?.evicts, known)}</dd>
          </dl>
        )}
      </section>
      <FlushPanel />
    </article>
  )
}
