import { describe, expect, it } from 'vitest'
import { NAV, NAV_GROUPS } from './nav'
import { ROUTES } from './routes'

describe('NAV_GROUPS', () => {
  it('groups Inspect, Mutate, and Ref in the approved order', () => {
    expect(NAV_GROUPS.map((g) => g.id)).toEqual(['inspect', 'mutate', 'ref'])
    expect(NAV_GROUPS.map((g) => g.label)).toEqual(['Inspect', 'Mutate', 'Ref'])
    expect(NAV_GROUPS[0]?.items.map((item) => item.label)).toEqual([
      'Overview',
      'Zones',
      'Resolve',
      'Forwarding',
      'Cache',
      'Chaos',
      'Audit',
    ])
    expect(NAV_GROUPS[1]?.items.map((item) => item.label)).toEqual(['Changes', 'State', 'Reset'])
    expect(NAV_GROUPS[2]?.items.map((item) => item.label)).toEqual(['Schema', 'Docs', 'Capabilities'])
    expect(NAV_GROUPS[1]?.items.find((item) => item.label === 'Reset')?.to).toBe(ROUTES.reset)
    expect(NAV_GROUPS[2]?.items.find((item) => item.label === 'Docs')?.to).toBe(ROUTES.docsIndex)
  })

  it('flattens without duplicates and keeps Overview first', () => {
    expect(NAV[0]).toEqual({ to: ROUTES.overview, label: 'Overview' })
    const labels = NAV.map((item) => item.label)
    expect(new Set(labels).size).toBe(labels.length)
    expect(labels).toContain('Reset')
    expect(labels).toContain('Zones')
  })
})
