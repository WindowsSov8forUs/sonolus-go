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
	{"consecutiveLife", consecutiveLifeFields},
	{"consecutiveScore", consecutiveScoreFields},
	{"scoreBase", scoreBaseFields},
	{"transform2d", transform2dFields},
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
	"transform2d": {
		"translate":         {fn: transform2dTranslate, minArity: 1, compositeArgAt: []int{0}},
		"scale":             {fn: transform2dScale, minArity: 1, compositeArgAt: []int{0}},
		"scaleAbout":        {fn: transform2dScaleAbout, minArity: 2, compositeArgAt: []int{0, 1}},
		"rotate":            {fn: transform2dRotate, minArity: 1},
		"rotateAbout":       {fn: transform2dRotateAbout, minArity: 2, compositeArgAt: []int{1}},
		"compose":           {fn: transform2dCompose, minArity: 1, compositeArgAt: []int{0}},
		"composeBefore":     {fn: transform2dComposeBefore, minArity: 1, compositeArgAt: []int{0}},
		"transformVec":      {fn: transform2dTransformVec, minArity: 1, compositeArgAt: []int{0}},
		"transformQuad":     {fn: transform2dTransformQuad, minArity: 1, compositeArgAt: []int{0}},
		"simplePerspectiveX": {fn: transform2dSimplePerspectiveX, minArity: 1},
		"simplePerspectiveY": {fn: transform2dSimplePerspectiveY, minArity: 1},
		"perspectiveX":      {fn: transform2dPerspectiveX, minArity: 2, compositeArgAt: []int{1}},
		"perspectiveY":      {fn: transform2dPerspectiveY, minArity: 2, compositeArgAt: []int{1}},
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
var transform2dFields = []string{
	"a00", "a01", "a02", "a03",
	"a10", "a11", "a12", "a13",
	"a20", "a21", "a22", "a23",
	"a30", "a31", "a32", "a33",
}
var consecutiveScoreFields = []string{"multiplier", "step", "cap"}
var scoreBaseFields = []string{"perfect", "great", "good"}
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

// --- Transform2d methods ---

func transform2dTranslate(t *tracer, m Num, args []Num) (Num, error) {
	v := args[0]; dx, dy := v.MustField("x").mustNode(), v.MustField("y").mustNode()
	add := func(n ir.Node) ir.Node { return t.gen.PureInstr(resource.RuntimeFunctionAdd, n, dx) }
	addY := func(n ir.Node) ir.Node { return t.gen.PureInstr(resource.RuntimeFunctionAdd, n, dy) }
	return compNumTyped("transform2d", map[string]Num{
		"a00": m.MustField("a00"), "a01": m.MustField("a01"), "a02": exprNum(add(m.MustField("a02").mustNode())), "a03": m.MustField("a03"),
		"a10": m.MustField("a10"), "a11": m.MustField("a11"), "a12": exprNum(addY(m.MustField("a12").mustNode())), "a13": m.MustField("a13"),
		"a20": m.MustField("a20"), "a21": m.MustField("a21"), "a22": m.MustField("a22"), "a23": m.MustField("a23"),
		"a30": m.MustField("a30"), "a31": m.MustField("a31"), "a32": m.MustField("a32"), "a33": m.MustField("a33"),
	}), nil
}

func transform2dScale(t *tracer, m Num, args []Num) (Num, error) {
	sx, sy := args[0].MustField("x").mustNode(), args[0].MustField("y").mustNode()
	mulX := func(n ir.Node) ir.Node { return t.gen.PureInstr(resource.RuntimeFunctionMultiply, n, sx) }
	mulY := func(n ir.Node) ir.Node { return t.gen.PureInstr(resource.RuntimeFunctionMultiply, n, sy) }
	return compNumTyped("transform2d", map[string]Num{
		"a00": exprNum(mulX(m.MustField("a00").mustNode())), "a01": m.MustField("a01"), "a02": m.MustField("a02"), "a03": m.MustField("a03"),
		"a10": m.MustField("a10"), "a11": exprNum(mulY(m.MustField("a11").mustNode())), "a12": m.MustField("a12"), "a13": m.MustField("a13"),
		"a20": m.MustField("a20"), "a21": m.MustField("a21"), "a22": m.MustField("a22"), "a23": m.MustField("a23"),
		"a30": m.MustField("a30"), "a31": m.MustField("a31"), "a32": m.MustField("a32"), "a33": m.MustField("a33"),
	}), nil
}

func transform2dRotate(t *tracer, m Num, args []Num) (Num, error) {
	a := args[0]
	c := exprNum(t.gen.PureInstr(resource.RuntimeFunctionCos, a.mustNode()))
	s := exprNum(t.gen.PureInstr(resource.RuntimeFunctionSin, a.mustNode()))
	// rotate: a00, a01, a10, a11
	rotX := func(x, y ir.Node) ir.Node { return t.gen.PureInstr(resource.RuntimeFunctionSubtract, x, y) }
	rotY := func(x, y ir.Node) ir.Node { return t.gen.PureInstr(resource.RuntimeFunctionAdd, x, y) }
	a00c := exprNum(t.gen.PureInstr(resource.RuntimeFunctionMultiply, m.MustField("a00").mustNode(), c.mustNode()))
	a01s := exprNum(t.gen.PureInstr(resource.RuntimeFunctionMultiply, m.MustField("a01").mustNode(), s.mustNode()))
	a00s := exprNum(t.gen.PureInstr(resource.RuntimeFunctionMultiply, m.MustField("a00").mustNode(), s.mustNode()))
	a01c := exprNum(t.gen.PureInstr(resource.RuntimeFunctionMultiply, m.MustField("a01").mustNode(), c.mustNode()))
	a10c := exprNum(t.gen.PureInstr(resource.RuntimeFunctionMultiply, m.MustField("a10").mustNode(), c.mustNode()))
	a11s := exprNum(t.gen.PureInstr(resource.RuntimeFunctionMultiply, m.MustField("a11").mustNode(), s.mustNode()))
	a10s := exprNum(t.gen.PureInstr(resource.RuntimeFunctionMultiply, m.MustField("a10").mustNode(), s.mustNode()))
	a11c := exprNum(t.gen.PureInstr(resource.RuntimeFunctionMultiply, m.MustField("a11").mustNode(), c.mustNode()))
	return compNumTyped("transform2d", map[string]Num{
		"a00": exprNum(rotX(a00c.mustNode(), a01s.mustNode())), "a01": exprNum(rotY(a00s.mustNode(), a01c.mustNode())),
		"a02": m.MustField("a02"), "a03": m.MustField("a03"),
		"a10": exprNum(rotX(a10c.mustNode(), a11s.mustNode())), "a11": exprNum(rotY(a10s.mustNode(), a11c.mustNode())),
		"a12": m.MustField("a12"), "a13": m.MustField("a13"),
		"a20": m.MustField("a20"), "a21": m.MustField("a21"), "a22": m.MustField("a22"), "a23": m.MustField("a23"),
		"a30": m.MustField("a30"), "a31": m.MustField("a31"), "a32": m.MustField("a32"), "a33": m.MustField("a33"),
	}), nil
}

func transform2dCompose(t *tracer, m Num, args []Num) (Num, error) {
	o := args[0]
	// compose: each element = sum of products from m row × o column
	dot4 := func(row []Num, col string) Num {
		mul := func(a, b Num) ir.Node { return t.gen.PureInstr(resource.RuntimeFunctionMultiply, a.mustNode(), b.mustNode()) }
		sum01 := t.gen.PureInstr(resource.RuntimeFunctionAdd, mul(row[0], o.MustField(col+"0")), mul(row[1], o.MustField(col+"1")))
		sum23 := t.gen.PureInstr(resource.RuntimeFunctionAdd, mul(row[2], o.MustField(col+"2")), mul(row[3], o.MustField(col+"3")))
		return exprNum(t.gen.PureInstr(resource.RuntimeFunctionAdd, sum01, sum23))
	}
	rows := [][]Num{
		{m.MustField("a00"), m.MustField("a01"), m.MustField("a02"), m.MustField("a03")},
		{m.MustField("a10"), m.MustField("a11"), m.MustField("a12"), m.MustField("a13")},
		{m.MustField("a20"), m.MustField("a21"), m.MustField("a22"), m.MustField("a23")},
		{m.MustField("a30"), m.MustField("a31"), m.MustField("a32"), m.MustField("a33")},
	}
	return compNumTyped("transform2d", map[string]Num{
		"a00": dot4(rows[0], "a0"), "a01": dot4(rows[0], "a1"), "a02": dot4(rows[0], "a2"), "a03": dot4(rows[0], "a3"),
		"a10": dot4(rows[1], "a0"), "a11": dot4(rows[1], "a1"), "a12": dot4(rows[1], "a2"), "a13": dot4(rows[1], "a3"),
		"a20": dot4(rows[2], "a0"), "a21": dot4(rows[2], "a1"), "a22": dot4(rows[2], "a2"), "a23": dot4(rows[2], "a3"),
		"a30": dot4(rows[3], "a0"), "a31": dot4(rows[3], "a1"), "a32": dot4(rows[3], "a2"), "a33": dot4(rows[3], "a3"),
	}), nil
}

func transform2dTransformVec(t *tracer, m Num, args []Num) (Num, error) {
	v := args[0]; x, y := v.MustField("x").mustNode(), v.MustField("y").mustNode()
	// x' = a00*x + a01*y + a02, y' = a10*x + a11*y + a12
	mul := func(a, b ir.Node) ir.Node { return t.gen.PureInstr(resource.RuntimeFunctionMultiply, a, b) }
	nx := t.gen.PureInstr(resource.RuntimeFunctionAdd,
		t.gen.PureInstr(resource.RuntimeFunctionAdd, mul(m.MustField("a00").mustNode(), x), mul(m.MustField("a01").mustNode(), y)),
		m.MustField("a02").mustNode())
	ny := t.gen.PureInstr(resource.RuntimeFunctionAdd,
		t.gen.PureInstr(resource.RuntimeFunctionAdd, mul(m.MustField("a10").mustNode(), x), mul(m.MustField("a11").mustNode(), y)),
		m.MustField("a12").mustNode())
	return compNumTyped("vec2", map[string]Num{"x": exprNum(nx), "y": exprNum(ny)}), nil
}

// --- Transform2d remaining methods ---

func transform2dScaleAbout(t *tracer, m Num, args []Num) (Num, error) {
	factor := args[0]; pivot := args[1]
	fx, fy := factor.MustField("x").mustNode(), factor.MustField("y").mustNode()
	px, py := pivot.MustField("x").mustNode(), pivot.MustField("y").mustNode()
	mulX := func(n ir.Node) ir.Node { return t.gen.PureInstr(resource.RuntimeFunctionAdd, t.gen.PureInstr(resource.RuntimeFunctionMultiply, t.gen.PureInstr(resource.RuntimeFunctionSubtract, n, px), fx), px) }
	mulY := func(n ir.Node) ir.Node { return t.gen.PureInstr(resource.RuntimeFunctionAdd, t.gen.PureInstr(resource.RuntimeFunctionMultiply, t.gen.PureInstr(resource.RuntimeFunctionSubtract, n, py), fy), py) }
	return compNumTyped("transform2d", map[string]Num{
		"a00": exprNum(mulX(m.MustField("a00").mustNode())), "a01": exprNum(mulX(m.MustField("a01").mustNode())),
		"a02": exprNum(mulX(m.MustField("a02").mustNode())), "a03": m.MustField("a03"),
		"a10": exprNum(mulY(m.MustField("a10").mustNode())), "a11": exprNum(mulY(m.MustField("a11").mustNode())),
		"a12": exprNum(mulY(m.MustField("a12").mustNode())), "a13": m.MustField("a13"),
		"a20": m.MustField("a20"), "a21": m.MustField("a21"), "a22": m.MustField("a22"), "a23": m.MustField("a23"),
		"a30": m.MustField("a30"), "a31": m.MustField("a31"), "a32": m.MustField("a32"), "a33": m.MustField("a33"),
	}), nil
}

func transform2dRotateAbout(t *tracer, m Num, args []Num) (Num, error) {
	angle := args[0]; pivot := args[1]
	px, py := pivot.MustField("x").mustNode(), pivot.MustField("y").mustNode()
	c := exprNum(t.gen.PureInstr(resource.RuntimeFunctionCos, angle.mustNode()))
	s := exprNum(t.gen.PureInstr(resource.RuntimeFunctionSin, angle.mustNode()))
	// Rotate matrix + adjust translation: a02' = a02 + a00*(-px) + a01*(-py) + px (rotated)
	mul := func(a, b ir.Node) ir.Node { return t.gen.PureInstr(resource.RuntimeFunctionMultiply, a, b) }
	add := func(a, b ir.Node) ir.Node { return t.gen.PureInstr(resource.RuntimeFunctionAdd, a, b) }
	sub := func(a, b ir.Node) ir.Node { return t.gen.PureInstr(resource.RuntimeFunctionSubtract, a, b) }
	a00c := exprNum(mul(m.MustField("a00").mustNode(), c.mustNode()))
	a01s := exprNum(mul(m.MustField("a01").mustNode(), s.mustNode()))
	a00s := exprNum(mul(m.MustField("a00").mustNode(), s.mustNode()))
	a01c := exprNum(mul(m.MustField("a01").mustNode(), c.mustNode()))
	a10c := exprNum(mul(m.MustField("a10").mustNode(), c.mustNode()))
	a11s := exprNum(mul(m.MustField("a11").mustNode(), s.mustNode()))
	a10s := exprNum(mul(m.MustField("a10").mustNode(), s.mustNode()))
	a11c := exprNum(mul(m.MustField("a11").mustNode(), c.mustNode()))
	// a02' = a02 + a00*px + a01*py - (a00c - a01s)*px - (a00s + a01c)*py
	// Simplified: a02' = a00*(px - c*px - s*py) + a01*(py + s*px - c*py) + a02
	newA00 := exprNum(sub(a00c.mustNode(), a01s.mustNode()))
	newA01 := exprNum(add(a00s.mustNode(), a01c.mustNode()))
	newA10 := exprNum(sub(a10c.mustNode(), a11s.mustNode()))
	newA11 := exprNum(add(a10s.mustNode(), a11c.mustNode()))
	a02p := exprNum(add(add(mul(m.MustField("a00").mustNode(), px), mul(m.MustField("a01").mustNode(), py)), m.MustField("a02").mustNode()))
	a02r := exprNum(add(add(mul(newA00.mustNode(), sub(ir.Const(0), px)), mul(newA01.mustNode(), sub(ir.Const(0), py))), a02p.mustNode()))
	a12p := exprNum(add(add(mul(m.MustField("a10").mustNode(), px), mul(m.MustField("a11").mustNode(), py)), m.MustField("a12").mustNode()))
	a12r := exprNum(add(add(mul(newA10.mustNode(), sub(ir.Const(0), px)), mul(newA11.mustNode(), sub(ir.Const(0), py))), a12p.mustNode()))
	return compNumTyped("transform2d", map[string]Num{
		"a00": newA00, "a01": newA01, "a02": a02r, "a03": m.MustField("a03"),
		"a10": newA10, "a11": newA11, "a12": a12r, "a13": m.MustField("a13"),
		"a20": m.MustField("a20"), "a21": m.MustField("a21"), "a22": m.MustField("a22"), "a23": m.MustField("a23"),
		"a30": m.MustField("a30"), "a31": m.MustField("a31"), "a32": m.MustField("a32"), "a33": m.MustField("a33"),
	}), nil
}

func transform2dComposeBefore(t *tracer, m Num, args []Num) (Num, error) {
	// composeBefore(other) = other.Compose(this)
	o := args[0]
	dot4 := func(row []Num, col string) Num {
		mul := func(a, b Num) ir.Node { return t.gen.PureInstr(resource.RuntimeFunctionMultiply, a.mustNode(), b.mustNode()) }
		sum01 := t.gen.PureInstr(resource.RuntimeFunctionAdd, mul(row[0], m.MustField("a0"+string(col[1]))), mul(row[1], m.MustField("a1"+string(col[1]))))
		sum23 := t.gen.PureInstr(resource.RuntimeFunctionAdd, mul(row[2], m.MustField("a2"+string(col[1]))), mul(row[3], m.MustField("a3"+string(col[1]))))
		return exprNum(t.gen.PureInstr(resource.RuntimeFunctionAdd, sum01, sum23))
	}
	rows := [][]Num{
		{o.MustField("a00"), o.MustField("a01"), o.MustField("a02"), o.MustField("a03")},
		{o.MustField("a10"), o.MustField("a11"), o.MustField("a12"), o.MustField("a13")},
		{o.MustField("a20"), o.MustField("a21"), o.MustField("a22"), o.MustField("a23")},
		{o.MustField("a30"), o.MustField("a31"), o.MustField("a32"), o.MustField("a33")},
	}
	return compNumTyped("transform2d", map[string]Num{
		"a00": dot4(rows[0], "a0"), "a01": dot4(rows[0], "a1"), "a02": dot4(rows[0], "a2"), "a03": dot4(rows[0], "a3"),
		"a10": dot4(rows[1], "a0"), "a11": dot4(rows[1], "a1"), "a12": dot4(rows[1], "a2"), "a13": dot4(rows[1], "a3"),
		"a20": dot4(rows[2], "a0"), "a21": dot4(rows[2], "a1"), "a22": dot4(rows[2], "a2"), "a23": dot4(rows[2], "a3"),
		"a30": dot4(rows[3], "a0"), "a31": dot4(rows[3], "a1"), "a32": dot4(rows[3], "a2"), "a33": dot4(rows[3], "a3"),
	}), nil
}

func transform2dSimplePerspectiveX(t *tracer, m Num, args []Num) (Num, error) {
	// 1 / (1 - x * a20) where x is vanishing point
	x := args[0].mustNode()
	f := t.gen.PureInstr(resource.RuntimeFunctionDivide, ir.Const(1),
		t.gen.PureInstr(resource.RuntimeFunctionSubtract, ir.Const(1),
			t.gen.PureInstr(resource.RuntimeFunctionMultiply, x, m.MustField("a20").mustNode())))
	return compNumTyped("transform2d", map[string]Num{
		"a00": exprNum(t.gen.PureInstr(resource.RuntimeFunctionMultiply, m.MustField("a00").mustNode(), f)), "a01": exprNum(t.gen.PureInstr(resource.RuntimeFunctionMultiply, m.MustField("a01").mustNode(), f)),
		"a02": exprNum(t.gen.PureInstr(resource.RuntimeFunctionMultiply, t.gen.PureInstr(resource.RuntimeFunctionAdd, t.gen.PureInstr(resource.RuntimeFunctionMultiply, m.MustField("a02").mustNode(), f), m.MustField("a00").mustNode()), t.gen.PureInstr(resource.RuntimeFunctionMultiply, x, f))),
		"a03": m.MustField("a03"),
		"a10": m.MustField("a10"), "a11": m.MustField("a11"), "a12": m.MustField("a12"), "a13": m.MustField("a13"),
		"a20": m.MustField("a20"), "a21": m.MustField("a21"), "a22": m.MustField("a22"), "a23": m.MustField("a23"),
		"a30": m.MustField("a30"), "a31": m.MustField("a31"), "a32": m.MustField("a32"), "a33": m.MustField("a33"),
	}), nil
}

func transform2dSimplePerspectiveY(t *tracer, m Num, args []Num) (Num, error) {
	y := args[0].mustNode()
	f := t.gen.PureInstr(resource.RuntimeFunctionDivide, ir.Const(1),
		t.gen.PureInstr(resource.RuntimeFunctionSubtract, ir.Const(1),
			t.gen.PureInstr(resource.RuntimeFunctionMultiply, y, m.MustField("a21").mustNode())))
	return compNumTyped("transform2d", map[string]Num{
		"a00": m.MustField("a00"), "a01": m.MustField("a01"), "a02": m.MustField("a02"), "a03": m.MustField("a03"),
		"a10": exprNum(t.gen.PureInstr(resource.RuntimeFunctionMultiply, m.MustField("a10").mustNode(), f)), "a11": exprNum(t.gen.PureInstr(resource.RuntimeFunctionMultiply, m.MustField("a11").mustNode(), f)),
		"a12": exprNum(t.gen.PureInstr(resource.RuntimeFunctionMultiply, t.gen.PureInstr(resource.RuntimeFunctionAdd, t.gen.PureInstr(resource.RuntimeFunctionMultiply, m.MustField("a12").mustNode(), f), m.MustField("a10").mustNode()), t.gen.PureInstr(resource.RuntimeFunctionMultiply, y, f))),
		"a13": m.MustField("a13"),
		"a20": m.MustField("a20"), "a21": m.MustField("a21"), "a22": m.MustField("a22"), "a23": m.MustField("a23"),
		"a30": m.MustField("a30"), "a31": m.MustField("a31"), "a32": m.MustField("a32"), "a33": m.MustField("a33"),
	}), nil
}

func transform2dPerspectiveX(t *tracer, m Num, args []Num) (Num, error) {
	fg, vp := args[0].mustNode(), args[1]
	vpx := vp.MustField("x").mustNode()
	// perspectiveApproach(foregroundX - vpx, a20) applied to a00,a01,a02
	x := t.gen.PureInstr(resource.RuntimeFunctionSubtract, fg, vpx)
	p := t.gen.PureInstr(resource.RuntimeFunctionDivide, ir.Const(1),
		t.gen.PureInstr(resource.RuntimeFunctionSubtract, ir.Const(1),
			t.gen.PureInstr(resource.RuntimeFunctionMultiply, x, m.MustField("a20").mustNode())))
	return compNumTyped("transform2d", map[string]Num{
		"a00": exprNum(t.gen.PureInstr(resource.RuntimeFunctionMultiply, m.MustField("a00").mustNode(), p)), "a01": exprNum(t.gen.PureInstr(resource.RuntimeFunctionMultiply, m.MustField("a01").mustNode(), p)),
		"a02": exprNum(t.gen.PureInstr(resource.RuntimeFunctionMultiply, t.gen.PureInstr(resource.RuntimeFunctionAdd, t.gen.PureInstr(resource.RuntimeFunctionMultiply, m.MustField("a02").mustNode(), p), m.MustField("a00").mustNode()), t.gen.PureInstr(resource.RuntimeFunctionMultiply, x, p))),
		"a03": m.MustField("a03"),
		"a10": m.MustField("a10"), "a11": m.MustField("a11"), "a12": m.MustField("a12"), "a13": m.MustField("a13"),
		"a20": m.MustField("a20"), "a21": m.MustField("a21"), "a22": m.MustField("a22"), "a23": m.MustField("a23"),
		"a30": m.MustField("a30"), "a31": m.MustField("a31"), "a32": m.MustField("a32"), "a33": m.MustField("a33"),
	}), nil
}

func transform2dPerspectiveY(t *tracer, m Num, args []Num) (Num, error) {
	fg, vp := args[0].mustNode(), args[1]
	vpy := vp.MustField("y").mustNode()
	y := t.gen.PureInstr(resource.RuntimeFunctionSubtract, fg, vpy)
	p := t.gen.PureInstr(resource.RuntimeFunctionDivide, ir.Const(1),
		t.gen.PureInstr(resource.RuntimeFunctionSubtract, ir.Const(1),
			t.gen.PureInstr(resource.RuntimeFunctionMultiply, y, m.MustField("a21").mustNode())))
	return compNumTyped("transform2d", map[string]Num{
		"a00": m.MustField("a00"), "a01": m.MustField("a01"), "a02": m.MustField("a02"), "a03": m.MustField("a03"),
		"a10": exprNum(t.gen.PureInstr(resource.RuntimeFunctionMultiply, m.MustField("a10").mustNode(), p)), "a11": exprNum(t.gen.PureInstr(resource.RuntimeFunctionMultiply, m.MustField("a11").mustNode(), p)),
		"a12": exprNum(t.gen.PureInstr(resource.RuntimeFunctionMultiply, t.gen.PureInstr(resource.RuntimeFunctionAdd, t.gen.PureInstr(resource.RuntimeFunctionMultiply, m.MustField("a12").mustNode(), p), m.MustField("a10").mustNode()), t.gen.PureInstr(resource.RuntimeFunctionMultiply, y, p))),
		"a13": m.MustField("a13"),
		"a20": m.MustField("a20"), "a21": m.MustField("a21"), "a22": m.MustField("a22"), "a23": m.MustField("a23"),
		"a30": m.MustField("a30"), "a31": m.MustField("a31"), "a32": m.MustField("a32"), "a33": m.MustField("a33"),
	}), nil
}

func transform2dTransformQuad(t *tracer, m Num, args []Num) (Num, error) {
	quad := args[0]
	tf := func(fx, fy string) (Num, Num) {
		mul := func(a, b ir.Node) ir.Node { return t.gen.PureInstr(resource.RuntimeFunctionMultiply, a, b) }
		nx := t.gen.PureInstr(resource.RuntimeFunctionAdd,
			t.gen.PureInstr(resource.RuntimeFunctionAdd, mul(m.MustField("a00").mustNode(), quad.MustField(fx).mustNode()), mul(m.MustField("a01").mustNode(), quad.MustField(fy).mustNode())),
			m.MustField("a02").mustNode())
		ny := t.gen.PureInstr(resource.RuntimeFunctionAdd,
			t.gen.PureInstr(resource.RuntimeFunctionAdd, mul(m.MustField("a10").mustNode(), quad.MustField(fx).mustNode()), mul(m.MustField("a11").mustNode(), quad.MustField(fy).mustNode())),
			m.MustField("a12").mustNode())
		return exprNum(nx), exprNum(ny)
	}
	blx, bly := tf("blx", "bly")
	tlx, tly := tf("tlx", "tly")
	trx, try := tf("trx", "try")
	brx, bry := tf("brx", "bry")
	return compNumTyped("quad", map[string]Num{"blx": blx, "bly": bly, "tlx": tlx, "tly": tly, "trx": trx, "try": try, "brx": brx, "bry": bry}), nil
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
