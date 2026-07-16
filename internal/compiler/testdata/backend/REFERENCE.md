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

After auditing a new reference revision, regenerate the four-mode Go audit
artifact and the JavaScript RuntimeFunction semantics golden with:

```powershell
& internal/compiler/testdata/backend/regenerate.ps1
```

The Go test suite only reads the checked-in result and does not require Python,
Node.js, or adjacent repositories.

`TestSNodeMatchesPinnedJavaScriptGolden` consumes the checked-in
`snode_golden.json` in this directory. It was generated from the pinned
JavaScript compiler and covers number serialization, peephole output, and the
child-first node pool. Unlike the four-mode audit artifact, this is a direct
cross-implementation golden comparison.

Regenerate the SNode golden separately from a checkout at the pinned commit:

```bash
internal/compiler/testdata/backend/regenerate_snode.sh ../sonolus.js-compiler/src
```

The PowerShell regeneration command compiles the Go fixture after the
implementation has been audited against the pinned reference sources and runs
the JavaScript native semantics harness. It does not execute the Python
compiler. The four-mode reference golden is therefore a checked-in semantic
audit artifact, while `runtime_native_golden.json` and `snode_golden.json` are
direct outputs from the pinned JavaScript implementation.
