# AP Psychology Learning Hub — Development Rules

## Mission

Build and run a one-stop, local-first AP Psychology learning platform for the school: concept library, three-tier flashcard review, a tag-based question bank with practice sets and timed mock exams, a wrong-question book, and class analytics — pleasant enough for real daily use after class.

## Product Priorities

1. Data correctness comes first. `keyterms.md` is the canonical concept skeleton (6 units / 41 topics / 794 concepts).
2. Preserve raw sources. Never overwrite or destructively transform provided source files.
3. Keep the app usable while enrichment improves. Empty definitions are acceptable only when clearly marked.
4. Single school tenant. Login is Microsoft Entra SSO (Teams accounts) first, local email login as fallback. Roles: student / teacher / admin.
5. Use the Notion-inspired design reference: quiet, warm, dense, tool-like, and readable.
6. No AI inside the platform. AI generation/grading runs as local workflows outside the codebase (e.g. `tools/ai-enrich.mjs`); the platform only imports their outputs.

## Data Source Priority

1. `keyterms.md`: canonical units, topics, and concepts.
2. `unit0.md` and `unit1.md`: local definitions, examples, and study notes.
3. `AP Psychology Notes.opml`: rich note fragments and source material.
4. GitHub unit text files: backup/cross-check source for units 2-5.
5. AI enrichment: structured definitions/examples/pitfalls, always traceable and reviewable.
6. Question bank: imported via CSV/JSON by teachers/TAs (validated two-step import; AI-generated questions arrive the same way, never generated in-app).

## Engineering Rules

- Use React + TypeScript + Vite for the frontend (react-router for URL routes, TanStack Query for data).
- Use Go + Gin + GORM + SQLite for the backend (handlers split by domain file; Entra via OAuth2 code flow).
- Keep backend and frontend independently testable.
- Store generated runtime data under `data/`, and keep raw downloaded source files under `data/sources/`.
- Use deterministic auto-migration. Seed data must be repeatable (794-concept canary test).
- Keep commits small by stage.
- Do not commit real API keys or generated databases unless intentionally needed for local demo data.
- Secret/env surface: `JWT_SECRET`, `ADMIN_EMAILS`, `ENTRA_*` (see `.env.example`). The app must stay fully usable with local login when Entra env is unset.

## Review Model

- Concepts have a three-tier self-assessment status: `proficient` / `fuzzy` / `unknown` (empty = unmarked). There is no numeric mastery score.
- Review events are append-only (`response` = proficient|fuzzy|unknown).
- The flashcard flow is flip-card first: show term → flip to reveal content → mark with three big buttons (keys 1/2/3).
- `fuzzy`/`unknown` concepts enter the short-term review queue; `proficient` clears it.

## Practice Model

- Question types: `mcq` (auto-graded) and `subjective` (FRQ/AAQ/EBQ-style with materials, parts, reference answers, rubrics; student self-rates proficient/partial/weak — no auto-grading).
- Questions carry tags + optional unit/topic links; practice sets are assembled from the bank (mode: `instant` per-question feedback, or `exam` with server-enforced deadline and grading on submit).
- Answers are append-per-attempt; the wrong book is derived from the latest answer per question, not stored separately.
- Student-facing payloads must never leak `answerKey`/`explanation`/reference answers before reveal time.

## AI Cost Guardrails (local workflows only)

- Batch enrichment by unit/topic.
- Prefer compact text protocols over verbose JSON when calling AI.
- Set timeouts. Limit retries. Store AI outputs before importing them.
- Never loop indefinitely on AI failures.
