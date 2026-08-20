import { client, throwOnError } from '../../api/client'

export const AUDIT_RING_MAX = 128
export const DEFAULT_AUDIT_LIMIT = 100
export const SCOPE_AUDIT_READ = 'dns.audit.read'

export type AuditEventView = {
  id: string
  time: string
  actorId: string
  actorClass: string
  transport: string
  capability: string
  reason: string
  ticket: string
  revision: string
  previous: string
  result: string
  errorCode: string
}

export type AuditListView = {
  events: AuditEventView[]
}

export type AuditFilters = {
  capability: string
  result: string
  actorId: string
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

export function parseAuditEvent(data: unknown): AuditEventView | null {
  const rec = asRecord(data)
  if (!rec) {
    return null
  }
  const id = asString(rec.id)
  if (id === '') {
    return null
  }
  return {
    id,
    time: asString(rec.time),
    actorId: asString(rec.actorId),
    actorClass: asString(rec.actorClass),
    transport: asString(rec.transport),
    capability: asString(rec.capability),
    reason: asString(rec.reason),
    ticket: asString(rec.ticket),
    revision: asString(rec.revision),
    previous: asString(rec.previous),
    result: asString(rec.result),
    errorCode: asString(rec.errorCode),
  }
}

export function parseAuditList(data: unknown): AuditListView {
  const rec = asRecord(data)
  if (!rec || !Array.isArray(rec.events)) {
    return { events: [] }
  }
  const events: AuditEventView[] = []
  for (const item of rec.events) {
    const ev = parseAuditEvent(item)
    if (ev) {
      events.push(ev)
    }
  }
  return { events }
}

export function clampAuditLimit(raw: string): number {
  const n = Number(raw)
  if (!Number.isInteger(n) || n < 1) {
    return DEFAULT_AUDIT_LIMIT
  }
  if (n > 100) {
    return 100
  }
  return n
}

export function eventMatches(ev: AuditEventView, filters: AuditFilters): boolean {
  if (filters.result !== '' && ev.result !== filters.result) {
    return false
  }
  if (filters.capability !== '' && !ev.capability.toLowerCase().includes(filters.capability.toLowerCase())) {
    return false
  }
  if (filters.actorId !== '' && !ev.actorId.toLowerCase().includes(filters.actorId.toLowerCase())) {
    return false
  }
  return true
}

export function dash(v: string): string {
  return v === '' ? '—' : v
}

export function eventHref(eventId: string): string {
  return `/audit/${encodeURIComponent(eventId)}`
}

export async function fetchAuditList(limit: number): Promise<unknown> {
  return throwOnError(await client.GET('/v1/audit', { params: { query: { limit } } }))
}

export async function fetchAuditEvent(eventId: string): Promise<unknown> {
  return throwOnError(await client.GET('/v1/audit/{eventId}', { params: { path: { eventId } } }))
}
