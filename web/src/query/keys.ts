import type { QueryClient } from '@tanstack/react-query'

// Snapshot-backed kinds are invalidated when status.revision changes.
const SNAPSHOT_KINDS = new Set([
  'state',
  'zones',
  'records',
  'chaosPolicies',
  'forwarding',
  'schema',
  'capabilities',
])

export const queryKeys = {
  status: () => ['labdns', 'status'] as const,
  session: () => ['labdns', 'session'] as const,
  version: () => ['labdns', 'version'] as const,
  state: (revision: string) => ['labdns', 'state', revision] as const,
  schema: (revision: string) => ['labdns', 'schema', revision] as const,
  capabilities: (revision: string) => ['labdns', 'capabilities', revision] as const,
  zones: (revision: string) => ['labdns', 'zones', revision] as const,
  zone: (revision: string, zoneId: string) => ['labdns', 'zones', revision, zoneId] as const,
  records: (revision: string, zoneId: string) => ['labdns', 'records', revision, zoneId] as const,
  record: (revision: string, zoneId: string, recordId: string) =>
    ['labdns', 'records', revision, zoneId, recordId] as const,
  forwarding: (revision: string) => ['labdns', 'forwarding', revision] as const,
  pools: (revision: string) => ['labdns', 'forwarding', revision, 'pools'] as const,
  chaosPolicies: (revision: string) => ['labdns', 'chaosPolicies', revision] as const,
  chaosPolicy: (revision: string, policyId: string) =>
    ['labdns', 'chaosPolicies', revision, policyId] as const,
  audit: () => ['labdns', 'audit'] as const,
  docs: (id: string) => ['labdns', 'docs', id] as const,
  liveUpstreams: () => ['labdns', 'live', 'upstreams'] as const,
  liveCache: () => ['labdns', 'live', 'cache'] as const,
  liveChaos: () => ['labdns', 'live', 'chaos'] as const,
}

export function isSnapshotQueryKey(queryKey: readonly unknown[]): boolean {
  return queryKey[0] === 'labdns' && typeof queryKey[1] === 'string' && SNAPSHOT_KINDS.has(queryKey[1])
}

export async function invalidateSnapshotQueries(queryClient: QueryClient): Promise<void> {
  await queryClient.invalidateQueries({
    predicate: (q) => isSnapshotQueryKey(q.queryKey),
  })
}

export function shouldInvalidateOnRevision(previous: string, next: string): boolean {
  return previous !== '' && next !== '' && previous !== next
}

export async function invalidateOnRevisionChange(
  queryClient: QueryClient,
  previous: string,
  next: string,
): Promise<void> {
  if (shouldInvalidateOnRevision(previous, next)) {
    await invalidateSnapshotQueries(queryClient)
  }
}
