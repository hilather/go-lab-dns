import { useQuery } from '@tanstack/react-query'
import { Link, NavLink, useNavigate, useParams } from 'react-router'
import { ScopeGate } from '../../components/ScopeGate'
import { hasWriteScope } from '../changes/changeIn'
import {
  parseSessionActor,
  scopeGateAllowed,
  useSessionActorQuery,
} from '../chaos/view'
import { queryKeys } from '../../query/keys'
import { ROUTES } from '../../routes'
import { CursorPager, QueryError } from './ui'
import {
  DEFAULT_PAGE_LIMIT,
  createRecordOperation,
  createZoneOperation,
  deleteZoneOperation,
  editZoneOperation,
  fetchRecordList,
  fetchZoneBundle,
  fetchZoneList,
  formatSOA,
  goToChanges,
  recordHref,
  recordsListKey,
  usePagedCursor,
  useRuntimeRevision,
  zoneHref,
  zonesListKey,
} from './zones'

export function ZonesPage() {
  const { zoneId = '' } = useParams()
  const navigate = useNavigate()
  const revision = useRuntimeRevision()
  const [listCursor, setListCursor] = usePagedCursor(revision)
  const sessionQuery = useSessionActorQuery()
  const actor = parseSessionActor(sessionQuery.data)
  const sessionKnown = sessionQuery.isSuccess || sessionQuery.isError
  const canWrite = hasWriteScope(actor)
  const writeAllowed = scopeGateAllowed(sessionKnown, canWrite)

  const listQuery = useQuery({
    queryKey: zonesListKey(revision, listCursor, DEFAULT_PAGE_LIMIT),
    queryFn: () => fetchZoneList(listCursor, DEFAULT_PAGE_LIMIT),
  })
  const page = listQuery.data
  const selectedId = zoneId !== '' ? zoneId : (page?.zones[0]?.id ?? '')
  const [recordsCursor, setRecordsCursor] = usePagedCursor(`${revision}\0${selectedId}`)

  const zoneQuery = useQuery({
    queryKey: queryKeys.zone(revision, selectedId),
    queryFn: () => fetchZoneBundle(selectedId),
    enabled: selectedId !== '',
  })
  const recordsQuery = useQuery({
    queryKey: recordsListKey(revision, selectedId, recordsCursor, DEFAULT_PAGE_LIMIT),
    queryFn: () => fetchRecordList(selectedId, recordsCursor, DEFAULT_PAGE_LIMIT),
    enabled: selectedId !== '' && zoneQuery.isSuccess,
  })

  const zone = zoneQuery.data?.view
  const zoneRaw = zoneQuery.data?.raw
  const records = recordsQuery.data
  const rawReady = zoneRaw !== undefined && zoneRaw !== null && typeof zoneRaw === 'object'
  const hopReady = sessionKnown && canWrite
  const editReady = hopReady && rawReady

  return (
    <article className="zones">
      <div className="zones-head">
        <div>
          <h1>Zones</h1>
          <p className="zones-lede">
            Authoritative and overlay snapshots. Writes enqueue operations on Changes.
          </p>
        </div>
        <ScopeGate allowed={writeAllowed} missingScope="dns.write">
          <button
            type="button"
            className="btn-accent"
            disabled={!hopReady}
            onClick={() => goToChanges(navigate, [createZoneOperation()], 'create zone')}
          >
            Create zone
          </button>
        </ScopeGate>
      </div>
      {listQuery.isError ? <QueryError error={listQuery.error} /> : null}
      {listQuery.isPending ? <p>Loading zones…</p> : null}
      {page && page.zones.length === 0 && !listQuery.isPending ? <p>No zones.</p> : null}
      {(page && page.zones.length > 0) || listCursor !== '' ? (
        <div className="zones-layout">
          <aside className="zones-inventory" aria-label="Inventory">
            <p className="inventory-label">Inventory</p>
            {(page?.zones ?? []).map((z) => {
              const count = z.records.length
              const selected = z.id === selectedId
              return (
                <NavLink
                  key={z.id}
                  to={zoneHref(z.id)}
                  className={() => `inventory-item${selected ? ' selected' : ''}`}
                >
                  <span className="inventory-fqdn">{z.name || z.id}</span>
                  <span className="inventory-meta">
                    {' '}
                    {z.id} · {count} {count === 1 ? 'record' : 'records'}
                  </span>{' '}
                  <span className={`mode-pill mode-${z.mode || 'unknown'}`}>{z.mode || '—'}</span>
                </NavLink>
              )
            })}
            <CursorPager
              cursor={listCursor}
              nextCursor={page?.nextCursor ?? ''}
              onFirst={() => setListCursor('')}
              onNext={setListCursor}
              firstLabel="First zones"
              nextLabel="Next zones"
            />
          </aside>
          <section className="zone-panel">
            {zoneQuery.isError ? <QueryError error={zoneQuery.error} /> : null}
            {selectedId !== '' && zoneQuery.isPending ? <p>Loading zone…</p> : null}
            {zone ? (
              <>
                <p className="zone-crumb">
                  <Link to={ROUTES.zones}>Zones</Link>
                  {' / '}
                  <span>{zone.id}</span>
                </p>
                <h2 className="zone-title">{zone.name || zone.id}</h2>
                <dl className="zone-meta">
                  <dt>ID</dt>
                  <dd>{zone.id}</dd>
                  <dt>Mode</dt>
                  <dd>{zone.mode || '—'}</dd>
                  <dt>Nameservers</dt>
                  <dd>{zone.nameservers.length > 0 ? zone.nameservers.join(', ') : '—'}</dd>
                  <dt>SOA</dt>
                  <dd>{formatSOA(zone.soa)}</dd>
                </dl>
                <div className="zone-actions">
                  <ScopeGate allowed={writeAllowed} missingScope="dns.write">
                    <button
                      type="button"
                      disabled={!editReady}
                      onClick={() =>
                        goToChanges(navigate, [editZoneOperation(zone.id, zoneRaw)], 'edit zone')
                      }
                    >
                      Edit zone
                    </button>
                  </ScopeGate>
                  <ScopeGate allowed={writeAllowed} missingScope="dns.write">
                    <button
                      type="button"
                      disabled={!hopReady}
                      onClick={() =>
                        goToChanges(navigate, [deleteZoneOperation(zone.id)], 'delete zone')
                      }
                    >
                      Delete zone
                    </button>
                  </ScopeGate>
                  <ScopeGate allowed={writeAllowed} missingScope="dns.write">
                    <button
                      type="button"
                      className="btn-accent"
                      disabled={!hopReady}
                      onClick={() =>
                        goToChanges(navigate, [createRecordOperation(zone.id)], 'create record')
                      }
                    >
                      Create record
                    </button>
                  </ScopeGate>
                </div>
              </>
            ) : null}
            {recordsQuery.isError ? <QueryError error={recordsQuery.error} /> : null}
            {selectedId !== '' && recordsQuery.isLoading ? <p>Loading records…</p> : null}
            {records && records.records.length === 0 && !recordsQuery.isPending ? (
              <p>No records.</p>
            ) : null}
            {records && records.records.length > 0 ? (
              <table className="zones-table">
                <caption>Records in this zone</caption>
                <thead>
                  <tr>
                    <th scope="col">Owner</th>
                    <th scope="col">Type</th>
                    <th scope="col">TTL</th>
                    <th scope="col">Values</th>
                    <th scope="col">Chaos</th>
                    <th scope="col">ID</th>
                  </tr>
                </thead>
                <tbody>
                  {records.records.map((r) => (
                    <tr key={r.id}>
                      <td>{r.owner || '—'}</td>
                      <td>{r.type || '—'}</td>
                      <td>{r.ttl || '—'}</td>
                      <td>{r.values.length > 0 ? r.values.join(', ') : '—'}</td>
                      <td>
                        {r.chaosPolicyRefs.length > 0
                          ? r.chaosPolicyRefs.map((ref) => (
                              <span key={ref} className="chaos-ref">
                                {ref}
                              </span>
                            ))
                          : '—'}
                      </td>
                      <td>
                        <Link to={recordHref(selectedId, r.id)}>{r.id}</Link>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            ) : null}
            <CursorPager
              cursor={recordsCursor}
              nextCursor={records?.nextCursor ?? ''}
              onFirst={() => setRecordsCursor('')}
              onNext={setRecordsCursor}
              firstLabel="First records"
              nextLabel="Next records"
            />
          </section>
        </div>
      ) : null}
    </article>
  )
}
