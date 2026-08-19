import { describe, expect, it } from 'vitest'
import { ROUTES, UI_BINDING_ROUTES } from './routes'

// Unique UIBinding.Route values from internal/capabilities/catalog.go.
const CATALOG_UI_BINDING_ROUTES = [
  '/',
  '/capabilities',
  '/schema',
  '/state',
  '/changes',
  '/reset',
  '/zones',
  '/zones/:zoneId',
  '/resolve',
  '/forwarding',
  '/cache',
  '/chaos',
  '/chaos/:policyId',
  '/audit',
  '/audit/:eventId',
  '/docs/:id',
] as const

describe('ROUTES', () => {
  it('matches every unique UIBinding.Route from the catalog', () => {
    expect([...UI_BINDING_ROUTES]).toEqual([...CATALOG_UI_BINDING_ROUTES])
    const values = new Set<string>(Object.values(ROUTES))
    for (const route of CATALOG_UI_BINDING_ROUTES) {
      expect(values.has(route)).toBe(true)
    }
  })

  it('includes login, record detail, and docs index paths', () => {
    expect(ROUTES.login).toBe('/login')
    expect(ROUTES.record).toBe('/zones/:zoneId/records/:recordId')
    expect(ROUTES.docsIndex).toBe('/docs')
    expect(ROUTES.docs).toBe('/docs/:id')
  })
})
