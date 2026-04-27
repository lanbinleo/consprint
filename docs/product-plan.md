# Product Plan

## Vision

AP Psych Final Sprint is a focused review platform for AP Psychology knowledge points. It treats each required term as a concept, enriches it with definitions, examples, pitfalls, and source snippets, and turns it into a rating-driven study flow.

The first user is one student preparing for the final AP push, but the architecture should support multiple tenants, multiple users, and future subjects.

## MVP Scope

- Multi-tenant registration and login.
- Import all concepts from `keyterms.md`.
- Browse concepts by Science Practices, unit, and topic.
- Rate each concept from 0 to 5.
- Run a recognition-first review session.
- Reveal card backs with definitions, Chinese explanations, examples, and notes when available.
- Track review events and mastery changes.
- Display dashboard summaries: total concepts, average mastery, low-score concepts, and review activity.
- Show import/data health.

## Non-Goals For The First Version

- Public hosted deployment.
- Payment, sharing, or class management.
- Full Anki-compatible scheduling.
- Perfect AI-generated content for every concept before the app is usable.

## Key UX Principles

- The app opens directly into the study workspace.
- The interface should be calm and information-dense.
- Ratings must be fast: one click or keyboard-friendly.
- The user should always know what is due, what is weak, and what was just improved.
- Missing enrichment should not block study; it should be visible and easy to fill later.

## Screens

- Auth: login/register with tenant name.
- Dashboard: progress, weak areas, review entry point.
- Browse: unit/topic tree and searchable concept table.
- Concept Detail: content, rating, card list, source metadata.
- Review: term-first recognition flow.
- Session Summary: counts, mastery changes, weak concepts.
- Data Status: import counts and source availability.

## Content Strategy

`keyterms.md` is the truth for what must be learned. Notes and AI enrichment make each concept useful:

- Definition: English and Chinese can be presented together as one content block.
- Examples: included when available or useful.
- Pitfalls: especially for easily confused AP Psych terms.
- Extra notes: compact supporting text.
- Source references: retained in data for traceability, not necessarily shown by default.

