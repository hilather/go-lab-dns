import { Route, Routes } from 'react-router'
import { Shell } from './components/Shell'
import { AuditEventPage } from './pages/audit/AuditEventPage'
import { AuditPage } from './pages/audit/AuditPage'
import { CachePage } from './pages/cache/CachePage'
import { CapabilitiesPage } from './pages/capabilities/CapabilitiesPage'
import { ChangesPage } from './pages/changes/ChangesPage'
import { ChaosPage } from './pages/chaos/ChaosPage'
import { ChaosPolicyPage } from './pages/chaos/ChaosPolicyPage'
import { DashboardPage } from './pages/dashboard/DashboardPage'
import { DocsPage } from './pages/docs/DocsPage'
import { ForwardingPage } from './pages/forwarding/ForwardingPage'
import { LoginPage } from './pages/login/LoginPage'
import { ResetPage } from './pages/reset/ResetPage'
import { ResolvePage } from './pages/resolve/ResolvePage'
import { SchemaPage } from './pages/schema/SchemaPage'
import { StatePage } from './pages/state/StatePage'
import { RecordDetailPage } from './pages/zones/RecordDetailPage'
import { ZoneDetailPage } from './pages/zones/ZoneDetailPage'
import { ZonesPage } from './pages/zones/ZonesPage'
import { ROUTES } from './routes'

export function App() {
  return (
    <Routes>
      <Route path={ROUTES.login} element={<LoginPage />} />
      <Route element={<Shell />}>
        <Route path={ROUTES.overview} element={<DashboardPage />} />
        <Route path={ROUTES.state} element={<StatePage />} />
        <Route path={ROUTES.changes} element={<ChangesPage />} />
        <Route path={ROUTES.zones} element={<ZonesPage />} />
        <Route path={ROUTES.zone} element={<ZoneDetailPage />} />
        <Route path={ROUTES.record} element={<RecordDetailPage />} />
        <Route path={ROUTES.resolve} element={<ResolvePage />} />
        <Route path={ROUTES.forwarding} element={<ForwardingPage />} />
        <Route path={ROUTES.cache} element={<CachePage />} />
        <Route path={ROUTES.chaos} element={<ChaosPage />} />
        <Route path={ROUTES.chaosPolicy} element={<ChaosPolicyPage />} />
        <Route path={ROUTES.audit} element={<AuditPage />} />
        <Route path={ROUTES.auditEvent} element={<AuditEventPage />} />
        <Route path={ROUTES.schema} element={<SchemaPage />} />
        <Route path={ROUTES.docsIndex} element={<DocsPage />} />
        <Route path={ROUTES.docs} element={<DocsPage />} />
        <Route path={ROUTES.capabilities} element={<CapabilitiesPage />} />
        <Route path={ROUTES.reset} element={<ResetPage />} />
      </Route>
    </Routes>
  )
}
