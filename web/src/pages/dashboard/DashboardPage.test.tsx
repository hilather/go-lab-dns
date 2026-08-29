import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, type ReactNode } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { MemoryRouter, Outlet, Route, Routes } from 'react-router'
import { afterEach, describe, expect, it, vi } from 'vitest'
import * as sessionApi from '../../auth/sessionApi'
import type { ShellContext } from '../../components/Shell'
import type { StatusView } from '../../status'
import { DashboardPage } from './DashboardPage'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

let root: Root | undefined
let host: HTMLDivElement | undefined

const status: StatusView = {
  ready: true,
  revisions: { runtimeRevision: 'sha256:abc123', bootstrapRevision: 'sha256:boot', generation: 1 },
  listeners: [{ name: 'dns', address: '127.0.0.1:5353' }],
  cache: { enabled: true, entries: 1, maxEntries: 10, hits: 2, misses: 0, evicts: 0 },
  chaos: { enabled: false, emergencyDisabled: false, activePolicies: 0 },
}

async function renderDashboard(ui: ReactNode) {
  host = document.createElement('div')
  document.body.appendChild(host)
  root = createRoot(host)
  const qc = new QueryClient({
    defaultOptions: { queries: { retry: false, refetchOnWindowFocus: false } },
  })
  await act(async () => {
    root!.render(
      <QueryClientProvider client={qc}>
        <MemoryRouter initialEntries={['/']}>
          <Routes>
            <Route element={<Outlet context={{ status } satisfies ShellContext} />}>
              <Route path="/" element={ui} />
            </Route>
          </Routes>
        </MemoryRouter>
      </QueryClientProvider>,
    )
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
})

describe('DashboardPage chrome', () => {
  it('uses page-lede and surface sections', async () => {
    vi.spyOn(sessionApi, 'getJSON').mockResolvedValue({ version: '1.2.0', commit: 'deadbeef' })
    await renderDashboard(<DashboardPage />)
    await act(async () => {
      await vi.waitFor(() => {
        expect(host?.textContent).toContain('1.2.0')
      })
    })
    expect(host?.querySelector('.page-lede')).not.toBeNull()
    expect(host?.querySelectorAll('section.surface').length).toBeGreaterThanOrEqual(4)
    expect(host?.querySelector('h1')?.textContent).toBe('Overview')
  })
})
