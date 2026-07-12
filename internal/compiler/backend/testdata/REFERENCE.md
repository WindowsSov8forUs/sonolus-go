# Backend reference provenance

The checked-in `reference.golden.json` records the normalized four-mode output
for `internal/compiler/testdata/reference`.

- sonolus.py: `1040bc0dcc116efdbca05f144edec302e839bcd3`
- sonolus.js-compiler: `37b0eee5aa16d1e01973d33d625d86f5ef72d268`

Static EngineData assembly follows the JS builders under
`src/build/{play,watch,preview,tutorial}`. CFG finalization and non-finite ROM
constants follow `sonolus/backend/finalize.py` and `sonolus/backend/ops.py`.
Node indexes are deliberately excluded from the golden: callback roots are
expanded into RuntimeFunction trees so child-first deduplication differences do
not appear as semantic differences.

After auditing a new reference revision, regenerate with:

```powershell
& internal/compiler/backend/testdata/regenerate.ps1
```

The Go test suite only reads the checked-in result and does not require Python,
Node.js, or adjacent repositories.

`TestSNodeMatchesPinnedJavaScriptGolden` consumes the checked-in
`snode_golden.json` in this directory. It was generated from the pinned
JavaScript compiler and covers number serialization, peephole output, and the
child-first node pool. Unlike the four-mode audit artifact, this is a direct
cross-implementation golden comparison.

The regeneration command compiles the Go fixture after the implementation has
been audited against the pinned reference sources. It does not execute the
Python or JavaScript compilers, so the golden is a checked-in semantic audit
artifact rather than an independently cross-compiled output.
