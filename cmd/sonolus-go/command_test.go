package main

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/scaffold"
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime/debug"
	"strconv"
)

func TestParseMode(t *testing.T) {
	tests := []struct {
		input string
		want  Mode
		err   bool
	}{
		{"play", ModePlay, false},
		{"watch", ModeWatch, false},
		{"preview", ModePreview, false},
		{"tutorial", ModeTutorial, false},
		{"all", ModeAll, false},
		{"", "", true},
		{"unknown", "", true},
		{"PLAY", "", true},
	}
	for _, tt := range tests {
		got, err := ParseMode(tt.input)
		if tt.err && err == nil {
			t.Errorf("ParseMode(%q): expected error, got %v", tt.input, got)
		}
		if !tt.err && err != nil {
			t.Errorf("ParseMode(%q): unexpected error: %v", tt.input, err)
		}
		if !tt.err && got != tt.want {
			t.Errorf("ParseMode(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestModeExpand(t *testing.T) {
	if len(ModePlay.Expand()) != 1 || ModePlay.Expand()[0] != ModePlay {
		t.Error("single mode should expand to itself")
	}
	got := ModeAll.Expand()
	if len(got) != 4 {
		t.Fatalf("ModeAll.Expand() len = %d, want 4", len(got))
	}
	for i, m := range []Mode{ModePlay, ModeWatch, ModePreview, ModeTutorial} {
		if got[i] != m {
			t.Errorf("ModeAll.Expand()[%d] = %v, want %v", i, got[i], m)
		}
	}
}

func TestAllModeNames(t *testing.T) {
	names := allModeNames()
	if len(names) != 4 {
		t.Fatalf("allModeNames() len = %d, want 4", len(names))
	}
	for i, want := range []string{"play", "watch", "preview", "tutorial"} {
		if names[i] != want {
			t.Errorf("allModeNames()[%d] = %q, want %q", i, names[i], want)
		}
	}
}

func TestMode_String(t *testing.T) {
	tests := []struct {
		mode Mode
		want string
	}{
		{ModePlay, "play"},
		{ModeWatch, "watch"},
		{ModePreview, "preview"},
		{ModeTutorial, "tutorial"},
		{ModeAll, "all"},
	}
	for _, tt := range tests {
		if got := tt.mode.String(); got != tt.want {
			t.Errorf("Mode(%q).String() = %q, want %q", tt.mode, got, tt.want)
		}
	}
}

func TestParseOptLevel(t *testing.T) {
	tests := []struct {
		input   int
		want    compiler.OptimizationLevel
		wantErr bool
	}{
		{0, compiler.OptimizationMinimal, false},
		{1, compiler.OptimizationFast, false},
		{2, compiler.OptimizationStandard, false},
		{-1, 0, true},
		{3, 0, true},
	}
	for _, tt := range tests {
		got, err := parseOptLevel(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("parseOptLevel(%d) error = %v, wantErr %v", tt.input, err, tt.wantErr)
		}
		if !tt.wantErr && got != tt.want {
			t.Errorf("parseOptLevel(%d) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestParseRuntimeChecks(t *testing.T) {
	for _, test := range []struct {
		name string
		want compiler.RuntimeChecks
	}{
		{"none", compiler.RuntimeChecksNone},
		{"terminate", compiler.RuntimeChecksTerminate},
		{"notify", compiler.RuntimeChecksNotify},
	} {
		got, err := parseRuntimeChecks(test.name)
		if err != nil || got != test.want {
			t.Errorf("parseRuntimeChecks(%q) = %v, %v", test.name, got, err)
		}
	}
	if _, err := parseRuntimeChecks("invalid"); err == nil {
		t.Fatal("invalid runtime checks level was accepted")
	}
}

func TestRunCLIParsesSubcommandFlags(t *testing.T) {
	err := runCLI([]string{"build", "-o", "fixture", "-O", "3", "./testdata/multimode"})
	if err == nil || !strings.Contains(err.Error(), "invalid optimization level 3") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunCLIInitCommand(t *testing.T) {
	project := filepath.Join(t.TempDir(), "project")
	err := runCLI([]string{"mod", "init", "-sonolus-version", "v2.0.1", "example.com/project", project})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"go.mod", "go.sum", ".vscode/settings.json"} {
		if _, err := os.Stat(filepath.Join(project, filepath.FromSlash(name))); err != nil {
			t.Errorf("missing %s: %v", name, err)
		}
	}
	directory := filepath.Join(project, "engines", "first")
	if err := runCLI([]string{"init", directory}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"main.go", "play.go", "watch.go", "preview.go", "tutorial.go"} {
		if _, err := os.Stat(filepath.Join(directory, name)); err != nil {
			t.Errorf("missing %s: %v", name, err)
		}
	}
	if err := runCLI([]string{"init", "first", "second"}); err == nil || !strings.Contains(err.Error(), "at most one target directory") {
		t.Fatalf("extra argument error = %v", err)
	}
	if err := runCLI([]string{"mod", "init", "example.com/project", "first", "second"}); err == nil || !strings.Contains(err.Error(), "at most one project directory") {
		t.Fatalf("extra module argument error = %v", err)
	}
	if err := runCLI([]string{"mod", "init"}); err == nil || !strings.Contains(err.Error(), "requires a module name") {
		t.Fatalf("missing module name error = %v", err)
	}
	if err := runCLI([]string{"mod"}); err == nil || !strings.Contains(err.Error(), "requires the init subcommand") {
		t.Fatalf("missing mod subcommand error = %v", err)
	}
	if err := runCLI([]string{"mod", "init", "-module", "example.com/engine"}); err == nil || !strings.Contains(err.Error(), "flag provided but not defined") {
		t.Fatalf("legacy mod init module flag error = %v", err)
	}
	if err := runCLI([]string{"init", "-module", "example.com/engine"}); err == nil || !strings.Contains(err.Error(), "flag provided but not defined") {
		t.Fatalf("legacy init module flag error = %v", err)
	}

	single := filepath.Join(t.TempDir(), "single")
	if err := runCLI([]string{"mod", "init", "-sonolus-version", "v2.0.1", "example.com/single", single}); err != nil {
		t.Fatal(err)
	}
	if err := runCLI([]string{"init", single}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(single, "main.go")); err != nil {
		t.Fatal(err)
	}
}

func TestRunCLIWorkInitCommand(t *testing.T) {
	root := t.TempDir()
	module := filepath.Join(root, "sirius")
	if _, err := scaffold.InitModule(scaffold.ModuleOptions{Directory: module, ModulePath: "example.com/sirius"}); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	if err := runCLI([]string{"work", "init", "./sirius"}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "go.work"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "use ./sirius") {
		t.Fatalf("go.work = %q", data)
	}
	if err := runCLI([]string{"work"}); err == nil || !strings.Contains(err.Error(), "requires the init subcommand") {
		t.Fatalf("missing work subcommand error = %v", err)
	}
}

func TestRunCLIRejectsInvalidRuntimeChecks(t *testing.T) {
	err := runCLI([]string{"vet", "-runtime-checks", "invalid", "./testdata/multimode"})
	if err == nil || !strings.Contains(err.Error(), "invalid runtime checks") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDevConsoleDecodesCurrentSnapshot(t *testing.T) {
	server := &devServer{state: devServerState{artifacts: &compiler.Artifacts{Diagnostics: map[int]string{42: "meaning"}}}}
	var output bytes.Buffer
	runDevConsole(strings.NewReader("help\ndecode 42\ndecode 7\n"), &output, server)
	text := output.String()
	for _, expected := range []string{"commands: decode <code>, help", "42: meaning", "unknown diagnostic code 7"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("console output %q does not contain %q", text, expected)
		}
	}
}

func TestRunCLIDevCommand(t *testing.T) {
	err := runCLI([]string{"dev", "-unknown"})
	if err == nil || !strings.Contains(err.Error(), "flag provided but not defined: -unknown") {
		t.Fatalf("dev command was not parsed: %v", err)
	}

	err = runCLI([]string{"serve"})
	if err == nil || !strings.Contains(err.Error(), `unknown command "serve"`) {
		t.Fatalf("legacy serve command remains available: %v", err)
	}
}

func TestRunCLIVetCommand(t *testing.T) {
	err := runCLI([]string{"vet", "-m", "invalid"})
	if err == nil || !strings.Contains(err.Error(), "unknown mode: invalid") {
		t.Fatalf("vet command was not parsed: %v", err)
	}

	err = runCLI([]string{"vet", "-unknown"})
	if err == nil || !strings.Contains(err.Error(), "flag provided but not defined: -unknown") {
		t.Fatalf("vet flags were not parsed: %v", err)
	}
}

func TestRunCLIListCommandRejectsFlags(t *testing.T) {
	err := runCLI([]string{"list", "-m", "play"})
	if err == nil || !strings.Contains(err.Error(), "flag provided but not defined: -m") {
		t.Fatalf("list unexpectedly accepted flags: %v", err)
	}
}

func TestRunCLIRejectsLegacyCheckAndSchemaCommands(t *testing.T) {
	for _, command := range []string{"check", "schema"} {
		err := runCLI([]string{command})
		if err == nil || !strings.Contains(err.Error(), `unknown command "`+command+`"`) {
			t.Errorf("legacy command %q remains available: %v", command, err)
		}
	}
}

func TestCompilerMode(t *testing.T) {
	tests := []struct {
		mode Mode
		want compiler.Mode
	}{
		{ModePlay, compiler.ModePlay},
		{ModeWatch, compiler.ModeWatch},
		{ModePreview, compiler.ModePreview},
		{ModeTutorial, compiler.ModeTutorial},
		{Mode("unknown"), compiler.ModePlay},
	}
	for _, tt := range tests {
		if got := tt.mode.CompilerMode(); got != tt.want {
			t.Errorf("CompilerMode() = %v, want %v", got, tt.want)
		}
	}
}
func TestResolveBuildMetadataPrefersInjectedValues(t *testing.T) {
	info := &debug.BuildInfo{
		Main: debug.Module{Version: "v2.0.0-rc2"},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "fallback-commit"},
			{Key: "vcs.time", Value: "fallback-date"},
		},
	}

	got := resolveBuildMetadata("2.0.0-rc2", "release-commit", "release-date", info)
	want := buildMetadata{version: "2.0.0-rc2", commit: "release-commit", date: "release-date"}
	if got != want {
		t.Fatalf("resolveBuildMetadata() = %#v, want %#v", got, want)
	}
}

func TestResolveBuildMetadataUsesModuleVersion(t *testing.T) {
	info := &debug.BuildInfo{Main: debug.Module{Version: "v2.0.0-rc1"}}

	got := resolveBuildMetadata("dev", "unknown", "unknown", info)
	want := buildMetadata{version: "2.0.0-rc1", commit: "unknown", date: "unknown"}
	if got != want {
		t.Fatalf("resolveBuildMetadata() = %#v, want %#v", got, want)
	}
	if formatted := got.String(); formatted != "sonolus-go 2.0.0-rc1" {
		t.Fatalf("String() = %q", formatted)
	}
}

func TestResolveBuildMetadataUsesVCSSettings(t *testing.T) {
	info := &debug.BuildInfo{
		Main: debug.Module{Version: "(devel)"},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "local-commit"},
			{Key: "vcs.time", Value: "2026-07-13T00:00:00Z"},
		},
	}

	got := resolveBuildMetadata("dev", "unknown", "unknown", info)
	if formatted := got.String(); formatted != "sonolus-go dev (commit local-commit, built 2026-07-13T00:00:00Z)" {
		t.Fatalf("String() = %q", formatted)
	}
}

func TestBuildMetadataStringOmitsUnavailableFields(t *testing.T) {
	metadata := buildMetadata{version: "dev", commit: "local-commit", date: "unknown"}
	if got := metadata.String(); got != "sonolus-go dev (commit local-commit)" {
		t.Fatalf("String() = %q", got)
	}
}
func TestCLICompilerImportsStayAtPublicBoundary(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	allowed := map[string]bool{
		"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler": true,
	}
	for _, filename := range files {
		file, err := parser.ParseFile(token.NewFileSet(), filename, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", filename, err)
		}
		for _, spec := range file.Imports {
			path, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				t.Fatalf("unquote import in %s: %v", filename, err)
			}
			const compilerPrefix = "github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler"
			if len(path) >= len(compilerPrefix) && path[:len(compilerPrefix)] == compilerPrefix && !allowed[path] {
				t.Errorf("%s imports compiler implementation package %q", filename, path)
			}
		}
	}
}
