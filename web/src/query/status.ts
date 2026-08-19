import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect, useRef, useSyncExternalStore } from 'react'
import { client, throwOnError } from '../api/client'
import { invalidateOnRevisionChange, queryKeys } from './keys'

export const STATUS_POLL_MS = 2000

export function pollInterval(visible: boolean, ms: number): number | false {
  return visible ? ms : false
}

function subscribeVisibility(onStoreChange: () => void): () => void {
  document.addEventListener('visibilitychange', onStoreChange)
  return () => document.removeEventListener('visibilitychange', onStoreChange)
}

export function useDocumentVisible(): boolean {
  return useSyncExternalStore(
    subscribeVisibility,
    () => document.visibilityState === 'visible',
    () => true,
  )
}

export function statusRevision(data: unknown): string {
  if (!data || typeof data !== 'object') {
    return ''
  }
  const revs = (data as { revisions?: unknown }).revisions
  if (!revs || typeof revs !== 'object') {
    return ''
  }
  const rev = (revs as { runtimeRevision?: unknown }).runtimeRevision
  return typeof rev === 'string' ? rev : ''
}

export async function fetchStatus(): Promise<unknown> {
  return throwOnError(await client.GET('/v1/status'))
}

export function useStatusQuery() {
  const queryClient = useQueryClient()
  const visible = useDocumentVisible()
  const query = useQuery({
    queryKey: queryKeys.status(),
    queryFn: fetchStatus,
    refetchInterval: pollInterval(visible, STATUS_POLL_MS),
    refetchIntervalInBackground: false,
  })
  const revision = statusRevision(query.data)
  const prev = useRef(revision)
  useEffect(() => {
    const previous = prev.current
    prev.current = revision
    void invalidateOnRevisionChange(queryClient, previous, revision)
  }, [queryClient, revision])
  return query
}
