import { describe, expect, it } from 'vitest'
import {
  AUDIT_RING_MAX,
  clampAuditLimit,
  eventHref,
  eventMatches,
  parseAuditEvent,
  parseAuditList,
} from './view'

describe('audit view parsers', () => {
  it('reads newest-first list envelopes and skips items without an id', () => {
    const list = parseAuditList({
      events: [
        {
          id: 'aud-2',
          time: '2026-08-19T00:00:01Z',
          actorId: 'loopback',
          actorClass: 'ui-session',
          transport: 'rest',
          capability: 'change.apply',
          result: 'ok',
          reason: 'lab',
        },
        { id: '', capability: 'skip' },
      ],
    })
    expect(list.events).toHaveLength(1)
    expect(list.events[0]?.id).toBe('aud-2')
    expect(list.events[0]?.transport).toBe('rest')
    expect(parseAuditList(null)).toEqual({ events: [] })
    expect(parseAuditEvent({ id: 'aud-1', reason: '[redacted]' })?.reason).toBe('[redacted]')
    expect(eventHref('aud/1')).toBe('/audit/aud%2F1')
    expect(AUDIT_RING_MAX).toBe(128)
  })

  it('clamps limit and filters locally without extra query params', () => {
    expect(clampAuditLimit('0')).toBe(100)
    expect(clampAuditLimit('7')).toBe(7)
    expect(clampAuditLimit('999')).toBe(100)
    const ev = parseAuditEvent({
      id: 'aud-1',
      actorId: 'viewer-1',
      capability: 'change.apply',
      result: 'denied',
    })!
    expect(eventMatches(ev, { capability: 'APPLY', result: 'denied', actorId: 'viewer' })).toBe(true)
    expect(eventMatches(ev, { capability: '', result: 'ok', actorId: '' })).toBe(false)
  })
})
