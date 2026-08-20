import { describe, expect, it } from 'vitest'
import {
  compiledMetadataName,
  confirmationMatches,
  hasAdminScope,
  parseResetResult,
  RESET_CONFIRM_EMPTY,
  resetConfirmationPhrase,
} from './reset'

describe('reset confirmation phrase', () => {
  it('uses compiled metadata.name from GET /v1/state', () => {
    expect(
      compiledMetadataName({
        runtimeRevision: 'sha256:aaa',
        canonical: { metadata: { name: 'lab-dns' } },
      }),
    ).toBe('lab-dns')
    expect(resetConfirmationPhrase('lab-dns')).toBe('lab-dns')
    expect(confirmationMatches('lab-dns', 'lab-dns')).toBe(true)
    expect(confirmationMatches(' lab-dns ', 'lab-dns')).toBe(true)
    expect(confirmationMatches('Lab-dns', 'lab-dns')).toBe(false)
  })

  it('uses literal RESET when the compiled name is empty', () => {
    expect(compiledMetadataName({ canonical: { metadata: { name: '' } } })).toBe('')
    expect(compiledMetadataName({ canonical: { metadata: {} } })).toBe('')
    expect(compiledMetadataName({ canonical: {} })).toBe('')
    expect(compiledMetadataName(null)).toBe('')
    expect(resetConfirmationPhrase('')).toBe(RESET_CONFIRM_EMPTY)
    expect(RESET_CONFIRM_EMPTY).toBe('RESET')
    expect(confirmationMatches('RESET', RESET_CONFIRM_EMPTY)).toBe(true)
    expect(confirmationMatches('reset', RESET_CONFIRM_EMPTY)).toBe(false)
  })
})

describe('hasAdminScope', () => {
  it('requires dns.admin or administrator role', () => {
    expect(hasAdminScope({ id: 'a', class: 'ui-session', role: 'administrator', scopes: [] })).toBe(true)
    expect(hasAdminScope({ id: 'a', class: 'ui-session', scopes: ['dns.admin'] })).toBe(true)
    expect(hasAdminScope({ id: 'a', class: 'ui-session', role: 'dns-editor', scopes: ['dns.write'] })).toBe(false)
    expect(hasAdminScope({ id: 'v', class: 'ui-session', role: 'viewer', scopes: ['dns.read'] })).toBe(false)
    expect(hasAdminScope(null)).toBe(false)
  })
})

describe('parseResetResult', () => {
  it('reads ApplyResult fields', () => {
    expect(
      parseResetResult({
        applied: true,
        previousRevision: 'sha256:old',
        candidateRevision: 'sha256:new',
        generation: 4,
        drifted: false,
        auditEventId: 'evt-9',
      }),
    ).toEqual({
      applied: true,
      previousRevision: 'sha256:old',
      candidateRevision: 'sha256:new',
      generation: 4,
      drifted: false,
      auditEventId: 'evt-9',
    })
    expect(parseResetResult(null)).toEqual({})
  })
})
