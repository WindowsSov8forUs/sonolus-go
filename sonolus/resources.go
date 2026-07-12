package sonolus

// Marker types are embedded in user declarations and recognized by go/types.
type Configuration struct{}

type SkinResource struct {
	RenderMode RenderMode
}

type EffectResource struct{}
type ParticleResource struct{}
type BucketsResource struct{}
type InstructionResource struct{}
type InstructionIconResource struct{}

type SliderOptionConfig struct {
	Name, Title, Description string
	Standard, Advanced       bool
	Scope                    string
	Default, Min, Max, Step  float64
	Unit                     string
}

type ToggleOptionConfig struct {
	Name, Title, Description string
	Standard, Advanced       bool
	Scope                    string
	Default                  bool
}

type SelectOptionConfig struct {
	Name, Title, Description string
	Standard, Advanced       bool
	Scope                    string
	Default                  int
	Values                   []string
}

func SliderOption(config SliderOptionConfig) float64 { return config.Default }
func ToggleOption(config ToggleOptionConfig) bool    { return config.Default }
func SelectOption(config SelectOptionConfig) int     { return config.Default }

type ROMValues []float32
type ROMFile []byte
type LevelFile []byte

// Resource handles deliberately have no exported representation. Engine
// source can obtain them only through the declaration constructors below; the
// compiler assigns their runtime IDs from declaration order.
type Sprite struct{}

func SkinSprite(name string) Sprite { return Sprite{} }

func (s Sprite) Draw(q Quad, z, opacity float64)                                            {}
func (s Sprite) DrawCurvedB(q Quad, control Vec2, segments, z, opacity float64)             {}
func (s Sprite) DrawCurvedT(q Quad, control Vec2, segments, z, opacity float64)             {}
func (s Sprite) DrawCurvedL(q Quad, control Vec2, segments, z, opacity float64)             {}
func (s Sprite) DrawCurvedR(q Quad, control Vec2, segments, z, opacity float64)             {}
func (s Sprite) DrawCurvedBT(q Quad, controlB, controlT Vec2, segments, z, opacity float64) {}
func (s Sprite) DrawCurvedLR(q Quad, controlL, controlR Vec2, segments, z, opacity float64) {}
func (s Sprite) Exists() bool                                                               { return false }

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

func (p Effect) Spawn(q Quad, duration float64, loop bool) ParticleHandle { return ParticleHandle{} }
func (h ParticleHandle) Move(q Quad)                                      {}
func (h ParticleHandle) Destroy()                                         {}

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

type Text struct{}
type Icon struct{}

func InstructionText(name string) Text { return Text{} }
func InstructionIcon(name string) Icon { return Icon{} }

type UIConfig struct {
	Scope                                               string
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

// RuntimeUILayout is the mutable layout of a Play or Watch UI element.
type RuntimeUILayout struct {
	Anchor, Pivot, Size Vec2
	Rotation, Alpha     float64
	HorizontalAlign     HorizontalAlign
	Background          bool
}

// RuntimeUIBasicLayout is the mutable layout used by Preview and Tutorial.
type RuntimeUIBasicLayout struct {
	Anchor, Pivot, Size Vec2
	Rotation, Alpha     float64
	Background          bool
}

// RuntimeUIConfiguration contains the user's read-only UI scale and opacity.
type RuntimeUIConfiguration struct{ Scale, Alpha float64 }
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
