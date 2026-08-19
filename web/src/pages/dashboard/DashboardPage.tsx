import { useEffect, useState } from 'react'
import { getJSON } from '../../auth/sessionApi'
import { readyKind, readyLabel, shortRevision } from '../../status'

type VersionView = {
  version?: string
  commit?: string
  buildTime?: string
}

type StatusView = {
  ready?: boolean
  degraded?: boolean
  version?: VersionView
  revisions?: {
    bootstrapRevision?: string
    runtimeRevision?: string
    generation?: number
    drifted?: boolean
    loadedAt?: string
  }
  listeners?: { name?: string; address?: string }[]
  cache?: {
    enabled?: boolean
    maxEntries?: number
    entries?: number
    hits?: number
    misses?: number
    evicts?: number
  }
  upstreams?: {
    id?: string
    poolId?: string
    endpoint?: string
    transport?: string
    healthy?: boolean
  }[]
  chaos?: {
    enabled?: boolean
    emergencyDisabled?: boolean
    activePolicies?: number
    nearestExpiry?: string
  }
  warnings?: { code?: string; message?: string }[]
}

function asStatus(v: unknown): StatusView {
  if (v && typeof v === 'object') {
    return v as StatusView
  }
  return {}
}

function asVersion(v: unknown): VersionView {
  if (v && typeof v === 'object') {
    return v as VersionView
  }
  return {}
}

export function DashboardPage() {
  const [status, setStatus] = useState<StatusView | null>(null)
  const [version, setVersion] = useState<VersionView | null>(null)
  const [error, setError] = useState('')

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

    const load = () => {
      if (document.visibilityState !== 'visible') {
        return
      }
      void getJSON('/v1/status')
        .then((body) => {
          if (cancelled) {
            return
          }
          setStatus(asStatus(body))
          setError('')
        })
        .catch((err: unknown) => {
          if (!cancelled) {
            setError(err instanceof Error ? err.message : 'status request failed')
          }
        })
    }
    load()
    const id = window.setInterval(load, 2000)
    const onVis = () => {
      if (document.visibilityState === 'visible') {
        load()
      }
    }
    document.addEventListener('visibilitychange', onVis)
    return () => {
      cancelled = true
      window.clearInterval(id)
      document.removeEventListener('visibilitychange', onVis)
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
          <dd className={`status status-${kind}`}>
            <span className="status-symbol" aria-hidden="true">
              {kind === 'ready' ? '●' : kind === 'degraded' ? '▲' : '○'}
            </span>{' '}
            {readyLabel(kind)}
          </dd>
          <dt>Runtime revision</dt>
          <dd title={rev}>{shortRevision(rev)}</dd>
          <dt>Bootstrap revision</dt>
          <dd>{shortRevision(status?.revisions?.bootstrapRevision)}</dd>
          <dt>Generation</dt>
          <dd>{status?.revisions?.generation ?? '—'}</dd>
          <dt>Drifted</dt>
          <dd>{status?.revisions?.drifted ? 'Yes' : 'No'}</dd>
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
        {status?.listeners && status.listeners.length > 0 ? (
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
          <dd>{status?.cache?.enabled ? 'Yes' : 'No'}</dd>
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
        {status?.upstreams && status.upstreams.length > 0 ? (
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
          <dd>{status?.chaos?.enabled ? 'Yes' : 'No'}</dd>
          <dt>Emergency disabled</dt>
          <dd>{status?.chaos?.emergencyDisabled ? 'Yes' : 'No'}</dd>
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
