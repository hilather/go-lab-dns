import { useEffect, useState, type FormEvent } from 'react'
import { useNavigate } from 'react-router'
import { APIError, createSession, getSession } from '../../auth/sessionApi'

export function isLoopbackHost(hostname: string): boolean {
  const h = hostname.replace(/^\[|\]$/g, '')
  if (h === 'localhost' || h === '::1' || h === '0:0:0:0:0:0:0:1') {
    return true
  }
  const mapped = /^::ffff:(\d+\.\d+\.\d+\.\d+)$/i.exec(h)
  const ipv4 = mapped?.[1] ?? h
  const parts = ipv4.split('.')
  if (parts.length !== 4) {
    return false
  }
  const nums = parts.map((p) => Number(p))
  if (nums.some((n) => !Number.isInteger(n) || n < 0 || n > 255)) {
    return false
  }
  return nums[0] === 127
}

export function LoginPage() {
  const navigate = useNavigate()
  const [token, setToken] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const [checking, setChecking] = useState(true)
  const loopback = isLoopbackHost(window.location.hostname)
  const disabled = checking || busy

  useEffect(() => {
    const ac = new AbortController()
    void getSession({ signal: ac.signal })
      .then((sess) => {
        if (ac.signal.aborted) {
          return
        }
        if (sess) {
          navigate('/', { replace: true })
          return
        }
        setChecking(false)
      })
      .catch(() => {
        if (!ac.signal.aborted) {
          setChecking(false)
        }
      })
    return () => {
      ac.abort()
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
      <div className="login-card surface">
        <div className="page-head">
          <div>
            <h1>LabDNS</h1>
            <p className="page-lede">Sign in to the operator console.</p>
          </div>
        </div>
        {error !== '' ? (
          <p role="alert" className="problem">
            {error}
          </p>
        ) : null}
        {loopback ? (
          <p>
            <button type="button" disabled={disabled} onClick={onContinue}>
              Continue as local administrator
            </button>
          </p>
        ) : null}
        <form className="stack-form" method="post" action="/login" onSubmit={onBearer}>
          <label>
            Bearer token
            <input
              type="password"
              autoComplete="off"
              value={token}
              disabled={disabled}
              onChange={(e) => setToken(e.target.value)}
            />
          </label>
          <button type="submit" className="btn-accent" disabled={disabled}>
            Sign in
          </button>
        </form>
      </div>
    </main>
  )
}
