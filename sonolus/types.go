// Package sonolus declares the shared, compile-time surface of the Sonolus Go
// DSL. The declarations are recognized by the compiler; their ordinary Go
// implementations intentionally return zero values.
package sonolus

import "iter"

type Vec2 struct{ X, Y float64 }

func NewVec2(x, y float64) Vec2                       { return Vec2{X: x, Y: y} }
func UnitVec2(angle float64) Vec2                     { return Vec2{} }
func (v Vec2) Add(o Vec2) Vec2                        { return Vec2{} }
func (v Vec2) Sub(o Vec2) Vec2                        { return Vec2{} }
func (v Vec2) Mul(s float64) Vec2                     { return Vec2{} }
func (v Vec2) Div(s float64) Vec2                     { return Vec2{} }
func (v Vec2) MulVec(o Vec2) Vec2                     { return Vec2{} }
func (v Vec2) DivVec(o Vec2) Vec2                     { return Vec2{} }
func (v Vec2) Negate() Vec2                           { return Vec2{} }
func (v Vec2) Lerp(o Vec2, t float64) Vec2            { return Vec2{} }
func (v Vec2) LerpClamped(o Vec2, t float64) Vec2     { return Vec2{} }
func (v Vec2) Magnitude() float64                     { return 0 }
func (v Vec2) MagnitudeSquared() float64              { return 0 }
func (v Vec2) Dot(o Vec2) float64                     { return 0 }
func (v Vec2) Normalize() Vec2                        { return Vec2{} }
func (v Vec2) NormalizeOrZero() Vec2                  { return Vec2{} }
func (v Vec2) Angle() float64                         { return 0 }
func (v Vec2) Rotate(angle float64) Vec2              { return Vec2{} }
func (v Vec2) RotateAbout(p Vec2, angle float64) Vec2 { return Vec2{} }
func (v Vec2) Orthogonal() Vec2                       { return Vec2{} }
func (v Vec2) AngleDiff(o Vec2) float64               { return 0 }
func (v Vec2) SignedAngleDiff(o Vec2) float64         { return 0 }

type Quad struct{ BL, TL, TR, BR Vec2 }

func (q Quad) Center() Vec2                  { return Vec2{} }
func (q Quad) Translate(v Vec2) Quad         { return Quad{} }
func (q Quad) Scale(s float64) Quad          { return Quad{} }
func (q Quad) ScaleVec(v Vec2) Quad          { return Quad{} }
func (q Quad) ScaleAbout(v, pivot Vec2) Quad { return Quad{} }
func (q Quad) ScaleCentered(v Vec2) Quad     { return Quad{} }
func (q Quad) Rotate(angle float64) Quad     { return Quad{} }
func (q Quad) RotateAbout(angle float64, pivot Vec2) Quad {
	return Quad{}
}
func (q Quad) RotateCentered(angle float64) Quad { return Quad{} }
func (q Quad) Permute(rotation float64) Quad     { return Quad{} }
func (q Quad) Top() Vec2                         { return Vec2{} }
func (q Quad) Right() Vec2                       { return Vec2{} }
func (q Quad) Bottom() Vec2                      { return Vec2{} }
func (q Quad) Left() Vec2                        { return Vec2{} }
func (q Quad) Contains(p Vec2) bool              { return false }

type Rect struct{ T, R, B, L float64 }

func RectFromCenter(center, dimensions Vec2) Rect { return Rect{} }
func RectFromMargin(top, right, bottom, left float64) Rect {
	return Rect{}
}
func (r Rect) Width() float64        { return 0 }
func (r Rect) Height() float64       { return 0 }
func (r Rect) Center() Vec2          { return Vec2{} }
func (r Rect) BL() Vec2              { return Vec2{} }
func (r Rect) TL() Vec2              { return Vec2{} }
func (r Rect) TR() Vec2              { return Vec2{} }
func (r Rect) BR() Vec2              { return Vec2{} }
func (r Rect) Top() Vec2             { return Vec2{} }
func (r Rect) Right() Vec2           { return Vec2{} }
func (r Rect) Bottom() Vec2          { return Vec2{} }
func (r Rect) Left() Vec2            { return Vec2{} }
func (r Rect) ToQuad() Quad          { return Quad{} }
func (r Rect) Translate(v Vec2) Rect { return Rect{} }
func (r Rect) Scale(s float64) Rect  { return Rect{} }
func (r Rect) ScaleVec(v Vec2) Rect  { return Rect{} }
func (r Rect) ScaleAbout(v, pivot Vec2) Rect {
	return Rect{}
}
func (r Rect) ScaleCentered(v Vec2) Rect { return Rect{} }
func (r Rect) Expand(v Vec2) Rect        { return Rect{} }
func (r Rect) Shrink(v Vec2) Rect        { return Rect{} }
func (r Rect) Contains(p Vec2) bool      { return false }

