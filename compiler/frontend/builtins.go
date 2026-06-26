package frontend

import (
	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

	"github.com/WindowsSov8forUs/sonolus-go/compiler/ir"
)

// runtimeFn is a callable runtime operation exposed to engine source.
type runtimeFn struct {
	op    resource.RuntimeFunction
	pure  bool // pure functions return a value; impure ones are statements
	arity int  // -1 = variadic (the author supplies what the runtime expects)
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
	"unlerp":        {resource.RuntimeFunctionUnlerp, true, 3},
	"unlerpClamped": {resource.RuntimeFunctionUnlerpClamped, true, 3},
	"remap":         {resource.RuntimeFunctionRemap, true, 5},
	"remapClamped":  {resource.RuntimeFunctionRemapClamped, true, 5},

	// Side-effecting (variadic).
	"draw":            {resource.RuntimeFunctionDraw, false, -1},
	"play":            {resource.RuntimeFunctionPlay, false, -1},
	"playScheduled":   {resource.RuntimeFunctionPlayScheduled, false, -1},
	"spawn":           {resource.RuntimeFunctionSpawn, false, -1},
	"spawnParticle":   {resource.RuntimeFunctionSpawnParticleEffect, false, -1},
	"moveParticle":    {resource.RuntimeFunctionMoveParticleEffect, false, -1},
	"destroyParticle": {resource.RuntimeFunctionDestroyParticleEffect, false, -1},
	"print":           {resource.RuntimeFunctionPrint, false, -1},
}

// modeAccessors are the runtime reads exposed as bare identifiers, per mode
// (field offsets from sonolus.py runtime.py; block 1000 = RuntimeEnvironment,
// 1001 = RuntimeUpdate except Preview where 1001 = RuntimeCanvas). All read-only.
var modeAccessors = map[ir.Mode]map[string]Binding{
	ir.ModePlay: {
		"time":          {Block: 1001, Index: 0},
		"deltaTime":     {Block: 1001, Index: 1},
		"scaledTime":    {Block: 1001, Index: 2},
		"touchCount":    {Block: 1001, Index: 3},
		"isDebug":       {Block: 1000, Index: 0},
		"aspectRatio":   {Block: 1000, Index: 1},
		"audioOffset":   {Block: 1000, Index: 2},
		"inputOffset":   {Block: 1000, Index: 3},
		"isMultiplayer": {Block: 1000, Index: 4},
		"safeAreaXMin":  {Block: 1000, Index: 5},
		"safeAreaXMax":  {Block: 1000, Index: 6},
		"safeAreaYMin":  {Block: 1000, Index: 7},
		"safeAreaYMax":  {Block: 1000, Index: 8},
	},
	ir.ModeWatch: {
		"time":         {Block: 1001, Index: 0},
		"deltaTime":    {Block: 1001, Index: 1},
		"scaledTime":   {Block: 1001, Index: 2},
		"isSkip":       {Block: 1001, Index: 3},
		"isDebug":      {Block: 1000, Index: 0},
		"aspectRatio":  {Block: 1000, Index: 1},
		"audioOffset":  {Block: 1000, Index: 2},
		"inputOffset":  {Block: 1000, Index: 3},
		"isReplay":     {Block: 1000, Index: 4},
		"safeAreaXMin": {Block: 1000, Index: 5},
		"safeAreaXMax": {Block: 1000, Index: 6},
		"safeAreaYMin": {Block: 1000, Index: 7},
		"safeAreaYMax": {Block: 1000, Index: 8},
	},
	ir.ModePreview: {
		// RuntimeCanvas block (1001).
		"scrollDirection": {Block: 1001, Index: 0},
		"canvasSize":      {Block: 1001, Index: 1},
		"isDebug":         {Block: 1000, Index: 0},
		"aspectRatio":     {Block: 1000, Index: 1},
		"safeAreaXMin":    {Block: 1000, Index: 2},
		"safeAreaXMax":    {Block: 1000, Index: 3},
		"safeAreaYMin":    {Block: 1000, Index: 4},
		"safeAreaYMax":    {Block: 1000, Index: 5},
	},
	ir.ModeTutorial: {
		"time":                {Block: 1001, Index: 0},
		"deltaTime":           {Block: 1001, Index: 1},
		"navigationDirection": {Block: 1001, Index: 2},
		"isDebug":             {Block: 1000, Index: 0},
		"aspectRatio":         {Block: 1000, Index: 1},
		"audioOffset":         {Block: 1000, Index: 2},
		"safeAreaXMin":        {Block: 1000, Index: 3},
		"safeAreaXMax":        {Block: 1000, Index: 4},
		"safeAreaYMin":        {Block: 1000, Index: 5},
		"safeAreaYMax":        {Block: 1000, Index: 6},
	},
}

// ModeAccessors returns a fresh map of the runtime accessors for a mode, ready
// to merge into an Env.
func ModeAccessors(mode ir.Mode) map[string]Binding {
	out := map[string]Binding{}
	for k, v := range modeAccessors[mode] {
		out[k] = v
	}
	return out
}
