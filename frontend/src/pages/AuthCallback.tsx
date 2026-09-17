import { useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router'
import { api } from '../lib/api'
import { useSession } from '../hooks/session'

// Receives the Entra sign-in result from the backend redirect fragment:
// /auth/callback#token=... (or #error=...).
export function AuthCallback() {
  const { t, refreshUser } = useSession()
  const navigate = useNavigate()
  const [error, setError] = useState('')
  const handled = useRef(false)

  useEffect(() => {
    if (handled.current) return
    handled.current = true
    const fragment = new URLSearchParams(window.location.hash.replace(/^#/, ''))
    const token = fragment.get('token')
    const failure = fragment.get('error')
    if (token) {
      api.setToken(token)
      void refreshUser().then(() => navigate('/', { replace: true }))
    } else {
      setError(failure || t.authFailed)
    }
  }, [t, refreshUser, navigate])

  if (error) {
    return (
      <main className="auth-page">
        <div className="auth-card center">
          <div className="error">{error}</div>
          <a className="secondary" href="/login">
            {t.signIn}
          </a>
        </div>
      </main>
    )
  }
  return (
    <main className="auth-page">
      <div className="auth-card center">
        <p className="muted">{t.signingIn}</p>
      </div>
    </main>
  )
}
