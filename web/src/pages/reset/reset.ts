import type { SessionActor } from '../../auth/sessionApi'

export const RESET_CONFIRM_EMPTY = 'RESET'

export type ResetResultView = {
  applied?: boolean
  previousRevision?: string
  candidateRevision?: string
  drifted?: boolean
  generation?: number
  auditEventId?: string
}

export function compiledMetadataName(state: unknown): string {
  if (!state || typeof state !== 'object') {
    return ''
  }
  const rec = state as Record<string, unknown>
  const canonical = rec.canonical && typeof rec.canonical === 'object' ? rec.canonical : rec
  if (!canonical || typeof canonical !== 'object' || Array.isArray(canonical)) {
    return ''
  }
  const metadata = (canonical as { metadata?: unknown }).metadata
  if (!metadata || typeof metadata !== 'object' || Array.isArray(metadata)) {
    return ''
  }
  const name = (metadata as { name?: unknown }).name
  return typeof name === 'string' ? name : ''
}

export function resetConfirmationPhrase(name: string): string {
  return name === '' ? RESET_CONFIRM_EMPTY : name
}

export function confirmationMatches(typed: string, expected: string): boolean {
  return typed.trim() === expected
}

export function hasAdminScope(actor: SessionActor | null): boolean {
  if (!actor) {
    return false
  }
  const scopes = actor.scopes ?? []
  if (scopes.includes('dns.admin')) {
    return true
  }
  return actor.role === 'administrator'
}

export function parseResetResult(data: unknown): ResetResultView {
  if (!data || typeof data !== 'object') {
    return {}
  }
  const o = data as Record<string, unknown>
  return {
    applied: typeof o.applied === 'boolean' ? o.applied : undefined,
    previousRevision: typeof o.previousRevision === 'string' ? o.previousRevision : undefined,
    candidateRevision: typeof o.candidateRevision === 'string' ? o.candidateRevision : undefined,
    drifted: typeof o.drifted === 'boolean' ? o.drifted : undefined,
    generation: typeof o.generation === 'number' ? o.generation : undefined,
    auditEventId: typeof o.auditEventId === 'string' ? o.auditEventId : undefined,
  }
}
