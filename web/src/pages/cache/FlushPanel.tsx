import { useQueryClient } from '@tanstack/react-query'
import { useEffect, useRef, useState } from 'react'
import { client } from '../../api/client'
import { APIError, getSession, type SessionActor } from '../../auth/sessionApi'
import { ConfirmDialog } from '../../components/ConfirmDialog'
import { ProblemAlert } from '../../components/ProblemAlert'
import { ScopeGate } from '../../components/ScopeGate'
import { queryKeys } from '../../query/keys'

function hasAdminScope(actor: SessionActor | null): boolean {
  if (!actor) {
    return false
  }
  const scopes = actor.scopes ?? []
  if (scopes.includes('dns.admin')) {
    return true
  }
  return actor.role === 'administrator'
}

function problemOf(err: unknown): { code?: string; detail?: string; message?: string } | null {
  if (!err) {
    return null
  }
  if (err instanceof APIError) {
    return { code: err.code, detail: err.detail, message: err.message }
  }
  if (err && typeof err === 'object') {
    const o = err as { code?: unknown; detail?: unknown; message?: unknown }
    return {
      code: typeof o.code === 'string' ? o.code : undefined,
      detail: typeof o.detail === 'string' ? o.detail : undefined,
      message: typeof o.message === 'string' ? o.message : undefined,
    }
  }
  if (err instanceof Error) {
    return { message: err.message }
  }
  return { message: 'request failed' }
}

export function FlushPanel() {
  const queryClient = useQueryClient()
  const [actor, setActor] = useState<SessionActor | null>(null)
  const [confirmOpen, setConfirmOpen] = useState(false)
  const [busy, setBusy] = useState(false)
  const [flushed, setFlushed] = useState(false)
  const [problem, setProblem] = useState<{ code?: string; detail?: string; message?: string } | null>(null)
  const inFlight = useRef(false)
  const canAdmin = hasAdminScope(actor)
  const canFlush = canAdmin && !busy && !inFlight.current

  useEffect(() => {
    let cancelled = false
    void getSession()
      .then((sess) => {
        if (!cancelled) {
          setActor(sess?.actor ?? null)
        }
      })
      .catch(() => {
        if (!cancelled) {
          setActor(null)
        }
      })
    return () => {
      cancelled = true
    }
  }, [])

  function openConfirm() {
    if (!canFlush) {
      return
    }
    setConfirmOpen(true)
  }

  async function runFlush() {
    if (inFlight.current || busy || !canAdmin) {
      return
    }
    inFlight.current = true
    setBusy(true)
    setProblem(null)
    try {
      const res = await client.POST('/v1/cache:flush', { body: { all: true } })
      if (!res.response.ok) {
        setProblem(problemOf(res.error) ?? { message: 'flush failed' })
        return
      }
      setFlushed(true)
      setConfirmOpen(false)
      await queryClient.invalidateQueries({ queryKey: queryKeys.liveCache() })
    } catch (err) {
      setProblem(problemOf(err))
    } finally {
      inFlight.current = false
      setBusy(false)
    }
  }

  return (
    <section>
      <h2>Flush</h2>
      <p>Requires dns.admin. Flush does not change desired state.</p>
      {problem ? <ProblemAlert error={problem} /> : null}
      {flushed ? <p>Cache flushed. Desired state is unchanged.</p> : null}
      <label>
        <input type="checkbox" checked disabled /> Flush all entries
      </label>
      <p>
        <ScopeGate allowed={canAdmin} missingScope="dns.admin">
          <button type="button" disabled={!canFlush} onClick={openConfirm}>
            Flush cache
          </button>
        </ScopeGate>
      </p>
      <ConfirmDialog
        open={confirmOpen}
        title="Flush the process cache?"
        confirmLabel="Flush"
        confirmDisabled={busy}
        onConfirm={() => void runFlush()}
        onCancel={() => setConfirmOpen(false)}
      >
        <p>This flushes all cache entries. Desired state is not modified.</p>
      </ConfirmDialog>
    </section>
  )
}
