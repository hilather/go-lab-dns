import { useState, type FormEvent } from 'react'
import type { components } from '../../api/openapi'
import { client } from '../../api/client'
import { ScopeGate } from '../../components/ScopeGate'
import { QueryError } from './ui'
import {
  hasScope,
  parseSimulateOut,
  SCOPE_CHAOS_READ,
  scopeGateAllowed,
  yn,
  type SessionActorView,
  type SimulateOutView,
} from './view'

function asSimulateIn(body: {
  name: string
  type: string
  clientContext?: { clientGroup: string }
}): components['schemas']['SimulateIn'] {
  return body
}

export function SimulateForm({
  actor,
  sessionKnown,
}: {
  actor: SessionActorView
  sessionKnown: boolean
}) {
  const canRead = hasScope(actor, SCOPE_CHAOS_READ)
  const allowed = scopeGateAllowed(sessionKnown, canRead)
  const [name, setName] = useState('')
  const [rrtype, setRrtype] = useState('A')
  const [clientGroup, setClientGroup] = useState('')
  const [busy, setBusy] = useState(false)
  const [problem, setProblem] = useState<unknown>(null)
  const [result, setResult] = useState<SimulateOutView | null>(null)
  const fieldsOn = sessionKnown && canRead && !busy
  const canSubmit = fieldsOn && name.trim() !== ''

  async function onSubmit(ev: FormEvent) {
    ev.preventDefault()
    if (!canSubmit) {
      return
    }
    setBusy(true)
    setProblem(null)
    try {
      const group = clientGroup.trim()
      const body = asSimulateIn({
        name: name.trim(),
        type: rrtype.trim() || 'A',
        ...(group !== '' ? { clientContext: { clientGroup: group } } : {}),
      })
      const res = await client.POST('/v1/chaos:simulate', { body })
      if (!res.response.ok) {
        setResult(null)
        setProblem(res.error ?? { code: 'internal_error', detail: 'simulate failed' })
        return
      }
      setResult(parseSimulateOut(res.data))
    } catch (err) {
      setResult(null)
      setProblem(err)
    } finally {
      setBusy(false)
    }
  }

  return (
    <section>
      <h2>Simulate</h2>
      <p>Side-effect free. Does not change live chaos state.</p>
      {problem ? <QueryError error={problem} /> : null}
      <form onSubmit={(ev) => void onSubmit(ev)}>
        <p>
          <label>
            Name
            <input
              name="name"
              type="text"
              value={name}
              onChange={(ev) => setName(ev.target.value)}
              disabled={!fieldsOn}
              autoComplete="off"
            />
          </label>
        </p>
        <p>
          <label>
            Type
            <input
              name="type"
              type="text"
              value={rrtype}
              onChange={(ev) => setRrtype(ev.target.value)}
              disabled={!fieldsOn}
              autoComplete="off"
            />
          </label>
        </p>
        <p>
          <label>
            Client group
            <input
              name="clientGroup"
              type="text"
              value={clientGroup}
              onChange={(ev) => setClientGroup(ev.target.value)}
              disabled={!fieldsOn}
              autoComplete="off"
            />
          </label>
        </p>
        <p>
          <ScopeGate allowed={allowed} missingScope={SCOPE_CHAOS_READ}>
            <button type="submit" disabled={!canSubmit}>
              Simulate
            </button>
          </ScopeGate>
        </p>
      </form>
      {result ? (
        <div>
          <h3>Simulation</h3>
          <dl>
            <dt>Algorithm</dt>
            <dd>{result.algorithm || '—'}</dd>
            <dt>Triggered</dt>
            <dd>{yn(result.triggered, result.triggered !== undefined)}</dd>
            <dt>Disabled</dt>
            <dd>{yn(result.disabled, result.disabled !== undefined)}</dd>
            <dt>Reason</dt>
            <dd>{result.reason || '—'}</dd>
          </dl>
          {result.decisions.length === 0 ? (
            <p>No policy decisions.</p>
          ) : (
            <ul>
              {result.decisions.map((d) => (
                <li key={`${d.policyId}:${d.outcomeId}:${d.digestHex}`}>
                  {d.policyId}
                  {d.outcomeId !== '' ? ` outcome ${d.outcomeId}` : ''}
                  {d.triggered === true ? ' triggered' : ' skipped'}
                  {d.skipReason !== '' ? ` (${d.skipReason})` : ''}
                  {d.digestHex !== '' ? ` ${d.digestHex}` : ''}
                </li>
              ))}
            </ul>
          )}
        </div>
      ) : null}
    </section>
  )
}
