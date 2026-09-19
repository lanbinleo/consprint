import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'
import { api, ApiError } from '../lib/api'
import { queryClient } from '../lib/queryClient'
import { getCopy, detectLang, type Copy } from '../lib/i18n'
import type { AppMeta, AuthPayload, Lang, User } from '../lib/types'

type Theme = 'light' | 'dark'

type SessionValue = {
  ready: boolean
  user: User | null
  tenant: { id: string; name: string } | null
  meta: AppMeta | null
  isStaff: boolean
  isAdmin: boolean
  lang: Lang
  theme: Theme
  t: Copy
  setLang: (lang: Lang) => void
  setTheme: (theme: Theme) => void
  onAuthed: (payload: AuthPayload) => void
  refreshUser: () => Promise<void>
  logout: () => void
}

const SessionContext = createContext<SessionValue | null>(null)

export function SessionProvider({ children }: { children: ReactNode }) {
  const [ready, setReady] = useState(false)
  const [user, setUser] = useState<User | null>(null)
  const [tenant, setTenant] = useState<{ id: string; name: string } | null>(null)
  const [meta, setMeta] = useState<AppMeta | null>(null)
  const [lang, setLangState] = useState<Lang>(detectLang)
  const [theme, setThemeState] = useState<Theme>(() => {
    const stored = localStorage.getItem('apPsychTheme')
    if (stored === 'light' || stored === 'dark') return stored
    // Follow the system when the user has never picked a theme.
    return window.matchMedia?.('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
  })

  useEffect(() => {
    document.documentElement.dataset.theme = theme
    localStorage.setItem('apPsychTheme', theme)
  }, [theme])

  useEffect(() => {
    localStorage.setItem('apPsychLang', lang)
  }, [lang])

  useEffect(() => {
    api
      .request<AppMeta>('/api/meta')
      .then(setMeta)
      .catch(() => setMeta({ entra: false, examDate: '', timezone: 'Asia/Shanghai' }))
  }, [])

  const refreshUser = useCallback(async () => {
    if (!api.token) {
      setReady(true)
      return
    }
    try {
      const payload = await api.request<Omit<AuthPayload, 'token'>>('/api/me')
      setUser(payload.user)
      setTenant(payload.tenant)
    } catch (err) {
      // Only a rejected token ends the session; transient network/backend errors keep the user.
      if (err instanceof ApiError && err.status === 401) {
        api.logout()
        setUser(null)
        setTenant(null)
        // Drop cached queries: on a shared machine the next login may be a
        // different user, and fresh-but-wrong cached data would flash.
        queryClient.clear()
      }
    } finally {
      setReady(true)
    }
  }, [])

  useEffect(() => {
    void refreshUser()
  }, [refreshUser])

  const value = useMemo<SessionValue>(() => {
    const role = user?.role ?? 'student'
    return {
      ready,
      user,
      tenant,
      meta,
      isStaff: role === 'teacher' || role === 'admin',
      isAdmin: role === 'admin',
      lang,
      theme,
      t: getCopy(lang),
      setLang: setLangState,
      setTheme: setThemeState,
      onAuthed: (payload) => {
        api.setToken(payload.token)
        // A new sign-in may be a different user on the same browser: never
        // serve the previous account's cached queries.
        queryClient.clear()
        setUser(payload.user)
        setTenant(payload.tenant)
      },
      refreshUser,
      logout: () => {
        api.logout()
        setUser(null)
        setTenant(null)
        queryClient.clear()
      },
    }
  }, [ready, user, tenant, meta, lang, theme, refreshUser])

  return <SessionContext.Provider value={value}>{children}</SessionContext.Provider>
}

export function useSession(): SessionValue {
  const context = useContext(SessionContext)
  if (!context) throw new Error('useSession must be used inside SessionProvider')
  return context
}
