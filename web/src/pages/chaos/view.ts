import { useQuery } from '@tanstack/react-query'
import { client, throwOnError } from '../../api/client'
import { queryKeys } from '../../query/keys'
import { statusRevision, useStatusQuery } from '../../query/status'

export const MUTATIONS_UI003 = 'mutations in UI-003'
export const SCOPE_ADMIN = 'dns.admin'
export const SCOPE_CHAOS_READ = 'dns.chaos.read'
export const SCOPE_CHAOS_ACTIVATE = 'dns.chaos.activate'
export const SCOPE_CHAOS_EMERGENCY = 'dns.chaos.emergency'

export type ChaosStatusView = {
  enabled?: boolean
  emergencyDisabled?: boolean
  activePolicies?: number
  nearestExpiry: string
}

export type ChaosPolicyListItem = {
  id: string
  owner: string
  enabled?: boolean
  safetyClass: string
  expiresAt: string
}

export type ChaosScopeView = {
  recordIds: string[]
  owners: string[]
  wildcardSourceIds: string[]
  zones: string[]
  forwardingPolicyIds: string[]
  upstreamPools: string[]
  clientGroups: string[]
  qtypes: string[]
  transports: string[]
}

export type ChaosSelectorView = {
  mode: string
  seed: string
  probability?: number
  timeBucket: string
  everyNth?: number
  samplingKey: string
  period: string
}

export type ChaosActionView = {
  type: string
  phase: string
  distribution: string
  duration: string
  min: string
  max: string
  value: string
  values: string[]
  ttl: string
  limit?: number
  upstreamId: string
  hold: string
  edeCode?: number
  edeText: string
}

export type ChaosOutcomeView = {
  id: string
  weight?: number
  actions: ChaosActionView[]
}

export type ChaosBudgetView = {
  maxDelay: string
  maxConcurrency?: number
  maxRate?: number
  maxFrequency?: number
}

export type ChaosPolicyView = ChaosPolicyListItem & {
  description: string
  reason: string
  ticket: string
  startsAt: string
  composition: string
  exclusiveGroup: string
  labels: { key: string; value: string }[]
  scope: ChaosScopeView
  selector: ChaosSelectorView
  outcomes: ChaosOutcomeView[]
  budget: ChaosBudgetView | null
}

export type SessionActorView = {
  id: string
  role: string
  scopes: string[]
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

function asNumber(v: unknown): number | undefined {
  return typeof v === 'number' && Number.isFinite(v) ? v : undefined
}

function asStringList(v: unknown): string[] {
  if (!Array.isArray(v)) {
    return []
  }
  return v.map(asString).filter((s) => s !== '')
}

function listFrom(data: unknown, key: string): unknown[] {
  const rec = asRecord(data)
  if (!rec) {
    return []
  }
  const raw = rec[key]
  return Array.isArray(raw) ? raw : []
}

export function parseChaosStatus(data: unknown): ChaosStatusView | null {
  const rec = asRecord(data)
  if (!rec) {
    return null
  }
  return {
    enabled: asBool(rec.enabled),
    emergencyDisabled: asBool(rec.emergencyDisabled),
    activePolicies: asNumber(rec.activePolicies),
    nearestExpiry: asString(rec.nearestExpiry),
  }
}

function parseListItem(v: unknown): ChaosPolicyListItem | null {
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
    owner: asString(rec.owner),
    enabled: asBool(rec.enabled),
    safetyClass: asString(rec.safetyClass),
    expiresAt: asString(rec.expiresAt),
  }
}

export function parseChaosPolicies(data: unknown): ChaosPolicyListItem[] {
  const out: ChaosPolicyListItem[] = []
  for (const item of listFrom(data, 'policies')) {
    const p = parseListItem(item)
    if (p) {
      out.push(p)
    }
  }
  return out
}

function parseScope(v: unknown): ChaosScopeView {
  const rec = asRecord(v)
  if (!rec) {
    return {
      recordIds: [],
      owners: [],
      wildcardSourceIds: [],
      zones: [],
      forwardingPolicyIds: [],
      upstreamPools: [],
      clientGroups: [],
      qtypes: [],
      transports: [],
    }
  }
  return {
    recordIds: asStringList(rec.recordIds),
    owners: asStringList(rec.owners),
    wildcardSourceIds: asStringList(rec.wildcardSourceIds),
    zones: asStringList(rec.zones),
    forwardingPolicyIds: asStringList(rec.forwardingPolicyIds),
    upstreamPools: asStringList(rec.upstreamPools),
    clientGroups: asStringList(rec.clientGroups),
    qtypes: asStringList(rec.qtypes),
    transports: asStringList(rec.transports),
  }
}

