import { useQuery } from '@tanstack/react-query'
import { ProblemAlert } from '../../components/ProblemAlert'
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
  const revision = statusRevision(status.data)
  const query = useQuery({
    queryKey: queryKeys.capabilities(revision),
    queryFn: () => fetchCapabilities(),
  })
  const err = problemFrom(query.error)
  const rows = capabilityList(query.data)

  return (
    <article className="dashboard">
      <h1>Capabilities</h1>
      {err ? <ProblemAlert error={err} /> : null}
      {query.isPending ? <p>Loading capabilities…</p> : null}
      {query.isSuccess && rows.length === 0 ? <p>No capabilities reported.</p> : null}
      {rows.length > 0 ? (
        <table>
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
      ) : null}
    </article>
  )
}
