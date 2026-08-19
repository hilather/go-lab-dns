import { useEffect, useState } from 'react'
import { Outlet, useNavigate } from 'react-router'
import { APIError, deleteSession, getJSON, getSession } from '../auth/sessionApi'
import { NAV } from '../nav'
import { readyKind, readyLabel, shortRevision } from '../status'
import { Nav } from './Nav'

type StatusChrome = {
  ready?: boolean
  degraded?: boolean
  revisions?: { runtimeRevision?: string }
}

export function Shell() {
  const navigate = useNavigate()
  const [sessionOK, setSessionOK] = useState<boolean | null>(null)
  const [ready, setReady] = useState(false)
  const [degraded, setDegraded] = useState(false)
  const [revision, setRevision] = useState('')

  useEffect(() => {
    let cancelled = false
    void getSession()
      .then((sess) => {
        if (cancelled) {
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
        if (!cancelled) {
          setSessionOK(false)
          navigate('/login', { replace: true })
        }
      })
    return () => {
      cancelled = true
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
          const st = body as StatusChrome
          setReady(st.ready === true)
          setDegraded(st.degraded === true)
          setRevision(st.revisions?.runtimeRevision ?? '')
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

  const kind = readyKind(ready, degraded)
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
        <div className="emergency-slot" />
        <button type="button" onClick={() => void onSignOut()}>
          Sign out
        </button>
      </header>
      <Nav items={NAV} />
      <main className="shell-main">
        <Outlet />
      </main>
    </div>
  )
}
