package sonolus

// Marker types are embedded in user declarations and recognized by go/types.
type Configuration struct{}
type ROMValues []float32
type ROMFile []byte

// Resource handles deliberately have no exported representation. Engine
// source can obtain them only through the declaration constructors below; the
// compiler assigns their runtime IDs from declaration order.
type Sprite struct{}

func SkinSprite(name string) Sprite { return Sprite{} }

func (s Sprite) Draw(q Quad, z, opacity float64)       {}
func (s Sprite) DrawCurved(q Quad, z, opacity float64) {}
func (s Sprite) Exists() bool                          { return false }

type Clip struct{}

func EffectClip(name string) Clip { return Clip{} }

type LoopedEffectHandle struct{ ID float64 }
type ScheduledLoopedEffectHandle struct{ ID float64 }

func (c Clip) Play(distance float64)              {}
func (c Clip) PlayScheduled(at, distance float64) {}
func (c Clip) PlayLooped() LoopedEffectHandle     { return LoopedEffectHandle{} }
func (c Clip) PlayLoopedScheduled(at float64) ScheduledLoopedEffectHandle {
	return ScheduledLoopedEffectHandle{}
}
func (h LoopedEffectHandle) Stop()                    {}
func (h ScheduledLoopedEffectHandle) Stop(at float64) {}

type Effect struct{}

func ParticleEffect(name string) Effect { return Effect{} }

type ParticleHandle struct{ ID float64 }

func (p Effect) Spawn(q Quad, duration, loop bool) ParticleHandle { return ParticleHandle{} }
func (h ParticleHandle) Move(q Quad)                              {}
func (h ParticleHandle) Destroy()                                 {}

type Bucket struct{}
type BucketSprite struct {
	Sprite, Fallback     Sprite
	HasFallback          bool
	X, Y, W, H, Rotation float64
}

func JudgmentBucket(unit string, sprites ...BucketSprite) Bucket { return Bucket{} }
func JudgmentBucketSprite(sprite Sprite, x, y, w, h, rotation float64) BucketSprite {
	return BucketSprite{}
}
func JudgmentBucketSpriteWithFallback(sprite, fallback Sprite, x, y, w, h, rotation float64) BucketSprite {
	return BucketSprite{}
}

func (b Bucket) Judge(actual, target float64, windows JudgmentWindows) Judgment { return JudgmentMiss }

type Text struct{}
type Icon struct{}

func InstructionText(name string) Text { return Text{} }
func InstructionIcon(name string) Icon { return Icon{} }

type UIConfig struct {
	PrimaryMetric, SecondaryMetric                      UIMetric
	MenuVisibility, JudgmentVisibility, ComboVisibility UIVisibility
	PrimaryMetricVisibility, SecondaryMetricVisibility  UIVisibility
	ProgressVisibility, TutorialNavigationVisibility    UIVisibility
	TutorialInstructionVisibility                       UIVisibility
	JudgmentAnimation, ComboAnimation                   UIAnimation
	JudgmentErrorStyle                                  UIJudgmentErrorStyle
	JudgmentErrorPlacement                              UIJudgmentErrorPlacement
	JudgmentErrorMin                                    float64
}
type UIMetric string
type UIJudgmentErrorStyle string
type UIJudgmentErrorPlacement string
type UIEase string
type UIVisibility struct{ Scale, Alpha float64 }
type UITween struct {
	From, To, Duration float64
	Ease               UIEase
}
type UIAnimation struct{ Scale, Alpha UITween }

const (
	UIMetricArcade        UIMetric                 = "arcade"
	UIMetricAccuracy      UIMetric                 = "accuracy"
	UIMetricLife          UIMetric                 = "life"
	UIEaseNone            UIEase                   = "none"
	UIEaseInSine          UIEase                   = "inSine"
	UIEaseOutSine         UIEase                   = "outSine"
	UIEaseInOutSine       UIEase                   = "inOutSine"
	UIJudgmentErrorNone   UIJudgmentErrorStyle     = "none"
	UIJudgmentErrorPlus   UIJudgmentErrorStyle     = "plus"
	UIJudgmentErrorMinus  UIJudgmentErrorStyle     = "minus"
	UIJudgmentErrorArrow  UIJudgmentErrorStyle     = "arrow"
	UIJudgmentErrorTop    UIJudgmentErrorPlacement = "top"
	UIJudgmentErrorBottom UIJudgmentErrorPlacement = "bottom"
)
