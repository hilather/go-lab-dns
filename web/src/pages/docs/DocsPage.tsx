import { useQuery } from '@tanstack/react-query'
import { Link, useParams } from 'react-router'
import { ProblemAlert } from '../../components/ProblemAlert'
import { queryKeys } from '../../query/keys'
import { ROUTES } from '../../routes'
import { DOC_CATALOG, docTitle, fetchDoc, isDocId, problemFrom } from './docs'

function DocsIndex() {
  return (
    <article className="dashboard">
      <h1>Docs</h1>
      <ul>
        {DOC_CATALOG.map((doc) => (
          <li key={doc.id}>
            <Link to={`/docs/${doc.id}`}>{doc.title}</Link>
          </li>
        ))}
      </ul>
    </article>
  )
}

function DocDetail({ id }: { id: string }) {
  const known = isDocId(id)
  const query = useQuery({
    queryKey: queryKeys.docs(id),
    queryFn: () => fetchDoc(id),
    enabled: known,
  })
  const unknown = known ? null : { code: 'not_found', detail: `unknown document ${id}` }
  const err = unknown || problemFrom(query.error)

  return (
    <article className="dashboard">
      <h1>{docTitle(id)}</h1>
      <p>
        <Link to={ROUTES.docsIndex}>All docs</Link>
      </p>
      {err ? <ProblemAlert error={err} /> : null}
      {known && query.isPending ? <p>Loading document…</p> : null}
      {typeof query.data === 'string' ? <pre>{query.data}</pre> : null}
    </article>
  )
}

export function DocsPage() {
  const { id } = useParams()
  if (!id) {
    return <DocsIndex />
  }
  return <DocDetail id={id} />
}
