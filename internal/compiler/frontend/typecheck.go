package frontend

import (
	"errors"
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"strings"
)

// PreludeSource returns the prelude source text for a given package name,
// including user-defined record types registered in records. It is combined
// with user source before type checking so go/types can resolve every identifier.
func PreludeSource(pkg string, records map[string][]string) string {
	var b strings.Builder
	b.WriteString("package " + pkg + "\n\n")

	// ── Record types ──
	b.WriteString("type Vec2 struct { x, y float64 }\n")
	b.WriteString("type Quad struct { blx, bly, tlx, tly, trx, try, brx, bry float64 }\n")
	b.WriteString("type Mat struct { m11, m12, m13, m21, m22, m23 float64 }\n")
	b.WriteString("type Rect struct { t, r, b, l float64 }\n")
	b.WriteString("type Trans struct { m11, m12, m13, m21, m22, m23, m31, m32, m33 float64 }\n")
	b.WriteString("type Pair struct { first, second float64 }\n")
	b.WriteString("type VarArray struct { _size float64; _array []float64 }\n")
	b.WriteString("type ArrayMap struct { _size float64; _array []float64 }\n")
	b.WriteString("type ArraySet struct { _values VarArray }\n")
	b.WriteString("type Box struct { val float64 }\n")
	b.WriteString("type FrozenNumSet struct { _size float64; _array []float64 }\n")
	b.WriteString("type JudgmentWindow struct { perfectMin, perfectMax, greatMin, greatMax, goodMin, goodMax float64 }\n")
	b.WriteString("type PlayEntityInput struct { judgment, accuracy, bucketIndex, bucketValue, haptic float64 }\n")
	b.WriteString("type EntityScore struct { perfect, great, good, miss float64 }\n")
	b.WriteString("type EntityLife struct { perfect, great, good, miss float64 }\n")
	b.WriteString("type Sprite struct { id float64 }\n")
	b.WriteString("type SkinSprites struct{}\n")
	b.WriteString("type SkinInfo struct { sprites SkinSprites }\n")
	b.WriteString("type CanvasObj struct{}\n")
	b.WriteString("type PrintOptions struct { value, format, decimalPlaces, anchorX, anchorY, pivotX, pivotY, sizeX, sizeY, rotation, color, alpha, horizontalAlign, background float64 }\n")
	b.WriteString("type Transform2d struct { a00, a01, a02, a03, a10, a11, a12, a13, a20, a21, a22, a23, a30, a31, a32, a33 float64 }\n")
	b.WriteString("type ConsecutiveScore struct { multiplier, step, cap float64 }\n")
	b.WriteString("type ScoreBase struct { perfect, great, good float64 }\n")
	b.WriteString("type RuntimeUiConfig struct { scale, alpha float64 }\n")

	// Handle types — thin records wrapping runtime resource IDs.
	b.WriteString("type LoopedEffectHandle struct { id float64 }\n")
	b.WriteString("type ScheduledLoopedEffectHandle struct { id float64 }\n")
	b.WriteString("type ParticleHandle struct { id float64 }\n")

	// LoopedEffectHandle methods
	b.WriteString("func (h LoopedEffectHandle) stop() {}\n")
	// ScheduledLoopedEffectHandle methods
	b.WriteString("func (h ScheduledLoopedEffectHandle) stop(endTime float64) {}\n")
	// ParticleHandle methods
	b.WriteString("func (h ParticleHandle) move(quad Quad) {}\n")
	b.WriteString("func (h ParticleHandle) destroy() {}\n")
	// EntityRef methods
	b.WriteString("type EntityRef struct { index float64 }\n")
	b.WriteString("func (r EntityRef) get(block, index float64) float64 { return 0 }\n")
	b.WriteString("func (r EntityRef) set(block, index, value float64) {}\n")
	b.WriteString("func (w JudgmentWindow) judge(actual, target float64) float64 { return 0 }\n")

	// User-defined records — type declarations are already in the user source.
	// The prelude generates constructor stubs with lowercase names to avoid
	// colliding with the uppercase type name in the user source.
	// records maps constructor name (lowerCamelCase) → field names.
	for ctor, fields := range records {
		args := make([]string, len(fields))
		for i, f := range fields {
			args[i] = f + " float64"
		}
		// Derive type name from constructor: myRec → MyRec, p → P.
		typeName := strings.ToUpper(ctor[:1]) + ctor[1:]
		b.WriteString("func " + ctor + "(" + strings.Join(args, ", ") + ") " + typeName + " { return " + typeName + "{} }\n")
	}

	// ── Record methods ──
	// Vec2 methods
	b.WriteString("func (v Vec2) add(o Vec2) Vec2 { return v }\n")
	b.WriteString("func (v Vec2) sub(o Vec2) Vec2 { return v }\n")
	b.WriteString("func (v Vec2) mul(s float64) Vec2 { return v }\n")
	b.WriteString("func (v Vec2) div(s float64) Vec2 { return v }\n")
	b.WriteString("func (v Vec2) magnitude() float64 { return 0 }\n")
	b.WriteString("func (v Vec2) dot(o Vec2) float64 { return 0 }\n")
	b.WriteString("func (v Vec2) normalize() Vec2 { return v }\n")
	b.WriteString("func (v Vec2) normalizeOrZero() Vec2 { return v }\n")
	b.WriteString("func (v Vec2) angle() float64 { return 0 }\n")
	b.WriteString("func (v Vec2) rotate(angle float64) Vec2 { return v }\n")
	b.WriteString("func (v Vec2) orthogonal() Vec2 { return v }\n")
	b.WriteString("func (v Vec2) rotateAbout(pt Vec2, angle float64) Vec2 { return v }\n")
	b.WriteString("func (v Vec2) angleDiff(o Vec2) float64 { return 0 }\n")
	b.WriteString("func (v Vec2) signedAngleDiff(o Vec2) float64 { return 0 }\n")

	// Quad methods
	b.WriteString("func (q Quad) center() Vec2 { return Vec2{} }\n")
	b.WriteString("func (q Quad) translate(v Vec2) Quad { return q }\n")
	b.WriteString("func (q Quad) scale(s float64) Quad { return q }\n")
	b.WriteString("func (q Quad) permute(rotation float64) Quad { return q }\n")
	b.WriteString("func (q Quad) rotate(angle float64) Quad { return q }\n")
	b.WriteString("func (q Quad) top() Vec2 { return Vec2{} }\n")
	b.WriteString("func (q Quad) right() Vec2 { return Vec2{} }\n")
	b.WriteString("func (q Quad) bottom() Vec2 { return Vec2{} }\n")
	b.WriteString("func (q Quad) left() Vec2 { return Vec2{} }\n")
	b.WriteString("func (q Quad) contains(p Vec2) float64 { return 0 }\n")

	// Mat methods
	b.WriteString("func (m Mat) scale(sx float64, sy ...float64) Mat { return m }\n")
	b.WriteString("func (m Mat) translate(tx float64, ty ...float64) Mat { return m }\n")

	// Rect methods — consistent with recordMethods (value.go).
	b.WriteString("func (r Rect) w() float64 { return 0 }\n")
	b.WriteString("func (r Rect) h() float64 { return 0 }\n")
	b.WriteString("func (r Rect) center() Vec2 { return Vec2{} }\n")
	b.WriteString("func (r Rect) translate(v Vec2) Rect { return r }\n")
	b.WriteString("func (r Rect) scale(s float64) Rect { return r }\n")

	// Trans methods
	b.WriteString("func (t Trans) compose(o Trans) Trans { return t }\n")
	b.WriteString("func (t Trans) translate(v Vec2) Trans { return t }\n")
	b.WriteString("func (t Trans) scale(v Vec2) Trans { return t }\n")
	b.WriteString("func (t Trans) rotate(angle float64) Trans { return t }\n")
	b.WriteString("func (t Trans) transformVec(v Vec2) Vec2 { return Vec2{} }\n")

	// Pair methods
	b.WriteString("func (p Pair) lt(o Pair) float64 { return 0 }\n")
	b.WriteString("func (p Pair) le(o Pair) float64 { return 0 }\n")
	b.WriteString("func (p Pair) gt(o Pair) float64 { return 0 }\n")
	b.WriteString("func (p Pair) ge(o Pair) float64 { return 0 }\n")
	b.WriteString("func (p Pair) tuple() [2]float64 { return [2]float64{} }\n")

	// VarArray methods
	b.WriteString("func (a VarArray) len() float64 { return 0 }\n")
	b.WriteString("func (a VarArray) capacity() float64 { return 0 }\n")
	b.WriteString("func (a VarArray) isFull() float64 { return 0 }\n")
	b.WriteString("func (a VarArray) append(v float64) {}\n")
	b.WriteString("func (a VarArray) pop(args ...float64) float64 { return 0 }\n")
	b.WriteString("func (a VarArray) insert(idx, val float64) {}\n")
	b.WriteString("func (a VarArray) sort() {}\n")
	b.WriteString("func (a VarArray) clear() {}\n")
	b.WriteString("func (a VarArray) contains(v float64) float64 { return 0 }\n")
	b.WriteString("func (a VarArray) index(v float64) float64 { return 0 }\n")
	b.WriteString("func (a VarArray) remove(v float64) float64 { return 0 }\n")
	b.WriteString("func (a VarArray) setAdd(v float64) float64 { return 0 }\n")
	b.WriteString("func (a VarArray) setRemove(v float64) float64 { return 0 }\n")

	// ArrayMap methods
	b.WriteString("func (m ArrayMap) len() float64 { return 0 }\n")
	b.WriteString("func (m ArrayMap) capacity() float64 { return 0 }\n")
	b.WriteString("func (m ArrayMap) clear() {}\n")
	b.WriteString("func (m ArrayMap) keys() ArrayMap { return m }\n")
	b.WriteString("func (m ArrayMap) values() ArrayMap { return m }\n")
	b.WriteString("func (m ArrayMap) items() ArrayMap { return m }\n")
	b.WriteString("func (m ArrayMap) get(k float64) float64 { return 0 }\n")
	b.WriteString("func (m ArrayMap) set(k, v float64) {}\n")
	b.WriteString("func (m ArrayMap) delete(k float64) float64 { return 0 }\n")
	b.WriteString("func (m ArrayMap) contains(k float64) float64 { return 0 }\n")
	b.WriteString("func (m ArrayMap) pop(k float64) float64 { return 0 }\n")

	// FrozenNumSet methods
	b.WriteString("func (f FrozenNumSet) len() float64 { return 0 }\n")
	b.WriteString("func (f FrozenNumSet) capacity() float64 { return 0 }\n")
	b.WriteString("func (f FrozenNumSet) contains(v float64) float64 { return 0 }\n")

	// ArraySet methods
	b.WriteString("func (s ArraySet) len() float64 { return 0 }\n")
	b.WriteString("func (s ArraySet) capacity() float64 { return 0 }\n")
	b.WriteString("func (s ArraySet) clear() {}\n")
	b.WriteString("func (s ArraySet) add(v float64) float64 { return 0 }\n")
	b.WriteString("func (s ArraySet) remove(v float64) float64 { return 0 }\n")
	b.WriteString("func (s ArraySet) contains(v float64) float64 { return 0 }\n")

	// ── Static constructors ──
	b.WriteString("func vec2(x, y float64) Vec2 { return Vec2{} }\n")
	b.WriteString("func vec2Zero() Vec2 { return Vec2{} }\n")
	b.WriteString("func vec2One() Vec2 { return Vec2{} }\n")
	b.WriteString("func vec2Up() Vec2 { return Vec2{} }\n")
	b.WriteString("func vec2Down() Vec2 { return Vec2{} }\n")
	b.WriteString("func vec2Left() Vec2 { return Vec2{} }\n")
	b.WriteString("func vec2Right() Vec2 { return Vec2{} }\n")
	b.WriteString("func quad(blx, bly, tlx, tly, trx, try, brx, bry float64) Quad { return Quad{} }\n")
	b.WriteString("func mat(m11, m12, m13, m21, m22, m23 float64) Mat { return Mat{} }\n")
	b.WriteString("func rect(t, r, b, l float64) Rect { return Rect{} }\n")
	b.WriteString("func trans(m11, m12, m13, m21, m22, m23, m31, m32, m33 float64) Trans { return Trans{} }\n")
	b.WriteString("func pair(first, second float64) Pair { return Pair{} }\n")
	b.WriteString("func varArray(capacity float64) VarArray { return VarArray{} }\n")
	b.WriteString("func arrayMap(capacity float64) ArrayMap { return ArrayMap{} }\n")
	b.WriteString("func arraySet(capacity float64) ArraySet { return ArraySet{} }\n")
	b.WriteString("func box(val float64) Box { return Box{} }\n")
	b.WriteString("func frozenNumSet(capacity float64) FrozenNumSet { return FrozenNumSet{} }\n")
	b.WriteString("func sortLinkedEntities(head, sortKeyOffset, nextOffset float64, prevOffset ...float64) float64 { return 0 }\n")

	// ── Runtime functions (all ~206 builtins, derived from runtimeFns) ──
	for name, rf := range runtimeFns {
		if rf.arity >= 0 {
			// Fixed arity: generate typed parameter list.
			params := make([]string, rf.arity)
			for i := range params {
				params[i] = fmt.Sprintf("p%d float64", i)
			}
			b.WriteString(fmt.Sprintf("func %s(%s) float64 { return 0 }\n", name, strings.Join(params, ", ")))
		} else {
			// Variadic (impure pass-through, or pure variadic like getShifted).
			b.WriteString(fmt.Sprintf("func %s(args ...float64) float64 { return 0 }\n", name))
		}
	}

	// ── Memory accessors ──
	b.WriteString("func get(block, index float64) float64 { return 0 }\n")
	b.WriteString("func set(block, index, value float64) float64 { return 0 }\n")

	// ── Touches ──
	b.WriteString("func touchId(ti float64) float64 { return 0 }\n")
	b.WriteString("func touchStarted(ti float64) float64 { return 0 }\n")
	b.WriteString("func touchEnded(ti float64) float64 { return 0 }\n")
	b.WriteString("func touchX(ti float64) float64 { return 0 }\n")
	b.WriteString("func touchY(ti float64) float64 { return 0 }\n")

	// ── Mode checks ──
	b.WriteString("func isPlay() float64 { return 0 }\n")
	b.WriteString("func isWatch() float64 { return 0 }\n")
	b.WriteString("func isPreview() float64 { return 0 }\n")
	b.WriteString("func isTutorial() float64 { return 0 }\n")

	// ── Special ──
	b.WriteString("func screen() Rect { return Rect{} }\n")
	b.WriteString("func safeArea() Rect { return Rect{} }\n")
	b.WriteString("func sprite(name string) float64 { return 0 }\n")
	b.WriteString("func offsetAdjustedTime() float64 { return 0 }\n")
	b.WriteString("func prevTime() float64 { return 0 }\n")
	b.WriteString("func pnpoly(point Vec2, quad Quad) float64 { return 0 }\n")
	b.WriteString("func perspectiveApproach(x, y float64) float64 { return 0 }\n")

	// ── Engine globals (mode accessors + entity state) ──
	// These are read from the runtime environment at compile time but must be
	// declared here so go/types doesn't report them as undefined. All engine
	// values are float64; we declare a superset covering all modes.
	b.WriteString("var (\n")
	for _, name := range engineGlobals {
		b.WriteString("\t" + name + " float64\n")
	}
	b.WriteString(")\n")

	// ── Array ──
	// array is a declaration-only builtin; declare it as returning []float64.
	b.WriteString("func array(size float64) []float64 { return nil }\n")
	b.WriteString("func len(a []float64) float64 { return 0 }\n")

	return b.String()
}

