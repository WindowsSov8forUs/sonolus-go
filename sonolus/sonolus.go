// Package sonolus provides the type declarations and runtime function stubs
// for Sonolus engine DSL. All functions here are declaration-only (body returns
// zero values) — they exist solely to make engine source visible to standard Go
// tools (gopls, go vet, IDE). The sonolus-go compiler recognizes sonolus.Xxx()
// calls and translates them to Sonolus IR operations internally.
//
// Engine authors import this package and prefix all runtime calls:
//
//	import "github.com/WindowsSov8forUs/sonolus-go/sonolus"
//
//	func (n *Note) Initialize() {
//	    sonolus.Draw(sonolus.SpriteNote, n.Beat, 0, 1, 1, 0, 1, 0, 0)
//	}
package sonolus

// ── Record types ──

type Vec2 struct{ X, Y float64 }
type Quad struct{ Blx, Bly, Tlx, Tly, Trx, Try, Brx, Bry float64 }
type Mat struct{ M11, M12, M13, M21, M22, M23 float64 }
type Rect struct{ T, R, B, L float64 }
type Trans struct{ M11, M12, M13, M21, M22, M23, M31, M32, M33 float64 }
type Pair struct{ First, Second float64 }
type VarArray struct{ Size float64; Array []float64 }
type ArrayMap struct{ Size float64; Array []float64 }
type ArraySet struct{ Values VarArray }
type Box struct{ Val float64 }
type FrozenNumSet struct{ Size float64; Array []float64 }
type JudgmentWindow struct {
	PerfectMin, PerfectMax, GreatMin, GreatMax, GoodMin, GoodMax float64
}

// PlayEntityInput represents the per-entity result written to the EntityInput
// block (4005). Fields map to: judgment(0), accuracy(1), bucket index(2),
// bucket value(3), haptic(4). Equivalent to EntityInputResult in sonolus.js
// and PlayEntityInput in sonolus.py.
type PlayEntityInput struct {
	Judgment    float64
	Accuracy    float64
	BucketIndex float64
	BucketValue float64
	Haptic      float64
}

// EntityScore represents per-entity score contributions at block 4006.
// Used with sonolus:"scored" tag. Equivalent to EntityScore in sonolus.js.
type EntityScore struct{ Perfect, Great, Good, Miss float64 }

// EntityLife represents per-entity life contributions at block 4007.
// Used with sonolus:"lifed" tag. Equivalent to EntityLife in sonolus.js
// and LifeInfo in sonolus.py.
type EntityLife struct{ Perfect, Great, Good, Miss float64 }

// ConsecutiveLife represents combo-scaling life increments at block 2005.
// Equivalent to life.consecutive.perfect in sonolus.js.
type ConsecutiveLife struct{ Increment, Step float64 }

// LifeConsecutiveEntries groups the three ConsecutiveLife entries.
type LifeConsecutiveEntries struct{ Perfect, Great, Good ConsecutiveLife }

// LifeInfo provides a unified namespace for all life settings (block 2005).
// Equivalent to the life object in sonolus.js.
type LifeInfo struct {
	Consecutive LifeConsecutiveEntries
	Initial     float64
	Max         float64
}

// Life returns a LifeInfo record providing structured access to level life settings.
func Life() LifeInfo { return LifeInfo{} }

// Archetype returns the archetype life settings (block 5000) for the given archetype index.
func (l LifeInfo) Archetype(idx float64) EntityLife { return EntityLife{} }

// AddScheduled schedules a life change at the given time.
func (l LifeInfo) AddScheduled(value, time float64) {}

// Judge evaluates the judgment for a hit given the actual and target times.
// Returns EntityStateWaiting (0, Miss), 1 (Perfect), 2 (Great), or 3 (Good).
// Equivalent to input.judge(actual, target, windows) in sonolus.js and
// window.judge(actual, target) in sonolus.py.
func (jw JudgmentWindow) Judge(actual, target float64) float64 { return 0 }

// ── Handle types ──

type LoopedEffectHandle struct{ ID float64 }
type ScheduledLoopedEffectHandle struct{ ID float64 }
type ParticleHandle struct{ ID float64 }
type EntityRef struct{ Index float64 }

// ── Resource record types ──

// EffectClip represents a sound effect clip with a compile-time ID.
// Created via sonolus.EffectClip(name). Equivalent to EffectClip in sonolus.js
// and Effect in sonolus.py.
type EffectClip struct{ ID float64 }

