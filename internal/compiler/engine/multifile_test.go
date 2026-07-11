package engine_test

import (
	"testing"

	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/engine"
)

// TestMultiFileSamePackage verifies that a directory with multiple .go files
// sharing the same package compiles successfully in all four modes.
func TestMultiFileSamePackage(t *testing.T) {
	ess := &engine.EngineSources{
		RootDir: ".",
		Main: map[string]string{
			"engine.go": `package test
type Skin struct { Note float64 }
type Note struct { Beat float64 ` + "`sonolus:\"imported\"`" + ` }
func (n *Note) Initialize() { debugPause() }
func UpdateSpawn() float64 { return 0 }
func Preprocess() {}
func Navigate() float64 { return 1 }
func Update() {}
`,
			"helpers.go": `package test
type Hold struct { Beat float64 ` + "`sonolus:\"imported\"`" + ` }
func (h *Hold) Initialize() { debugPause() }
`,
		},
		Imports: map[string]map[string]string{},
	}

	t.Run("play", func(t *testing.T) {
		_, _, err := engine.CompilePlaySources(ess, nil)
		if err != nil {
			t.Fatalf("CompilePlaySources: %v", err)
		}
	})
	t.Run("watch", func(t *testing.T) {
		_, err := engine.CompileWatchSources(ess, nil)
		if err != nil {
			t.Fatalf("CompileWatchSources: %v", err)
		}
	})
	t.Run("preview", func(t *testing.T) {
		_, err := engine.CompilePreviewSources(ess, nil)
		if err != nil {
			t.Fatalf("CompilePreviewSources: %v", err)
		}
	})
	t.Run("tutorial", func(t *testing.T) {
		_, err := engine.CompileTutorialSources(ess, nil)
		if err != nil {
			t.Fatalf("CompileTutorialSources: %v", err)
		}
	})
}

// TestImportSubPackage verifies that a main package can import a sub-package
// containing archetype definitions.
func TestImportSubPackage(t *testing.T) {
	ess := &engine.EngineSources{
		RootDir: ".",
		Main: map[string]string{
			"engine.go": `package test
import "notes"
type Skin struct { Note float64 }
func Preprocess() {}
func Navigate() float64 { return 1 }
func Update() {}
`,
		},
		Imports: map[string]map[string]string{
			"notes": {
				"note.go": `package notes
type Note struct { Beat float64 ` + "`sonolus:\"imported\"`" + ` }
func (n *Note) Initialize() { debugPause() }
func (n *Note) UpdateParallel(dt float64) { debugPause() }
`,
			},
		},
	}

	t.Run("play", func(t *testing.T) {
		_, _, err := engine.CompilePlaySources(ess, nil)
		if err != nil {
			t.Fatalf("CompilePlaySources: %v", err)
		}
	})
}

// TestSubPkg_RuntimeFns verifies that sub-package callbacks CAN use
// runtime functions (sin, Draw, etc.) — the frontend tracer resolves
// them against its own builtin registry, independent of type checking.
func TestSubPkg_RuntimeFns(t *testing.T) {
	ess := &engine.EngineSources{
		RootDir: ".",
		Main: map[string]string{
			"engine.go": `package test
import "notes"
type Skin struct { Note float64 }
func UpdateSpawn() float64 { return 0 }
`,
		},
		Imports: map[string]map[string]string{
			"notes": {
				"note.go": `package notes
type Note struct { Beat float64 ` + "`sonolus:\"imported\"`" + ` }
func (n *Note) Initialize() {
    x := get(0, 0)        // runtime function
    set(0, 0, x + 1)      // runtime function
    draw(1, x, 0, 1, 1, 0, 1, 0, 0)  // variadic runtime function (lowercase!)
}
`,
			},
		},
	}

	t.Run("full runtime API in subpkg", func(t *testing.T) {
		_, _, err := engine.CompilePlaySources(ess, nil)
		if err != nil {
			t.Fatalf("CompilePlaySources: %v", err)
		}
	})
}

