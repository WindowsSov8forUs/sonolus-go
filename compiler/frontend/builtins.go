package frontend

import (
	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

	"github.com/WindowsSov8forUs/sonolus-go/compiler/ir"
)

// runtimeFn is a callable runtime operation exposed to engine source.
type runtimeFn struct {
	op      resource.RuntimeFunction
	returns bool // returns a value (expression); false means statement with side effects
	arity   int  // -1 = variadic (the author supplies what the runtime expects)
}

// runtimeFns maps engine-source function names to runtime operations. Pure math
// functions have fixed arity; side-effecting operations (whose exact arity is
// Sonolus-version-dependent) are variadic pass-throughs.
var runtimeFns = map[string]runtimeFn{
	// Pure math.
	"abs":           {resource.RuntimeFunctionAbs, true, 1},
	"sign":          {resource.RuntimeFunctionSign, true, 1},
	"floor":         {resource.RuntimeFunctionFloor, true, 1},
	"ceil":          {resource.RuntimeFunctionCeil, true, 1},
	"round":         {resource.RuntimeFunctionRound, true, 1},
	"trunc":         {resource.RuntimeFunctionTrunc, true, 1},
	"frac":          {resource.RuntimeFunctionFrac, true, 1},
	"log":           {resource.RuntimeFunctionLog, true, 1},
	"sin":           {resource.RuntimeFunctionSin, true, 1},
	"cos":           {resource.RuntimeFunctionCos, true, 1},
	"tan":           {resource.RuntimeFunctionTan, true, 1},
	"sinh":          {resource.RuntimeFunctionSinh, true, 1},
	"cosh":          {resource.RuntimeFunctionCosh, true, 1},
	"tanh":          {resource.RuntimeFunctionTanh, true, 1},
	"asin":          {resource.RuntimeFunctionArcsin, true, 1},
	"acos":          {resource.RuntimeFunctionArccos, true, 1},
	"atan":          {resource.RuntimeFunctionArctan, true, 1},
	"atan2":         {resource.RuntimeFunctionArctan2, true, 2},
	"degree":        {resource.RuntimeFunctionDegree, true, 1},
	"radian":        {resource.RuntimeFunctionRadian, true, 1},
	"min":           {resource.RuntimeFunctionMin, true, 2},
	"max":           {resource.RuntimeFunctionMax, true, 2},
	"clamp":         {resource.RuntimeFunctionClamp, true, 3},
	"power":         {resource.RuntimeFunctionPower, true, 2},
	"mod":           {resource.RuntimeFunctionMod, true, 2},
	"rem":           {resource.RuntimeFunctionRem, true, 2},
	"lerp":          {resource.RuntimeFunctionLerp, true, 3},
	"lerpClamped":   {resource.RuntimeFunctionLerpClamped, true, 3},
	"interp":        {resource.RuntimeFunctionLerp, true, 3},
	"interpClamped": {resource.RuntimeFunctionLerpClamped, true, 3},
	"unlerp":        {resource.RuntimeFunctionUnlerp, true, 3},
	"unlerpClamped": {resource.RuntimeFunctionUnlerpClamped, true, 3},
	"remap":         {resource.RuntimeFunctionRemap, true, 5},
	"remapClamped":  {resource.RuntimeFunctionRemapClamped, true, 5},

	// Easing functions (pure, arity 1).
	"easeInBack":       {resource.RuntimeFunctionEaseInBack, true, 1},
	"easeInCirc":       {resource.RuntimeFunctionEaseInCirc, true, 1},
	"easeInCubic":      {resource.RuntimeFunctionEaseInCubic, true, 1},
	"easeInElastic":    {resource.RuntimeFunctionEaseInElastic, true, 1},
	"easeInExpo":       {resource.RuntimeFunctionEaseInExpo, true, 1},
	"easeInOutBack":    {resource.RuntimeFunctionEaseInOutBack, true, 1},
	"easeInOutCirc":    {resource.RuntimeFunctionEaseInOutCirc, true, 1},
	"easeInOutCubic":   {resource.RuntimeFunctionEaseInOutCubic, true, 1},
	"easeInOutElastic": {resource.RuntimeFunctionEaseInOutElastic, true, 1},
	"easeInOutExpo":    {resource.RuntimeFunctionEaseInOutExpo, true, 1},
	"easeInOutQuad":    {resource.RuntimeFunctionEaseInOutQuad, true, 1},
	"easeInOutQuart":   {resource.RuntimeFunctionEaseInOutQuart, true, 1},
	"easeInOutQuint":   {resource.RuntimeFunctionEaseInOutQuint, true, 1},
	"easeInOutSine":    {resource.RuntimeFunctionEaseInOutSine, true, 1},
	"easeInQuad":       {resource.RuntimeFunctionEaseInQuad, true, 1},
	"easeInQuart":      {resource.RuntimeFunctionEaseInQuart, true, 1},
	"easeInQuint":      {resource.RuntimeFunctionEaseInQuint, true, 1},
	"easeInSine":       {resource.RuntimeFunctionEaseInSine, true, 1},
	"easeOutBack":      {resource.RuntimeFunctionEaseOutBack, true, 1},
	"easeOutCirc":      {resource.RuntimeFunctionEaseOutCirc, true, 1},
	"easeOutCubic":     {resource.RuntimeFunctionEaseOutCubic, true, 1},
	"easeOutElastic":   {resource.RuntimeFunctionEaseOutElastic, true, 1},
	"easeOutExpo":      {resource.RuntimeFunctionEaseOutExpo, true, 1},
	"easeOutInBack":    {resource.RuntimeFunctionEaseOutInBack, true, 1},
	"easeOutInCirc":    {resource.RuntimeFunctionEaseOutInCirc, true, 1},
	"easeOutInCubic":   {resource.RuntimeFunctionEaseOutInCubic, true, 1},
	"easeOutInElastic": {resource.RuntimeFunctionEaseOutInElastic, true, 1},
	"easeOutInExpo":    {resource.RuntimeFunctionEaseOutInExpo, true, 1},
	"easeOutInQuad":    {resource.RuntimeFunctionEaseOutInQuad, true, 1},
	"easeOutInQuart":   {resource.RuntimeFunctionEaseOutInQuart, true, 1},
	"easeOutInQuint":   {resource.RuntimeFunctionEaseOutInQuint, true, 1},
	"easeOutInSine":    {resource.RuntimeFunctionEaseOutInSine, true, 1},
	"easeOutQuad":      {resource.RuntimeFunctionEaseOutQuad, true, 1},
	"easeOutQuart":     {resource.RuntimeFunctionEaseOutQuart, true, 1},
	"easeOutQuint":     {resource.RuntimeFunctionEaseOutQuint, true, 1},
	"easeOutSine":      {resource.RuntimeFunctionEaseOutSine, true, 1},

	// Side-effecting (variadic).
	"draw":            {resource.RuntimeFunctionDraw, false, -1},
	"play":            {resource.RuntimeFunctionPlay, false, -1},
	"playScheduled":   {resource.RuntimeFunctionPlayScheduled, false, -1},
	"spawn":           {resource.RuntimeFunctionSpawn, false, -1},
	"spawnParticle":   {resource.RuntimeFunctionSpawnParticleEffect, false, -1},
	"moveParticle":    {resource.RuntimeFunctionMoveParticleEffect, false, -1},
	"destroyParticle": {resource.RuntimeFunctionDestroyParticleEffect, false, -1},
	"print":           {resource.RuntimeFunctionPrint, false, -1},
	// Timing (pure, variadic).
	"beatToTime":               {resource.RuntimeFunctionBeatToTime, true, -1},
	"beatToBPM":                {resource.RuntimeFunctionBeatToBPM, true, -1},
	"timeToScaledTime":         {resource.RuntimeFunctionTimeToScaledTime, true, -1},
	"timeToStartingTime":       {resource.RuntimeFunctionTimeToStartingTime, true, -1},
	"timeToStartingScaledTime": {resource.RuntimeFunctionTimeToStartingScaledTime, true, -1},
	"beatToStartingBeat":       {resource.RuntimeFunctionBeatToStartingBeat, true, -1},
	"beatToStartingTime":       {resource.RuntimeFunctionBeatToStartingTime, true, -1},
	"timeToTimeScale":          {resource.RuntimeFunctionTimeToTimeScale, true, -1},
	// Side-effecting — canvas (preview).
	"paint": {resource.RuntimeFunctionPaint, false, -1},

	// Side-effecting — drawing.
	"drawCurvedB": {resource.RuntimeFunctionDrawCurvedB, false, -1},
	"drawCurvedT": {resource.RuntimeFunctionDrawCurvedT, false, -1},
	"drawCurvedL": {resource.RuntimeFunctionDrawCurvedL, false, -1},
	"drawCurvedR": {resource.RuntimeFunctionDrawCurvedR, false, -1},
	// Side-effecting — audio.
	"playLooped":          {resource.RuntimeFunctionPlayLooped, false, -1},
	"playLoopedScheduled": {resource.RuntimeFunctionPlayLoopedScheduled, false, -1},
	"stopLooped":          {resource.RuntimeFunctionStopLooped, false, -1},
	"stopLoopedScheduled": {resource.RuntimeFunctionStopLoopedScheduled, false, -1},
	// Side-effecting — judgment / exports.
	"judge":       {resource.RuntimeFunctionJudge, false, -1},
	"judgeSimple": {resource.RuntimeFunctionJudgeSimple, false, -1},
	"exportValue": {resource.RuntimeFunctionExportValue, false, -1},
	// Side-effecting — debug.
	"debugLog":   {resource.RuntimeFunctionDebugLog, false, -1},
	"debugPause": {resource.RuntimeFunctionDebugPause, false, -1},
	// Debug/assert — special-cased in trace_call.go (CFG branching).
	"debugError":       {resource.RuntimeFunctionDebugLog, false, 1},
	"debugRequire":     {resource.RuntimeFunctionDebugLog, false, 2},
	"debugAssertTrue":  {resource.RuntimeFunctionDebugLog, false, 2},
	"debugAssertFalse": {resource.RuntimeFunctionDebugLog, false, 2},
	"debugTerminate":   {resource.RuntimeFunctionDebugLog, false, 0},
	// Random (returns value, arity 1 — max value). NOT mathematically pure;
	// classified as side-effecting in ops_gen.go — RNG state is mutated.
	"random":        {resource.RuntimeFunctionRandom, true, 1},
	"randomInteger": {resource.RuntimeFunctionRandomInteger, true, 1},

	// Stream operations (side-effecting — read/watch/replay data).
	"streamGetValue":       {resource.RuntimeFunctionStreamGetValue, false, 1},
	"streamGetNextKey":     {resource.RuntimeFunctionStreamGetNextKey, false, 1},
	"streamGetPreviousKey": {resource.RuntimeFunctionStreamGetPreviousKey, false, 1},
	"streamHas":            {resource.RuntimeFunctionStreamHas, false, 1},
	"streamSet":            {resource.RuntimeFunctionStreamSet, false, 2},

	// Life management (side-effecting, variadic).
	"addLife": {resource.RuntimeFunctionAddLifeScheduled, false, -1},

	// Resource queries (pure).
	"hasSkinSprite": {resource.RuntimeFunctionHasSkinSprite, true, 1},
	"hasEffectClip": {resource.RuntimeFunctionHasEffectClip, true, 1},
	"hasParticle":   {resource.RuntimeFunctionHasParticleEffect, true, 1},

	// ── Stack operations (side-effecting, variadic) ──
	"stackInit":            {resource.RuntimeFunctionStackInit, false, -1},
	"stackPush":            {resource.RuntimeFunctionStackPush, false, -1},
	"stackPop":             {resource.RuntimeFunctionStackPop, false, -1},
	"stackGrow":            {resource.RuntimeFunctionStackGrow, false, -1},
	"stackEnter":           {resource.RuntimeFunctionStackEnter, false, -1},
	"stackLeave":           {resource.RuntimeFunctionStackLeave, false, -1},
	"stackGet":             {resource.RuntimeFunctionStackGet, false, -1},
	"stackSet":             {resource.RuntimeFunctionStackSet, false, -1},
	"stackGetPointer":      {resource.RuntimeFunctionStackGetPointer, false, -1},
	"stackSetPointer":      {resource.RuntimeFunctionStackSetPointer, false, -1},
	"stackGetFrame":        {resource.RuntimeFunctionStackGetFrame, false, -1},
	"stackSetFrame":        {resource.RuntimeFunctionStackSetFrame, false, -1},
	"stackGetFramePointer": {resource.RuntimeFunctionStackGetFramePointer, false, -1},
	"stackSetFramePointer": {resource.RuntimeFunctionStackSetFramePointer, false, -1},

	// ── Shifted / pointed memory access (side-effecting, variadic) ──
	"getShifted":    {resource.RuntimeFunctionGetShifted, true, -1},
	"getPointed":    {resource.RuntimeFunctionGetPointed, true, -1},
	"setShifted":    {resource.RuntimeFunctionSetShifted, false, -1},
	"setPointed":    {resource.RuntimeFunctionSetPointed, false, -1},
	"setAddShifted": {resource.RuntimeFunctionSetAddShifted, false, -1},
	"setAddPointed": {resource.RuntimeFunctionSetAddPointed, false, -1},
	"setSubShifted": {resource.RuntimeFunctionSetSubtractShifted, false, -1},
	"setSubPointed": {resource.RuntimeFunctionSetSubtractPointed, false, -1},
	"setMulShifted": {resource.RuntimeFunctionSetMultiplyShifted, false, -1},
	"setMulPointed": {resource.RuntimeFunctionSetMultiplyPointed, false, -1},
	"setDivShifted": {resource.RuntimeFunctionSetDivideShifted, false, -1},
	"setDivPointed": {resource.RuntimeFunctionSetDividePointed, false, -1},
	"setModShifted": {resource.RuntimeFunctionSetModShifted, false, -1},
	"setModPointed": {resource.RuntimeFunctionSetModPointed, false, -1},
	"setRemShifted": {resource.RuntimeFunctionSetRemShifted, false, -1},
	"setRemPointed": {resource.RuntimeFunctionSetRemPointed, false, -1},
	"setPowShifted": {resource.RuntimeFunctionSetPowerShifted, false, -1},
	"setPowPointed": {resource.RuntimeFunctionSetPowerPointed, false, -1},

	// ── Increment / decrement variants (side-effecting, variadic) ──
	"incrementPre":         {resource.RuntimeFunctionIncrementPre, false, -1},
	"incrementPost":        {resource.RuntimeFunctionIncrementPost, false, -1},
	"incrementPrePointed":  {resource.RuntimeFunctionIncrementPrePointed, false, -1},
	"incrementPostPointed": {resource.RuntimeFunctionIncrementPostPointed, false, -1},
	"incrementPreShifted":  {resource.RuntimeFunctionIncrementPreShifted, false, -1},
	"incrementPostShifted": {resource.RuntimeFunctionIncrementPostShifted, false, -1},
	"decrementPre":         {resource.RuntimeFunctionDecrementPre, false, -1},
	"decrementPost":        {resource.RuntimeFunctionDecrementPost, false, -1},
	"decrementPrePointed":  {resource.RuntimeFunctionDecrementPrePointed, false, -1},
	"decrementPostPointed": {resource.RuntimeFunctionDecrementPostPointed, false, -1},
	"decrementPreShifted":  {resource.RuntimeFunctionDecrementPreShifted, false, -1},
	"decrementPostShifted": {resource.RuntimeFunctionDecrementPostShifted, false, -1},

	// ── Drawing / misc remaining ──
	"drawCurvedBT": {resource.RuntimeFunctionDrawCurvedBT, false, -1},
	"drawCurvedLR": {resource.RuntimeFunctionDrawCurvedLR, false, -1},
	"copy":         {resource.RuntimeFunctionCopy, false, 2},
}

