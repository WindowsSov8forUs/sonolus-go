# Sonolus-Go Performance Baseline

> Last updated: 2026-07-01

## Go Compiler Benchmarks

Run with: `go test ./compiler/engine/ -bench . -benchmem`

### Reference (Intel i7-13650HX, 20 threads)

| Benchmark | ns/op | allocs/op |
|-----------|-------|-----------|
| BenchmarkCompilePlayStages | ~221,300 | TBD |

Full benchmark suite:

```bash
go test ./compiler/engine/ -bench . -benchmem -benchtime=10x
go test ./compiler/ir/optimize/ -bench . -benchmem -benchtime=10x
```

## Cross-Project Comparison (Go vs Python)

Run with: `bash scripts/bench_cross.sh [iterations]`

### Methodology

1. Compile equivalent engine definitions in both compilers
2. Measure wall-clock time for N iterations
3. Report: latency (cold), throughput (hot), memory

### Target

| Metric | Target |
|--------|--------|
| Cold compile latency | Go < 2× Python |
| Hot compile latency (cached) | Go < Python |
| Throughput (100 compiles) | Go > 5× Python |
| Memory (peak RSS) | Go < Python |

## Optimizer Benchmarks

Run with: `go test ./compiler/ir/optimize/ -bench . -benchmem`

| Benchmark | Description |
|-----------|-------------|
| BenchmarkSCCP | Sparse Conditional Constant Propagation |
| BenchmarkCSE | Common Subexpression Elimination |
| BenchmarkLICM | Loop Invariant Code Motion |
| BenchmarkInlining | Variable Inlining |

## Notes

- Go benchmarks use `b.ReportAllocs()` for allocation tracking
- Python comparison requires sonolus.py at `../sonolus.py` with Python >= 3.11
- For production profiling, use `-cpuprofile` and `-memprofile` flags
