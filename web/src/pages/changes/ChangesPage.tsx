import { useQueryClient } from '@tanstack/react-query'
import { useEffect, useMemo, useRef, useState } from 'react'
import { useLocation, useOutletContext } from 'react-router'
import { client } from '../../api/client'
import { getSession, type SessionActor } from '../../auth/sessionApi'
import { ConfirmDialog } from '../../components/ConfirmDialog'
import { OperationBuilder } from '../../components/OperationBuilder'
import { ProblemAlert } from '../../components/ProblemAlert'
import { ScopeGate } from '../../components/ScopeGate'
import type { ShellContext } from '../../components/Shell'
import { YamlJsonEditor } from '../../components/YamlJsonEditor'
import { invalidateSnapshotQueries } from '../../query/keys'
import { shortRevision } from '../../status'
import {
  asChangeInSchema,
  asValidateInSchema,
  changeFingerprint,
  compileChangeIn,
  hasOperations,
  hasPlanSource,
  hasWriteScope,
  isPlanCurrent,
  newIdempotencyKey,
  parseEditorDocument,
  parsePlanView,
  parseProblem,
  type ChangeInBody,
  type Operation,
  type PlanView,
  type PlannedChange,
  type ProblemView,
} from './changeIn'
import { DocumentParseError } from './parseDocument'
import './changes.css'

export type ChangesLocationState = {
  operations?: Operation[]
  document?: string
  reason?: string
  ticket?: string
}

type EditorMode = 'document' | 'operations'

function jsonPreview(v: unknown): string {
  try {
    return JSON.stringify(v)
  } catch {
    return ''
  }
}

function asLocationState(state: unknown): ChangesLocationState {
  if (!state || typeof state !== 'object') {
    return {}
  }
  return state as ChangesLocationState
}

function seededOperations(state: unknown): Operation[] {
  const ops = asLocationState(state).operations
  return Array.isArray(ops) ? ops : []
}

