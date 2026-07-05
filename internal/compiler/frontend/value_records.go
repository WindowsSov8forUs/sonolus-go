package frontend

import (
	"fmt"
	"go/token"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/ir"
)

// recordMethodEntry bundles a record-method implementation with arity and
// type-safety guards. minArity is the minimum number of arguments; args
// at indices in compositeArgAt (0-based relative to the supplied args,
// NOT including the receiver) must be composite values.
//
// If containerFn is set (non-nil), it is called instead of fn when the
// receiver is a container-backed variable (VarArray, ArrayMap, ArraySet),
// giving the implementation access to the containerInfo for direct IR
// emission to the backing array.
type recordMethodEntry struct {
	fn             func(*tracer, Num, []Num) (Num, error)
	containerFn    func(*tracer, *containerInfo, Num, []Num) (Num, error)
	minArity       int
	compositeArgAt []int
}

// builtinRecordDef describes a built-in record type: its constructor name
// (lowerCamelCase) and ordered field names. This single registry replaces the
// previously duplicated lists in knownRecordFields, PreludeSource, compositeDecl,
// and resolveBuiltinCall.
type builtinRecordDef struct {
	name   string
	fields []string
}

// builtinRecords lists every built-in record type that has fields known to the
// compiler. Container types (varArray, arrayMap, arraySet, frozenNumSet) and
// handle types (loopedEffectHandle, etc.) are included so that knownRecordFields
// and PreludeSource stay in sync automatically.
var builtinRecords = []builtinRecordDef{
	{"vec2", vec2Fields},
	{"quad", quadFields},
	{"mat", matFields},
	{"rect", rectFields},
	{"trans", transFields},
	{"judgmentWindow", judgmentWindowFields},
	{"sprite", spriteFields},
	{"effect", effectFields},
	{"effectClip", effectFields},
	{"entityInfo", entityInfoFields},
	{"particle", particleFields},
	{"particleClip", particleFields},
	{"entityRef", entityRefFields},
	{"pair", pairFields},
	{"varArray", varArrayFields},
	{"arrayMap", arrayMapFields},
	{"arraySet", arraySetFields},
	{"box", boxFields},
	{"printOptions", printOptionsFields},
	{"PrintOptions", printOptionsFields},
	{"consecutiveLife", consecutiveLifeFields},
	{"frozenNumSet", frozenNumSetFields},
	{"loopedEffectHandle", loopedEffectHandleFields},
	{"scheduledLoopedEffectHandle", scheduledLoopedEffectHandleFields},
	{"particleHandle", particleHandleFields},
}

// recordTypeNameFromFields matches a field list against builtin records and
// returns the record type name (e.g. ["t","r","b","l"] → "rect").
func recordTypeNameFromFields(fields []string) string {
	fieldSet := make(map[string]bool, len(fields))
	for _, f := range fields {
		fieldSet[f] = true
	}
	for _, rd := range builtinRecords {
		if len(rd.fields) != len(fields) {
			continue
		}
		match := true
		for _, f := range rd.fields {
			if !fieldSet[f] {
				match = false
				break
			}
		}
		if match {
			return rd.name
		}
	}
	return ""
}

// builtinRecordFields returns the fields for name if it is a built-in record.
func builtinRecordFields(name string) ([]string, bool) {
	for _, r := range builtinRecords {
		if r.name == name {
			return r.fields, true
		}
	}
	return nil, false
}

// knownRecordFields returns the ordered field names for a built-in or
// user-defined record type. Known built-in names: vec2, quad, mat, rect, trans.
func knownRecordFields(name string, userRecords map[string][]string) ([]string, bool) {
	if f, ok := builtinRecordFields(name); ok {
		return f, true
	}
	if f, ok := userRecords[name]; ok {
		return f, true
	}
	return nil, false
}

