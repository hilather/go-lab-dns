import { useQuery } from '@tanstack/react-query'
import { client, throwOnError } from '../api/client'
import { queryKeys } from './keys'
import { pollInterval, useDocumentVisible } from './status'

export const LIVE_POLL_MS = 5000

function liveOptions(visible: boolean) {
  return {
    refetchInterval: pollInterval(visible, LIVE_POLL_MS),
    refetchIntervalInBackground: false,
  }
}

export async function fetchUpstreamsStatus(): Promise<unknown> {
  return throwOnError(await client.GET('/v1/upstreams/status'))
}

export async function fetchCacheStatus(): Promise<unknown> {
  return throwOnError(await client.GET('/v1/cache/status'))
}

export async function fetchChaosStatus(): Promise<unknown> {
  return throwOnError(await client.GET('/v1/chaos/status'))
}

export function useUpstreamsStatusQuery() {
  const visible = useDocumentVisible()
  return useQuery({
    queryKey: queryKeys.liveUpstreams(),
    queryFn: fetchUpstreamsStatus,
    ...liveOptions(visible),
  })
}

export function useCacheStatusQuery() {
  const visible = useDocumentVisible()
  return useQuery({
    queryKey: queryKeys.liveCache(),
    queryFn: fetchCacheStatus,
    ...liveOptions(visible),
  })
}

export function useChaosStatusQuery() {
  const visible = useDocumentVisible()
  return useQuery({
    queryKey: queryKeys.liveChaos(),
    queryFn: fetchChaosStatus,
    ...liveOptions(visible),
  })
}
