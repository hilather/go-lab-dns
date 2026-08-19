import { useQuery } from '@tanstack/react-query'
import { Link, useParams } from 'react-router'
import { queryKeys } from '../../query/keys'
import { MutationsPending, QueryError } from './ui'
import { fetchRecord, useRuntimeRevision, zoneHref } from './zones'

export function RecordDetailPage() {
  const { zoneId = '', recordId = '' } = useParams()
  const revision = useRuntimeRevision()
  const query = useQuery({
    queryKey: queryKeys.record(revision, zoneId, recordId),
    queryFn: () => fetchRecord(zoneId, recordId),
    enabled: zoneId !== '' && recordId !== '',
  })
  const rec = query.data

  return (
    <article className="record-detail">
      <p>
        <Link to={zoneHref(zoneId)}>{zoneId || 'Zone'}</Link>
      </p>
      <h1>{rec?.id || recordId || 'Record'}</h1>
      {query.isError ? <QueryError error={query.error} /> : null}
      {query.isPending ? <p>Loading record…</p> : null}
      {rec ? (
        <dl>
          <dt>ID</dt>
          <dd>{rec.id}</dd>
          <dt>Owner</dt>
          <dd>{rec.owner || '—'}</dd>
          <dt>Type</dt>
          <dd>{rec.type || '—'}</dd>
          <dt>TTL</dt>
          <dd>{rec.ttl || '—'}</dd>
          <dt>Values</dt>
          <dd>{rec.values.length > 0 ? rec.values.join(', ') : '—'}</dd>
          <dt>Chaos policy refs</dt>
          <dd>{rec.chaosPolicyRefs.length > 0 ? rec.chaosPolicyRefs.join(', ') : '—'}</dd>
        </dl>
      ) : null}
      <MutationsPending>
        <button type="button" disabled>
          Edit record
        </button>
        <button type="button" disabled>
          Delete record
        </button>
      </MutationsPending>
    </article>
  )
}
