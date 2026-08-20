import { useQuery } from '@tanstack/react-query'
import { Link, useParams } from 'react-router'
import { queryKeys } from '../../query/keys'
import { ROUTES } from '../../routes'
import { CursorPager, MutationsPending, QueryError } from './ui'
import {
  DEFAULT_PAGE_LIMIT,
  fetchRecordList,
  fetchZone,
  recordHref,
  recordsListKey,
  usePagedCursor,
  useRuntimeRevision,
} from './zones'

export function ZoneDetailPage() {
  const { zoneId = '' } = useParams()
  const revision = useRuntimeRevision()
  const [cursor, setCursor] = usePagedCursor(`${revision}\0${zoneId}`)

  const zoneQuery = useQuery({
    queryKey: queryKeys.zone(revision, zoneId),
    queryFn: () => fetchZone(zoneId),
    enabled: zoneId !== '',
  })
  const recordsQuery = useQuery({
    queryKey: recordsListKey(revision, zoneId, cursor, DEFAULT_PAGE_LIMIT),
    queryFn: () => fetchRecordList(zoneId, cursor, DEFAULT_PAGE_LIMIT),
    enabled: zoneId !== '',
  })
  const zone = zoneQuery.data
  const records = recordsQuery.data

  return (
    <article className="zone-detail">
      <p>
        <Link to={ROUTES.zones}>Zones</Link>
      </p>
      <h1>{zone?.id || zoneId || 'Zone'}</h1>
      {zoneQuery.isError ? <QueryError error={zoneQuery.error} /> : null}
      {zoneQuery.isPending ? <p>Loading zone…</p> : null}
      {zone ? (
        <dl>
          <dt>ID</dt>
          <dd>{zone.id}</dd>
          <dt>Name</dt>
          <dd>{zone.name || '—'}</dd>
          <dt>Mode</dt>
          <dd>{zone.mode || '—'}</dd>
          <dt>Nameservers</dt>
          <dd>{zone.nameservers.length > 0 ? zone.nameservers.join(', ') : '—'}</dd>
        </dl>
      ) : null}
      <MutationsPending>
        <button type="button" disabled>
          Edit zone
        </button>
        <button type="button" disabled>
          Delete zone
        </button>
      </MutationsPending>

      <h2>Records</h2>
      <MutationsPending>
        <button type="button" disabled>
          Create record
        </button>
      </MutationsPending>
      {recordsQuery.isError ? <QueryError error={recordsQuery.error} /> : null}
      {recordsQuery.isPending ? <p>Loading records…</p> : null}
      {records && records.records.length === 0 && !recordsQuery.isPending ? (
        <p>No records.</p>
      ) : null}
      {records && records.records.length > 0 ? (
        <table>
          <caption>Records in this zone</caption>
          <thead>
            <tr>
              <th scope="col">Owner</th>
              <th scope="col">Type</th>
              <th scope="col">TTL</th>
              <th scope="col">ID</th>
            </tr>
          </thead>
          <tbody>
            {records.records.map((r) => (
              <tr key={r.id}>
                <td>{r.owner || '—'}</td>
                <td>{r.type || '—'}</td>
                <td>{r.ttl || '—'}</td>
                <td>
                  <Link to={recordHref(zoneId, r.id)}>{r.id}</Link>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      ) : null}
      <CursorPager
        cursor={cursor}
        nextCursor={records?.nextCursor ?? ''}
        onFirst={() => setCursor('')}
        onNext={setCursor}
      />
    </article>
  )
}
