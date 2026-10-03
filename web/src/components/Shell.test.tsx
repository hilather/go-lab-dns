import { QueryClientProvider } from '@tanstack/react-query'
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { MemoryRouter, Route, Routes } from 'react-router'
import { expect, it, vi } from 'vitest'
import { client } from '../api/client'
import * as sessionApi from '../auth/sessionApi'
import { createQueryClient } from '../query/client'
import { queryKeys } from '../query/keys'
import { Shell } from './Shell'
;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
it('refreshes shell revision through shared status invalidation and clears cache on logout', async () => {
  const qc = createQueryClient(); const host = document.createElement('div'); document.body.appendChild(host)
  const root = createRoot(host)
  const session = vi.spyOn(sessionApi, 'getSession').mockResolvedValue({ csrf: 'csrf', actor: { id: 'admin', class: 'ui-session' } })
  const logout = vi.spyOn(sessionApi, 'deleteSession').mockResolvedValue()
  let revision = 'sha256:111111111111111'
  const oldGet = vi.spyOn(sessionApi, 'getJSON').mockImplementation(async () => ({ ready: true, revisions: { runtimeRevision: revision } }))
  const get = vi.spyOn(client, 'GET').mockImplementation(async (path) => ({ data: path === '/v1/status' ? { ready: true, revisions: { runtimeRevision: revision } } : {}, response: new Response(null, { status: 200 }) }) as never)
  try {
    await act(async () => root.render(<QueryClientProvider client={qc}><MemoryRouter><Routes><Route path="/" element={<Shell />} /><Route path="/login" element={<p>Login</p>} /></Routes></MemoryRouter></QueryClientProvider>))
    await act(async () => { await vi.waitFor(() => expect(host.querySelector('.revision')?.getAttribute('title')).toBe(revision)) })
    revision = 'sha256:222222222222222'
    await act(async () => { await qc.invalidateQueries({ queryKey: queryKeys.status() }) })
    await act(async () => { await vi.waitFor(() => expect(host.querySelector('.revision')?.getAttribute('title')).toBe(revision)) })
    qc.setQueryData(queryKeys.audit(), ['private']); qc.setQueryData(queryKeys.session(), { scopes: ['dns.admin'] })
    await act(async () => [...host.querySelectorAll('button')].find((button) => button.textContent === 'Sign out')!.click())
    expect(logout).toHaveBeenCalledOnce(); expect(qc.getQueryCache().getAll().every((query) => query.state.data === undefined)).toBe(true)
  } finally { await act(async () => root.unmount()); qc.clear(); host.remove(); session.mockRestore(); logout.mockRestore(); oldGet.mockRestore(); get.mockRestore() }
})
