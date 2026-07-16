package sonolus_test

import (
	"testing"

	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/play"
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/preview"
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/tutorial"
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/watch"
)

func TestPublicSurfaceCompiles(t *testing.T) {
	q := sonolus.Rect{}.ToQuad().Translate(sonolus.NewVec2(1, 2))
	_ = sonolus.RectFromCenter(sonolus.NewVec2(0, 0), sonolus.NewVec2(1, 1))
	_ = sonolus.RectFromMargin(1, 1, 1, 1)
	_ = q.ScaleVec(sonolus.NewVec2(1, 1)).ScaleCentered(sonolus.NewVec2(1, 1)).RotateCentered(0)
	transform := sonolus.IdentityTransform2D().PerspectiveY(-0.5, sonolus.NewVec2(0, 1.35))
	_ = transform.TransformQuad(q)
	_ = transform.TransformRect(sonolus.Rect{})
	_ = sonolus.IdentityInvertibleTransform2D().Translate(sonolus.NewVec2(1, 2)).InverseTransformVec(sonolus.NewVec2(1, 2))
	_ = sonolus.UnitVec2(0).Lerp(sonolus.NewVec2(1, 1), 0.5)
	_ = sonolus.UnitVec2(0).LerpClamped(sonolus.NewVec2(1, 1), 0.5)
	_ = sonolus.UnitVec2(0).MulVec(sonolus.NewVec2(1, 1)).DivVec(sonolus.NewVec2(1, 1)).Negate()
	_, _, _, _ = sonolus.Rect{}.BL(), sonolus.Rect{}.TR(), sonolus.Rect{}.Top(), sonolus.Rect{}.Left()
	_ = sonolus.PerspectiveApproach(2, 0.5)
	_, _, _, _, _ = sonolus.ArchetypeTimeScaleGroup, sonolus.ImportTimeScaleSkip, sonolus.ImportTimeScaleGroup, sonolus.ImportTimeScaleEase, sonolus.TimescaleEaseLinear
	var bucket sonolus.Bucket
	bucket.SetWindow(sonolus.JudgmentWindows{Perfect: sonolus.NewRange(-50, 50)})
	_ = bucket.Window()
	sonolus.SkinSprite("test").Draw(q, 0, 1)
	_ = sonolus.EffectClip("test")
	_ = sonolus.ParticleEffect("test")
	_ = sonolus.InstructionText("test")
	_ = sonolus.InstructionIcon("test")
	_ = sonolus.VarArray[sonolus.Vec2](nil).Len()
	_ = sonolus.NewVarArray[sonolus.Vec2](8)
	_ = sonolus.NewArrayMap[int, sonolus.Vec2](8)
	_ = sonolus.NewArraySet[int](8)
	_ = (sonolus.EntityRef[struct{}]{}).Get()
	_ = (sonolus.EntityRef[struct{}]{}).Key()
	_ = play.CurrentEntityRef[struct{}]()
	_ = watch.CurrentEntityRef[struct{}]()
	_ = preview.CurrentEntityRef[struct{}]()
	_ = play.ArchetypeID[struct{}]()
	_ = watch.ArchetypeID[struct{}]()
	_ = preview.ArchetypeID[struct{}]()
	_ = play.ArchetypeKey[struct{}]()
	_ = watch.ArchetypeKey[struct{}]()
	_ = preview.ArchetypeKey[struct{}]()
	interval := sonolus.NewRange(-1, 1)
	_ = interval.Length()
	_ = interval.IsEmpty()
	_ = interval.Mid()
	_ = interval.Contains(0)
	_ = interval.ContainsRange(interval)
	_ = interval.Add(1)
	_ = interval.Sub(1)
	_ = interval.Mul(2)
	_ = interval.Div(2)
	_ = interval.Intersect(interval)
	_ = interval.Shrink(1)
	_ = interval.Expand(1)
	_ = interval.Lerp(0.5)
	_ = interval.LerpClamped(0.5)
	_ = interval.Unlerp(0)
	_ = interval.UnlerpClamped(0)
	_ = interval.Clamp(0)
	_ = sonolus.Ease(sonolus.EaseOutIn, sonolus.EaseQuad, 0.5)
	_ = sonolus.Linstep(0.5)
	_ = sonolus.Smoothstep(0.5)
	_ = sonolus.Smootherstep(0.5)
	_ = sonolus.StepStart(0.5)
	_ = sonolus.StepEnd(0.5)
	_ = sonolus.UnlerpClamped(0, 1, 0.5)
	_ = sonolus.RemapClamped(0, 1, -1, 1, 0.5)
	_, _, _, _, _ = sonolus.IsPlay(), sonolus.IsWatch(), sonolus.IsPreview(), sonolus.IsTutorial(), sonolus.IsPreprocessing()
	_ = play.Time.Now()
	_, _, _, _ = play.Time.BeatToBPM(0), play.Time.BeatToTime(0), play.Time.BeatToStartingBeat(0), play.Time.BeatToStartingTime(0)
	_, _, _, _ = play.Time.TimeToScaledTime(0), play.Time.TimeToStartingScaledTime(0), play.Time.TimeToStartingTime(0), play.Time.TimeToTimeScale(0)
	_ = play.Entity.Key()
	_ = play.Touches.Values()
	_ = play.Touches.Items()
	_ = play.LevelMemory.Get(0)
	play.LevelMemory.Set(0, 1)
	_ = play.LevelData.Get(0)
	play.LevelData.Set(0, 1)
	_ = play.Screen.Rect()
	_ = watch.Replay.IsReplay()
	_ = watch.Entity.Key()
	_ = watch.LevelMemory.Get(0)
	watch.LevelMemory.Set(0, 1)
	_ = watch.LevelData.Get(0)
	watch.LevelData.Set(0, 1)
	_, _, _, _ = watch.Time.BeatToBPM(0), watch.Time.BeatToTime(0), watch.Time.BeatToStartingBeat(0), watch.Time.BeatToStartingTime(0)
	_, _, _, _ = watch.Time.TimeToScaledTime(0), watch.Time.TimeToStartingScaledTime(0), watch.Time.TimeToStartingTime(0), watch.Time.TimeToTimeScale(0)
	watch.SkinTransform.Set(watch.SkinTransform.Get())
	watch.ParticleTransform.Set(watch.ParticleTransform.Get())
	_ = preview.Canvas.Size()
	_ = preview.Entity.Key()
	_, _, _, _ = preview.Time.BeatToBPM(0), preview.Time.BeatToTime(0), preview.Time.BeatToStartingBeat(0), preview.Time.BeatToStartingTime(0)
	_, _, _, _ = preview.Time.TimeToScaledTime(0), preview.Time.TimeToStartingScaledTime(0), preview.Time.TimeToStartingTime(0), preview.Time.TimeToTimeScale(0)
	_ = preview.LevelData.Get(0)
	preview.LevelData.Set(0, 1)
	preview.SkinTransform.Set(preview.SkinTransform.Get())
	_ = tutorial.Navigation.Direction()
	_, _, _, _ = tutorial.Time.BeatToBPM(0), tutorial.Time.BeatToTime(0), tutorial.Time.BeatToStartingBeat(0), tutorial.Time.BeatToStartingTime(0)
	_, _, _, _ = tutorial.Time.TimeToScaledTime(0), tutorial.Time.TimeToStartingScaledTime(0), tutorial.Time.TimeToStartingTime(0), tutorial.Time.TimeToTimeScale(0)
	_ = tutorial.Audio.Offset()
	_ = tutorial.LevelMemory.Get(0)
	tutorial.LevelMemory.Set(0, 1)
	_ = tutorial.LevelData.Get(0)
	tutorial.LevelData.Set(0, 1)
	tutorial.SkinTransform.Set(tutorial.SkinTransform.Get())
	tutorial.ParticleTransform.Set(tutorial.ParticleTransform.Get())
	_ = sonolus.LevelMemoryResource{}
	_ = sonolus.LevelDataResource{}
}

func TestStaticDebugUtilitiesHaveOrdinaryGoBehavior(t *testing.T) {
	if !sonolus.RuntimeChecksEnabled() {
		t.Fatal("RuntimeChecksEnabled must be conservative outside the compiler")
	}
	panicked := false
	func() {
		defer func() { panicked = recover() == "stop" }()
		sonolus.Unreachable("stop")
	}()
	if !panicked {
		t.Fatal("Unreachable did not panic during ordinary Go execution")
	}
	sonolus.Notify("notice")
	panicked = false
	func() {
		defer func() { panicked = recover() == "terminate" }()
		sonolus.Terminate("terminate")
	}()
	if !panicked {
		t.Fatal("Terminate did not panic during ordinary Go execution")
	}
}
