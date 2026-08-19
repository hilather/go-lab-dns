import { useEffect, useState } from 'react'
import { useOutletContext } from 'react-router'
import { getJSON } from '../../auth/sessionApi'
import type { ShellContext } from '../../components/Shell'
import { readyKind, readyLabel, shortRevision, type VersionView } from '../../status'

function asVersion(v: unknown): VersionView {
  if (v && typeof v === 'object') {
    return v as VersionView
  }
  return {}
}

function yn(v: boolean | undefined, known: boolean): string {
  if (!known) {
    return '—'
  }
  return v ? 'Yes' : 'No'
}

export function DashboardPage() {
  const { status } = useOutletContext<ShellContext>()
  const [version, setVersion] = useState<VersionView | null>(null)
  const [error, setError] = useState('')
  const known = status !== null

  useEffect(() => {
    let cancelled = false
    void getJSON('/v1/version')
      .then((body) => {
        if (!cancelled) {
          setVersion(asVersion(body))
        }
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : 'version request failed')
        }
      })
    return () => {
      cancelled = true
    }
  }, [])

  const kind = readyKind(status?.ready, status?.degraded)
  const rev = status?.revisions?.runtimeRevision

  return (
    <article className="dashboard">
      <h1>Overview</h1>
      {error !== '' ? (
        <p role="alert" className="problem">
          {error}
        </p>
      ) : null}
      <section>
        <h2>Process</h2>
        <dl>
          <dt>Ready</dt>
          {known ? (
            <dd className={`status status-${kind}`}>
              <span className="status-symbol" aria-hidden="true">
                {kind === 'ready' ? '●' : kind === 'degraded' ? '▲' : '○'}
              </span>{' '}
              {readyLabel(kind)}
            </dd>
          ) : (
            <dd>—</dd>
          )}
          <dt>Runtime revision</dt>
          <dd title={rev}>{known ? shortRevision(rev) : '—'}</dd>
          <dt>Bootstrap revision</dt>
          <dd>{known ? shortRevision(status?.revisions?.bootstrapRevision) : '—'}</dd>
          <dt>Generation</dt>
          <dd>{status?.revisions?.generation ?? '—'}</dd>
          <dt>Drifted</dt>
          <dd>{yn(status?.revisions?.drifted, known)}</dd>
        </dl>
      </section>
      <section>
        <h2>Version</h2>
        <dl>
          <dt>Version</dt>
          <dd>{version?.version || status?.version?.version || '—'}</dd>
          <dt>Commit</dt>
          <dd>{version?.commit || status?.version?.commit || '—'}</dd>
          <dt>Build time</dt>
          <dd>{version?.buildTime || status?.version?.buildTime || '—'}</dd>
        </dl>
      </section>
      <section>
        <h2>Listeners</h2>
        {status === null ? (
          <p>—</p>
        ) : status.listeners && status.listeners.length > 0 ? (
          <ul>
            {status.listeners.map((l) => (
              <li key={`${l.name ?? ''}:${l.address ?? ''}`}>
                {l.name}: {l.address}
              </li>
            ))}
          </ul>
        ) : (
          <p>No listeners reported.</p>
        )}
      </section>
      <section>
        <h2>Cache</h2>
        <dl>
          <dt>Enabled</dt>
          <dd>{yn(status?.cache?.enabled, known)}</dd>
          <dt>Entries</dt>
          <dd>
            {status?.cache?.entries ?? '—'} / {status?.cache?.maxEntries ?? '—'}
          </dd>
          <dt>Hits / misses / evicts</dt>
          <dd>
            {status?.cache?.hits ?? '—'} / {status?.cache?.misses ?? '—'} / {status?.cache?.evicts ?? '—'}
          </dd>
        </dl>
      </section>
      <section>
        <h2>Upstreams</h2>
        {status === null ? (
          <p>—</p>
        ) : status.upstreams && status.upstreams.length > 0 ? (
          <ul>
            {status.upstreams.map((u) => (
              <li key={u.id ?? u.endpoint}>
                {u.id} ({u.endpoint}) {u.healthy ? 'healthy' : 'unhealthy'}
              </li>
            ))}
          </ul>
        ) : (
          <p>No upstreams reported.</p>
        )}
      </section>
      <section>
        <h2>Chaos</h2>
        <dl>
          <dt>Enabled</dt>
          <dd>{yn(status?.chaos?.enabled, known)}</dd>
          <dt>Emergency disabled</dt>
          <dd>{yn(status?.chaos?.emergencyDisabled, known)}</dd>
          <dt>Active policies</dt>
          <dd>{status?.chaos?.activePolicies ?? '—'}</dd>
        </dl>
      </section>
      {status?.warnings && status.warnings.length > 0 ? (
        <section>
          <h2>Warnings</h2>
          <ul>
            {status.warnings.map((w) => (
              <li key={`${w.code ?? ''}:${w.message ?? ''}`}>
                {w.code}: {w.message}
              </li>
            ))}
          </ul>
        </section>
      ) : null}
    </article>
  )
}
