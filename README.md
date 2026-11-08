# linearizability-checker

[![Go Reference](https://pkg.go.dev/badge/github.com/Trysac/u402-linearizability-checker.svg)](https://pkg.go.dev/github.com/Trysac/u402-linearizability-checker)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![CI](https://github.com/Trysac/u402-linearizability-checker/actions/workflows/ci.yml/badge.svg)](https://github.com/Trysac/u402-linearizability-checker/actions/workflows/ci.yml)

Deterministic linearizability checker for concurrent execution histories using Wing and Gong search with Lowe pruning, key partitioning, and minimal counterexample generation.

## Problem

Distributed systems engineers and test authors need to verify whether concurrent execution logs uphold strict linearizability. Without automated verification, subtle concurrency bugs such as stale reads or dropped writes escape unit tests and produce irreproducible production incidents. An embeddable zero-dependency checker provides fast, deterministic verdicts and minimal counterexample traces directly in local test suites and CI pipelines.

## Install

```sh
go get github.com/Trysac/u402-linearizability-checker
```

Requires Go 1.23 or newer. This library uses only the Go standard library.

## Usage

```go
import "github.com/Trysac/u402-linearizability-checker"

func main() {
    history := []linearizabilitychecker.Op{ /* ... */ }
    res, err := linearizabilitychecker.Check(context.Background(), linearizabilitychecker.RegisterModel(), history, linearizabilitychecker.Options{})
    if err != nil {
        log.Fatal(err)
    }
    if !res.Ok {
        log.Printf("Non-linearizable trace! Minimal ops: %d", len(res.Counterexample.MinimalHistory))
    }
}
```

## Public API Summary

- `Check[S any](ctx context.Context, model Model[S], history []Op, opts Options) (Result, error)`
- `ParseEDNHistory(r io.Reader, maxBytes int64) ([]Op, error)`
- `ParseJSONHistory(r io.Reader, maxBytes int64) ([]Op, error)`
- `RenderHTMLReport(w io.Writer, report *Report) error`

### Limits
- Default MaxHistoryOps: 50,000 (exceeding returns `history_too_large`).
- MaxStates (Lowe pruning cache): 1,000,000 (exceeding returns `depth_limit_exceeded`).
- Max EDN/JSON payload: 10 MB.

### Errors
`CheckerError` codes: `history_too_large`, `search_cancelled`, `invalid_history_sequence`, `edn_parse_error`, `input_too_large`, `depth_limit_exceeded`.

## Why not Porcupine?

Existing Go checkers like Porcupine implement W&G without native support for indeterminate ':info' Jepsen crash semantics or clean separation for zero-dependency test assertion. This library requires zero third-party runtime dependencies and bounds memory with `MaxStates`.

## Security

See [docs/SECURITY.md](docs/SECURITY.md).
