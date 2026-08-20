import { ScopeGate } from '../../components/ScopeGate'
import { MutationsPending } from './ui'
import { activateMissingScope, SCOPE_CHAOS_ACTIVATE, type SessionActorView } from './view'

// PR-14 owns activate/deactivate/expiry. Keep the controls visible but inert.
export function ActivationPanel({
  actor,
  safetyClass,
}: {
  actor: SessionActorView
  safetyClass: string
}) {
  const missing = activateMissingScope(actor, safetyClass)
  const allowed = missing === ''
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
          <span>
            <button type="button" disabled>
              Activate
            </button>{' '}
            <button type="button" disabled>
              Deactivate
            </button>{' '}
            <button type="button" disabled>
              Set expiry
            </button>
          </span>
        </ScopeGate>
      </MutationsPending>
    </section>
  )
}
