import type { ConceptRow, ConceptState } from './types'

// Storage layer for the client-side concept corpus (localStorage, per user).
// Kept free of React/session imports so hooks/session.tsx can call the
// cleanup on logout without a circular dependency on the store module.

const STORAGE_PREFIX = 'apPsychConcepts:'
export const SNAPSHOT_VERSION = 2

export interface ConceptSnapshot {
  v: number
  contentVersion: number
  conceptCount: number
  stateVersion: number
  syncedAt: number
  rows: ConceptRow[]
}

export function defaultState(conceptId: string): ConceptState {
  return { conceptId, status: '', reviewCount: 0, shortTermReview: false, starred: false }
}

// Parsed-snapshot memo: initialData runs on every mount and re-parsing ~500KB
// each time would show. Invalidated on write/clear.
let snapshotMemo: { userId: string; snapshot: ConceptSnapshot | null } | null = null

export function readSnapshot(userId: string): ConceptSnapshot | null {
  if (snapshotMemo?.userId === userId) return snapshotMemo.snapshot
  let snapshot: ConceptSnapshot | null = null
  try {
    const raw = localStorage.getItem(STORAGE_PREFIX + userId)
    if (raw) {
      const parsed = JSON.parse(raw) as ConceptSnapshot
      if (parsed.v === SNAPSHOT_VERSION && Array.isArray(parsed.rows)) snapshot = parsed
    }
  } catch {
    snapshot = null
  }
  snapshotMemo = { userId, snapshot }
  return snapshot
}

export function writeSnapshot(userId: string, snapshot: ConceptSnapshot) {
  try {
    localStorage.setItem(STORAGE_PREFIX + userId, JSON.stringify(snapshot))
  } catch {
    // Quota exceeded or private mode: the cache stays memory-only this session.
  }
  snapshotMemo = { userId, snapshot }
}

// Logout / forced sign-out: drop every account's snapshot so a shared machine
// never serves one user's cached rows (or marked progress) to the next.
export function clearConceptSnapshots() {
  try {
    for (let i = localStorage.length - 1; i >= 0; i--) {
      const key = localStorage.key(i)
      if (key?.startsWith(STORAGE_PREFIX)) localStorage.removeItem(key)
    }
  } catch {
    // ignore
  }
  snapshotMemo = null
}
