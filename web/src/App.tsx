import { Route, Routes } from 'react-router'
import { Shell } from './components/Shell'
import { DashboardPage } from './pages/dashboard/DashboardPage'
import { LoginPage } from './pages/login/LoginPage'

export function App() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route element={<Shell />}>
        <Route path="/" element={<DashboardPage />} />
      </Route>
    </Routes>
  )
}
