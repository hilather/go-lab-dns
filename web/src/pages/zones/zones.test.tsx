import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, type ReactNode } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { MemoryRouter, Route, Routes, useLocation } from 'react-router'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { queryKeys } from '../../query/keys'
import { RecordDetailPage } from './RecordDetailPage'
import { ZoneDetailPage } from './ZoneDetailPage'
import { ZonesPage } from './ZonesPage'
import {
  DEFAULT_PAGE_LIMIT,
  createRecordOperation,
  createZoneOperation,
  formatSOA,
  parseRecordList,
  parseZoneList,
  recordsListKey,
  zonesListKey,
} from './zones'

const fetchMock = vi.hoisted(() => vi.fn())

vi.mock('../../api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../../api/client')>()
  return {
    ...actual,
    client: actual.createLabdnsClient({
      fetch: ((input: RequestInfo | URL, init?: RequestInit) => fetchMock(input, init)) as typeof fetch,
    }),
  }
})

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

const REV = 'sha256:abc'

const writerSession = {
  csrf: 'csrf',
  actor: {
    id: 'admin',
    class: 'ui-session',
    role: 'administrator',
    scopes: ['dns.admin', 'dns.write', 'dns.read'],
  },
}

const viewerSession = {
  csrf: 'csrf',
  actor: {
    id: 'viewer',
    class: 'ui-session',
    role: 'viewer',
    scopes: ['dns.read'],
  },
}

const genericRdata = { typeCode: 99, presentation: '\\# 4 0a2a0014' }

const labZoneRaw = {
  id: 'lab-zone',
  name: 'lab.example.net.',
  mode: 'authoritative',
  nameservers: ['ns1.lab.example.net.'],
  soa: {
    primary: 'ns1.lab.example.net.',
    administrator: 'hostmaster.lab.example.net.',
    serial: 'auto',
    refresh: '1h',
    retry: '5m',
    expire: '24h',
  },
  records: [
    { id: 'ns1-a', owner: 'ns1', type: 'A', ttl: '30s', values: ['10.42.0.53'] },
    {
      id: 'generic-1',
      owner: 'odd',
      type: 'TYPE99',
      ttl: '30s',
      genericRdata,
    },
  ],
}

const vendorZoneRaw = {
  id: 'vendor-overlay',
  name: 'vendor.example.',
  mode: 'overlay',
  records: [{ id: 'vendor-special-a', owner: 'special-api', type: 'A', values: ['10.42.0.30'] }],
}

const ns1RecordRaw = {
  id: 'ns1-a',
  owner: 'ns1',
  type: 'A',
  ttl: '30s',
  values: ['10.42.0.53'],
  chaosPolicyRefs: ['slow-tools'],
  genericRdata,
}

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': status >= 400 ? 'application/problem+json' : 'application/json' },
  })
}

function asURL(input: RequestInfo | URL): URL {
  if (input instanceof URL) {
    return input
  }
  if (input instanceof Request) {
    return new URL(input.url)
  }
  return new URL(String(input), 'http://localhost:3000')
}

function requested(): URL[] {
  return fetchMock.mock.calls.map((c) => asURL(c[0] as RequestInfo | URL))
}

function mockAPI(handler: (url: URL) => Response) {
  fetchMock.mockImplementation(async (input: RequestInfo | URL) => handler(asURL(input)))
}

function testQueryClient(): QueryClient {
  return new QueryClient({
    defaultOptions: {
      queries: { retry: false, refetchOnWindowFocus: false },
    },
  })
}

function ChangesProbe() {
  const loc = useLocation()
  return <pre data-testid="changes-state">{JSON.stringify(loc.state)}</pre>
}

let root: Root | undefined
let host: HTMLDivElement | undefined
let qc: QueryClient | undefined

async function renderAt(path: string, page: ReactNode = <ZonesPage />) {
  host = document.createElement('div')
  document.body.appendChild(host)
  root = createRoot(host)
  qc = testQueryClient()
  await act(async () => {
    root!.render(
      <QueryClientProvider client={qc!}>
        <MemoryRouter initialEntries={[path]}>
          <Routes>
            <Route path="/zones" element={page} />
            <Route path="/zones/:zoneId" element={<ZoneDetailPage />} />
            <Route path="/zones/:zoneId/records/:recordId" element={<RecordDetailPage />} />
            <Route path="/changes" element={<ChangesProbe />} />
          </Routes>
        </MemoryRouter>
      </QueryClientProvider>,
    )
  })
}

