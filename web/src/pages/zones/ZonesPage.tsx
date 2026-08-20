import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router'
import { CursorPager, MutationsPending, QueryError } from './ui'
import {
  DEFAULT_PAGE_LIMIT,
  fetchZoneList,
  usePagedCursor,
  useRuntimeRevision,
  zoneHref,
  zonesListKey,
} from './zones'

export function ZonesPage() {
  const revision = useRuntimeRevision()
  const [cursor, setCursor] = usePagedCursor(revision)

  const query = useQuery({
    queryKey: zonesListKey(revision, cursor, DEFAULT_PAGE_LIMIT),
    queryFn: () => fetchZoneList(cursor, DEFAULT_PAGE_LIMIT),
  })
  const page = query.data

  return (
    <article className="zones">
      <h1>Zones</h1>
      <MutationsPending>
        <button type="button" disabled>
          Create zone
        </button>
      </MutationsPending>
      {query.isError ? <QueryError error={query.error} /> : null}
      {query.isPending ? <p>Loading zones…</p> : null}
      {page && page.zones.length === 0 && !query.isPending ? <p>No zones.</p> : null}
      {page && page.zones.length > 0 ? (
        <table>
          <caption>Authoritative and overlay zones</caption>
          <thead>
            <tr>
              <th scope="col">ID</th>
              <th scope="col">Name</th>
              <th scope="col">Mode</th>
            </tr>
          </thead>
          <tbody>
            {page.zones.map((z) => (
              <tr key={z.id}>
                <td>
                  <Link to={zoneHref(z.id)}>{z.id}</Link>
                </td>
                <td>{z.name || '—'}</td>
                <td>{z.mode || '—'}</td>
              </tr>
            ))}
          </tbody>
        </table>
      ) : null}
      <CursorPager
        cursor={cursor}
        nextCursor={page?.nextCursor ?? ''}
        onFirst={() => setCursor('')}
        onNext={setCursor}
      />
    </article>
  )
}
