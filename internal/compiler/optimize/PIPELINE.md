# compiler optimizer provenance

The optimizer pipeline follows `sonolus.py` commit `1040bc0`:

- `MINIMAL_PASSES`: `CoalesceFlow`, unreachable-code elimination, and basic allocation.
- `FAST_PASSES`: `CoalesceFlow`, unreachable-code elimination, `TryAllocateBasic`, and final flow coalescing. Fast never enters SSA.
- `STANDARD_PASSES`: the pass order in `sonolus/backend/optimize/optimize.py`, including SSA construction/destruction, SCCP, DCE, inlining, associative simplification, switch rewriting, LICM, CSE, copy coalescing, and liveness allocation.

The Go IR represents CFG edges and Phi arguments explicitly, so CFG passes also update predecessor IDs and split critical Phi edges before SSA destruction. Mode-specific memory has already been normalized by the frontend catalog; `NormalizeBlocks` therefore normalizes CFG order rather than coercing Python `BlockPlace` values.

Temporary Memory allocation is owned by this package. Minimal allocates locals sequentially, Fast falls back to conservative liveness reuse when sequential allocation exceeds 4096 slots, and Standard uses deterministic size-first first-fit coloring of the local interference graph. Backend finalization only accepts allocated final-form IR.

Backend SNode peephole rules follow `sonolus.js-compiler` commit `37b0eee`, `src/snode/optimize`. They run bottom-up after IR finalization and preserve evaluation of eliminated dynamic arithmetic arguments with `Execute`.

The pinned Python optimizer golden and regeneration harness live under this
package's `testdata` directory. The Python snapshot uses an SNode-like text
format, while the Go compiler uses typed CFG blocks, values, places, and Phi
edges. It therefore remains provenance for pass behavior and ordering rather
than a byte-for-byte IR interchange format. The compiler verifies the shared
observable contract by compiling the same four-mode fixture at Minimal, Fast,
and Standard and interpreting every emitted callback tree across multiple
runtime-memory seeds.

The compiler parity tests also consume that Python-generated golden
directly. ToSSA and SCCP use a shared reverse-postorder/first-definition
canonical form. FromSSA and allocation use final-form invariants because the
Python block-object CFG and Go explicit-edge CFG split Phi critical edges into
different but equivalent block shapes. Any other structural difference must be
listed by exact snapshot with a reason; unknown differences fail the tests.
