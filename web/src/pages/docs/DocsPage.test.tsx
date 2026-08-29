import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, type ReactNode } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { MemoryRouter, Route, Routes } from 'react-router'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { createLabdnsClient } from '../../api/client'
import { APIError } from '../../auth/sessionApi'
import { queryKeys } from '../../query/keys'
import { ROUTES } from '../../routes'
import { DocsPage } from './DocsPage'
import { DOC_CATALOG, fetchDoc, isDocId } from './docs'

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

function page(initial: string) {
  const qc = testClient()
  return (
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={[initial]}>
        <Routes>
          <Route path={ROUTES.docsIndex} element={<DocsPage />} />
          <Route path={ROUTES.docs} element={<DocsPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>
  )
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

describe('docs API', () => {
  it('GETs markdown for the frozen document ids only', async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(new Response('# DNS\n', { status: 200, headers: { 'Content-Type': 'text/markdown; charset=utf-8' } }))
      .mockResolvedValueOnce(new Response('# Chaos\n', { status: 200, headers: { 'Content-Type': 'text/markdown; charset=utf-8' } }))
    const api = createLabdnsClient({ fetch: fetchMock })
    await expect(fetchDoc('dns-semantics', api)).resolves.toBe('# DNS\n')
    await expect(fetchDoc('chaos-safety', api)).resolves.toBe('# Chaos\n')
    expect(pathOf(requestOf(fetchMock.mock.calls[0] as unknown[]))).toBe('/v1/docs/dns-semantics')
    expect(pathOf(requestOf(fetchMock.mock.calls[1] as unknown[]))).toBe('/v1/docs/chaos-safety')
    expect(requestOf(fetchMock.mock.calls[0] as unknown[]).method).toBe('GET')
    await expect(fetchDoc('nope', api)).rejects.toBeInstanceOf(APIError)
    expect(fetchMock).toHaveBeenCalledTimes(2)
    expect(isDocId('dns-semantics')).toBe(true)
    expect(isDocId('nope')).toBe(false)
  })
})

describe('DocsPage', () => {
  it('lists both documents on the index', async () => {
    await render(page('/docs'))
    expect(host?.querySelector('.page-lede')).not.toBeNull()
    expect(host?.querySelector('section.surface')).not.toBeNull()
    for (const doc of DOC_CATALOG) {
      expect(host?.textContent).toContain(doc.title)
      const link = host?.querySelector(`a[href="/docs/${doc.id}"]`)
      expect(link).not.toBeNull()
    }
  })

  it('renders markdown as text and does not execute HTML', async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      const req = input instanceof Request ? input : new Request(String(input))
      if (pathOf(req) === '/v1/docs/dns-semantics') {
        return new Response('# Flags\n<script>alert(1)</script>\n', {
          status: 200,
          headers: { 'Content-Type': 'text/markdown; charset=utf-8' },
        })
      }
      return jsonResponse(404, { code: 'not_found', detail: pathOf(req), status: 404, title: 'Not found', type: 'about:blank' })
    })
    vi.stubGlobal('fetch', fetchMock)
    const qc = testClient()
    await render(
      <QueryClientProvider client={qc}>
        <MemoryRouter initialEntries={['/docs/dns-semantics']}>
          <Routes>
            <Route path={ROUTES.docs} element={<DocsPage />} />
          </Routes>
        </MemoryRouter>
      </QueryClientProvider>,
    )
    await act(async () => {
      await vi.waitFor(() => {
        expect(host?.textContent).toContain('# Flags')
      })
    })
    expect(host?.textContent).toContain('<script>alert(1)</script>')
    expect(host?.querySelector('pre.code-block')).not.toBeNull()
    expect(host?.innerHTML).toContain('&lt;script&gt;alert(1)&lt;/script&gt;')
    expect(host?.querySelector('script')).toBeNull()
    expect(qc.getQueryCache().find({ queryKey: queryKeys.docs('dns-semantics') })).toBeTruthy()
  })

  it('does not fetch unknown document ids', async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse(200, {}))
    vi.stubGlobal('fetch', fetchMock)
    await render(page('/docs/not-a-doc'))
    await act(async () => {
      await vi.waitFor(() => {
        expect(host?.querySelector('[role="alert"]')?.textContent).toContain('unknown document not-a-doc')
      })
    })
    expect(fetchMock).not.toHaveBeenCalled()
  })
})