// recordMethods maps record type name → method name → implementation.
// Used by the table-driven dispatch in tracer.methodCall (trace.go).
var recordMethods = map[string]map[string]recordMethodEntry{
	"vec2": {
		"add":             {fn: vec2Add, minArity: 1},
		"sub":             {fn: vec2Sub, minArity: 1},
		"mul":             {fn: vec2Mul, minArity: 1},
		"div":             {fn: vec2Div, minArity: 1},
		"magnitude":       {fn: vec2Magnitude, minArity: 0},
		"dot":             {fn: vec2Dot, minArity: 1, compositeArgAt: []int{0}},
		"normalize":       {fn: vec2Normalize, minArity: 0},
		"normalizeOrZero": {fn: vec2NormalizeOrZero, minArity: 0},
		"angle":           {fn: vec2Angle, minArity: 0},
		"rotate":          {fn: vec2Rotate, minArity: 1},
		"orthogonal":      {fn: vec2Orthogonal, minArity: 0},
		"rotateAbout":     {fn: vec2RotateAbout, minArity: 2, compositeArgAt: []int{0}},
		"angleDiff":       {fn: vec2AngleDiff, minArity: 1, compositeArgAt: []int{0}},
		"signedAngleDiff": {fn: vec2SignedAngleDiff, minArity: 1, compositeArgAt: []int{0}},
	},
	"mat": {
		"scale":     {fn: matScale, minArity: 1},
		"translate": {fn: matTranslate, minArity: 1},
		"compose":   {fn: matCompose, minArity: 1, compositeArgAt: []int{0}},
		"rotate":    {fn: matRotate, minArity: 1},
	},
	"rect": {
		"w":         {fn: rectW, minArity: 0},
		"h":         {fn: rectH, minArity: 0},
		"center":    {fn: rectCenter, minArity: 0},
		"translate": {fn: rectTranslate, minArity: 1, compositeArgAt: []int{0}},
		"scale":     {fn: rectScale, minArity: 1},
	},
	"quad": {
		"center":    {fn: quadCenter, minArity: 0},
		"translate": {fn: quadTranslate, minArity: 1, compositeArgAt: []int{0}},
		"scale":     {fn: quadScale, minArity: 1},
		"permute":   {fn: quadPermute, minArity: 1},
		"rotate":    {fn: quadRotate, minArity: 1},
		"top":       {fn: quadTop, minArity: 0},
		"right":     {fn: quadRight, minArity: 0},
		"bottom":    {fn: quadBottom, minArity: 0},
		"left":      {fn: quadLeft, minArity: 0},
		"contains":  {fn: quadContains, minArity: 1, compositeArgAt: []int{0}},
	},
	"trans": {
		"compose":      {fn: transCompose, minArity: 1, compositeArgAt: []int{0}},
		"translate":    {fn: transTranslate, minArity: 1, compositeArgAt: []int{0}},
		"scale":        {fn: transScale, minArity: 1, compositeArgAt: []int{0}},
		"rotate":       {fn: transRotate, minArity: 1},
		"transformVec": {fn: transTransformVec, minArity: 1, compositeArgAt: []int{0}},
	},
	"judgmentWindow": {
		"judge": {fn: judgmentWindowJudge, minArity: 2},
	},
	"sprite": {
		"draw":   {fn: spriteDraw, minArity: 0},
		"exists": {fn: spriteExists, minArity: 0},
	},
	"skinSprites": {
		"exists": {fn: skinSpritesExists, minArity: 1},
	},
	"canvasObj": {
		"print": {fn: canvasPrint, minArity: 1, compositeArgAt: []int{0}},
	},
	"effect": {
		"play":         {fn: effectPlay, minArity: 1},
		"schedule":     {fn: effectSchedule, minArity: 2},
		"loop":         {fn: effectLoop, minArity: 0},
		"scheduleLoop": {fn: effectScheduleLoop, minArity: 1},
	},
	"effectClip": {
		"play":     {fn: effectPlay, minArity: 1},
		"schedule": {fn: effectSchedule, minArity: 2},
	},
	"particle": {
		"spawn": {fn: particleSpawn, minArity: 0, compositeArgAt: []int{0}},
	},
	"particleClip": {
		"spawn": {fn: particleSpawn, minArity: 0, compositeArgAt: []int{0}},
	},
	"loopedEffectHandle": {
		"stop": {fn: loopedEffectHandleStop, minArity: 0},
	},
	"scheduledLoopedEffectHandle": {
		"stop": {fn: scheduledLoopedEffectHandleStop, minArity: 1},
	},
	"particleHandle": {
		"move":    {fn: particleHandleMove, minArity: 1, compositeArgAt: []int{0}},
		"destroy": {fn: particleHandleDestroy, minArity: 0},
	},
	"entityInfo": {
		"isWaiting":   {fn: entityInfoIsWaiting, minArity: 0},
		"isActive":    {fn: entityInfoIsActive, minArity: 0},
		"isDespawned": {fn: entityInfoIsDespawned, minArity: 0},
	},
	"lifeInfo": {
		"addScheduled": {fn: lifeInfoAddScheduled, minArity: 2},
	},
	"entityRef": {
		"get": {fn: entityRefGet, minArity: 2},
		"set": {fn: entityRefSet, minArity: 3},
	},
	"pair": {
		"lt":    {fn: pairLt, minArity: 1, compositeArgAt: []int{0}},
		"le":    {fn: pairLe, minArity: 1, compositeArgAt: []int{0}},
		"gt":    {fn: pairGt, minArity: 1, compositeArgAt: []int{0}},
		"ge":    {fn: pairGe, minArity: 1, compositeArgAt: []int{0}},
		"tuple": {fn: pairTuple, minArity: 0},
	},
	"varArray": {
		"len":       {fn: varArrayLen, containerFn: varArrayLenCI, minArity: 0},
		"capacity":  {fn: varArrayCapacity, containerFn: varArrayCapacityCI, minArity: 0},
		"isFull":    {fn: varArrayIsFull, containerFn: varArrayIsFullCI, minArity: 0},
		"append":    {fn: varArrayAppend, containerFn: varArrayAppendCI, minArity: 1},
		"pop":       {fn: varArrayPop, containerFn: varArrayPopCI, minArity: 0},
		"clear":     {fn: varArrayClear, containerFn: varArrayClearCI, minArity: 0},
		"contains":  {containerFn: varArrayContainsCI, minArity: 1},
		"index":     {containerFn: varArrayIndexCI, minArity: 1},
		"remove":    {containerFn: varArrayRemoveCI, minArity: 1},
		"insert":    {containerFn: varArrayInsertCI, minArity: 2},
		"sort":      {containerFn: varArraySortCI, minArity: 0},
		"setAdd":    {containerFn: varArraySetAddCI, minArity: 1},
		"setRemove": {containerFn: varArraySetRemoveCI, minArity: 1},
	},
	"arrayMap": {
		"len":      {fn: varArrayLen, containerFn: varArrayLenCI, minArity: 0},
		"capacity": {fn: varArrayCapacity, containerFn: varArrayCapacityCI, minArity: 0},
		"clear":    {fn: varArrayClear, containerFn: varArrayClearCI, minArity: 0},
		"get":      {containerFn: arrayMapGetCI, minArity: 1},
		"set":      {containerFn: arrayMapSetCI, minArity: 2},
		"delete":   {containerFn: arrayMapDeleteCI, minArity: 1},
		"contains": {containerFn: arrayMapContainsCI, minArity: 1},
		"pop":      {containerFn: arrayMapPopCI, minArity: 1},
		"keys":     {fn: containerSelf, minArity: 0},
		"values":   {fn: containerSelf, minArity: 0},
		"items":    {fn: containerSelf, minArity: 0},
	},
	"arraySet": {
		"len":      {fn: varArrayLen, containerFn: varArrayLenCI, minArity: 0},
		"capacity": {fn: varArrayCapacity, containerFn: varArrayCapacityCI, minArity: 0},
		"clear":    {fn: varArrayClear, containerFn: varArrayClearCI, minArity: 0},
		"add":      {containerFn: varArraySetAddCI, minArity: 1},
		"remove":   {containerFn: varArraySetRemoveCI, minArity: 1},
		"contains": {containerFn: varArrayContainsCI, minArity: 1},
	},
	"frozenNumSet": {
		"len":      {fn: varArrayLen, containerFn: varArrayLenCI, minArity: 0},
		"capacity": {fn: varArrayCapacity, containerFn: varArrayCapacityCI, minArity: 0},
		"contains": {containerFn: frozenNumSetContainsCI, minArity: 1},
	},
}

