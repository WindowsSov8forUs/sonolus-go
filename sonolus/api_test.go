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
	_ = play.Time.Now()
	_ = play.LevelMemory.Get(0)
	play.LevelMemory.Set(0, 1)
	_ = play.Screen.Rect()
	_ = watch.Replay.IsReplay()
	_ = preview.Canvas.Size()
	_ = preview.LevelData.Get(0)
	preview.LevelData.Set(0, 1)
	_ = tutorial.Navigation.Direction()
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
