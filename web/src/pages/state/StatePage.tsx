import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { useOutletContext } from 'react-router'
import { ProblemAlert } from '../../components/ProblemAlert'
import type { ShellContext } from '../../components/Shell'
import { queryKeys } from '../../query/keys'
import { statusRevision, useStatusQuery } from '../../query/status'
import { downloadStateExport, fetchState, prettyJSON, problemFrom, type ExportFormat } from './state'

function yn(v: boolean | undefined): string {
  if (v === undefined) {
    return '—'
  }
  return v ? 'Yes' : 'No'
}

export function StatePage() {
  const status = useStatusQuery()
  const outlet = useOutletContext<ShellContext | null>()
  const revision = statusRevision(status.data) || outlet?.status?.revisions?.runtimeRevision || ''
  const query = useQuery({
    queryKey: queryKeys.state(revision),
    queryFn: () => fetchState(),
    enabled: revision !== '',
  })
  const [exportError, setExportError] = useState<unknown>(null)
  const [exporting, setExporting] = useState<ExportFormat | null>(null)

  async function onDownload(format: ExportFormat) {
    setExportError(null)
    setExporting(format)
    try {
      await downloadStateExport(format)
    } catch (err) {
      setExportError(err)
    } finally {
      setExporting(null)
    }
  }

  const data = query.data && typeof query.data === 'object' ? (query.data as Record<string, unknown>) : null
  const err = problemFrom(query.error) || problemFrom(exportError)

  return (
    <article className="dashboard">
      <div className="page-head">
        <div>
          <h1>State</h1>
          <p className="page-lede">Compiled runtime snapshot. Export is read-only; writes go through Changes.</p>
        </div>
      </div>
      {err ? <ProblemAlert error={err} /> : null}
      {revision === '' || query.isFetching ? <p className="empty">Loading state…</p> : null}
      {data ? (
        <section className="surface">
          <h2>Revisions</h2>
          <dl>
            <dt>Runtime revision</dt>
            <dd>{typeof data.runtimeRevision === 'string' ? data.runtimeRevision : '—'}</dd>
            <dt>Bootstrap revision</dt>
            <dd>{typeof data.bootstrapRevision === 'string' ? data.bootstrapRevision : '—'}</dd>
            <dt>Generation</dt>
            <dd>{typeof data.generation === 'number' ? data.generation : '—'}</dd>
            <dt>Drifted</dt>
            <dd>{yn(typeof data.drifted === 'boolean' ? data.drifted : undefined)}</dd>
            <dt>Loaded at</dt>
            <dd>{typeof data.loadedAt === 'string' ? data.loadedAt : '—'}</dd>
          </dl>
        </section>
      ) : null}
      <section className="surface">
        <h2>Export</h2>
        <p>Download canonical desired state. YAML is the default REST encoding; JSON includes export metadata.</p>
        <p>
          <button type="button" disabled={exporting !== null} onClick={() => void onDownload('yaml')}>
            {exporting === 'yaml' ? 'Downloading YAML…' : 'Download YAML'}
          </button>{' '}
          <button type="button" disabled={exporting !== null} onClick={() => void onDownload('json')}>
            {exporting === 'json' ? 'Downloading JSON…' : 'Download JSON'}
          </button>
        </p>
      </section>
      {data ? (
        <section className="surface">
          <h2>Canonical</h2>
          <pre className="code-block">{prettyJSON(data.canonical ?? data)}</pre>
        </section>
      ) : null}
    </article>
  )
}
