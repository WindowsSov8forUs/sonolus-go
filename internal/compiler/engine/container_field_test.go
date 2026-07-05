package engine_test

import (
	"testing"

	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/engine"
	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/ir/optimize"
)

func TestContainerField_VarArray(t *testing.T) {
	src := `package test
type Skin struct { Note float64 }
type Note struct {
    Beat       float64 ` + "`sonolus:\"imported\"`" + `
    Candidates VarArray ` + "`sonolus:\"memory,cap=8\"`" + `
}
func (n *Note) Initialize() {
    n.Candidates.clear()
    n.Candidates.append(n.Beat)
    x := n.Candidates.len()
    set(0, 0, x)
}
func UpdateSpawn() float64 { return 0 }
`
	ess := engine.NewSingleFileSources(src)
	_, _, err := engine.CompilePlaySources(ess, &engine.CompileOptions{
		Opt: optimize.LevelStandard,
	})
	if err != nil {
		t.Fatalf("VarArray field: %v", err)
	}
	t.Log("OK")
}

func TestContainerField_ArrayMap(t *testing.T) {
	src := `package test
type Skin struct { Note float64 }
type Note struct {
    Beat float64 ` + "`sonolus:\"imported\"`" + `
    Map  ArrayMap ` + "`sonolus:\"memory,cap=16\"`" + `
}
func (n *Note) Initialize() {
    n.Map.clear()
    n.Map.set(n.Beat, 1)
    v := n.Map.get(n.Beat)
    set(0, 0, v)
}
func UpdateSpawn() float64 { return 0 }
`
	ess := engine.NewSingleFileSources(src)
	_, _, err := engine.CompilePlaySources(ess, &engine.CompileOptions{
		Opt: optimize.LevelStandard,
	})
	if err != nil {
		t.Fatalf("ArrayMap field: %v", err)
	}
	t.Log("OK")
}

func TestContainerField_ArraySet(t *testing.T) {
	src := `package test
type Skin struct { Note float64 }
type Note struct {
    Beat float64 ` + "`sonolus:\"imported\"`" + `
    Set  ArraySet ` + "`sonolus:\"memory,cap=10\"`" + `
}
func (n *Note) Initialize() {
    n.Set.clear()
    n.Set.add(n.Beat)
    v := n.Set.contains(n.Beat)
    set(0, 0, v)
}
func UpdateSpawn() float64 { return 0 }
`
	ess := engine.NewSingleFileSources(src)
	_, _, err := engine.CompilePlaySources(ess, &engine.CompileOptions{
		Opt: optimize.LevelStandard,
	})
	if err != nil {
		t.Fatalf("ArraySet field: %v", err)
	}
	t.Log("OK")
}

func TestContainerField_VarArrayPopInsert(t *testing.T) {
	src := `package test
type Skin struct { Note float64 }
type Note struct {
    Beat float64 ` + "`sonolus:\"imported\"`" + `
    Arr  VarArray ` + "`sonolus:\"memory,cap=64\"`" + `
}
func (n *Note) Initialize() {
    n.Arr.append(1)
    n.Arr.append(2)
    n.Arr.append(3)
    v := n.Arr.pop()
    n.Arr.insert(1, v)
    sz := n.Arr.len()
    set(0, 0, sz)
}
func UpdateSpawn() float64 { return 0 }
`
	ess := engine.NewSingleFileSources(src)
	_, _, err := engine.CompilePlaySources(ess, &engine.CompileOptions{
		Opt: optimize.LevelStandard,
	})
	if err != nil {
		t.Fatalf("VarArray pop/insert: %v", err)
	}
	t.Log("OK")
}

func TestContainerField_WithoutCapacityTag(t *testing.T) {
	// Container field without cap= should be an error.
	src := `package test
type Skin struct { Note float64 }
type Note struct {
    Beat float64 ` + "`sonolus:\"imported\"`" + `
    Arr  VarArray ` + "`sonolus:\"memory\"`" + `
}
func (n *Note) Initialize() {
    debugPause()
}
func UpdateSpawn() float64 { return 0 }
`
	ess := engine.NewSingleFileSources(src)
	_, _, err := engine.CompilePlaySources(ess, &engine.CompileOptions{
		Opt: optimize.LevelStandard,
	})
	if err == nil {
		t.Fatal("expected error for container field without cap=")
	}
	t.Logf("expected: %v", err)
}

