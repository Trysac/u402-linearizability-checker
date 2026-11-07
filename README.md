# linearizability-checker

[![Go Reference](https://pkg.go.dev/badge/github.com/Trysac/u402-linearizability-checker.svg)](https://pkg.go.dev/github.com/Trysac/u402-linearizability-checker)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![CI](https://github.com/Trysac/u402-linearizability-checker/actions/workflows/ci.yml/badge.svg)](https://github.com/Trysac/u402-linearizability-checker/actions/workflows/ci.yml)

Deterministic linearizability checker for concurrent execution histories using Wing and Gong search with Lowe pruning, key partitioning, and minimal counterexample generation.

## Problem

Describe in two or three sentences the concrete problem this utility solves, who hits it, and what
they do today without it. Replace this paragraph when the first real feature lands.

## Install

```sh
go get github.com/Trysac/u402-linearizability-checker
```

Requires Go 1.23 or newer. No dependencies outside the standard library.

## Usage

```go
package main

import (
	"errors"
	"fmt"

	"github.com/Trysac/u402-linearizability-checker"
)

func main() {
	// Normalize trims surrounding whitespace and lower-cases the text.
	out, _ := linearizabilitychecker.Normalize("  Hello World ")
	fmt.Println(out) // -> "hello world"

	// Blank input is rejected explicitly instead of returning an empty string.
	if _, err := linearizabilitychecker.Normalize("   "); errors.Is(err, linearizabilitychecker.ErrBlank) {
		fmt.Println("rejected:", err)
	}
}
```

## Documentation

- [Architecture](docs/ARCHITECTURE.md) and [decisions](docs/decisions/)
- [Testing](docs/TESTING.md) and [security](docs/SECURITY.md)
- [Project record](docs/PROJECT.md) and [stack](docs/STACK.md)
- [Changelog](CHANGELOG.md) and [contributing guide](CONTRIBUTING.md)

## License

[MIT](LICENSE) (c) 2026 Miguel Gomez
