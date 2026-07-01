# Optimizer Pipeline: Go vs Python Comparison

This document records the intentional (and incidental) divergences between the
Go optimizer pipeline (`pipeline.go`) and its Python reference
(`sonolus.py/sonolus/backend/optimize/optimize.py`).

## Intentional Divergences

### 1. Pre-SSA: Earlier CoalesceSmallConditionalBlocks

| # | Python Standard | Go Standard |
|---|-----------------|-------------|
| 1 | `CoalesceFlow` | `CoalesceFlow` |
| 2 | `UnreachableCodeElimination` | `CoalesceSmallConditionalBlocks` |
| 3 | `DeadCodeElimination` | `UnreachableCodeElimination` |
| 4 | `CoalesceSmallConditionalBlocks` | `DeadCodeElimination` |

**Rationale:** Running `CoalesceSmallConditionalBlocks` before UCE/DCE allows
trivial diamond patterns (short blocks that only differ by a single value
assignment) to be collapsed earlier, giving UCE and DCE more dead code to
eliminate. This ordering was chosen intentionally during the port.

### 2. Post-FromSSA: UnflattenAssociativeOps Inserted

Go runs `UnflattenAssociativeOps` immediately after `FromSSA`, before
`CopyCoalesce`. Python's `FromSSA` is followed directly by `CoalesceFlow`.
This extra unflatten pass catches associative operation chains that may have
been introduced by SSA destruction and gives later passes cleaner input.

### 3. Post-SSA Cleanup: Extra DCE Round after AdvancedDCE

Go adds a full cleanup round (`CoalesceFlow` + `UnreachableCodeElimination` +
`DeadCodeElimination`) after `AdvancedDCE`, before `RenumberVars`. Python's
post-SSA sequence is more compact: `AdvancedDCE` → `CoalesceFlow` →
`NormalizeSwitch` → `CombineExitBlocks` → `Allocate`.

**Rationale:** `AdvancedDCE` can expose new unreachable code and dead stores.
The extra cleanup round catches these cascading opportunities before final
allocation, producing slightly smaller output for complex callbacks.

### 4. CombineExitBlocks before NormalizeSwitch

| # | Python (post-SSA extract) | Go (post-SSA extract) |
|---|---------------------------|-----------------------|
| … | `NormalizeSwitch` | `CombineExitBlocks` |
| … | `CombineExitBlocks` | `NormalizeSwitch` |

**Rationale:** Go reverses these two passes. `CombineExitBlocks` merges
identical exit targets, which simplifies the switch structures that
`NormalizeSwitch` needs to inspect. Reversing the order reduces work done by
`NormalizeSwitch` and improves its effectiveness.

### 5. RenumberVars in Standard Pipeline

Go includes `RenumberVars` at the end of the Standard pipeline, after all
cleanup and before allocation. Python does not include `RenumberVars` in
`STANDARD_PASSES` — variable renumbering is handled implicitly by Python's
`Allocate` pass which assigns deterministic offsets based on sorted
interference order.

**Rationale:** Explicit renumbering guarantees deterministic variable ordering
regardless of the allocation strategy. This is important for Go's
`AllocateLive` which uses liveness-based linear scan rather than
interference-graph allocation.

### 6. Allocation Strategy

| Level | Python | Go |
|-------|--------|-----|
| MINIMAL | `AllocateBasic` (sequential, no liveness) | `AllocateBasic` (sequential, no liveness) |
| FAST | `TryAllocateBasic` (sequential with fallback) | `TryAllocateBasic` (sequential with fallback to AllocateLive) |
| STANDARD | `Allocate` (liveness-aware) | `AllocateLive` (liveness-based linear scan) |

**Rationale:** Go uses `AllocateBasic` for MINIMAL/FAST to keep compilation
fast, and `AllocateLive` (interval-packing linear scan) for STANDARD to produce
compact output. Python's `AllocateFast` and `TryAllocateBasic` tiered
strategies are not yet implemented; implementing them is tracked as P2 step 2-1.

## Analysis Pass Model Difference

