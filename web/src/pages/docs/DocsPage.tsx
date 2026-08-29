import { useQuery } from '@tanstack/react-query'
import { Link, useParams } from 'react-router'
import { ProblemAlert } from '../../components/ProblemAlert'
import { queryKeys } from '../../query/keys'
import { ROUTES } from '../../routes'
import { DOC_CATALOG, docTitle, fetchDoc, isDocId, problemFrom } from './docs'

function DocsIndex() {
  return (
    <article className="dashboard">
      <div className="page-head">
        <div>
          <h1>Docs</h1>
          <p className="page-lede">Operator documents served from the management API.</p>
        </div>
      </div>
      <section className="surface">
      <ul>
        {DOC_CATALOG.map((doc) => (
          <li key={doc.id}>
            <Link to={`${ROUTES.docsIndex}/${doc.id}`}>{doc.title}</Link>
          </li>
        ))}
      </ul>
      </section>
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
      <p className="page-crumb">
        <Link to={ROUTES.docsIndex}>All docs</Link>
      </p>
      <div className="page-head">
        <div>
          <h1>{docTitle(id)}</h1>
          <p className="page-lede">Rendered as text. HTML in the document is not executed.</p>
        </div>
      </div>
      {err ? <ProblemAlert error={err} /> : null}
      {known && query.isPending ? <p className="empty">Loading document…</p> : null}
      {typeof query.data === 'string' ? (
        <section className="surface">
          <pre className="code-block">{query.data}</pre>
        </section>
      ) : null}
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
