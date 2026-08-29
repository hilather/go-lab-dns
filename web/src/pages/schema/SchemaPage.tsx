import { useQuery } from '@tanstack/react-query'
import { useOutletContext } from 'react-router'
import { ProblemAlert } from '../../components/ProblemAlert'
import type { ShellContext } from '../../components/Shell'
import { queryKeys } from '../../query/keys'
import { statusRevision, useStatusQuery } from '../../query/status'
import { fetchConfigSchema, prettyJSON, problemFrom } from './schema'

export function SchemaPage() {
  const status = useStatusQuery()
  const outlet = useOutletContext<ShellContext | null>()
  const revision = statusRevision(status.data) || outlet?.status?.revisions?.runtimeRevision || ''
  const query = useQuery({
    queryKey: queryKeys.schema(revision),
    queryFn: () => fetchConfigSchema(),
    enabled: revision !== '',
  })
  const err = problemFrom(query.error)

  return (
    <article className="dashboard">
      <div className="page-head">
        <div>
          <h1>Config schema</h1>
          <p className="page-lede">JSON Schema for the LabDNS desired-state document.</p>
        </div>
      </div>
      {err ? <ProblemAlert error={err} /> : null}
      {revision === '' || query.isFetching ? <p className="empty">Loading schema…</p> : null}
      {query.data !== undefined ? (
        <section className="surface">
          <pre className="code-block">{prettyJSON(query.data)}</pre>
        </section>
      ) : null}
    </article>
  )
}
