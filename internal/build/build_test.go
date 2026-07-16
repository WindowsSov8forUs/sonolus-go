package build

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/WindowsSov8forUs/sonolus-core-go/codec"
	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler"
)

func tinyPlayData(t *testing.T) *resource.EnginePlayData {
	t.Helper()
	return &resource.EnginePlayData{
		Skin:     resource.EngineSkinData{},
		Effect:   resource.EngineEffectData{},
		Particle: resource.EngineParticleData{},
		Buckets:  []resource.EngineDataBucket{},
		Archetypes: []resource.EnginePlayDataArchetype{{
			Name:           "A",
			Imports:        []resource.EngineDataArchetypeImport{},
			Exports:        []resource.EngineArchetypeDataName{},
			UpdateParallel: &resource.EnginePlayDataArchetypeCallback{Index: 2},
		}},
		Nodes: []resource.EngineDataNode{
			resource.EngineDataValueNode{Value: 1000},
			resource.EngineDataValueNode{Value: 0},
			resource.EngineDataFunctionNode{Func: resource.RuntimeFunctionGet, Args: []int{0, 1}},
		},
	}
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
	blob, err := codec.Compress(data)
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
	pkg, err := PackagePlay(&resource.EngineConfiguration{}, data, nil)
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

func TestPackageAnyNilInput(t *testing.T) {
	blob, err := PackageAny(nil)
	if err != nil {
		t.Fatalf("PackageAny(nil) should not error: %v", err)
	}
	if len(blob) == 0 {
		t.Error("PackageAny(nil) returned empty blob")
	}
	// Verify it round-trips.
	_, err = codec.Decompress[json.RawMessage](blob)
	if err != nil {
		t.Errorf("PackageAny(nil) output not valid gzip JSON: %v", err)
	}
}

func TestPackageRoundTripError(t *testing.T) {
	// Decompress valid output from Compress.
	data := tinyPlayData(t)
	blob, err := codec.Compress(data)
	if err != nil {
		t.Fatal(err)
	}
	got, err := codec.Decompress[resource.EnginePlayData](blob)
	if err != nil {
		t.Fatalf("valid round-trip failed: %v", err)
	}
	if len(got.Nodes) != len(data.Nodes) {
		t.Errorf("node count mismatch: got %d, want %d", len(got.Nodes), len(data.Nodes))
	}
}

func TestROMInvalidFile(t *testing.T) {
	rom, err := BuildROMFromFile(filepath.Join(t.TempDir(), "nonexistent.rom"))
	if err == nil {
		t.Error("expected error for nonexistent ROM file, got nil")
	}
	if rom != nil {
		t.Error("expected nil ROM for invalid file")
	}
}

func TestROMTruncatedFile(t *testing.T) {
	dir := t.TempDir()
	truncROM := filepath.Join(dir, "trunc.rom")
	// Write 5 bytes (not divisible by 4 — float32 requires 4-byte alignment).
	if err := os.WriteFile(truncROM, []byte{0, 0, 0, 0, 0}, 0o644); err != nil {
		t.Fatal(err)
	}
	rom, err := BuildROMFromFile(truncROM)
	if err == nil {
		t.Error("expected error for truncated ROM file, got nil")
	}
	if rom != nil {
		t.Error("expected nil ROM for truncated file")
	}
}

func TestDefaultROMBytes(t *testing.T) {
	b, err := DefaultROMBytes()
	if err != nil {
		t.Fatalf("DefaultROMBytes: %v", err)
	}
	if len(b) == 0 {
		t.Error("DefaultROMBytes returned empty bytes")
	}
}

func TestPackageNonPlay_Write(t *testing.T) {
	cfg := &resource.EngineConfiguration{}
	rom, err := DefaultROMBytes()
	if err != nil {
		t.Fatalf("DefaultROMBytes: %v", err)
	}

	// Preview mode
	previewData := &resource.EnginePreviewData{
		Skin:  resource.EngineSkinData{},
		Nodes: []resource.EngineDataNode{},
	}
	pkg, err := PackageNonPlay(cfg, rom, previewData, func(p *PackagedEngine, b []byte) { p.PreviewData = b })
	if err != nil {
		t.Fatalf("PackageNonPlay(preview): %v", err)
	}
	dir := filepath.Join(t.TempDir(), "preview-engine")
	if err := pkg.Write(dir); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{FileConfiguration, FilePreviewData, FileROM} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("missing %s in non-play package: %v", name, err)
		}
	}
}

func TestPackageArtifactsRawROMAndSelectedModes(t *testing.T) {
	rawROM := []byte{0, 0, 0xc0, 0x7f, 0, 0, 0x80, 0x7f}
	packaged, err := PackageArtifacts(&compiler.Artifacts{
		Configuration: &resource.EngineConfiguration{},
		ROM:           rawROM,
		Play:          &resource.EnginePlayData{},
	})
	if err != nil {
		t.Fatal(err)
	}
	reader, err := gzip.NewReader(bytes.NewReader(packaged.ROM))
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decoded, rawROM) {
		t.Fatalf("ROM round-trip = %v, want %v", decoded, rawROM)
	}
	if packaged.PlayData == nil || packaged.WatchData != nil || packaged.PreviewData != nil || packaged.TutorialData != nil {
		t.Fatalf("unexpected packaged modes: play=%t watch=%t preview=%t tutorial=%t", packaged.PlayData != nil, packaged.WatchData != nil, packaged.PreviewData != nil, packaged.TutorialData != nil)
	}
}

func TestWriteAtomicReplacesCompleteSnapshot(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "engine")
	first := &PackagedEngine{Configuration: []byte("old-config"), ROM: []byte("old-rom"), PlayData: []byte("old-play")}
	if err := first.WriteAtomic(dir); err != nil {
		t.Fatal(err)
	}
	second := &PackagedEngine{Configuration: []byte("new-config"), ROM: []byte("new-rom"), WatchData: []byte("new-watch")}
	if err := second.WriteAtomic(dir); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{
		FileConfiguration: "new-config",
		FileROM:           "new-rom",
		FileWatchData:     "new-watch",
	} {
		got, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if string(got) != want {
			t.Fatalf("%s = %q, want %q", name, got, want)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, FilePlayData)); !os.IsNotExist(err) {
		t.Fatalf("stale play data survived atomic replacement: %v", err)
	}
}

func TestBuildROMFromFile_Valid(t *testing.T) {
	dir := t.TempDir()
	romPath := filepath.Join(dir, "valid.rom")
	// Write 8 bytes (2 float32 values).
	data := []byte{0, 0, 0x80, 0x3F, 0, 0, 0, 0x40} // 1.0, 2.0 in little-endian
	if err := os.WriteFile(romPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	rom, err := BuildROMFromFile(romPath)
	if err != nil {
		t.Fatalf("BuildROMFromFile: %v", err)
	}
	// BuildROMFromFile returns gzipped bytes, not raw float32 values.
	if len(rom) == 0 {
		t.Fatal("ROM bytes should not be empty")
	}
}

func TestDefaultROM(t *testing.T) {
	rom := DefaultROM()
	if len(rom) == 0 {
		t.Error("DefaultROM should have entries for non-finite float values")
	}
	// Verify all entries are non-finite (NaN/Inf).
	for _, v := range rom {
		if !(v != v || v > 3e38 || v < -3e38) {
			t.Errorf("expected non-finite ROM value, got %v", v)
		}
	}
}
