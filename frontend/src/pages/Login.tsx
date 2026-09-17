import { useState } from 'react'
import { useNavigate } from 'react-router'
import { Loader2 } from 'lucide-react'
import { api } from '../lib/api'
import { useSession } from '../hooks/session'

export function Login() {
  const { t, onAuthed } = useSession()
  const navigate = useNavigate()
  const [mode, setMode] = useState<'login' | 'register'>('login')
  const [showLocal, setShowLocal] = useState(false)
  const [name, setName] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [inviteCode, setInviteCode] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  const entraEnabled = useSession().meta?.entra ?? false

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
          <div className="badge">AP Psychology</div>
          <h1>{t.authTitle}</h1>
          <p>{t.authCopy}</p>
        </div>
        <div className="auth-card">
          {entraEnabled && (
            <>
              <a className="primary entra-button" href="/api/auth/entra/login">
                <span className="ms-mark">_mC_</span> {t.microsoftSignIn}
              </a>
              {!showLocal && (
                <button className="link-button" onClick={() => setShowLocal(true)}>
                  {t.localLogin}
                </button>
              )}
            </>
          )}
          {(showLocal || !entraEnabled) && (
            <>
              <div className="segmented">
                <button className={mode === 'login' ? 'active' : ''} onClick={() => setMode('login')}>
                  {t.signIn}
                </button>
                <button className={mode === 'register' ? 'active' : ''} onClick={() => setMode('register')}>
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
                <input placeholder="student@tsinglan.org" value={email} onChange={(e) => setEmail(e.target.value)} />
              </label>
              <label>
                {t.password}
                <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} />
              </label>
              {error && <div className="error">{error}</div>}
              <button className="primary" disabled={busy} onClick={submit}>
                {busy ? <Loader2 className="spin" size={16} /> : null}
                {mode === 'register' ? t.createAccount : t.signIn}
              </button>
            </>
          )}
        </div>
      </section>
    </main>
  )
}
