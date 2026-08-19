import { client, createLabdnsClient, throwOnError } from '../../api/client'
import { APIError } from '../../auth/sessionApi'

export type CapabilitiesClient = Pick<typeof client, 'GET'>

function liveClient(): CapabilitiesClient {
  return createLabdnsClient()
}

export type CapabilityInfo = {
  name?: string
  version?: string
  description?: string
  mutating?: boolean
  idempotent?: boolean
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

export function capabilityList(data: unknown): CapabilityInfo[] {
  if (!data || typeof data !== 'object') {
    return []
  }
  const raw = (data as { capabilities?: unknown }).capabilities
  if (!Array.isArray(raw)) {
    return []
  }
  return raw.filter((row): row is CapabilityInfo => row != null && typeof row === 'object')
}

export async function fetchCapabilities(api: CapabilitiesClient = liveClient()) {
  return throwOnError(await api.GET('/v1/capabilities'))
}
