package compiler

import (
	"go/types"
	"reflect"
	"strings"
	"sync"
	"testing"

	"golang.org/x/tools/go/packages"

	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/frontend"
	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/ir"
	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/mode"
	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/source"
)

func loadInto(t *testing.T, parser *frontend.Parser, m mode.Mode, pattern string) {
	t.Helper()
	pkgs, err := source.LoadMode(m, pattern)
	if err != nil {
		t.Fatal(err)
	}
	if len(pkgs) != 1 {
		t.Fatalf("packages = %d, want 1", len(pkgs))
	}
	if err := parser.Parse(m, pkgs[0]); err != nil {
		t.Fatal(err)
	}
}

func TestParserBuildTagsAndAggregation(t *testing.T) {
	parser := frontend.NewParser()
	for _, m := range []mode.Mode{mode.ModePlay, mode.ModeWatch, mode.ModePreview, mode.ModeTutorial} {
		loadInto(t, parser, m, "./testdata/multimode")
	}
	project, err := parser.GetProject()
	if err != nil {
		t.Fatal(err)
	}
	want := map[mode.Mode]string{
		mode.ModePlay: "play.sprite", mode.ModeWatch: "watch.sprite",
		mode.ModePreview: "preview.sprite", mode.ModeTutorial: "tutorial.sprite",
	}
	var fields []*types.Var
	for _, m := range []mode.Mode{mode.ModePlay, mode.ModeWatch, mode.ModePreview, mode.ModeTutorial} {
		decl := project.Modes[m]
		if decl == nil || decl.Resources.Skin == nil || len(decl.Resources.Skin.Sprites) != 1 {
			t.Fatalf("%s skin = %#v", m, decl)
		}
		if got := string(decl.Resources.Skin.Sprites[0].Name); got != want[m] {
			t.Fatalf("%s sprite = %q, want %q", m, got, want[m])
		}
		for field := range decl.Resources.FieldIDs {
			fields = append(fields, field)
		}
	}
	if len(fields) != 4 {
		t.Fatalf("resource fields = %d, want 4", len(fields))
	}
	for i := range fields {
		for j := i + 1; j < len(fields); j++ {
			if fields[i] == fields[j] {
				t.Fatal("resource field object was reused across mode package graphs")
			}
		}
	}
	if len(project.Configuration.Options) != 1 || len(project.ROM) != 8 {
		t.Fatalf("shared outputs: configuration=%#v ROM=%v", project.Configuration, project.ROM)
	}
}

func TestParserRejectsDuplicateMode(t *testing.T) {
	parser := frontend.NewParser()
	pkgs, err := source.LoadMode(mode.ModePlay, "./testdata/multimode")
	if err != nil || len(pkgs) != 1 {
		t.Fatalf("load: packages=%d error=%v", len(pkgs), err)
	}
	if err := parser.Parse(mode.ModePlay, pkgs[0]); err != nil {
		t.Fatal(err)
	}
	if err := parser.Parse(mode.ModePlay, pkgs[0]); err == nil || !strings.Contains(err.Error(), "already been parsed") {
		t.Fatalf("duplicate mode error = %v", err)
	}
}

func TestParserInputValidation(t *testing.T) {
	parser := frontend.NewParser()
	if _, err := parser.GetProject(); err == nil || !strings.Contains(err.Error(), "no Sonolus modes") {
		t.Fatalf("empty project error = %v", err)
	}
	if err := parser.Parse(mode.Mode("invalid"), nil); err == nil || !strings.Contains(err.Error(), "invalid Sonolus mode") {
		t.Fatalf("invalid mode error = %v", err)
	}
	if err := parser.Parse(mode.ModePlay, nil); err == nil || !strings.Contains(err.Error(), "package is nil") {
		t.Fatalf("nil package error = %v", err)
	}
}

func TestParserRejectsConfigurationMismatch(t *testing.T) {
	parser := frontend.NewParser()
	loadInto(t, parser, mode.ModePlay, "./testdata/configurationmismatch")
	loadInto(t, parser, mode.ModeWatch, "./testdata/configurationmismatch")
	_, err := parser.GetProject()
	if err == nil || !strings.Contains(err.Error(), "configuration.options[0].def") || !strings.Contains(err.Error(), "play") || !strings.Contains(err.Error(), "watch") {
		t.Fatalf("configuration mismatch error = %v", err)
	}
}

