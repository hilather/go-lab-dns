import { afterEach, describe, expect, it, vi } from 'vitest'
import { createLabdnsClient } from '../../api/client'
import { CSRF_HEADER } from '../../auth/sessionApi'
import { clear, setCsrf } from '../../auth/sessionMemory'
import {
  actorHasScope,
  answerRows,
  APPLY_CHAOS_DEFAULT,
  buildResolveBody,
  defaultResolveForm,
  DNS_READ_SCOPE,
  explanationFromOut,
  explainRows,
  formatChaosDecision,
  formatRR,
  formatTTL,
  resolveAndExplain,
  resultFromOut,
} from './resolve'

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

function requestOf(call: unknown[] | undefined): Request {
  const first = call?.[0]
  if (first instanceof Request) {
    return first
  }
  return new Request(String(first), (call?.[1] ?? {}) as RequestInit)
}

function pathnameOf(req: Request): string {
  return new URL(req.url, 'http://127.0.0.1:8080').pathname
}

describe('resolve form and body', () => {
  it('defaults applyChaos off to match REST', () => {
    expect(APPLY_CHAOS_DEFAULT).toBe(false)
    const form = defaultResolveForm()
    expect(form.applyChaos).toBe(false)
    expect(form.useCache).toBe(false)
    expect(form.type).toBe('A')
    expect(form.transport).toBe('udp')
    const body = buildResolveBody(form)
    expect(body.options).toEqual({ useCache: false, applyChaos: false })
  })

  it('omits empty client group and sends transport', () => {
    const body = buildResolveBody({
      ...defaultResolveForm(),
      name: 'ns1.lab.example.net.',
      type: 'A',
    })
    expect(body.name).toBe('ns1.lab.example.net.')
    expect(body.type).toBe('A')
    expect(body.clientContext).toEqual({ transport: 'udp' })
  })

  it('includes client group and applyChaos when set', () => {
    const body = buildResolveBody({
      name: '  foo.tools.lab.example.net.  ',
      type: 'AAAA',
      clientGroup: 'test-devices',
      transport: 'tcp',
      useCache: true,
      applyChaos: true,
    })
    expect(body).toEqual({
      name: 'foo.tools.lab.example.net.',
      type: 'AAAA',
      clientContext: { clientGroup: 'test-devices', transport: 'tcp' },
      options: { useCache: true, applyChaos: true },
    })
  })
})

describe('actorHasScope', () => {
  it('allows administrator role and explicit dns.read', () => {
    expect(actorHasScope({ role: 'administrator' }, DNS_READ_SCOPE)).toBe(true)
    expect(actorHasScope({ role: 'viewer', scopes: [] }, DNS_READ_SCOPE)).toBe(true)
    expect(actorHasScope({ scopes: ['dns.read'] }, DNS_READ_SCOPE)).toBe(true)
    expect(actorHasScope({ scopes: ['dns.write'] }, DNS_READ_SCOPE)).toBe(false)
    expect(actorHasScope({ scopes: ['dns.admin'] }, DNS_READ_SCOPE)).toBe(true)
    expect(actorHasScope(undefined, DNS_READ_SCOPE)).toBe(false)
  })
})

