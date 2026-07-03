package engine

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/WindowsSov8forUs/sonolus-core-go/codec"
	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/build"
)

// nodeContains reports whether the node list has a function node with the op.
func nodeContains(nodes []resource.EngineDataNode, op resource.RuntimeFunction) bool {
	for _, n := range nodes {
		if f, ok := n.(resource.EngineDataFunctionNode); ok && f.Func == op {
			return true
		}
	}
	return false
}

func TestCompilePlayEndToEnd(t *testing.T) {
	src := `package engine
type Counter struct{}
func (c Counter) UpdateParallel() { set(2000, 0, get(2000, 0) + 1) }
type Stage struct{}
func (s Stage) Initialize() { set(2000, 1, 5) }
`
	data, _, err := CompilePlayFile(src)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	if len(data.Archetypes) != 2 {
		t.Fatalf("archetypes = %d, want 2", len(data.Archetypes))
	}
	counter := data.Archetypes[0]
	if counter.Name != "Counter" || counter.UpdateParallel == nil {
		t.Fatalf("Counter.updateParallel missing: %+v", counter)
	}
	if data.Archetypes[1].Initialize == nil {
		t.Fatalf("Stage.initialize missing")
	}
	if len(data.Nodes) == 0 {
		t.Fatal("no nodes")
	}

	// The `cell += 1` pattern should fold to a SetAdd compound assignment.
	if !nodeContains(data.Nodes, resource.RuntimeFunctionSetAdd) {
		b, _ := json.Marshal(data.Nodes)
		t.Errorf("expected a SetAdd node, nodes: %s", b)
	}

	t.Logf("compiled engine: %d archetypes, %d nodes", len(data.Archetypes), len(data.Nodes))
}

func TestCompilePlayPackages(t *testing.T) {
	src := `package engine
type Note struct{}
func (n Note) Touch() {}
func (n Note) UpdateParallel() { set(2000, 0, get(2000, 0) + 1) }
`
	data, cfg, err := CompilePlayFile(src)
	if err != nil {
		t.Fatal(err)
	}

	pkg, err := build.PackagePlay(cfg, data, nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := codec.Decompress[resource.EnginePlayData](pkg.PlayData)
	if err != nil {
		t.Fatalf("round trip: %v", err)
	}
	if len(got.Archetypes) != 1 || got.Archetypes[0].Name != "Note" {
		t.Fatalf("round trip lost archetype: %+v", got.Archetypes)
	}
	if got.Archetypes[0].UpdateParallel == nil {
		t.Fatal("round trip lost callback")
	}
}

func TestCompilePlayWithArray(t *testing.T) {
	src := `package engine
type Buffer struct{}
func (b Buffer) UpdateParallel() {
	a := array(4)
	a[0] = get(2000, 0)
	a[1] = a[0] + 1
	i := get(2000, 5)
	set(2000, 0, a[i])
}
`
	data, _, err := CompilePlayFile(src)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if data.Archetypes[0].UpdateParallel == nil {
		t.Fatal("missing callback")
	}
	if len(data.Nodes) == 0 {
		t.Fatal("no nodes")
	}
	if _, err := codec.Decompress[resource.EnginePlayData](mustPackage(t, data)); err != nil {
		t.Fatalf("round trip: %v", err)
	}
}

func mustPackage(t *testing.T, data *resource.EnginePlayData) []byte {
	t.Helper()
	pkg, err := build.PackagePlay(&resource.EngineConfiguration{}, data, nil)
	if err != nil {
		t.Fatal(err)
	}
	return pkg.PlayData
}

func TestCompilePlayWithRecord(t *testing.T) {
	src := `package engine
type Mover struct{}
func (m Mover) UpdateParallel() {
	p := vec2(get(2000, 0), get(2000, 1))
	p.x = p.x + 1
	set(2000, 0, p.x)
	set(2000, 1, p.y)
}
`
	data, _, err := CompilePlayFile(src)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if data.Archetypes[0].UpdateParallel == nil || len(data.Nodes) == 0 {
		t.Fatal("missing callback/nodes")
	}
	if _, err := codec.Decompress[resource.EnginePlayData](mustPackage(t, data)); err != nil {
		t.Fatalf("round trip: %v", err)
	}
}

func TestCompilePlayDraws(t *testing.T) {
	src := `package engine
type Sprite struct{}
func (s Sprite) UpdateParallel() {
	x := sin(time)
	p := vec2(x, 0)
	draw(1, p.x, p.y, p.x, 1, 0, 1, 0, 0)
}
`
	data, _, err := CompilePlayFile(src)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if data.Archetypes[0].UpdateParallel == nil {
		t.Fatal("missing updateParallel")
	}
	if !nodeContains(data.Nodes, resource.RuntimeFunctionDraw) {
		b, _ := json.Marshal(data.Nodes)
		t.Errorf("expected a Draw node, got: %s", b)
	}
	if !nodeContains(data.Nodes, resource.RuntimeFunctionSin) {
		t.Errorf("expected a Sin node")
	}
	if _, err := codec.Decompress[resource.EnginePlayData](mustPackage(t, data)); err != nil {
		t.Fatalf("round trip: %v", err)
	}
	t.Logf("drawing engine compiled: %d nodes", len(data.Nodes))
}

func TestArchetypeFields(t *testing.T) {
	src := `package engine
type Note struct {
	Beat       float64 ` + "`sonolus:\"imported\"`" + `
	TargetTime float64 ` + "`sonolus:\"memory\"`" + `
}
func (n Note) Initialize() { n.TargetTime = n.Beat * 0.5 }
func (n Note) UpdateParallel() { draw(1, n.TargetTime, 0, n.TargetTime, 1, 0, 1, 0, 0) }
`
	data, _, err := CompilePlayFile(src)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	a := data.Archetypes[0]
	if len(a.Imports) != 1 || a.Imports[0].Name != "Beat" || a.Imports[0].Index != 0 {
		t.Fatalf("imports = %+v, want [{Beat 0}]", a.Imports)
	}
	if a.Initialize == nil || a.UpdateParallel == nil {
		t.Fatalf("missing callbacks: %+v", a)
	}
	if !nodeContains(data.Nodes, resource.RuntimeFunctionDraw) {
		t.Errorf("expected Draw")
	}
	if _, err := codec.Decompress[resource.EnginePlayData](mustPackage(t, data)); err != nil {
		t.Fatalf("round trip: %v", err)
	}
}

func TestArchetypeImportedFieldReadOnly(t *testing.T) {
	src := `package engine
type Note struct {
	Beat float64 ` + "`sonolus:\"imported\"`" + `
}
func (n Note) Initialize() { n.Beat = 5 }
`
	_, _, err := CompilePlayFile(src)
	if err == nil {
		t.Fatal("expected error writing to imported (read-only) field")
	}
	if !strings.Contains(err.Error(), "read-only") {
		t.Errorf("error should mention read-only: %v", err)
	}
}

func TestCompilePlayReportsSourceErrors(t *testing.T) {
	src := `package engine
type Bad struct{}
func (b Bad) UpdateParallel() { set(2000, 0, undefinedVar) }
`
	_, _, err := CompilePlayFile(src)
	if err == nil {
		t.Fatal("expected error for undefined identifier")
	}
	// TypeCheck catches undefined identifiers first; frontend trace would
	// name the archetype and callback. Accept either path.
	if !strings.Contains(err.Error(), "undefined") &&
		!(strings.Contains(err.Error(), "Bad") && strings.Contains(err.Error(), "updateParallel")) {
		t.Errorf("error should name undefined identifier: %v", err)
	}
}
