# Testing

## Running

```sh
go test -cover .          # tests with coverage summary
go run ./scripts/check    # every gate, as CI and the factory run it
```

## Policy

- Statement coverage must stay at or above 85%.
- Tests reside in `linearizabilitychecker_test` and exercise the public API only.
- Uses table-driven tests with `t.Run` and runnable `Example` functions.
- No network, wall clock, or test-order dependence.

## Tiers

- Local: Unit and property tests (`testing/quick`), conformance on small fixtures.
- CI: `conformance` tagged tests (files starting with `//go:build conformance`) running large histories (5000 oracle histories, 10000-operation KV benchmark) via `scripts/check -tags=conformance`.

## Layout

`<file>_test.go` sits next to the file it tests. Fixtures and golden files reside in `testdata/`.
