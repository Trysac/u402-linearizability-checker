// Package linearizabilitychecker is a small, dependency-free Go library: Deterministic linearizability checker for concurrent execution histories using Wing and Gong search with Lowe pruning, key partitioning, and minimal counterexample generation.
//
// The public API is every exported identifier of this root package. Functions are pure: they do
// no I/O, keep no global mutable state and report invalid input through returned errors.
package linearizabilitychecker