// Play plays the effect clip at the given distance.
// Equivalent to clip.play(distance) in sonolus.js and effect.play(distance) in sonolus.py.
func (c EffectClip) Play(distance float64) {}

// Schedule schedules the effect clip to play at a specific time with the given distance.
// Equivalent to clip.schedule(time, distance) in sonolus.js and effect.schedule(time, distance) in sonolus.py.
func (c EffectClip) Schedule(time, distance float64) {}

// ParticleClip represents a particle effect with a compile-time ID.
// Created via sonolus.ParticleClip(name). Equivalent to ParticleEffect in sonolus.js
// and ParticleEffect in sonolus.py.
type ParticleClip struct{ ID float64 }

// Spawn spawns the particle effect. Composite args (Quad, Vec2) are auto-destructured.
// Returns a ParticleHandle for move/destroy.
func (c ParticleClip) Spawn(args ...float64) ParticleHandle { return ParticleHandle{} }

// ── Handle methods ──

func (h LoopedEffectHandle) Stop()                                 {}
func (h ScheduledLoopedEffectHandle) Stop(endTime float64)         {}
func (h ParticleHandle) Move(quad Quad)                            {}
func (h ParticleHandle) Destroy()                                  {}
func (r EntityRef) Get(block, index float64) float64               { return 0 }
func (r EntityRef) Set(block, index, value float64)                {}

// ── Vec2 methods ──

func (v Vec2) Add(o Vec2) Vec2                     { return Vec2{} }
func (v Vec2) Sub(o Vec2) Vec2                     { return Vec2{} }
func (v Vec2) Mul(s float64) Vec2                  { return Vec2{} }
func (v Vec2) Div(s float64) Vec2                  { return Vec2{} }
func (v Vec2) Magnitude() float64                  { return 0 }
func (v Vec2) Dot(o Vec2) float64                  { return 0 }
func (v Vec2) Normalize() Vec2                     { return Vec2{} }
func (v Vec2) NormalizeOrZero() Vec2               { return Vec2{} }
func (v Vec2) Angle() float64                      { return 0 }
func (v Vec2) Rotate(angle float64) Vec2           { return Vec2{} }
func (v Vec2) Orthogonal() Vec2                    { return Vec2{} }
func (v Vec2) RotateAbout(pt Vec2, angle float64) Vec2 { return Vec2{} }
func (v Vec2) AngleDiff(o Vec2) float64            { return 0 }
func (v Vec2) SignedAngleDiff(o Vec2) float64      { return 0 }

// ── Quad methods ──

func (q Quad) Center() Vec2                    { return Vec2{} }
func (q Quad) Translate(v Vec2) Quad           { return Quad{} }
func (q Quad) Scale(s float64) Quad            { return Quad{} }
func (q Quad) Permute(rotation float64) Quad   { return Quad{} }
func (q Quad) Rotate(angle float64) Quad       { return Quad{} }
func (q Quad) Top() Vec2                       { return Vec2{} }
func (q Quad) Right() Vec2                     { return Vec2{} }
func (q Quad) Bottom() Vec2                    { return Vec2{} }
func (q Quad) Left() Vec2                      { return Vec2{} }
func (q Quad) Contains(p Vec2) float64         { return 0 }

// ── Mat methods ──

func (m Mat) Scale(sx float64, sy ...float64) Mat     { return Mat{} }
func (m Mat) Translate(tx float64, ty ...float64) Mat  { return Mat{} }

// ── Rect methods ──

func (r Rect) W() float64                  { return 0 }
func (r Rect) H() float64                  { return 0 }
func (r Rect) Center() Vec2                { return Vec2{} }
func (r Rect) Translate(v Vec2) Rect       { return Rect{} }
func (r Rect) Scale(s float64) Rect        { return Rect{} }

// ── Trans methods ──

func (t Trans) Compose(o Trans) Trans           { return Trans{} }
func (t Trans) Translate(v Vec2) Trans          { return Trans{} }
func (t Trans) Scale(v Vec2) Trans              { return Trans{} }
func (t Trans) Rotate(angle float64) Trans      { return Trans{} }
func (t Trans) TransformVec(v Vec2) Vec2        { return Vec2{} }

// ── Pair methods ──

