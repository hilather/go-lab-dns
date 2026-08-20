import type { components } from '../../api/openapi'
import { parseYamlOrJson } from './parseDocument'

export const OP_KINDS = ['add', 'update', 'remove'] as const
export type OpKind = (typeof OP_KINDS)[number]

export const TARGET_KINDS = [
  'zone',
  'record',
  'forwardingPolicy',
  'upstreamPool',
  'upstream',
  'clientGroup',
  'chaosPolicy',
  'chaosSafety',
  'cache',
  'defaults',
  'listeners',
  'access',
  'observability',
  'management',
  'ui',
  'chaosActivation',
] as const

export type Operation = {
  op: OpKind
  target: {
    kind: string
    id?: string
    zoneId?: string
  }
  value?: unknown
}

export type BuilderRow = {
  key: string
  op: OpKind
  target: {
    kind: string
    id?: string
    zoneId?: string
  }
  valueText: string
}

let builderRowSeq = 0

export function newBuilderRowKey(): string {
  builderRowSeq += 1
  return `row-${builderRowSeq}`
}

export function operationToRow(op: Operation, key = newBuilderRowKey()): BuilderRow {
  let valueText = ''
  if (op.value !== undefined) {
    try {
      valueText = JSON.stringify(op.value, null, 2)
    } catch {
      valueText = ''
    }
  }
  return { key, op: op.op, target: { ...op.target }, valueText }
}

export function emptyBuilderRow(): BuilderRow {
  return {
    key: newBuilderRowKey(),
    op: 'add',
    target: { kind: 'record' },
    valueText: '{}',
  }
}

export function builderValueError(row: BuilderRow): string {
  if (row.op === 'remove') {
    return ''
  }
  const trimmed = row.valueText.trim()
  if (trimmed === '') {
    return ''
  }
  try {
    JSON.parse(trimmed)
    return ''
  } catch {
    return 'Invalid JSON value'
  }
}

export function compileBuilderRows(rows: BuilderRow[]): { operations: Operation[]; error: string } {
  const operations: Operation[] = []
  for (const row of rows) {
    const err = builderValueError(row)
    if (err !== '') {
      return { operations: [], error: err }
    }
    const op: Operation = { op: row.op, target: { ...row.target } }
    const trimmed = row.valueText.trim()
    if (row.op !== 'remove' && trimmed !== '') {
      op.value = JSON.parse(trimmed) as unknown
    }
    operations.push(op)
  }
  return { operations, error: '' }
}

export type ChangeInBody = {
  expectedRevision?: string
  idempotencyKey?: string
  reason?: string
  ticket?: string
  operations?: Operation[]
  state?: unknown
}

export type PlanView = {
  previousRevision?: string
  candidateRevision?: string
  drifted?: boolean
  diff?: { path?: string; op?: string; before?: unknown; after?: unknown }[]
  impact?: {
    names?: string[]
    zones?: string[]
    wildcardCoverage?: boolean
    authoritativeMisses?: boolean
    clientGroups?: string[]
    forwardingChanged?: boolean
    chaosPolicies?: { id?: string; enabled?: boolean; expiresAt?: string }[]
    compatibilityWarnings?: string[]
    requiredPermissions?: string[]
    suggestedProbes?: string[]
  }
  warnings?: { code?: string; message?: string }[]
  operations?: Operation[]
  auth?: { allowed?: boolean; scopes?: string[] }
  applied?: boolean
  generation?: number
  auditEventId?: string
}

export type ProblemView = {
  code: string
  detail: string
  currentRevision: string
  expectedRevision: string
  status: number
}

export type PlannedChange = {
  revision: string
  fingerprint: string
  body: PlanView
}

export function newIdempotencyKey(): string {
  return crypto.randomUUID()
}

export function isOpKind(v: unknown): v is OpKind {
  return v === 'add' || v === 'update' || v === 'remove'
}

export function parseOperation(v: unknown): Operation | null {
  if (!v || typeof v !== 'object' || Array.isArray(v)) {
    return null
  }
  const o = v as Record<string, unknown>
  if (!isOpKind(o.op)) {
    return null
  }
  const target = o.target
  if (!target || typeof target !== 'object' || Array.isArray(target)) {
    return null
  }
  const t = target as Record<string, unknown>
  if (typeof t.kind !== 'string' || t.kind === '') {
    return null
  }
  const op: Operation = {
    op: o.op,
    target: { kind: t.kind },
  }
  if (typeof t.id === 'string' && t.id !== '') {
    op.target.id = t.id
  }
  if (typeof t.zoneId === 'string' && t.zoneId !== '') {
    op.target.zoneId = t.zoneId
  }
  if ('value' in o) {
    op.value = o.value
  }
  return op
}

