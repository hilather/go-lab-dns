import { describe, expect, it } from 'vitest'
import { ROUTES, UI_BINDING_ROUTES } from './routes'

describe('ROUTES', () => {
  it('includes every UIBinding.Route', () => {
    const values = new Set<string>(Object.values(ROUTES))
    for (const route of UI_BINDING_ROUTES) {
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
