package engine_test
import ("testing"; "github.com/WindowsSov8forUs/sonolus-go/internal/compiler/engine"; "github.com/WindowsSov8forUs/sonolus-go/internal/compiler/ir/optimize")

func TestSkinTransformAPI(t *testing.T) {
	src := "package test\nimport \"github.com/WindowsSov8forUs/sonolus-go/sonolus\"\n" +
		"type Skin struct { Note float64 }\n" +
		"type Note struct { Beat float64 `sonolus:\"imported\"` }\n" +
		"func (n *Note) Initialize() {\n" +
		"    t := sonolus.SkinTransform()\n    t.A00 = 1\n    sonolus.SetSkinTransform(t)\n}\n" +
		"func UpdateSpawn() float64 { return 0 }\n"
	if _, _, err := engine.CompilePlaySources(engine.NewSingleFileSources(src), &engine.CompileOptions{Opt: optimize.LevelStandard}); err != nil {
		t.Fatalf("SkinTransform: %v", err)
	}
}

func TestBackgroundAPI(t *testing.T) {
	src := "package test\nimport \"github.com/WindowsSov8forUs/sonolus-go/sonolus\"\n" +
		"type Skin struct { Note float64 }\n" +
		"type Note struct { Beat float64 `sonolus:\"imported\"` }\n" +
		"func (n *Note) Initialize() {\n" +
		"    t := sonolus.Background()\n    t.A00 = 1\n    sonolus.SetBackground(t)\n}\n" +
		"func UpdateSpawn() float64 { return 0 }\n"
	if _, _, err := engine.CompilePlaySources(engine.NewSingleFileSources(src), &engine.CompileOptions{Opt: optimize.LevelStandard}); err != nil {
		t.Fatalf("Background: %v", err)
	}
}