func (p Pair) Lt(o Pair) float64    { return 0 }
func (p Pair) Le(o Pair) float64    { return 0 }
func (p Pair) Gt(o Pair) float64    { return 0 }
func (p Pair) Ge(o Pair) float64    { return 0 }
func (p Pair) Tuple() [2]float64    { return [2]float64{} }

// ── VarArray methods ──

func (a VarArray) Len() float64              { return 0 }
func (a VarArray) Capacity() float64         { return 0 }
func (a VarArray) IsFull() float64           { return 0 }
func (a VarArray) Append(v float64)          {}
func (a VarArray) Pop(args ...float64) float64 { return 0 }
func (a VarArray) Insert(idx, val float64)   {}
func (a VarArray) Sort()                     {}
func (a VarArray) Clear()                    {}
func (a VarArray) Contains(v float64) float64 { return 0 }
func (a VarArray) Index(v float64) float64   { return 0 }
func (a VarArray) Remove(v float64) float64  { return 0 }
func (a VarArray) SetAdd(v float64) float64  { return 0 }
func (a VarArray) SetRemove(v float64) float64 { return 0 }

// ── ArrayMap methods ──

func (m ArrayMap) Len() float64               { return 0 }
func (m ArrayMap) Capacity() float64          { return 0 }
func (m ArrayMap) Clear()                     {}
func (m ArrayMap) Keys() ArrayMap             { return ArrayMap{} }
func (m ArrayMap) Values() ArrayMap           { return ArrayMap{} }
func (m ArrayMap) Items() ArrayMap            { return ArrayMap{} }
func (m ArrayMap) Get(k float64) float64      { return 0 }
func (m ArrayMap) Set(k, v float64)           {}
func (m ArrayMap) Delete(k float64) float64   { return 0 }
func (m ArrayMap) Contains(k float64) float64 { return 0 }
func (m ArrayMap) Pop(k float64) float64      { return 0 }

// ── FrozenNumSet methods ──

func (f FrozenNumSet) Len() float64              { return 0 }
func (f FrozenNumSet) Capacity() float64         { return 0 }
func (f FrozenNumSet) Contains(v float64) float64 { return 0 }

// ── ArraySet methods ──

func (s ArraySet) Len() float64               { return 0 }
func (s ArraySet) Capacity() float64          { return 0 }
func (s ArraySet) Clear()                     {}
func (s ArraySet) Add(v float64) float64      { return 0 }
func (s ArraySet) Remove(v float64) float64   { return 0 }
func (s ArraySet) Contains(v float64) float64 { return 0 }

// ── Static constructors ──

func Vec2_(x, y float64) Vec2                                 { return Vec2{} }
func Vec2Zero() Vec2                                           { return Vec2{} }
func Vec2One() Vec2                                            { return Vec2{} }
func Vec2Up() Vec2                                             { return Vec2{} }
func Vec2Down() Vec2                                           { return Vec2{} }
func Vec2Left() Vec2                                           { return Vec2{} }
func Vec2Right() Vec2                                          { return Vec2{} }
func Quad_(blx, bly, tlx, tly, trx, try, brx, bry float64) Quad { return Quad{} }
func Mat_(m11, m12, m13, m21, m22, m23 float64) Mat           { return Mat{} }
func Rect_(t, r, b, l float64) Rect                            { return Rect{} }
func Trans_(m11, m12, m13, m21, m22, m23, m31, m32, m33 float64) Trans { return Trans{} }
func Pair_(first, second float64) Pair                         { return Pair{} }
func VarArray_(capacity float64) VarArray                      { return VarArray{} }
func ArrayMap_(capacity float64) ArrayMap                      { return ArrayMap{} }
func ArraySet_(capacity float64) ArraySet                      { return ArraySet{} }
func Box_(val float64) Box                                     { return Box{} }
func EffectClip_(id float64) EffectClip                        { return EffectClip{} }
func ParticleClip_(id float64) ParticleClip                    { return ParticleClip{} }
func FrozenNumSet_(capacity float64) FrozenNumSet              { return FrozenNumSet{} }
func SortLinkedEntities(head, sortKeyOffset, nextOffset float64, prevOffset ...float64) float64 { return 0 }

// ── Runtime functions — pure math ──

