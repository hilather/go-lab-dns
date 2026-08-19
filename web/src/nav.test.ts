import { describe, expect, it } from 'vitest'
import { NAV } from './nav'

describe('NAV', () => {
  it('exports the Overview row only', () => {
    expect(NAV).toEqual([{ to: '/', label: 'Overview' }])
  })
})
