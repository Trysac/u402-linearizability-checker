# Benchmarks

Benchmarks are run against deterministic test histories to profile search throughput and memory scaling.

Numbers are filled in by a benchmark run, never invented.

## Running

```sh
go test -bench=. -benchmem ./...
```

## Results

| Date | Hardware | Runtime version | Benchmark | Iterations | Time/op | Bytes/op | Allocs/op |
|---|---|---|---|---|---|---|---|
