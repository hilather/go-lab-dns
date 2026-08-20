import { useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import type { components } from '../../api/openapi'
import { client } from '../../api/client'
import { ConfirmDialog } from '../../components/ConfirmDialog'
import { ScopeGate } from '../../components/ScopeGate'
import { invalidateSnapshotQueries, queryKeys } from '../../query/keys'
import { QueryError } from './ui'
import {
  EMERGENCY_REASON,
  emergencyDisableMissingScope,
  emergencyEnableMissingScope,
  parseSessionActor,
  SCOPE_CHAOS_EMERGENCY,
  scopeGateAllowed,
  useSessionActorQuery,
} from './view'

type Pending = 'disable' | 'enable' | null

function asEmergencyIn(body: { reason: string }): components['schemas']['EmergencyIn'] {
  return body
}

export function EmergencyControl({ emergencyDisabled }: { emergencyDisabled?: boolean }) {
  const queryClient = useQueryClient()
  const sessionQuery = useSessionActorQuery()
  const actor = parseSessionActor(sessionQuery.data)
  const sessionKnown = sessionQuery.isSuccess || sessionQuery.isError
  const disableMissing = emergencyDisableMissingScope(actor)
  const enableMissing = emergencyEnableMissingScope(actor)
  const disableAllowed = scopeGateAllowed(sessionKnown, disableMissing === '')
  const enableAllowed = scopeGateAllowed(sessionKnown, enableMissing === '')
  const namedDisable = disableMissing === '' ? SCOPE_CHAOS_EMERGENCY : disableMissing
  const namedEnable = enableMissing === '' ? SCOPE_CHAOS_EMERGENCY : enableMissing

  const [confirm, setConfirm] = useState<Pending>(null)
  const [busy, setBusy] = useState(false)
  const [problem, setProblem] = useState<unknown>(null)
  const [localOff, setLocalOff] = useState<boolean | null>(null)
  const off = localOff ?? emergencyDisabled
  const canDisable = sessionKnown && disableMissing === '' && !busy
  const canEnable = sessionKnown && enableMissing === '' && !busy

  async function run(kind: Exclude<Pending, null>) {
    setBusy(true)
    setProblem(null)
    try {
      const path = kind === 'disable' ? '/v1/chaos:emergency-disable' : '/v1/chaos:emergency-enable'
      const res = await client.POST(path, { body: asEmergencyIn({ reason: EMERGENCY_REASON }) })
      if (!res.response.ok) {
        setProblem(res.error ?? { code: 'internal_error', detail: `emergency ${kind} failed` })
        return
      }
      setLocalOff(kind === 'disable')
      setConfirm(null)
      await Promise.all([
        invalidateSnapshotQueries(queryClient),
        queryClient.invalidateQueries({ queryKey: queryKeys.liveChaos() }),
        queryClient.invalidateQueries({ queryKey: queryKeys.status() }),
      ])
    } catch (err) {
      setProblem(err)
    } finally {
      setBusy(false)
    }
  }

  let stateLabel = 'Chaos emergency unknown'
  if (off === true) {
    stateLabel = 'Chaos emergency disabled'
  } else if (off === false) {
    stateLabel = 'Chaos engine live'
  }

  return (
    <div className="emergency-control">
      <p className={`status ${off === true ? 'status-emergency' : ''}`}>{stateLabel}</p>
      {problem ? <QueryError error={problem} /> : null}
      <ScopeGate allowed={disableAllowed} missingScope={namedDisable}>
        <button
          type="button"
          disabled={!canDisable}
          onClick={() => {
            setProblem(null)
            setConfirm('disable')
          }}
        >
          Emergency disable
        </button>
      </ScopeGate>
      <ScopeGate allowed={enableAllowed} missingScope={namedEnable}>
        <button
          type="button"
          disabled={!canEnable}
          onClick={() => {
            setProblem(null)
            setConfirm('enable')
          }}
        >
          Emergency enable
        </button>
      </ScopeGate>
      <ConfirmDialog
        open={confirm !== null}
        title={confirm === 'enable' ? 'Re-enable chaos?' : 'Disable chaos?'}
        confirmLabel={confirm === 'enable' ? 'Enable' : 'Disable'}
        confirmDisabled={busy}
        onConfirm={() => {
          if (confirm) {
            void run(confirm)
          }
        }}
        onCancel={() => {
          if (!busy) {
            setConfirm(null)
          }
        }}
      >
        <p>
          {confirm === 'enable'
            ? 'This clears the runtime emergency inhibit. YAML startup locks still win.'
            : 'This stops new faults immediately. One confirm; no typed phrase.'}
        </p>
      </ConfirmDialog>
    </div>
  )
}
