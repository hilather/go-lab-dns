import { describe, expect, it } from 'vitest'
import {
  formatFailover,
  healthKind,
  healthLabel,
  healthSymbol,
  parsePolicies,
  parsePools,
  parseUpstreamsStatus,
} from './view'

describe('forwarding view parsers', () => {
  it('parses policies, pools, and live upstream health', () => {
    const policies = parsePolicies({
      policies: [
        {
          id: 'corp-policy',
          suffix: 'corp.example.net.',
          upstreamPool: 'corporate',
          failover: {
            timeout: '1s',
            onTimeout: true,
            onSERVFAIL: true,
            onREFUSED: false,
          },
        },
        { id: 'default-policy', suffix: '.', upstreamPool: 'default' },
        { not: 'a-policy' },
      ],
    })
    expect(policies).toEqual([
      {
        id: 'corp-policy',
        suffix: 'corp.example.net.',
        upstreamPool: 'corporate',
        failover: {
          timeout: '1s',
          onTimeout: true,
          onTransportError: undefined,
          onSERVFAIL: true,
          onREFUSED: false,
          udpTruncateRetryTCP: undefined,
        },
      },
      {
        id: 'default-policy',
        suffix: '.',
        upstreamPool: 'default',
        failover: {},
      },
    ])

    const pools = parsePools({
      pools: [
        {
          id: 'corporate',
          strategy: 'ordered',
          upstreams: [{ id: 'corp-1', endpoint: '10.0.0.53:53', transport: 'udp' }],
        },
      ],
    })
    expect(pools[0]?.id).toBe('corporate')
    expect(pools[0]?.upstreams[0]?.endpoint).toBe('10.0.0.53:53')

    const ups = parseUpstreamsStatus({
      upstreams: [
        {
          id: 'corp-1',
          poolId: 'corporate',
          endpoint: '10.0.0.53:53',
          transport: 'udp',
          healthy: true,
        },
        { id: 'default-2', poolId: 'default', healthy: false },
      ],
    })
    expect(ups.map((u) => u.healthy)).toEqual([true, false])
  })

  it('returns empty lists for missing or invalid payloads', () => {
    expect(parsePolicies(null)).toEqual([])
    expect(parsePolicies({})).toEqual([])
    expect(parsePools({ pools: 'nope' })).toEqual([])
    expect(parseUpstreamsStatus({ upstreams: [{}] })).toEqual([])
  })
})

describe('failover and health labels', () => {
  it('summarizes failover flags without color-only status', () => {
    expect(formatFailover({})).toBe('none')
    expect(formatFailover({ timeout: '1s', onTimeout: true, onSERVFAIL: true })).toBe(
      'timeout 1s; on timeout; on SERVFAIL',
    )
    expect(healthKind(true)).toBe('healthy')
    expect(healthKind(false)).toBe('unhealthy')
    expect(healthKind(undefined)).toBe('unknown')
    expect(healthSymbol('healthy')).toBe('●')
    expect(healthLabel('unhealthy')).toBe('Unhealthy')
  })
})