describe('result display', () => {
  it('formats Go duration TTL as seconds', () => {
    expect(formatTTL(300_000_000_000)).toBe('300s')
    expect(formatTTL(500)).toBe('500ns')
  })

  it('formats an RR in presentation order', () => {
    expect(
      formatRR({ name: 'ns1.lab.example.net.', ttl: 60_000_000_000, type: 'A', data: '10.42.0.10' }),
    ).toBe('ns1.lab.example.net. 60s IN A 10.42.0.10')
  })

  it('reads result and explanation from REST envelopes', () => {
    const resolveOut = { result: { rcode: 'NOERROR', zoneId: 'lab-zone', source: 'exact' } }
    const explainOut = {
      result: { rcode: 'NOERROR', source: 'wildcard' },
      explanation: {
        zoneId: 'lab-zone',
        zoneMode: 'authoritative',
        wildcardSource: 'tools-wildcard-a',
        forwardingId: 'fwd-1',
        poolId: 'pool-1',
        source: 'exact',
        chaosPolicyIds: ['delay-a'],
        chaosActions: ['delay'],
      },
    }
    expect(resultFromOut(resolveOut)?.rcode).toBe('NOERROR')
    const expl = explanationFromOut(explainOut)
    const rows = explainRows(expl)
    const byLabel = Object.fromEntries(rows.map((r) => [r.label, r.value]))
    expect(byLabel['Matched zone']).toBe('lab-zone / authoritative')
    expect(byLabel['Wildcard source']).toBe('tools-wildcard-a')
    expect(byLabel['Forwarder']).toBe('fwd-1 / pool-1')
    expect(byLabel['Cache']).toBe('no')
    expect(byLabel['Chaos decision']).toBe('policies delay-a; actions delay')
  })

  it('labels cache hits and disabled chaos', () => {
    expect(formatChaosDecision({ chaosDisabled: true, chaosReason: 'emergency' })).toBe(
      'disabled: emergency',
    )
    expect(formatChaosDecision({ chaosDisabled: false })).toBe('none')
    const rows = answerRows({ rcode: 'NOERROR', source: 'cache', fallthrough: false })
    expect(rows.find((r) => r.label === 'Cache')?.value).toBe('hit')
  })
})

describe('resolveAndExplain client', () => {
  afterEach(() => {
    clear()
    vi.restoreAllMocks()
  })

  it('POSTs /v1/resolve and /v1/resolve:explain with applyChaos false and CSRF', async () => {
    setCsrf('csrf-secret')
    const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const req = input instanceof Request ? input : new Request(String(input), init)
      const path = pathnameOf(req)
      if (path === '/v1/resolve') {
        return jsonResponse(200, { result: { rcode: 'NOERROR' } })
      }
      if (path === '/v1/resolve:explain') {
        return jsonResponse(200, {
          result: { rcode: 'NOERROR' },
          explanation: { zoneId: 'lab-zone' },
        })
      }
      return jsonResponse(404, { code: 'not_found', detail: path })
    })
    const api = createLabdnsClient({ fetch: fetchMock })
    const body = buildResolveBody({
      ...defaultResolveForm(),
      name: 'ns1.lab.example.net.',
    })
    const out = await resolveAndExplain(api, body)
    expect(out.answerError).toBeNull()
    expect(out.explainError).toBeNull()
    expect(resultFromOut(out.answer)?.rcode).toBe('NOERROR')
    expect(explanationFromOut(out.explain)?.zoneId).toBe('lab-zone')

    const calls = fetchMock.mock.calls.map((c) => requestOf(c as unknown[]))
    const paths = calls.map(pathnameOf).sort()
    expect(paths).toEqual(['/v1/resolve', '/v1/resolve:explain'])
    for (const req of calls) {
      expect(req.method).toBe('POST')
      expect(req.credentials).toBe('include')
      expect(req.headers.get(CSRF_HEADER)).toBe('csrf-secret')
      const sent = JSON.parse(await req.clone().text()) as { options?: { applyChaos?: boolean } }
      expect(sent.options?.applyChaos).toBe(false)
    }
  })

  it('keeps per-column problem+json when one side fails', async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const req = input instanceof Request ? input : new Request(String(input), init)
      if (pathnameOf(req) === '/v1/resolve') {
        return jsonResponse(200, { result: { rcode: 'NXDOMAIN' } })
      }
      return jsonResponse(403, { code: 'forbidden', detail: 'dns.read required' })
    })
    const api = createLabdnsClient({ fetch: fetchMock })
    const out = await resolveAndExplain(api, buildResolveBody({ ...defaultResolveForm(), name: 'x' }))
    expect(resultFromOut(out.answer)?.rcode).toBe('NXDOMAIN')
    expect(out.explainError).toEqual({ code: 'forbidden', detail: 'dns.read required' })
  })

  it('does not write Web Storage', async () => {
    const setItem = vi.spyOn(Storage.prototype, 'setItem')
    setCsrf('csrf-secret')
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse(200, { result: { rcode: 'NOERROR' } }))
    const api = createLabdnsClient({ fetch: fetchMock })
    await resolveAndExplain(api, buildResolveBody({ ...defaultResolveForm(), name: 'a.example.' }))
    expect(setItem).not.toHaveBeenCalled()
  })
})
