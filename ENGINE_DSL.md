# Sonolus Engine DSL (Go Subset)

sonolus-go compiles a restricted subset of Go source code into Sonolus EngineData
(EngineConfiguration + EnginePlayData / EngineWatchData / EnginePreviewData /
EngineTutorialData) for use with the Sonolus game engine.

## Quick Start

```go
package myengine

type Skin struct {
    JudgeLine float64
}

type Note struct {
    Beat  float64 `sonolus:"imported"`
    X     float64 `sonolus:"memory"`
}

func (n Note) Preprocess() {
    n.X = n.Beat * 0.5
}

func (n Note) Initialize() {
    Draw(JudgeLine, n.X, 0, 0, 0, 0, 0, 0, 0, 0)
}

func (n Note) Touch() {
    if n.X > 0.5 {
        PlayLooped(1) // SFX
    }
    Despawn(0)
}
```

Compile with:

```bash
sonolus-go build engine.go -m play -O 2
```

## Supported Constructs

### Package Declaration

```go
package <name>
```

The package name is used as the engine name. Only `package main`-style
declarations are supported (the package name itself is used for labeling).

### Struct Definitions

```go
type Name struct {
    Field Type `sonolus:"tag"`
}
```

Fields use struct tags to control engine semantics:

| Tag | Block | Writable | Description |
|-----|-------|----------|-------------|
| `imported` | EntityMemory (0) | No | Imported from parent archetype |
| `memory` | EntityMemory (1+) | Yes | Private per-entity storage |
| `data` | EntityData | No | Read-only archetype data |
| `shared` | EntityShared | Yes | Shared mutable state |
| `input` | EntityInput | Yes | Input state |
| `despawn` | EntityDespawn | Yes | Despawn effect storage |
| `info` | EntityInfo | No | Read-only entity metadata |
| `exported` | Exported | Yes | Exported value (Play only) |
| `scored` | Exported | Yes | Score counter (Play only) |
| `lifed` | Exported | Yes | Life value (Play only) |

**Skin/Buckets/Effect/Particle/Instruction**: Special struct names per mode
that define engine resources:
- `Skin` — sprite/texture definitions (all modes)
- `Effect` — sound effect definitions (Play, Watch, Tutorial)
- `Particle` — particle effect definitions (Play, Watch, Tutorial)
- `Buckets` — bucket/spawn definitions (Play, Watch)
- `Instruction` — tutorial text/icon definitions (Tutorial only)

### Methods (Callbacks)

Methods on archetype structs define callbacks. The method name determines which
callback is compiled:

**Play mode (8 callbacks)**:
`Preprocess`, `SpawnOrder`, `ShouldSpawn`, `Initialize`,
`UpdateSequential`, `Touch`, `UpdateParallel`, `Terminate`

**Watch mode (7 callbacks)**:
`Preprocess`, `SpawnTime`, `DespawnTime`, `Initialize`,
`UpdateSequential`, `UpdateParallel`, `Terminate`

**Preview mode (2 callbacks)**:
`Preprocess`, `Render`

**Tutorial mode (3 global functions)**:
`Preprocess`, `Navigate`, `Update`

**Watch mode global**: `UpdateSpawn` (standalone function, not a method)

### Control Flow

```go
// If/else
if condition {
    statements
} else {
    statements
}

// Switch (with optional tag and fallthrough)
switch value {
case 0:
    statements
case 1, 2:
    statements
    fallthrough
default:
    statements
}

// Tagless switch
switch {
case x > 0:
    statements
default:
    statements
}

// For loop (condition only)
for i < 10 {
    statements
}

// For range
for i := range 5 {
    statements   // i is 0, 1, 2, 3, 4
}

// Short-circuit operators
if a && b { ... }   // b evaluated only if a is true
if a || b { ... }   // b evaluated only if a is false
```

### Variables and Assignments

```go
x := 1.0           // declare and assign (float64)
x = 2.0            // reassign
x += 1.0           // compound assignment
x++                // increment
x--                // decrement
```

All numeric types are `float64` at runtime. Integer literals are automatically
promoted.

### Type System

Only these types are supported:
- **Numeric**: `float64`, `int`, `bool` (all map to float64 at runtime)
- **Records**: `Vec2` (2D vector), `Mat3x3` (3x3 matrix), `Quad` (quadratic bezier),
  `Trans` (2D transform), `Rect` (rectangle), `Record` (key-value container)