// engineGlobals lists all mode accessor identifiers and entity state variables
// that are available as bare names in engine source. Declaring them in the
// prelude prevents go/types from reporting spurious "undefined" errors for
// engine builtins while still catching genuinely undeclared user identifiers.
var engineGlobals = []string{
	// Mode accessors (union across Play/Watch/Preview/Tutorial).
	"time", "deltaTime", "scaledTime",
	"touchCount", "isSkip", "isReplay",
	"isDebug", "aspectRatio", "audioOffset", "inputOffset",
	"isMultiplayer", "scrollDirection", "canvasSize",
	"navigationDirection",
	"safeAreaXMin", "safeAreaXMax", "safeAreaYMin", "safeAreaYMax",
	"perfectMultiplier", "greatMultiplier", "goodMultiplier",
	"lifeInitial", "lifeMaximum",
	// Entity score/life state (score_life.go).
	"entityPerfect", "entityGreat", "entityGood", "entityMiss",
	"entityLifePerfect", "entityLifeGreat", "entityLifeGood", "entityLifeMiss",
}

// TypeCheck parses and type-checks engine source combined with the prelude.
// Returns the *types.Info (for use in D2/D3 type-driven dispatch) or an error
// if the source fails type checking.
func TypeCheck(src string, records map[string][]string) (*token.FileSet, *ast.File, *types.Info, error) {
	fset, files, info, err := TypeCheckFiles(map[string]string{"engine.go": src}, records)
	if err != nil {
		return fset, nil, info, err
	}
	if len(files) > 0 {
		return fset, files[0], info, nil
	}
	return fset, nil, info, nil
}

