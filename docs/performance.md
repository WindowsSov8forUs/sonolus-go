# Sonolus-Go Performance Baseline

> Last updated: 2026-07-02

## Go Compiler Benchmarks

Run with: `go test ./compiler/engine/ -bench . -benchmem`

### Reference (Intel i7-13650HX, 20 threads)

| Benchmark | ns/op | B/op | allocs/op |
|-----------|-------|------|-----------|
| BenchmarkCompilePlayStages | 1,546,500 | 1,725,040 | 20,544 |
| BenchmarkCompilePlay | 1,732,800 | 1,706,792 | 20,170 |
| BenchmarkCompilePlayWithArithmetic | 1,533,600 | 1,732,592 | 21,014 |
| BenchmarkCompilePlayWithLoop | 2,142,900 | 1,779,720 | 22,139 |
| BenchmarkCompileAllModes | 7,569,700 | 6,850,320 | 81,421 |
| BenchmarkCompileAllModesParallel | 4,886,500 | 6,850,576 | 81,578 |
| BenchmarkOptLevels/Minimal | 1,450,400 | 1,692,872 | 20,046 |
| BenchmarkOptLevels/Fast | 1,562,700 | 1,705,080 | 20,408 |
| BenchmarkOptLevels/Standard | 2,038,000 | 1,732,768 | 21,016 |

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

| Benchmark | ns/op | B/op | allocs/op | Description |
|-----------|-------|------|-----------|-------------|
| BenchmarkSCCP | 34,100 | 6,048 | 137 | Sparse Conditional Constant Propagation |
| BenchmarkCSE | 34,400 | 10,616 | 307 | Common Subexpression Elimination |
| BenchmarkLICM | 50,700 | 12,400 | 297 | Loop Invariant Code Motion |
| BenchmarkInlining | 25,200 | 8,832 | 206 | Variable Inlining |

## Notes

- Go benchmarks use `b.ReportAllocs()` for allocation tracking
- Python comparison requires sonolus.py at `../sonolus.py` with Python >= 3.11
- For production profiling, use `-cpuprofile` and `-memprofile` flags
