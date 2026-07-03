# Compiler Resource Handling — Completeness Matrix

Status as of 2026-06-29 (updated per Stage 3 docs drift fix; Stage 5 items still pending). Cross-references:
- `internal/compiler/engine/parse_resources.go` — current Go frontend resource parsing
- `sonolus-core-go/core/resource/` — target output data structures
- `sonolus.py/sonolus/script/` — Python reference implementation

## Engine Configuration

| Feature | Go frontend | core/resource target | sonolus.py ref | Status |
|---|---|---|---|---|
| Options (slider/toggle/select) | Yes | `EngineConfiguration.Options` | `script/options.py` | ✅ |
| UI configuration (all fields) | Yes — via `type UI struct { ... }` with `sonolus:"key=value"` tags | `EngineConfiguration.UI` | `script/ui.py` | ✅ |
| Judgment error style | Yes — `judgmentErrorStyle=...` tag | `EngineConfigurationUI.JudgmentErrorStyle` | `script/ui.py` | ✅ |
| Judgment error placement | Yes — `judgmentErrorPlacement=...` tag | `EngineConfigurationUI.JudgmentErrorPlacement` | `script/ui.py` | ✅ |
| Judgment error min | Yes — `judgmentErrorMin=...` tag | `EngineConfigurationUI.JudgmentErrorMin` | `script/ui.py` | ✅ |
| Metrics | Yes — `primaryMetric=...`/`secondaryMetric=...` tags | `EngineConfigurationUI.PrimaryMetric`/`SecondaryMetric` | `script/ui.py` | ✅ |
| Visibility (per-component scale/alpha) | Yes — e.g. `menuVisibilityScale=1` | `EngineConfigurationUI.*Visibility` | `script/ui.py` | ✅ |
| Animation (judgment/combo tweens) | Yes — e.g. `judgmentAnimationScaleEase=inQuart` | `EngineConfigurationUI.*Animation` | `script/ui.py` | ✅ |

## Skin

| Feature | Go frontend | core/resource target | sonolus.py ref | Status |
|---|---|---|---|---|
| Skin data + sprites | Yes | `EngineSkinData` | `script/sprite.py` / skin.py | ✅ |
| Skin transforms | Yes | `EngineSkinDataSprite.Transform` | skin transform | ✅ Trans compose/translate/scale/rotate/transformVec implemented |

## Effect

| Feature | Go frontend | core/resource target | sonolus.py ref | Status |
|---|---|---|---|---|
| Effect data + clips | Yes | `EngineEffectData` | `script/effect.py` | ✅ |

## Particle

| Feature | Go frontend | core/resource target | sonolus.py ref | Status |
|---|---|---|---|---|
| Particle data + effects | Yes | `EngineParticleData` | `script/particle.py` | ✅ |

## Buckets

| Feature | Go frontend | core/resource target | sonolus.py ref | Status |
|---|---|---|---|---|
| Buckets + sprites | Yes | `EngineDataBucket` | `script/bucket.py` | ✅ |

## Instruction

| Feature | Go frontend | core/resource target | sonolus.py ref | Status |
|---|---|---|---|---|
| Instruction data (text+icon) | Yes | `EngineInstructionData` | `script/ui.py` | ✅ |

## Summary

Core resource types (Skin, Effect, Particle, Instruction, Buckets, Options, UI) are fully routed.
Engine configuration fields (Judgment, UI, Metrics, Visibility, Animation) are now parsable
from engine source via a `type UI struct { ... }` with `sonolus:"key=value"` tags.

## Next Steps
1. Verify skin sprite transform coverage **✅ Done (2026-06-29)** — Transform2d compose/translate/scale/rotate/transformVec
2. Add Judgment/Metric/Visibility/Animation config parsing from engine source **✅ Done (2026-06-28)**
3. Engine data value model now has explicit `NumKind` (Scalar/Record/Array) with array-of-records support **✅ Done (2026-06-29)**
4. Container types (VarArray, ArrayMap, ArraySet, FrozenNumSet) with sort/search/iterate protocol — **✅ Done (2026-07)** (see `internal/compiler/frontend/containers.go`)
