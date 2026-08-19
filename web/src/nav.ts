import { ROUTES } from './routes'

export type NavItem = {
  to: string
  label: string
}

export const NAV: NavItem[] = [
  { to: ROUTES.overview, label: 'Overview' },
  { to: ROUTES.state, label: 'State' },
  { to: ROUTES.changes, label: 'Changes' },
  { to: ROUTES.zones, label: 'Zones' },
  { to: ROUTES.resolve, label: 'Resolve' },
  { to: ROUTES.forwarding, label: 'Forwarding' },
  { to: ROUTES.cache, label: 'Cache' },
  { to: ROUTES.chaos, label: 'Chaos' },
  { to: ROUTES.audit, label: 'Audit' },
  { to: ROUTES.schema, label: 'Schema' },
  { to: ROUTES.docsIndex, label: 'Docs' },
  { to: ROUTES.capabilities, label: 'Capabilities' },
]
