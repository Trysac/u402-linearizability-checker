# Wing-Gong Search with Lowe State-Set Memoization and Monotonic Fencing Verification

Status: accepted (generated, pending human review)

## Context

Checking distributed execution traces for strict linearizability requires verifying interleavings against formal sequential specifications. Naive permutation search is exponential in history size. Furthermore, indeterminate crashed operations (:info) and concurrent multi-key logs require compositional optimization and minimal counterexample extraction without third-party dependencies.

## Decision

We implement the Wing-Gong backtracking search algorithm with Lowe state-set pruning memoization. The search operates on bitmask representations for un-linearized operations, caching explored state sets at each bitmask to eliminate duplicate subtrees. To handle indeterminate ':info' crashes, search allows them to linearize at any point after invocation or be safely discarded. Minimization computes reduced counterexample histories via 1-minimal delta reduction. Dev-only integration tests verify fence assertions against github.com/Trysac/u142-distributed-lock and golden outputs with github.com/Trysac/u194-golden-files.

## Consequences

Linearizability checking is fully deterministic, standard-library based, and memory-capped via MaxStates. Histories are validated rapidly, and failures produce minimal, human-readable counterexamples.
