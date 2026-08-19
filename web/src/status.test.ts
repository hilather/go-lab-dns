import { describe, expect, it } from 'vitest'
import { readyKind, readyLabel, shortRevision } from './status'

describe('shortRevision', () => {
  it('shortens a sha256 revision to 12 hex characters', () => {
    expect(shortRevision('sha256:abcdef0123456789ffff')).toBe('abcdef012345')
  })

  it('returns unknown when empty', () => {
    expect(shortRevision(undefined)).toBe('unknown')
    expect(shortRevision('')).toBe('unknown')
  })
})

describe('readyKind', () => {
  it('labels ready, degraded, and not-ready with text', () => {
    expect(readyLabel(readyKind(true, false))).toBe('Ready')
    expect(readyLabel(readyKind(true, true))).toBe('Degraded')
    expect(readyLabel(readyKind(false, false))).toBe('Not ready')
  })
})
