import { client, createLabdnsClient, throwOnError } from '../../api/client'
import { APIError } from '../../auth/sessionApi'

export type StateClient = Pick<typeof client, 'GET'>

function liveClient(): StateClient {
  return createLabdnsClient()
}

export type ExportFormat = 'yaml' | 'json'

export const EXPORT_FILES: Record<ExportFormat, { filename: string; mime: string }> = {
  yaml: { filename: 'labdns-state.yaml', mime: 'application/yaml' },
  json: { filename: 'labdns-state.json', mime: 'application/json' },
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

export async function fetchState(api: StateClient = liveClient()) {
  return throwOnError(await api.GET('/v1/state'))
}

export async function fetchStateExport(format: ExportFormat, api: StateClient = liveClient()): Promise<string> {
  if (format === 'yaml') {
    const body = throwOnError(
      await api.GET('/v1/state:export', {
        params: { query: { format: 'yaml' } },
        parseAs: 'text',
      }),
    )
    return typeof body === 'string' ? body : prettyJSON(body)
  }
  const body = throwOnError(
    await api.GET('/v1/state:export', {
      params: { query: { format: 'json' } },
    }),
  )
  return prettyJSON(body)
}

export function triggerBrowserDownload(filename: string, content: string, mime: string): void {
  const blob = new Blob([content], { type: mime })
  const href = URL.createObjectURL(blob)
  try {
    const a = document.createElement('a')
    a.href = href
    a.download = filename
    a.rel = 'noopener'
    document.body.appendChild(a)
    a.click()
    a.remove()
  } finally {
    URL.revokeObjectURL(href)
  }
}

export async function downloadStateExport(format: ExportFormat, api: StateClient = liveClient()): Promise<void> {
  const content = await fetchStateExport(format, api)
  const file = EXPORT_FILES[format]
  triggerBrowserDownload(file.filename, content, file.mime)
}