// Sprite, Effect, Particle handles are thin records wrapping a numeric runtime ID.
var spriteFields = []string{"id"}
var effectFields = []string{"id"}
var particleFields = []string{"id"}
var loopedEffectHandleFields = []string{"id"}
var scheduledLoopedEffectHandleFields = []string{"id"}
var particleHandleFields = []string{"id"}

// EntityInfo is a structured record representing an entity's compile-time info.
var entityInfoFields = []string{"index", "archetype", "state"}

// ConsecutiveLife has two fields: increment (base) and step (combo bonus).
var printOptionsFields = []string{
	"value", "format", "decimalPlaces",
	"anchorX", "anchorY", "pivotX", "pivotY",
	"sizeX", "sizeY", "rotation",
	"color", "alpha", "horizontalAlign", "background",
}
var consecutiveLifeFields = []string{"increment", "step"}

// EntityRef wraps an entity index, enabling cross-entity data access.
var entityRefFields = []string{"index"}

// Pair is a generic two-element record.
var pairFields = []string{"first", "second"}

// VarArray is a variable-size array backed by a fixed-capacity buffer.
// It is compiled as a record with two fields: _size (current element count)
// and _array (backing storage). The _array is a multi-slot TempBlock allocated
// at declaration time with capacity slots.
var varArrayFields = []string{"_size", "_array"}

