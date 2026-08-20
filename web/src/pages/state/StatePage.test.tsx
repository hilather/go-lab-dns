import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, type ReactNode } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { createLabdnsClient } from '../../api/client'
import { queryKeys } from '../../query/keys'
import { StatePage } from './StatePage'
import { downloadStateExport, fetchState, fetchStateExport, prettyJSON } from './state'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

function jsonResponse(status: number, body: unknown, contentType = 'application/json'): Response {
  return new Response(typeof body === 'string' ? body : JSON.stringify(body), {
    status,
    headers: { 'Content-Type': contentType },
  })
}

function requestOf(call: unknown[] | undefined): Request {
  const first = call?.[0]
  if (first instanceof Request) {
    return first
  }
  return new Request(String(first), (call?.[1] ?? {}) as RequestInit)
}

function pathOf(req: Request): string {
  return new URL(req.url, 'http://localhost').pathname
}

let root: Root | undefined
let host: HTMLDivElement | undefined

async function render(ui: ReactNode) {
  host = document.createElement('div')
  document.body.appendChild(host)
  root = createRoot(host)
  await act(async () => {
    root!.render(ui)
  })
}

function testClient() {
  return new QueryClient({
    defaultOptions: { queries: { retry: false, refetchOnWindowFocus: false, refetchInterval: false } },
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
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})

describe('state API', () => {
  it('GETs /v1/state with credentials include and no CSRF', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      jsonResponse(200, {
        runtimeRevision: 'sha256:abc',
        bootstrapRevision: 'sha256:boot',
        generation: 3,
        drifted: true,
        canonical: { kind: 'LabDNS' },
      }),
    )
    const api = createLabdnsClient({ fetch: fetchMock })
    const data = await fetchState(api)
    expect(data).toMatchObject({ runtimeRevision: 'sha256:abc', drifted: true })
    const req = requestOf(fetchMock.mock.calls[0] as unknown[])
    expect(pathOf(req)).toBe('/v1/state')
    expect(req.method).toBe('GET')
    expect(req.credentials).toBe('include')
    expect(req.headers.get('X-LabDNS-CSRF')).toBeNull()
  })

  it('exports YAML as text and JSON as pretty metadata', async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(new Response('apiVersion: labdns.dev/v1alpha1\n', { status: 200, headers: { 'Content-Type': 'application/yaml' } }))
      .mockResolvedValueOnce(jsonResponse(200, { format: 'json', revision: 'sha256:abc', body: { kind: 'LabDNS' } }))
    const api = createLabdnsClient({ fetch: fetchMock })
    const yaml = await fetchStateExport('yaml', api)
    const json = await fetchStateExport('json', api)
    expect(yaml).toContain('apiVersion: labdns.dev/v1alpha1')
    expect(json).toBe(prettyJSON({ format: 'json', revision: 'sha256:abc', body: { kind: 'LabDNS' } }))
    expect(pathOf(requestOf(fetchMock.mock.calls[0] as unknown[]))).toBe('/v1/state:export')
    expect(new URL(requestOf(fetchMock.mock.calls[0] as unknown[]).url).searchParams.get('format')).toBe('yaml')
    expect(new URL(requestOf(fetchMock.mock.calls[1] as unknown[]).url).searchParams.get('format')).toBe('json')
    expect(requestOf(fetchMock.mock.calls[0] as unknown[]).method).toBe('GET')
  })

  it('download uses a blob URL and never writes storage', async () => {
    const setItem = vi.spyOn(Storage.prototype, 'setItem')
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      const req = input instanceof Request ? input : new Request(String(input))
      const format = new URL(req.url, 'http://localhost').searchParams.get('format')
      if (format === 'json') {
        return jsonResponse(200, { format: 'json', revision: 'sha256:abc', body: { kind: 'LabDNS' } })
      }
      return new Response('kind: LabDNS\n', { status: 200, headers: { 'Content-Type': 'application/yaml' } })
    })
    const createObjectURL = vi.fn((blob: Blob) => `blob:labdns-export-${blob.type}`)
    const revokeObjectURL = vi.fn()
    const origCreate = URL.createObjectURL
    const origRevoke = URL.revokeObjectURL
    URL.createObjectURL = createObjectURL as typeof URL.createObjectURL
    URL.revokeObjectURL = revokeObjectURL
    const clicked: HTMLAnchorElement[] = []
    vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(function (this: HTMLAnchorElement) {
      clicked.push(this)
    })
    const api = createLabdnsClient({ fetch: fetchMock })
    try {
      await downloadStateExport('yaml', api)
      await downloadStateExport('json', api)
    } finally {
      URL.createObjectURL = origCreate
      URL.revokeObjectURL = origRevoke
    }
    expect(clicked).toHaveLength(2)
    expect(clicked[0]?.download).toBe('labdns-state.yaml')
    expect(clicked[0]?.href).toContain('blob:')
    expect(clicked[1]?.download).toBe('labdns-state.json')
    expect(clicked[1]?.href).toContain('blob:')
    expect(createObjectURL.mock.calls[0]?.[0]).toMatchObject({ type: 'application/yaml' })
    expect(createObjectURL.mock.calls[1]?.[0]).toMatchObject({ type: 'application/json' })
    expect(revokeObjectURL).toHaveBeenCalledTimes(2)
    expect(setItem).not.toHaveBeenCalled()
    expect(window.localStorage.getItem('token')).toBeNull()
  })

  it('maps problem+json export failures', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      jsonResponse(403, { code: 'forbidden', detail: 'dns.read required', status: 403, title: 'Forbidden', type: 'urn:labdns:error:forbidden' }, 'application/problem+json'),
    )
    const api = createLabdnsClient({ fetch: fetchMock })
    await expect(fetchStateExport('json', api)).rejects.toMatchObject({
      status: 403,
      code: 'forbidden',
    })
  })
})

