import { useState } from 'react'
import { useNavigate } from 'react-router'
import { Loader2 } from 'lucide-react'
import { api } from '../lib/api'
import { useSession } from '../hooks/session'
import { Logo } from '../components/Logo'

export function Login() {
  const { t, meta, onAuthed } = useSession()
  const navigate = useNavigate()
  const [mode, setMode] = useState<'login' | 'register'>('login')
  const [showLocal, setShowLocal] = useState(false)
  const [name, setName] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [inviteCode, setInviteCode] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  const entraEnabled = meta?.entra ?? false

  async function submit() {
    setBusy(true)
    setError('')
    try {
      const payload =
        mode === 'register' ? await api.register({ name, email, password, inviteCode }) : await api.login({ email, password })
      onAuthed(payload)
      navigate('/')
    } catch (err) {
      setError(err instanceof Error ? err.message : t.authFailed)
    } finally {
      setBusy(false)
    }
  }

  return (
    <main className="auth-page">
      <section className="auth-panel">
        <div className="auth-copy">
          <div className="auth-brand">
            <div className="brand-mark">
              <Logo size={40} />
            </div>
            <strong>Psych Hub</strong>
          </div>
          <div className="auth-greet">{t.authBubble}</div>
          <h1>{t.authTitle}</h1>
          <p>{t.authCopy}</p>
        </div>
        <div className="auth-card">
          {entraEnabled && (
            <>
              <a className="entra-button" href="/api/auth/entra/login">
                {/* Official Microsoft four-square brand mark */}
                <svg className="ms-mark" width="18" height="18" viewBox="0 0 23 23" aria-hidden="true">
                  <path fill="#f25022" d="M1 1h10v10H1z" />
                  <path fill="#7fba00" d="M12 1h10v10H12z" />
                  <path fill="#00a4ef" d="M1 12h10v10H1z" />
                  <path fill="#ffb900" d="M12 12h10v10H12z" />
                </svg>
                {t.microsoftSignIn}
              </a>
              {!showLocal && (
                <button className="link-button" onClick={() => setShowLocal(true)}>
                  {t.localLogin}
                </button>
              )}
            </>
          )}
          {(showLocal || !entraEnabled) && (
            <form
              className="auth-form"
              onSubmit={(event) => {
                event.preventDefault()
                void submit()
              }}
            >
              <div className="segmented">
                <button type="button" className={mode === 'login' ? 'active' : ''} onClick={() => setMode('login')}>
                  {t.signIn}
                </button>
                <button type="button" className={mode === 'register' ? 'active' : ''} onClick={() => setMode('register')}>
                  {t.register}
                </button>
              </div>
              {mode === 'register' && (
                <>
                  <label>
                    {t.name}
                    <input placeholder="Student" value={name} onChange={(e) => setName(e.target.value)} />
                  </label>
                  <label>
                    {t.inviteCode}
                    <input value={inviteCode} onChange={(e) => setInviteCode(e.target.value)} />
                  </label>
                </>
              )}
              <label>
                {t.email}
                <input type="email" placeholder="student@tsinglan.org" value={email} onChange={(e) => setEmail(e.target.value)} />
              </label>
              <label>
                {t.password}
                <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} />
              </label>
              {error && <div className="error">{error}</div>}
              <button type="submit" className="primary" disabled={busy}>
                {busy ? <Loader2 className="spin" size={16} /> : null}
                {mode === 'register' ? t.createAccount : t.signIn}
              </button>
            </form>
          )}
        </div>
      </section>
    </main>
  )
}