// JudgmentWindow field names.
var judgmentWindowFields = []string{
	"perfectMin", "perfectMax",
	"greatMin", "greatMax",
	"goodMin", "goodMax",
}

// judgmentWindowJudge implements JudgmentWindow.judge(actual, target) → Judgment.
// Calls native judge(actual, target, perfectMin, perfectMax, greatMin, greatMax,
// goodMin, goodMax) which returns 0 (miss), 1 (perfect), 2 (great), or 3 (good).
func judgmentWindowJudge(t *tracer, w Num, args []Num) (Num, error) {
	actual, target := args[0], args[1]
	return exprNum(t.gen.PureInstr(resource.RuntimeFunctionJudge,
		actual.mustNode(),
		target.mustNode(),
		w.MustField("perfectMin").mustNode(),
		w.MustField("perfectMax").mustNode(),
		w.MustField("greatMin").mustNode(),
		w.MustField("greatMax").mustNode(),
		w.MustField("goodMin").mustNode(),
		w.MustField("goodMax").mustNode(),
	)), nil
}

// spriteExists implements Sprite.Exists() → HasSkinSprite(id).
func spriteExists(t *tracer, s Num, args []Num) (Num, error) {
	return exprNum(t.gen.PureInstr(resource.RuntimeFunctionHasSkinSprite,
		s.MustField("id").mustNode())), nil
}

// canvasPrint implements CanvasObj.Print(opts) → Print(15 args).
// In non-preview modes, emits a no-op.
func canvasPrint(t *tracer, c Num, args []Num) (Num, error) {
	if t.env.Mode != ir.ModePreview {
		return constNum(0), nil
	}
	opts := args[0]
	fields := []string{"value", "format", "decimalPlaces",
		"anchorX", "anchorY", "pivotX", "pivotY",
		"sizeX", "sizeY", "rotation",
		"color", "alpha", "horizontalAlign", "background"}
	nodes := make([]ir.Node, len(fields))
	for i, f := range fields {
		nodes[i] = opts.MustField(f).mustNode()
	}
	t.emit(t.gen.ImpureInstr(resource.RuntimeFunctionPrint, nodes...))
	return constNum(0), nil
}

