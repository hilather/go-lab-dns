import { ROUTES } from './routes'

export type NavItem = {
  to: string
  label: string
}

export type NavGroup = {
  id: string
  label: string
  items: readonly NavItem[]
}

export const NAV_GROUPS: readonly NavGroup[] = [
  {
    id: 'inspect',
    label: 'Inspect',
    items: [
      { to: ROUTES.overview, label: 'Overview' },
      { to: ROUTES.zones, label: 'Zones' },
      { to: ROUTES.resolve, label: 'Resolve' },
      { to: ROUTES.forwarding, label: 'Forwarding' },
      { to: ROUTES.cache, label: 'Cache' },
      { to: ROUTES.chaos, label: 'Chaos' },
      { to: ROUTES.audit, label: 'Audit' },
    ],
  },
  {
    id: 'mutate',
    label: 'Mutate',
    items: [
      { to: ROUTES.changes, label: 'Changes' },
      { to: ROUTES.state, label: 'State' },
      { to: ROUTES.reset, label: 'Reset' },
    ],
  },
  {
    id: 'ref',
    label: 'Ref',
    items: [
      { to: ROUTES.schema, label: 'Schema' },
      { to: ROUTES.docsIndex, label: 'Docs' },
      { to: ROUTES.capabilities, label: 'Capabilities' },
    ],
  },
]

export const NAV: NavItem[] = NAV_GROUPS.flatMap((group) => [...group.items])
