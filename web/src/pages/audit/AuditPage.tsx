import { useQuery } from '@tanstack/react-query'
import { useMemo, useState } from 'react'
import { Link } from 'react-router'
import { queryKeys } from '../../query/keys'
import { QueryError } from './ui'
import {
  AUDIT_RING_MAX,
  DEFAULT_AUDIT_LIMIT,
  SCOPE_AUDIT_READ,
  clampAuditLimit,
  dash,
  eventHref,
  eventMatches,
  fetchAuditList,
  parseAuditList,
  type AuditFilters,
} from './view'

export function AuditPage() {
  const [limitInput, setLimitInput] = useState(String(DEFAULT_AUDIT_LIMIT))
  const limit = clampAuditLimit(limitInput)
  const [filters, setFilters] = useState<AuditFilters>({ capability: '', result: '', actorId: '' })
  const query = useQuery({
    queryKey: [...queryKeys.audit(), limit],
    queryFn: () => fetchAuditList(limit),
  })
  const events = parseAuditList(query.data).events
  const visible = useMemo(() => events.filter((ev) => eventMatches(ev, filters)), [events, filters])

  return (
    <article className="dashboard">
      <h1>Audit</h1>
      <p>
        In-memory ring of {AUDIT_RING_MAX} events; oldest fall off the front. LabDNS does not persist a durable
        audit log. Secret-looking values stay redacted as the API returns them. Requires {SCOPE_AUDIT_READ}.
      </p>
      {query.isError ? <QueryError error={query.error} /> : null}

      <section>
        <h2>Filters</h2>
        <p>
          <label>
            Limit
            <input
              name="limit"
              type="number"
              min={1}
              max={100}
              value={limitInput}
              onChange={(ev) => setLimitInput(ev.target.value)}
            />
          </label>
        </p>
        <p>
          <label>
            Capability
            <input
              name="capability"
              type="search"
              value={filters.capability}
              onChange={(ev) => setFilters((f) => ({ ...f, capability: ev.target.value }))}
              autoComplete="off"
            />
          </label>
        </p>
        <p>
          <label>
            Result
            <select
              name="result"
              value={filters.result}
              onChange={(ev) => setFilters((f) => ({ ...f, result: ev.target.value }))}
            >
              <option value="">All</option>
              <option value="ok">ok</option>
              <option value="denied">denied</option>
              <option value="error">error</option>
            </select>
          </label>
        </p>
        <p>
          <label>
            Actor
            <input
              name="actorId"
              type="search"
              value={filters.actorId}
              onChange={(ev) => setFilters((f) => ({ ...f, actorId: ev.target.value }))}
              autoComplete="off"
            />
          </label>
        </p>
      </section>

      <section>
        <h2>Events</h2>
        {query.isPending ? <p>Loading audit…</p> : null}
        {!query.isPending && !query.isError && visible.length === 0 ? <p>No audit events.</p> : null}
        {visible.length > 0 ? (
          <table>
            <caption>Newest-first in-memory audit ring</caption>
            <thead>
              <tr>
                <th scope="col">ID</th>
                <th scope="col">Time</th>
                <th scope="col">Actor</th>
                <th scope="col">Class</th>
                <th scope="col">Transport</th>
                <th scope="col">Capability</th>
                <th scope="col">Result</th>
                <th scope="col">Reason</th>
              </tr>
            </thead>
            <tbody>
              {visible.map((ev) => (
                <tr key={ev.id}>
                  <td>
                    <Link to={eventHref(ev.id)}>{ev.id}</Link>
                  </td>
                  <td>{dash(ev.time)}</td>
                  <td>{dash(ev.actorId)}</td>
                  <td>{dash(ev.actorClass)}</td>
                  <td>{dash(ev.transport)}</td>
                  <td>{dash(ev.capability)}</td>
                  <td>{dash(ev.result)}</td>
                  <td>{dash(ev.reason)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        ) : null}
      </section>
    </article>
  )
}