async function waitForText(text: string) {
  await act(async () => {
    await vi.waitFor(() => {
      expect(host?.textContent).toContain(text)
    })
  })
}

function btn(label: string): HTMLButtonElement | undefined {
  return Array.from(host?.querySelectorAll('button') ?? []).find((b) => b.textContent === label)
}

function changesState(): { operations?: unknown[]; reason?: string } {
  const raw = host?.querySelector('[data-testid="changes-state"]')?.textContent ?? 'null'
  return JSON.parse(raw) as { operations?: unknown[]; reason?: string }
}

function zoneListURLs(): URL[] {
  return requested().filter((u) => u.pathname === '/v1/zones')
}

function recordListURLs(): URL[] {
  return requested().filter((u) => u.pathname === '/v1/zones/lab-zone/records')
}

function defaultHandler(session: unknown = writerSession): (url: URL) => Response {
  return (url) => {
    if (url.pathname === '/v1/session') {
      return jsonResponse(200, session)
    }
    if (url.pathname === '/v1/status') {
      return jsonResponse(200, { revisions: { runtimeRevision: REV } })
    }
    if (url.pathname === '/v1/zones') {
      const cursor = url.searchParams.get('cursor') ?? ''
      if (cursor === '') {
        return jsonResponse(200, { zones: [labZoneRaw], nextCursor: '1' })
      }
      return jsonResponse(200, { zones: [vendorZoneRaw] })
    }
    if (url.pathname === '/v1/zones/lab-zone') {
      return jsonResponse(200, labZoneRaw)
    }
    if (url.pathname === '/v1/zones/vendor-overlay') {
      return jsonResponse(200, vendorZoneRaw)
    }
    if (url.pathname === '/v1/zones/lab-zone/records') {
      const cursor = url.searchParams.get('cursor') ?? ''
      if (cursor === '') {
        return jsonResponse(200, {
          records: [
            { id: 'ns1-a', owner: 'ns1', type: 'A', ttl: '30s', values: ['10.42.0.53'] },
            {
              id: 'tools-wildcard-a',
              owner: '*.tools',
              type: 'A',
              ttl: '30s',
              values: ['10.42.0.20'],
              chaosPolicyRefs: ['slow-tools'],
            },
          ],
          nextCursor: '1',
        })
      }
      return jsonResponse(200, {
        records: [{ id: 'grafana-cname', owner: 'grafana.tools', type: 'CNAME', ttl: '30s' }],
      })
    }
    if (url.pathname === '/v1/zones/vendor-overlay/records') {
      return jsonResponse(200, { records: vendorZoneRaw.records })
    }
    if (url.pathname === '/v1/zones/lab-zone/records/ns1-a') {
      return jsonResponse(200, ns1RecordRaw)
    }
    return jsonResponse(404, { code: 'not_found', detail: url.pathname })
  }
}

afterEach(async () => {
  if (root) {
    await act(async () => {
      root!.unmount()
    })
  }
  host?.remove()
  root = undefined
  host = undefined
  qc = undefined
  fetchMock.mockReset()
  vi.restoreAllMocks()
})

describe('zone/record list parsers', () => {
  it('reads nextCursor, soa, and records from list envelopes', () => {
    const zones = parseZoneList({
      zones: [
        {
          id: 'lab-zone',
          name: 'lab.example.net.',
          mode: 'authoritative',
          records: [{ id: 'ns1-a', owner: 'ns1', type: 'A' }],
        },
        { id: '', name: 'skip' },
      ],
      nextCursor: '1',
    })
    expect(zones.zones).toEqual([
      {
        id: 'lab-zone',
        name: 'lab.example.net.',
        mode: 'authoritative',
        nameservers: [],
        soa: null,
        records: [
          { id: 'ns1-a', owner: 'ns1', type: 'A', ttl: '', values: [], chaosPolicyRefs: [] },
        ],
      },
    ])
    expect(zones.nextCursor).toBe('1')
    expect(formatSOA(null)).toBe('—')

    const records = parseRecordList({
      records: [{ id: 'ns1-a', owner: 'ns1', type: 'A', ttl: '30s', values: ['10.42.0.53'] }],
    })
    expect(records.nextCursor).toBe('')
    expect(records.records[0]?.ttl).toBe('30s')
    expect(records.records[0]?.values).toEqual(['10.42.0.53'])
  })

  it('treats a missing envelope as an empty page', () => {
    expect(parseZoneList(null)).toEqual({ zones: [], nextCursor: '' })
    expect(parseRecordList({})).toEqual({ records: [], nextCursor: '' })
  })
})