describe('StatePage', () => {
  it('renders GET /v1/state using a revision-scoped query key', async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      const req = input instanceof Request ? input : new Request(String(input))
      const path = pathOf(req)
      if (path === '/v1/status') {
        return jsonResponse(200, { revisions: { runtimeRevision: 'sha256:abc' } })
      }
      if (path === '/v1/state') {
        return jsonResponse(200, {
          runtimeRevision: 'sha256:abc',
          bootstrapRevision: 'sha256:boot',
          generation: 7,
          drifted: false,
          canonical: { kind: 'LabDNS', spec: { ui: { enabled: true } } },
        })
      }
      return jsonResponse(404, { code: 'not_found', detail: path, status: 404, title: 'Not found', type: 'about:blank' })
    })
    vi.stubGlobal('fetch', fetchMock)
    const qc = testClient()
    qc.setQueryData(queryKeys.status(), { revisions: { runtimeRevision: 'sha256:abc' } })
    await render(
      <QueryClientProvider client={qc}>
        <StatePage />
      </QueryClientProvider>,
    )
    await act(async () => {
      await vi.waitFor(() => {
        expect(host?.textContent).toContain('sha256:abc')
      })
    })
    expect(host?.textContent).toContain('sha256:boot')
    expect(host?.textContent).toContain('7')
    expect(host?.textContent).toContain('"kind": "LabDNS"')
    expect(qc.getQueryCache().find({ queryKey: queryKeys.state('sha256:abc') })).toBeTruthy()
    const methods = fetchMock.mock.calls.map((c) => requestOf(c as unknown[]).method)
    expect(methods.every((m) => m === 'GET')).toBe(true)
    const labels = [...(host?.querySelectorAll('button') ?? [])].map((b) => b.textContent)
    expect(labels).toContain('Download YAML')
    expect(labels).toContain('Download JSON')
  })

  it('does not GET /v1/state until a runtime revision is known', async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      const req = input instanceof Request ? input : new Request(String(input))
      return jsonResponse(404, { code: 'not_found', detail: pathOf(req), status: 404, title: 'Not found', type: 'about:blank' })
    })
    vi.stubGlobal('fetch', fetchMock)
    await render(
      <QueryClientProvider client={testClient()}>
        <StatePage />
      </QueryClientProvider>,
    )
    await act(async () => {
      await new Promise((r) => setTimeout(r, 50))
    })
    const stateGets = fetchMock.mock.calls.filter((c) => pathOf(requestOf(c as unknown[])) === '/v1/state')
    expect(stateGets).toHaveLength(0)
    expect(host?.textContent).toContain('Loading state')
  })

  it('announces problem+json with role=alert', async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      const req = input instanceof Request ? input : new Request(String(input))
      if (pathOf(req) === '/v1/status') {
        return jsonResponse(200, { revisions: { runtimeRevision: 'sha256:abc' } })
      }
      return jsonResponse(403, { code: 'forbidden', detail: 'dns.read required', status: 403, title: 'Forbidden', type: 'about:blank' }, 'application/problem+json')
    })
    vi.stubGlobal('fetch', fetchMock)
    const qc = testClient()
    qc.setQueryData(queryKeys.status(), { revisions: { runtimeRevision: 'sha256:abc' } })
    await render(
      <QueryClientProvider client={qc}>
        <StatePage />
      </QueryClientProvider>,
    )
    await act(async () => {
      await vi.waitFor(() => {
        expect(host?.querySelector('[role="alert"]')?.textContent).toContain('forbidden')
      })
    })
    expect(host?.querySelector('[role="alert"]')?.textContent).toContain('dns.read required')
  })
})
