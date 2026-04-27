# AP Psych Final Sprint Development Rules

## Mission

Build a usable local-first, multi-tenant AP Psychology sprint platform. The app must import the provided AP Psych key terms, enrich them with local notes and AI assistance where useful, and support rating-based review workflows that are pleasant enough for real daily use.

## Product Priorities

1. Data correctness comes first. `keyterms.md` is the canonical concept skeleton.
2. Preserve raw sources. Never overwrite or destructively transform provided source files.
3. Keep the app usable while enrichment improves. Empty definitions are acceptable only when clearly marked.
4. Support multi-tenant/login from the start, even if the first deployment is local.
5. Use the Notion-inspired design reference: quiet, warm, dense, tool-like, and readable.

## Data Source Priority

1. `keyterms.md`: canonical units, topics, and concepts.
2. `unit0.md` and `unit1.md`: local definitions, examples, and study notes.
3. `AP Psychology Notes.opml`: rich note fragments and source material.
4. GitHub unit text files: backup/cross-check source for units 2-5.
5. AI enrichment: structured definitions/examples/pitfalls, always traceable and reviewable.

## Engineering Rules

- Use React + TypeScript + Vite for the frontend.
- Use Go + Gin + GORM + SQLite for the backend.
- Keep backend and frontend independently testable.
- Store generated runtime data under `data/`, and keep raw downloaded source files under `data/sources/`.
- Use migrations or deterministic auto-migration. Seed data must be repeatable.
- Keep commits small by stage.
- Do not commit real API keys or generated databases unless intentionally needed for local demo data.

## Review Model

- Concepts have a 0-5 mastery score. Scores may be decimal internally.
- Cards belong to concepts. A concept can have multiple cards.
- Review events are append-only.
- The first usable review mode is recognition-first:
  - show term
  - user marks Know / Fuzzy / Don't know
  - if needed, reveal content
  - update mastery with diminishing returns near 5

## AI Cost Guardrails

- Batch enrichment by unit/topic.
- Prefer compact text protocols over verbose JSON when calling AI.
- Set timeouts.
- Limit retries.
- Store AI outputs before importing them.
- Never loop indefinitely on AI failures.

