# Data Report

## Current Import Summary

- Parsed rows from `keyterms.md`: 797
- Canonical concepts imported: 794
- Units: 6
- Topics: 41

The difference between parsed rows and canonical concepts comes from exact duplicate terms inside the same topic:

- Unit 5 / 5.2 - Positive Psychology / Subjective well-being
- Unit 5 / 5.4 - Selection of Categories of Psychological Disorders / Delusions
- Unit 5 / 5.4 - Selection of Categories of Psychological Disorders / Hallucinations

These are treated as one concept each because the stable concept ID is based on course, unit, topic, and term.

## Enrichment Sources

Current deterministic enrichment uses:

- `unit0.md`
- `unit1.md`
- `AP Psychology Notes.opml`

Definitions, examples, pitfalls, and notes are extracted conservatively. The app remains usable when enrichment is pending because every concept has a recognition card.