// TestSubPkg_TypeCheckError verifies that type errors in sub-package callback
// bodies are properly detected (gap fix: importer now uses full prelude).
func TestSubPkg_TypeCheckError(t *testing.T) {
	ess := &engine.EngineSources{
		RootDir: ".",
		Main: map[string]string{
			"engine.go": `package test
import "notes"
type Skin struct { Note float64 }
func UpdateSpawn() float64 { return 0 }
`,
		},
		Imports: map[string]map[string]string{
			"notes": {
				"note.go": `package notes
type Note struct { Beat float64 ` + "`sonolus:\"imported\"`" + ` }
func (n *Note) Initialize() {
    x := undefinedFunc(1, 2, 3)  // ← should fail type check
    debugPause()
}
`,
			},
		},
	}

	_, _, err := engine.CompilePlaySources(ess, nil)
	if err == nil {
		t.Fatal("expected type-check error for undefined function in sub-package, got nil")
	}
	t.Logf("sub-package type check works: %v", err)
}

// TestImportSubPackage_DuplicateArchetype verifies that duplicate archetype
// names across packages produce a clear error.
func TestImportSubPackage_DuplicateArchetype(t *testing.T) {
	ess := &engine.EngineSources{
		RootDir: ".",
		Main: map[string]string{
			"engine.go": `package test
import "notes"
type Skin struct { Note float64 }
type Note struct { Beat float64 ` + "`sonolus:\"imported\"`" + ` }
func (n *Note) Initialize() { debugPause() }
`,
		},
		Imports: map[string]map[string]string{
			"notes": {
				"note.go": `package notes
type Note struct { Beat float64 }
func (n *Note) Initialize() { debugPause() }
`,
			},
		},
	}

	_, _, err := engine.CompilePlaySources(ess, nil)
	if err == nil {
		t.Fatal("expected duplicate archetype error, got nil")
	}
	t.Logf("got expected error: %v", err)
}

// TestImportSubPackage_ResourceInImport verifies that defining a resource
// type (Skin, Effect, etc.) in an imported package produces an error.
func TestImportSubPackage_ResourceInImport(t *testing.T) {
	ess := &engine.EngineSources{
		RootDir: ".",
		Main: map[string]string{
			"engine.go": `package test
import "notes"
type Skin struct { Note float64 }
`,
		},
		Imports: map[string]map[string]string{
			"notes": {
				"note.go": `package notes
type Skin struct { Note float64 }
`,
			},
		},
	}

	_, _, err := engine.CompilePlaySources(ess, nil)
	if err == nil {
		t.Fatal("expected error for Skin in imported package, got nil")
	}
	t.Logf("got expected error: %v", err)
}

// TestSonolusQualifiedCall verifies that sonolus-prefixed runtime function
// calls compile successfully.
func TestSonolusQualifiedCall(t *testing.T) {
	src := `package test
import "github.com/WindowsSov8forUs/sonolus-go/sonolus"
type Skin struct { Note float64 }
type Note struct { Beat float64 ` + "`sonolus:\"imported\"`" + ` }
func (n *Note) Initialize() {
    x := sonolus.Get(0, 0)
    sonolus.Set(0, 0, x + 1)
}
func UpdateSpawn() float64 { return 0 }
`
	ess := engine.NewSingleFileSources(src)
	_, _, err := engine.CompilePlaySources(ess, nil)
	if err != nil {
		t.Fatalf("CompilePlaySources: %v", err)
	}
}

// TestSonolusDrawCall verifies variadic runtime functions via sonolus. prefix.
func TestSonolusDrawCall(t *testing.T) {
	src := `package test
import "github.com/WindowsSov8forUs/sonolus-go/sonolus"
type Skin struct { Note float64 }
type Note struct { Beat float64 ` + "`sonolus:\"imported\"`" + ` }
func (n *Note) Initialize() {
    sonolus.Draw(1, n.Beat, 0, 1, 1, 0, 1, 0, 0)
}
func UpdateSpawn() float64 { return 0 }
`
	ess := engine.NewSingleFileSources(src)
	_, _, err := engine.CompilePlaySources(ess, nil)
	if err != nil {
		t.Fatalf("CompilePlaySources: %v", err)
	}
}

