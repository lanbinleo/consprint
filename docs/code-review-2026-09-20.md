# Code review 2026-09-20 — concept cache & network optimization series

Scope: commits `803b7d6..4a30609` (gzip, content version + slim states, slim
concept payloads + delta, id-only deck, localStorage concept store, Terms
client-side filtering, flashcard hydration, admin debounce, perfcheck).
Both suites green at HEAD (`go test ./backend/ -count=1` 42.8s ok; frontend
`npm test` 27/27).

## Findings

### 1. `order=outline` deck is effectively random (P1, verified)

`backend/handlers_flashcards.go:87-90`:

```go
q := scope.Order("s.short_term_review desc, case s.status ... end asc, random()")
if order == "outline" {
    q = scope.Order("units.position asc, topics.position asc, concepts.position asc")
}
```

GORM chain methods mutate the shared statement (clone only happens on the
first chain call), so both `Order` calls APPEND: the final SQL is
`ORDER BY short_term_review desc, status-priority, random(), units.position, topics.position, concepts.position`.
`random()` never ties, so the position columns are dead letters — the outline
deck comes out review-priority + fully random.

Verified empirically with a throwaway test comparing the deck against the
expected outline order: 199/200 positions mismatched, first card from Unit 5.
This is a regression from `5614321` — the pre-rewrite code picked one `Order`
in an if/else. `TestReviewNextTopicIDsFilter` passes `order=outline` but only
asserts membership, not order, so the suite stays green.

Fix: choose the ordering string first, then a single `scope.Order(ordering)`,
and add an order assertion to the outline test.

**Fixed (same day):** `handlers_flashcards.go` now picks the ordering before
touching the statement; `TestReviewNextOutlineOrder` compares the deck against
the DB's outline order (it failed 199/200 positions on the old code).

### 2. Full-refetch path in `syncConcepts` wipes cached marks (P2)

`frontend/src/lib/conceptStore.ts:50-53`: when the post-delta count mismatch
triggers the deletion fallback, the full fetch maps EVERY row to
`defaultState(...)`. The states overlay afterwards runs only when
`local.stateVersion !== version.stateVersion` — but a deletion bumps neither
stateVersion nor contentVersion, so a user who has not marked anything since
their last sync gets their snapshot persisted with all marks reset to
unmarked. Glossary pills, flashcard topic counts, and review-queue filters
then show wrong data until the next mark (which bumps stateVersion and
recalibrates).

Trigger: any concept removal — i.e. source-file edit + admin reimport, which
is exactly the planned Phase 2 workflow. Server truth is unaffected; this is
a client cache correctness bug.

Fix: in the fallback, carry over the cached state by id:

```ts
const prevState = new Map(local.rows.map((row) => [row.id, row.state]))
rows = (await deps.fetchFull()).map((concept) => ({
  ...concept,
  state: prevState.get(concept.id) ?? defaultState(concept.id),
}))
```

The existing test at `conceptStore.test.ts:112` ("deleted concept") only
asserts ids — give row `a` a real mark and assert it survives.

**Fixed (same day):** the fallback now carries cached states over by id
(`cachedState.get(concept.id) ?? defaultState(...)`); the deleted-concept test
gives row `a` a `fuzzy` mark and asserts it (and `shortTermReview`) survives,
plus that `fetchStates` is not called.

### Minor observations (no action required now)

- **Empty glossary is silent**: with no snapshot and a failed version check,
  Terms renders an empty list with no error (pre-existing pattern, unchanged).
- **Backfill burst**: if the concept store is empty but the deck succeeds,
  `Flashcards.start()` fires up to 200 parallel `/api/concepts/:id` requests.
  Bounded by the 200-card session cap; consider a concurrency limit or a
  batch endpoint if it ever bites.
- **1000-row ceiling**: `/api/concepts` caps `limit` at 1000. If the corpus
  ever exceeds that, `syncConcepts` permanently mismatches `conceptCount` and
  degrades to a full fetch every sync (no corruption — just no caching). Fine
  at 794.
- **Delta merge can't express content removal**: `Concept.Content` uses
  `omitempty`, so a delta row whose content was deleted is indistinguishable
  from "unchanged" and the stale cached content survives. No flow currently
  deletes content rows; theoretical.
- **`ensureStates` on every concept endpoint** (including the 84B version
  check) runs a 794-row id scan + count. Fine at school scale; worth knowing
  it's there.

## What is good

- Deletion detection via row count is exact: after an upsert delta,
  `rows.length === conceptCount` iff no deletes (adds and deletes can't cancel
  out, because delta adds are deduped by id).
- The `strftime('%Y-%m-%dT%H:%M:%f', …)` millisecond boundary (`a547d01`) is
  well reasoned (UTC normalization, ms precision vs the import burst) and has
  a regression test.
- Cross-user cache hygiene is thorough: logout, sign-in, and 401 all clear
  both the query cache and every localStorage snapshot.
- Slim-shape tests use field allow-lists (fail on ANY new leaked field) — a
  good pattern to keep.
- `tools/perfcheck` turns the perf claims into a reproducible end-to-end
  report.
- Docs (`docs/concept-cache-performance.md`) match the implementation.

## Verification commands

```bash
go test ./backend/ -count=1   # 42.8s ok at review time
cd frontend && npm test       # 27/27 ok
```

The outline-order bug was reproduced with a temporary test (since deleted)
requesting `order=outline&limit=200` and comparing against the DB's outline
order; the fix should reinstate that assertion permanently.

## After the fixes

Both findings fixed and re-verified: backend suite `ok` (43.9s, now including
`TestReviewNextOutlineOrder`), frontend `npm test` 27/27 (strengthened
deleted-concept test). Changed files:

- `backend/handlers_flashcards.go` — single `Order` with a pre-picked ordering.
- `backend/handlers_flashcards_test.go` — `TestReviewNextOutlineOrder`.
- `frontend/src/lib/conceptStore.ts` — deletion fallback carries cached
  states over by id.
- `frontend/src/lib/conceptStore.test.ts` — deleted-concept test asserts the
  mark survives and no states fetch runs.
