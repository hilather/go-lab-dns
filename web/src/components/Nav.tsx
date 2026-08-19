import { NavLink } from 'react-router'
import type { NavItem } from '../nav'

export function Nav({ items }: { items: readonly NavItem[] }) {
  return (
    <nav aria-label="Primary">
      <ul className="nav-list">
        {items.map((item) => (
          <li key={item.to}>
            <NavLink to={item.to} end={item.to === '/'}>
              {item.label}
            </NavLink>
          </li>
        ))}
      </ul>
    </nav>
  )
}
