# Data Model

## Core Entities

### Tenant

A tenant owns users and review state. The first app is local, but tenant boundaries are present from the start.

### User

A login identity within a tenant.

### Course

The AP Psychology course container.

### Unit

Examples:

- Science Practices
- Unit 1: Biological Bases of Behavior
- Unit 2: Cognition

### Topic

Examples:

- Set A
- 1.3 - The Neuron and Neural Firing

### Concept

The canonical knowledge point from `keyterms.md`.

Important fields:

- stable ID
- term
- normalized term
- unit ID
- topic ID
- position
- content status

### ConceptContent

Optional enriched content for a concept.

Fields:

- definition: rich text block payload
- examples: rich text block payload
- pitfalls: rich text block payload
- notes: rich text block payload
- source kind and source confidence

Rich text uses compact JSON blocks internally because it maps cleanly to frontend rendering and SQLite storage.

### Card

A practice item linked to a concept.

Initial card types:

- recognition
- definition
- application
- contrast

The first version creates at least one recognition card per concept.

### UserConceptState

Per-user mastery state:

- mastery: 0-5 decimal
- manual rating: 0-5 optional
- review count
- last reviewed at
- short-term review flag

### UserCardState

Per-user card scheduling state:

- due at
- stability/difficulty placeholders for future SRS
- last response

### ReviewEvent

Append-only history:

- concept ID
- card ID
- response: know, fuzzy, unknown, reveal
- mastery before
- mastery after
- duration
- created at

## Mastery Update

The MVP uses a diminishing-return formula:

```text
gain = base * (1 - mastery / 5)
```

Responses:

- know: `base = 0.45`
- fuzzy: `base = 0.18`
- unknown: `-0.12`, clamped to 0, and flagged for short-term review

Manual ratings directly set mastery.

## Import Rules

- Imports are idempotent by stable ID.
- Existing user state is never reset by content imports.
- Raw files remain unchanged.
- Missing enrichment is marked as `pending`.