type Transform2D struct {
	A00, A01, A02 float64
	A10, A11, A12 float64
	A20, A21, A22 float64
}

func IdentityTransform2D() Transform2D                         { return Transform2D{} }
func (t Transform2D) Translate(v Vec2) Transform2D             { return Transform2D{} }
func (t Transform2D) Scale(v Vec2) Transform2D                 { return Transform2D{} }
func (t Transform2D) Rotate(angle float64) Transform2D         { return Transform2D{} }
func (t Transform2D) ShearX(value float64) Transform2D         { return Transform2D{} }
func (t Transform2D) ShearY(value float64) Transform2D         { return Transform2D{} }
func (t Transform2D) SimplePerspectiveX(x float64) Transform2D { return Transform2D{} }
func (t Transform2D) SimplePerspectiveY(y float64) Transform2D { return Transform2D{} }
func (t Transform2D) PerspectiveX(foreground float64, vanishingPoint Vec2) Transform2D {
	return Transform2D{}
}
func (t Transform2D) PerspectiveY(foreground float64, vanishingPoint Vec2) Transform2D {
	return Transform2D{}
}
func (t Transform2D) InversePerspectiveX(foreground float64, vanishingPoint Vec2) Transform2D {
	return Transform2D{}
}
func (t Transform2D) InversePerspectiveY(foreground float64, vanishingPoint Vec2) Transform2D {
	return Transform2D{}
}
func (t Transform2D) Normalize() Transform2D                    { return Transform2D{} }
func (t Transform2D) Compose(o Transform2D) Transform2D         { return Transform2D{} }
func (t Transform2D) ComposeBefore(o Transform2D) Transform2D   { return Transform2D{} }
func (t Transform2D) ScaleAbout(v, pivot Vec2) Transform2D      { return Transform2D{} }
func (t Transform2D) RotateAbout(a float64, p Vec2) Transform2D { return Transform2D{} }
func (t Transform2D) TransformVec(v Vec2) Vec2                  { return Vec2{} }
func (t Transform2D) TransformQuad(q Quad) Quad                 { return Quad{} }
func (t Transform2D) TransformRect(r Rect) Quad                 { return Quad{} }

type InvertibleTransform2D struct {
	Forward Transform2D
	Inverse Transform2D
}

func IdentityInvertibleTransform2D() InvertibleTransform2D { return InvertibleTransform2D{} }
func (t InvertibleTransform2D) Translate(v Vec2) InvertibleTransform2D {
	return InvertibleTransform2D{}
}
func (t InvertibleTransform2D) Scale(v Vec2) InvertibleTransform2D {
	return InvertibleTransform2D{}
}
func (t InvertibleTransform2D) ScaleAbout(v, pivot Vec2) InvertibleTransform2D {
	return InvertibleTransform2D{}
}
func (t InvertibleTransform2D) Rotate(angle float64) InvertibleTransform2D {
	return InvertibleTransform2D{}
}
func (t InvertibleTransform2D) RotateAbout(angle float64, pivot Vec2) InvertibleTransform2D {
	return InvertibleTransform2D{}
}
func (t InvertibleTransform2D) ShearX(value float64) InvertibleTransform2D {
	return InvertibleTransform2D{}
}
func (t InvertibleTransform2D) ShearY(value float64) InvertibleTransform2D {
	return InvertibleTransform2D{}
}
func (t InvertibleTransform2D) SimplePerspectiveX(x float64) InvertibleTransform2D {
	return InvertibleTransform2D{}
}
func (t InvertibleTransform2D) SimplePerspectiveY(y float64) InvertibleTransform2D {
	return InvertibleTransform2D{}
}
func (t InvertibleTransform2D) PerspectiveX(foreground float64, vanishingPoint Vec2) InvertibleTransform2D {
	return InvertibleTransform2D{}
}
func (t InvertibleTransform2D) PerspectiveY(foreground float64, vanishingPoint Vec2) InvertibleTransform2D {
	return InvertibleTransform2D{}
}
func (t InvertibleTransform2D) Normalize() InvertibleTransform2D {
	return InvertibleTransform2D{}
}
func (t InvertibleTransform2D) Compose(o InvertibleTransform2D) InvertibleTransform2D {
	return InvertibleTransform2D{}
}
func (t InvertibleTransform2D) ComposeBefore(o InvertibleTransform2D) InvertibleTransform2D {
	return InvertibleTransform2D{}
}
func (t InvertibleTransform2D) TransformVec(v Vec2) Vec2 { return Vec2{} }
func (t InvertibleTransform2D) InverseTransformVec(v Vec2) Vec2 {
	return Vec2{}
}
func (t InvertibleTransform2D) TransformQuad(q Quad) Quad { return Quad{} }
func (t InvertibleTransform2D) TransformRect(r Rect) Quad { return Quad{} }
func (t InvertibleTransform2D) InverseTransformQuad(q Quad) Quad {
	return Quad{}
}
func (t InvertibleTransform2D) InverseTransformRect(r Rect) Quad {
	return Quad{}
}