func Abs(x float64) float64                     { return 0 }
func Sign(x float64) float64                    { return 0 }
func Floor(x float64) float64                   { return 0 }
func Ceil(x float64) float64                    { return 0 }
func Round(x float64) float64                   { return 0 }
func Trunc(x float64) float64                   { return 0 }
func Frac(x float64) float64                    { return 0 }
func Log(x float64) float64                     { return 0 }
func Sin(x float64) float64                     { return 0 }
func Cos(x float64) float64                     { return 0 }
func Tan(x float64) float64                     { return 0 }
func Sinh(x float64) float64                    { return 0 }
func Cosh(x float64) float64                    { return 0 }
func Tanh(x float64) float64                    { return 0 }
func Asin(x float64) float64                    { return 0 }
func Acos(x float64) float64                    { return 0 }
func Atan(x float64) float64                    { return 0 }
func Atan2(y, x float64) float64                { return 0 }
func Degree(x float64) float64                  { return 0 }
func Radian(x float64) float64                  { return 0 }
func Min(a, b float64) float64                  { return 0 }
func Max(a, b float64) float64                  { return 0 }
func Clamp(x, lo, hi float64) float64           { return 0 }
func Power(base, exp float64) float64           { return 0 }
func Mod(x, y float64) float64                  { return 0 }
func Rem(x, y float64) float64                  { return 0 }
func Lerp(a, b, t float64) float64              { return 0 }
func LerpClamped(a, b, t float64) float64       { return 0 }
func Interp(a, b, t float64) float64            { return 0 }
func InterpClamped(a, b, t float64) float64     { return 0 }
func Unlerp(a, b, x float64) float64            { return 0 }
func UnlerpClamped(a, b, x float64) float64     { return 0 }
func Remap(a1, b1, a2, b2, x float64) float64   { return 0 }
func RemapClamped(a1, b1, a2, b2, x float64) float64 { return 0 }

// ── Comparison and logic ──
//
// The engine runtime defines Equal, NotEqual, Less, LessOr, Greater, GreaterOr,
// And, Or, and Not as RuntimeFunctions. In sonolus-go these are expressed through
// native Go operators (== != < <= > >= && || !) compiled via applyBinary/applyUnary,
// not through sonolus.Xxx() calls. The corresponding stub functions are intentionally
// omitted — use Go operators directly.

// ── Runtime functions — easing ──

func EaseInSine(x float64) float64      { return 0 }
func EaseOutSine(x float64) float64     { return 0 }
func EaseInOutSine(x float64) float64   { return 0 }
func EaseOutInSine(x float64) float64   { return 0 }
func EaseInQuad(x float64) float64      { return 0 }
func EaseOutQuad(x float64) float64     { return 0 }
func EaseInOutQuad(x float64) float64   { return 0 }
func EaseOutInQuad(x float64) float64   { return 0 }
func EaseInCubic(x float64) float64     { return 0 }
func EaseOutCubic(x float64) float64    { return 0 }
func EaseInOutCubic(x float64) float64  { return 0 }
func EaseOutInCubic(x float64) float64  { return 0 }
func EaseInQuart(x float64) float64     { return 0 }
func EaseOutQuart(x float64) float64    { return 0 }
func EaseInOutQuart(x float64) float64  { return 0 }
func EaseOutInQuart(x float64) float64  { return 0 }
func EaseInQuint(x float64) float64     { return 0 }
func EaseOutQuint(x float64) float64    { return 0 }
func EaseInOutQuint(x float64) float64  { return 0 }
func EaseOutInQuint(x float64) float64  { return 0 }
func EaseInExpo(x float64) float64      { return 0 }
func EaseOutExpo(x float64) float64     { return 0 }
func EaseInOutExpo(x float64) float64   { return 0 }
func EaseOutInExpo(x float64) float64   { return 0 }
func EaseInCirc(x float64) float64      { return 0 }
func EaseOutCirc(x float64) float64     { return 0 }
func EaseInOutCirc(x float64) float64   { return 0 }
func EaseOutInCirc(x float64) float64   { return 0 }
func EaseInBack(x float64) float64      { return 0 }
func EaseOutBack(x float64) float64     { return 0 }
func EaseInOutBack(x float64) float64   { return 0 }
func EaseOutInBack(x float64) float64   { return 0 }
func EaseInElastic(x float64) float64   { return 0 }
func EaseOutElastic(x float64) float64  { return 0 }
func EaseInOutElastic(x float64) float64 { return 0 }
func EaseOutInElastic(x float64) float64 { return 0 }