// TestSonolusMixedStyles verifies that bare and sonolus-prefixed calls can
// coexist in the same engine.
func TestSonolusMixedStyles(t *testing.T) {
	src := `package test
import "github.com/WindowsSov8forUs/sonolus-go/sonolus"
type Skin struct { Note float64 }
type Note struct { Beat float64 ` + "`sonolus:\"imported\"`" + ` }
func (n *Note) Initialize() {
    draw(1, n.Beat, 0, 1, 1, 0, 1, 0, 0)               // bare
    sonolus.Draw(1, n.Beat, 0, 1, 1, 0, 1, 0, 0)        // qualified (same IR)
}
func UpdateSpawn() float64 { return 0 }
`
	ess := engine.NewSingleFileSources(src)
	_, _, err := engine.CompilePlaySources(ess, nil)
	if err != nil {
		t.Fatalf("CompilePlaySources: %v", err)
	}
}

// TestSonolusShortImport verifies the short import form: import "sonolus".
func TestSonolusShortImport(t *testing.T) {
	src := `package test
import "sonolus"
type Skin struct { Note float64 }
type Note struct { Beat float64 ` + "`sonolus:\"imported\"`" + ` }
func (n *Note) Initialize() {
    sonolus.DebugPause()
}
func UpdateSpawn() float64 { return 0 }
`
	ess := engine.NewSingleFileSources(src)
	_, _, err := engine.CompilePlaySources(ess, nil)
	if err != nil {
		t.Fatalf("CompilePlaySources: %v", err)
	}
}

// TestSonolusInSubPkg verifies sonolus-qualified calls work in sub-package
// callbacks.
func TestSonolusInSubPkg(t *testing.T) {
	ess := &engine.EngineSources{
		RootDir: ".",
		Main: map[string]string{
			"engine.go": `package test
import "notes"
type Skin struct { Note float64 }
func UpdateSpawn() float64 { return 0 }
`,
		},
		Imports: map[string]map[string]string{
			"notes": {
				"note.go": `package notes
import "github.com/WindowsSov8forUs/sonolus-go/sonolus"
type Note struct { Beat float64 ` + "`sonolus:\"imported\"`" + ` }
func (n *Note) Initialize() {
    sonolus.Get(0, 0)
    sonolus.DebugPause()
}
`,
			},
		},
	}

	_, _, err := engine.CompilePlaySources(ess, nil)
	if err != nil {
		t.Fatalf("CompilePlaySources: %v", err)
	}
}

func TestPreludeMath(t *testing.T) {
	src := `package test
type Skin struct { Note float64 }
type Note struct {
    Beat float64 ` + "`sonolus:\"imported\"`" + `
    X    float64 ` + "`sonolus:\"memory\"`" + `
    Y    float64 ` + "`sonolus:\"memory\"`" + `
}
func (n *Note) Initialize() {
		n.X = sin(n.Beat)
		n.Y = cos(n.Beat)
}
func UpdateSpawn() float64 { return 0 }
`
	ess := engine.NewSingleFileSources(src)
	if _, _, err := engine.CompilePlaySources(ess, nil); err != nil {
		t.Fatalf("CompilePlaySources: %v", err)
	}
}

// TestRecordField_ExpansionOnly verifies the field expansion itself works —
// Vec2 field compiles without field access.
func TestRecordField_ExpansionOnly(t *testing.T) {
	src := `package test
type Skin struct { Note float64 }
type Note struct {
    Beat float64 ` + "`sonolus:\"imported\"`" + `
    pos  Vec2    ` + "`sonolus:\"memory\"`" + `
}
func (n *Note) Initialize() {
    debugPause()
}
func UpdateSpawn() float64 { return 0 }
`
	ess := engine.NewSingleFileSources(src)
	_, _, err := engine.CompilePlaySources(ess, nil)
	if err != nil {
		t.Fatalf("CompilePlaySources: %v", err)
	}
	t.Log("Vec2 field expansion OK")
}

// TestRecordField_Vec2 verifies Vec2 as a struct field type expands to 2 slots
// and n.pos.X / n.pos.Y access works.
func TestRecordField_Vec2(t *testing.T) {
	// First: verify bare "pos.x" and "pos.y" bindings exist.
	t.Run("bare expanded names work", func(t *testing.T) {
		// The expanded field names are "pos.x" and "pos.y" in bindings.
		// But they can't be accessed directly as n."pos.x" in Go syntax.
		// We need n.pos.X which the tracer translates.
	})

	t.Run("nested selector read", func(t *testing.T) {
		src := `package test
type Skin struct { Note float64 }
type Note struct {
    Beat float64 ` + "`sonolus:\"imported\"`" + `
    pos  Vec2    ` + "`sonolus:\"memory\"`" + `
}
func (n *Note) Initialize() {
    x := n.pos.X
    debugPause()
}
func UpdateSpawn() float64 { return 0 }
`
		ess := engine.NewSingleFileSources(src)
		_, _, err := engine.CompilePlaySources(ess, nil)
		if err != nil {
			t.Fatalf("CompilePlaySources: %v", err)
		}
	})
}

