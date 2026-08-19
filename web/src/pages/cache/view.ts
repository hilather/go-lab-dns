export type CacheStatusView = {
  enabled?: boolean
  maxEntries?: number
  entries?: number
  hits?: number
  misses?: number
  evicts?: number
}

function asRecord(v: unknown): Record<string, unknown> | null {
  if (!v || typeof v !== 'object' || Array.isArray(v)) {
    return null
  }
  return v as Record<string, unknown>
}

function asBool(v: unknown): boolean | undefined {
  return typeof v === 'boolean' ? v : undefined
}

function asNumber(v: unknown): number | undefined {
  return typeof v === 'number' && Number.isFinite(v) ? v : undefined
}

export function parseCacheStatus(data: unknown): CacheStatusView | null {
  const rec = asRecord(data)
  if (!rec) {
    return null
  }
  return {
    enabled: asBool(rec.enabled),
    maxEntries: asNumber(rec.maxEntries),
    entries: asNumber(rec.entries),
    hits: asNumber(rec.hits),
    misses: asNumber(rec.misses),
    evicts: asNumber(rec.evicts),
  }
}

export function yn(v: boolean | undefined, known: boolean): string {
  if (!known || v === undefined) {
    return '—'
  }
  return v ? 'Yes' : 'No'
}

export function num(v: number | undefined, known: boolean): string {
  if (!known || v === undefined) {
    return '—'
  }
  return String(v)
}