// ── Runtime functions — side-effecting (variadic) ──

func Draw(args ...float64) float64                    { return 0 }
func Play(args ...float64) float64                    { return 0 }
func PlayScheduled(args ...float64) float64           { return 0 }
func Spawn(args ...float64) float64                   { return 0 }
func SpawnParticle(args ...float64) float64           { return 0 }
func MoveParticle(args ...float64) float64            { return 0 }
func DestroyParticle(args ...float64) float64         { return 0 }
func Print(args ...float64) float64                   { return 0 }
func Paint(args ...float64) float64                   { return 0 }
func DrawCurvedB(args ...float64) float64             { return 0 }
func DrawCurvedT(args ...float64) float64             { return 0 }
func DrawCurvedL(args ...float64) float64             { return 0 }
func DrawCurvedR(args ...float64) float64             { return 0 }
func DrawCurvedBT(args ...float64) float64            { return 0 }
func DrawCurvedLR(args ...float64) float64            { return 0 }
func PlayLooped(args ...float64) float64              { return 0 }
func PlayLoopedScheduled(args ...float64) float64     { return 0 }
func StopLooped(args ...float64) float64              { return 0 }
func StopLoopedScheduled(args ...float64) float64     { return 0 }
func Judge(args ...float64) float64                   { return 0 }
func JudgeSimple(args ...float64) float64             { return 0 }
func ExportValue(args ...float64) float64             { return 0 }
func AddLife(args ...float64) float64                 { return 0 }
func Copy(args ...float64) float64                    { return 0 }

// ── Runtime functions — debug ──

func DebugLog(args ...float64) float64    { return 0 }
func DebugPause(args ...float64) float64  { return 0 }
func DebugError(msg float64)              {}
func DebugRequire(cond, msg float64)       {}
func DebugAssertTrue(cond, msg float64)    {}
func DebugAssertFalse(cond, msg float64)   {}
func DebugTerminate()                     {}

// ── Entity state constants ──

// EntityState values for entity lifecycle tracking (Play/Watch modes).
// Matches sonolus.js EntityState enum and sonolus.py is_waiting/is_active/is_despawned.
const (
	EntityStateWaiting   float64 = 0
	EntityStateActive    float64 = 1
	EntityStateDespawned float64 = 2
)

// ── EntityInfo record type ──

// EntityInfo is a structured record representing an entity's compile-time
// info. Fields: Index (entity's own index), Archetype (archetype ID),
// State (lifecycle: 0=Waiting, 1=Active, 2=Despawned).
// Matches PlayEntityInfo / WatchEntityInfo in sonolus.py and the EntityInfo
// type in sonolus.js.
type EntityInfo struct {
	Index     float64
	Archetype float64
	State     float64
}

// IsWaiting returns 1 if the entity is in the Waiting state (state == 0).
func (info EntityInfo) IsWaiting() float64 { return 0 }

// IsActive returns 1 if the entity is in the Active state (state == 1).
func (info EntityInfo) IsActive() float64 { return 0 }

// IsDespawned returns 1 if the entity is in the Despawned state (state == 2).
func (info EntityInfo) IsDespawned() float64 { return 0 }

// ── Convenience wrappers ──

// EntityInfoIndex reads the entity's own index from EntityInfoArray at the given
// entity index. Equivalent to entityInfos.get(index).index in sonolus.js and
// entity_info_at(index).index in sonolus.py.
func EntityInfoIndex(index float64) float64 { return 0 }

// EntityInfoArchetype reads the entity's archetype ID from EntityInfoArray at the
// given entity index. Equivalent to entityInfos.get(index).archetype in sonolus.js
// and entity_info_at(index).archetype_id in sonolus.py.
func EntityInfoArchetype(index float64) float64 { return 0 }

// EntityInfoState reads the entity's lifecycle state from EntityInfoArray at the
// given entity index. Returns EntityStateWaiting (0), EntityStateActive (1), or
// EntityStateDespawned (2). Equivalent to entityInfos.get(index).state in
// sonolus.js and entity_info_at(index).state in sonolus.py.
// Not available in Preview mode.
func EntityInfoState(index float64) float64 { return 0 }

