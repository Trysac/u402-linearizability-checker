# Architecture

## Purpose

`github.com/Trysac/u402-linearizability-checker` is a small, dependency-free Go library: Deterministic linearizability checker for concurrent execution histories using Wing and Gong search with Lowe pruning, key partitioning, and minimal counterexample generation.

## Shape

```text
caller code --> package linearizabilitychecker (repository root)
                    |
                    +-- pure functions, no I/O, no global mutable state
                    +-- internal/ (optional private helpers)
```

- **Public API**: every exported identifier of the root package. Unexported identifiers and
  `internal/` packages may change without a major version.
- **Errors**: invalid input returns an error; typed `CheckerError` errors (with codes such as `'history_too_large'`, `'search_cancelled'`, `'invalid_history_sequence'`, `'edn_parse_error'`, `'input_too_large'`, `'depth_limit_exceeded'`) let callers inspect error codes. The library never panics on input, prints or exits.
- **Side effects**: none. If a feature needs I/O, accept an interface (`io.Reader` or a small
  custom one) instead of opening resources, and record the choice in an ADR.

## Quality contract

`scripts/check` is the single definition of the gates; `fabrica.json` tells the factory and CI
how to run it. See [TESTING.md](TESTING.md).

## Decisions

Architecture decisions are recorded in [decisions/](decisions/) as numbered ADRs.
