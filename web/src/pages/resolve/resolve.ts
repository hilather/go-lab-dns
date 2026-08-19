import { APIError } from '../../auth/sessionApi'
import { createLabdnsClient, throwOnError, type components } from '../../api/client'

export type LabdnsClient = ReturnType<typeof createLabdnsClient>
export type ResolveBody = components['schemas']['ResolveIn']

export const DNS_READ_SCOPE = 'dns.read'

/** REST toResolveIn leaves ApplyChaos false unless options.applyChaos is sent true. */
export const APPLY_CHAOS_DEFAULT = false

export const RR_TYPES = [
  'A',
  'AAAA',
  'CNAME',
  'TXT',
  'MX',
  'SRV',
  'PTR',
  'CAA',
  'NS',
  'SOA',
  'SVCB',
  'HTTPS',
] as const

export const TRANSPORTS = ['udp', 'tcp'] as const

export type ResolveForm = {
  name: string
  type: string
  clientGroup: string
  transport: string
  useCache: boolean
  applyChaos: boolean
}

export type DisplayRow = {
  label: string
  value: string
}

export type ProblemFields = {
  code: string
  detail: string
}

export type SideBySideResult = {
  answer: unknown
  explain: unknown
  answerError: ProblemFields | null
  explainError: ProblemFields | null
}

const ROLE_SCOPES: Record<string, readonly string[]> = {
  viewer: ['dns.read', 'dns.forwarders.read', 'dns.chaos.read'],
  'dns-editor': ['dns.read', 'dns.write'],
  'forwarder-operator': ['dns.read', 'dns.forwarders.read', 'dns.forwarders.write'],
  'chaos-designer': ['dns.read', 'dns.chaos.read', 'dns.chaos.write'],
  'chaos-operator': ['dns.read', 'dns.chaos.read', 'dns.chaos.activate'],
  'chaos-admin': [
    'dns.read',
    'dns.chaos.read',
    'dns.chaos.write',
    'dns.chaos.activate',
    'dns.chaos.emergency',
  ],
  'emergency-operator': ['dns.read', 'dns.chaos.read', 'dns.chaos.emergency'],
}

export function defaultResolveForm(): ResolveForm {
  return {
    name: '',
    type: 'A',
    clientGroup: '',
    transport: 'udp',
    useCache: false,
    applyChaos: APPLY_CHAOS_DEFAULT,
  }
}

export function buildResolveBody(form: ResolveForm): ResolveBody {
  const body: Record<string, unknown> = {
    name: form.name.trim(),
    type: form.type.trim() === '' ? 'A' : form.type.trim(),
  }
  const clientContext: Record<string, unknown> = {}
  const group = form.clientGroup.trim()
  const transport = form.transport.trim()
  if (group !== '') {
    clientContext.clientGroup = group
  }
  if (transport !== '') {
    clientContext.transport = transport
  }
  if (Object.keys(clientContext).length > 0) {
    body.clientContext = clientContext
  }
  body.options = {
    useCache: form.useCache,
    applyChaos: form.applyChaos,
  }
  return body
}

export function actorHasScope(
  actor: { role?: string; scopes?: readonly string[] } | undefined,
  scope: string,
): boolean {
  if (!actor || scope === '') {
    return false
  }
  const scopes = actor.scopes ?? []
  if (scopes.length > 0) {
    if (actor.role === 'administrator' || scopes.includes('dns.admin')) {
      return true
    }
    return scopes.includes(scope)
  }
  if (actor.role === 'administrator') {
    return true
  }
  const roleScopes = ROLE_SCOPES[actor.role ?? '']
  return roleScopes !== undefined && roleScopes.includes(scope)
}

export function asRecord(v: unknown): Record<string, unknown> | null {
  if (v !== null && typeof v === 'object' && !Array.isArray(v)) {
    return v as Record<string, unknown>
  }
  return null
}

export function displayText(v: unknown): string {
  if (v === undefined || v === null || v === '') {
    return '—'
  }
  if (typeof v === 'string' || typeof v === 'number' || typeof v === 'boolean') {
    return String(v)
  }
  if (Array.isArray(v)) {
    if (v.length === 0) {
      return '—'
    }
    if (v.every((x) => typeof x === 'string' || typeof x === 'number' || typeof x === 'boolean')) {
      return v.map(String).join(', ')
    }
  }
  return JSON.stringify(v)
}

export function formatTTL(ttl: unknown): string {
  if (typeof ttl === 'number' && Number.isFinite(ttl)) {
    if (Math.abs(ttl) >= 1e9) {
      return `${ttl / 1e9}s`
    }
    return `${ttl}ns`
  }
  if (typeof ttl === 'string' && ttl !== '') {
    return ttl
  }
  return '—'
}

export function formatRR(rr: unknown): string {
  const rec = asRecord(rr)
  if (!rec) {
    return displayText(rr)
  }
  return `${displayText(rec.name)} ${formatTTL(rec.ttl)} IN ${displayText(rec.type)} ${displayText(rec.data)}`
}

export function rrList(v: unknown): string[] {
  if (!Array.isArray(v) || v.length === 0) {
    return []
  }
  return v.map(formatRR)
}