export function ChangesPage() {
  const { status } = useOutletContext<ShellContext>()
  const location = useLocation()
  const queryClient = useQueryClient()
  const revision = status?.revisions?.runtimeRevision ?? ''

  const incoming = asLocationState(location.state)
  const initialOps = seededOperations(location.state)
  const [mode, setMode] = useState<EditorMode>(initialOps.length > 0 ? 'operations' : 'document')
  const [documentText, setDocumentText] = useState(
    typeof incoming.document === 'string' ? incoming.document : '',
  )
  const [operations, setOperations] = useState<Operation[]>(initialOps)
  const [reason, setReason] = useState(typeof incoming.reason === 'string' ? incoming.reason : '')
  const [ticket, setTicket] = useState(typeof incoming.ticket === 'string' ? incoming.ticket : '')
  const [actor, setActor] = useState<SessionActor | null>(null)
  const [problem, setProblem] = useState<ProblemView | null>(null)
  const [validateResult, setValidateResult] = useState<PlanView | null>(null)
  const [plan, setPlan] = useState<PlannedChange | null>(null)
  const [applyResult, setApplyResult] = useState<PlanView | null>(null)
  const [busy, setBusy] = useState<'validate' | 'plan' | 'apply' | null>(null)
  const [confirmOpen, setConfirmOpen] = useState(false)
  const idempotencyRef = useRef('')

  useEffect(() => {
    let cancelled = false
    void getSession().then((sess) => {
      if (!cancelled) {
        setActor(sess?.actor ?? null)
      }
    })
    return () => {
      cancelled = true
    }
  }, [])

  const compiled = useMemo((): { body: ChangeInBody; error: string } => {
    try {
      if (mode === 'operations') {
        return {
          body: compileChangeIn(undefined, { expectedRevision: revision, reason, ticket, operations }),
          error: '',
        }
      }
      if (documentText.trim() === '') {
        return { body: compileChangeIn(undefined, { expectedRevision: revision, reason, ticket }), error: '' }
      }
      const parsed = parseEditorDocument(documentText)
      return {
        body: compileChangeIn(parsed, { expectedRevision: revision, reason, ticket }),
        error: '',
      }
    } catch (err) {
      const message = err instanceof DocumentParseError || err instanceof Error ? err.message : 'invalid document'
      return { body: compileChangeIn(undefined, { expectedRevision: revision, reason, ticket }), error: message }
    }
  }, [mode, documentText, operations, revision, reason, ticket])

  const fingerprint = changeFingerprint(compiled.body)
  const parseError = compiled.error
  const planCurrent = isPlanCurrent(plan, revision, fingerprint)
  const canWrite = hasWriteScope(actor)
  const canValidate = canWrite && parseError === '' && hasPlanSource(compiled.body) && busy === null
  const canPlan = canWrite && parseError === '' && hasOperations(compiled.body) && revision !== '' && busy === null
  const canApply = canWrite && planCurrent && busy === null

  useEffect(() => {
    if (plan && plan.revision !== revision) {
      setPlan(null)
    }
  }, [plan, revision])

  function discardPlan() {
    setPlan(null)
  }

  function openConfirm() {
    if (!canApply) {
      return
    }
    idempotencyRef.current = newIdempotencyKey()
    setConfirmOpen(true)
  }

  function closeConfirm() {
    setConfirmOpen(false)
  }

  async function runValidate() {
    if (!canValidate) {
      return
    }
    setBusy('validate')
    setProblem(null)
    setValidateResult(null)
    try {
      const result = await client.POST('/v1/state:validate', { body: asValidateInSchema(compiled.body) })
      if (!result.response.ok) {
        setProblem(parseProblem(result.error, result.response.status))
        return
      }
      setValidateResult(parsePlanView(result.data))
    } catch (err) {
      setProblem(parseProblem(err, 0))
    } finally {
      setBusy(null)
    }
  }

  async function runPlan() {
    if (!canPlan) {
      return
    }
    setBusy('plan')
    setProblem(null)
    setApplyResult(null)
    try {
      const result = await client.POST('/v1/changes:plan', { body: asChangeInSchema(compiled.body) })
      if (!result.response.ok) {
        const p = parseProblem(result.error, result.response.status)
        setProblem(p)
        if (p.code === 'revision_conflict') {
          discardPlan()
        }
        return
      }
      setPlan({
        revision,
        fingerprint,
        body: parsePlanView(result.data),
      })
    } catch (err) {
      setProblem(parseProblem(err, 0))
    } finally {
      setBusy(null)
    }
  }

  async function runApply() {
    if (!planCurrent || reason.trim() === '' || busy !== null) {
      return
    }
    const key = idempotencyRef.current
    if (key === '') {
      return
    }
    setBusy('apply')
    setProblem(null)
    try {
      const body: ChangeInBody = {
        ...compiled.body,
        expectedRevision: plan?.revision ?? revision,
        idempotencyKey: key,
        reason: reason.trim(),
      }
      const result = await client.POST('/v1/changes:apply', { body: asChangeInSchema(body) })
      if (!result.response.ok) {
        const p = parseProblem(result.error, result.response.status)
        setProblem(p)
        if (p.code === 'revision_conflict') {
          discardPlan()
          setConfirmOpen(false)
        }
        return
      }
      setApplyResult(parsePlanView(result.data))
      discardPlan()
      setConfirmOpen(false)
      idempotencyRef.current = ''
      await invalidateSnapshotQueries(queryClient)
    } catch (err) {
      setProblem(parseProblem(err, 0))
    } finally {
      setBusy(null)
    }
  }

  return (
    <article className="changes-page">
      <h1>Changes</h1>
      <p className="revision" title={revision || undefined}>
        Runtime revision {shortRevision(revision)}
      </p>
      {problem ? (
        <div>
          <ProblemAlert code={problem.code} detail={problem.detail} />
          {problem.code === 'revision_conflict' ? (
            <p role="alert">
              Current revision {problem.currentRevision || revision}. The stale plan was discarded. Re-plan
              before apply. The candidate was not overwritten.
            </p>
          ) : null}
        </div>
      ) : null}

      <section>
        <h2>Candidate</h2>
        <div className="changes-mode" role="radiogroup" aria-label="Candidate source">
          <label>
            <input
              type="radio"
              name="changes-mode"
              checked={mode === 'document'}
              onChange={() => setMode('document')}
            />{' '}
            YAML/JSON
          </label>
          <label>
            <input
              type="radio"
              name="changes-mode"
              checked={mode === 'operations'}
              onChange={() => setMode('operations')}
            />{' '}
            Operations
          </label>
        </div>
        {mode === 'document' ? (
          <YamlJsonEditor value={documentText} onChange={setDocumentText} parseError={parseError} />
        ) : (
          <OperationBuilder operations={operations} onChange={setOperations} />
        )}
        <p>
          Validate accepts a candidate document and/or operations. Plan and apply require structured
          operations and the current runtime revision. Apply stays disabled until a plan for this
          revision exists.
        </p>
      </section>

      <section>
        <h2>Reason</h2>
        <label className="changes-reason">
          Reason (required to apply)
          <input
            type="text"
            value={reason}
            onChange={(ev) => setReason(ev.target.value)}
            autoComplete="off"
          />
        </label>
        <label className="changes-ticket">
          Ticket (optional)
          <input type="text" value={ticket} onChange={(ev) => setTicket(ev.target.value)} autoComplete="off" />
        </label>
        <div className="changes-actions">
          <ScopeGate allowed={canWrite} missingScope="dns.write">
            <button type="button" disabled={!canValidate} onClick={() => void runValidate()}>
              Validate
            </button>
          </ScopeGate>
          <ScopeGate allowed={canWrite} missingScope="dns.write">
            <button type="button" disabled={!canPlan} onClick={() => void runPlan()}>
              Plan
            </button>
          </ScopeGate>
          <ScopeGate allowed={canWrite} missingScope="dns.write">
            <button type="button" disabled={!canApply} onClick={openConfirm}>
              Apply
            </button>
          </ScopeGate>
        </div>
      </section>

      {validateResult ? <PlanPanel title="Validate result" plan={validateResult} /> : null}
      {planCurrent && plan ? <PlanPanel title="Plan" plan={plan.body} /> : null}
      {applyResult ? <PlanPanel title="Apply result" plan={applyResult} applied /> : null}

      <ConfirmDialog
        open={confirmOpen}
        title="Apply this plan?"
        confirmLabel="Apply"
        confirmDisabled={reason.trim() === '' || busy === 'apply'}
        onConfirm={() => void runApply()}
        onCancel={closeConfirm}
      >
        <p>Expected revision {plan?.revision || revision}.</p>
        <p>Reason: {reason.trim() || '(required)'}</p>
      </ConfirmDialog>
    </article>
  )
}

