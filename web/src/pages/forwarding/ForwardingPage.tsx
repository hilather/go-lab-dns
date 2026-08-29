import { useQuery } from '@tanstack/react-query'
import { useOutletContext } from 'react-router'
import { client, throwOnError } from '../../api/client'
import { APIError } from '../../auth/sessionApi'
import type { ShellContext } from '../../components/Shell'
import { ProblemAlert } from '../../components/ProblemAlert'
import { queryKeys } from '../../query/keys'
import { useUpstreamsStatusQuery } from '../../query/live'
import {
  formatFailover,
  healthKind,
  healthLabel,
  healthSymbol,
  parsePolicies,
  parsePools,
  parseUpstreamsStatus,
} from './view'

export async function fetchForwardingPolicies(): Promise<unknown> {
  return throwOnError(await client.GET('/v1/forwarding/policies'))
}

export async function fetchUpstreamPools(): Promise<unknown> {
  return throwOnError(await client.GET('/v1/upstream-pools'))
}

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

function QueryError({ err }: { err: unknown }) {
  return <ProblemAlert error={problemOf(err)} />
}

export function ForwardingPage() {
  const { status } = useOutletContext<ShellContext>()
  const revision = status?.revisions?.runtimeRevision ?? ''
  const policiesQuery = useQuery({
    queryKey: queryKeys.forwarding(revision),
    queryFn: fetchForwardingPolicies,
    enabled: revision !== '',
  })
  const poolsQuery = useQuery({
    queryKey: queryKeys.pools(revision),
    queryFn: fetchUpstreamPools,
    enabled: revision !== '',
  })
  const upstreamsQuery = useUpstreamsStatusQuery()

  const policies = parsePolicies(policiesQuery.data)
  const pools = parsePools(poolsQuery.data)
  const upstreams = parseUpstreamsStatus(upstreamsQuery.data)

  return (
    <article className="dashboard">
      <div className="page-head">
        <div>
          <h1>Forwarding</h1>
          <p className="page-lede">Policies, upstream pools, and live health. Health polls independently of snapshot revision.</p>
        </div>
      </div>

      <section className="surface">
        <h2>Policies</h2>
        {revision === '' ? (
          <p>—</p>
        ) : policiesQuery.isPending ? (
          <p>Loading…</p>
        ) : policiesQuery.error ? (
          <QueryError err={policiesQuery.error} />
        ) : policies.length === 0 ? (
          <p className="empty">No forwarding policies.</p>
        ) : (
          <table className="data-table">
            <caption>Compiled forwarding policies</caption>
            <thead>
              <tr>
                <th scope="col">ID</th>
                <th scope="col">Suffix</th>
                <th scope="col">Upstream pool</th>
                <th scope="col">Failover</th>
              </tr>
            </thead>
            <tbody>
              {policies.map((p) => (
                <tr key={p.id}>
                  <td>{p.id}</td>
                  <td>{p.suffix || '—'}</td>
                  <td>{p.upstreamPool || '—'}</td>
                  <td>{formatFailover(p.failover)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </section>

      <section className="surface">
        <h2>Pools</h2>
        {revision === '' ? (
          <p>—</p>
        ) : poolsQuery.isPending ? (
          <p>Loading…</p>
        ) : poolsQuery.error ? (
          <QueryError err={poolsQuery.error} />
        ) : pools.length === 0 ? (
          <p className="empty">No upstream pools.</p>
        ) : (
          <table className="data-table">
            <caption>Configured upstream pools</caption>
            <thead>
              <tr>
                <th scope="col">ID</th>
                <th scope="col">Strategy</th>
                <th scope="col">Upstreams</th>
              </tr>
            </thead>
            <tbody>
              {pools.map((p) => (
                <tr key={p.id}>
                  <td>{p.id}</td>
                  <td>{p.strategy || '—'}</td>
                  <td>
                    {p.upstreams.length === 0
                      ? '—'
                      : p.upstreams
                          .map((u) => `${u.id} (${u.endpoint || '—'}, ${u.transport || '—'})`)
                          .join(', ')}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </section>

      <section className="surface">
        <h2>Upstream health</h2>
        {upstreamsQuery.isPending && !upstreamsQuery.data ? (
          <p>Loading…</p>
        ) : upstreamsQuery.error ? (
          <QueryError err={upstreamsQuery.error} />
        ) : upstreams.length === 0 ? (
          <p className="empty">No upstreams reported.</p>
        ) : (
          <table className="data-table">
            <caption>Live upstream status (5s poll, independent of snapshot revision)</caption>
            <thead>
              <tr>
                <th scope="col">ID</th>
                <th scope="col">Pool</th>
                <th scope="col">Endpoint</th>
                <th scope="col">Transport</th>
                <th scope="col">Health</th>
              </tr>
            </thead>
            <tbody>
              {upstreams.map((u) => {
                const kind = healthKind(u.healthy)
                return (
                  <tr key={`${u.poolId}:${u.id}`}>
                    <td>{u.id}</td>
                    <td>{u.poolId || '—'}</td>
                    <td>{u.endpoint || '—'}</td>
                    <td>{u.transport || '—'}</td>
                    <td className={`status status-${kind}`}>
                      <span className="status-symbol" aria-hidden="true">
                        {healthSymbol(kind)}
                      </span>{' '}
                      {healthLabel(kind)}
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        )}
      </section>
    </article>
  )
}
