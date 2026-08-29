import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, type ReactNode } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { client } from '../../api/client'
import { queryKeys } from '../../query/keys'
import { LIVE_POLL_MS } from '../../query/live'
import { CachePage } from './CachePage'
import { FlushPanel } from './FlushPanel'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

function ok(data: unknown) {
  return { data, error: undefined, response: jsonResponse(200, data) }
}

function fail(status: number, body: unknown) {
  return { data: undefined, error: body, response: jsonResponse(status, body) }
}

let root: Root | undefined
let host: HTMLDivElement | undefined
let queryClient: QueryClient | undefined

function testQueryClient(): QueryClient {
  return new QueryClient({
    defaultOptions: {
      queries: { retry: false, refetchOnWindowFocus: false },
    },
  })
}

async function render(ui: ReactNode) {
  host = document.createElement('div')
  document.body.appendChild(host)
  queryClient = testQueryClient()
  root = createRoot(host)
  await act(async () => {
    root!.render(<QueryClientProvider client={queryClient!}>{ui}</QueryClientProvider>)
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
  queryClient = undefined
  vi.restoreAllMocks()
})

describe('CachePage', () => {
  it('polls cache status on a revision-independent live key and leaves flush disabled', async () => {
    expect(LIVE_POLL_MS).toBe(5000)
    const get = vi.spyOn(client, 'GET').mockImplementation((async (path: string) => {
      if (path === '/v1/cache/status') {
        return ok({
          enabled: true,
          maxEntries: 10000,
          entries: 12,
          hits: 40,
          misses: 3,
          evicts: 1,
        })
      }
      return fail(404, { code: 'not_found', detail: path })
    }) as typeof client.GET)
    const post = vi.spyOn(client, 'POST')

    await render(<CachePage />)
    await act(async () => {
      await vi.waitFor(() => {
        expect(host?.textContent).toContain('40')
        expect(host?.textContent).toContain('12')
        expect(host?.textContent).toContain('10000')
      })
    })

    expect(host?.textContent).toContain('Yes')
    expect(host?.querySelector('.page-lede')).not.toBeNull()
    expect(host?.querySelectorAll('section.surface').length).toBeGreaterThanOrEqual(2)
    const flush = [...(host?.querySelectorAll('button') ?? [])].find((b) => b.textContent === 'Flush cache')
    expect(flush).toBeDefined()
    expect(flush?.disabled).toBe(true)
    const selector = host?.querySelector('input[type="checkbox"]') as HTMLInputElement | null
    expect(selector?.disabled).toBe(true)
    expect(selector?.checked).toBe(true)

    expect(get.mock.calls.map((c) => c[0])).toContain('/v1/cache/status')
    expect(post).not.toHaveBeenCalled()
    expect(queryClient?.getQueryData(queryKeys.liveCache())).toBeTruthy()
    expect(queryKeys.liveCache()).toEqual(['labdns', 'live', 'cache'])
  })

  it('announces problem+json when cache status fails', async () => {
    vi.spyOn(client, 'GET').mockImplementation((async () =>
      fail(403, { code: 'forbidden', detail: 'dns.read required' })) as typeof client.GET)
    await render(<CachePage />)
    await act(async () => {
      await vi.waitFor(() => {
        expect(host?.querySelector('[role="alert"]')?.textContent).toBe('forbidden: dns.read required')
      })
    })
  })
})

describe('FlushPanel', () => {
  it('is an empty slot: flush control exists but cannot run', async () => {
    await render(<FlushPanel />)
    const button = host?.querySelector('button') as HTMLButtonElement
    expect(button.textContent).toBe('Flush cache')
    expect(button.disabled).toBe(true)
    expect(host?.textContent).toContain('dns.admin')
    expect(host?.textContent).toContain('does not change desired state')
  })
})
