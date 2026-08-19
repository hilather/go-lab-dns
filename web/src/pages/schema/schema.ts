import { client, createLabdnsClient, throwOnError } from '../../api/client'
import { APIError } from '../../auth/sessionApi'

export type SchemaClient = Pick<typeof client, 'GET'>

function liveClient(): SchemaClient {
  return createLabdnsClient()
}

export function prettyJSON(value: unknown): string {
  return JSON.stringify(value, null, 2)
}

export function problemFrom(err: unknown): { code?: string; detail?: string; message?: string } | null {
  if (err == null) {
    return null
  }
  if (err instanceof APIError) {
    return err
  }
  if (err instanceof Error) {
    return { message: err.message }
  }
  return { message: 'request failed' }
}

export async function fetchConfigSchema(api: SchemaClient = liveClient()): Promise<unknown> {
  return throwOnError(await api.GET('/v1/schema/config', { parseAs: 'json' }))
}