func TestParserRejectsROMMismatch(t *testing.T) {
	parser := frontend.NewParser()
	loadInto(t, parser, mode.ModePlay, "./testdata/rommismatch")
	loadInto(t, parser, mode.ModeWatch, "./testdata/rommismatch")
	_, err := parser.GetProject()
	if err == nil || !strings.Contains(err.Error(), "first differing byte at offset") || !strings.Contains(err.Error(), "play") || !strings.Contains(err.Error(), "watch") {
		t.Fatalf("ROM mismatch error = %v", err)
	}
}

func TestParserTreatsMissingAndExplicitEmptySharedValuesAsEqual(t *testing.T) {
	parser := frontend.NewParser()
	loadInto(t, parser, mode.ModePlay, "./testdata/emptyshared")
	loadInto(t, parser, mode.ModeWatch, "./testdata/emptyshared")
	project, err := parser.GetProject()
	if err != nil {
		t.Fatal(err)
	}
	if len(project.Configuration.Options) != 0 || len(project.ROM) != 0 {
		t.Fatalf("shared outputs: configuration=%#v ROM=%v", project.Configuration, project.ROM)
	}
}

func TestParserFailureDoesNotCommitMode(t *testing.T) {
	parser := frontend.NewParser()
	invalid, err := source.LoadMode(mode.ModePlay, "./testdata/invalidphase")
	if err != nil || len(invalid) != 1 {
		t.Fatalf("load invalid package: packages=%d error=%v", len(invalid), err)
	}
	if err := parser.Parse(mode.ModePlay, invalid[0]); err == nil {
		t.Fatal("invalid callback unexpectedly parsed")
	}
	valid, err := source.LoadMode(mode.ModePlay, "./testdata/lowering")
	if err != nil || len(valid) != 1 {
		t.Fatalf("load valid package: packages=%d error=%v", len(valid), err)
	}
	if err := parser.Parse(mode.ModePlay, valid[0]); err != nil {
		t.Fatalf("retry after failed parse: %v", err)
	}
	project, err := parser.GetProject()
	if err != nil || project.Modes[mode.ModePlay] == nil {
		t.Fatalf("project after retry: project=%#v error=%v", project, err)
	}
}

func TestParserParsesModesConcurrently(t *testing.T) {
	patterns := map[mode.Mode]string{
		mode.ModePlay:     "./testdata/lowering",
		mode.ModeWatch:    "./testdata/lowering_watch",
		mode.ModePreview:  "./testdata/lowering_preview",
		mode.ModeTutorial: "./testdata/lowering_tutorial",
	}
	loaded := make(map[mode.Mode]*packages.Package, len(patterns))
	for m, pattern := range patterns {
		pkgs, err := source.LoadMode(m, pattern)
		if err != nil || len(pkgs) != 1 {
			t.Fatalf("load %s: packages=%d error=%v", m, len(pkgs), err)
		}
		loaded[m] = pkgs[0]
	}
	parser := frontend.NewParser()
	errs := make(chan error, len(loaded))
	var wg sync.WaitGroup
	for m, pkg := range loaded {
		wg.Add(1)
		go func(m mode.Mode, pkg *packages.Package) {
			defer wg.Done()
			errs <- parser.Parse(m, pkg)
		}(m, pkg)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	project, err := parser.GetProject()
	if err != nil || len(project.Modes) != 4 {
		t.Fatalf("concurrent project: modes=%d error=%v", len(project.Modes), err)
	}
}

func TestCallbackIRIsDeterministicAcrossPackageGraphs(t *testing.T) {
	parse := func() *ir.Function {
		decl, err := parseMode(mode.ModePlay, "./testdata/lowering")
		if err != nil {
			t.Fatal(err)
		}
		return callbackByName(t, decl.Archetypes[0].Callbacks, "preprocess")
	}
	if first, second := parse(), parse(); !reflect.DeepEqual(first, second) {
		t.Fatal("callback IR differs across independent package graphs")
	}
}

func TestParserComparesROMByFinalBytes(t *testing.T) {
	parser := frontend.NewParser()
	loadInto(t, parser, mode.ModePlay, "./testdata/romequivalent")
	loadInto(t, parser, mode.ModeWatch, "./testdata/romequivalent")
	project, err := parser.GetProject()
	if err != nil {
		t.Fatal(err)
	}
	if string(project.ROM) != "ABC\n" {
		t.Fatalf("ROM = %v", project.ROM)
	}
}