// EntityInfoAt returns the full EntityInfo record for the entity at the given
// index. Equivalent to entityInfos.get(index) in sonolus.js and
// entity_info_at(index) in sonolus.py.
func EntityInfoAt(index float64) EntityInfo { return EntityInfo{} }

// SelfInfo returns the EntityInfo record for the current entity.
// Equivalent to this.info in sonolus.js and self._info in sonolus.py.
func SelfInfo() EntityInfo { return EntityInfo{} }

// ── Memory block constants ──

// Block ID constants for commonly accessed engine memory blocks.
const (
	BlockLevelMemory       = 2000
	BlockLevelData         = 2001
	BlockSkinTransform     = 1003
	BlockParticleTransform = 1004
	BlockBackground        = 1005
	BlockUI                = 1006
	BlockUIConfiguration   = 1007
	BlockLevelScore        = 2004
	BlockLevelLife         = 2005
)

// ── Transform / background convenience getters/setters ──

func SkinTransform(index float64) float64          { return Get(BlockSkinTransform, index) }
func SetSkinTransform(index, value float64)         { Set(BlockSkinTransform, index, value) }
func ParticleTransform(index float64) float64       { return Get(BlockParticleTransform, index) }
func SetParticleTransform(index, value float64)     { Set(BlockParticleTransform, index, value) }
func Background(index float64) float64              { return Get(BlockBackground, index) }
func SetBackground(index, value float64)            { Set(BlockBackground, index, value) }

// ── Score / life configuration getters/setters ──

func LevelScore(index float64) float64              { return Get(BlockLevelScore, index) }
func SetLevelScore(index, value float64)            { Set(BlockLevelScore, index, value) }
func LevelLife(index float64) float64               { return Get(BlockLevelLife, index) }
func SetLevelLife(index, value float64)             { Set(BlockLevelLife, index, value) }

// ── Runtime functions — memory ──

func Get(block, index float64) float64           { return 0 }
func Set(block, index, value float64) float64    { return 0 }
func GetShifted(args ...float64) float64         { return 0 }
func GetPointed(args ...float64) float64         { return 0 }
func SetShifted(args ...float64) float64         { return 0 }
func SetPointed(args ...float64) float64         { return 0 }
func SetAddShifted(args ...float64) float64      { return 0 }
func SetAddPointed(args ...float64) float64      { return 0 }
func SetSubShifted(args ...float64) float64      { return 0 }
func SetSubPointed(args ...float64) float64      { return 0 }
func SetMulShifted(args ...float64) float64      { return 0 }
func SetMulPointed(args ...float64) float64      { return 0 }
func SetDivShifted(args ...float64) float64      { return 0 }
func SetDivPointed(args ...float64) float64      { return 0 }
func SetModShifted(args ...float64) float64      { return 0 }
func SetModPointed(args ...float64) float64      { return 0 }
func SetRemShifted(args ...float64) float64      { return 0 }
func SetRemPointed(args ...float64) float64      { return 0 }
func SetPowShifted(args ...float64) float64      { return 0 }
func SetPowPointed(args ...float64) float64      { return 0 }
func IncrementPre(args ...float64) float64       { return 0 }
func IncrementPost(args ...float64) float64      { return 0 }
func IncrementPrePointed(args ...float64) float64 { return 0 }
func IncrementPostPointed(args ...float64) float64 { return 0 }
func IncrementPreShifted(args ...float64) float64 { return 0 }
func IncrementPostShifted(args ...float64) float64 { return 0 }
func DecrementPre(args ...float64) float64       { return 0 }
func DecrementPost(args ...float64) float64      { return 0 }
func DecrementPrePointed(args ...float64) float64 { return 0 }
func DecrementPostPointed(args ...float64) float64 { return 0 }
func DecrementPreShifted(args ...float64) float64 { return 0 }
func DecrementPostShifted(args ...float64) float64 { return 0 }

// ── Runtime functions — stack ──

func StackInit(args ...float64) float64         { return 0 }
func StackPush(args ...float64) float64         { return 0 }
func StackPop(args ...float64) float64          { return 0 }
func StackGrow(args ...float64) float64         { return 0 }
func StackEnter(args ...float64) float64        { return 0 }
func StackLeave(args ...float64) float64        { return 0 }
func StackGet(args ...float64) float64          { return 0 }
func StackSet(args ...float64) float64          { return 0 }
func StackGetPointer(args ...float64) float64   { return 0 }
func StackSetPointer(args ...float64) float64   { return 0 }
func StackGetFrame(args ...float64) float64     { return 0 }
func StackSetFrame(args ...float64) float64     { return 0 }
func StackGetFramePointer(args ...float64) float64 { return 0 }
func StackSetFramePointer(args ...float64) float64 { return 0 }

