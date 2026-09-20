import { afterEach, describe, expect, it, vi } from 'vitest'
import { attachUnitTopic, syncConcepts, type SyncDeps } from './conceptStore'
import { clearConceptSnapshots, defaultState, readSnapshot, writeSnapshot, type ConceptSnapshot } from './conceptSnapshot'
import type { Concept, ConceptRow, ConceptState, ConceptStatus, ContentVersion, Unit } from './types'

// localStorage comes from src/test/setup.ts (memory-backed).

function concept(id: string, term: string, position = 0): Concept {
  return { id, unitId: 'u1', topicId: 't1', term, position, contentStatus: 'ready' }
}

function markedRow(id: string, term: string, status: ConceptStatus): ConceptRow {
  return { ...concept(id, term), state: { ...defaultState(id), status, shortTermReview: status !== 'proficient' && status !== '', reviewCount: 1 } }
}

function stateOf(id: string, status: ConceptStatus): ConceptState {
  return { ...defaultState(id), status, shortTermReview: status !== 'proficient' && status !== '', reviewCount: 1 }
}

function snapshot(rows: ConceptRow[], overrides: Partial<ConceptSnapshot> = {}): ConceptSnapshot {
  return {
    v: 2,
    contentVersion: 100,
    conceptCount: rows.length,
    stateVersion: 50,
    syncedAt: 12345,
    rows,
    ...overrides,
  }
}

function version(overrides: Partial<ContentVersion> = {}): ContentVersion {
  return { contentVersion: 100, conceptCount: 2, stateVersion: 50, ...overrides }
}

// Deps bundle that records what actually got fetched.
function makeDeps(v: ContentVersion, full: Concept[], delta: Concept[] = [], states: ConceptState[] = []) {
  return {
    deps: {
      getVersion: vi.fn(async () => v),
      fetchFull: vi.fn(async () => full),
      fetchDelta: vi.fn(async () => delta),
      fetchStates: vi.fn(async () => states),
    } satisfies SyncDeps,
  }
}

describe('syncConcepts', () => {
  afterEach(() => {
    clearConceptSnapshots()
  })

  it('first sync: full fetch plus states overlay, snapshot records versions', async () => {
    const { deps } = makeDeps(version(), [concept('a', 'Term A'), concept('b', 'Term B')], [], [stateOf('b', 'fuzzy')])
    const result = await syncConcepts(null, deps)
    expect(deps.fetchFull).toHaveBeenCalledTimes(1)
    expect(deps.fetchDelta).not.toHaveBeenCalled()
    expect(deps.fetchStates).toHaveBeenCalledTimes(1)
    expect(result.rows).toHaveLength(2)
    expect(result.rows[0].state.status).toBe('') // absent from states = default
    expect(result.rows[1].state.status).toBe('fuzzy')
    expect(result.rows[1].state.shortTermReview).toBe(true)
    expect(result.contentVersion).toBe(100)
    expect(result.stateVersion).toBe(50)
    expect(result.syncedAt).toBeGreaterThan(0)
  })

  it('unchanged content and state: only the version check runs', async () => {
    const local = snapshot([markedRow('a', 'Term A', 'fuzzy'), markedRow('b', 'Term B', '')])
    const { deps } = makeDeps(version(), [concept('a', 'unused')])
    const result = await syncConcepts(local, deps)
    expect(deps.getVersion).toHaveBeenCalledTimes(1)
    expect(deps.fetchFull).not.toHaveBeenCalled()
    expect(deps.fetchDelta).not.toHaveBeenCalled()
    expect(deps.fetchStates).not.toHaveBeenCalled()
    expect(result.rows).toBe(local.rows)
    expect(result.rows[0].state.status).toBe('fuzzy') // local marks survive
  })

  it('content change: delta upserts the row and keeps its cached state', async () => {
    const local = snapshot([markedRow('a', 'Old A', 'fuzzy'), concept('b', 'Term B') as ConceptRow])
    const { deps } = makeDeps(
      version({ contentVersion: 200 }),
      [concept('a', 'unused'), concept('b', 'unused')],
      [concept('a', 'New A')],
    )
    const result = await syncConcepts(local, deps)
    expect(deps.fetchDelta).toHaveBeenCalledTimes(1)
    expect(deps.fetchDelta).toHaveBeenCalledWith(100)
    expect(deps.fetchFull).not.toHaveBeenCalled()
    expect(deps.fetchStates).not.toHaveBeenCalled() // stateVersion unchanged
    expect(result.rows).toHaveLength(2)
    const updated = result.rows.find((row) => row.id === 'a')
    expect(updated?.term).toBe('New A')
    expect(updated?.state.status).toBe('fuzzy') // delta must not reset progress
    expect(result.contentVersion).toBe(200)
  })

  it('delta adds a new concept when the count grew', async () => {
    const local = snapshot([markedRow('a', 'Term A', 'proficient')])
    const { deps } = makeDeps(
      version({ contentVersion: 200, conceptCount: 2 }),
      [concept('a', 'unused'), concept('b', 'unused')],
      [concept('b', 'Term B', 1)],
    )
    const result = await syncConcepts(local, deps)
    expect(deps.fetchFull).not.toHaveBeenCalled()
    expect(result.rows).toHaveLength(2)
    expect(result.rows.find((row) => row.id === 'b')?.state.status).toBe('')
  })

  it('deleted concept (count mismatch after delta): falls back to a full fetch, keeping cached marks', async () => {
    const local = snapshot([markedRow('a', 'Term A', 'fuzzy'), markedRow('b', 'Term B', '')])
    const { deps } = makeDeps(version({ contentVersion: 200, conceptCount: 1 }), [concept('a', 'Term A')], [])
    const result = await syncConcepts(local, deps)
    expect(deps.fetchDelta).toHaveBeenCalledTimes(1)
    expect(deps.fetchFull).toHaveBeenCalledTimes(1)
    expect(deps.fetchStates).not.toHaveBeenCalled() // a deletion does not bump stateVersion
    expect(result.rows).toHaveLength(1)
    expect(result.rows[0].id).toBe('a')
    // Resetting to default states here would wipe the user's marks from the
    // snapshot (server truth only returns with the next states sync).
    expect(result.rows[0].state.status).toBe('fuzzy')
    expect(result.rows[0].state.shortTermReview).toBe(true)
  })

  it('stateVersion change: states recalibrate, unlisted rows reset to default', async () => {
    const local = snapshot([markedRow('a', 'Term A', 'fuzzy'), markedRow('b', 'Term B', 'unknown')])
    const { deps } = makeDeps(
      version({ stateVersion: 300 }),
      [concept('a', 'unused'), concept('b', 'unused')],
      [],
      [stateOf('a', 'proficient')], // b no longer marked anywhere
    )
    const result = await syncConcepts(local, deps)
    expect(deps.fetchDelta).not.toHaveBeenCalled() // contentVersion unchanged
    expect(deps.fetchStates).toHaveBeenCalledTimes(1)
    expect(result.rows.find((row) => row.id === 'a')?.state.status).toBe('proficient')
    expect(result.rows.find((row) => row.id === 'b')?.state.status).toBe('')
    expect(result.rows.find((row) => row.id === 'b')?.state.shortTermReview).toBe(false)
  })
})

