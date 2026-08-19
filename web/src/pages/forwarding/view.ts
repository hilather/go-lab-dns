export type FailoverView = {
  timeout?: string
  onTimeout?: boolean
  onTransportError?: boolean
  onSERVFAIL?: boolean
  onREFUSED?: boolean
  udpTruncateRetryTCP?: boolean
}

export type ForwardingPolicyView = {
  id: string
  suffix: string
  upstreamPool: string
  failover: FailoverView
}

export type UpstreamView = {
  id: string
  endpoint: string
  transport: string
}

export type PoolView = {
  id: string
  strategy: string
  upstreams: UpstreamView[]
}

export type UpstreamStatusView = {
  id: string
  poolId: string
  endpoint: string
  transport: string
  healthy?: boolean
}

function asRecord(v: unknown): Record<string, unknown> | null {
  if (!v || typeof v !== 'object' || Array.isArray(v)) {
    return null
  }
  return v as Record<string, unknown>
}

function asString(v: unknown): string {
  if (typeof v === 'string') {
    return v
  }
  if (typeof v === 'number' && Number.isFinite(v)) {
    return String(v)
  }
  return ''
}

function asBool(v: unknown): boolean | undefined {
  return typeof v === 'boolean' ? v : undefined
}

function parseFailover(v: unknown): FailoverView {
  const rec = asRecord(v)
  if (!rec) {
    return {}
  }
  return {
    timeout: asString(rec.timeout) || undefined,
    onTimeout: asBool(rec.onTimeout),
    onTransportError: asBool(rec.onTransportError),
    onSERVFAIL: asBool(rec.onSERVFAIL),
    onREFUSED: asBool(rec.onREFUSED),
    udpTruncateRetryTCP: asBool(rec.udpTruncateRetryTCP),
  }
}

function parsePolicy(v: unknown): ForwardingPolicyView | null {
  const rec = asRecord(v)
  if (!rec) {
    return null
  }
  const id = asString(rec.id)
  if (id === '') {
    return null
  }
  return {
    id,
    suffix: asString(rec.suffix),
    upstreamPool: asString(rec.upstreamPool),
    failover: parseFailover(rec.failover),
  }
}

function parseUpstream(v: unknown): UpstreamView | null {
  const rec = asRecord(v)
  if (!rec) {
    return null
  }
  const id = asString(rec.id)
  if (id === '') {
    return null
  }
  return {
    id,
    endpoint: asString(rec.endpoint),
    transport: asString(rec.transport),
  }
}

function parsePool(v: unknown): PoolView | null {
  const rec = asRecord(v)
  if (!rec) {
    return null
  }
  const id = asString(rec.id)
  if (id === '') {
    return null
  }
  const raw = rec.upstreams
  const upstreams: UpstreamView[] = []
  if (Array.isArray(raw)) {
    for (const item of raw) {
      const u = parseUpstream(item)
      if (u) {
        upstreams.push(u)
      }
    }
  }
  return {
    id,
    strategy: asString(rec.strategy),
    upstreams,
  }
}

function parseUpstreamStatus(v: unknown): UpstreamStatusView | null {
  const rec = asRecord(v)
  if (!rec) {
    return null
  }
  const id = asString(rec.id)
  if (id === '') {
    return null
  }
  return {
    id,
    poolId: asString(rec.poolId),
    endpoint: asString(rec.endpoint),
    transport: asString(rec.transport),
    healthy: asBool(rec.healthy),
  }
}

function listFrom(data: unknown, key: string): unknown[] {
  const rec = asRecord(data)
  if (!rec) {
    return []
  }
  const raw = rec[key]
  return Array.isArray(raw) ? raw : []
}

export function parsePolicies(data: unknown): ForwardingPolicyView[] {
  const out: ForwardingPolicyView[] = []
  for (const item of listFrom(data, 'policies')) {
    const p = parsePolicy(item)
    if (p) {
      out.push(p)
    }
  }
  return out
}

export function parsePools(data: unknown): PoolView[] {
  const out: PoolView[] = []
  for (const item of listFrom(data, 'pools')) {
    const p = parsePool(item)
    if (p) {
      out.push(p)
    }
  }
  return out
}

export function parseUpstreamsStatus(data: unknown): UpstreamStatusView[] {
  const out: UpstreamStatusView[] = []
  for (const item of listFrom(data, 'upstreams')) {
    const u = parseUpstreamStatus(item)
    if (u) {
      out.push(u)
    }
  }
  return out
}

export function yn(v: boolean | undefined): string {
  if (v === undefined) {
    return '—'
  }
  return v ? 'Yes' : 'No'
}

export function formatFailover(fo: FailoverView): string {
  const parts: string[] = []
  if (fo.timeout) {
    parts.push(`timeout ${fo.timeout}`)
  }
  if (fo.onTimeout) {
    parts.push('on timeout')
  }
  if (fo.onTransportError) {
    parts.push('on transport error')
  }
  if (fo.onSERVFAIL) {
    parts.push('on SERVFAIL')
  }
  if (fo.onREFUSED) {
    parts.push('on REFUSED')
  }
  if (fo.udpTruncateRetryTCP) {
    parts.push('UDP truncate retry TCP')
  }
  return parts.length > 0 ? parts.join('; ') : 'none'
}

export function healthKind(healthy: boolean | undefined): 'healthy' | 'unhealthy' | 'unknown' {
  if (healthy === true) {
    return 'healthy'
  }
  if (healthy === false) {
    return 'unhealthy'
  }
  return 'unknown'
}

export function healthSymbol(kind: ReturnType<typeof healthKind>): string {
  switch (kind) {
    case 'healthy':
      return '●'
    case 'unhealthy':
      return '▲'
    default:
      return '○'
  }
}

export function healthLabel(kind: ReturnType<typeof healthKind>): string {
  switch (kind) {
    case 'healthy':
      return 'Healthy'
    case 'unhealthy':
      return 'Unhealthy'
    default:
      return 'Unknown'
  }
}