function parseSelector(v: unknown): ChaosSelectorView {
  const rec = asRecord(v)
  if (!rec) {
    return { mode: '', seed: '', timeBucket: '', samplingKey: '', period: '' }
  }
  return {
    mode: asString(rec.mode),
    seed: asString(rec.seed),
    probability: asNumber(rec.probability),
    timeBucket: asString(rec.timeBucket),
    everyNth: asNumber(rec.everyNth),
    samplingKey: asString(rec.samplingKey),
    period: asString(rec.period),
  }
}

function parseEde(v: unknown): { edeCode?: number; edeText: string } {
  const rec = asRecord(v)
  if (!rec) {
    return { edeText: '' }
  }
  return { edeCode: asNumber(rec.code), edeText: asString(rec.text) }
}

function parseAction(v: unknown): ChaosActionView | null {
  const rec = asRecord(v)
  if (!rec) {
    return null
  }
  const type = asString(rec.type)
  if (type === '') {
    return null
  }
  const ede = parseEde(rec.ede)
  return {
    type,
    phase: asString(rec.phase),
    distribution: asString(rec.distribution),
    duration: asString(rec.duration),
    min: asString(rec.min),
    max: asString(rec.max),
    value: asString(rec.value),
    values: asStringList(rec.values),
    ttl: asString(rec.ttl),
    limit: asNumber(rec.limit),
    upstreamId: asString(rec.upstreamId),
    hold: asString(rec.hold),
    edeCode: ede.edeCode,
    edeText: ede.edeText,
  }
}

function parseOutcome(v: unknown): ChaosOutcomeView | null {
  const rec = asRecord(v)
  if (!rec) {
    return null
  }
  const id = asString(rec.id)
  if (id === '') {
    return null
  }
  const actions: ChaosActionView[] = []
  if (Array.isArray(rec.actions)) {
    for (const a of rec.actions) {
      const parsed = parseAction(a)
      if (parsed) {
        actions.push(parsed)
      }
    }
  }
  return { id, weight: asNumber(rec.weight), actions }
}

function parseBudget(v: unknown): ChaosBudgetView | null {
  const rec = asRecord(v)
  if (!rec) {
    return null
  }
  return {
    maxDelay: asString(rec.maxDelay),
    maxConcurrency: asNumber(rec.maxConcurrency),
    maxRate: asNumber(rec.maxRate),
    maxFrequency: asNumber(rec.maxFrequency),
  }
}

function parseLabels(v: unknown): { key: string; value: string }[] {
  const rec = asRecord(v)
  if (!rec) {
    return []
  }
  const out: { key: string; value: string }[] = []
  for (const [key, value] of Object.entries(rec)) {
    out.push({ key, value: asString(value) })
  }
  return out
}

export function parseChaosPolicy(data: unknown): ChaosPolicyView | null {
  const item = parseListItem(data)
  if (!item) {
    return null
  }
  const rec = asRecord(data)
  if (!rec) {
    return null
  }
  const outcomes: ChaosOutcomeView[] = []
  if (Array.isArray(rec.outcomes)) {
    for (const o of rec.outcomes) {
      const parsed = parseOutcome(o)
      if (parsed) {
        outcomes.push(parsed)
      }
    }
  }
  return {
    ...item,
    description: asString(rec.description),
    reason: asString(rec.reason),
    ticket: asString(rec.ticket),
    startsAt: asString(rec.startsAt),
    composition: asString(rec.composition),
    exclusiveGroup: asString(rec.exclusiveGroup),
    labels: parseLabels(rec.labels),
    scope: parseScope(rec.scope),
    selector: parseSelector(rec.selector),
    outcomes,
    budget: parseBudget(rec.budget),
  }
}

export function parseSessionActor(data: unknown): SessionActorView {
  const rec = asRecord(data)
  const actor = rec ? asRecord(rec.actor) : null
  if (!actor) {
    return { id: '', role: '', scopes: [] }
  }
  return {
    id: asString(actor.id),
    role: asString(actor.role),
    scopes: asStringList(actor.scopes),
  }
}

export function hasScope(actor: SessionActorView, scope: string): boolean {
  if (scope === '') {
    return true
  }
  return actor.scopes.includes(scope) || actor.scopes.includes(SCOPE_ADMIN)
}

