import { ScopeGate } from '../../components/ScopeGate'
import { MutationsPending } from './ui'
import {
  activateMissingScope,
  SCOPE_CHAOS_ACTIVATE,
  scopeGateAllowed,
  type SessionActorView,
} from './view'

// Read slice: mutations stay inert. PR-14 drops `true ||` so pending (`disabled`) plus ScopeGate remain the live gate.
function ActivationControls({ disabled }: { disabled?: boolean }) {
  const off = true || disabled === true
  return (
    <span>
      <button type="button" disabled={off}>
        Activate
      </button>{' '}
      <button type="button" disabled={off}>
        Deactivate
      </button>{' '}
      <button type="button" disabled={off}>
        Set expiry
      </button>
    </span>
  )
}

// PR-14 owns activate/deactivate/expiry. Keep the controls visible but inert.
export function ActivationPanel({
  actor,
  safetyClass,
  sessionKnown,
}: {
  actor: SessionActorView
  safetyClass: string
  sessionKnown: boolean
}) {
  const missing = activateMissingScope(actor, safetyClass)
  const allowed = scopeGateAllowed(sessionKnown, missing === '')
  const named = missing === '' ? SCOPE_CHAOS_ACTIVATE : missing
  return (
    <section>
      <h2>Activation</h2>
      <p>Same privilege split as REST. High-impact policies need dns.chaos.activate and dns.chaos.emergency.</p>
      <p>
        <label>
          Expiry
          <input name="expiresAt" type="datetime-local" disabled />
        </label>
      </p>
      <MutationsPending>
        <ScopeGate allowed={allowed} missingScope={named}>
          <ActivationControls disabled={!sessionKnown} />
        </ScopeGate>
      </MutationsPending>
    </section>
  )
}
