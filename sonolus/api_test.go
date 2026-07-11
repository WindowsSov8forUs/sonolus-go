package sonolus_test

import (
	"testing"

	"github.com/WindowsSov8forUs/sonolus-go/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/sonolus/play"
	"github.com/WindowsSov8forUs/sonolus-go/sonolus/preview"
	"github.com/WindowsSov8forUs/sonolus-go/sonolus/tutorial"
	"github.com/WindowsSov8forUs/sonolus-go/sonolus/watch"
)

func TestPublicSurfaceCompiles(t *testing.T) {
	q := sonolus.Rect{}.ToQuad().Translate(sonolus.NewVec2(1, 2))
	sonolus.SkinSprite("test").Draw(q, 0, 1)
	_ = sonolus.EffectClip("test")
	_ = sonolus.ParticleEffect("test")
	_ = sonolus.InstructionText("test")
	_ = sonolus.InstructionIcon("test")
	_ = sonolus.VarArray[sonolus.Vec2]{}.Len()
	_ = play.Time.Now()
	_ = play.Screen.Rect()
	_ = watch.Replay.IsReplay()
	_ = preview.Canvas.Size()
	_ = tutorial.Navigation.Direction()
}
