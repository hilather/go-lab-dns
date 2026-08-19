import { useQuery } from '@tanstack/react-query'
import { ProblemAlert } from '../../components/ProblemAlert'
import { queryKeys } from '../../query/keys'
import { statusRevision, useStatusQuery } from '../../query/status'
import { fetchConfigSchema, prettyJSON, problemFrom } from './schema'

export function SchemaPage() {
  const status = useStatusQuery()
  const revision = statusRevision(status.data)
  const query = useQuery({
    queryKey: queryKeys.schema(revision),
    queryFn: () => fetchConfigSchema(),
  })
  const err = problemFrom(query.error)

  return (
    <article className="dashboard">
      <h1>Config schema</h1>
      {err ? <ProblemAlert error={err} /> : null}
      {query.isPending ? <p>Loading schema…</p> : null}
      {query.data !== undefined ? <pre>{prettyJSON(query.data)}</pre> : null}
    </article>
  )
}
