import { StrictMode, type ReactNode } from 'react'
import { createRoot } from 'react-dom/client'
import {
  createBrowserRouter,
  createRoutesFromElements,
  Navigate,
  Route,
  RouterProvider,
} from 'react-router'
import { QueryClientProvider } from '@tanstack/react-query'
import { queryClient } from './lib/queryClient'
import { SessionProvider, useSession } from './hooks/session'
import { Shell } from './components/Shell'
import { Login } from './pages/Login'
import { AuthCallback } from './pages/AuthCallback'
import { Dashboard } from './pages/Dashboard'
import { Terms } from './pages/Terms'
import { Flashcards } from './pages/Flashcards'
import { Notes } from './pages/Notes'
import { Practice } from './pages/Practice'
import { PracticeRunner } from './pages/PracticeRunner'
import { Writing } from './pages/Writing'
import { WritingRunner } from './pages/WritingRunner'
import { WrongBook } from './pages/WrongBook'
import { Profile } from './pages/Profile'
import { AdminLayout } from './pages/admin/AdminLayout'
import { Questions } from './pages/admin/Questions'
import { Import } from './pages/admin/Import'
import { Sets } from './pages/admin/Sets'
import { SetEditor } from './pages/admin/SetEditor'
import { NoteResources } from './pages/admin/NoteResources'
import { Boards } from './pages/admin/Boards'
import { Analytics } from './pages/admin/Analytics'
import { Users } from './pages/admin/Users'
import { Content } from './pages/admin/Content'
import './styles.css'

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

// Data router (not <BrowserRouter>): useBlocker in the runner pages requires it.
const router = createBrowserRouter(
  createRoutesFromElements(
    <>
      <Route path="/login" element={<Login />} />
      <Route path="/auth/callback" element={<AuthCallback />} />
      <Route
        path="/practice/:setId/writing"
        element={
          <RequireAuth>
            <WritingRunner />
          </RequireAuth>
        }
      />
      <Route
        path="/"
        element={
          <RequireAuth>
            <Shell />
          </RequireAuth>
        }
      >
        <Route index element={<Dashboard />} />
        <Route path="terms" element={<Terms />} />
        <Route path="learn" element={<Navigate to="/terms" replace />} />
        <Route path="flashcards" element={<Flashcards />} />
        <Route path="notes" element={<Notes />} />
        <Route path="practice" element={<Practice />} />
        <Route path="practice/writing" element={<Writing />} />
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
          <Route path="sets/:setId" element={<SetEditor />} />
          <Route path="notes" element={<NoteResources />} />
          <Route path="boards" element={<Boards />} />
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
    </>,
  ),
)

createRoot(document.getElementById('app')!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <SessionProvider>
        <RouterProvider router={router} />
      </SessionProvider>
    </QueryClientProvider>
  </StrictMode>,
)