Python models `DominanceFrontiers` and `LivenessAnalysis` as first-class
`CompilerPass` objects with `requires()` dependency declarations. The
`run_passes` orchestrator automatically inserts missing analyses.

Go computes dominance and liveness internally within each pass that needs
them. This is simpler but means `ComputeDominance` may be called multiple times
per pipeline (up to 6× in Standard: `ToSSA` ×1, `InlineVars` ×3, `LICM` ×1,
`CSE` ×1). A dominance cache is tracked as P2 step 2-2.

## Pass Count Summary

| Pipeline | Python Passes | Go Passes (pipeline + allocator) |
|----------|---------------|----------------------------------|
| Standard | 37 | 44 |
| Fast | 4 | 26 |
| Minimal | 3 | 6 |

Go runs more passes because it includes multiple cleanup rounds and explicit
`RenumberVars`. The extra passes are low-cost (O(N) block/statement scans) and
improve output quality.

## Additional Divergences (Documented Post-Audit, 2026-07)

### 7. SCCP Frozenset Limit

Python caps frozenset at 100 elements before collapsing to NAC; Go originally used
8. Aligned to 100 in P1-1 to match Python precision for switch-edge pruning on
phases with many constant candidates.
- Python: `sonolus.py/sonolus/backend/optimize/constant_evaluation.py:135`
- Go: `compiler/ir/optimize/sccp.go:32` (`frozensetMax`)

### 8. RemoveRedundantArguments — Sub(0, a) → Negate(a)

Python folds `Subtract(0, a)` into `Negate(a)`. Go's `trimArgs` previously only
removed identity elements (0 for Add/Subtract, 1 for Multiply/Divide) and did not
handle the `Sub(0, a)` pattern. Added in P1-2.
- Python: `sonolus.py/sonolus/backend/optimize/simplify.py:325-326`
- Go: `compiler/ir/optimize/simplify.go:99-104`

### 9. CopyCoalesce: Def-Count Heuristic vs Full Liveness

Python's `CopyCoalesce` uses `LivenessAnalysis` to build an interference graph,
checking whether the copy target is live at the copy point. Go uses a simpler
def-count heuristic: coalesce on single-predecessor edge blocks unconditionally;
for multi-predecessor blocks, only coalesce when the destination temp has a
single definition. The Go heuristic is cheaper but may miss coalescing
opportunities that full liveness analysis would catch.
- Python: `sonolus.py/sonolus/backend/optimize/copy_coalesce.py:69-80`
- Go: `compiler/ir/optimize/copycoalesce.go:61-69`

### 10. NormalizeBlocks: Mode-Aware Type Coercion Omitted

Python's `NormalizeBlocks` performs mode-aware block type coercion, converting
`BlockPlace` block/index pairs to mode-specific block types based on
`config.mode.blocks`. Go's version only handles nil `BlockPlace.Index → Const(0)`.
In Go's architecture, block types are determined at IR construction time by the
frontend (via `compiler/frontend/trace.go`), making the runtime coercion
unnecessary. This is an architectural difference, not a functional gap.
- Python: `sonolus.py/sonolus/backend/optimize/simplify.py:345-381`
- Go: `compiler/ir/optimize/simplify.go:304-330`

### 11. Go FAST Pipeline Scope vs Python FAST

Python's FAST level is 4 passes (CoalesceFlow → UCE → TryAllocateBasic →
CoalesceFlow) with no SSA construction. Go's FAST level includes a full SSA
round (ToSSA → SCCP → InlineVars → FromSSA) plus cleanup, totalling 26 passes.
The two optimization levels are not comparable in scope; Go's FAST is
effectively a Standard-lite pipeline. This is an intentional design choice to
provide meaningful optimization at the FAST level without requiring the full
Standard pipeline cost.

## Reference

- Python reference: `sonolus.py/sonolus/backend/optimize/optimize.py`
- Go implementation: `compiler/ir/optimize/pipeline.go`
- Port verification: golden tests in `compiler/ir/optimize/optimize_test.go`
