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
	_ = play.Time.Now()
	_ = play.Screen.Rect()
	_ = watch.Replay.IsReplay()
	_ = preview.Canvas.Size()
	_ = tutorial.Navigation.Direction()
}