// ── Runtime functions — timing ──

func BeatToTime(args ...float64) float64             { return 0 }
func BeatToBPM(args ...float64) float64              { return 0 }
func TimeToScaledTime(args ...float64) float64       { return 0 }
func TimeToStartingTime(args ...float64) float64     { return 0 }
func TimeToStartingScaledTime(args ...float64) float64 { return 0 }
func BeatToStartingBeat(args ...float64) float64     { return 0 }
func BeatToStartingTime(args ...float64) float64     { return 0 }
func TimeToTimeScale(args ...float64) float64        { return 0 }

// ── Runtime functions — touch ──

func TouchID(ti float64) float64      { return 0 }
func TouchStarted(ti float64) float64  { return 0 }
func TouchEnded(ti float64) float64   { return 0 }
func TouchX(ti float64) float64       { return 0 }
func TouchY(ti float64) float64       { return 0 }

// ── Runtime functions — random ──

func Random(args ...float64) float64     { return 0 }
func RandomInteger(args ...float64) float64 { return 0 }

// ── Runtime functions — stream ──

func StreamGetValue(args ...float64) float64       { return 0 }
func StreamGetNextKey(args ...float64) float64     { return 0 }
func StreamGetPreviousKey(args ...float64) float64 { return 0 }
func StreamHas(args ...float64) float64            { return 0 }
func StreamSet(args ...float64) float64            { return 0 }

// ── Runtime functions — resource query ──

func HasSkinSprite(args ...float64) float64   { return 0 }
func HasEffectClip(args ...float64) float64   { return 0 }
func HasParticle(args ...float64) float64     { return 0 }
func Sprite(name string) float64               { return 0 }

// ── Runtime functions — mode checks ──

func IsPlay() float64     { return 0 }
func IsWatch() float64    { return 0 }
func IsPreview() float64  { return 0 }
func IsTutorial() float64 { return 0 }

// ── Runtime functions — special ──

func Screen() Rect                { return Rect{} }
func SafeArea() Rect              { return Rect{} }
func OffsetAdjustedTime() float64 { return 0 }
func PrevTime() float64           { return 0 }
func Pnpoly(point Vec2, quad Quad) float64 { return 0 }
func PerspectiveApproach(x, y float64) float64 { return 0 }

// ── Runtime functions — misc ──

func Array(args ...float64) []float64       { return nil }
func Len(a []float64) float64               { return 0 }

// ── Engine globals ──

var (
	Time               float64
	DeltaTime          float64
	ScaledTime         float64
	TouchCount         float64
	IsSkip             float64
	IsReplay           float64
	IsDebug            float64
	AspectRatio        float64
	AudioOffset        float64
	InputOffset        float64
	IsMultiplayer      float64
	ScrollDirection    float64
	CanvasSize         float64
	NavigationDirection float64
	SafeAreaXMin       float64
	SafeAreaXMax       float64
	SafeAreaYMin       float64
	SafeAreaYMax       float64
	PerfectMultiplier  float64
	GreatMultiplier    float64
	GoodMultiplier     float64
	LifeInitial        float64
	LifeMaximum        float64
	EntityPerfect      float64
	EntityGreat        float64
	EntityGood         float64
	EntityMiss         float64
	EntityLifePerfect  float64
	EntityLifeGreat    float64
	EntityLifeGood     float64
	EntityLifeMiss     float64
)

// ── UI configuration types ──────────────────────────────────────────────────

type (
	Metric                  string
	JudgmentErrorStyle      string
	JudgmentErrorPlacement  string
	Ease                    string
)

type Visibility struct{ Scale, Alpha float64 }

type Tween struct {
	From, To, Duration float64
	Ease               Ease
}

type Animation struct{ Scale, Alpha Tween }