export function canActivateHigh(actor: SessionActorView): boolean {
  return hasScope(actor, SCOPE_CHAOS_ACTIVATE) && (hasScope(actor, SCOPE_CHAOS_EMERGENCY) || hasScope(actor, SCOPE_ADMIN))
}

export function scopeGateAllowed(sessionKnown: boolean, has: boolean): boolean {
  return !sessionKnown || has
}

export function activateMissingScope(actor: SessionActorView, safetyClass: string): string {
  if (!hasScope(actor, SCOPE_CHAOS_ACTIVATE)) {
    return SCOPE_CHAOS_ACTIVATE
  }
  if (safetyClass === 'high' && !canActivateHigh(actor)) {
    return SCOPE_CHAOS_EMERGENCY
  }
  return ''
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

export function dash(v: string): string {
  return v === '' ? '—' : v
}

export function joinList(items: string[]): string {
  return items.length === 0 ? '—' : items.join(', ')
}

export function activationLabel(enabled: boolean | undefined): string {
  if (enabled === true) {
    return 'Active'
  }
  if (enabled === false) {
    return 'Inactive'
  }
  return 'Unknown'
}

export function activationSymbol(enabled: boolean | undefined): string {
  if (enabled === true) {
    return '●'
  }
  if (enabled === false) {
    return '○'
  }
  return '○'
}

export type ChaosRuntimeKind = 'emergency' | 'enabled' | 'disabled' | 'unknown'

export function chaosRuntimeKind(st: ChaosStatusView | null): ChaosRuntimeKind {
  if (!st) {
    return 'unknown'
  }
  if (st.emergencyDisabled === true) {
    return 'emergency'
  }
  if (st.enabled === true) {
    return 'enabled'
  }
  if (st.enabled === false) {
    return 'disabled'
  }
  return 'unknown'
}

export function chaosRuntimeSymbol(kind: ChaosRuntimeKind): string {
  switch (kind) {
    case 'emergency':
      return '▲'
    case 'enabled':
      return '●'
    default:
      return '○'
  }
}

export function chaosRuntimeLabel(kind: ChaosRuntimeKind): string {
  switch (kind) {
    case 'emergency':
      return 'Emergency disabled'
    case 'enabled':
      return 'Enabled'
    case 'disabled':
      return 'Disabled'
    default:
      return 'Unknown'
  }
}

export function formatAction(a: ChaosActionView): string {
  const parts = [a.type]
  if (a.phase) {
    parts.push(a.phase)
  }
  if (a.distribution) {
    parts.push(a.distribution)
  }
  if (a.min && a.max) {
    parts.push(`${a.min}–${a.max}`)
  } else if (a.duration) {
    parts.push(a.duration)
  } else if (a.min || a.max) {
    parts.push(a.min || a.max)
  }
  if (a.value) {
    parts.push(a.value)
  }
  if (a.values.length > 0) {
    parts.push(a.values.join(', '))
  }
  if (a.ttl) {
    parts.push(a.ttl)
  }
  if (a.limit !== undefined) {
    parts.push(`limit ${a.limit}`)
  }
  if (a.upstreamId) {
    parts.push(a.upstreamId)
  }
  if (a.hold) {
    parts.push(`hold ${a.hold}`)
  }
  if (a.edeCode !== undefined) {
    parts.push(a.edeText !== '' ? `ede ${a.edeCode} ${a.edeText}` : `ede ${a.edeCode}`)
  } else if (a.edeText !== '') {
    parts.push(`ede ${a.edeText}`)
  }
  return parts.join(' ')
}

export function policyHref(policyId: string): string {
  return `/chaos/${encodeURIComponent(policyId)}`
}

export function useRuntimeRevision(): string {
  const statusQuery = useStatusQuery()
  return statusRevision(statusQuery.data)
}

export function useSessionActorQuery() {
  return useQuery({
    queryKey: queryKeys.session(),
    queryFn: fetchSession,
  })
}

export async function fetchSession(): Promise<unknown> {
  return throwOnError(await client.GET('/v1/session'))
}

export async function fetchChaosPolicies(): Promise<unknown> {
  return throwOnError(await client.GET('/v1/chaos/policies'))
}

export async function fetchChaosPolicy(policyId: string): Promise<unknown> {
  return throwOnError(
    await client.GET('/v1/chaos/policies/{policyId}', { params: { path: { policyId } } }),
  )
}