// skinSpritesExists implements SkinSprites.Exists(id) → HasSkinSprite(id).
func skinSpritesExists(t *tracer, ss Num, args []Num) (Num, error) {
	return exprNum(t.gen.PureInstr(resource.RuntimeFunctionHasSkinSprite,
		args[0].mustNode())), nil
}

// spriteDraw implements Sprite.draw(args...) → native Draw(id, args...).
func spriteDraw(t *tracer, s Num, args []Num) (Num, error) {
	nodes := make([]ir.Node, 1+len(args))
	nodes[0] = s.MustField("id").mustNode()
	for i, a := range args {
		nodes[i+1] = a.mustNode()
	}
	return exprNum(t.gen.ImpureInstr(resource.RuntimeFunctionDraw, nodes...)), nil
}

// effectPlay implements Effect.play(distance) → native Play(id, distance).
func effectPlay(t *tracer, e Num, args []Num) (Num, error) {
	return exprNum(t.gen.ImpureInstr(resource.RuntimeFunctionPlay,
		e.MustField("id").mustNode(),
		args[0].mustNode(),
	)), nil
}

// effectSchedule implements Effect.schedule(time, distance) → native PlayScheduled(id, time, distance).
func effectSchedule(t *tracer, e Num, args []Num) (Num, error) {
	return exprNum(t.gen.ImpureInstr(resource.RuntimeFunctionPlayScheduled,
		e.MustField("id").mustNode(),
		args[0].mustNode(),
		args[1].mustNode(),
	)), nil
}

// effectLoop implements Effect.loop() → LoopedEffectHandle.
func effectLoop(t *tracer, e Num, args []Num) (Num, error) {
	id := exprNum(t.gen.ImpureInstr(resource.RuntimeFunctionPlayLooped,
		e.MustField("id").mustNode(),
	))
	return compNumTyped("loopedEffectHandle", map[string]Num{"id": id}), nil
}

// effectScheduleLoop implements Effect.scheduleLoop(startTime) → ScheduledLoopedEffectHandle.
func effectScheduleLoop(t *tracer, e Num, args []Num) (Num, error) {
	id := exprNum(t.gen.ImpureInstr(resource.RuntimeFunctionPlayLoopedScheduled,
		e.MustField("id").mustNode(),
		args[0].mustNode(),
	))
	return compNumTyped("scheduledLoopedEffectHandle", map[string]Num{"id": id}), nil
}

// particleSpawn implements Particle.spawn(args...) → ParticleHandle.
// Composite args (Quad, Vec2, etc.) are automatically destructured into scalar fields.
func particleSpawn(t *tracer, p Num, args []Num) (Num, error) {
	nodes := []ir.Node{p.MustField("id").mustNode()}
	for _, a := range args {
		if a.IsComposite() {
			order, err := a.CompositeFieldOrder()
			if err != nil {
				return Num{}, err
			}
			for _, f := range order {
				nodes = append(nodes, a.MustField(f).mustNode())
			}
		} else {
			nodes = append(nodes, a.mustNode())
		}
	}
	id := exprNum(t.gen.ImpureInstr(resource.RuntimeFunctionSpawnParticleEffect, nodes...))
	return compNumTyped("particleHandle", map[string]Num{"id": id}), nil
}

// loopedEffectHandleStop implements LoopedEffectHandle.stop() → native StopLooped(id).
func loopedEffectHandleStop(t *tracer, h Num, args []Num) (Num, error) {
	t.emit(t.gen.ImpureInstr(resource.RuntimeFunctionStopLooped,
		h.MustField("id").mustNode(),
	))
	return constNum(0), nil
}

// scheduledLoopedEffectHandleStop implements ScheduledLoopedEffectHandle.stop(endTime) → native StopLoopedScheduled(id, endTime).
func scheduledLoopedEffectHandleStop(t *tracer, h Num, args []Num) (Num, error) {
	t.emit(t.gen.ImpureInstr(resource.RuntimeFunctionStopLoopedScheduled,
		h.MustField("id").mustNode(),
		args[0].mustNode(),
	))
	return constNum(0), nil
}

