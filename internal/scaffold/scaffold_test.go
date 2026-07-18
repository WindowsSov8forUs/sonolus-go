package scaffold

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler"
)

func TestInitCreatesDeterministicModule(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "engine")
	result, err := Init(Options{
		Directory:         directory,
		ModulePath:        "example.com/engine",
		DependencyVersion: "v2.0.1",
	})
	if err != nil {
		t.Fatal(err)
	}
	wantFiles := []string{".gitignore", ".vscode/settings.json", "go.mod", "main.go", "play.go", "preview.go", "tutorial.go", "watch.go"}
	if !result.CreatedModule || result.ModulePath != "example.com/engine" || !reflect.DeepEqual(result.Files, wantFiles) {
		t.Fatalf("result = %#v, want module with files %v", result, wantFiles)
	}
	goModData, err := os.ReadFile(filepath.Join(directory, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	wantGoMod := "module example.com/engine\n\ngo 1.25.12\n\nrequire " + SonolusModulePath + " v2.0.1\n"
	if string(goModData) != wantGoMod {
		t.Fatalf("go.mod = %q, want %q", goModData, wantGoMod)
	}
	settings, err := os.ReadFile(filepath.Join(directory, ".vscode", "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(settings), `"buildFlags": ["-tags=play"]`) || !strings.Contains(string(settings), `"standaloneTags": ["ignore"]`) {
		t.Fatalf("settings.json = %s", settings)
	}
}

func TestInitReusesExistingModule(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/workspace\n\ngo 1.25.12\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(root, "engines", "first")
	result, err := Init(Options{Directory: directory})
	if err != nil {
		t.Fatal(err)
	}
	if result.CreatedModule || result.ModulePath != "" {
		t.Fatalf("result = %#v, want existing module", result)
	}
	if _, err := os.Stat(filepath.Join(directory, "go.mod")); !os.IsNotExist(err) {
		t.Fatalf("nested go.mod exists: %v", err)
	}
}

func TestInitAllowsModuleMetadataOnly(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "go.mod"), []byte("module example.com/engine\n\ngo 1.25.12\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(directory, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	result, err := Init(Options{Directory: directory})
	if err != nil {
		t.Fatal(err)
	}
	if result.CreatedModule {
		t.Fatal("existing module was recreated")
	}
	if _, err := os.Stat(filepath.Join(directory, "main.go")); err != nil {
		t.Fatal(err)
	}
}

func TestInitRejectsInvalidOrNonemptyTargets(t *testing.T) {
	if _, err := Init(Options{Directory: filepath.Join(t.TempDir(), "bad"), ModulePath: "bad path"}); err == nil || !strings.Contains(err.Error(), "invalid module path") {
		t.Fatalf("invalid module error = %v", err)
	}
	if _, err := Init(Options{Directory: filepath.Join(t.TempDir(), "bad-version"), ModulePath: "example.com/engine", DependencyVersion: "v3.0.0"}); err == nil || !strings.Contains(err.Error(), "expected v2 semantic version") {
		t.Fatalf("invalid version error = %v", err)
	}
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "existing.go"), []byte("package existing\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Init(Options{Directory: directory}); err == nil || !strings.Contains(err.Error(), "is not empty") {
		t.Fatalf("nonempty target error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(directory, "main.go")); !os.IsNotExist(err) {
		t.Fatalf("failed init wrote main.go: %v", err)
	}
}

func TestGeneratedEngineCompilesAllModes(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	directory, err := os.MkdirTemp(root, ".sonolus-engine-init-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(directory) })
	if _, err := Init(Options{Directory: directory}); err != nil {
		t.Fatal(err)
	}
	artifacts, err := compiler.NewCompiler(compiler.Options{}, directory).CompileAll()
	if err != nil {
		t.Fatal(err)
	}
	if artifacts.Play == nil || artifacts.Watch == nil || artifacts.Preview == nil || artifacts.Tutorial == nil {
		t.Fatalf("incomplete generated artifacts: %#v", artifacts)
	}
}
