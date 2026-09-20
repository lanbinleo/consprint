import { useEffect, useState } from 'react'
import { Save, Upload } from 'lucide-react'
import { api } from '../lib/api'
import { useSession } from '../hooks/session'
import { Avatar, Header, SpinnerButton } from '../components/ui'
import type { AuthPayload } from '../lib/types'

// Matches the backend's data-URL cap (~260k chars ≈ 190 KB binary).
const AVATAR_MAX_BYTES = 190 * 1024

export function Profile() {
  const { t, user, refreshUser, passwordSet, theme, setTheme, lang, setLang } = useSession()
  const [name, setName] = useState(user?.name ?? '')
  const [avatarDataUrl, setAvatarDataUrl] = useState(user?.avatarDataUrl ?? '')
  const [saved, setSaved] = useState(false)
  const [error, setError] = useState('')
  const [currentPassword, setCurrentPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [pwBusy, setPwBusy] = useState(false)
  const [pwSaved, setPwSaved] = useState(false)
  const [pwError, setPwError] = useState('')

  useEffect(() => {
    setName(user?.name ?? '')
    setAvatarDataUrl(user?.avatarDataUrl ?? '')
  }, [user])

  async function save() {
    setError('')
    try {
      await api.request<Omit<AuthPayload, 'token'>>('/api/me', {
        method: 'PATCH',
        body: JSON.stringify({ name, avatarDataUrl }),
      })
      await refreshUser()
      setSaved(true)
      setTimeout(() => setSaved(false), 1200)
    } catch (err) {
      setError(err instanceof Error ? err.message : t.errorGeneric)
    }
  }

  function loadAvatar(file?: File) {
    if (!file) return
    if (file.size > AVATAR_MAX_BYTES) {
      setError(t.avatarTooLarge)
      return
    }
    const reader = new FileReader()
    reader.onload = () => setAvatarDataUrl(String(reader.result ?? ''))
    reader.readAsDataURL(file)
  }

  // Entra accounts without a password set one directly (that also enables
  // email+password sign-in); afterwards, and for local accounts, changing
  // requires the current password.
  async function savePassword() {
    setPwError('')
    if (newPassword.length < 8) {
      setPwError(t.passwordTooShort)
      return
    }
    setPwBusy(true)
    try {
      await api.request('/api/me/password', {
        method: 'PATCH',
        body: JSON.stringify({ currentPassword, newPassword }),
      })
      setCurrentPassword('')
      setNewPassword('')
      setPwSaved(true)
      setTimeout(() => setPwSaved(false), 1500)
      await refreshUser()
    } catch (err) {
      setPwError(err instanceof Error ? err.message : t.errorGeneric)
    } finally {
      setPwBusy(false)
    }
  }

  return (
    <section className="page">
      <Header eyebrow={t.profile} title={t.profileTitle} />
      <div className="settings-grid">
        <div className="settings-card">
          <h3>{t.userCenter}</h3>
          <div className="avatar-upload">
            <Avatar user={{ name: name || 'AP', avatarDataUrl }} />
            <label className="secondary upload-button">
              <Upload size={16} /> {t.uploadAvatar}
              <input type="file" accept="image/*" onChange={(e) => loadAvatar(e.target.files?.[0])} />
            </label>
          </div>
          <label>
            {t.name}
            <input value={name} onChange={(e) => setName(e.target.value)} />
          </label>
          <label>
            {t.email}
            <input value={user?.email ?? ''} disabled readOnly />
            <small className="field-hint">{t.emailLocked}</small>
          </label>
          {error && <div className="error">{error}</div>}
          <button className="primary" onClick={() => void save()}>
            <Save size={16} /> {saved ? t.saved : t.saveProfile}
          </button>
        </div>
        <div className="settings-card">
          <h3>{t.accountSecurity}</h3>
          {!passwordSet && <p className="muted">{t.passwordSetHint}</p>}
          {passwordSet && (
            <label>
              {t.currentPassword}
              <input type="password" value={currentPassword} onChange={(e) => setCurrentPassword(e.target.value)} autoComplete="current-password" />
            </label>
          )}
          <label>
            {t.newPassword}
            <input
              type="password"
              value={newPassword}
              onChange={(e) => setNewPassword(e.target.value)}
              autoComplete="new-password"
            />
          </label>
          {pwError && <div className="error">{pwError}</div>}
          <SpinnerButton busy={pwBusy} onClick={() => void savePassword()}>
            <Save size={16} /> {pwSaved ? t.passwordUpdated : passwordSet ? t.changePassword : t.setPassword}
          </SpinnerButton>
        </div>
        <div className="settings-card">
          <h3>{t.settings}</h3>
          <div className="settings-row">
            <span>{t.theme}</span>
            <div className="segmented inline">
              <button className={theme === 'light' ? 'active' : ''} onClick={() => setTheme('light')}>
                {t.light}
              </button>
              <button className={theme === 'dark' ? 'active' : ''} onClick={() => setTheme('dark')}>
                {t.dark}
              </button>
            </div>
          </div>
          <div className="settings-row">
            <span>{t.language}</span>
            <div className="segmented inline">
              <button className={lang === 'en' ? 'active' : ''} onClick={() => setLang('en')}>
                English
              </button>
              <button className={lang === 'zh' ? 'active' : ''} onClick={() => setLang('zh')}>
                中文
              </button>
            </div>
          </div>
          <div className="settings-row">
            <span>{t.provider}</span>
            <strong>{user?.provider === 'entra' ? t.providerEntra : t.providerLocal}</strong>
          </div>
        </div>
      </div>
    </section>
  )
}
