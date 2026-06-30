// Package frontend — standard name constants for Sonolus engine resources.
// These mirror the standard names defined in sonolus-core / sonolus.py,
// allowing engines to reference standard sprites/effects/particles/imports
// by name instead of magic numbers.
//
// Usage in engine source:
//
//	import "std"
//	_ = StandardImport.BEAT  // declares the standard BEAT import
package frontend

// Sprites (standard skin sprite names, prefixed with '#' in the runtime).
// Source: sonolus-core-go/core/resource/skin.go
var StandardSpriteNames = []string{
	// Note heads (7 colors)
	"#NOTE_HEAD_NEUTRAL", "#NOTE_HEAD_RED", "#NOTE_HEAD_GREEN",
	"#NOTE_HEAD_BLUE", "#NOTE_HEAD_YELLOW", "#NOTE_HEAD_PURPLE", "#NOTE_HEAD_CYAN",
	// Note ticks (7 colors)
	"#NOTE_TICK_NEUTRAL", "#NOTE_TICK_RED", "#NOTE_TICK_GREEN",
	"#NOTE_TICK_BLUE", "#NOTE_TICK_YELLOW", "#NOTE_TICK_PURPLE", "#NOTE_TICK_CYAN",
	// Note tails (7 colors)
	"#NOTE_TAIL_NEUTRAL", "#NOTE_TAIL_RED", "#NOTE_TAIL_GREEN",
	"#NOTE_TAIL_BLUE", "#NOTE_TAIL_YELLOW", "#NOTE_TAIL_PURPLE", "#NOTE_TAIL_CYAN",
	// Note connections (7 colors)
	"#NOTE_CONNECTION_NEUTRAL", "#NOTE_CONNECTION_RED", "#NOTE_CONNECTION_GREEN",
	"#NOTE_CONNECTION_BLUE", "#NOTE_CONNECTION_YELLOW", "#NOTE_CONNECTION_PURPLE", "#NOTE_CONNECTION_CYAN",
	// Note connections seamless (7 colors)
	"#NOTE_CONNECTION_NEUTRAL_SEAMLESS", "#NOTE_CONNECTION_RED_SEAMLESS",
	"#NOTE_CONNECTION_GREEN_SEAMLESS", "#NOTE_CONNECTION_BLUE_SEAMLESS",
	"#NOTE_CONNECTION_YELLOW_SEAMLESS", "#NOTE_CONNECTION_PURPLE_SEAMLESS", "#NOTE_CONNECTION_CYAN_SEAMLESS",
	// Simultaneous connections (7 colors)
	"#SIMULTANEOUS_CONNECTION_NEUTRAL", "#SIMULTANEOUS_CONNECTION_RED",
	"#SIMULTANEOUS_CONNECTION_GREEN", "#SIMULTANEOUS_CONNECTION_BLUE",
	"#SIMULTANEOUS_CONNECTION_YELLOW", "#SIMULTANEOUS_CONNECTION_PURPLE", "#SIMULTANEOUS_CONNECTION_CYAN",
	// Simultaneous connections seamless (7 colors)
	"#SIMULTANEOUS_CONNECTION_NEUTRAL_SEAMLESS", "#SIMULTANEOUS_CONNECTION_RED_SEAMLESS",
	"#SIMULTANEOUS_CONNECTION_GREEN_SEAMLESS", "#SIMULTANEOUS_CONNECTION_BLUE_SEAMLESS",
	"#SIMULTANEOUS_CONNECTION_YELLOW_SEAMLESS", "#SIMULTANEOUS_CONNECTION_PURPLE_SEAMLESS",
	"#SIMULTANEOUS_CONNECTION_CYAN_SEAMLESS",
	// Directional markers (7 colors)
	"#DIRECTIONAL_MARKER_NEUTRAL", "#DIRECTIONAL_MARKER_RED",
	"#DIRECTIONAL_MARKER_GREEN", "#DIRECTIONAL_MARKER_BLUE",
	"#DIRECTIONAL_MARKER_YELLOW", "#DIRECTIONAL_MARKER_PURPLE", "#DIRECTIONAL_MARKER_CYAN",
	// Simultaneous markers (7 colors)
	"#SIMULTANEOUS_MARKER_NEUTRAL", "#SIMULTANEOUS_MARKER_RED",
	"#SIMULTANEOUS_MARKER_GREEN", "#SIMULTANEOUS_MARKER_BLUE",
	"#SIMULTANEOUS_MARKER_YELLOW", "#SIMULTANEOUS_MARKER_PURPLE", "#SIMULTANEOUS_MARKER_CYAN",
	// Stage elements
	"#STAGE_MIDDLE",
	"#STAGE_LEFT_BORDER", "#STAGE_RIGHT_BORDER", "#STAGE_TOP_BORDER", "#STAGE_BOTTOM_BORDER",
	"#STAGE_LEFT_BORDER_SEAMLESS", "#STAGE_RIGHT_BORDER_SEAMLESS",
	"#STAGE_TOP_BORDER_SEAMLESS", "#STAGE_BOTTOM_BORDER_SEAMLESS",
	"#STAGE_TOP_LEFT_CORNER", "#STAGE_TOP_RIGHT_CORNER",
	"#STAGE_BOTTOM_LEFT_CORNER", "#STAGE_BOTTOM_RIGHT_CORNER",
	"#STAGE_COVER",
	// Lanes
	"#LANE", "#LANE_SEAMLESS", "#LANE_ALTERNATIVE", "#LANE_ALTERNATIVE_SEAMLESS",
	// Judgment
	"#JUDGMENT_LINE",
	// Other
	"#NOTE_SLOT",
	// Grid (7 colors)
	"#GRID_NEUTRAL", "#GRID_RED", "#GRID_GREEN",
	"#GRID_BLUE", "#GRID_YELLOW", "#GRID_PURPLE", "#GRID_CYAN",
}

