import { useEffect, useState } from 'react'
import { Outlet, useNavigate } from 'react-router'
import { APIError, deleteSession, getJSON, getSession } from '../auth/sessionApi'
import { NAV } from '../nav'
import { readyKind, readyLabel, shortRevision, type StatusView } from '../status'
import { EmergencyControl } from '../pages/chaos/EmergencyControl'
import { Nav } from './Nav'

export type ShellContext = {
  status: StatusView | null
}

function asStatus(v: unknown): StatusView {
  if (v && typeof v === 'object') {
    return v as StatusView
  }
  return {}
}

export function Shell() {
  const navigate = useNavigate()
  const [sessionOK, setSessionOK] = useState<boolean | null>(null)
  const [status, setStatus] = useState<StatusView | null>(null)

  useEffect(() => {
    const ac = new AbortController()
    void getSession({ signal: ac.signal })
      .then((sess) => {
        if (ac.signal.aborted) {
          return
        }
        if (!sess) {
          setSessionOK(false)
          navigate('/login', { replace: true })
          return
        }
        setSessionOK(true)
      })
      .catch(() => {
        if (!ac.signal.aborted) {
          setSessionOK(false)
          navigate('/login', { replace: true })
        }
      })
    return () => {
      ac.abort()
    }
  }, [navigate])

  useEffect(() => {
    if (!sessionOK) {
      return
    }
    let cancelled = false
    const load = () => {
      if (document.visibilityState !== 'visible') {
        return
      }
      void getJSON('/v1/status')
        .then((body) => {
          if (cancelled) {
            return
          }
          setStatus(asStatus(body))
        })
        .catch((err: unknown) => {
          if (cancelled) {
            return
          }
          if (err instanceof APIError && err.status === 401) {
            setSessionOK(false)
            navigate('/login', { replace: true })
          }
        })
    }
    load()
    const id = window.setInterval(load, 2000)
    const onVis = () => {
      if (document.visibilityState === 'visible') {
        load()
      }
    }
    document.addEventListener('visibilitychange', onVis)
    return () => {
      cancelled = true
      window.clearInterval(id)
      document.removeEventListener('visibilitychange', onVis)
    }
  }, [navigate, sessionOK])

  async function onSignOut() {
    try {
      await deleteSession()
    } finally {
      navigate('/login', { replace: true })
    }
  }

  if (sessionOK !== true) {
    return <p className="loading">Loading LabDNS…</p>
  }

  const kind = readyKind(status?.ready, status?.degraded)
  const revision = status?.revisions?.runtimeRevision ?? ''
  return (
    <div className="shell">
      <header className="shell-header">
        <p className="product">LabDNS</p>
        <p className="revision" title={revision || undefined}>
          Revision {shortRevision(revision)}
        </p>
        <p className={`status status-${kind}`}>
          <span className="status-symbol" aria-hidden="true">
            {kind === 'ready' ? '●' : kind === 'degraded' ? '▲' : '○'}
          </span>{' '}
          {readyLabel(kind)}
        </p>
        <div className="emergency-slot">
          <EmergencyControl emergencyDisabled={status?.chaos?.emergencyDisabled} />
        </div>
        <button type="button" onClick={() => void onSignOut()}>
          Sign out
        </button>
      </header>
      <Nav items={NAV} />
      <main className="shell-main">
        <Outlet context={{ status } satisfies ShellContext} />
      </main>
    </div>
  )
}
