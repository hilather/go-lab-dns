import { useState } from 'react'
import type { NavigateFunction } from 'react-router'
import { client, throwOnError } from '../../api/client'
import type { Operation } from '../changes/changeIn'
import { queryKeys } from '../../query/keys'
import { statusRevision, useStatusQuery } from '../../query/status'
import { ROUTES } from '../../routes'

// Matches internal/app defaultPageLimit. Zero is not "return everything".
export const DEFAULT_PAGE_LIMIT = 100

export type SOAView = {
  primary: string
  administrator: string
  serial: string
  refresh: string
  retry: string
  expire: string
  minimum: string
}

export type ZoneView = {
  id: string
  name: string
  mode: string
  nameservers: string[]
  soa: SOAView | null
  records: RecordView[]
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

export type ZoneBundle = {
  view: ZoneView
  raw: unknown
}

export type RecordBundle = {
  view: RecordView
  raw: unknown
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

// Reset during render so the same commit queries cursor='' (useEffect would fetch the old offset first).
export function usePagedCursor(scope: string): [string, (next: string) => void] {
  const [cursor, setCursor] = useState('')
  const [scopedTo, setScopedTo] = useState(scope)
  if (scopedTo !== scope) {
    setScopedTo(scope)
    setCursor('')
  }
  const active = scopedTo === scope ? cursor : ''
  return [active, setCursor]
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

export function parseSOA(data: unknown): SOAView | null {
  if (!data || typeof data !== 'object') {
    return null
  }
  const o = data as Record<string, unknown>
  return {
    primary: str(o.primary),
    administrator: str(o.administrator),
    serial: str(o.serial),
    refresh: formatTTL(o.refresh),
    retry: formatTTL(o.retry),
    expire: formatTTL(o.expire),
    minimum: formatTTL(o.minimum),
  }
}

export function formatSOA(soa: SOAView | null): string {
  if (!soa) {
    return '—'
  }
  const parts = [
    soa.primary,
    soa.administrator,
    soa.serial,
    [soa.refresh, soa.retry, soa.expire].filter((p) => p !== '').join('/'),
  ].filter((p) => p !== '')
  return parts.length > 0 ? parts.join(' · ') : '—'
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
  const records = Array.isArray(o.records)
    ? o.records.map(parseRecord).filter((r): r is RecordView => r !== null)
    : []
  return {
    id,
    name: str(o.name),
    mode: str(o.mode),
    nameservers: strList(o.nameservers),
    soa: parseSOA(o.soa),
    records,
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

export async function fetchZoneBundle(zoneId: string): Promise<ZoneBundle> {
  const raw: unknown = throwOnError(
    await client.GET('/v1/zones/{zoneId}', { params: { path: { zoneId } } }),
  )
  const view = parseZone(raw)
  if (!view) {
    throw new Error('zone response missing id')
  }
  return { view, raw }
}

export async function fetchZone(zoneId: string): Promise<ZoneView> {
  return (await fetchZoneBundle(zoneId)).view
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

export async function fetchRecordBundle(zoneId: string, recordId: string): Promise<RecordBundle> {
  const raw: unknown = throwOnError(
    await client.GET('/v1/zones/{zoneId}/records/{recordId}', {
      params: { path: { zoneId, recordId } },
    }),
  )
  const view = parseRecord(raw)
  if (!view) {
    throw new Error('record response missing id')
  }
  return { view, raw }
}

export async function fetchRecord(zoneId: string, recordId: string): Promise<RecordView> {
  return (await fetchRecordBundle(zoneId, recordId)).view
}

export function goToChanges(navigate: NavigateFunction, operations: Operation[], reason?: string) {
  navigate(ROUTES.changes, { state: { operations, reason } })
}

export function createZoneOperation(): Operation {
  return {
    op: 'add',
    target: { kind: 'zone', id: 'new-zone' },
    value: {
      id: 'new-zone',
      name: 'new.example.',
      mode: 'authoritative',
      soa: {
        primary: 'ns1.new.example.',
        administrator: 'hostmaster.new.example.',
        serial: 'auto',
        refresh: '1h',
        retry: '5m',
        expire: '24h',
      },
      records: [],
    },
  }
}

export function editZoneOperation(id: string, raw: unknown): Operation {
  return { op: 'update', target: { kind: 'zone', id }, value: raw }
}

export function deleteZoneOperation(id: string): Operation {
  return { op: 'remove', target: { kind: 'zone', id } }
}

export function createRecordOperation(zoneId: string): Operation {
  return {
    op: 'add',
    target: { kind: 'record', id: 'new-record', zoneId },
    value: { id: 'new-record', owner: 'new', type: 'A', values: ['192.0.2.1'] },
  }
}

export function editRecordOperation(zoneId: string, id: string, raw: unknown): Operation {
  return { op: 'update', target: { kind: 'record', id, zoneId }, value: raw }
}

export function deleteRecordOperation(zoneId: string, id: string): Operation {
  return { op: 'remove', target: { kind: 'record', id, zoneId } }
}