// Effects (standard effect clip names).
// Source: sonolus-core-go/core/resource/effect.go
var StandardEffectNames = []string{
	"#MISS", "#PERFECT", "#GREAT", "#GOOD",
	"#MISS_ALTERNATIVE", "#PERFECT_ALTERNATIVE", "#GREAT_ALTERNATIVE", "#GOOD_ALTERNATIVE",
	"#HOLD", "#HOLD_ALTERNATIVE",
	"#STAGE",
}

// Particles (standard particle effect names).
// Source: sonolus-core-go/core/resource/particle.go
var StandardParticleNames = []string{
	// Note circular tap (7 colors)
	"#NOTE_CIRCULAR_TAP_NEUTRAL", "#NOTE_CIRCULAR_TAP_RED",
	"#NOTE_CIRCULAR_TAP_GREEN", "#NOTE_CIRCULAR_TAP_BLUE",
	"#NOTE_CIRCULAR_TAP_YELLOW", "#NOTE_CIRCULAR_TAP_PURPLE", "#NOTE_CIRCULAR_TAP_CYAN",
	// Note circular alternative (7 colors)
	"#NOTE_CIRCULAR_ALTERNATIVE_NEUTRAL", "#NOTE_CIRCULAR_ALTERNATIVE_RED",
	"#NOTE_CIRCULAR_ALTERNATIVE_GREEN", "#NOTE_CIRCULAR_ALTERNATIVE_BLUE",
	"#NOTE_CIRCULAR_ALTERNATIVE_YELLOW", "#NOTE_CIRCULAR_ALTERNATIVE_PURPLE",
	"#NOTE_CIRCULAR_ALTERNATIVE_CYAN",
	// Note circular hold (7 colors)
	"#NOTE_CIRCULAR_HOLD_NEUTRAL", "#NOTE_CIRCULAR_HOLD_RED",
	"#NOTE_CIRCULAR_HOLD_GREEN", "#NOTE_CIRCULAR_HOLD_BLUE",
	"#NOTE_CIRCULAR_HOLD_YELLOW", "#NOTE_CIRCULAR_HOLD_PURPLE", "#NOTE_CIRCULAR_HOLD_CYAN",
	// Note linear tap (7 colors)
	"#NOTE_LINEAR_TAP_NEUTRAL", "#NOTE_LINEAR_TAP_RED",
	"#NOTE_LINEAR_TAP_GREEN", "#NOTE_LINEAR_TAP_BLUE",
	"#NOTE_LINEAR_TAP_YELLOW", "#NOTE_LINEAR_TAP_PURPLE", "#NOTE_LINEAR_TAP_CYAN",
	// Note linear alternative (7 colors)
	"#NOTE_LINEAR_ALTERNATIVE_NEUTRAL", "#NOTE_LINEAR_ALTERNATIVE_RED",
	"#NOTE_LINEAR_ALTERNATIVE_GREEN", "#NOTE_LINEAR_ALTERNATIVE_BLUE",
	"#NOTE_LINEAR_ALTERNATIVE_YELLOW", "#NOTE_LINEAR_ALTERNATIVE_PURPLE",
	"#NOTE_LINEAR_ALTERNATIVE_CYAN",
	// Note linear hold (7 colors)
	"#NOTE_LINEAR_HOLD_NEUTRAL", "#NOTE_LINEAR_HOLD_RED",
	"#NOTE_LINEAR_HOLD_GREEN", "#NOTE_LINEAR_HOLD_BLUE",
	"#NOTE_LINEAR_HOLD_YELLOW", "#NOTE_LINEAR_HOLD_PURPLE", "#NOTE_LINEAR_HOLD_CYAN",
	// Lane / Slot / Judge line
	"#LANE_CIRCULAR", "#LANE_LINEAR",
	"#SLOT_CIRCULAR", "#SLOT_LINEAR",
	"#JUDGE_LINE_CIRCULAR", "#JUDGE_LINE_LINEAR",
}

// Standard import names (standard archetype import names).
const (
	StandardImportBEAT       = "#BEAT"
	StandardImportBPM        = "#BPM"
	StandardImportTIMESCALE  = "#TIMESCALE"
	StandardImportJUDGMENT   = "#JUDGMENT"
	StandardImportACCURACY   = "#ACCURACY"
	StandardImportBPM_CHANGE = "#BPM_CHANGE"
)

// HapticType values for entity input block offset 4 (EntityInput, block 4005).
// Usage in engine source: haptic = HapticLight assigns haptic feedback intensity
// for the current touch entity.
// Reference: sonolus.js-compiler src/lib/play/enums/HapticType.ts
const (
	HapticNone   = 0
	HapticLight  = 1
	HapticMedium = 2
	HapticHeavy  = 3
	HapticLong   = 4
)

// EntityState values for entity info block offset 2 (EntityInfo, block 4003).
// Usage in engine source: if info[2] == EntityStateDespawned { ... }
// Reference: sonolus.js-compiler src/lib/play/enums/EntityState.ts
const (
	EntityStateWaiting   = 0
	EntityStateActive    = 1
	EntityStateDespawned = 2
)
