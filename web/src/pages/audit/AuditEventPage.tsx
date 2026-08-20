import { useQuery } from '@tanstack/react-query'
import { Link, useParams } from 'react-router'
import { queryKeys } from '../../query/keys'
import { ROUTES } from '../../routes'
import { QueryError } from './ui'
import { dash, fetchAuditEvent, parseAuditEvent } from './view'

export function AuditEventPage() {
  const { eventId = '' } = useParams()
  const query = useQuery({
    queryKey: [...queryKeys.audit(), eventId],
    queryFn: () => fetchAuditEvent(eventId),
    enabled: eventId !== '',
  })
  const ev = parseAuditEvent(query.data)

  return (
    <article className="dashboard">
      <p>
        <Link to={ROUTES.audit}>Audit</Link>
      </p>
      <h1>{ev?.id || eventId || 'Audit event'}</h1>
      {query.isError ? <QueryError error={query.error} /> : null}
      {query.isPending ? <p>Loading event…</p> : null}
      {ev ? (
        <section>
          <h2>Event</h2>
          <dl>
            <dt>ID</dt>
            <dd>{ev.id}</dd>
            <dt>Time</dt>
            <dd>{dash(ev.time)}</dd>
            <dt>Actor</dt>
            <dd>{dash(ev.actorId)}</dd>
            <dt>Actor class</dt>
            <dd>{dash(ev.actorClass)}</dd>
            <dt>Transport</dt>
            <dd>{dash(ev.transport)}</dd>
            <dt>Capability</dt>
            <dd>{dash(ev.capability)}</dd>
            <dt>Result</dt>
            <dd>{dash(ev.result)}</dd>
            <dt>Error code</dt>
            <dd>{dash(ev.errorCode)}</dd>
            <dt>Reason</dt>
            <dd>{dash(ev.reason)}</dd>
            <dt>Ticket</dt>
            <dd>{dash(ev.ticket)}</dd>
            <dt>Revision</dt>
            <dd>{dash(ev.revision)}</dd>
            <dt>Previous</dt>
            <dd>{dash(ev.previous)}</dd>
          </dl>
        </section>
      ) : null}
    </article>
  )
}