// TypeCheckFiles type-checks multiple source files of the same package combined
// with the prelude. Files are keyed by filename (relative or base name). All files
// must share the same package name. Returns the token.FileSet, parsed AST files,
// and populated types.Info.
func TypeCheckFiles(files map[string]string, records map[string][]string) (*token.FileSet, []*ast.File, *types.Info, error) {
	if len(files) == 0 {
		return nil, nil, nil, fmt.Errorf("typecheck: no source files provided")
	}
	fset := token.NewFileSet()
	var pkgName string
	var userFiles []*ast.File
	for name, src := range files {
		f, err := parser.ParseFile(fset, name, src, 0)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("typecheck: parse %s: %w", name, err)
		}
		if pkgName == "" {
			pkgName = f.Name.Name
		} else if f.Name.Name != pkgName {
			return nil, nil, nil, fmt.Errorf("typecheck: conflicting package names: %q uses %q, expected %q", name, f.Name.Name, pkgName)
		}
		userFiles = append(userFiles, f)
	}
	info, err := checkWithPrelude(fset, pkgName, userFiles, records)
	if err != nil {
		return fset, userFiles, info, err
	}
	return fset, userFiles, info, nil
}

// checkWithPrelude runs type-checking on the prelude + user files and applies
// the engine DSL error filter: only undeclared identifiers and wrong argument
// counts are propagated; Go-strictness mismatches (float64 indices, implicit
// callback returns) are treated as soft and ignored.
func checkWithPrelude(fset *token.FileSet, pkgName string, userFiles []*ast.File, records map[string][]string) (*types.Info, error) {
	return checkWithPreludeImp(fset, pkgName, userFiles, records, importer.Default())
}

