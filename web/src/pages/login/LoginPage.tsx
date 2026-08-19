import { useEffect, useState, type FormEvent } from 'react'
import { useNavigate } from 'react-router'
import { APIError, createSession, getSession } from '../../auth/sessionApi'

export function isLoopbackHost(hostname: string): boolean {
  const h = hostname.replace(/^\[|\]$/g, '')
  return h === 'localhost' || h === '127.0.0.1' || h === '::1' || h === '0:0:0:0:0:0:0:1'
}

export function LoginPage() {
  const navigate = useNavigate()
  const [token, setToken] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const loopback = isLoopbackHost(window.location.hostname)

  useEffect(() => {
    let cancelled = false
    void getSession().then((sess) => {
      if (!cancelled && sess) {
        navigate('/', { replace: true })
      }
    })
    return () => {
      cancelled = true
    }
  }, [navigate])

  async function finishLogin(run: () => Promise<unknown>) {
    setBusy(true)
    setError('')
    try {
      await run()
      navigate('/', { replace: true })
    } catch (err) {
      if (err instanceof APIError) {
        setError(err.detail || err.code)
      } else {
        setError('sign-in failed')
      }
    } finally {
      setBusy(false)
    }
  }

  function onContinue() {
    void finishLogin(() => createSession())
  }

  function onBearer(ev: FormEvent<HTMLFormElement>) {
    ev.preventDefault()
    const bearer = token
    setToken('')
    if (bearer.trim() === '') {
      setError('bearer token is required')
      return
    }
    void finishLogin(() => createSession(bearer))
  }

  return (
    <main className="login">
      <h1>LabDNS</h1>
      <p>Sign in to the operator console.</p>
      {error !== '' ? (
        <p role="alert" className="problem">
          {error}
        </p>
      ) : null}
      {loopback ? (
        <p>
          <button type="button" disabled={busy} onClick={onContinue}>
            Continue as local administrator
          </button>
        </p>
      ) : null}
      <form onSubmit={onBearer}>
        <label>
          Bearer token
          <input
            type="password"
            name="token"
            autoComplete="off"
            value={token}
            disabled={busy}
            onChange={(e) => setToken(e.target.value)}
          />
        </label>
        <button type="submit" disabled={busy}>
          Sign in
        </button>
      </form>
    </main>
  )
}
