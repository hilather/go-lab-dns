import { expect, it } from 'vitest'
import { clearSessionQueries, createQueryClient } from './client'
import { queryKeys } from './keys'

it('clears all session data and prevents ignored cancellation from repopulating it', async () => {
  const client = createQueryClient()
  client.setQueryData(queryKeys.audit(), ['private-admin-event'])
  client.setQueryData(queryKeys.session(), { scopes: ['dns.admin'] })
  let finish!: (value: unknown) => void
  const request = client.fetchQuery({ queryKey: queryKeys.state('r1'), queryFn: () => new Promise((resolve) => { finish = resolve }) })
  const cancelled = request.catch(() => undefined)
  await clearSessionQueries(client)
  expect(client.getQueryCache().getAll()).toHaveLength(0)
  finish({ private: 'old-admin-state' })
  await cancelled
  await Promise.resolve()
  expect(client.getQueryCache().getAll()).toHaveLength(0)
})
