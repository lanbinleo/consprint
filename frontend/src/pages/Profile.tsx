import { useEffect, useState } from 'react'
import { Save, Upload } from 'lucide-react'
import { api } from '../lib/api'
import { useSession } from '../hooks/session'
import { Avatar, Header } from '../components/ui'
import type { AuthPayload } from '../lib/types'

export function Profile() {
  const { t, user, refreshUser } = useSession()
  const [name, setName] = useState(user?.name ?? '')
  const [avatarDataUrl, setAvatarDataUrl] = useState(user?.avatarDataUrl ?? '')
  const [saved, setSaved] = useState(false)

  useEffect(() => {
    setName(user?.name ?? '')
    setAvatarDataUrl(user?.avatarDataUrl ?? '')
  }, [user])

  async function save() {
    await api.request<Omit<AuthPayload, 'token'>>('/api/me', {
      method: 'PATCH',
      body: JSON.stringify({ name, avatarDataUrl }),
    })
    await refreshUser()
    setSaved(true)
    setTimeout(() => setSaved(false), 1200)
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
            <input value={user?.email ?? ''} disabled />
          </label>
          <button className="primary" onClick={save}>
            <Save size={16} /> {saved ? t.saved : t.saveProfile}
          </button>
        </div>
        <div className="settings-card">
          <h3>{t.settings}</h3>
          <p className="muted">
            {t.theme} / {t.language} — {t.collapse}
          </p>
          <p className="muted">
            {t.role}: {user?.role} · {user?.provider === 'entra' ? t.providerEntra : t.providerLocal}
          </p>
        </div>
      </div>
    </section>
  )
}
