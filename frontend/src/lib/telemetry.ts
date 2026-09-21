// Client-side telemetry: page views on route change and feature events.
// Events are batched briefly and sent with fetch keepalive so they survive
// navigation; failures are dropped silently — telemetry must never disturb
// the user or spam retries.

import { API_BASE, api } from './api'

type TrackType = 'page_view' | 'feature'

type PendingEvent = {
  type: TrackType
  name: string
  path?: string
  meta?: Record<string, unknown>
}

const FLUSH_DELAY_MS = 1500
const MAX_BATCH = 50

let queue: PendingEvent[] = []
let flushTimer: number | null = null

export function track(type: TrackType, name: string, meta?: Record<string, unknown>, path?: string) {
  if (!api.token || queue.length >= MAX_BATCH) return
  queue.push({ type, name, meta, path })
  if (flushTimer == null) flushTimer = window.setTimeout(flush, FLUSH_DELAY_MS)
}

function flush() {
  if (flushTimer != null) {
    window.clearTimeout(flushTimer)
    flushTimer = null
  }
  if (queue.length === 0) return
  const events = queue
  queue = []
  fetch(`${API_BASE}/api/telemetry/events`, {
    method: 'POST',
    keepalive: true,
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${api.token}` },
    body: JSON.stringify({ events }),
  }).catch(() => {
    // dropped on purpose
  })
}

// pageNameForPath maps a route to the telemetry page key. Admin and auth
// routes are excluded: staff usage is covered by server heartbeats, and
// logins are recorded server-side where they cannot be spoofed.
export function pageNameForPath(pathname: string): string | null {
  if (pathname.startsWith('/admin') || pathname.startsWith('/login') || pathname.startsWith('/auth')) return null
  if (pathname === '/') return 'page.dashboard'
  if (pathname.startsWith('/terms')) return 'page.terms'
  if (pathname.startsWith('/flashcards')) return 'page.flashcards'
  if (pathname.startsWith('/notes')) return 'page.notes'
  if (pathname.startsWith('/practice/writing')) return 'page.writing'
  if (pathname.startsWith('/practice/history')) return 'page.practice-history'
  if (/^\/practice\/[^/]+$/.test(pathname)) return 'page.practice-runner'
  if (pathname.startsWith('/practice')) return 'page.practice'
  if (pathname.startsWith('/wrongbook')) return 'page.wrongbook'
  if (pathname.startsWith('/profile')) return 'page.profile'
  return null
}

let lastTrackedPath = ''

// trackPageView reports a page_view once per distinct path (StrictMode mounts
// effects twice in dev; the guard also avoids double-counting remounts).
export function trackPageView(pathname: string) {
  if (pathname === lastTrackedPath) return
  const name = pageNameForPath(pathname)
  if (!name) {
    lastTrackedPath = pathname
    return
  }
  lastTrackedPath = pathname
  track('page_view', name, undefined, pathname)
}
