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
      <h1>Config schema</h1>
      {err ? <ProblemAlert error={err} /> : null}
      {revision === '' || query.isFetching ? <p>Loading schema…</p> : null}
      {query.data !== undefined ? <pre>{prettyJSON(query.data)}</pre> : null}
    </article>
  )
}
