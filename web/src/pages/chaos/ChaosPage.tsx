import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router'
import { queryKeys } from '../../query/keys'
import { useChaosStatusQuery } from '../../query/live'
import { SimulateForm } from './SimulateForm'
import { QueryError } from './ui'
import {
  activationLabel,
  activationSymbol,
  chaosRuntimeKind,
  chaosRuntimeLabel,
  chaosRuntimeSymbol,
  dash,
  fetchChaosPolicies,
  num,
  parseChaosPolicies,
  parseChaosStatus,
  parseSessionActor,
  policyHref,
  useRuntimeRevision,
  useSessionActorQuery,
  yn,
} from './view'

export function ChaosPage() {
  const revision = useRuntimeRevision()
  const sessionQuery = useSessionActorQuery()
  const actor = parseSessionActor(sessionQuery.data)
  const statusQuery = useChaosStatusQuery()
  const policiesQuery = useQuery({
    queryKey: queryKeys.chaosPolicies(revision),
    queryFn: fetchChaosPolicies,
    enabled: revision !== '',
  })
  const status = parseChaosStatus(statusQuery.data)
  const policies = parseChaosPolicies(policiesQuery.data)
  const known = status !== null
  const kind = chaosRuntimeKind(status)

  return (
    <article className="dashboard">
      <h1>Chaos</h1>

      <section>
        <h2>Status</h2>
        {statusQuery.isPending && !statusQuery.data ? (
          <p>Loading…</p>
        ) : statusQuery.error ? (
          <QueryError error={statusQuery.error} />
        ) : (
          <dl>
            <dt>Engine</dt>
            <dd className={`status status-${kind}`}>
              <span className="status-symbol" aria-hidden="true">
                {chaosRuntimeSymbol(kind)}
              </span>{' '}
              {chaosRuntimeLabel(kind)}
            </dd>
            <dt>Enabled</dt>
            <dd>{yn(status?.enabled, known)}</dd>
            <dt>Emergency disabled</dt>
            <dd>{yn(status?.emergencyDisabled, known)}</dd>
            <dt>Active policies</dt>
            <dd>{num(status?.activePolicies, known)}</dd>
            <dt>Nearest expiry</dt>
            <dd>{dash(status?.nearestExpiry ?? '')}</dd>
          </dl>
        )}
        <p>Emergency disable is on the shell for principals with dns.chaos.emergency.</p>
      </section>

      <section>
        <h2>Policies</h2>
        {revision === '' ? (
          <p>—</p>
        ) : policiesQuery.isPending ? (
          <p>Loading…</p>
        ) : policiesQuery.error ? (
          <QueryError error={policiesQuery.error} />
        ) : policies.length === 0 ? (
          <p>No chaos policies.</p>
        ) : (
          <table>
            <caption>Compiled chaos policies</caption>
            <thead>
              <tr>
                <th scope="col">ID</th>
                <th scope="col">Activation</th>
                <th scope="col">Safety class</th>
                <th scope="col">Expiry</th>
                <th scope="col">Owner</th>
              </tr>
            </thead>
            <tbody>
              {policies.map((p) => (
                <tr key={p.id}>
                  <td>
                    <Link to={policyHref(p.id)}>{p.id}</Link>
                  </td>
                  <td className="status">
                    <span className="status-symbol" aria-hidden="true">
                      {activationSymbol(p.enabled)}
                    </span>{' '}
                    {activationLabel(p.enabled)}
                  </td>
                  <td>{dash(p.safetyClass)}</td>
                  <td>{dash(p.expiresAt)}</td>
                  <td>{dash(p.owner)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </section>

      <SimulateForm actor={actor} sessionKnown={sessionQuery.isSuccess || sessionQuery.isError} />
    </article>
  )
}
