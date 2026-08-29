import { useQuery } from '@tanstack/react-query'
import { Link, useNavigate, useParams } from 'react-router'
import { ScopeGate } from '../../components/ScopeGate'
import { hasWriteScope } from '../changes/changeIn'
import {
  parseSessionActor,
  scopeGateAllowed,
  useSessionActorQuery,
} from '../chaos/view'
import { queryKeys } from '../../query/keys'
import { QueryError } from './ui'
import {
  deleteRecordOperation,
  editRecordOperation,
  fetchRecordBundle,
  goToChanges,
  useRuntimeRevision,
  zoneHref,
} from './zones'

export function RecordDetailPage() {
  const { zoneId = '', recordId = '' } = useParams()
  const navigate = useNavigate()
  const revision = useRuntimeRevision()
  const sessionQuery = useSessionActorQuery()
  const actor = parseSessionActor(sessionQuery.data)
  const sessionKnown = sessionQuery.isSuccess || sessionQuery.isError
  const canWrite = hasWriteScope(actor)
  const writeAllowed = scopeGateAllowed(sessionKnown, canWrite)

  const query = useQuery({
    queryKey: queryKeys.record(revision, zoneId, recordId),
    queryFn: () => fetchRecordBundle(zoneId, recordId),
    enabled: zoneId !== '' && recordId !== '',
  })
  const rec = query.data?.view
  const raw = query.data?.raw
  const rawReady = raw !== undefined && raw !== null && typeof raw === 'object'
  const hopReady = sessionKnown && canWrite
  const editReady = hopReady && rawReady

  return (
    <article className="record-detail">
      <p>
        <Link to={zoneHref(zoneId)}>{zoneId || 'Zone'}</Link>
      </p>
      <h1>{rec?.id || recordId || 'Record'}</h1>
      {query.isError ? <QueryError error={query.error} /> : null}
      {query.isPending ? <p>Loading record…</p> : null}
      {rec ? (
        <dl className="zone-meta">
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
          <dd>
            {rec.chaosPolicyRefs.length > 0 ? (
              rec.chaosPolicyRefs.map((ref) => (
                <span key={ref} className="chaos-ref">
                  {ref}
                </span>
              ))
            ) : (
              '—'
            )}
          </dd>
        </dl>
      ) : null}
      <p className="zone-actions">
        <ScopeGate allowed={writeAllowed} missingScope="dns.write">
          <button
            type="button"
            disabled={!editReady}
            onClick={() =>
              goToChanges(navigate, [editRecordOperation(zoneId, recordId, raw)], 'edit record')
            }
          >
            Edit record
          </button>
        </ScopeGate>
        <ScopeGate allowed={writeAllowed} missingScope="dns.write">
          <button
            type="button"
            disabled={!hopReady || recordId === '' || zoneId === ''}
            onClick={() =>
              goToChanges(navigate, [deleteRecordOperation(zoneId, recordId)], 'delete record')
            }
          >
            Delete record
          </button>
        </ScopeGate>
      </p>
    </article>
  )
}
