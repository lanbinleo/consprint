import { useEffect, useState } from 'react'
import { Save, Upload } from 'lucide-react'
import { api } from '../lib/api'
import { useSession } from '../hooks/session'
import { Avatar, Header } from '../components/ui'
import type { AuthPayload } from '../lib/types'

export function Profile() {
  const { t, user, refreshUser, theme, setTheme, lang, setLang } = useSession()
  const [name, setName] = useState(user?.name ?? '')
  const [avatarDataUrl, setAvatarDataUrl] = useState(user?.avatarDataUrl ?? '')
  const [saved, setSaved] = useState(false)
  const [error, setError] = useState('')

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
    const reader = new FileReader()
    reader.onload = () => setAvatarDataUrl(String(reader.result ?? ''))
    reader.readAsDataURL(file)
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
