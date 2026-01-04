# alexandreia

Explainable grammar and text analysis for Ancient Greek.

Alexandreia is a collection of philological services focused on **rule-based, traceable analysis** of Ancient Greek.  
Each service produces structured output and full execution traces, allowing results to be inspected, replayed, and reused.

The project follows an Alexandrian approach:  
**rules first, attestations second, commentary always.**

---

## Philosophy

Alexandreia deliberately avoids black-box analysis.

Instead of producing a single opaque answer, each service:
- applies explicit rules,
- records what matched and what failed,
- verifies results against lexica and texts,
- and preserves a full trace of the reasoning process.

Mistakes are expected and accepted — but they are always visible.

---

## Current services

### Dionysios
**Rule-based grammar parser**

Dionysios analyzes and declines Ancient Greek words using explicit morphological rules
(endings, contractions, and limited prefix handling).

It:
- generates candidate forms,
- records which rules matched,
- verifies candidates against a lexicon,
- and produces a structured, explainable trace.

Coverage is intentionally partial but growing.

---

### Aristarchos
**Attested form index**

Aristarchos collects and indexes word forms produced by Dionysios and other sources.

It allows:
- querying texts beyond a single surface form (e.g. λόγος across declensions),
- tracking provenance (how and when a form was generated),
- distinguishing verified vs. unverified forms.

This index is intentionally permissive; correctness is refined downstream.

---

## Data seeders

### Anaximander
**Grammar data seeder**

Anaximander seeds Dionysios with structured grammatical data such as articles,
declension patterns, and rule metadata.

All data seeders in Alexandreia are named after Presocratic philosophers.

Example data shape (simplified):

```json
{
  "name": "misc",
  "type": "article",
  "dialect": "attic",
  "declensions": [
    {
      "declension": "ὁ",
      "ruleName": "article - sing - masc - nom",
      "searchTerm": ["ὁ"]
    }
  ]
}
```

## Planned services

### Kallimachos
Text and evidence analysis.

Kallimachos will correlate grammatical forms with:
- occurrences in Greek texts,
- aligned translations,
- contextual metadata (author, work, segment).

It will provide:
- textual evidence for candidate forms,
- translation cross-checks,
- structured scoring inputs.

### Eratosthenes
Dictionary lookup and cache layer.

Eratosthenes will act as a heavily cached gateway to dictionary and lexical services.

Responsibilities:
- fast form and lemma verification,
- aggressive in-memory caching,
- rate limiting and request coalescing,
- consistent responses across services.

### Galenos
Integration and system tests.

Galenos verifies the health of the Alexandrian pipeline end-to-end.

It tests:
- grammar → lexicon → text analysis flows,
- trace completeness and consistency,
- expected failure modes.

---

## Status

Alexandreia is under active development. Interfaces are expected to evolve, but
traceability and explainability are considered stable, core guarantees.

This repository is intended as a foundation for higher-level learning tools, not a finished grammar.
