import { ScopeGate } from '../../components/ScopeGate'
import { MutationsPending } from './ui'
import { hasScope, SCOPE_CHAOS_READ, scopeGateAllowed, type SessionActorView } from './view'

// PR-14 owns simulate mutations. Keep the form visible but inert.
export function SimulateForm({
  actor,
  sessionKnown,
}: {
  actor: SessionActorView
  sessionKnown: boolean
}) {
  const allowed = scopeGateAllowed(sessionKnown, hasScope(actor, SCOPE_CHAOS_READ))
  return (
    <section>
      <h2>Simulate</h2>
      <p>Side-effect free. Does not change live chaos state.</p>
      <form
        onSubmit={(ev) => {
          ev.preventDefault()
        }}
      >
        <p>
          <label>
            Name
            <input name="name" type="text" disabled autoComplete="off" />
          </label>
        </p>
        <p>
          <label>
            Type
            <input name="type" type="text" defaultValue="A" disabled autoComplete="off" />
          </label>
        </p>
        <p>
          <label>
            Client group
            <input name="clientGroup" type="text" disabled autoComplete="off" />
          </label>
        </p>
        <MutationsPending>
          <ScopeGate allowed={allowed} missingScope={SCOPE_CHAOS_READ}>
            <button type="submit" disabled>
              Simulate
            </button>
          </ScopeGate>
        </MutationsPending>
      </form>
    </section>
  )
}
