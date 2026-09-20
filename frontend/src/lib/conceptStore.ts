import { useMemo } from 'react'
import { useQuery, type QueryClient } from '@tanstack/react-query'
import { api } from './api'
import { useSession } from '../hooks/session'
import { useUnits } from '../components/ScopePicker'
import { defaultState, readSnapshot, writeSnapshot, SNAPSHOT_VERSION, type ConceptSnapshot } from './conceptSnapshot'
import type { Concept, ConceptRow, ConceptState, ContentVersion, Topic, Unit } from './types'

// The concept corpus is ~794 rows / a few hundred KB — the heaviest payload in
// the app. This store keeps it in one query backed by localStorage: reloads
// paint from the snapshot instantly, then a ~200B version check decides
// whether anything needs refetching (usually nothing). Content changes arrive
// as an updatedSince delta; progress made on another device arrives via the
// slim states endpoint. Marks never touch this store's network path — they
// patch the cache in place and flush through the existing queue/PATCH flows.

export const conceptsKey = ['concepts'] as const

export interface SyncDeps {
  getVersion: () => Promise<ContentVersion>
  fetchFull: () => Promise<Concept[]>
  fetchDelta: (sinceVersion: number) => Promise<Concept[]>
  fetchStates: () => Promise<ConceptState[]>
}

// The sync algorithm, kept pure so the cache behaviour is unit-testable:
// version check first, then delta-or-full for content, then states.
export async function syncConcepts(local: ConceptSnapshot | null, deps: SyncDeps): Promise<ConceptSnapshot> {
  const version = await deps.getVersion()
  const hasBase = !!local && local.rows.length > 0
  let rows: ConceptRow[]
  if (!hasBase || !local) {
    rows = (await deps.fetchFull()).map((concept) => ({ ...concept, state: defaultState(concept.id) }))
  } else if (local.contentVersion === version.contentVersion && local.rows.length === version.conceptCount) {
    // Content unchanged: zero list bytes transferred.
    rows = local.rows
  } else {
    // Incremental: upsert changed rows onto the cached corpus. A changed row
    // keeps its cached state — state sync below owns that field.
    const changed = await deps.fetchDelta(local.contentVersion)
    const byId = new Map(local.rows.map((row) => [row.id, row]))
    for (const concept of changed) {
      const prev = byId.get(concept.id)
      const merged = prev
        ? { ...prev, ...concept, state: prev.state }
        : { ...concept, state: defaultState(concept.id) }
      byId.set(concept.id, merged)
    }
    rows = [...byId.values()]
    if (rows.length !== version.conceptCount) {
      // A concept was deleted (nothing bumps updated_at on delete), so the
      // delta cannot express it — rebuild from a full fetch. The cached states
      // carry over by id: a deletion does not bump stateVersion, so the states
      // sync below will not run, and default states here would wipe every
      // mark from the snapshot.
      const cachedState = new Map(local.rows.map((row) => [row.id, row.state]))
      rows = (await deps.fetchFull()).map((concept) => ({
        ...concept,
        state: cachedState.get(concept.id) ?? defaultState(concept.id),
      }))
    }
  }
  if (!local || local.stateVersion !== version.stateVersion) {
    // Progress changed on the server (this or another device). The endpoint
    // omits default rows, so absent here means back to default.
    const states = await deps.fetchStates()
    const byConcept = new Map(states.map((state) => [state.conceptId, state]))
    rows = rows.map((row) => ({ ...row, state: byConcept.get(row.id) ?? defaultState(row.id) }))
  }
  return {
    v: SNAPSHOT_VERSION,
    contentVersion: version.contentVersion,
    conceptCount: version.conceptCount,
    stateVersion: version.stateVersion,
    syncedAt: Date.now(),
    rows,
  }
}

export function useConcepts() {
  const { user } = useSession()
  const userId = user?.id ?? ''
  return useQuery({
    queryKey: conceptsKey,
    // The version check is ~200B, so a long staleTime only delays noticing
    // fresh content by minutes; the disk snapshot keeps reloads instant.
    staleTime: 5 * 60_000,
    gcTime: Infinity,
    enabled: !!user,
    initialData: () => readSnapshot(userId)?.rows,
    initialDataUpdatedAt: () => readSnapshot(userId)?.syncedAt,
    queryFn: async () => {
      const snapshot = await syncConcepts(readSnapshot(userId), {
        getVersion: () => api.request<ContentVersion>('/api/content/version'),
        fetchFull: async () => (await api.request<Concept[] | null>('/api/concepts?limit=1000')) ?? [],
        fetchDelta: async (since) =>
          (await api.request<Concept[] | null>(`/api/concepts?limit=1000&updatedSince=${new Date(since).toISOString()}`)) ?? [],
        fetchStates: async () => (await api.request<ConceptState[] | null>('/api/concepts/states')) ?? [],
      })
      writeSnapshot(userId, snapshot)
      return snapshot.rows
    },
  })
}

// Optimistic state patch shared by the glossary and flashcard flows, so marks
// made in one surface show up instantly in the other. Not persisted — server
// truth arrives with the states sync on a later load.
export function markConceptState(client: QueryClient, conceptId: string, patch: Partial<ConceptState>) {
  client.setQueryData<ConceptRow[]>(conceptsKey, (rows) =>
    rows?.map((row) => (row.id === conceptId ? { ...row, state: { ...row.state, ...patch } } : row)),
  )
}

// The slim concept list omits unit/topic objects (the client already has
// /api/units); join them back and restore outline order so list views render
// exactly like the old server-ordered payload.
export function attachUnitTopic<T extends Concept>(concepts: T[], units: Unit[]): T[] {
  const unitById = new Map<string, Unit>()
  const topicById = new Map<string, Topic>()
  const topicOrder = new Map<string, number>()
  units.forEach((unit, unitIndex) => {
    unitById.set(unit.id, unit)
    unit.topics.forEach((topic, topicIndex) => {
      topicById.set(topic.id, topic)
      topicOrder.set(topic.id, unitIndex * 100000 + topicIndex)
    })
  })
  return concepts
    .map((concept) => ({ ...concept, unit: unitById.get(concept.unitId), topic: topicById.get(concept.topicId) }))
    .sort(
      (a, b) =>
        (topicOrder.get(a.topicId) ?? 1e9) - (topicOrder.get(b.topicId) ?? 1e9) ||
        a.position - b.position,
    )
}

// The store rows plus joined unit/topic, ready for list rendering.
export function useConceptRows() {
  const { data: units = [] } = useUnits()
  const query = useConcepts()
  const rows = useMemo(() => attachUnitTopic(query.data ?? [], units), [query.data, units])
  return { rows, isPending: query.isPending }
}