- **Structs**: User-defined archetype/resource structs
- **Slices**: Only fixed-size arrays via record types

### Runtime Functions

#### Arithmetic
`Add`, `Subtract`, `Multiply`, `Divide`, `Power`, `Mod`, `Rem`, `Negate`, `Abs`, `Sign`

#### Comparison
`Equal`, `NotEqual`, `Less`, `LessOr`, `Greater`, `GreaterOr`

#### Logic
`And`, `Or`, `Not`

#### Math
`Sin`, `Cos`, `Tan`, `Sinh`, `Cosh`, `Tanh`, `Asin`, `Acos`, `Atan`, `Atan2`
`Log`, `Ceil`, `Floor`, `Round`, `Frac`, `Rad`, `Deg`
`Max`, `Min`, `Clamp`
`Lerp`, `LerpClamped`, `Remap`, `RemapClamped`

#### Control
`Execute`, `Execute0`, `If`, `While`, `Switch`, `SwitchInteger`,
`SwitchWithDefault`, `SwitchIntegerWithDefault`, `Block`, `Break`

#### Memory
`Get`, `GetShifted`, `Set`, `SetShifted`
`GetPointed`, `SetPointed`, `DecrementPre`, `DecrementPost`,
`DecrementPrePointed`, `DecrementPostPointed`, `DecrementPreShifted`,
`DecrementPostShifted`

#### Entity Management
`Spawn`, `Despawn`, `Move`

#### Drawing
`Draw`, `DrawCurvedL`, `DrawCurvedR`, `DrawCurvedT`, `DrawCurvedB`,
`DrawCurvedLR`, `DrawCurvedBT`

#### Audio
`PlayLooped`, `PlayLoopedScheduled`, `StopLooped`, `StopLoopedScheduled`

#### Particles
`SpawnParticleEffect`, `MoveParticleEffect`, `DestroyParticleEffect`

#### Easing (32 functions)
`EaseInSine`, `EaseOutSine`, `EaseInOutSine`, `EaseOutInSine`
`EaseInQuad`, `EaseOutQuad`, `EaseInOutQuad`, `EaseOutInQuad`
`EaseInCubic`, `EaseOutCubic`, `EaseInOutCubic`, `EaseOutInCubic`
`EaseInQuart`, `EaseOutQuart`, `EaseInOutQuart`, `EaseOutInQuart`
`EaseInQuint`, `EaseOutQuint`, `EaseInOutQuint`, `EaseOutInQuint`
`EaseInExpo`, `EaseOutExpo`, `EaseInOutExpo`, `EaseOutInExpo`
`EaseInCirc`, `EaseOutCirc`, `EaseInOutCirc`, `EaseOutInCirc`
`EaseInBack`, `EaseOutBack`, `EaseInOutBack`, `EaseOutInBack`
`EaseInElastic`, `EaseOutElastic`, `EaseInOutElastic`, `EaseOutInElastic`

#### Timing
`Time`, `TimeToBeat`, `BeatToTime`, `BeatToBPM`, `BeatToStartingBeat`,
`BeatToStartingTime`

#### Scores and Life
`AddScore`, `AddLife`, `AddLifeScheduled`, `Judge`, `JudgeSimple`

#### State
`EntityInfo`, `EntityState`, `IsDebug`, `HasSkinSprite`, `HasEffectClip`,
`HasParticleEffect`

#### Streams
`StreamSet`, `StreamHas`, `StreamGetNextKey`, `StreamGetPreviousKey`,
`StreamGetValue`

#### Haptics
`Haptic` (Play mode only)

#### Debug
`debugLog`, `debugPause`, `debugError`, `debugRequire`,
`debugAssertTrue`, `debugAssertFalse`, `debugTerminate`

## Unsupported Go Constructs

These standard Go features are NOT supported:

- `defer`, `go` (goroutines), `select`, `channel`
- `map`, `interface`, type assertions, type switches
- Methods on non-archetype types
- Closures, anonymous functions, function values
- Variadic functions (`...`)
- `import` for sub-packages is now **supported** (see Multi-File Engines below). Stdlib imports (`"fmt"`, `"math"`) are allowed but produce no IR.
- Struct embedding, nested structs (only simple field types)
- Packages other than `main`