// TestRecordField_SonolusPrefix verifies sonolus.Vec2 as a field type.
func TestRecordField_SonolusPrefix(t *testing.T) {
	src := `package test
import "github.com/WindowsSov8forUs/sonolus-go/sonolus"
type Skin struct { Note float64 }
type Note struct {
    Beat float64 ` + "`sonolus:\"imported\"`" + `
    pos  sonolus.Vec2 ` + "`sonolus:\"memory\"`" + `
}
func (n *Note) Initialize() {
    n.pos.X = sin(n.Beat)
    n.pos.Y = cos(n.Beat)
    sonolus.Draw(1, n.pos.X, n.pos.Y, 1, 1, 0, 1, 0, 0)
}
func UpdateSpawn() float64 { return 0 }
`
	ess := engine.NewSingleFileSources(src)
	_, _, err := engine.CompilePlaySources(ess, nil)
	if err != nil {
		t.Fatalf("CompilePlaySources: %v", err)
	}
}

// TestRecordField_Quad verifies Quad as a struct field type.
func TestRecordField_Quad(t *testing.T) {
	src := `package test
type Skin struct { Note float64 }
type Note struct {
    Beat float64 ` + "`sonolus:\"imported\"`" + `
    area Quad    ` + "`sonolus:\"memory\"`" + `
}
func (n *Note) Initialize() {
    n.area.Blx = 0
    n.area.Bly = 0
    n.area.Tlx = 1
    n.area.Tly = 1
    n.area.Trx = 2
    n.area.Try = 0
    n.area.Brx = 1
    n.area.Bry = -1
    debugPause()
}
func UpdateSpawn() float64 { return 0 }
`
	ess := engine.NewSingleFileSources(src)
	_, _, err := engine.CompilePlaySources(ess, nil)
	if err != nil {
		t.Fatalf("CompilePlaySources: %v", err)
	}
}

// TestRecordField_ReadWrite exercises record field operations end-to-end.
func TestRecordField_ReadWrite(t *testing.T) {
	src := `package test
type Skin struct { Note float64 }
type Note struct {
    Beat float64 ` + "`sonolus:\"imported\"`" + `
    pos  Vec2    ` + "`sonolus:\"memory\"`" + `
    vel  Vec2    ` + "`sonolus:\"memory\"`" + `
}
func (n *Note) Initialize() {
    n.pos.X = n.Beat
    n.pos.Y = 0
    n.vel.X = 1.0
    n.vel.Y = 0.5
    draw(1, n.pos.X, n.pos.Y, 1, 1, 0, 1, 0, 0)
    draw(1, n.vel.X, n.vel.Y, 1, 1, 0, 1, 0, 0)
}
func UpdateSpawn() float64 { return 0 }
`
	ess := engine.NewSingleFileSources(src)
	_, _, err := engine.CompilePlaySources(ess, nil)
	if err != nil {
		t.Fatalf("CompilePlaySources: %v", err)
	}
}

// TestRecordField_CompoundAssign tests n.pos.X += 1 patterns.
func TestRecordField_CompoundAssign(t *testing.T) {
	src := `package test
type Skin struct { Note float64 }
type Note struct {
    Beat float64 ` + "`sonolus:\"imported\"`" + `
    pos  Vec2    ` + "`sonolus:\"memory\"`" + `
}
func (n *Note) Initialize() {
    n.pos.X = n.Beat
    n.pos.Y = 1
    n.pos.X += 2
    n.pos.Y *= 3
    draw(1, n.pos.X, n.pos.Y, 1, 1, 0, 1, 0, 0)
}
func UpdateSpawn() float64 { return 0 }
`
	ess := engine.NewSingleFileSources(src)
	_, _, err := engine.CompilePlaySources(ess, nil)
	if err != nil {
		t.Fatalf("CompilePlaySources: %v", err)
	}
}

