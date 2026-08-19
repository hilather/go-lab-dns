import { client, throwOnError } from '../../api/client'
import { queryKeys } from '../../query/keys'
import { statusRevision, useStatusQuery } from '../../query/status'

// Matches internal/app defaultPageLimit. Zero is not "return everything".
export const DEFAULT_PAGE_LIMIT = 100

export const MUTATIONS_UI003 = 'mutations in UI-003'

export type ZoneView = {
  id: string
  name: string
  mode: string
  nameservers: string[]
}

export type RecordView = {
  id: string
  owner: string
  type: string
  ttl: string
  values: string[]
  chaosPolicyRefs: string[]
}

export type ZoneListPage = {
  zones: ZoneView[]
  nextCursor: string
}

export type RecordListPage = {
  records: RecordView[]
  nextCursor: string
}

export function zoneHref(zoneId: string): string {
  return `/zones/${encodeURIComponent(zoneId)}`
}

export function recordHref(zoneId: string, recordId: string): string {
  return `/zones/${encodeURIComponent(zoneId)}/records/${encodeURIComponent(recordId)}`
}

export function zonesListKey(revision: string, cursor: string, limit: number) {
  return [...queryKeys.zones(revision), 'page', cursor, limit] as const
}

export function recordsListKey(revision: string, zoneId: string, cursor: string, limit: number) {
  return [...queryKeys.records(revision, zoneId), 'page', cursor, limit] as const
}

export function useRuntimeRevision(): string {
  const statusQuery = useStatusQuery()
  return statusRevision(statusQuery.data)
}

function str(v: unknown): string {
  return typeof v === 'string' ? v : ''
}

function strList(v: unknown): string[] {
  if (!Array.isArray(v)) {
    return []
  }
  return v.filter((item): item is string => typeof item === 'string')
}

function formatTTL(v: unknown): string {
  if (typeof v === 'string') {
    return v
  }
  if (typeof v === 'number' && Number.isFinite(v)) {
    return String(v)
  }
  return ''
}

export function parseZone(data: unknown): ZoneView | null {
  if (!data || typeof data !== 'object') {
    return null
  }
  const o = data as Record<string, unknown>
  const id = str(o.id)
  if (id === '') {
    return null
  }
  return {
    id,
    name: str(o.name),
    mode: str(o.mode),
    nameservers: strList(o.nameservers),
  }
}

export function parseRecord(data: unknown): RecordView | null {
  if (!data || typeof data !== 'object') {
    return null
  }
  const o = data as Record<string, unknown>
  const id = str(o.id)
  if (id === '') {
    return null
  }
  return {
    id,
    owner: str(o.owner),
    type: str(o.type),
    ttl: formatTTL(o.ttl),
    values: strList(o.values),
    chaosPolicyRefs: strList(o.chaosPolicyRefs),
  }
}

export function parseZoneList(data: unknown): ZoneListPage {
  if (!data || typeof data !== 'object') {
    return { zones: [], nextCursor: '' }
  }
  const o = data as Record<string, unknown>
  const zones = Array.isArray(o.zones)
    ? o.zones.map(parseZone).filter((z): z is ZoneView => z !== null)
    : []
  return { zones, nextCursor: str(o.nextCursor) }
}

export function parseRecordList(data: unknown): RecordListPage {
  if (!data || typeof data !== 'object') {
    return { records: [], nextCursor: '' }
  }
  const o = data as Record<string, unknown>
  const records = Array.isArray(o.records)
    ? o.records.map(parseRecord).filter((r): r is RecordView => r !== null)
    : []
  return { records, nextCursor: str(o.nextCursor) }
}

export function listQuery(cursor: string, limit: number): { cursor?: string; limit?: number } {
  const query: { cursor?: string; limit?: number } = { limit }
  if (cursor !== '') {
    query.cursor = cursor
  }
  return query
}

export async function fetchZoneList(cursor: string, limit: number): Promise<ZoneListPage> {
  const data: unknown = throwOnError(
    await client.GET('/v1/zones', { params: { query: listQuery(cursor, limit) } }),
  )
  return parseZoneList(data)
}

export async function fetchZone(zoneId: string): Promise<ZoneView> {
  const data: unknown = throwOnError(
    await client.GET('/v1/zones/{zoneId}', { params: { path: { zoneId } } }),
  )
  const zone = parseZone(data)
  if (!zone) {
    throw new Error('zone response missing id')
  }
  return zone
}

export async function fetchRecordList(
  zoneId: string,
  cursor: string,
  limit: number,
): Promise<RecordListPage> {
  const data: unknown = throwOnError(
    await client.GET('/v1/zones/{zoneId}/records', {
      params: { path: { zoneId }, query: listQuery(cursor, limit) },
    }),
  )
  return parseRecordList(data)
}

export async function fetchRecord(zoneId: string, recordId: string): Promise<RecordView> {
  const data: unknown = throwOnError(
    await client.GET('/v1/zones/{zoneId}/records/{recordId}', {
      params: { path: { zoneId, recordId } },
    }),
  )
  const rec = parseRecord(data)
  if (!rec) {
    throw new Error('record response missing id')
  }
  return rec
}