func PerspectiveApproach(distanceRatio, progress float64) float64 { return 0 }

type Range struct{ Min, Max float64 }

func NewRange(min, max float64) Range               { return Range{Min: min, Max: max} }
func (r Range) Length() float64                     { return 0 }
func (r Range) IsEmpty() bool                       { return false }
func (r Range) Mid() float64                        { return 0 }
func (r Range) Contains(value float64) bool         { return false }
func (r Range) ContainsRange(other Range) bool      { return false }
func (r Range) Add(value float64) Range             { return Range{} }
func (r Range) Sub(value float64) Range             { return Range{} }
func (r Range) Mul(value float64) Range             { return Range{} }
func (r Range) Div(value float64) Range             { return Range{} }
func (r Range) Intersect(other Range) Range         { return Range{} }
func (r Range) Shrink(value float64) Range          { return Range{} }
func (r Range) Expand(value float64) Range          { return Range{} }
func (r Range) Lerp(value float64) float64          { return 0 }
func (r Range) LerpClamped(value float64) float64   { return 0 }
func (r Range) Unlerp(value float64) float64        { return 0 }
func (r Range) UnlerpClamped(value float64) float64 { return 0 }
func (r Range) Clamp(value float64) float64         { return 0 }

type JudgmentWindow struct{ Perfect, Great, Good Range }
type JudgmentWindows struct{ Perfect, Great, Good Range }

func (w JudgmentWindow) Judge(actual, target float64) Judgment  { return JudgmentMiss }
func (w JudgmentWindows) Judge(actual, target float64) Judgment { return JudgmentMiss }

type EntityRef[T any] struct{ Index float64 }

type AnyArchetype struct{}

func (r EntityRef[T]) Get() *T          { return nil }
func (r EntityRef[T]) GetUnchecked() *T { return nil }
func (r EntityRef[T]) Key() float64     { return -1 }

func EntityRefAs[T, U any](ref EntityRef[U]) EntityRef[T] { return EntityRef[T]{Index: ref.Index} }
func EntityRefMatches[T, U any](ref EntityRef[U], strict bool) bool {
	return false
}
func EntityRefGetAs[T, U any](ref EntityRef[U]) *T { return nil }

func Assert(condition bool, message string)       {}
func Require(condition bool, message string)      {}
func StaticAssert(condition bool, message string) {}
func RuntimeChecksEnabled() bool                  { return true }
func IsPlay() bool                                { return false }
func IsWatch() bool                               { return false }
func IsPreview() bool                             { return false }
func IsTutorial() bool                            { return false }
func IsPreprocessing() bool                       { return false }
func Unreachable(message string)                  { panic(message) }
func Terminate(message string)                    { panic(message) }
func Notify(message string)                       {}

func Zero[T any]() (value T) { return value }
func SlotsOf[T any]() int    { return 0 }

type StreamResource struct{}
type LevelMemoryResource struct{}
type LevelDataResource struct{}
type Stream[T any] struct{}
type StreamData[T any] struct{}