// SubPreludeSource returns a minimal prelude for imported sub-packages.
// Sub-packages only need type declarations (Vec2, Mat, Quad, Rect, Trans, Pair,
// handle types) so their struct field types resolve correctly. They do not need
// runtime function stubs, constructors, or engine globals — imported packages
// define archetype structs but do not call Draw/Spawn/etc. at the package level.
func SubPreludeSource(pkg string) string {
	var b strings.Builder
	b.WriteString("package " + pkg + "\n\n")
	// Record types — needed for struct field types.
	b.WriteString("type Vec2 struct { x, y float64 }\n")
	b.WriteString("type Quad struct { blx, bly, tlx, tly, trx, try, brx, bry float64 }\n")
	b.WriteString("type Mat struct { m11, m12, m13, m21, m22, m23 float64 }\n")
	b.WriteString("type Rect struct { t, r, b, l float64 }\n")
	b.WriteString("type Trans struct { m11, m12, m13, m21, m22, m23, m31, m32, m33 float64 }\n")
	b.WriteString("type Pair struct { first, second float64 }\n")
	// Handle types.
	b.WriteString("type LoopedEffectHandle struct { id float64 }\n")
	b.WriteString("type ScheduledLoopedEffectHandle struct { id float64 }\n")
	b.WriteString("type ParticleHandle struct { id float64 }\n")
	return b.String()
}

