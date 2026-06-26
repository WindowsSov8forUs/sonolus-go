package build

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/WindowsSov8forUs/sonolus-core-go/codec"
	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

	"github.com/WindowsSov8forUs/sonolus-go/compiler/play"
	"github.com/WindowsSov8forUs/sonolus-go/compiler/snode"
)

func tinyPlayData(t *testing.T) *resource.EnginePlayData {
	t.Helper()
	data := play.BuildPlayData(
		resource.EngineSkinData{},
		resource.EngineEffectData{},
		resource.EngineParticleData{},
		nil,
		[]play.ArchetypeDef{{Name: "A"}},
	)
	get := snode.Call(resource.RuntimeFunctionGet, snode.Val(1000), snode.Val(0))
	if err := play.Assemble(data, []*play.CompileResult{
		{ArchetypeIndex: 0, Callback: play.CallbackUpdateParallel, Order: 0, Node: get},
	}); err != nil {
		t.Fatal(err)
	}
	return data
}

func TestNodeSerializationByteExact(t *testing.T) {
	data := tinyPlayData(t)
	b, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	// Nodes must serialize identically to the reference: value nodes as
	// {"value":n}, function nodes as {"func":...,"args":[...]}, children first.
	want := `"nodes":[{"value":1000},{"value":0},{"func":"Get","args":[0,1]}]`
	if !strings.Contains(string(b), want) {
		t.Errorf("nodes serialization mismatch.\ngot: %s\nwant substring: %s", b, want)
	}
}

func TestPackageRoundTrip(t *testing.T) {
	data := tinyPlayData(t)
	blob, err := PackagePlayData(data)
	if err != nil {
		t.Fatal(err)
	}
	got, err := codec.Decompress[resource.EnginePlayData](blob)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Nodes) != 3 || len(got.Archetypes) != 1 {
		t.Fatalf("round-trip lost data: %d nodes, %d archetypes", len(got.Nodes), len(got.Archetypes))
	}
	if got.Archetypes[0].UpdateParallel == nil || got.Archetypes[0].UpdateParallel.Index != 2 {
		t.Errorf("round-trip callback = %+v", got.Archetypes[0].UpdateParallel)
	}
}

func TestWriteFiles(t *testing.T) {
	data := tinyPlayData(t)
	pkg, err := PackagePlay(&resource.EngineConfiguration{}, data)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "engine")
	if err := pkg.Write(dir); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{FileConfiguration, FilePlayData} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("missing %s: %v", name, err)
		}
	}
}