// particleHandleMove implements ParticleHandle.move(quad) → native MoveParticleEffect(id, quad...).
func particleHandleMove(t *tracer, h Num, args []Num) (Num, error) {
	quad := args[0]
	nodes := []ir.Node{h.MustField("id").mustNode()}
	for _, f := range quadFields {
		nodes = append(nodes, quad.MustField(f).mustNode())
	}
	t.emit(t.gen.ImpureInstr(resource.RuntimeFunctionMoveParticleEffect, nodes...))
	return constNum(0), nil
}

// particleHandleDestroy implements ParticleHandle.destroy() → native DestroyParticleEffect(id).
func particleHandleDestroy(t *tracer, h Num, args []Num) (Num, error) {
	t.emit(t.gen.ImpureInstr(resource.RuntimeFunctionDestroyParticleEffect,
		h.MustField("id").mustNode(),
	))
	return constNum(0), nil
}

// entityRefGet implements EntityRef.get(block, index) → Get(block, ref.index + index).
func entityRefGet(t *tracer, r Num, args []Num) (Num, error) {
	idx := t.gen.PureInstr(resource.RuntimeFunctionAdd,
		r.MustField("index").mustNode(), args[1].mustNode())
	return exprNum(ir.GetPlace(ir.NewBlockPlace(args[0].mustNode(), idx, 0))), nil
}

// entityRefSet implements EntityRef.set(block, index, value) → Set(block, ref.index + index, value).
func entityRefSet(t *tracer, r Num, args []Num) (Num, error) {
	idx := t.gen.PureInstr(resource.RuntimeFunctionAdd,
		r.MustField("index").mustNode(), args[1].mustNode())
	t.emit(t.gen.SetPlace(ir.NewBlockPlace(args[0].mustNode(), idx, 0), args[2].mustNode()))
	return constNum(0), nil
}

// entityInfoIsWaiting implements EntityInfo.IsWaiting() → state == 0.
func entityInfoIsWaiting(t *tracer, info Num, args []Num) (Num, error) {
	eq, ok := applyBinary(t.gen, token.EQL, info.MustField("state"), constNum(0))
	if !ok {
		return Num{}, fmt.Errorf("isWaiting: comparison not supported")
	}
	return eq, nil
}

// entityInfoIsActive implements EntityInfo.IsActive() → state == 1.
func entityInfoIsActive(t *tracer, info Num, args []Num) (Num, error) {
	eq, ok := applyBinary(t.gen, token.EQL, info.MustField("state"), constNum(1))
	if !ok {
		return Num{}, fmt.Errorf("isActive: comparison not supported")
	}
	return eq, nil
}


// lifeInfoAddScheduled implements LifeInfo.AddScheduled(value, time).
func lifeInfoAddScheduled(t *tracer, info Num, args []Num) (Num, error) {
	t.emit(t.gen.ImpureInstr(resource.RuntimeFunctionAddLifeScheduled,
		args[0].mustNode(),
		args[1].mustNode(),
	))
	return constNum(0), nil
}

// entityInfoIsDespawned implements EntityInfo.IsDespawned() → state == 2.
func entityInfoIsDespawned(t *tracer, info Num, args []Num) (Num, error) {
	eq, ok := applyBinary(t.gen, token.EQL, info.MustField("state"), constNum(2))
	if !ok {
		return Num{}, fmt.Errorf("isDespawned: comparison not supported")
	}
	return eq, nil
}

// recordStatics maps record type name → static-constructor name → implementation.
var recordStatics = map[string]map[string]func() (Num, error){
	"vec2": {
		"zero":  func() (Num, error) { return vec2Zero(), nil },
		"one":   func() (Num, error) { return vec2One(), nil },
		"up":    func() (Num, error) { return vec2Up(), nil },
		"down":  func() (Num, error) { return vec2Down(), nil },
		"left":  func() (Num, error) { return vec2Left(), nil },
		"right": func() (Num, error) { return vec2Right(), nil },
	},
}
