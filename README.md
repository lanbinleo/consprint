# AP Psychology Learning Hub

A one-stop, local-first AP Psychology learning platform: concept library, three-tier flashcard review, a tag-based question bank with practice sets and timed mock exams, a wrong-question book, and class analytics — built with React, Go/Gin, GORM, and SQLite.

School sign-in is handled by Microsoft Entra ID (Teams accounts); local email login stays available as a fallback.

## What It Does

- Imports the canonical AP Psychology concept skeleton from `data/sources/keyterms.md` (6 units / 41 topics / 794 concepts) and enriches it with local notes, OPML fragments, and the compact AI enrichment file.
- **Learn**: browse units → topics → concepts with bilingual definitions, examples, pitfalls, and notes; mark each concept as 熟练 / 模糊 / 不熟悉 (proficient / fuzzy / unfamiliar).
- **Flashcards**: big flip-card review sessions (space to flip, keys 1/2/3) scoped by unit/topic and status filter; fuzzy/unfamiliar concepts enter the short-term queue.
- **Question bank**: teachers/TAs manage MCQ and subjective (FRQ/AAQ/EBQ-style) questions with materials, per-part prompts, reference answers, and rubrics; questions carry tags and unit/topic links.
- **Bulk import**: two-step CSV (Excel-friendly) and JSON import with full validation and per-row error preview; nothing is written before review. CSV template included at `/api/admin/questions/import/template`.
- **Practice sets**: assemble papers from the bank by tag/unit/type; two modes — instant per-question feedback, or timed exam mode with server-enforced deadline and grading on submit.
- **Subjective self-assessment**: students compare their answer against the reference + rubric and rate themselves (solid / partially there / off track) — groundwork for a future AI grading workflow.
- **Wrong book**: tracks the latest-wrong MCQs and weakly self-rated subjective items until they are resolved.
- **Class analytics**: active users, accuracy by unit, flashcard status distribution, weakest concepts, and per-student drill-down for teachers/admins.
- **Roles**: student / teacher / admin. `ADMIN_EMAILS` bootstraps admins (e.g. `leo.huo_27@tsinglan.org`); teachers manage questions/sets/analytics; admins additionally manage users and concept content.
- AI generation happens **outside** the platform (local workflows like `tools/ai-enrich.mjs`); the platform itself contains no AI calls.

## Run Locally

Install frontend dependencies:

```powershell
cd frontend
npm install
```

Run the backend:

```powershell
cd ..
go run .
```

Run the frontend in another terminal:

```powershell
cd frontend
npm run dev
```

Open `http://localhost:5173`. The backend API runs on `http://localhost:8080`. On first startup, it creates `data/app.db` and imports the source data (794 concepts).

## Build

```powershell
cd frontend
npm run build
cd ..
go run .
```

After the frontend is built, the Go server also serves the site from `http://localhost:8080`.

## Docker Compose

```powershell
docker compose up -d --build
```

See [docs/deploy-onepanel-compose.md](docs/deploy-onepanel-compose.md) for the full OnePanel/server deployment flow.

## Configuration

Runtime configuration lives in `.env` (see `.env.example`).

Core:

```text
JWT_SECRET=replace-with-a-long-random-secret
CORS_ORIGIN=https://your-domain.example
REGISTRATION_INVITE_CODE=optional-class-code
APP_TIMEZONE=Asia/Shanghai
AP_EXAM_DATE=2027-05-01T12:00:00+08:00   # dashboard countdown, optional
```

Microsoft Entra SSO (leave empty to hide the Microsoft button and use local login):

```text
ENTRA_TENANT_ID=
ENTRA_CLIENT_ID=
ENTRA_CLIENT_SECRET=
ENTRA_REDIRECT_URI=https://your-domain.example/api/auth/entra/callback
ENTRA_ALLOWED_DOMAINS=tsinglan.org
ENTRA_FRONTEND_REDIRECT=/auth/callback
```

Roles and school:

```text
ADMIN_EMAILS=leo.huo_27@tsinglan.org
SCHOOL_NAME=AP Psychology
```

In production (`APP_ENV=production` or `GIN_MODE=release`) the server refuses to start unless `JWT_SECRET` is at least 32 characters.

### Entra app registration checklist

1. Register an app in the school's Microsoft Entra tenant.
2. Add a web redirect URI: `https://your-domain.example/api/auth/entra/callback`.
3. Grant the `User.Read` (Microsoft Graph) delegated permission.
4. Create a client secret and fill in the four `ENTRA_*` values.

Until these are set, the platform is fully usable with local email login.

## Question Import Format

CSV columns (semicolon-separated tags, `Title :: text` materials, one material per line):

```text
type,stem,choice_a,choice_b,choice_c,choice_d,choice_e,answer,explanation,unit,topic,tags,materials,reference_answer,rubric,source_note
```

JSON is an array of question objects with `type` (`mcq`|`subjective`), `stem`, `choices`+`answerKey`, `explanation`, `materials`, `parts` (label/prompt/referenceAnswer/rubric), `unit`, `topic`, `tags`. Subjective questions need a reference answer or rubric.

## AI Enrichment (local workflow, concept content)

The enrichment tool runs locally against an OpenAI-compatible endpoint and writes `data/sources/ai-enrichment.compact`; the app only imports the file:

```powershell
node tools/ai-enrich.mjs --limit=20
```

Config: `OPENAI_API_KEY`, `OPENAI_BASE_URL`, `OPENAI_MODEL`, `AI_BATCH_SIZE`, `AI_TIMEOUT_MS`, `AI_RETRIES`. Re-import from Admin → Concept content.

## Tests

```powershell
go test ./...
cd frontend
npm run build
```

## Accounts And Roles

Register a local account on first launch — the first account becomes admin when `ADMIN_EMAILS` is unset (development fallback). In production set `ADMIN_EMAILS`; those accounts are promoted at startup and on first Entra sign-in. Teachers and admins manage the question bank, practice sets, and analytics; admins also manage user roles and concept content.
