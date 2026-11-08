# Security notes

## Threat model

`github.com/Trysac/u402-linearizability-checker` is a pure library without I/O. The threat model includes:

- **Supply chain**: Mitigated by using standard library only, `go.sum` verification, `govulncheck`, Dependabot updates, and minimal workflow permissions.
- **Leaked secrets**: Mitigated by `gitleaks` in CI.
- **Untrusted input**: Callers may pass hostile strings. Mitigated by validating inputs and returning explicit `CheckerError` types; the library avoids `unsafe` and `os/exec`.
- **Limits**: To prevent DoS from resource exhaustion, the library enforces a default 50,000 operation limit per history, a 1,000,000 state limit in the Lowe pruning cache, and a 10 MB payload limit for parsers.

## Publishing

Releases are git tags served through the Go module proxy. No registry token is stored in the repository.
