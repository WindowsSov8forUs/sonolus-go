package source

import (
	"errors"
	"strings"
	"testing"
)

func TestLoadAndCrossPackageStaticValues(t *testing.T) {
	packages, err := Load("./testdata/staticmain")
	if err != nil {
		t.Fatalf("Load staticmain: %v", err)
	}
	if len(packages) != 1 {
		t.Fatalf("package count = %d, want 1", len(packages))
	}
	tracer := NewASTTracer(packages[0])

	for name, want := range map[string]int64{
		"CrossConst":   12,
		"CrossVar":     13,
		"CrossPointed": 12,
		"LocalGood":    7,
	} {
		if got := staticInt64(t, mustEvalBinding(t, tracer, name).Value); got != want {
			t.Fatalf("%s = %d, want %d", name, got, want)
		}
	}
	if pointer := mustEvalBinding(t, tracer, "CrossPointer").Value; pointer.Kind != StaticPointer || pointer.Pointer == nil {
		t.Fatalf("CrossPointer = %#v", pointer)
	}
	crossStruct := mustEvalBinding(t, tracer, "CrossStruct").Value
	if crossStruct.Kind != StaticStruct || staticInt64(t, crossStruct.Fields[0].Value) != 12 || staticString(t, crossStruct.Fields[1].Value) != "dependency" {
		t.Fatalf("CrossStruct = %#v", crossStruct)
	}

	for _, name := range []string{"CrossBad", "LocalBad"} {
		if _, err := tracer.EvalPackageValue(name); !errors.Is(err, ErrNotStatic) {
			t.Fatalf("%s error = %v, want ErrNotStatic", name, err)
		}
	}
}

func TestLoadPreservesImportValidation(t *testing.T) {
	if _, err := Load("./testdata/staticdep"); err == nil || !strings.Contains(err.Error(), "package is not main package") {
		t.Fatalf("non-main package error = %v", err)
	}
	if _, err := Load("./testdata/dotmain"); err == nil || !strings.Contains(err.Error(), "do not use dot import") {
		t.Fatalf("dot import error = %v", err)
	}
	if _, err := Load("./testdata/stdlibmain"); err == nil ||
		!strings.Contains(err.Error(), `could not import "fmt"`) ||
		!strings.Contains(err.Error(), "only embed, math, and math/rand are allowed") {
		t.Fatalf("standard library error = %v", err)
	}
	if _, err := Load("./testdata/thirdpartymain"); err == nil || !strings.Contains(err.Error(), "invalid third party lib") {
		t.Fatalf("third-party error = %v", err)
	}
}
