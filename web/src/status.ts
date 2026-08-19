export type ReadyKind = 'ready' | 'degraded' | 'not-ready'

export type VersionView = {
  version?: string
  commit?: string
  buildTime?: string
}

export type StatusView = {
  ready?: boolean
  degraded?: boolean
  version?: VersionView
  revisions?: {
    bootstrapRevision?: string
    runtimeRevision?: string
    generation?: number
    drifted?: boolean
    loadedAt?: string
  }
  listeners?: { name?: string; address?: string }[]
  cache?: {
    enabled?: boolean
    maxEntries?: number
    entries?: number
    hits?: number
    misses?: number
    evicts?: number
  }
  upstreams?: {
    id?: string
    poolId?: string
    endpoint?: string
    transport?: string
    healthy?: boolean
  }[]
  chaos?: {
    enabled?: boolean
    emergencyDisabled?: boolean
    activePolicies?: number
    nearestExpiry?: string
  }
  warnings?: { code?: string; message?: string }[]
}

export function shortRevision(rev: string | undefined): string {
  if (!rev) {
    return 'unknown'
  }
  const hex = rev.startsWith('sha256:') ? rev.slice('sha256:'.length) : rev
  if (hex.length <= 12) {
    return hex
  }
  return hex.slice(0, 12)
}

export function readyKind(ready: boolean | undefined, degraded: boolean | undefined): ReadyKind {
  if (!ready) {
    return 'not-ready'
  }
  if (degraded) {
    return 'degraded'
  }
  return 'ready'
}

export function readyLabel(kind: ReadyKind): string {
  switch (kind) {
    case 'ready':
      return 'Ready'
    case 'degraded':
      return 'Degraded'
    default:
      return 'Not ready'
  }
}