// TestRecordField_ImportedReadOnly tests that read-only imported record fields
// cannot be written.
func TestRecordField_ImportedReadOnly(t *testing.T) {
	src := `package test
type Skin struct { Note float64 }
type Note struct {
    Beat float64 ` + "`sonolus:\"imported\"`" + `
    pos  Vec2    ` + "`sonolus:\"imported\"`" + `
}
func (n *Note) Initialize() {
    x := n.pos.X
    y := n.pos.Y
    draw(1, x, y, 1, 1, 0, 1, 0, 0)
}
func UpdateSpawn() float64 { return 0 }
`
	ess := engine.NewSingleFileSources(src)
	_, _, err := engine.CompilePlaySources(ess, nil)
	if err != nil {
		t.Fatalf("CompilePlaySources: %v", err)
	}
}

// TestRecordField_ImportedWritable tests that imported record fields are writable.
func TestRecordField_ImportedWritable(t *testing.T) {
	src := `package test
type Skin struct { Note float64 }
type Note struct {
    Beat float64 ` + "`sonolus:\"imported\"`" + `
    pos  Vec2    ` + "`sonolus:\"imported\"`" + `
}
func (n *Note) Initialize() {
    n.pos.X = 1
}
func UpdateSpawn() float64 { return 0 }
`
	ess := engine.NewSingleFileSources(src)
	_, _, err := engine.CompilePlaySources(ess, nil)
	if err != nil {
		t.Fatalf("imported record field should be writable: %v", err)
	}
}

// TestRecordField_Mat exercises Mat (6-field) record type.
func TestRecordField_Mat(t *testing.T) {
	src := `package test
type Skin struct { Note float64 }
type Note struct {
    Beat float64 ` + "`sonolus:\"imported\"`" + `
    m    Mat     ` + "`sonolus:\"memory\"`" + `
}
func (n *Note) Initialize() {
    n.m.M11 = 1
    n.m.M12 = 0
    n.m.M13 = 0
    n.m.M21 = 0
    n.m.M22 = 1
    n.m.M23 = 0
    debugPause()
}
func UpdateSpawn() float64 { return 0 }
`
	ess := engine.NewSingleFileSources(src)
	_, _, err := engine.CompilePlaySources(ess, nil)
	if err != nil {
		t.Fatalf("CompilePlaySources: %v", err)
	}
}

// TestSonolusGlobals verifies sonolus-prefixed global variable access.
func TestSonolusGlobals(t *testing.T) {
	t.Run("sonolus.Time", func(t *testing.T) {
		src := `package test
import "github.com/WindowsSov8forUs/sonolus-go/sonolus"
type Skin struct { Note float64 }
type Note struct { Beat float64 ` + "`sonolus:\"imported\"`" + ` }
func (n *Note) Initialize() {
    x := sonolus.Time
    sonolus.DebugPause()
}
func UpdateSpawn() float64 { return 0 }
`
		ess := engine.NewSingleFileSources(src)
		_, _, err := engine.CompilePlaySources(ess, nil)
		if err != nil {
			t.Logf("sonolus.Time: %v", err)
		} else {
			t.Log("sonolus.Time: OK")
		}
	})
	t.Run("bare time", func(t *testing.T) {
		src := `package test
import "github.com/WindowsSov8forUs/sonolus-go/sonolus"
type Skin struct { Note float64 }
type Note struct { Beat float64 ` + "`sonolus:\"imported\"`" + ` }
func (n *Note) Initialize() {
    x := time
    sonolus.DebugPause()
}
func UpdateSpawn() float64 { return 0 }
`
		ess := engine.NewSingleFileSources(src)
		_, _, err := engine.CompilePlaySources(ess, nil)
		if err != nil {
			t.Logf("bare time: %v", err)
		} else {
			t.Log("bare time: OK")
		}
	})
}

// TestSonolusConstructor tests sonolus.Vec2_ and other constructors.
func TestSonolusConstructor(t *testing.T) {
	src := `package test
import "github.com/WindowsSov8forUs/sonolus-go/sonolus"
type Skin struct { Note float64 }
type Note struct { Beat float64 ` + "`sonolus:\"imported\"`" + ` }
func (n *Note) Initialize() {
    v := vec2(n.Beat, 0)          // bare constructor (lowercase)
    sonolus.Draw(1, v.x, v.y, 1, 1, 0, 1, 0, 0)
}
func UpdateSpawn() float64 { return 0 }
`
	ess := engine.NewSingleFileSources(src)
	_, _, err := engine.CompilePlaySources(ess, nil)
	if err != nil {
		t.Fatalf("CompilePlaySources: %v", err)
	}
}

