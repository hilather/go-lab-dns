import { describe, expect, it } from 'vitest'
import { num, parseCacheStatus, yn } from './view'

describe('cache status parser', () => {
  it('reads counters from GET /v1/cache/status', () => {
    expect(
      parseCacheStatus({
        enabled: true,
        maxEntries: 10000,
        entries: 12,
        hits: 40,
        misses: 3,
        evicts: 1,
      }),
    ).toEqual({
      enabled: true,
      maxEntries: 10000,
      entries: 12,
      hits: 40,
      misses: 3,
      evicts: 1,
    })
  })

  it('returns null for non-objects and placeholders when unknown', () => {
    expect(parseCacheStatus(null)).toBeNull()
    expect(parseCacheStatus([])).toBeNull()
    expect(yn(true, false)).toBe('—')
    expect(yn(false, true)).toBe('No')
    expect(num(7, false)).toBe('—')
    expect(num(7, true)).toBe('7')
  })
})
