import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect, useRef, useState } from 'react'
import { useOutletContext } from 'react-router'
import { client, throwOnError } from '../../api/client'
import { getSession, type SessionActor } from '../../auth/sessionApi'
import { ConfirmDialog } from '../../components/ConfirmDialog'
import { ProblemAlert } from '../../components/ProblemAlert'
import { ScopeGate } from '../../components/ScopeGate'
import type { ShellContext } from '../../components/Shell'
import { invalidateSnapshotQueries, queryKeys } from '../../query/keys'
import { shortRevision } from '../../status'
import { parseProblem } from '../changes/changeIn'
import {
  compiledMetadataName,
  confirmationMatches,
  hasAdminScope,
  parseResetResult,
  resetConfirmationPhrase,
  type ResetResultView,
} from './reset'
import './reset.css'

async function fetchState(): Promise<unknown> {
  return throwOnError(await client.GET('/v1/state'))
}

export function ResetPage() {
  const { status } = useOutletContext<ShellContext>()
  const queryClient = useQueryClient()
  const revision = status?.revisions?.runtimeRevision ?? ''
  const [actor, setActor] = useState<SessionActor | null>(null)
  const [typed, setTyped] = useState('')
  const [reason, setReason] = useState('')
  const [ticket, setTicket] = useState('')
  const [confirmOpen, setConfirmOpen] = useState(false)
  const [busy, setBusy] = useState(false)
  const [result, setResult] = useState<ResetResultView | null>(null)
  const [problem, setProblem] = useState<ReturnType<typeof parseProblem> | null>(null)
  const inFlight = useRef(false)

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

  const stateQuery = useQuery({
    queryKey: queryKeys.state(revision),
    queryFn: fetchState,
  })
  const name = compiledMetadataName(stateQuery.data)
  const expected = resetConfirmationPhrase(name)
  const matches = confirmationMatches(typed, expected)
  const canAdmin = hasAdminScope(actor)
  const canSubmit =
    canAdmin && matches && reason.trim() !== '' && !busy && !inFlight.current && stateQuery.data !== undefined

  function openConfirm() {
    if (!canSubmit) {
      return
    }
    setConfirmOpen(true)
  }

  async function runReset() {
    if (inFlight.current || busy || !matches || reason.trim() === '' || !canAdmin) {
      return
    }
    inFlight.current = true
    setBusy(true)
    setProblem(null)
    try {
      const body: Record<string, unknown> = { reason: reason.trim() }
      if (ticket.trim() !== '') {
        body.ticket = ticket.trim()
      }
      const res = await client.POST('/v1/state:reset', { body })
      if (!res.response.ok) {
        setProblem(parseProblem(res.error, res.response.status))
        return
      }
      setResult(parseResetResult(res.data))
      setConfirmOpen(false)
      setTyped('')
      await queryClient.invalidateQueries({ queryKey: queryKeys.status() })
      await invalidateSnapshotQueries(queryClient)
    } catch (err) {
      setProblem(parseProblem(err, 0))
    } finally {
      inFlight.current = false
      setBusy(false)
    }
  }

  return (
    <article className="reset-page">
      <h1>Reset</h1>
      <p className="revision" title={revision || undefined}>
        Runtime revision {shortRevision(revision)}
      </p>
      <p>Reread the bootstrap mount, compile, and swap. This never writes the bootstrap file.</p>
      {problem ? <ProblemAlert code={problem.code} detail={problem.detail} /> : null}
      {stateQuery.isPending && stateQuery.data === undefined ? <p>Loading compiled state…</p> : null}
      {stateQuery.error ? (
        <ProblemAlert
          error={
            stateQuery.error instanceof Error
              ? { message: stateQuery.error.message, ...(stateQuery.error as { code?: string; detail?: string }) }
              : { message: 'failed to load state' }
          }
        />
      ) : null}

      <section>
        <h2>Confirmation</h2>
        <dl>
          <dt>Compiled metadata name</dt>
          <dd>{name === '' ? '(empty)' : name}</dd>
          <dt>Type this phrase</dt>
          <dd>
            <code>{expected}</code>
          </dd>
        </dl>
        <label>
          Confirmation
          <input
            type="text"
            value={typed}
            onChange={(ev) => setTyped(ev.target.value)}
            autoComplete="off"
            aria-describedby="reset-confirm-hint"
          />
        </label>
        <p id="reset-confirm-hint">
          Type the compiled metadata name, or the literal RESET if the name is empty. Requires dns.admin.
        </p>
        <label>
          Reason (required)
          <input type="text" value={reason} onChange={(ev) => setReason(ev.target.value)} autoComplete="off" />
        </label>
        <label>
          Ticket (optional)
          <input type="text" value={ticket} onChange={(ev) => setTicket(ev.target.value)} autoComplete="off" />
        </label>
        <div className="reset-actions">
          <ScopeGate allowed={canAdmin} missingScope="dns.admin">
            <button type="button" disabled={!canSubmit} onClick={openConfirm}>
              Reset to bootstrap
            </button>
          </ScopeGate>
        </div>
      </section>

      {result ? (
        <section>
          <h2>Reset result</h2>
          <p>{result.applied === false ? 'Reset did not apply.' : 'Reset applied.'}</p>
          <dl>
            <dt>Previous revision</dt>
            <dd>{result.previousRevision || '—'}</dd>
            <dt>Candidate revision</dt>
            <dd>{result.candidateRevision || '—'}</dd>
            <dt>Generation</dt>
            <dd>{result.generation ?? '—'}</dd>
            <dt>Drifted</dt>
            <dd>{result.drifted === undefined ? '—' : result.drifted ? 'Yes' : 'No'}</dd>
            <dt>Audit event</dt>
            <dd>{result.auditEventId || '—'}</dd>
          </dl>
        </section>
      ) : null}

      <ConfirmDialog
        open={confirmOpen}
        title="Reset to bootstrap?"
        confirmLabel="Reset"
        confirmDisabled={busy || !matches || reason.trim() === ''}
        onConfirm={() => void runReset()}
        onCancel={() => setConfirmOpen(false)}
      >
        <p>Expected phrase: {expected}</p>
        <p>Runtime revision {revision || 'unknown'}.</p>
        <p>Reason: {reason.trim() || '(required)'}</p>
      </ConfirmDialog>
    </article>
  )
}