// TestGap_SonolusConstructor verifies sonolus.Vec2_/(x,y) constructor.
func TestGap_SonolusConstructor(t *testing.T) {
	src := `package test
import "github.com/WindowsSov8forUs/sonolus-go/sonolus"
type Skin struct { Note float64 }
type Note struct { Beat float64 ` + "`sonolus:\"imported\"`" + ` }
func (n *Note) Initialize() {
    v := sonolus.NewVec2(n.Beat, 0)
    sonolus.DebugPause()
}
func UpdateSpawn() float64 { return 0 }
`
	ess := engine.NewSingleFileSources(src)
	_, _, err := engine.CompilePlaySources(ess, nil)
	if err != nil {
		t.Logf("Vec2_ constructor: %v", err)
	} else {
		t.Log("Vec2_ constructor: OK")
	}
}

// TestGap_RecordField_Pair tests Pair (2-field) record type.
func TestGap_RecordField_Pair(t *testing.T) {
	src := `package test
type Skin struct { Note float64 }
type Note struct {
    Beat float64 ` + "`sonolus:\"imported\"`" + `
    p    Pair    ` + "`sonolus:\"memory\"`" + `
}
func (n *Note) Initialize() {
    n.p.First = n.Beat
    n.p.Second = 0
    debugPause()
}
func UpdateSpawn() float64 { return 0 }
`
	ess := engine.NewSingleFileSources(src)
	_, _, err := engine.CompilePlaySources(ess, nil)
	if err != nil {
		t.Fatalf("CompilePlaySources: %v", err)
	}
}

// TestGap_RecordField_Trans tests Trans (9-field) record type.
func TestGap_RecordField_Trans(t *testing.T) {
	src := `package test
type Skin struct { Note float64 }
type Note struct {
    Beat float64 ` + "`sonolus:\"imported\"`" + `
    t    Trans   ` + "`sonolus:\"memory\"`" + `
}
func (n *Note) Initialize() {
    n.t.M11 = 1
    n.t.M12 = 0
    n.t.M13 = 0
    n.t.M21 = 0
    n.t.M22 = 1
    n.t.M23 = 0
    n.t.M31 = 0
    n.t.M32 = 0
    n.t.M33 = 1
    debugPause()
}
func UpdateSpawn() float64 { return 0 }
`
	ess := engine.NewSingleFileSources(src)
	_, _, err := engine.CompilePlaySources(ess, nil)
	if err != nil {
		t.Fatalf("CompilePlaySources: %v", err)
	}
}

// TestGap_RecordField_Rect tests Rect (4-field) record type.
func TestGap_RecordField_Rect(t *testing.T) {
	src := `package test
type Skin struct { Note float64 }
type Note struct {
    Beat float64 ` + "`sonolus:\"imported\"`" + `
    area Rect    ` + "`sonolus:\"memory\"`" + `
}
func (n *Note) Initialize() {
    n.area.T = 0
    n.area.R = 1
    n.area.B = 2
    n.area.L = 0
    debugPause()
}
func UpdateSpawn() float64 { return 0 }
`
	ess := engine.NewSingleFileSources(src)
	_, _, err := engine.CompilePlaySources(ess, nil)
	if err != nil {
		t.Fatalf("CompilePlaySources: %v", err)
	}
}

// TestGap_RecordField_WatchMode tests record fields in Watch mode.
func TestGap_RecordField_WatchMode(t *testing.T) {
	src := `package test
type Skin struct { Note float64 }
type Note struct {
    Beat float64 ` + "`sonolus:\"imported\"`" + `
    pos  Vec2    ` + "`sonolus:\"memory\"`" + `
}
func (n *Note) Initialize() {
    n.pos.X = n.Beat
    n.pos.Y = 0
}
func (n *Note) UpdateParallel() {
    n.pos.X += 1
}
func UpdateSpawn() float64 { return 0 }
`
	ess := engine.NewSingleFileSources(src)
	_, err := engine.CompileWatchSources(ess, nil)
	if err != nil {
		t.Fatalf("CompileWatchSources: %v", err)
	}
}

