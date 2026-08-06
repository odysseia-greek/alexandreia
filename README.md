# Alexandreia

Explainable grammar, dictionary, and text analysis for Ancient Greek.

Alexandreia is a collection of small philological services built around one
principle: **rules first, attestations second, commentary always**. Results are
structured and auditable so later services can inspect how an answer was
produced instead of treating it as a black box.

## Services

### Dionysios

The public grammar and text-analysis orchestrator. Its gRPC API provides:

- `CheckGrammar` for rule-based analysis of a word;
- `Research` for lexical forms and their occurrences in texts;
- `TextMode` for token-by-token analysis, literal glosses, textual matches, and
  an optional audit trail;
- `Health` for Dionysios and its downstream dependencies.

Dionysios coordinates Eratosthenes, Aristarchos, and Kallimachos. The current
released module version is `dionysios/v0.3.3`.

### Eratosthenes

The dictionary gateway. It queries lexical entries, prefers canonical enriched
entries over lower-quality duplicates, and returns definitions and lexical
metadata used by Dionysios.

### Aristarchos

The attested-form index. It stores grammatical forms with their lemma,
part-of-speech information, rule, translation, and provenance. Dionysios uses
it to expand a lemma before requesting textual evidence.

### Kallimachos

The corpus and textual-evidence service. It can find a supplied phrase using
the original text, punctuation normalization, and bounded word windows. Its
`Analyze` endpoint searches the lemma and forms prepared by Dionysios; it no
longer depends directly on Aristarchos.

### Anaximander

The grammar-data seeder. The JSON files under `anaximander/arkho` contain the
explicit noun, adjective, verb, participle, article, pronoun, particle, and
other rules consumed by the grammar pipeline. Coverage is intentionally
incremental rather than exhaustive.

### Galenos

The Ginkgo integration suite for the public Dionysios gRPC contract. It checks
health, validation, representative grammar analysis, audit evidence, and a
Herodotus passage that exercises the complete dictionary-and-text flow. See
[`galenos/README.md`](galenos/README.md) for local and container usage.

## Development

Each service is an independent Go module with its own `go.mod` and
`Containerfile`. A local `go.work` may be used to work across modules, but it is
ignored because released modules and container builds must resolve explicit
versions.

Run the unit suites from the repository root, without requiring a workspace
file:

```shell
for module in anaximander aristarchos dionysios eratosthenes kallimachos; do
  (cd "$module" && go test ./...) || exit 1
done
```

Galenos is an integration suite and requires a running Dionysios stack:

```shell
cd galenos
DIONYSIOS_GRPC_ADDRESS=localhost:50060 go test ./...
```

Protocol definitions live in each service's `proto` directory. Generated Go
bindings are committed so downstream modules and container builds do not need
the protobuf toolchain.

## Status

Alexandreia is under active development. Grammar coverage and service
interfaces will continue to evolve; traceability and explainability are the
stable design goals. A future service, Demosthenes, is intended to turn the
structured evidence produced by Dionysios into more fluent contextual output.
