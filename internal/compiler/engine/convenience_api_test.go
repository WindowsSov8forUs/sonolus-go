package engine_test

import (
	"testing"

	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/engine"
	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/ir/optimize"
)

func TestSkinTransformAPI(t *testing.T) {
	src := `package test
import "github.com/WindowsSov8forUs/sonolus-go/sonolus"
type Skin struct { Note float64 }
type Note struct { Beat float64 ` + "`sonolus:\"imported\"`" + ` }
func (n *Note) Initialize() {
    sonolus.SetSkinTransformAt(0, 1)
    x := sonolus.SkinTransformAt(0)
    sonolus.Set(0, 0, x)
}
func UpdateSpawn() float64 { return 0 }
`
	ess := engine.NewSingleFileSources(src)
	_, _, err := engine.CompilePlaySources(ess, &engine.CompileOptions{
		Opt: optimize.LevelStandard,
	})
	if err != nil {
		t.Fatalf("SkinTransform: %v", err)
	}
	t.Log("OK")
}

func TestParticleTransformAPI(t *testing.T) {
	src := `package test
import "github.com/WindowsSov8forUs/sonolus-go/sonolus"
type Skin struct { Note float64 }
type Note struct { Beat float64 ` + "`sonolus:\"imported\"`" + ` }
func (n *Note) Initialize() {
    sonolus.SetParticleTransformAt(0, 1)
    x := sonolus.ParticleTransformAt(0)
    sonolus.Set(0, 0, x)
}
func UpdateSpawn() float64 { return 0 }
`
	ess := engine.NewSingleFileSources(src)
	_, _, err := engine.CompilePlaySources(ess, &engine.CompileOptions{
		Opt: optimize.LevelStandard,
	})
	if err != nil {
		t.Fatalf("ParticleTransform: %v", err)
	}
	t.Log("OK")
}

func TestBackgroundAPI(t *testing.T) {
	src := `package test
import "github.com/WindowsSov8forUs/sonolus-go/sonolus"
type Skin struct { Note float64 }
type Note struct { Beat float64 ` + "`sonolus:\"imported\"`" + ` }
func (n *Note) Initialize() {
    sonolus.SetBackgroundAt(0, 1)
    x := sonolus.BackgroundAt(0)
    sonolus.Set(0, 0, x)
}
func UpdateSpawn() float64 { return 0 }
`
	ess := engine.NewSingleFileSources(src)
	_, _, err := engine.CompilePlaySources(ess, &engine.CompileOptions{
		Opt: optimize.LevelStandard,
	})
	if err != nil {
		t.Fatalf("Background: %v", err)
	}
	t.Log("OK")
}

func TestLevelScoreLifeAPI(t *testing.T) {
	// LevelScore/LevelLife are writable in preprocess.
	src := `package test
import "github.com/WindowsSov8forUs/sonolus-go/sonolus"
type Skin struct { Note float64 }
type Note struct { Beat float64 ` + "`sonolus:\"imported\"`" + ` }
func Preprocess() {
    sonolus.SetLevelScore(0, 1000000)
    sonolus.SetLevelLife(6, 20)
    x := sonolus.LevelScore(0)
    y := sonolus.LevelLife(6)
    sonolus.Set(0, 0, x + y)
}
func UpdateSpawn() float64 { return 0 }
`
	ess := engine.NewSingleFileSources(src)
	_, _, err := engine.CompilePlaySources(ess, &engine.CompileOptions{
		Opt: optimize.LevelStandard,
	})
	if err != nil {
		t.Fatalf("LevelScore/Life: %v", err)
	}
	t.Log("OK")
}

func TestConvenienceAPIInHelper(t *testing.T) {
	// Convenience APIs should work in helper functions too.
	src := `package test
import "github.com/WindowsSov8forUs/sonolus-go/sonolus"
type Skin struct { Note float64 }
type Note struct {
    Beat float64 ` + "`sonolus:\"imported\"`" + `
    X    float64 ` + "`sonolus:\"memory\"`" + `
}
func helper() {
    sonolus.SetSkinTransformAt(0, 1)
    sonolus.SetParticleTransformAt(1, 2)
}
func (n *Note) Initialize() {
    helper()
    n.X = sonolus.SkinTransformAt(0)
}
func UpdateSpawn() float64 { return 0 }
`
	ess := engine.NewSingleFileSources(src)
	_, _, err := engine.CompilePlaySources(ess, &engine.CompileOptions{
		Opt: optimize.LevelStandard,
	})
	if err != nil {
		t.Fatalf("Helper: %v", err)
	}
	t.Log("OK")
}
