import { StrictMode, type ReactNode } from 'react'
import { createRoot } from 'react-dom/client'
import { BrowserRouter, Navigate, Route, Routes } from 'react-router'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { SessionProvider, useSession } from './hooks/session'
import { Shell } from './components/Shell'
import { Login } from './pages/Login'
import { AuthCallback } from './pages/AuthCallback'
import { Dashboard } from './pages/Dashboard'
import { Learn } from './pages/Learn'
import { Flashcards } from './pages/Flashcards'
import { Practice } from './pages/Practice'
import { PracticeRunner } from './pages/PracticeRunner'
import { WrongBook } from './pages/WrongBook'
import { Profile } from './pages/Profile'
import { AdminLayout } from './pages/admin/AdminLayout'
import { Questions } from './pages/admin/Questions'
import { Import } from './pages/admin/Import'
import { Sets } from './pages/admin/Sets'
import { Analytics } from './pages/admin/Analytics'
import { Users } from './pages/admin/Users'
import { Content } from './pages/admin/Content'
import './styles.css'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: { retry: 1, refetchOnWindowFocus: false, staleTime: 30_000 },
  },
})

function RequireAuth({ children }: { children: ReactNode }) {
  const { ready, user } = useSession()
  if (!ready) {
    return (
      <main className="auth-page">
        <p className="muted">…</p>
      </main>
    )
  }
  if (!user) return <Navigate to="/login" replace />
  return <>{children}</>
}

function RequireStaff({ children }: { children: ReactNode }) {
  const { isStaff } = useSession()
  if (!isStaff) return <Navigate to="/" replace />
  return <>{children}</>
}

function RequireAdmin({ children }: { children: ReactNode }) {
  const { isAdmin } = useSession()
  if (!isAdmin) return <Navigate to="/admin/analytics" replace />
  return <>{children}</>
}

createRoot(document.getElementById('app')!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <SessionProvider>
        <BrowserRouter>
          <Routes>
            <Route path="/login" element={<Login />} />
            <Route path="/auth/callback" element={<AuthCallback />} />
            <Route
              path="/"
              element={
                <RequireAuth>
                  <Shell />
                </RequireAuth>
              }
            >
              <Route index element={<Dashboard />} />
              <Route path="learn" element={<Learn />} />
              <Route path="flashcards" element={<Flashcards />} />
              <Route path="practice" element={<Practice />} />
              <Route path="practice/:setId" element={<PracticeRunner />} />
              <Route path="wrongbook" element={<WrongBook />} />
              <Route path="profile" element={<Profile />} />
              <Route
                path="admin"
                element={
                  <RequireStaff>
                    <AdminLayout />
                  </RequireStaff>
                }
              >
                <Route index element={<Navigate to="/admin/analytics" replace />} />
                <Route path="analytics" element={<Analytics />} />
                <Route path="questions" element={<Questions />} />
                <Route path="import" element={<Import />} />
                <Route path="sets" element={<Sets />} />
                <Route
                  path="users"
                  element={
                    <RequireAdmin>
                      <Users />
                    </RequireAdmin>
                  }
                />
                <Route
                  path="content"
                  element={
                    <RequireAdmin>
                      <Content />
                    </RequireAdmin>
                  }
                />
              </Route>
            </Route>
            <Route path="*" element={<Navigate to="/" replace />} />
          </Routes>
        </BrowserRouter>
      </SessionProvider>
    </QueryClientProvider>
  </StrictMode>,
)
