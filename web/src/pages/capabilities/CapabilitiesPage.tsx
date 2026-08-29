import { useQuery } from '@tanstack/react-query'
import { useOutletContext } from 'react-router'
import { ProblemAlert } from '../../components/ProblemAlert'
import type { ShellContext } from '../../components/Shell'
import { queryKeys } from '../../query/keys'
import { statusRevision, useStatusQuery } from '../../query/status'
import { capabilityList, fetchCapabilities, problemFrom } from './capabilities'

function yn(v: boolean | undefined): string {
  if (v === undefined) {
    return '—'
  }
  return v ? 'Yes' : 'No'
}

export function CapabilitiesPage() {
  const status = useStatusQuery()
  const outlet = useOutletContext<ShellContext | null>()
  const revision = statusRevision(status.data) || outlet?.status?.revisions?.runtimeRevision || ''
  const query = useQuery({
    queryKey: queryKeys.capabilities(revision),
    queryFn: () => fetchCapabilities(),
    enabled: revision !== '',
  })
  const err = problemFrom(query.error)
  const rows = capabilityList(query.data)

  return (
    <article className="dashboard">
      <div className="page-head">
        <div>
          <h1>Capabilities</h1>
          <p className="page-lede">Registry rows the console uses for scope gating and parity.</p>
        </div>
      </div>
      {err ? <ProblemAlert error={err} /> : null}
      {revision === '' || query.isFetching ? <p className="empty">Loading capabilities…</p> : null}
      {query.isSuccess && rows.length === 0 ? <p className="empty">No capabilities reported.</p> : null}
      {rows.length > 0 ? (
        <section className="surface">
        <table className="data-table">
          <thead>
            <tr>
              <th scope="col">Name</th>
              <th scope="col">Version</th>
              <th scope="col">Mutating</th>
              <th scope="col">Idempotent</th>
              <th scope="col">Description</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((row, i) => (
              <tr key={row.name || String(i)}>
                <td>{row.name || '—'}</td>
                <td>{row.version || '—'}</td>
                <td>{yn(row.mutating)}</td>
                <td>{yn(row.idempotent)}</td>
                <td>{row.description || '—'}</td>
              </tr>
            ))}
          </tbody>
        </table>
        </section>
      ) : null}
    </article>
  )
}
