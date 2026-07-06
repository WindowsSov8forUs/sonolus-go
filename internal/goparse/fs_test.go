package goparse

import (
	"testing"
)

func TestLoadProject_Complex(t *testing.T) {
	pkgs, err := LoadProject("testdata/complex", FilterStdlib, FilterNonSonolus)
	if err != nil {
		t.Fatalf("LoadProject: %v", err)
	}

	// Should have 3 packages: main, notes, stage.
	if len(pkgs) != 3 {
		t.Fatalf("len(pkgs) = %d, want 3 (%v)", len(pkgs), pkgKeys(pkgs))
	}

	// Main package.
	mainPkg := pkgs[""]
	if mainPkg == nil {
		t.Fatal("main package (key \"\") is missing")
	}
	if mainPkg.Path != "" {
		t.Errorf("main.Path = %q, want \"\"", mainPkg.Path)
	}
	if mainPkg.Name != "engine" {
		t.Errorf("main.Name = %q, want %q", mainPkg.Name, "engine")
	}
	if len(mainPkg.Sources) != 1 {
		t.Errorf("main.Sources has %d entries, want 1", len(mainPkg.Sources))
	}

	// Collect all types across files in main package.
	mainTypes := collectTypeNames(mainPkg)
	if len(mainTypes) != 1 || mainTypes[0] != "Skin" {
		t.Errorf("main types = %v, want [Skin]", mainTypes)
	}
	mainFuncs := collectFuncNames(mainPkg)
	if len(mainFuncs) != 2 {
		t.Errorf("main funcs = %v, want [UpdateSpawn, Preprocess]", mainFuncs)
	}

	// Notes package — package name != directory name.
	notesPkg := pkgs["notes"]
	if notesPkg == nil {
		t.Fatal("notes package (key \"notes\") is missing")
	}
	if notesPkg.Path != "notes" {
		t.Errorf("notes.Path = %q, want %q", notesPkg.Path, "notes")
	}
	if notesPkg.Name != "notegarupa" {
		t.Errorf("notes.Name = %q, want %q (package name ≠ directory name)", notesPkg.Name, "notegarupa")
	}
	if len(notesPkg.Sources) != 2 {
		t.Errorf("notes.Sources has %d entries, want 2 (tap.go + slide.go)", len(notesPkg.Sources))
	}

	notesTypes := collectTypeNames(notesPkg)
	if len(notesTypes) != 2 {
		t.Errorf("notes types = %v, want [TapNote, SlideNote]", notesTypes)
	}
	notesMethods := collectMethodCount(notesPkg)
	if notesMethods != 2 {
		t.Errorf("notes methods = %d, want 2", notesMethods)
	}

	// Stage package.
	stagePkg := pkgs["stage"]
	if stagePkg == nil {
		t.Fatal("stage package (key \"stage\") is missing")
	}
	if stagePkg.Path != "stage" {
		t.Errorf("stage.Path = %q, want %q", stagePkg.Path, "stage")
	}
	if stagePkg.Name != "stage" {
		t.Errorf("stage.Name = %q, want %q", stagePkg.Name, "stage")
	}

	// Unused directory should not have been loaded.
	if _, ok := pkgs["unused"]; ok {
		t.Error("unused directory was loaded but should not have been (not imported)")
	}
}

func TestLoadProject_FilterSkips(t *testing.T) {
	// Filter that skips everything except main.
	filter := func(path string) bool { return false }
	pkgs, err := LoadProject("testdata/complex", filter)
	if err != nil {
		t.Fatalf("LoadProject: %v", err)
	}

	if len(pkgs) != 1 {
		t.Fatalf("len(pkgs) = %d, want 1 (only main)", len(pkgs))
	}
	if pkgs[""] == nil {
		t.Fatal("main package is missing")
	}
}

func TestLoadProjectFromFiles(t *testing.T) {
	files := map[string]string{
		"main.go": `package main
type Skin struct { Note float64 }
func UpdateSpawn() float64 { return 0 }
`,
	}

	pkgs, err := LoadProjectFromFiles(files)
	if err != nil {
		t.Fatalf("LoadProjectFromFiles: %v", err)
	}

	if len(pkgs) != 1 {
		t.Fatalf("len(pkgs) = %d, want 1", len(pkgs))
	}
	mainPkg := pkgs[""]
	if mainPkg == nil {
		t.Fatal("main package missing")
	}
	if mainPkg.Name != "main" {
		t.Errorf("Name = %q, want %q", mainPkg.Name, "main")
	}
	if len(mainPkg.Sources) != 1 {
		t.Errorf("Sources has %d entries, want 1", len(mainPkg.Sources))
	}
}

