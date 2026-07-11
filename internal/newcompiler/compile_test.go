package newcompiler

import (
	"math"
	"strings"
	"testing"

	"github.com/WindowsSov8forUs/sonolus-go/internal/newcompiler/mode"
)

func TestParseDeclarationsPlay(t *testing.T) {
	decl, err := ParseDeclarations(mode.ModePlay, "./testdata/declarations")
	if err != nil {
		t.Fatal(err)
	}
	if len(decl.Archetypes) != 1 || decl.Archetypes[0].Name != "TapNote" {
		t.Fatalf("unexpected archetypes: %#v", decl.Archetypes)
	}
	a := decl.Archetypes[0]
	if !a.HasInput || len(a.Imports) != 1 || len(a.Exports) != 1 {
		t.Fatalf("unexpected archetype metadata: %#v", a)
	}
	if len(a.Callbacks) != 3 {
		t.Fatalf("unexpected callbacks: %#v", a.Callbacks)
	}
	if len(a.Callbacks[0].Intrinsics) != 4 {
		t.Fatalf("unexpected preprocess intrinsics: %#v", a.Callbacks[0].Intrinsics)
	}
	if decl.Resources.Skin == nil || len(decl.Resources.Skin.Sprites) != 1 {
		t.Fatalf("unexpected skin: %#v", decl.Resources.Skin)
	}
	if len(decl.Configuration.Options) != 3 {
		t.Fatalf("unexpected options: %#v", decl.Configuration.Options)
	}
	if decl.ROM == nil || len(decl.ROM.Values) != 3 {
		t.Fatalf("unexpected ROM: %#v", decl.ROM)
	}
}

func TestParseDeclarationsNamedResourceValues(t *testing.T) {
	decl, err := ParseDeclarations(mode.ModePlay, "./testdata/namedresource")
	if err != nil {
		t.Fatal(err)
	}
	if decl.Resources.Skin == nil || len(decl.Resources.Skin.Sprites) != 4 {
		t.Fatalf("unexpected skin: %#v", decl.Resources.Skin)
	}
	if decl.Resources.Skin.RenderMode != "lightweight" {
		t.Fatalf("render mode = %q", decl.Resources.Skin.RenderMode)
	}
	if got := decl.Resources.Skin.Sprites[0].Name; got != "#NOTE_HEAD" {
		t.Fatalf("first sprite name = %q", got)
	}
	if got := decl.Resources.Skin.Sprites[1].Name; got != "custom.sprite" {
		t.Fatalf("second sprite name = %q", got)
	}
	if decl.Resources.SpriteIDs["#NOTE_HEAD"] != 0 || decl.Resources.SpriteIDs["custom.sprite"] != 1 {
		t.Fatalf("unexpected sprite IDs: %#v", decl.Resources.SpriteIDs)
	}
	if decl.Resources.SpriteIDs["group.0"] != 2 || decl.Resources.SpriteIDs["group.1"] != 3 {
		t.Fatalf("unexpected group IDs: %#v", decl.Resources.SpriteIDs)
	}
}

func TestParseDeclarationsSeparateInstructionNamespaces(t *testing.T) {
	decl, err := ParseDeclarations(mode.ModeTutorial, "./testdata/instructionresource")
	if err != nil {
		t.Fatal(err)
	}
	if decl.Resources.Instruction == nil || len(decl.Resources.Instruction.Texts) != 1 || len(decl.Resources.Instruction.Icons) != 1 {
		t.Fatalf("unexpected instructions: %#v", decl.Resources.Instruction)
	}
	if decl.Resources.Instruction.Texts[0].Name != "Tap" || decl.Resources.Instruction.Icons[0].Name != "#HAND" {
		t.Fatalf("unexpected instruction names: %#v", decl.Resources.Instruction)
	}
}

func TestParseDeclarationsBuckets(t *testing.T) {
	decl, err := ParseDeclarations(mode.ModePlay, "./testdata/bucketresource")
	if err != nil {
		t.Fatal(err)
	}
	if len(decl.Resources.Buckets) != 1 || len(decl.Resources.Buckets[0].Sprites) != 2 {
		t.Fatalf("unexpected buckets: %#v", decl.Resources.Buckets)
	}
	bucket := decl.Resources.Buckets[0]
	if bucket.Unit != "#MILLISECONDS" || bucket.Sprites[0].ID != 0 || bucket.Sprites[1].FallbackID != 1 {
		t.Fatalf("unexpected bucket metadata: %#v", bucket)
	}
}

func TestParseDeclarationsRejectsUnsupportedStandardSymbol(t *testing.T) {
	_, err := ParseDeclarations(mode.ModePlay, "./testdata/invalidstdlib")
	if err == nil || !strings.Contains(err.Error(), "math/rand.Seed is not a Sonolus intrinsic") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseDeclarationsRejectsWrongModeAPI(t *testing.T) {
	_, err := ParseDeclarations(mode.ModePlay, "./testdata/invalidmode")
	if err == nil || !strings.Contains(err.Error(), "not available in play mode") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseDeclarationsRejectsWrongCallbackPhase(t *testing.T) {
	_, err := ParseDeclarations(mode.ModePlay, "./testdata/invalidphase")
	if err == nil || !strings.Contains(err.Error(), "sonolus/play.uiAPI.Configure cannot write during updateParallel callback") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseDeclarationsROMFile(t *testing.T) {
	decl, err := ParseDeclarations(mode.ModePlay, "./testdata/romfile")
	if err != nil {
		t.Fatal(err)
	}
	if decl.ROM == nil || len(decl.ROM.Bytes) != 4 || len(decl.ROM.Values) != 1 {
		t.Fatalf("unexpected embedded ROM: %#v", decl.ROM)
	}
	want := math.Float32frombits(0x0a434241)
	if decl.ROM.Values[0] != want {
		t.Fatalf("ROM value = %v, want %v", decl.ROM.Values[0], want)
	}
}

func TestParseDeclarationsRejectsUnknownTags(t *testing.T) {
	_, err := ParseDeclarations(mode.ModePlay, "./testdata/invalid")
	if err == nil {
		t.Fatal("expected invalid tags to be rejected")
	}
	for _, key := range []string{"typo", "unknown", "mystery"} {
		if !strings.Contains(err.Error(), key) {
			t.Errorf("error does not mention %q: %v", key, err)
		}
	}
}

func TestParseDeclarationsOtherModes(t *testing.T) {
	tests := []struct {
		mode       mode.Mode
		globals    int
		archetypes int
	}{
		{mode.ModeWatch, 1, 1},
		{mode.ModePreview, 0, 1},
		{mode.ModeTutorial, 3, 0},
	}
	for _, tt := range tests {
		t.Run(string(tt.mode), func(t *testing.T) {
			decl, err := ParseDeclarations(tt.mode, "./testdata/declarations")
			if err != nil {
				t.Fatal(err)
			}
			if len(decl.Globals) != tt.globals || len(decl.Archetypes) != tt.archetypes {
				t.Fatalf("globals=%d archetypes=%d", len(decl.Globals), len(decl.Archetypes))
			}
		})
	}
}
