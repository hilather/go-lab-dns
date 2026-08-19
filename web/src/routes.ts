// Frozen SPA paths. Must match internal/capabilities UIBinding.Route
// (plus /login and list/detail paths the console actually navigates).

export const ROUTES = {
  login: '/login',
  overview: '/',
  state: '/state',
  changes: '/changes',
  zones: '/zones',
  zone: '/zones/:zoneId',
  record: '/zones/:zoneId/records/:recordId',
  resolve: '/resolve',
  forwarding: '/forwarding',
  cache: '/cache',
  chaos: '/chaos',
  chaosPolicy: '/chaos/:policyId',
  audit: '/audit',
  auditEvent: '/audit/:eventId',
  schema: '/schema',
  docsIndex: '/docs',
  docs: '/docs/:id',
  capabilities: '/capabilities',
  reset: '/reset',
} as const

export type RoutePath = (typeof ROUTES)[keyof typeof ROUTES]

// Unique UIBinding.Route values from the frozen catalog (docs/22 map).
export const UI_BINDING_ROUTES = [
  ROUTES.overview,
  ROUTES.capabilities,
  ROUTES.schema,
  ROUTES.state,
  ROUTES.changes,
  ROUTES.reset,
  ROUTES.zones,
  ROUTES.zone,
  ROUTES.resolve,
  ROUTES.forwarding,
  ROUTES.cache,
  ROUTES.chaos,
  ROUTES.chaosPolicy,
  ROUTES.audit,
  ROUTES.auditEvent,
  ROUTES.docs,
] as const