describe('concept snapshot storage', () => {
  afterEach(() => {
    clearConceptSnapshots()
  })

  it('round-trips a snapshot for one user without leaking to another', () => {
    const snap = snapshot([markedRow('a', 'Term A', 'fuzzy')])
    writeSnapshot('user1', snap)
    expect(readSnapshot('user1')?.rows[0].term).toBe('Term A')
    expect(readSnapshot('user2')).toBeNull()
  })

  it('rejects snapshots written by an older format', () => {
    localStorage.setItem('apPsychConcepts:user1', JSON.stringify({ ...snapshot([], {}), v: 1 }))
    expect(readSnapshot('user1')).toBeNull()
  })

  it('clearConceptSnapshots removes only concept snapshots', () => {
    writeSnapshot('user1', snapshot([]))
    localStorage.setItem('apPsychToken', 'keep-me')
    clearConceptSnapshots()
    expect(readSnapshot('user1')).toBeNull()
    expect(localStorage.getItem('apPsychToken')).toBe('keep-me')
  })
})

describe('attachUnitTopic', () => {
  it('joins unit/topic and restores outline order', () => {
    // /api/units returns units and topics in position order; the corpus rows
    // may arrive shuffled (e.g. delta-appended).
    const units: Unit[] = [
      {
        id: 'u1',
        title: 'Unit 1',
        topics: [
          { id: 't1', unitId: 'u1', title: 'Topic 1' },
          { id: 't2', unitId: 'u1', title: 'Topic 2' },
        ],
      },
      { id: 'u2', title: 'Unit 2', topics: [{ id: 't3', unitId: 'u2', title: 'Topic 3' }] },
    ]
    const rows = [
      { ...concept('c', 'Third', 0), topicId: 't3', unitId: 'u2' },
      { ...concept('a', 'First', 0), topicId: 't1', unitId: 'u1' },
      { ...concept('b', 'Second', 0), topicId: 't2', unitId: 'u1' },
    ]
    const result = attachUnitTopic(rows, units)
    expect(result.map((row) => row.id)).toEqual(['a', 'b', 'c'])
    expect(result[0].topic?.title).toBe('Topic 1')
    expect(result[0].unit?.title).toBe('Unit 1')
    expect(result[2].unit?.title).toBe('Unit 2')
  })

  it('orders concepts inside a topic by position', () => {
    const units: Unit[] = [{ id: 'u1', title: 'Unit 1', topics: [{ id: 't1', unitId: 'u1', title: 'Topic 1' }] }]
    const rows = [concept('b', 'B', 2), concept('a', 'A', 1)]
    expect(attachUnitTopic(rows, units).map((row) => row.id)).toEqual(['a', 'b'])
  })
})
