import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './styles/app.css'

const root = document.getElementById('root')
if (!root) {
  throw new Error('root element missing')
}

createRoot(root).render(
  <StrictMode>
    <p>LabDNS operator console</p>
  </StrictMode>,
)
