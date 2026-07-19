package scaffold

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler"
)

func TestInitModuleCreatesDeterministicProject(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "project")
	result, err := InitModule(ModuleOptions{
		Directory:         directory,
		ModulePath:        "example.com/project",
		DependencyVersion: "v2.0.1",
	})
	if err != nil {
		t.Fatal(err)
	}
	wantFiles := []string{".gitignore", ".vscode/settings.json", "go.mod", "go.sum"}
	if result.ModulePath != "example.com/project" || !reflect.DeepEqual(result.Files, wantFiles) {
		t.Fatalf("result = %#v, want files %v", result, wantFiles)
	}
	goModData, err := os.ReadFile(filepath.Join(directory, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	wantGoMod := "module example.com/project\n\ngo 1.25.12\n\nrequire " + SonolusModulePath + " v2.0.1\n"
	if string(goModData) != wantGoMod {
		t.Fatalf("go.mod = %q, want %q", goModData, wantGoMod)
	}
	goSumData, err := os.ReadFile(filepath.Join(directory, "go.sum"))
	if err != nil || len(goSumData) != 0 {
		t.Fatalf("go.sum = %q, err = %v", goSumData, err)
	}
	settings, err := os.ReadFile(filepath.Join(directory, ".vscode", "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(settings), `"buildFlags": ["-tags=play"]`) || !strings.Contains(string(settings), `"standaloneTags": ["ignore"]`) || !strings.Contains(string(settings), `"SA4017": false`) {
		t.Fatalf("settings.json = %s", settings)
	}
}

func TestInitWorkspaceCreatesDeterministicWorkFile(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"sirius", "shared"} {
		if _, err := InitModule(ModuleOptions{Directory: filepath.Join(root, name), ModulePath: "example.com/" + name}); err != nil {
			t.Fatal(err)
		}
	}
	result, err := InitWorkspace(WorkspaceOptions{
		Directory:         root,
		ModuleDirectories: []string{"./sirius", "./shared"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(result.Modules, []string{"./sirius", "./shared"}) || !reflect.DeepEqual(result.Files, []string{"go.work"}) {
		t.Fatalf("result = %#v", result)
	}
	data, err := os.ReadFile(filepath.Join(root, "go.work"))
	if err != nil {
		t.Fatal(err)
	}
	want := "go 1.25.12\n\nuse (\n\t./shared\n\t./sirius\n)\n"
	if string(data) != want {
		t.Fatalf("go.work = %q, want %q", data, want)
	}
	if _, err := os.Stat(filepath.Join(root, "go.work.sum")); !os.IsNotExist(err) {
		t.Fatalf("work init unexpectedly created go.work.sum: %v", err)
	}
}

func TestInitWorkspaceRejectsInvalidModulesAndExistingWorkFile(t *testing.T) {
	root := t.TempDir()
	if _, err := InitWorkspace(WorkspaceOptions{Directory: root, ModuleDirectories: []string{"./missing"}}); err == nil || !strings.Contains(err.Error(), "inspect workspace module") {
		t.Fatalf("missing module error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "go.work")); !os.IsNotExist(err) {
		t.Fatalf("failed work init wrote go.work: %v", err)
	}
	if _, err := InitModule(ModuleOptions{Directory: filepath.Join(root, "sirius"), ModulePath: "example.com/sirius"}); err != nil {
		t.Fatal(err)
	}
	if _, err := InitWorkspace(WorkspaceOptions{Directory: root, ModuleDirectories: []string{"./sirius"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := InitWorkspace(WorkspaceOptions{Directory: root}); err == nil || !strings.Contains(err.Error(), "create \"go.work\"") {
		t.Fatalf("existing workspace error = %v", err)
	}
}

func TestInitCreatesEngineBelowModuleRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "project")
	if _, err := InitModule(ModuleOptions{Directory: root, ModulePath: "example.com/project"}); err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(root, "engines", "first")
	result, err := Init(Options{Directory: directory})
	if err != nil {
		t.Fatal(err)
	}
	wantFiles := []string{"main.go", "play.go", "preview.go", "tutorial.go", "watch.go"}
	if result.ModuleRoot != root || !reflect.DeepEqual(result.Files, wantFiles) {
		t.Fatalf("result = %#v, want module root %q and files %v", result, root, wantFiles)
	}
	for _, name := range []string{"go.mod", "go.sum", ".vscode/settings.json"} {
		if _, err := os.Stat(filepath.Join(directory, filepath.FromSlash(name))); !os.IsNotExist(err) {
			t.Fatalf("engine package unexpectedly contains %s: %v", name, err)
		}
	}
}

func TestInitCreatesSingleEngineAtModuleRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "project")
	if _, err := InitModule(ModuleOptions{Directory: root, ModulePath: "example.com/project"}); err != nil {
		t.Fatal(err)
	}
	result, err := Init(Options{Directory: root})
	if err != nil {
		t.Fatal(err)
	}
	if result.Directory != root || result.ModuleRoot != root {
		t.Fatalf("result = %#v, want single engine at %q", result, root)
	}
	for _, name := range []string{"go.mod", "go.sum", ".vscode/settings.json", "main.go", "play.go", "watch.go", "preview.go", "tutorial.go"} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(name))); err != nil {
			t.Errorf("missing %s: %v", name, err)
		}
	}
}

func TestInitModuleAllowsGitOnly(t *testing.T) {
	directory := t.TempDir()
	if err := os.Mkdir(filepath.Join(directory, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := InitModule(ModuleOptions{Directory: directory, ModulePath: "example.com/project"}); err != nil {
		t.Fatal(err)
	}
}

func TestInitRejectsMissingOrMisplacedModuleMetadata(t *testing.T) {
	withoutModule := filepath.Join(t.TempDir(), "engine")
	if _, err := Init(Options{Directory: withoutModule}); err == nil || !strings.Contains(err.Error(), "run sonolus-go mod init") {
		t.Fatalf("missing module error = %v", err)
	}

	for _, missing := range []string{"go.mod", "go.sum"} {
		root := t.TempDir()
		present := "go.mod"
		content := []byte("module example.com/project\n\ngo 1.25.12\n")
		if missing == "go.mod" {
			present = "go.sum"
			content = nil
		}
		if err := os.WriteFile(filepath.Join(root, present), content, 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := Init(Options{Directory: filepath.Join(root, "engine")})
		if err == nil || !strings.Contains(err.Error(), "missing "+missing) {
			t.Errorf("missing %s error = %v", missing, err)
		}
	}

	invalidRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(invalidRoot, "go.mod"), []byte("not a module file\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(invalidRoot, "go.sum"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Init(Options{Directory: filepath.Join(invalidRoot, "engine")}); err == nil || !strings.Contains(err.Error(), "invalid module file") {
		t.Fatalf("invalid go.mod error = %v", err)
	}

	nonRegularRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(nonRegularRoot, "go.mod"), []byte("module example.com/project\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(nonRegularRoot, "go.sum"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Init(Options{Directory: filepath.Join(nonRegularRoot, "engine")}); err == nil || !strings.Contains(err.Error(), "must be regular files") {
		t.Fatalf("non-regular metadata error = %v", err)
	}

}

func TestInitRejectsInvalidOrNonemptyTargets(t *testing.T) {
	if _, err := InitModule(ModuleOptions{Directory: filepath.Join(t.TempDir(), "missing-name")}); err == nil || !strings.Contains(err.Error(), "module path is required") {
		t.Fatalf("missing module path error = %v", err)
	}
	if _, err := InitModule(ModuleOptions{Directory: filepath.Join(t.TempDir(), "bad"), ModulePath: "bad path"}); err == nil || !strings.Contains(err.Error(), "invalid module path") {
		t.Fatalf("invalid module error = %v", err)
	}
	if _, err := InitModule(ModuleOptions{Directory: filepath.Join(t.TempDir(), "bad-version"), ModulePath: "example.com/project", DependencyVersion: "v3.0.0"}); err == nil || !strings.Contains(err.Error(), "expected v2 semantic version") {
		t.Fatalf("invalid version error = %v", err)
	}
	moduleDirectory := t.TempDir()
	if err := os.WriteFile(filepath.Join(moduleDirectory, "existing.txt"), []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := InitModule(ModuleOptions{Directory: moduleDirectory, ModulePath: "example.com/project"}); err == nil || !strings.Contains(err.Error(), "only .git") {
		t.Fatalf("nonempty module target error = %v", err)
	}

	root := filepath.Join(t.TempDir(), "project")
	if _, err := InitModule(ModuleOptions{Directory: root, ModulePath: "example.com/project"}); err != nil {
		t.Fatal(err)
	}
	engineDirectory := filepath.Join(root, "engine")
	if err := os.Mkdir(engineDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(engineDirectory, "existing.go"), []byte("package existing\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Init(Options{Directory: engineDirectory}); err == nil || !strings.Contains(err.Error(), "is not empty") {
		t.Fatalf("nonempty engine target error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(engineDirectory, "main.go")); !os.IsNotExist(err) {
		t.Fatalf("failed init wrote main.go: %v", err)
	}

	singleRoot := filepath.Join(t.TempDir(), "project")
	if _, err := InitModule(ModuleOptions{Directory: singleRoot, ModulePath: "example.com/project"}); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(singleRoot, "shared"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Init(Options{Directory: singleRoot}); err == nil || !strings.Contains(err.Error(), "initialize engine packages in subdirectories") {
		t.Fatalf("nonempty module root error = %v", err)
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
