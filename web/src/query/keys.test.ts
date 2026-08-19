import { describe, expect, it } from 'vitest'
import { createQueryClient } from './client'
import {
  invalidateOnRevisionChange,
  isSnapshotQueryKey,
  queryKeys,
  shouldInvalidateOnRevision,
} from './keys'
import { LIVE_POLL_MS } from './live'
import { pollInterval, STATUS_POLL_MS, statusRevision } from './status'

describe('query keys', () => {
  it('include revision on snapshot-backed resources', () => {
    const rev = 'sha256:abc'
    expect(queryKeys.state(rev)).toEqual(['labdns', 'state', rev])
    expect(queryKeys.zones(rev)).toEqual(['labdns', 'zones', rev])
    expect(queryKeys.records(rev, 'zone-1')).toEqual(['labdns', 'records', rev, 'zone-1'])
    expect(queryKeys.chaosPolicies(rev)).toEqual(['labdns', 'chaosPolicies', rev])
    expect(queryKeys.forwarding(rev)).toEqual(['labdns', 'forwarding', rev])
    expect(queryKeys.schema(rev)).toEqual(['labdns', 'schema', rev])
    expect(queryKeys.capabilities(rev)).toEqual(['labdns', 'capabilities', rev])
    expect(isSnapshotQueryKey(queryKeys.state(rev))).toBe(true)
  })

  it('does not include revision on live status keys', () => {
    expect(queryKeys.liveUpstreams()).toEqual(['labdns', 'live', 'upstreams'])
    expect(queryKeys.liveCache()).toEqual(['labdns', 'live', 'cache'])
    expect(queryKeys.liveChaos()).toEqual(['labdns', 'live', 'chaos'])
    expect(isSnapshotQueryKey(queryKeys.liveUpstreams())).toBe(false)
    expect(isSnapshotQueryKey(queryKeys.status())).toBe(false)
  })
})

describe('revision invalidation', () => {
  it('invalidates snapshot keys and leaves live keys', async () => {
    const qc = createQueryClient()
    qc.setQueryData(queryKeys.state('r1'), { n: 1 })
    qc.setQueryData(queryKeys.zones('r1'), { n: 2 })
    qc.setQueryData(queryKeys.liveCache(), { n: 3 })
    qc.setQueryData(queryKeys.status(), { revisions: { runtimeRevision: 'r1' } })

    expect(shouldInvalidateOnRevision('r1', 'r2')).toBe(true)
    await invalidateOnRevisionChange(qc, 'r1', 'r2')

    expect(qc.getQueryState(queryKeys.state('r1'))?.isInvalidated).toBe(true)
    expect(qc.getQueryState(queryKeys.zones('r1'))?.isInvalidated).toBe(true)
    expect(qc.getQueryState(queryKeys.liveCache())?.isInvalidated).toBe(false)
    expect(qc.getQueryState(queryKeys.status())?.isInvalidated).toBe(false)
  })

  it('does not invalidate when revision is unchanged or empty', async () => {
    const qc = createQueryClient()
    qc.setQueryData(queryKeys.state('r1'), { n: 1 })
    await invalidateOnRevisionChange(qc, 'r1', 'r1')
    await invalidateOnRevisionChange(qc, '', 'r2')
    expect(qc.getQueryState(queryKeys.state('r1'))?.isInvalidated).toBe(false)
  })
})

describe('poll intervals', () => {
  it('pauses status and live polls when the document is hidden', () => {
    expect(STATUS_POLL_MS).toBe(2000)
    expect(LIVE_POLL_MS).toBe(5000)
    expect(pollInterval(true, STATUS_POLL_MS)).toBe(2000)
    expect(pollInterval(false, STATUS_POLL_MS)).toBe(false)
    expect(pollInterval(true, LIVE_POLL_MS)).toBe(5000)
    expect(pollInterval(false, LIVE_POLL_MS)).toBe(false)
  })

  it('reads runtimeRevision from status payloads', () => {
    expect(statusRevision({ revisions: { runtimeRevision: 'sha256:ff' } })).toBe('sha256:ff')
    expect(statusRevision({})).toBe('')
    expect(statusRevision(null)).toBe('')
  })
})