// Metrics
const (
	MetricArcade              Metric = "arcade"
	MetricArcadePercentage    Metric = "arcadePercentage"
	MetricAccuracy            Metric = "accuracy"
	MetricAccuracyPercentage  Metric = "accuracyPercentage"
	MetricLife                Metric = "life"
	MetricPerfect             Metric = "perfect"
	MetricPerfectPercentage   Metric = "perfectPercentage"
	MetricGreatGoodMiss       Metric = "greatGoodMiss"
	MetricGreatGoodMissPct    Metric = "greatGoodMissPercentage"
	MetricMiss                Metric = "miss"
	MetricMissPercentage      Metric = "missPercentage"
	MetricErrorHeatmap        Metric = "errorHeatmap"
)

// Judgment error styles
const (
	JudgmentErrorNone          JudgmentErrorStyle = "none"
	JudgmentErrorLate          JudgmentErrorStyle = "late"
	JudgmentErrorEarly         JudgmentErrorStyle = "early"
	JudgmentErrorPlus          JudgmentErrorStyle = "plus"
	JudgmentErrorMinus         JudgmentErrorStyle = "minus"
	JudgmentErrorArrowUp       JudgmentErrorStyle = "arrowUp"
	JudgmentErrorArrowDown     JudgmentErrorStyle = "arrowDown"
	JudgmentErrorArrowLeft     JudgmentErrorStyle = "arrowLeft"
	JudgmentErrorArrowRight    JudgmentErrorStyle = "arrowRight"
	JudgmentErrorTriangleUp    JudgmentErrorStyle = "triangleUp"
	JudgmentErrorTriangleDown  JudgmentErrorStyle = "triangleDown"
	JudgmentErrorTriangleLeft  JudgmentErrorStyle = "triangleLeft"
	JudgmentErrorTriangleRight JudgmentErrorStyle = "triangleRight"
)

// Judgment error placements
const (
	JudgmentErrorPlacementLeft      JudgmentErrorPlacement = "left"
	JudgmentErrorPlacementRight     JudgmentErrorPlacement = "right"
	JudgmentErrorPlacementLeftRight JudgmentErrorPlacement = "leftRight"
	JudgmentErrorPlacementTop       JudgmentErrorPlacement = "top"
	JudgmentErrorPlacementBottom    JudgmentErrorPlacement = "bottom"
	JudgmentErrorPlacementTopBottom JudgmentErrorPlacement = "topBottom"
	JudgmentErrorPlacementCenter    JudgmentErrorPlacement = "center"
)

// Easing functions
const (
	EasingLinear      Ease = "linear"
	EasingNone        Ease = "none"
	EasingInSine      Ease = "inSine"
	EasingInQuad      Ease = "inQuad"
	EasingInCubic     Ease = "inCubic"
	EasingInQuart     Ease = "inQuart"
	EasingInQuint     Ease = "inQuint"
	EasingInExpo      Ease = "inExpo"
	EasingInCirc      Ease = "inCirc"
	EasingInBack      Ease = "inBack"
	EasingInElastic   Ease = "inElastic"
	EasingOutSine     Ease = "outSine"
	EasingOutQuad     Ease = "outQuad"
	EasingOutCubic    Ease = "outCubic"
	EasingOutQuart    Ease = "outQuart"
	EasingOutQuint    Ease = "outQuint"
	EasingOutExpo     Ease = "outExpo"
	EasingOutCirc     Ease = "outCirc"
	EasingOutBack     Ease = "outBack"
	EasingOutElastic  Ease = "outElastic"
	EasingInOutSine   Ease = "inOutSine"
	EasingInOutQuad   Ease = "inOutQuad"
	EasingInOutCubic  Ease = "inOutCubic"
	EasingInOutQuart  Ease = "inOutQuart"
	EasingInOutQuint  Ease = "inOutQuint"
	EasingInOutExpo   Ease = "inOutExpo"
	EasingInOutCirc   Ease = "inOutCirc"
	EasingInOutBack   Ease = "inOutBack"
	EasingInOutElastic Ease = "inOutElastic"
	EasingOutInSine   Ease = "outInSine"
	EasingOutInQuad   Ease = "outInQuad"
	EasingOutInCubic  Ease = "outInCubic"
	EasingOutInQuart  Ease = "outInQuart"
	EasingOutInQuint  Ease = "outInQuint"
	EasingOutInExpo   Ease = "outInExpo"
	EasingOutInCirc   Ease = "outInCirc"
	EasingOutInBack   Ease = "outInBack"
	EasingOutInElastic Ease = "outInElastic"
)
