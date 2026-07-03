# Supported Go Subset — Frontend Syntax Matrix

The `internal/compiler/frontend` package compiles a subset of Go source code into the
Sonolus engine IR. The supported subset is intentionally limited to constructs
that map naturally to the engine runtime model.

## Supported

| Construct | Notes |
|---|---|
| Package declaration | Standard `package p` header (ignored) |
| Import declarations | Standard Go imports (for stdlib only) |
| Top-level type declarations | Structs with `sonolus:` tags for archetypes/resources |
| Top-level functions | Free functions become helpers (inlinable) |
| Methods on structs | Callback bodies (`Initialize`, `UpdateParallel`, etc.) |
| `if`/`else` | Standard conditional |
| `for` loops | Standard `for init; cond; inc` and `for cond` (while) |
| `range` loops | `for i := range n` iterates `[0, n)` |
| `switch` | Single-expression tag switch with constant cases |
| Assignment `=`, `:=` | Local variables and archetype fields |
| Compound assignment `+=`, `-=`, `*=`, `/=` | Expanded to read-modify-write |
| Increment/decrement `++`, `--` | Supported |
| `return` statement | With or without value |
| `break`, `continue` | Inside loops |
| Arithmetic: `+`, `-`, `*`, `/`, `%` | `%` maps to floored modulo (Python `%`) |
| Comparison: `==`, `!=`, `<`, `<=`, `>`, `>=` | Standard |
| Logical: `&&`, `||`, `!` | Short-circuit evaluation |
| Unary: `+`, `-`, `!` | Numeric/boolean negation |
| Method calls | On engine types (Vec2, Quad, Mat, Rect, Trans) |
| Builtin function calls | ~200 runtime builtins (math, draw, audio, etc.) |
| Composite literals `Type{}` | Record construction |
| Field access `x.Field` | Record field read |
| Index `x[i]` | Array access |
| Integer and boolean literals | Constant folding |
| Float literals | Constant folding |
| String literals | For tag parsing only |
| Callback helpers | `Initialize()`, `UpdateParallel(dt)`, `Touch()`, etc. |
| UI struct (`type UI struct{...}`) | `sonolus:"key=value"` tags override EngineConfigurationUI defaults |

## Unsupported (by design)

These Go constructs are rejected with "unsupported" errors during compilation.
They don't have meaningful equivalents in the Sonolus runtime model.

| Construct | Error | Reason |
|---|---|---|
| `defer` | `unsupported statement` | No runtime defer mechanism |
| `go` (goroutines) | `unsupported statement` | Engine is single-threaded |
| `chan`, `select` | Not parsed | No concurrency support |
| `func` literals (closures) | `unsupported statement` | No closure support |
| `interface`, `map` | Not parsed | No heap types |
| `type` (non-struct) | Ignored / not parsed | Only structs are meaningful |
| `slice` | Not supported | Limited container support |
| `append` | Not supported | Use VarArray (planned) |
| `copy` | Supported | Deep copy builtin (`."copy"`) |
| `panic`, `recover` | `unsupported statement` | No panic support |
| Complex `switch` cases | Rejected if non-constant | Tag must be a single expression |
| Multi-value returns | Limited | Only single-value returns |
| Exported identifiers | Restricted | Only builtins and engine types |
| Recursive helpers | Rejected | No recursion support |

## Comparing to sonolus.py / sonolus.js-compiler

| Feature | sonolus-go | sonolus.py | sonolus.js-compiler |
|---|---|---|---|
| Host language | Go (subset) | Python (subset) | JavaScript (subset) |
| Parser | `go/parser` + `go/ast` | `ast` (Python stdlib) | `acorn` (npm) |
| Type checking | `go/types` | Runtime tracing | None (duck-typed) |
| Record types | `struct` + `sonolus:` tags | Class + decorators | Object shapes |
| Builtins | ~200 runtime functions | ~200+ runtime functions | ~200+ runtime functions |
| IR output | CFG + SSA optimize | Same CFG + SSA | Rich IR nodes |