// modeAccessors are the runtime reads exposed as bare identifiers, per mode.
// Blocks reference ir.BlockRuntimeEnvironment (1000 = RuntimeEnvironment) and
// ir.BlockRuntimeUpdate (1001, RuntimeUpdate; RuntimeCanvas in Preview mode). All
// read-only.
var modeAccessors = map[ir.Mode]map[string]Binding{
	ir.ModePlay: {
		"time":              {Block: ir.BlockRuntimeUpdate, Index: 0},
		"deltaTime":         {Block: ir.BlockRuntimeUpdate, Index: 1},
		"scaledTime":        {Block: ir.BlockRuntimeUpdate, Index: 2},
		"touchCount":        {Block: ir.BlockRuntimeUpdate, Index: 3},
		"isDebug":           {Block: ir.BlockRuntimeEnvironment, Index: 0},
		"aspectRatio":       {Block: ir.BlockRuntimeEnvironment, Index: 1},
		"audioOffset":       {Block: ir.BlockRuntimeEnvironment, Index: 2},
		"inputOffset":       {Block: ir.BlockRuntimeEnvironment, Index: 3},
		"isMultiplayer":     {Block: ir.BlockRuntimeEnvironment, Index: 4},
		"safeAreaXMin":      {Block: ir.BlockRuntimeEnvironment, Index: 5},
		"safeAreaXMax":      {Block: ir.BlockRuntimeEnvironment, Index: 6},
		"safeAreaYMin":      {Block: ir.BlockRuntimeEnvironment, Index: 7},
		"safeAreaYMax":      {Block: ir.BlockRuntimeEnvironment, Index: 8},
		"perfectMultiplier": {Block: 2004, Index: 0},
		"greatMultiplier":   {Block: 2004, Index: 1},
		"goodMultiplier":    {Block: 2004, Index: 2},
		"lifeInitial":       {Block: 2005, Index: 6},
		"lifeMaximum":       {Block: 2005, Index: 7},
	},
	ir.ModeWatch: {
		"time":         {Block: ir.BlockRuntimeUpdate, Index: 0},
		"deltaTime":    {Block: ir.BlockRuntimeUpdate, Index: 1},
		"scaledTime":   {Block: ir.BlockRuntimeUpdate, Index: 2},
		"isSkip":       {Block: ir.BlockRuntimeUpdate, Index: 3},
		"isDebug":      {Block: ir.BlockRuntimeEnvironment, Index: 0},
		"aspectRatio":  {Block: ir.BlockRuntimeEnvironment, Index: 1},
		"audioOffset":  {Block: ir.BlockRuntimeEnvironment, Index: 2},
		"inputOffset":  {Block: ir.BlockRuntimeEnvironment, Index: 3},
		"isReplay":     {Block: ir.BlockRuntimeEnvironment, Index: 4},
		"safeAreaXMin": {Block: ir.BlockRuntimeEnvironment, Index: 5},
		"safeAreaXMax": {Block: ir.BlockRuntimeEnvironment, Index: 6},
		"safeAreaYMin": {Block: ir.BlockRuntimeEnvironment, Index: 7},
		"safeAreaYMax": {Block: ir.BlockRuntimeEnvironment, Index: 8},
	},
	ir.ModePreview: {
		// RuntimeCanvas block (1001).
		"scrollDirection": {Block: ir.BlockRuntimeUpdate, Index: 0},
		"canvasSize":      {Block: ir.BlockRuntimeUpdate, Index: 1},
		"isDebug":         {Block: ir.BlockRuntimeEnvironment, Index: 0},
		"aspectRatio":     {Block: ir.BlockRuntimeEnvironment, Index: 1},
		"safeAreaXMin":    {Block: ir.BlockRuntimeEnvironment, Index: 2},
		"safeAreaXMax":    {Block: ir.BlockRuntimeEnvironment, Index: 3},
		"safeAreaYMin":    {Block: ir.BlockRuntimeEnvironment, Index: 4},
		"safeAreaYMax":    {Block: ir.BlockRuntimeEnvironment, Index: 5},
	},
	ir.ModeTutorial: {
		"time":                {Block: ir.BlockRuntimeUpdate, Index: 0},
		"deltaTime":           {Block: ir.BlockRuntimeUpdate, Index: 1},
		"navigationDirection": {Block: ir.BlockRuntimeUpdate, Index: 2},
		"isDebug":             {Block: ir.BlockRuntimeEnvironment, Index: 0},
		"aspectRatio":         {Block: ir.BlockRuntimeEnvironment, Index: 1},
		"audioOffset":         {Block: ir.BlockRuntimeEnvironment, Index: 2},
		"safeAreaXMin":        {Block: ir.BlockRuntimeEnvironment, Index: 3},
		"safeAreaXMax":        {Block: ir.BlockRuntimeEnvironment, Index: 4},
		"safeAreaYMin":        {Block: ir.BlockRuntimeEnvironment, Index: 5},
		"safeAreaYMax":        {Block: ir.BlockRuntimeEnvironment, Index: 6},
	},
}

// CloneBindings shallow-copies a name→Binding map so callers can add per-archetype
// field bindings without mutating a shared table.
func CloneBindings(src map[string]Binding) map[string]Binding {
	out := make(map[string]Binding, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

// ModeAccessors returns a shallow copy of the mode's accessor bindings so
// callers can add per-archetype field bindings. For read-only use, prefer
// ModeAccessorsReadOnly to avoid allocation.
func ModeAccessors(mode ir.Mode) map[string]Binding {
	return CloneBindings(modeAccessors[mode])
}

// ModeAccessorsReadOnly returns the mode accessors without cloning.
// The returned map MUST NOT be modified — use CloneBindings if you
// need per-archetype additions.
func ModeAccessorsReadOnly(mode ir.Mode) map[string]Binding {
	return modeAccessors[mode]
}