// checkWithPreludeImp is the internal variant of checkWithPrelude that accepts
// a custom types.Importer. When imp is nil, importer.Default() is used.
// TypeCheckEngine uses this with an engineImporter to resolve local imports.
func checkWithPreludeImp(fset *token.FileSet, pkgName string, userFiles []*ast.File, records map[string][]string, imp types.Importer) (*types.Info, error) {
	preludeSrc := PreludeSource(pkgName, records)
	preludeFile, err := parser.ParseFile(fset, "prelude.go", preludeSrc, 0)
	if err != nil {
		return nil, fmt.Errorf("typecheck: prelude parse: %w", err)
	}
	allFiles := append([]*ast.File{preludeFile}, userFiles...)
	if imp == nil {
		imp = importer.Default()
	}
	conf := types.Config{Importer: imp}
	info := &types.Info{
		Types:      map[ast.Expr]types.TypeAndValue{},
		Defs:       map[*ast.Ident]types.Object{},
		Uses:       map[*ast.Ident]types.Object{},
		Selections: map[*ast.SelectorExpr]*types.Selection{},
	}
	_, err = conf.Check(pkgName, fset, allFiles, info)
	if err != nil {
		if hard := filterHardErrors(err); hard != nil {
			return info, fmt.Errorf("typecheck: %w", hard)
		}
	}
	return info, nil
}

