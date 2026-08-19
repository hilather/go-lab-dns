import { describe, expect, it } from 'vitest'
import { NAV } from './nav'
import { ROUTES } from './routes'

describe('NAV', () => {
  it('keeps Overview first and appends the remaining shell entries', () => {
    expect(NAV[0]).toEqual({ to: ROUTES.overview, label: 'Overview' })
    expect(NAV.map((item) => item.label)).toEqual([
      'Overview',
      'State',
      'Changes',
      'Zones',
      'Resolve',
      'Forwarding',
      'Cache',
      'Chaos',
      'Audit',
      'Schema',
      'Docs',
      'Capabilities',
    ])
    expect(NAV.find((item) => item.label === 'Docs')?.to).toBe(ROUTES.docsIndex)
  })
})
