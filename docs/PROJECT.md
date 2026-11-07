# Project record

## Objective

Provide a deterministic linearizability checker for concurrent execution histories using Wing and Gong search with Lowe pruning, key partitioning, and minimal counterexample generation.

## Scope

- Core: Wing-Gong algorithm with Lowe state-set pruning cache; models (Register, KV, FIFO, Set, Counter, CAS); Jepsen EDN parser and JSON parser; brute-force oracle verification in tests; cancellation via `context.Context`; minimal counterexample extraction.
- Integration: Integration tests driving `github.com/Trysac/u142-distributed-lock` (acquire, release, lease expiry) with `github.com/Trysac/u194-golden-files` for counterexample validation.
- Stretch: `RenderHTMLReport` static per-process timeline; parallel partition checking.

## Out of scope

- Network capture or live Jepsen node agent orchestration.
- Continuous streaming or unbounded online verification.
- TLA+ / formal proof specification generation.

## Timeline

| Date | Event |
|---|---|
| 2026 | Repository created from the factory utility template. |

## Effort metrics

| Metric | Value |
|---|---|
| Tokens (input / output) | 5054220 / 45014 |
| Hours (human) | pending measurement |
| Cost (USD) | 0.8017 |

## Line comparison metrics

| Model | Line | Fix rounds | Input tokens | Output tokens | Cost | Time | Failing checks per round | Conformance percentage | Integrations achieved | Public API only |
|---|---|---|---|---|---|---|---|---|---|---|

## Factory run u402 (measured from API usage)

| Phase | Model | Batch | Input | Output | Cache write | Cache read | Cost (USD) |
|---|---|---|---|---|---|---|---|
| analyst | gemini-3.8-flash | no | 3536 | 1215 | 0 | 0 | 0.0072 |
| generation | gemini-3.8-flash | yes | 7261 | 13991 | 0 | 0 | 0.0290 |
| fix-1 | gemini-3.8-flash | no | 142911 | 20124 | 0 | 1489525 | 0.2944 |
| fix-2 | gemini-3.8-flash | no | 138453 | 6410 | 0 | 1833752 | 0.2654 |
| audit | gemini-3.8-flash | no | 131295 | 1510 | 0 | 1302163 | 0.2018 |
| docs | gemini-3.1-flash-lite | no | 5324 | 1764 | 0 | 0 | 0.0040 |

Total: $0.8017. Prices: Anthropic list prices.