describe('ZonesPage', () => {
  it('lists zones FQDN-first with record counts and loads selected records', async () => {
    mockAPI(defaultHandler())
    await renderAt('/zones')
    await waitForText('lab.example.net.')
    expect(host?.textContent).toContain('lab-zone · 2 records')
    expect(host?.textContent).toContain('authoritative')
    expect(host?.querySelector('a[href="/zones/lab-zone"]')?.textContent).toContain('lab-zone')
    await waitForText('ns1.lab.example.net.')
    expect(host?.textContent).toContain('auto')
    expect(host?.textContent).toContain('10.42.0.53')
    expect(host?.querySelector('.chaos-ref')?.textContent).toBe('slow-tools')
    expect(host?.querySelector('a[href="/zones/lab-zone/records/ns1-a"]')?.textContent).toBe('ns1-a')
    expect(host?.textContent).not.toContain('mutations in UI-003')

    const firstZones = requested().find((u) => u.pathname === '/v1/zones')
    expect(firstZones?.searchParams.get('limit')).toBe(String(DEFAULT_PAGE_LIMIT))
    expect(firstZones?.searchParams.get('cursor')).toBeNull()

    const next = btn('Next zones')
    expect(next).toBeDefined()
    await act(async () => {
      next!.click()
    })
    await waitForText('vendor-overlay')
    expect(host?.textContent).toContain('overlay')
    expect(host?.textContent).toContain('1 record')

    const paged = zoneListURLs().filter((u) => u.searchParams.get('cursor') === '1')
    expect(paged.length).toBeGreaterThan(0)
    expect(qc?.getQueryState(zonesListKey(REV, '1', DEFAULT_PAGE_LIMIT))?.status).toBe('success')

    const firstPageGets = zoneListURLs().filter((u) => u.searchParams.get('cursor') === null).length
    await act(async () => {
      btn('First zones')!.click()
    })
    await waitForText('lab.example.net.')
    expect(zoneListURLs().filter((u) => u.searchParams.get('cursor') === null).length).toBeGreaterThan(
      firstPageGets,
    )
  })

  it('pages records independently of the inventory', async () => {
    mockAPI(defaultHandler())
    await renderAt('/zones/lab-zone')
    await waitForText('ns1-a')
    await act(async () => {
      btn('Next records')!.click()
    })
    await waitForText('grafana-cname')
    expect(host?.textContent).toContain('CNAME')
    const paged = recordListURLs().filter((u) => u.searchParams.get('cursor') === '1')
    expect(paged.length).toBeGreaterThan(0)
    expect(qc?.getQueryState(queryKeys.zone(REV, 'lab-zone'))?.status).toBe('success')
    expect(qc?.getQueryState(recordsListKey(REV, 'lab-zone', '1', DEFAULT_PAGE_LIMIT))?.status).toBe(
      'success',
    )
    await act(async () => {
      btn('First records')!.click()
    })
    await waitForText('ns1-a')
  })

  it('drops inventory cursor on the same render when runtimeRevision changes', async () => {
    const rev2 = 'sha256:def'
    mockAPI(defaultHandler())
    await renderAt('/zones')
    await waitForText('lab.example.net.')
    await act(async () => {
      btn('Next zones')!.click()
    })
    await waitForText('vendor-overlay')

    const marked = requested().length
    mockAPI((url) => {
      if (url.pathname === '/v1/session') {
        return jsonResponse(200, writerSession)
      }
      if (url.pathname === '/v1/status') {
        return jsonResponse(200, { revisions: { runtimeRevision: rev2 } })
      }
      if (url.pathname === '/v1/zones') {
        return jsonResponse(200, {
          zones: [{ id: 'after-rev', name: 'new.example.', mode: 'authoritative', records: [] }],
        })
      }
      if (url.pathname === '/v1/zones/after-rev') {
        return jsonResponse(200, { id: 'after-rev', name: 'new.example.', mode: 'authoritative', records: [] })
      }
      if (url.pathname === '/v1/zones/after-rev/records') {
        return jsonResponse(200, { records: [] })
      }
      return jsonResponse(404, { code: 'not_found', detail: url.pathname })
    })
    await act(async () => {
      await qc!.invalidateQueries({ queryKey: queryKeys.status() })
    })
    await waitForText('after-rev')
    expect(qc?.getQueryState(zonesListKey(rev2, '1', DEFAULT_PAGE_LIMIT))).toBeUndefined()
    expect(qc?.getQueryState(zonesListKey(rev2, '', DEFAULT_PAGE_LIMIT))?.status).toBe('success')
    const after = requested()
      .slice(marked)
      .filter((u) => u.pathname === '/v1/zones' && u.searchParams.get('cursor') === null)
    expect(after.length).toBeGreaterThan(0)
  })

  it('keeps First zones when the next inventory page fails', async () => {
    mockAPI((url) => {
      if (url.pathname === '/v1/session') {
        return jsonResponse(200, writerSession)
      }
      if (url.pathname === '/v1/status') {
        return jsonResponse(200, { revisions: { runtimeRevision: REV } })
      }
      if (url.pathname === '/v1/zones') {
        const cursor = url.searchParams.get('cursor') ?? ''
        if (cursor === '') {
          return jsonResponse(200, { zones: [labZoneRaw], nextCursor: '1' })
        }
        return jsonResponse(403, { code: 'forbidden', detail: 'dns.read required' })
      }
      if (url.pathname === '/v1/zones/lab-zone') {
        return jsonResponse(200, labZoneRaw)
      }
      if (url.pathname === '/v1/zones/lab-zone/records') {
        return jsonResponse(200, { records: [] })
      }
      return jsonResponse(404, { code: 'not_found', detail: url.pathname })
    })

    await renderAt('/zones')
    await waitForText('lab.example.net.')
    await act(async () => {
      btn('Next zones')!.click()
    })
    await waitForText('forbidden: dns.read required')
    expect(btn('First zones')).toBeDefined()
    expect(btn('Next zones')).toBeUndefined()
    expect(host?.querySelector('[role="alert"]')?.textContent).toBe('forbidden: dns.read required')
  })

  it('hops Create zone with a SOA skeleton', async () => {
    mockAPI(defaultHandler())
    await renderAt('/zones')
    await waitForText('Create zone')
    await act(async () => {
      await vi.waitFor(() => {
        expect(btn('Create zone')?.disabled).toBe(false)
      })
    })
    await act(async () => {
      btn('Create zone')!.click()
    })
    expect(changesState().operations).toEqual([createZoneOperation()])
  })

  it('hops Edit zone with the raw GET JSON', async () => {
    mockAPI(defaultHandler())
    await renderAt('/zones/lab-zone')
    await act(async () => {
      await vi.waitFor(() => {
        expect(btn('Edit zone')?.disabled).toBe(false)
      })
    })
    await act(async () => {
      btn('Edit zone')!.click()
    })
    const op = changesState().operations?.[0] as { value?: Record<string, unknown> }
    expect(op).toMatchObject({ op: 'update', target: { kind: 'zone', id: 'lab-zone' } })
    expect(op.value).toEqual(labZoneRaw)
    expect(op.value).not.toHaveProperty('recordCount')
    expect((op.value?.soa as { minimum?: string } | undefined)?.minimum).toBeUndefined()
    expect((op.value?.records as { genericRdata?: unknown }[])?.[1]?.genericRdata).toEqual(genericRdata)
  })

  it('hops Delete zone', async () => {
    mockAPI(defaultHandler())
    await renderAt('/zones/lab-zone')
    await act(async () => {
      await vi.waitFor(() => {
        expect(btn('Delete zone')?.disabled).toBe(false)
      })
    })
    await act(async () => {
      btn('Delete zone')!.click()
    })
    expect(changesState().operations).toEqual([{ op: 'remove', target: { kind: 'zone', id: 'lab-zone' } }])
  })

  it('hops Create record with zoneId', async () => {
    mockAPI(defaultHandler())
    await renderAt('/zones/lab-zone')
    await act(async () => {
      await vi.waitFor(() => {
        expect(btn('Create record')?.disabled).toBe(false)
      })
    })
    await act(async () => {
      btn('Create record')!.click()
    })
    expect(changesState().operations).toEqual([createRecordOperation('lab-zone')])
  })

  it('disables hops for a viewer and names dns.write', async () => {
    mockAPI(defaultHandler(viewerSession))
    await renderAt('/zones/lab-zone')
    await waitForText('Missing scope dns.write')
    expect(btn('Create zone')?.disabled).toBe(true)
    expect(btn('Edit zone')?.disabled).toBe(true)
    expect(btn('Delete zone')?.disabled).toBe(true)
    expect(btn('Create record')?.disabled).toBe(true)
  })

  it('announces list errors with role=alert', async () => {
    mockAPI((url) => {
      if (url.pathname === '/v1/session') {
        return jsonResponse(200, writerSession)
      }
      if (url.pathname === '/v1/status') {
        return jsonResponse(200, { revisions: { runtimeRevision: REV } })
      }
      return jsonResponse(403, { code: 'forbidden', detail: 'dns.read required' })
    })
    await renderAt('/zones')
    await waitForText('forbidden: dns.read required')
    expect(host?.querySelector('[role="alert"]')?.textContent).toBe('forbidden: dns.read required')
  })

  it('shows a single not_found alert for an unknown zoneId', async () => {
    mockAPI((url) => {
      if (url.pathname === '/v1/session') {
        return jsonResponse(200, writerSession)
      }
      if (url.pathname === '/v1/status') {
        return jsonResponse(200, { revisions: { runtimeRevision: REV } })
      }
      if (url.pathname === '/v1/zones') {
        return jsonResponse(200, { zones: [labZoneRaw] })
      }
      if (
        url.pathname === '/v1/zones/does-not-exist' ||
        url.pathname === '/v1/zones/does-not-exist/records'
      ) {
        return jsonResponse(404, { code: 'not_found', detail: 'zone does-not-exist not found' })
      }
      return jsonResponse(404, { code: 'not_found', detail: url.pathname })
    })

    await renderAt('/zones/does-not-exist')
    await waitForText('not_found: zone does-not-exist not found')
    const alerts = [...(host?.querySelectorAll('[role="alert"]') ?? [])]
    expect(alerts).toHaveLength(1)
    expect(alerts[0]?.textContent).toBe('not_found: zone does-not-exist not found')
    expect(host?.textContent).toContain('lab.example.net.')
    expect(host?.textContent).not.toContain('Loading records…')
    expect(requested().some((u) => u.pathname === '/v1/zones/does-not-exist/records')).toBe(false)
  })
})

