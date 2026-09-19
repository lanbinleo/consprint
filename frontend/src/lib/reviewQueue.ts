import { useEffect, useState } from 'react'
import { API_BASE, api } from './api'
import { queryClient } from './queryClient'

export type ReviewResponse = 'proficient' | 'fuzzy' | 'unknown'

export type PendingReviewEvent = {
  conceptId: string
  response: ReviewResponse
  durationMs?: number
}

type Listener = (pending: number) => void

// Local queue for flashcard self-assessments. Marking a card must feel
// instant, so events are applied optimally in the UI, buffered here, and
// flushed to POST /api/review/events/batch in the background with retry.
// A keepalive fetch on pagehide delivers whatever is left when the tab closes.
class ReviewQueue {
  private events: PendingReviewEvent[] = []
  private listeners = new Set<Listener>()
  private timer: number | null = null
  private retryCount = 0
  private flushing = false

  get pendingCount() {
    return this.events.length
  }

  subscribe(listener: Listener): () => void {
    this.listeners.add(listener)
    listener(this.events.length)
    return () => {
      this.listeners.delete(listener)
    }
  }

  private notify() {
    for (const listener of this.listeners) listener(this.events.length)
  }

  private scheduleFlush(delay: number) {
    if (this.timer != null) window.clearTimeout(this.timer)
    this.timer = window.setTimeout(() => {
      this.timer = null
      void this.flush()
    }, delay)
  }

  enqueue(event: PendingReviewEvent) {
    this.events.push(event)
    this.notify()
    // Short debounce so a quick run of marks leaves as one batch.
    this.scheduleFlush(400)
  }

  async flush() {
    if (this.flushing || this.events.length === 0) return
    this.flushing = true
    try {
      const batch = this.events.slice(0, 200)
      await api.request('/api/review/events/batch', {
        method: 'POST',
        body: JSON.stringify({ events: batch }),
      })
      this.events.splice(0, batch.length)
      this.retryCount = 0
      this.notify()
      void queryClient.invalidateQueries({ queryKey: ['dashboard'] })
      void queryClient.invalidateQueries({ queryKey: ['units'] })
      if (this.events.length > 0) this.scheduleFlush(50)
    } catch {
      this.retryCount += 1
      this.scheduleFlush(Math.min(2000 * this.retryCount, 30_000))
    } finally {
      this.flushing = false
    }
  }

  // Best-effort delivery when the page is going away: keepalive lets the
  // request outlive the unload. Delivered events are dropped from the queue;
  // if the request still fails they are simply lost (review marks are
  // re-markable, append-only analytics tolerate gaps).
  flushOnExit() {
    if (this.events.length === 0) return
    const headers: Record<string, string> = { 'Content-Type': 'application/json' }
    if (api.token) headers.Authorization = `Bearer ${api.token}`
    void fetch(`${API_BASE}/api/review/events/batch`, {
      method: 'POST',
      headers,
      body: JSON.stringify({ events: this.events }),
      keepalive: true,
    })
    this.events = []
    this.notify()
  }
}

export const reviewQueue = new ReviewQueue()

window.addEventListener('pagehide', () => reviewQueue.flushOnExit())

export function usePendingReviewCount() {
  const [count, setCount] = useState(reviewQueue.pendingCount)
  useEffect(() => reviewQueue.subscribe(setCount), [])
  return count
}