export function resultFromOut(out: unknown): Record<string, unknown> | null {
  const rec = asRecord(out)
  return rec ? asRecord(rec.result) : null
}

export function explanationFromOut(out: unknown): Record<string, unknown> | null {
  const rec = asRecord(out)
  if (!rec) {
    return null
  }
  const top = asRecord(rec.explanation)
  if (top) {
    return top
  }
  const result = asRecord(rec.result)
  return result ? asRecord(result.explanation) : null
}

export function formatChaosDecision(expl: Record<string, unknown> | null): string {
  if (!expl) {
    return '—'
  }
  if (expl.chaosDisabled === true) {
    const reason = typeof expl.chaosReason === 'string' ? expl.chaosReason : ''
    return reason !== '' ? `disabled: ${reason}` : 'disabled'
  }
  const ids = Array.isArray(expl.chaosPolicyIds) ? expl.chaosPolicyIds.map(String) : []
  const actions = Array.isArray(expl.chaosActions) ? expl.chaosActions.map(String) : []
  if (ids.length === 0 && actions.length === 0) {
    return 'none'
  }
  const parts: string[] = []
  if (ids.length > 0) {
    parts.push(`policies ${ids.join(', ')}`)
  }
  if (actions.length > 0) {
    parts.push(`actions ${actions.join(', ')}`)
  }
  if (typeof expl.chaosReason === 'string' && expl.chaosReason !== '') {
    parts.push(expl.chaosReason)
  }
  return parts.join('; ')
}

function joinKnown(...parts: unknown[]): string {
  const bits = parts.filter((p) => p !== undefined && p !== null && p !== '').map(String)
  return bits.length === 0 ? '—' : bits.join(' / ')
}

export function answerRows(result: Record<string, unknown> | null): DisplayRow[] {
  if (!result) {
    return []
  }
  return [
    { label: 'Rcode', value: displayText(result.rcode) },
    { label: 'Source', value: displayText(result.source) },
    { label: 'Zone', value: displayText(result.zoneId) },
    { label: 'Zone mode', value: displayText(result.zoneMode) },
    { label: 'AA', value: result.aa === true ? 'Yes' : result.aa === false ? 'No' : '—' },
    { label: 'RA', value: result.ra === true ? 'Yes' : result.ra === false ? 'No' : '—' },
    { label: 'Fallthrough', value: result.fallthrough === true ? 'Yes' : 'No' },
    { label: 'Wildcard source', value: displayText(result.wildcardSource) },
    { label: 'Forwarding', value: displayText(result.forwardingId) },
    { label: 'Upstream', value: displayText(result.upstreamId) },
    { label: 'Cache', value: result.source === 'cache' ? 'hit' : 'no' },
  ]
}

export function explainRows(expl: Record<string, unknown> | null): DisplayRow[] {
  if (!expl) {
    return []
  }
  const zone = joinKnown(expl.zoneId, expl.zoneMode)
  const wildcard = joinKnown(expl.wildcardSource, expl.closestEncloser)
  const forwarder = joinKnown(expl.forwardingId, expl.poolId, expl.upstreamId)
  return [
    { label: 'Matched zone', value: zone },
    { label: 'Source', value: displayText(expl.source) },
    { label: 'Wildcard source', value: wildcard },
    { label: 'Forwarder', value: forwarder },
    { label: 'Cache', value: expl.source === 'cache' ? 'hit' : 'no' },
    { label: 'Chaos decision', value: formatChaosDecision(expl) },
    { label: 'Client group', value: displayText(expl.clientGroupId) },
    { label: 'Revision', value: displayText(expl.revision) },
    { label: 'Base rcode', value: displayText(expl.baseRcode) },
  ]
}

export function problemFromUnknown(err: unknown): ProblemFields {
  if (err instanceof APIError) {
    return { code: err.code, detail: err.detail }
  }
  if (err instanceof Error) {
    return { code: '', detail: err.message }
  }
  return { code: '', detail: 'request failed' }
}

export async function postResolve(api: LabdnsClient, body: ResolveBody): Promise<unknown> {
  return throwOnError(await api.POST('/v1/resolve', { body }))
}

export async function postExplain(api: LabdnsClient, body: ResolveBody): Promise<unknown> {
  return throwOnError(await api.POST('/v1/resolve:explain', { body }))
}

export async function resolveAndExplain(api: LabdnsClient, body: ResolveBody): Promise<SideBySideResult> {
  const [answerSettled, explainSettled] = await Promise.allSettled([
    postResolve(api, body),
    postExplain(api, body),
  ])
  const out: SideBySideResult = {
    answer: null,
    explain: null,
    answerError: null,
    explainError: null,
  }
  if (answerSettled.status === 'fulfilled') {
    out.answer = answerSettled.value
  } else {
    out.answerError = problemFromUnknown(answerSettled.reason)
  }
  if (explainSettled.status === 'fulfilled') {
    out.explain = explainSettled.value
  } else {
    out.explainError = problemFromUnknown(explainSettled.reason)
  }
  return out
}