func TestCollectGoFiles(t *testing.T) {
	files, err := CollectGoFiles("testdata/complex/notes")
	if err != nil {
		t.Fatalf("CollectGoFiles: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("len(files) = %d, want 2", len(files))
	}
	if _, ok := files["tap.go"]; !ok {
		t.Error("tap.go missing")
	}
	if _, ok := files["slide.go"]; !ok {
		t.Error("slide.go missing")
	}
}

func TestExtractImportPaths(t *testing.T) {
	files, err := CollectGoFiles("testdata/complex")
	if err != nil {
		t.Fatalf("CollectGoFiles: %v", err)
	}

	// No filter = all imports.
	paths, err := ExtractImportPaths(files)
	if err != nil {
		t.Fatalf("ExtractImportPaths: %v", err)
	}
	if len(paths) != 2 {
		t.Fatalf("len(paths) = %d, want 2 (%v)", len(paths), paths)
	}

	// Filter that skips "stage".
	paths2, err := ExtractImportPaths(files, func(path string) bool { return path != "stage" })
	if err != nil {
		t.Fatalf("ExtractImportPaths: %v", err)
	}
	if len(paths2) != 1 || paths2[0] != "notes" {
		t.Errorf("filtered paths = %v, want [notes]", paths2)
	}
}

func TestLoadProject_SingleFile(t *testing.T) {
	// Loading a single file that imports sub-packages resolves those imports
	// relative to the file's parent directory.
	pkgs, err := LoadProject("testdata/complex/engine.go", FilterStdlib, FilterNonSonolus)
	if err != nil {
		t.Fatalf("LoadProject: %v", err)
	}

	if len(pkgs) != 3 {
		t.Fatalf("len(pkgs) = %d, want 3 (main + notes + stage)", len(pkgs))
	}
	mainPkg := pkgs[""]
	if mainPkg.Name != "engine" {
		t.Errorf("Name = %q, want %q", mainPkg.Name, "engine")
	}
	if pkgs["notes"] == nil || pkgs["stage"] == nil {
		t.Error("imported packages not resolved")
	}
}

func TestLoadProject_SingleFile_NoImports(t *testing.T) {
	src := `package main
type Skin struct { Note float64 }
func UpdateSpawn() float64 { return 0 }
`
	pkgs, err := LoadProjectFromFiles(map[string]string{"engine.go": src})
	if err != nil {
		t.Fatalf("LoadProjectFromFiles: %v", err)
	}

	if len(pkgs) != 1 {
		t.Fatalf("len(pkgs) = %d, want 1", len(pkgs))
	}
	if pkgs[""].Name != "main" {
		t.Errorf("Name = %q, want %q", pkgs[""].Name, "main")
	}
}

func TestFilterStdlib(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"notes", true},
		{"stage", true},
		{"fmt", true},                    // bare name, not filtered
		{"github.com/foo/bar", false},    // dotted → external
		{"engine/sub", true},             // slash, no dot → local
	}
	for _, tt := range tests {
		got := FilterStdlib(tt.path)
		if got != tt.want {
			t.Errorf("FilterStdlib(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestFilterNonSonolus(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"sonolus", false},
		{"notes", true},
		{"fmt", true},
	}
	for _, tt := range tests {
		got := FilterNonSonolus(tt.path)
		if got != tt.want {
			t.Errorf("FilterNonSonolus(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

// ── helpers ──────────────────────────────────────────────────────────────────

func pkgKeys(pkgs map[string]*Package) []string {
	var keys []string
	for k := range pkgs {
		if k == "" {
			keys = append(keys, `""`)
		} else {
			keys = append(keys, k)
		}
	}
	return keys
}

func collectTypeNames(pkg *Package) []string {
	var names []string
	for _, f := range pkg.Files {
		for _, td := range f.Types {
			names = append(names, td.Name)
		}
	}
	return names
}

func collectFuncNames(pkg *Package) []string {
	var names []string
	for _, f := range pkg.Files {
		for _, fn := range f.Funcs {
			names = append(names, fn.Name)
		}
	}
	return names
}

func collectMethodCount(pkg *Package) int {
	n := 0
	for _, f := range pkg.Files {
		n += len(f.Methods)
	}
	return n
}
