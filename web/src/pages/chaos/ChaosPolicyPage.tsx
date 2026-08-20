import { useQuery } from '@tanstack/react-query'
import { Fragment } from 'react'
import { Link, useParams } from 'react-router'
import { queryKeys } from '../../query/keys'
import { ROUTES } from '../../routes'
import { ActivationPanel } from './ActivationPanel'
import { QueryError } from './ui'
import {
  activationLabel,
  activationSymbol,
  dash,
  fetchChaosPolicy,
  formatAction,
  joinList,
  parseChaosPolicy,
  parseSessionActor,
  useRuntimeRevision,
  useSessionActorQuery,
  yn,
  type ChaosScopeView,
} from './view'

function scopeRows(scope: ChaosScopeView): { label: string; value: string }[] {
  return [
    { label: 'Record IDs', value: joinList(scope.recordIds) },
    { label: 'Owners', value: joinList(scope.owners) },
    { label: 'Wildcard sources', value: joinList(scope.wildcardSourceIds) },
    { label: 'Zones', value: joinList(scope.zones) },
    { label: 'Forwarding policies', value: joinList(scope.forwardingPolicyIds) },
    { label: 'Upstream pools', value: joinList(scope.upstreamPools) },
    { label: 'Client groups', value: joinList(scope.clientGroups) },
    { label: 'Qtypes', value: joinList(scope.qtypes) },
    { label: 'Transports', value: joinList(scope.transports) },
  ]
}

export function ChaosPolicyPage() {
  const { policyId = '' } = useParams()
  const revision = useRuntimeRevision()
  const sessionQuery = useSessionActorQuery()
  const actor = parseSessionActor(sessionQuery.data)
  const query = useQuery({
    queryKey: queryKeys.chaosPolicy(revision, policyId),
    queryFn: () => fetchChaosPolicy(policyId),
    enabled: policyId !== '' && revision !== '',
  })
  const policy = parseChaosPolicy(query.data)

  return (
    <article className="dashboard">
      <p>
        <Link to={ROUTES.chaos}>Chaos</Link>
      </p>
      <h1>{policy?.id || policyId || 'Chaos policy'}</h1>
      {query.isError ? <QueryError error={query.error} /> : null}
      {revision === '' || policyId === '' ? <p>—</p> : query.isPending ? <p>Loading policy…</p> : null}
      {policy ? (
        <>
          <section>
            <h2>Policy</h2>
            <dl>
              <dt>ID</dt>
              <dd>{policy.id}</dd>
              <dt>Activation</dt>
              <dd className="status">
                <span className="status-symbol" aria-hidden="true">
                  {activationSymbol(policy.enabled)}
                </span>{' '}
                {activationLabel(policy.enabled)}
              </dd>
              <dt>Enabled</dt>
              <dd>{yn(policy.enabled, true)}</dd>
              <dt>Safety class</dt>
              <dd>{dash(policy.safetyClass)}</dd>
              <dt>Owner</dt>
              <dd>{dash(policy.owner)}</dd>
              <dt>Reason</dt>
              <dd>{dash(policy.reason)}</dd>
              <dt>Ticket</dt>
              <dd>{dash(policy.ticket)}</dd>
              <dt>Description</dt>
              <dd>{dash(policy.description)}</dd>
              <dt>Starts at</dt>
              <dd>{dash(policy.startsAt)}</dd>
              <dt>Expires at</dt>
              <dd>{dash(policy.expiresAt)}</dd>
              <dt>Composition</dt>
              <dd>{dash(policy.composition)}</dd>
              <dt>Exclusive group</dt>
              <dd>{dash(policy.exclusiveGroup)}</dd>
              <dt>Labels</dt>
              <dd>
                {policy.labels.length === 0
                  ? '—'
                  : policy.labels.map((l) => `${l.key}=${l.value}`).join(', ')}
              </dd>
            </dl>
          </section>
          <section>
            <h2>Scope</h2>
            <dl>
              {scopeRows(policy.scope).map((row) => (
                <Fragment key={row.label}>
                  <dt>{row.label}</dt>
                  <dd>{row.value}</dd>
                </Fragment>
              ))}
            </dl>
          </section>
          <section>
            <h2>Selector</h2>
            <dl>
              <dt>Mode</dt>
              <dd>{dash(policy.selector.mode)}</dd>
              <dt>Seed</dt>
              <dd>{dash(policy.selector.seed)}</dd>
              <dt>Probability</dt>
              <dd>{policy.selector.probability === undefined ? '—' : String(policy.selector.probability)}</dd>
              <dt>Time bucket</dt>
              <dd>{dash(policy.selector.timeBucket)}</dd>
              <dt>Every nth</dt>
              <dd>{policy.selector.everyNth === undefined ? '—' : String(policy.selector.everyNth)}</dd>
              <dt>Sampling key</dt>
              <dd>{dash(policy.selector.samplingKey)}</dd>
              <dt>Period</dt>
              <dd>{dash(policy.selector.period)}</dd>
            </dl>
          </section>
          <section>
            <h2>Outcomes</h2>
            {policy.outcomes.length === 0 ? (
              <p>No outcomes.</p>
            ) : (
              <ul>
                {policy.outcomes.map((o) => (
                  <li key={o.id}>
                    {o.id}
                    {o.weight === undefined ? '' : ` (weight ${o.weight})`}
                    {o.actions.length === 0 ? '' : `: ${o.actions.map(formatAction).join('; ')}`}
                  </li>
                ))}
              </ul>
            )}
          </section>
          {policy.budget ? (
            <section>
              <h2>Budget</h2>
              <dl>
                <dt>Max delay</dt>
                <dd>{dash(policy.budget.maxDelay)}</dd>
                <dt>Max concurrency</dt>
                <dd>{policy.budget.maxConcurrency === undefined ? '—' : String(policy.budget.maxConcurrency)}</dd>
                <dt>Max rate</dt>
                <dd>{policy.budget.maxRate === undefined ? '—' : String(policy.budget.maxRate)}</dd>
                <dt>Max frequency</dt>
                <dd>{policy.budget.maxFrequency === undefined ? '—' : String(policy.budget.maxFrequency)}</dd>
              </dl>
            </section>
          ) : null}
          <ActivationPanel
            actor={actor}
            safetyClass={policy.safetyClass}
            sessionKnown={sessionQuery.isSuccess || sessionQuery.isError}
            policyId={policy.id}
            expectedRevision={revision}
            enabled={policy.enabled}
          />
        </>
      ) : null}
    </article>
  )
}
