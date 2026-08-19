import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, type ReactNode } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { MemoryRouter, Route, Routes } from 'react-router'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { queryKeys } from '../../query/keys'
import { RecordDetailPage } from './RecordDetailPage'
import { ZoneDetailPage } from './ZoneDetailPage'
import { ZonesPage } from './ZonesPage'
import {
  DEFAULT_PAGE_LIMIT,
  MUTATIONS_UI003,
  parseRecordList,
  parseZoneList,
  recordsListKey,
  zonesListKey,
} from './zones'

// Singleton openapi-fetch client binds fetch at import time.
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

let root: Root | undefined
let host: HTMLDivElement | undefined
let qc: QueryClient | undefined

async function renderAt(path: string, page: ReactNode) {
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
            <Route path="/zones/:zoneId" element={page} />
            <Route path="/zones/:zoneId/records/:recordId" element={page} />
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
  it('reads nextCursor and items from list envelopes', () => {
    const zones = parseZoneList({
      zones: [
        { id: 'lab-zone', name: 'lab.example.net.', mode: 'authoritative' },
        { id: '', name: 'skip' },
      ],
      nextCursor: '1',
    })
    expect(zones.zones).toEqual([
      { id: 'lab-zone', name: 'lab.example.net.', mode: 'authoritative', nameservers: [] },
    ])
    expect(zones.nextCursor).toBe('1')

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
  it('lists zones and sends OpenAPI cursor/limit on GET /v1/zones', async () => {
    mockAPI((url) => {
      if (url.pathname === '/v1/status') {
        return jsonResponse(200, { revisions: { runtimeRevision: REV } })
      }
      if (url.pathname === '/v1/zones') {
        const cursor = url.searchParams.get('cursor') ?? ''
        if (cursor === '') {
          return jsonResponse(200, {
            zones: [{ id: 'lab-zone', name: 'lab.example.net.', mode: 'authoritative' }],
            nextCursor: '1',
          })
        }
        return jsonResponse(200, {
          zones: [{ id: 'vendor-overlay', name: 'vendor.example.', mode: 'overlay' }],
        })
      }
      return jsonResponse(404, { code: 'not_found', detail: url.pathname })
    })

    await renderAt('/zones', <ZonesPage />)
    await waitForText('lab-zone')
    expect(host?.textContent).toContain('lab.example.net.')
    expect(host?.textContent).toContain('authoritative')
    expect(host?.querySelector('a[href="/zones/lab-zone"]')?.textContent).toBe('lab-zone')

    const firstZones = requested().find((u) => u.pathname === '/v1/zones')
    expect(firstZones?.searchParams.get('limit')).toBe(String(DEFAULT_PAGE_LIMIT))
    expect(firstZones?.searchParams.get('cursor')).toBeNull()

    const next = Array.from(host!.querySelectorAll('button')).find((b) => b.textContent === 'Next')
    expect(next).toBeDefined()
    await act(async () => {
      next!.click()
    })
    await waitForText('vendor-overlay')
    expect(host?.textContent).toContain('overlay')

    const paged = requested().filter(
      (u) => u.pathname === '/v1/zones' && u.searchParams.get('cursor') === '1',
    )
    expect(paged.length).toBeGreaterThan(0)
    expect(paged[0]?.searchParams.get('limit')).toBe(String(DEFAULT_PAGE_LIMIT))

    expect(qc?.getQueryState(zonesListKey(REV, '1', DEFAULT_PAGE_LIMIT))?.status).toBe('success')
  })

  it('disables create with mutations in UI-003', async () => {
    mockAPI((url) => {
      if (url.pathname === '/v1/status') {
        return jsonResponse(200, { revisions: { runtimeRevision: REV } })
      }
      return jsonResponse(200, { zones: [] })
    })
    await renderAt('/zones', <ZonesPage />)
    await waitForText('No zones.')
    const create = Array.from(host!.querySelectorAll('button')).find((b) => b.textContent === 'Create zone')
    expect(create?.disabled).toBe(true)
    expect(host?.textContent).toContain(MUTATIONS_UI003)
  })

  it('announces list errors with role=alert', async () => {
    mockAPI((url) => {
      if (url.pathname === '/v1/status') {
        return jsonResponse(200, { revisions: { runtimeRevision: REV } })
      }
      return jsonResponse(403, { code: 'forbidden', detail: 'dns.read required' })
    })
    await renderAt('/zones', <ZonesPage />)
    await waitForText('forbidden: dns.read required')
    expect(host?.querySelector('[role="alert"]')?.textContent).toBe('forbidden: dns.read required')
  })
})

describe('ZoneDetailPage', () => {
  it('shows zone metadata and records with type/name/TTL plus cursor pagination', async () => {
    mockAPI((url) => {
      if (url.pathname === '/v1/status') {
        return jsonResponse(200, { revisions: { runtimeRevision: REV } })
      }
      if (url.pathname === '/v1/zones/lab-zone') {
        return jsonResponse(200, {
          id: 'lab-zone',
          name: 'lab.example.net.',
          mode: 'authoritative',
          nameservers: ['ns1.lab.example.net.'],
        })
      }
      if (url.pathname === '/v1/zones/lab-zone/records') {
        const cursor = url.searchParams.get('cursor') ?? ''
        if (cursor === '') {
          return jsonResponse(200, {
            records: [{ id: 'ns1-a', owner: 'ns1', type: 'A', ttl: '30s', values: ['10.42.0.53'] }],
            nextCursor: '1',
          })
        }
        return jsonResponse(200, {
          records: [{ id: 'grafana-cname', owner: 'grafana.tools', type: 'CNAME', ttl: '30s' }],
        })
      }
      return jsonResponse(404, { code: 'not_found', detail: url.pathname })
    })

    await renderAt('/zones/lab-zone', <ZoneDetailPage />)
    await waitForText('ns1-a')
    expect(host?.textContent).toContain('lab.example.net.')
    expect(host?.textContent).toContain('authoritative')
    expect(host?.textContent).toContain('ns1')
    expect(host?.textContent).toContain('A')
    expect(host?.textContent).toContain('30s')
    expect(host?.querySelector('a[href="/zones/lab-zone/records/ns1-a"]')?.textContent).toBe('ns1-a')

    const recsFirst = requested().find((u) => u.pathname === '/v1/zones/lab-zone/records')
    expect(recsFirst?.searchParams.get('limit')).toBe(String(DEFAULT_PAGE_LIMIT))
    expect(recsFirst?.searchParams.get('cursor')).toBeNull()

    const next = Array.from(host!.querySelectorAll('button')).find((b) => b.textContent === 'Next')
    await act(async () => {
      next!.click()
    })
    await waitForText('grafana-cname')
    expect(host?.textContent).toContain('CNAME')

    const paged = requested().filter(
      (u) => u.pathname === '/v1/zones/lab-zone/records' && u.searchParams.get('cursor') === '1',
    )
    expect(paged.length).toBeGreaterThan(0)
    expect(paged[0]?.searchParams.get('limit')).toBe(String(DEFAULT_PAGE_LIMIT))

    expect(qc?.getQueryState(queryKeys.zone(REV, 'lab-zone'))?.status).toBe('success')
    expect(qc?.getQueryState(recordsListKey(REV, 'lab-zone', '1', DEFAULT_PAGE_LIMIT))?.status).toBe(
      'success',
    )
  })

  it('disables zone and record mutations with mutations in UI-003', async () => {
    mockAPI((url) => {
      if (url.pathname === '/v1/status') {
        return jsonResponse(200, { revisions: { runtimeRevision: REV } })
      }
      if (url.pathname === '/v1/zones/lab-zone') {
        return jsonResponse(200, { id: 'lab-zone', name: 'lab.example.net.', mode: 'authoritative' })
      }
      return jsonResponse(200, { records: [] })
    })
    await renderAt('/zones/lab-zone', <ZoneDetailPage />)
    await waitForText('No records.')
    const labels = Array.from(host!.querySelectorAll('button')).map((b) => b.textContent)
    expect(labels).toEqual(expect.arrayContaining(['Edit zone', 'Delete zone', 'Create record']))
    for (const b of host!.querySelectorAll('button')) {
      if (b.textContent === 'Next' || b.textContent === 'First page') {
        continue
      }
      expect(b).toHaveProperty('disabled', true)
    }
    expect(host?.textContent).toContain(MUTATIONS_UI003)
  })
})

describe('RecordDetailPage', () => {
  it('loads GET /v1/zones/{zoneId}/records/{recordId} with the record query key', async () => {
    mockAPI((url) => {
      if (url.pathname === '/v1/status') {
        return jsonResponse(200, { revisions: { runtimeRevision: REV } })
      }
      if (url.pathname === '/v1/zones/lab-zone/records/ns1-a') {
        return jsonResponse(200, {
          id: 'ns1-a',
          owner: 'ns1',
          type: 'A',
          ttl: '30s',
          values: ['10.42.0.53'],
          chaosPolicyRefs: ['slow-tools'],
        })
      }
      return jsonResponse(404, { code: 'not_found', detail: url.pathname })
    })

    await renderAt('/zones/lab-zone/records/ns1-a', <RecordDetailPage />)
    await waitForText('10.42.0.53')
    expect(host?.textContent).toContain('ns1')
    expect(host?.textContent).toContain('A')
    expect(host?.textContent).toContain('30s')
    expect(host?.textContent).toContain('slow-tools')
    expect(host?.querySelector('a[href="/zones/lab-zone"]')?.textContent).toBe('lab-zone')

    const edit = Array.from(host!.querySelectorAll('button')).find((b) => b.textContent === 'Edit record')
    const del = Array.from(host!.querySelectorAll('button')).find((b) => b.textContent === 'Delete record')
    expect(edit?.disabled).toBe(true)
    expect(del?.disabled).toBe(true)
    expect(host?.textContent).toContain(MUTATIONS_UI003)

    expect(qc?.getQueryState(queryKeys.record(REV, 'lab-zone', 'ns1-a'))?.status).toBe('success')
  })
})
