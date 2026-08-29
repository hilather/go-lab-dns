import { useQuery } from '@tanstack/react-query'
import { Fragment, useState, type FormEvent } from 'react'
import { client, throwOnError } from '../../api/client'
import { ProblemAlert } from '../../components/ProblemAlert'
import { ScopeGate } from '../../components/ScopeGate'
import { queryKeys } from '../../query/keys'
import {
  actorHasScope,
  answerRows,
  buildResolveBody,
  defaultResolveForm,
  DNS_READ_SCOPE,
  explanationFromOut,
  explainRows,
  problemFromUnknown,
  resolveAndExplain,
  resultFromOut,
  RR_TYPES,
  rrList,
  TRANSPORTS,
  type LabdnsClient,
  type ProblemFields,
  type ResolveForm,
} from './resolve'
import styles from './resolve.module.css'

function Rows({ rows }: { rows: ReturnType<typeof answerRows> }) {
  if (rows.length === 0) {
    return null
  }
  return (
    <dl>
      {rows.map((row) => (
        <Fragment key={row.label}>
          <dt>{row.label}</dt>
          <dd>{row.value}</dd>
        </Fragment>
      ))}
    </dl>
  )
}

function RRSet({ title, records }: { title: string; records: string[] }) {
  return (
    <div className={styles.rrsets}>
      <h3>{title}</h3>
      {records.length === 0 ? (
        <p>None</p>
      ) : (
        <ul>
          {records.map((rr, i) => (
            <li key={`${title}-${i}`}>{rr}</li>
          ))}
        </ul>
      )}
    </div>
  )
}

export function ResolvePage({ api = client }: { api?: LabdnsClient } = {}) {
  const [form, setForm] = useState<ResolveForm>(defaultResolveForm)
  const [busy, setBusy] = useState(false)
  const [formError, setFormError] = useState('')
  const [answer, setAnswer] = useState<unknown>(null)
  const [explain, setExplain] = useState<unknown>(null)
  const [answerError, setAnswerError] = useState<ProblemFields | null>(null)
  const [explainError, setExplainError] = useState<ProblemFields | null>(null)

  const sessionQuery = useQuery({
    queryKey: queryKeys.session(),
    queryFn: async () => throwOnError(await api.GET('/v1/session')),
  })

  const actor = sessionQuery.data?.actor
  const sessionReady = sessionQuery.isSuccess
  const canResolve = actorHasScope(actor, DNS_READ_SCOPE)
  const disabled = busy || !sessionReady

  function patch<K extends keyof ResolveForm>(key: K, value: ResolveForm[K]) {
    setForm((prev) => ({ ...prev, [key]: value }))
  }

  async function onSubmit(ev: FormEvent<HTMLFormElement>) {
    ev.preventDefault()
    if (!canResolve) {
      return
    }
    if (form.name.trim() === '') {
      setFormError('name is required')
      return
    }
    setFormError('')
    setAnswer(null)
    setExplain(null)
    setAnswerError(null)
    setExplainError(null)
    setBusy(true)
    try {
      const out = await resolveAndExplain(api, buildResolveBody(form))
      setAnswer(out.answer)
      setExplain(out.explain)
      setAnswerError(out.answerError)
      setExplainError(out.explainError)
    } catch (err) {
      setFormError(problemFromUnknown(err).detail)
    } finally {
      setBusy(false)
    }
  }

  const result = resultFromOut(answer)
  const expl = explanationFromOut(explain)
  const submit = (
    <button type="submit" className="btn-accent" disabled={disabled}>
      Resolve
    </button>
  )

  return (
    <article className={styles.page}>
      <div className="page-head">
        <div>
          <h1>Resolve</h1>
          <p className={`page-lede ${styles.hint}`}>
            Management-plane lookup. Apply chaos defaults to off, matching REST.
          </p>
        </div>
      </div>
      {sessionQuery.isError ? <ProblemAlert error={problemFromUnknown(sessionQuery.error)} /> : null}
      {formError !== '' ? <ProblemAlert detail={formError} /> : null}
      <form className={`${styles.form} stack-form`} onSubmit={(ev) => void onSubmit(ev)}>
        <label>
          Name
          <input
            name="name"
            value={form.name}
            autoComplete="off"
            required
            disabled={disabled}
            onChange={(e) => patch('name', e.target.value)}
          />
        </label>
        <label>
          Type
          <input
            name="type"
            list="resolve-types"
            value={form.type}
            autoComplete="off"
            disabled={disabled}
            onChange={(e) => patch('type', e.target.value)}
          />
        </label>
        <datalist id="resolve-types">
          {RR_TYPES.map((t) => (
            <option key={t} value={t} />
          ))}
        </datalist>
        <label>
          Client group
          <input
            name="clientGroup"
            value={form.clientGroup}
            autoComplete="off"
            disabled={disabled}
            onChange={(e) => patch('clientGroup', e.target.value)}
          />
        </label>
        <label>
          Transport
          <select
            name="transport"
            value={form.transport}
            disabled={disabled}
            onChange={(e) => patch('transport', e.target.value)}
          >
            {TRANSPORTS.map((t) => (
              <option key={t} value={t}>
                {t}
              </option>
            ))}
          </select>
        </label>
        <label className={styles.check}>
          <input
            name="useCache"
            type="checkbox"
            checked={form.useCache}
            disabled={disabled}
            onChange={(e) => patch('useCache', e.target.checked)}
          />
          Use cache
        </label>
        <label className={styles.check}>
          <input
            name="applyChaos"
            type="checkbox"
            checked={form.applyChaos}
            disabled={disabled}
            onChange={(e) => patch('applyChaos', e.target.checked)}
          />
          Apply chaos
        </label>
        {sessionReady && !canResolve ? (
          <ScopeGate allowed={false} missingScope={DNS_READ_SCOPE}>
            {submit}
          </ScopeGate>
        ) : (
          submit
        )}
      </form>
      <div className={styles.columns}>
        <section className={`${styles.panel} surface`} aria-labelledby="resolve-answer-heading">
          <h2 id="resolve-answer-heading">Answer</h2>
          {answerError ? <ProblemAlert error={answerError} /> : null}
          {result ? (
            <>
              <Rows rows={answerRows(result)} />
              <RRSet title="Answers" records={rrList(result.answers)} />
              <RRSet title="Authority" records={rrList(result.authority)} />
              <RRSet title="Additional" records={rrList(result.additional)} />
            </>
          ) : answerError ? null : (
            <p className="empty">Submit a query to see the answer.</p>
          )}
        </section>
        <section className={`${styles.panel} surface`} aria-labelledby="resolve-explain-heading">
          <h2 id="resolve-explain-heading">Explain</h2>
          {explainError ? <ProblemAlert error={explainError} /> : null}
          {expl ? (
            <Rows rows={explainRows(expl)} />
          ) : explainError ? null : (
            <p className="empty">Submit a query to see the explanation.</p>
          )}
        </section>
      </div>
    </article>
  )
}
