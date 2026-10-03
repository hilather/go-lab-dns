import { QueryClient } from '@tanstack/react-query'
import { APIError } from '../auth/sessionApi'

export function createQueryClient(): QueryClient {
  return new QueryClient({
    defaultOptions: {
      queries: {
        refetchOnWindowFocus: true,
        refetchIntervalInBackground: false,
        retry: (count, err) => {
          if (err instanceof APIError && (err.status === 401 || err.status === 403)) {
            return false
          }
          return count < 2
        },
      },
    },
  })
}

export const queryClient = createQueryClient()

// Cancel before clearing so ignored AbortSignals cannot repopulate another identity's cache.
export async function clearSessionQueries(client: QueryClient): Promise<void> {
  const cancelled = client.cancelQueries()
  client.clear()
  await cancelled
}
