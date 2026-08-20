import { useQueryClient } from '@tanstack/react-query'
import { useRef, useState } from 'react'
import type { components } from '../../api/openapi'
import { client } from '../../api/client'
import { ScopeGate } from '../../components/ScopeGate'
import { invalidateSnapshotQueries, queryKeys } from '../../query/keys'
import { QueryError } from './ui'
import {
  activateMissingScope,
  datetimeLocalToRFC3339,
  deactivateMissingScope,
  expireMissingScope,
  newChaosIdempotencyKey,
  parseApplyResult,
  SCOPE_CHAOS_ACTIVATE,
  scopeGateAllowed,
  type ApplyResultView,
  type SessionActorView,
} from './view'

type MutationKind = 'activate' | 'deactivate' | 'expire'

function asActivationIn(body: Record<string, string>): components['schemas']['ActivationIn'] {
  return body
}

function asExpiryIn(body: Record<string, string>): components['schemas']['ExpiryIn'] {
  return body
}

export function ActivationPanel({
  actor,
  safetyClass,
  sessionKnown,
  policyId,
  expectedRevision,
  enabled,
}: {
  actor: SessionActorView
  safetyClass: string
  sessionKnown: boolean
  policyId: string
  expectedRevision: string
  enabled?: boolean
}) {
  const queryClient = useQueryClient()
  const activateMissing = activateMissingScope(actor, safetyClass)
  const deactivateMissing = deactivateMissingScope(actor)
  const expireMissing = expireMissingScope(actor, safetyClass, enabled)
  const activateAllowed = scopeGateAllowed(sessionKnown, activateMissing === '')
  const deactivateAllowed = scopeGateAllowed(sessionKnown, deactivateMissing === '')
  const expireAllowed = scopeGateAllowed(sessionKnown, expireMissing === '')
  const namedActivate = activateMissing === '' ? SCOPE_CHAOS_ACTIVATE : activateMissing
  const namedDeactivate = deactivateMissing === '' ? SCOPE_CHAOS_ACTIVATE : deactivateMissing
  const namedExpire = expireMissing === '' ? SCOPE_CHAOS_ACTIVATE : expireMissing

  const [reason, setReason] = useState('')
  const [expiresAt, setExpiresAt] = useState('')
  const [busy, setBusy] = useState<MutationKind | null>(null)
  const [problem, setProblem] = useState<unknown>(null)
  const [result, setResult] = useState<ApplyResultView | null>(null)
  const keyRef = useRef('')
  const lastKind = useRef<MutationKind | null>(null)

  const fieldsOn = sessionKnown && deactivateMissing === '' && busy === null
  const reasonOk = reason.trim() !== ''
  const revisionOk = expectedRevision !== '' && policyId !== ''
  const canActivate = fieldsOn && reasonOk && revisionOk && activateMissing === ''
  const canDeactivate = fieldsOn && reasonOk && revisionOk && deactivateMissing === ''
  const rfcExpiry = datetimeLocalToRFC3339(expiresAt)
  const canExpire = fieldsOn && reasonOk && revisionOk && rfcExpiry !== '' && expireMissing === ''

  function keyFor(kind: MutationKind): string {
    if (lastKind.current !== kind || keyRef.current === '') {
      keyRef.current = newChaosIdempotencyKey()
      lastKind.current = kind
    }
    return keyRef.current
  }

  async function run(kind: MutationKind) {
    const ready = kind === 'activate' ? canActivate : kind === 'deactivate' ? canDeactivate : canExpire
    if (!ready) {
      return
    }
    setBusy(kind)
    setProblem(null)
    const body: Record<string, string> = {
      expectedRevision,
      reason: reason.trim(),
      idempotencyKey: keyFor(kind),
    }
    if (kind === 'expire' || rfcExpiry !== '') {
      body.expiresAt = rfcExpiry
    }
    const params = { path: { id: policyId } }
    try {
      const res =
        kind === 'activate'
          ? await client.POST('/v1/chaos/policies/{id}:activate', { params, body: asActivationIn(body) })
          : kind === 'deactivate'
            ? await client.POST('/v1/chaos/policies/{id}:deactivate', { params, body: asActivationIn(body) })
            : await client.POST('/v1/chaos/policies/{id}:expire', { params, body: asExpiryIn(body) })
      if (!res.response.ok) {
        setResult(null)
        setProblem(res.error ?? { code: 'internal_error', detail: `${kind} failed` })
        return
      }
      keyRef.current = ''
      lastKind.current = null
      setResult(parseApplyResult(res.data))
      await Promise.all([
        invalidateSnapshotQueries(queryClient),
        queryClient.invalidateQueries({ queryKey: queryKeys.liveChaos() }),
        queryClient.invalidateQueries({ queryKey: queryKeys.status() }),
      ])
    } catch (err) {
      setResult(null)
      setProblem(err)
    } finally {
      setBusy(null)
    }
  }

  return (
    <section>
      <h2>Activation</h2>
      <p>Same privilege split as REST. High-impact policies need dns.chaos.activate and dns.chaos.emergency.</p>
      {problem ? <QueryError error={problem} /> : null}
      {result ? (
        <p>
          {result.applied === false ? 'Not applied.' : 'Applied.'} Revision {result.candidateRevision || '—'}
          {result.auditEventId !== '' ? `. Audit event ${result.auditEventId}` : ''}
        </p>
      ) : null}
      <p>
        <label>
          Reason
          <input
            name="reason"
            type="text"
            value={reason}
            onChange={(ev) => setReason(ev.target.value)}
            disabled={!sessionKnown || busy !== null}
            autoComplete="off"
          />
        </label>
      </p>
      <p>
        <label>
          Expiry
          <input
            name="expiresAt"
            type="datetime-local"
            value={expiresAt}
            onChange={(ev) => setExpiresAt(ev.target.value)}
            disabled={!sessionKnown || busy !== null}
          />
        </label>
      </p>
      <p>
        <ScopeGate allowed={activateAllowed} missingScope={namedActivate}>
          <button type="button" disabled={!canActivate} onClick={() => void run('activate')}>
            Activate
          </button>
        </ScopeGate>{' '}
        <ScopeGate allowed={deactivateAllowed} missingScope={namedDeactivate}>
          <button type="button" disabled={!canDeactivate} onClick={() => void run('deactivate')}>
            Deactivate
          </button>
        </ScopeGate>{' '}
        <ScopeGate allowed={expireAllowed} missingScope={namedExpire}>
          <button type="button" disabled={!canExpire} onClick={() => void run('expire')}>
            Set expiry
          </button>
        </ScopeGate>
      </p>
    </section>
  )
}
