import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, type ReactNode } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { createLabdnsClient } from '../../api/client'
import { queryKeys } from '../../query/keys'
import { SchemaPage } from './SchemaPage'
import { fetchConfigSchema } from './schema'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

function jsonResponse(status: number, body: unknown, contentType = 'application/json'): Response {
  return new Response(JSON.stringify(body), {
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

describe('schema API', () => {
  it('GETs /v1/schema/config as JSON', async () => {
    const schema = { $id: 'labdns.dev/v1alpha1', $defs: { spec: {} } }
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse(200, schema, 'application/schema+json'))
    const api = createLabdnsClient({ fetch: fetchMock })
    await expect(fetchConfigSchema(api)).resolves.toEqual(schema)
    const req = requestOf(fetchMock.mock.calls[0] as unknown[])
    expect(pathOf(req)).toBe('/v1/schema/config')
    expect(req.method).toBe('GET')
    expect(req.credentials).toBe('include')
  })
})

describe('SchemaPage', () => {
  it('renders the config schema under a revision-scoped query key', async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      const req = input instanceof Request ? input : new Request(String(input))
      const path = pathOf(req)
      if (path === '/v1/status') {
        return jsonResponse(200, { revisions: { runtimeRevision: 'sha256:schema' } })
      }
      if (path === '/v1/schema/config') {
        return jsonResponse(200, { $id: 'https://labdns.dev/schemas/v1alpha1', title: 'LabDNS' }, 'application/schema+json')
      }
      return jsonResponse(404, { code: 'not_found', detail: path, status: 404, title: 'Not found', type: 'about:blank' })
    })
    vi.stubGlobal('fetch', fetchMock)
    const qc = testClient()
    qc.setQueryData(queryKeys.status(), { revisions: { runtimeRevision: 'sha256:schema' } })
    await render(
      <QueryClientProvider client={qc}>
        <SchemaPage />
      </QueryClientProvider>,
    )
    await act(async () => {
      await vi.waitFor(() => {
        expect(host?.textContent).toContain('labdns.dev/schemas/v1alpha1')
      })
    })
    expect(host?.textContent).toContain('Config schema')
    expect(qc.getQueryCache().find({ queryKey: queryKeys.schema('sha256:schema') })).toBeTruthy()
    expect(fetchMock.mock.calls.every((c) => requestOf(c as unknown[]).method === 'GET')).toBe(true)
  })

  it('announces schema fetch failures', async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      const req = input instanceof Request ? input : new Request(String(input))
      if (pathOf(req) === '/v1/status') {
        return jsonResponse(200, { revisions: { runtimeRevision: 'sha256:x' } })
      }
      return jsonResponse(403, { code: 'forbidden', detail: 'dns.read required', status: 403, title: 'Forbidden', type: 'about:blank' }, 'application/problem+json')
    })
    vi.stubGlobal('fetch', fetchMock)
    const qc = testClient()
    qc.setQueryData(queryKeys.status(), { revisions: { runtimeRevision: 'sha256:x' } })
    await render(
      <QueryClientProvider client={qc}>
        <SchemaPage />
      </QueryClientProvider>,
    )
    await act(async () => {
      await vi.waitFor(() => {
        expect(host?.querySelector('[role="alert"]')?.textContent).toBe('forbidden: dns.read required')
      })
    })
  })
})