// filterHardErrors returns err only for diagnostics that indicate genuine
// user mistakes: undeclared identifiers and wrong argument counts. All other
// type errors (float64 indices, implicit callback returns, etc.) are soft —
// they arise from Go's stricter type rules that don't apply to the engine DSL.
//
// Prefer structured error inspection via errors.As when available; fall back
// to message substring matching for compatibility with older Go versions.
func filterHardErrors(err error) error {
	if err == nil {
		return nil
	}
	// Try structured inspection first (go/types errors carry optional codes).
	var te *types.Error
	if errors.As(err, &te) {
		msg := te.Msg
		// sonolus package declarations exist for IDE support only; the compiler
		// provides equivalent symbols via its internal prelude.
		if strings.Contains(msg, "sonolus") {
			return nil
		}
		if strings.Contains(msg, "undeclared name") ||
			strings.Contains(msg, "not enough arguments") ||
			strings.Contains(msg, "too many arguments") ||
			strings.Contains(msg, "undefined: ") {
			return err
		}
		return nil
	}
	// Fallback: inspect the error message string.
	msg := err.Error()
	if strings.Contains(msg, "sonolus") {
		return nil
	}
	if strings.Contains(msg, "undefined: ") ||
		strings.Contains(msg, "not enough arguments") ||
		strings.Contains(msg, "too many arguments") {
		return err
	}
	return nil
}
