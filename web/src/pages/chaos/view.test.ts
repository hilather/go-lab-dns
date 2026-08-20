import { describe, expect, it } from 'vitest'
import {
  activateMissingScope,
  canActivateHigh,
  chaosRuntimeKind,
  formatAction,
  hasScope,
  parseChaosPolicies,
  parseChaosPolicy,
  parseChaosStatus,
  parseSessionActor,
  policyHref,
  SCOPE_CHAOS_ACTIVATE,
  SCOPE_CHAOS_EMERGENCY,
  scopeGateAllowed,
} from './view'

describe('chaos view parsers', () => {
  it('parses live status and policy list envelopes', () => {
    expect(
      parseChaosStatus({
        enabled: true,
        emergencyDisabled: false,
        activePolicies: 1,
        nearestExpiry: '2026-09-01T00:00:00Z',
      }),
    ).toEqual({
      enabled: true,
      emergencyDisabled: false,
      activePolicies: 1,
      nearestExpiry: '2026-09-01T00:00:00Z',
    })
    expect(parseChaosStatus(null)).toBeNull()
    expect(chaosRuntimeKind({ enabled: true, emergencyDisabled: true, nearestExpiry: '' })).toBe('emergency')

    const policies = parseChaosPolicies({
      policies: [
        { id: 'slow-tools', owner: 'platform-lab', enabled: false, safetyClass: 'low' },
        { id: '', owner: 'skip' },
      ],
    })
    expect(policies).toEqual([
      { id: 'slow-tools', owner: 'platform-lab', enabled: false, safetyClass: 'low', expiresAt: '' },
    ])
    expect(parseChaosPolicies(null)).toEqual([])
  })

  it('parses policy detail scope, selector, and outcomes', () => {
    const policy = parseChaosPolicy({
      id: 'slow-tools',
      owner: 'platform-lab',
      reason: 'Test application startup timeouts',
      enabled: false,
      safetyClass: 'low',
      scope: { recordIds: ['tools-wildcard-a'], clientGroups: ['test-devices'] },
      selector: { mode: 'deterministic', seed: 'startup-v1', probability: 1 },
      outcomes: [
        {
          id: 'delayed',
          weight: 100,
          actions: [{ type: 'delay', phase: 'before-response', distribution: 'uniform', min: '100ms', max: '750ms' }],
        },
      ],
    })
    expect(policy?.id).toBe('slow-tools')
    expect(policy?.scope.recordIds).toEqual(['tools-wildcard-a'])
    expect(policy?.selector.mode).toBe('deterministic')
    expect(policy?.outcomes[0]?.actions[0]?.type).toBe('delay')
    expect(formatAction(policy!.outcomes[0]!.actions[0]!)).toBe('delay before-response uniform 100ms–750ms')
    expect(policyHref('slow tools')).toBe('/chaos/slow%20tools')

    const alt = parseChaosPolicy({
      id: 'spoof-a',
      owner: 'platform-lab',
      reason: 'swap A',
      enabled: false,
      safetyClass: 'medium',
      outcomes: [
        {
          id: 'swap',
          weight: 1,
          actions: [
            {
              type: 'alternate',
              values: ['10.42.0.30', '10.42.0.31'],
              ttl: '5s',
              limit: 2,
              upstreamId: 'corp-1',
              hold: '1s',
              ede: { code: 21, text: 'blocked' },
            },
          ],
        },
      ],
    })
    const formatted = formatAction(alt!.outcomes[0]!.actions[0]!)
    expect(formatted).toContain('10.42.0.30')
    expect(formatted).toContain('10.42.0.31')
    expect(formatted).toContain('5s')
    expect(formatted).toContain('limit 2')
    expect(formatted).toContain('corp-1')
    expect(formatted).toContain('hold 1s')
    expect(formatted).toContain('ede 21 blocked')
  })

  it('gates high-impact activation using session EffectiveScopes', () => {
    const viewer = parseSessionActor({
      actor: { id: 'v', role: 'viewer', scopes: ['dns.read', 'dns.chaos.read'] },
    })
    const operator = parseSessionActor({
      actor: { id: 'op', role: 'chaos-operator', scopes: ['dns.read', 'dns.chaos.read', 'dns.chaos.activate'] },
    })
    const admin = parseSessionActor({
      actor: { id: 'a', role: 'administrator', scopes: ['dns.admin'] },
    })
    expect(hasScope(viewer, SCOPE_CHAOS_ACTIVATE)).toBe(false)
    expect(activateMissingScope(viewer, 'low')).toBe(SCOPE_CHAOS_ACTIVATE)
    expect(activateMissingScope(operator, 'high')).toBe(SCOPE_CHAOS_EMERGENCY)
    expect(canActivateHigh(admin)).toBe(true)
    expect(activateMissingScope(admin, 'high')).toBe('')
    expect(scopeGateAllowed(false, false)).toBe(true)
    expect(scopeGateAllowed(true, false)).toBe(false)
    expect(scopeGateAllowed(true, true)).toBe(true)
  })
})