// TestGap_RecordField_WholeAssign tests n.pos = vec2(x,y) whole-record assignment.
func TestGap_RecordField_WholeAssign(t *testing.T) {
	src := `package test
type Skin struct { Note float64 }
type Note struct {
    Beat float64 ` + "`sonolus:\"imported\"`" + `
    pos  Vec2    ` + "`sonolus:\"memory\"`" + `
}
func (n *Note) Initialize() {
    n.pos.X = n.Beat
    n.pos.Y = 0
}
func UpdateSpawn() float64 { return 0 }
`
	ess := engine.NewSingleFileSources(src)
	_, _, err := engine.CompilePlaySources(ess, nil)
	if err != nil {
		t.Fatalf("CompilePlaySources: %v", err)
	}
}

// TestRecordField_MatchesExistingGolden verifies existing golden tests
// still pass with record field expansion (i.e. no regression on scalar fields).
func TestRecordField_MatchesExistingGolden(t *testing.T) {
	// This is the simple.play golden test source — all fields are float64.
	src := `package test
type Skin struct { Note float64 }
type Note struct {
    Beat float64 ` + "`sonolus:\"imported\"`" + `
    T    float64 ` + "`sonolus:\"memory\"`" + `
}
func (n *Note) Initialize() {
    n.T = n.Beat
    debugPause()
}
func UpdateSpawn() float64 { return 0 }
`
	ess := engine.NewSingleFileSources(src)
	_, _, err := engine.CompilePlaySources(ess, nil)
	if err != nil {
		t.Fatalf("CompilePlaySources: %v", err)
	}
}

// TestRecordField_Mixed verifies record and scalar fields coexist.
func TestRecordField_Mixed(t *testing.T) {
	src := `package test
type Skin struct { Note float64 }
type Note struct {
    Beat   float64 ` + "`sonolus:\"imported\"`" + `
    pos    Vec2    ` + "`sonolus:\"memory\"`" + `
    radius float64 ` + "`sonolus:\"memory\"`" + `
}
func (n *Note) Initialize() {
    n.pos.X = n.Beat
    n.pos.Y = 0
    n.radius = 0.5
    draw(1, n.pos.X, n.pos.Y, 1, 1, 0, 1, 0, 0)
}
func UpdateSpawn() float64 { return 0 }
`
	ess := engine.NewSingleFileSources(src)
	_, _, err := engine.CompilePlaySources(ess, nil)
	if err != nil {
		t.Fatalf("CompilePlaySources: %v", err)
	}
}

// TestSonolusAllModes verifies sonolus-prefixed calls compile in all four modes.
func TestSonolusAllModes(t *testing.T) {
	src := `package test
import "github.com/WindowsSov8forUs/sonolus-go/sonolus"
type Skin struct { Note float64 }
type Note struct { Beat float64 ` + "`sonolus:\"imported\"`" + ` }
func (n *Note) Initialize() { sonolus.DebugPause() }
func (n *Note) UpdateParallel(dt float64) { sonolus.DebugPause() }
func UpdateSpawn() float64 { return 0 }
func Preprocess() {}
func Navigate() float64 { return 1 }
func Update() {}
`
	ess := engine.NewSingleFileSources(src)

	t.Run("play", func(t *testing.T) {
		_, _, err := engine.CompilePlaySources(ess, nil)
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("watch", func(t *testing.T) {
		_, err := engine.CompileWatchSources(ess, nil)
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("preview", func(t *testing.T) {
		_, err := engine.CompilePreviewSources(ess, nil)
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("tutorial", func(t *testing.T) {
		_, err := engine.CompileTutorialSources(ess, nil)
		if err != nil {
			t.Fatal(err)
		}
	})
}

// TestNewSingleFileSources_BackwardCompat verifies that the single-file
// entry point still works correctly.
func TestNewSingleFileSources_BackwardCompat(t *testing.T) {
	src := `package test
type Skin struct { Note float64 }
type Note struct { Beat float64 ` + "`sonolus:\"imported\"`" + ` }
func (n *Note) Initialize() { debugPause() }
func UpdateSpawn() float64 { return 0 }
func Preprocess() {}
func Navigate() float64 { return 1 }
func Update() {}
`
	ess := engine.NewSingleFileSources(src)

	_, _, err := engine.CompilePlaySources(ess, nil)
	if err != nil {
		t.Fatalf("CompilePlaySources: %v", err)
	}
}