describe('RecordDetailPage', () => {
  it('hops Edit record with raw GET JSON and zoneId', async () => {
    mockAPI(defaultHandler())
    await renderAt('/zones/lab-zone/records/ns1-a')
    await waitForText('10.42.0.53')
    expect(host?.textContent).toContain('slow-tools')
    expect(host?.querySelector('article.record-detail .surface')).not.toBeNull()
    await act(async () => {
      await vi.waitFor(() => {
        expect(btn('Edit record')?.disabled).toBe(false)
      })
    })
    expect(btn('Edit record')?.classList.contains('btn-accent')).toBe(true)
    await act(async () => {
      btn('Edit record')!.click()
    })
    const op = changesState().operations?.[0] as {
      op?: string
      target?: { kind?: string; id?: string; zoneId?: string }
      value?: unknown
    }
    expect(op.op).toBe('update')
    expect(op.target).toEqual({ kind: 'record', id: 'ns1-a', zoneId: 'lab-zone' })
    expect(op.value).toEqual(ns1RecordRaw)
    expect(host?.textContent).not.toContain('mutations in UI-003')
    expect(qc?.getQueryState(queryKeys.record(REV, 'lab-zone', 'ns1-a'))?.status).toBe('success')
  })

  it('hops Delete record with zoneId', async () => {
    mockAPI(defaultHandler())
    await renderAt('/zones/lab-zone/records/ns1-a')
    await act(async () => {
      await vi.waitFor(() => {
        expect(btn('Delete record')?.disabled).toBe(false)
      })
    })
    await act(async () => {
      btn('Delete record')!.click()
    })
    expect(changesState().operations).toEqual([
      { op: 'remove', target: { kind: 'record', id: 'ns1-a', zoneId: 'lab-zone' } },
    ])
  })
})
