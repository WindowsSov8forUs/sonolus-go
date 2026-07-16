# compiler optimizer provenance

The optimizer pipeline follows `sonolus.py` commit `1040bc0`:

- `MINIMAL_PASSES`: `CoalesceFlow`, unreachable-code elimination, and basic allocation.
- `FAST_PASSES`: `CoalesceFlow`, unreachable-code elimination, `TryAllocateBasic`, and final flow coalescing. Fast never enters SSA.
- `STANDARD_PASSES`: the pass order in `sonolus/backend/optimize/optimize.py`, including SSA construction/destruction, SCCP, DCE, inlining, associative simplification, switch rewriting, LICM, CSE, copy coalescing, and liveness allocation.

The Go IR represents CFG edges and Phi arguments explicitly, so CFG passes also update predecessor IDs and split critical Phi edges before SSA destruction. Mode-specific memory has already been normalized by the frontend catalog; `NormalizeBlocks` therefore normalizes CFG order rather than coercing Python `BlockPlace` values.

Temporary Memory allocation is owned by this package. Minimal allocates locals sequentially, Fast falls back to conservative liveness reuse when sequential allocation exceeds 4096 slots, and Standard uses deterministic size-first first-fit coloring of the local interference graph. Backend finalization only accepts allocated final-form IR.

Backend SNode peephole rules follow `sonolus.js-compiler` commit `37b0eee`, `src/snode/optimize`. They run bottom-up after IR finalization and preserve evaluation of eliminated dynamic arithmetic arguments with `Execute`.

The pinned Python optimizer golden and regeneration harness live under
`internal/compiler/testdata/optimize`. Schema v3 uses one neutral JSON CFG
fixture parsed independently by Go and Python. Tests compare normalized RPO CFG
snapshots after ToSSA, first SCCP cleanup, second SCCP, FromSSA, Allocate, and
the final Standard EngineData tree.

Equivalent private CFG differences are listed by exact case, checkpoint, JSON
pointer, Go value, Python value, and reason in `py_pass_allowlist.json`.
Unknown differences and stale entries both fail. A fixed input matrix executes
both final trees through `internal/simexec` and compares semantic memory and
ordered effects; Temporary Memory remains non-observable.