func (Stream[T]) Set(key float64, value T)        {}
func (Stream[T]) Has(key float64) bool            { return false }
func (Stream[T]) PreviousKey(key float64) float64 { return 0 }
func (Stream[T]) NextKey(key float64) float64     { return 0 }
func (Stream[T]) Get(key float64) (value T)       { return value }
func (Stream[T]) PreviousKeyOrDefault(key, fallback float64) float64 {
	return fallback
}
func (Stream[T]) NextKeyOrDefault(key, fallback float64) float64 { return fallback }
func (Stream[T]) HasPreviousKey(key float64) bool                { return false }
func (Stream[T]) HasNextKey(key float64) bool                    { return false }
func (Stream[T]) PreviousKeyInclusive(key float64) float64       { return key }
func (Stream[T]) NextKeyInclusive(key float64) float64           { return key }
func (Stream[T]) GetPrevious(key float64) (value T)              { return value }
func (Stream[T]) GetNext(key float64) (value T)                  { return value }
func (Stream[T]) GetPreviousInclusive(key float64) (value T)     { return value }
func (Stream[T]) GetNextInclusive(key float64) (value T)         { return value }
func (Stream[T]) ItemsFrom(start float64) iter.Seq2[float64, T]  { return nil }
func (Stream[T]) ItemsFromDescending(start float64) iter.Seq2[float64, T] {
	return nil
}
func (Stream[T]) ItemsSincePreviousFrame() iter.Seq2[float64, T] { return nil }
func (Stream[T]) KeysFrom(start float64) iter.Seq[float64]       { return nil }
func (Stream[T]) KeysFromDescending(start float64) iter.Seq[float64] {
	return nil
}
func (Stream[T]) KeysSincePreviousFrame() iter.Seq[float64] { return nil }
func (Stream[T]) ValuesFrom(start float64) iter.Seq[T]      { return nil }
func (Stream[T]) ValuesFromDescending(start float64) iter.Seq[T] {
	return nil
}
func (Stream[T]) ValuesSincePreviousFrame() iter.Seq[T] { return nil }

func (StreamData[T]) Set(value T)    {}
func (StreamData[T]) Get() (value T) { return value }

type Pair[A, B any] struct {
	First  A
	Second B
}

type Judgment int

const (
	JudgmentMiss Judgment = iota
	JudgmentPerfect
	JudgmentGreat
	JudgmentGood
)

type EntityState int

const (
	EntityStateWaiting EntityState = iota
	EntityStateActive
	EntityStateDespawned
)

type HapticType int
type TimescaleEase int

const (
	HapticNone HapticType = iota
	HapticLight
	HapticMedium
	HapticHeavy
	HapticLong
)

const (
	TimescaleEaseNone TimescaleEase = iota
	TimescaleEaseLinear
)

type RenderMode string
type PrintFormat int
type PrintColor int
type HorizontalAlign int

const (
	RenderModeDefault       RenderMode      = "default"
	RenderModeStandard      RenderMode      = "standard"
	RenderModeLightweight   RenderMode      = "lightweight"
	PrintFormatNumber       PrintFormat     = 0
	PrintFormatPercentage   PrintFormat     = 1
	PrintFormatTime         PrintFormat     = 10
	PrintFormatScore        PrintFormat     = 11
	PrintFormatBPM          PrintFormat     = 20
	PrintFormatTimeScale    PrintFormat     = 21
	PrintFormatBeatCount    PrintFormat     = 30
	PrintFormatMeasureCount PrintFormat     = 31
	PrintFormatEntityCount  PrintFormat     = 32
	PrintColorTheme         PrintColor      = -1
	PrintColorNeutral       PrintColor      = 0
	PrintColorRed           PrintColor      = 1
	PrintColorGreen         PrintColor      = 2
	PrintColorBlue          PrintColor      = 3
	PrintColorYellow        PrintColor      = 4
	PrintColorPurple        PrintColor      = 5
	PrintColorCyan          PrintColor      = 6
	HorizontalAlignLeft     HorizontalAlign = -1
	HorizontalAlignCenter   HorizontalAlign = 0
	HorizontalAlignRight    HorizontalAlign = 1
)

type StandardArchetypeName string

const (
	ArchetypeBPMChange       StandardArchetypeName = "#BPM_CHANGE"
	ArchetypeTimeScaleChange StandardArchetypeName = "#TIMESCALE_CHANGE"
	ArchetypeTimeScaleGroup  StandardArchetypeName = "#TIMESCALE_GROUP"
)

type StandardImportName string

const (
	ImportBeat           StandardImportName = "#BEAT"
	ImportBPM            StandardImportName = "#BPM"
	ImportTimeScale      StandardImportName = "#TIMESCALE"
	ImportTimeScaleSkip  StandardImportName = "#TIMESCALE_SKIP"
	ImportTimeScaleGroup StandardImportName = "#TIMESCALE_GROUP"
	ImportTimeScaleEase  StandardImportName = "#TIMESCALE_EASE"
	ImportJudgment       StandardImportName = "#JUDGMENT"
	ImportAccuracy       StandardImportName = "#ACCURACY"
)