## Multi-File Engines & Package Imports

sonolus-go supports organizing large engines across multiple files and directories.

### Single Directory (Same Package)

All `.go` files sharing the same package name in the engine root:

```
my-engine/
├── engine.go      // package myengine — Skin, resources, UpdateSpawn
├── notes.go       // package myengine — Note archetype
└── helpers.go     // package myengine — helper functions
```

```bash
sonolus-go build ./my-engine/ -m play
```

No `import` needed — files are merged into one package automatically.

### Sub-Directory Imports

Archetypes can live in sub-directories as separate packages:

```
my-engine/
├── engine.go           // package myengine
├── notes/
│   ├── note.go         // package notes — Note
│   └── flick.go        // package notes — FlickNote
└── stage/
    └── stage.go        // package stage — Stage
```

**engine.go:**
```go
package myengine
import "notes"    // loads ./notes/*.go
import "stage"    // loads ./stage/*.go
type Skin struct { Note float64 }
func UpdateSpawn() float64 { return 0 }
```

**notes/note.go:**
```go
package notes
type Note struct { Beat float64 `sonolus:"imported"` }
func (n *Note) Initialize() { debugPause() }
```

**Rules:**
- Bare name `"notes"` → resolves to `./notes/` relative to engine root
- Paths with `.` (`"fmt"`, `"math"`) → treated as stdlib (resolved but not compiled)
- Resource types (Skin, Effect, Buckets, Instruction) **only in main package**
- Archetypes from all imported packages are **auto-discovered**
- Duplicate archetype names across packages → error

## Standard Go Import: the `sonolus` Package

For full IDE support (gopls autocomplete, go vet, static analysis), engine source can
import the `sonolus` declaration package and use qualified function calls:

```go
package myengine

import "github.com/WindowsSov8forUs/sonolus-go/sonolus"

type Note struct {
    Beat float64 `sonolus:"imported"`
}

func (n *Note) Initialize() {
    sonolus.Draw(1, n.Beat, 0, 1, 1, 0, 1, 0, 0)
}
```

**Name mapping**: PascalCase in the import → lowercase-first for the runtime function.

| sonolus call | Runtime function |
|---|---|
| `sonolus.Draw(...)` | `draw` |
| `sonolus.Sin(x)` | `sin` |
| `sonolus.Get(block, idx)` | `get` |
| `sonolus.DebugPause()` | `debugPause` |
| `sonolus.GetShifted(...)` | `getShifted` |

**Both import forms are equivalent**:

```go
import "sonolus"                                          // short form
import "github.com/WindowsSov8forUs/sonolus-go/sonolus"   // full module path
```

**Mixed styles**: bare (lowercase) and qualified calls can coexist in the same source file.

For engine projects using the `sonolus` import, create a `go.mod`:

```go
module my-engine
require github.com/WindowsSov8forUs/sonolus-go v0.0.0
replace github.com/WindowsSov8forUs/sonolus-go => ../sonolus-go
```

Then standard Go tooling works normally: `gopls` provides autocomplete, `go vet` runs
static analysis, and the `sonolus-go` compiler translates qualified calls to Sonolus IR.

## Optimization Levels

```bash
-O 0  # Minimal: basic cleanup only (fastest compilation)
-O 1  # Fast: single SSA round with SCCP + one inlining pass
-O 2  # Standard: full pipeline with LICM + CSE + AdvancedDCE (default)
```

## Compilation Modes

```bash
-m play      # Interactive gameplay (8 callbacks, exports, score/life)
-m watch     # Replay playback (7 callbacks + global UpdateSpawn)
-m preview   # Chart preview (2 callbacks: Preprocess + Render)
-m tutorial  # Tutorial (3 global callbacks: Preprocess + Navigate + Update)
-m all       # Compile all four modes
```

## Reference

- **Optimizer pipeline**: `internal/compiler/ir/optimize/PIPELINE.md`
- **Full builtin list**: `internal/compiler/frontend/builtins.go`
- **Example engines**: `internal/compiler/engine/testdata/*.go`
- **sonolus-core-go types**: `../sonolus-core-go/core/resource/`