function PlanPanel({ title, plan, applied }: { title: string; plan: PlanView; applied?: boolean }) {
  const diff = plan.diff ?? []
  const warnings = plan.warnings ?? []
  const impact = plan.impact
  return (
    <section>
      <h2>{title}</h2>
      {applied ? <p>Applied. Audit event {plan.auditEventId || '—'}</p> : null}
      <dl>
        <dt>Previous revision</dt>
        <dd>{plan.previousRevision || '—'}</dd>
        <dt>Candidate revision</dt>
        <dd>{plan.candidateRevision || '—'}</dd>
        <dt>Drifted</dt>
        <dd>{plan.drifted ? 'Yes' : 'No'}</dd>
        {applied ? (
          <>
            <dt>Generation</dt>
            <dd>{plan.generation ?? '—'}</dd>
          </>
        ) : null}
      </dl>
      {impact ? (
        <div>
          <h3>Impact</h3>
          <dl>
            <dt>Zones</dt>
            <dd>{impact.zones?.join(', ') || '—'}</dd>
            <dt>Names</dt>
            <dd>{impact.names?.join(', ') || '—'}</dd>
            <dt>Forwarding changed</dt>
            <dd>{impact.forwardingChanged ? 'Yes' : 'No'}</dd>
            <dt>Required permissions</dt>
            <dd>{impact.requiredPermissions?.join(', ') || '—'}</dd>
          </dl>
        </div>
      ) : null}
      {warnings.length > 0 ? (
        <div>
          <h3>Warnings</h3>
          <ul>
            {warnings.map((w) => (
              <li key={`${w.code ?? ''}:${w.message ?? ''}`}>
                {w.code}: {w.message}
              </li>
            ))}
          </ul>
        </div>
      ) : null}
      {diff.length > 0 ? (
        <div className="plan-diff">
          <h3>Diff</h3>
          <table>
            <thead>
              <tr>
                <th>Path</th>
                <th>Op</th>
                <th>Before</th>
                <th>After</th>
              </tr>
            </thead>
            <tbody>
              {diff.map((d) => (
                <tr key={`${d.op ?? ''}:${d.path ?? ''}`}>
                  <td>{d.path}</td>
                  <td>{d.op}</td>
                  <td>
                    <code>{jsonPreview(d.before)}</code>
                  </td>
                  <td>
                    <code>{jsonPreview(d.after)}</code>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : (
        <p>No diff entries.</p>
      )}
    </section>
  )
}
