import { client, createLabdnsClient, throwOnError } from '../../api/client'
import { APIError } from '../../auth/sessionApi'

export type DocsClient = Pick<typeof client, 'GET'>

function liveClient(): DocsClient {
  return createLabdnsClient()
}

export const DOC_CATALOG = [
  { id: 'dns-semantics', title: 'DNS semantics', path: '/v1/docs/dns-semantics' },
  { id: 'chaos-safety', title: 'Chaos safety', path: '/v1/docs/chaos-safety' },
] as const

export type DocId = (typeof DOC_CATALOG)[number]['id']

export function isDocId(id: string): id is DocId {
  return DOC_CATALOG.some((d) => d.id === id)
}

export function docTitle(id: string): string {
  return DOC_CATALOG.find((d) => d.id === id)?.title ?? id
}

export function problemFrom(err: unknown): { code?: string; detail?: string; message?: string } | null {
  if (err == null) {
    return null
  }
  if (err instanceof APIError) {
    return err
  }
  if (err instanceof Error) {
    return { message: err.message }
  }
  return { message: 'request failed' }
}

export async function fetchDoc(id: string, api: DocsClient = liveClient()): Promise<string> {
  switch (id) {
    case 'dns-semantics':
      return throwOnError(await api.GET('/v1/docs/dns-semantics', { parseAs: 'text' }))
    case 'chaos-safety':
      return throwOnError(await api.GET('/v1/docs/chaos-safety', { parseAs: 'text' }))
    default:
      throw new APIError(404, 'not_found', `unknown document ${id}`)
  }
}