export function parseOperations(v: unknown): Operation[] | null {
  if (!Array.isArray(v)) {
    return null
  }
  const ops: Operation[] = []
  for (const item of v) {
    const op = parseOperation(item)
    if (!op) {
      return null
    }
    ops.push(op)
  }
  return ops
}

function isLabdnsDocument(o: Record<string, unknown>): boolean {
  return (
    typeof o.apiVersion === 'string' ||
    o.kind === 'LabDNS' ||
    (o.spec !== undefined && typeof o.spec === 'object' && o.spec !== null)
  )
}

function isEnvelope(o: Record<string, unknown>): boolean {
  return (
    Array.isArray(o.operations) ||
    o.state !== undefined ||
    typeof o.expectedRevision === 'string' ||
    typeof o.idempotencyKey === 'string' ||
    typeof o.reason === 'string' ||
    typeof o.ticket === 'string' ||
    typeof o.mode === 'string'
  )
}

export function compileChangeIn(
  parsed: unknown,
  opts: {
    expectedRevision: string
    reason?: string
    ticket?: string
    idempotencyKey?: string
    operations?: Operation[]
  },
): ChangeInBody {
  const body: ChangeInBody = {}
  if (opts.expectedRevision !== '') {
    body.expectedRevision = opts.expectedRevision
  }
  if (opts.reason !== undefined && opts.reason !== '') {
    body.reason = opts.reason
  }
  if (opts.ticket !== undefined && opts.ticket !== '') {
    body.ticket = opts.ticket
  }
  if (opts.idempotencyKey !== undefined && opts.idempotencyKey !== '') {
    body.idempotencyKey = opts.idempotencyKey
  }

  if (opts.operations && opts.operations.length > 0) {
    body.operations = opts.operations
  }

  if (parsed === undefined) {
    return body
  }

  const asOp = parseOperation(parsed)
  if (asOp) {
    body.operations = [asOp]
    return body
  }

  const asOps = parseOperations(parsed)
  if (asOps) {
    body.operations = asOps
    return body
  }

  if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
    const o = parsed as Record<string, unknown>
    if (isEnvelope(o) && !isLabdnsDocument(o)) {
      const envOps = parseOperations(o.operations)
      if (envOps) {
        body.operations = envOps
      }
      if (o.state !== undefined) {
        body.state = o.state
      }
      if (!body.reason && typeof o.reason === 'string' && o.reason !== '') {
        body.reason = o.reason
      }
      if (!body.ticket && typeof o.ticket === 'string' && o.ticket !== '') {
        body.ticket = o.ticket
      }
      return body
    }
    body.state = parsed
  }

  return body
}

export function parseEditorDocument(text: string): unknown {
  return parseYamlOrJson(text)
}

export function changeFingerprint(body: ChangeInBody): string {
  return JSON.stringify({
    operations: body.operations ?? [],
    state: body.state ?? null,
  })
}

export function hasPlanSource(body: ChangeInBody): boolean {
  return (body.operations !== undefined && body.operations.length > 0) || body.state !== undefined
}

export function hasOperations(body: ChangeInBody): boolean {
  return body.operations !== undefined && body.operations.length > 0
}

export function isPlanCurrent(
  plan: PlannedChange | null,
  revision: string,
  fingerprint: string,
): boolean {
  return (
    plan !== null &&
    revision !== '' &&
    fingerprint !== '' &&
    plan.revision === revision &&
    plan.fingerprint === fingerprint
  )
}

export function asChangeInSchema(body: ChangeInBody): components['schemas']['ChangeIn'] {
  return body as components['schemas']['ChangeIn']
}

export function asValidateInSchema(body: ChangeInBody): components['schemas']['ValidateIn'] {
  return body as components['schemas']['ValidateIn']
}

export function parsePlanView(data: unknown): PlanView {
  if (!data || typeof data !== 'object') {
    return {}
  }
  return data as PlanView
}

export function parseProblem(error: unknown, status: number): ProblemView {
  const empty: ProblemView = { code: '', detail: '', currentRevision: '', expectedRevision: '', status }
  if (!error || typeof error !== 'object') {
    return empty
  }
  const o = error as Record<string, unknown>
  return {
    code: typeof o.code === 'string' ? o.code : '',
    detail: typeof o.detail === 'string' ? o.detail : typeof o.message === 'string' ? o.message : '',
    currentRevision: typeof o.currentRevision === 'string' ? o.currentRevision : '',
    expectedRevision: typeof o.expectedRevision === 'string' ? o.expectedRevision : '',
    status,
  }
}

export function hasWriteScope(actor: { role?: string; scopes?: string[]; class?: string } | null): boolean {
  if (!actor) {
    return false
  }
  const scopes = actor.scopes ?? []
  if (scopes.includes('dns.write') || scopes.includes('dns.admin')) {
    return true
  }
  if (actor.role === 'administrator') {
    return true
  }
  return false
}
