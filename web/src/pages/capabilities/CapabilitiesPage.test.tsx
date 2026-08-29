import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, type ReactNode } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { createLabdnsClient } from '../../api/client'
import { queryKeys } from '../../query/keys'
import { CapabilitiesPage } from './CapabilitiesPage'
import { capabilityList, fetchCapabilities } from './capabilities'

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

describe('capabilities API', () => {
  it('GETs /v1/capabilities', async () => {
    const body = {
      capabilities: [{ name: 'dns_state_get', version: 'v1', description: 'Get state', mutating: false, idempotent: true }],
    }
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse(200, body))
    const api = createLabdnsClient({ fetch: fetchMock })
    await expect(fetchCapabilities(api)).resolves.toEqual(body)
    const req = requestOf(fetchMock.mock.calls[0] as unknown[])
    expect(pathOf(req)).toBe('/v1/capabilities')
    expect(req.method).toBe('GET')
    expect(capabilityList(body)).toHaveLength(1)
  })
})

describe('CapabilitiesPage', () => {
  it('renders capability rows under a revision-scoped query key', async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      const req = input instanceof Request ? input : new Request(String(input))
      const path = pathOf(req)
      if (path === '/v1/status') {
        return jsonResponse(200, { revisions: { runtimeRevision: 'sha256:caps' } })
      }
      if (path === '/v1/capabilities') {
        return jsonResponse(200, {
          capabilities: [
            { name: 'dns_state_get', version: 'v1', description: 'Active revisions', mutating: false, idempotent: true },
            { name: 'dns_change_apply', version: 'v1', description: 'Apply operations', mutating: true, idempotent: true },
          ],
        })
      }
      return jsonResponse(404, { code: 'not_found', detail: path, status: 404, title: 'Not found', type: 'about:blank' })
    })
    vi.stubGlobal('fetch', fetchMock)
    const qc = testClient()
    qc.setQueryData(queryKeys.status(), { revisions: { runtimeRevision: 'sha256:caps' } })
    await render(
      <QueryClientProvider client={qc}>
        <CapabilitiesPage />
      </QueryClientProvider>,
    )
    await act(async () => {
      await vi.waitFor(() => {
        expect(host?.textContent).toContain('dns_state_get')
      })
    })
    expect(host?.textContent).toContain('dns_change_apply')
    expect(host?.textContent).toContain('Apply operations')
    expect(host?.querySelector('table.data-table')).not.toBeNull()
    expect(host?.querySelector('.page-lede')).not.toBeNull()
    expect(qc.getQueryCache().find({ queryKey: queryKeys.capabilities('sha256:caps') })).toBeTruthy()
    expect(fetchMock.mock.calls.every((c) => requestOf(c as unknown[]).method === 'GET')).toBe(true)
  })

  it('announces capabilities fetch failures', async () => {
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
        <CapabilitiesPage />
      </QueryClientProvider>,
    )
    await act(async () => {
      await vi.waitFor(() => {
        expect(host?.querySelector('[role="alert"]')?.textContent).toBe('forbidden: dns.read required')
      })
    })
  })
})
