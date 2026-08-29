import { NavLink } from 'react-router'
import type { NavGroup } from '../nav'

export function Nav({ groups }: { groups: readonly NavGroup[] }) {
  return (
    <nav aria-label="Primary">
      <div className="nav-groups">
        {groups.map((group) => (
          <div key={group.id} className="nav-group">
            <p className="nav-group-label">{group.label}</p>
            <ul className="nav-list">
              {group.items.map((item) => (
                <li key={item.to}>
                  <NavLink to={item.to} end={item.to === '/'}>
                    {item.label}
                  </NavLink>
                </li>
              ))}
            </ul>
          </div>
        ))}
      </div>
    </nav>
  )
}