func TestContainerField_MultipleContainers(t *testing.T) {
	src := `package test
type Skin struct { Note float64 }
type Note struct {
    Beat  float64 ` + "`sonolus:\"imported\"`" + `
    Arr1  VarArray ` + "`sonolus:\"memory,cap=32\"`" + `
    Arr2  ArrayMap ` + "`sonolus:\"memory,cap=8\"`" + `
}
func (n *Note) Initialize() {
    n.Arr1.append(n.Beat)
    n.Arr2.set(0, n.Beat)
    n.Arr2.set(1, 2)
    v := n.Arr2.get(0)
    set(0, 0, v)
}
func UpdateSpawn() float64 { return 0 }
`
	ess := engine.NewSingleFileSources(src)
	_, _, err := engine.CompilePlaySources(ess, &engine.CompileOptions{
		Opt: optimize.LevelStandard,
	})
	if err != nil {
		t.Fatalf("Multiple containers: %v", err)
	}
	t.Log("OK")
}

func TestContainerField_IndexExpr(t *testing.T) {
	// Struct field container with bracket indexing: n.Arr[i]
	src := `package test
type Skin struct { Note float64 }
type Note struct {
    Beat float64 ` + "`sonolus:\"imported\"`" + `
    Arr  VarArray ` + "`sonolus:\"memory,cap=64\"`" + `
    X    float64 ` + "`sonolus:\"memory\"`" + `
}
func (n *Note) Initialize() {
    n.Arr.append(n.Beat)
    n.Arr.append(10)
    x := n.Arr[0]                     // read via index
    n.Arr[1] = x + 1                  // write via index
    n.X = n.Arr[1]                    // read back
}
func UpdateSpawn() float64 { return 0 }
`
	ess := engine.NewSingleFileSources(src)
	_, _, err := engine.CompilePlaySources(ess, &engine.CompileOptions{
		Opt: optimize.LevelStandard,
	})
	if err != nil {
		t.Fatalf("IndexExpr: %v", err)
	}
	t.Log("OK")
}

func TestContainerField_SetAddSetRemoveIsFull(t *testing.T) {
	src := `package test
type Skin struct { Note float64 }
type Note struct {
    Beat float64 ` + "`sonolus:\"imported\"`" + `
    Arr  VarArray ` + "`sonolus:\"memory,cap=8\"`" + `
    X    float64 ` + "`sonolus:\"memory\"`" + `
}
func (n *Note) Initialize() {
    n.Arr.setAdd(n.Beat)
    n.Arr.setAdd(n.Beat + 1)
    x := n.Arr.contains(n.Beat)
    n.Arr.setRemove(n.Beat)
    n.X = n.Arr.isFull()
}
func UpdateSpawn() float64 { return 0 }
`
	ess := engine.NewSingleFileSources(src)
	_, _, err := engine.CompilePlaySources(ess, &engine.CompileOptions{
		Opt: optimize.LevelStandard,
	})
	if err != nil {
		t.Fatalf("SetAdd/SetRemove/IsFull: %v", err)
	}
	t.Log("OK")
}

func TestConfigOptionAccess(t *testing.T) {
	src := `package test
type Skin struct { Note float64 }
type Note struct { Beat float64 ` + "`sonolus:\"imported\"`" + ` }
type Config struct {
    Speed float64 ` + "`sonolus:\"slider,min=0,max=100,def=50\"`" + `
    Boost float64 ` + "`sonolus:\"toggle,def=1\"`" + `
}
func (n *Note) Initialize() {
    x := sonolus.Speed              // slider option
    y := sonolus.Boost              // toggle option
    set(0, 0, x + y)
}
func UpdateSpawn() float64 { return 0 }
`
	ess := engine.NewSingleFileSources(src)
	_, _, err := engine.CompilePlaySources(ess, nil)
	if err != nil {
		t.Fatalf("Config option access: %v", err)
	}
	t.Log("OK")
}

func TestJudgmentWindowStructField(t *testing.T) {
	src := `package test
type Skin struct { Note float64 }
type Note struct {
    Beat   float64 ` + "`sonolus:\"imported\"`" + `
    Window JudgmentWindow ` + "`sonolus:\"memory\"`" + `
}
func (n *Note) Initialize() {
    n.Window.perfectMin = -0.1
    n.Window.perfectMax = 0.1
    n.Window.greatMin = -0.2
    n.Window.greatMax = 0.2
    n.Window.goodMin = -0.4
    n.Window.goodMax = 0.4
    result := n.Window.judge(0, n.Beat)
    set(0, 0, result)
}
func UpdateSpawn() float64 { return 0 }
`
	ess := engine.NewSingleFileSources(src)
	_, _, err := engine.CompilePlaySources(ess, nil)
	if err != nil {
		t.Fatalf("JW: %v", err)
	}
	t.Log("OK")
}
