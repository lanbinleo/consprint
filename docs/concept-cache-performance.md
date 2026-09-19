# Concept corpus caching & network optimization (2026-09-20)

## Motivation

The glossary (Terms) page pulled the full concept list — ~1.4MB of JSON — on
every mount, and because `search` was part of the query key, **every keystroke
in the search box re-downloaded all of it**. Marking a concept invalidated the
whole list query (another 1.4MB). Starting a flashcard session re-downloaded
~360KB of the exact same content the glossary already had. There was no HTTP
caching (no ETag / gzip / Cache-Control) and no persistence — a reload always
paid the full cost.

Measured on the 794-concept corpus (`go run ./tools/perfcheck`):

| Scenario | Before | After |
|---|---|---|
| First visit, full list | ~1.44MB uncompressed | **135KB gzipped** |
| Reload, content unchanged | 1.44MB | **84B** (version check) |
| Content edited (one concept) | 1.44MB | **~2KB delta** |
| Progress made elsewhere | — | 142B–20KB (`/api/concepts/states`) |
| Typing in search box | 1.44MB × per keystroke | **0** (client-side filter) |
| Marking a concept | 1.44MB refetch | **0** (in-place cache patch) |
| Starting flashcards (200 cards) | ~360KB | **2.6KB** (ids + state only) |

## API changes (breaking — deploy frontend and backend together; production serves both from one binary, so a normal redeploy covers it)

- **gzip on all `/api` responses** (`gin-contrib/gzip`, `backend/router.go`).
  JSON with repeated structures compresses to ~17% of raw size.
- **`GET /api/content/version`** → `{contentVersion, conceptCount, stateVersion}`.
  `contentVersion` = max `updated_at` over concepts + concept_contents + units +
  topics (unix ms). `stateVersion` = max `updated_at` of the caller's
  `user_concept_states`. Marks bump only `stateVersion`; content edits and
  imports bump only `contentVersion` — the two signals never invalidate each
  other. `conceptCount` detects deletions.
- **`GET /api/concepts` slimmed**: concept + content rows only. The per-row
  `unit`/`topic` objects (~230KB of duplication) and merged `state` are gone;
  clients join units from `/api/units` and state from the states endpoint.
  `Concept.Unit/Topic` became pointers so unpreloaded rows omit the keys.
  New `updatedSince` (RFC3339) param returns only rows changed at/after the
  boundary (inclusive — a client that stores the served version never misses a
  row). Filter and comparison detail: SQLite stores datetimes as
  timezone-suffixed strings, so both sides are normalized with
  `strftime('%Y-%m-%dT%H:%M:%f', …)` (UTC, millisecond precision). `datetime()`
  alone truncates to whole seconds and — because the importer stamps all rows
  within a few milliseconds — degraded every delta to a full fetch.
- **`GET /api/concepts/states`** → `[{conceptId, status, reviewCount,
  shortTermReview, starred}]`, **only non-default rows** (absent conceptId =
  unmarked). This is how a locally cached list recalibrates after progress made
  on another device, without re-sending content.
- **`GET /api/review/next`** → same slim state rows (`conceptId` + state), in
  the chosen order, no concept payloads. The client hydrates card content from
  its cached corpus; ids missing from the cache backfill from
  `/api/concepts/:id`.

`GET /api/concepts/:id` is unchanged (unit/topic/state included) — it is the
deep-link/backfill fallback.

## Frontend architecture

- `lib/conceptSnapshot.ts` — per-user localStorage snapshot
  (`apPsychConcepts:<userId>`, format-versioned; dropped on logout / sign-in /
  401 alongside `queryClient.clear()`).
- `lib/conceptStore.ts` — the single `['concepts']` query. `initialData` comes
  synchronously from the snapshot (instant paint, `initialDataUpdatedAt` =
  sync time so staleness is measured from the real sync), `staleTime` 5min.
  The queryFn is the pure `syncConcepts` algorithm: version check → full fetch
  (no snapshot) / zero transfer (version + count match) / delta merge
  (upsert by id; count mismatch ⇒ deletion ⇒ full rebuild) → states overlay if
  `stateVersion` moved. `attachUnitTopic` joins unit/topic from `/api/units`
  and restores outline order. `markConceptState` patches one row's state in
  place — used by both the Terms mark flow and the flashcard queue, so marks
  made in either surface appear in both with zero refetching. Marks are not
  persisted to disk; server truth arrives with the next states sync.
- `Terms.tsx` — unit/topic/search filtering is client-side over the store
  (search matches `term` and `normalizedTerm`, case-insensitive — English
  aliases now also match, which the old server search did not do). Deep links
  resolve from the store; only an id absent after the corpus loaded falls back
  to the detail endpoint. Marking keeps its optimistic patch but no longer
  invalidates the list.
- `Flashcards.tsx` — deck request carries ids; cards hydrate from the store.
  Marks enqueue through the existing `reviewQueue` and patch the shared cache.
- `admin/Content.tsx` — search debounced (300ms), unit/topic joined
  client-side; edits and reimports invalidate `['concepts']`, which the store
  picks up as a ~2KB delta on its next version check.

## Verifying

```bash
go test ./backend/ -count=1        # full suite (~45s) incl. slim-shape and version-signal tests
cd frontend && npm test            # syncConcepts unit tests (localStorage stubbed in src/test/setup.ts)
go run ./tools/perfcheck           # end-to-end wire-size report against a throwaway DB
```

In the browser (dev servers): open DevTools Network — first visit pulls
`/api/concepts` once (~135KB transfered); reload shows an 84B
`/api/content/version` and nothing else; typing in the glossary search box
fires no requests; marking a concept fires only the 408B PATCH.
