package source

import (
	"fmt"
	"strings"
	"sync"
	"testing"
)

func TestLoadAndCrossPackageStaticValues(t *testing.T) {
	packages, err := Load("../testdata/source/staticmain")
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
		value := mustEvalBinding(t, tracer, name).Value
		if value.Kind != StaticFunctionCall || value.Call == nil || value.Call.Object.Name() != "dynamic" {
			t.Fatalf("%s = %#v, want symbolic dynamic call", name, value)
		}
	}
}

func TestLoadPreservesImportValidation(t *testing.T) {
	if _, err := Load("../testdata/source/staticdep"); err == nil || !strings.Contains(err.Error(), "package is not main package") {
		t.Fatalf("non-main package error = %v", err)
	}
	if _, err := Load("../testdata/source/dotmain"); err == nil || !strings.Contains(err.Error(), "do not use dot import") {
		t.Fatalf("dot import error = %v", err)
	}
	if _, err := Load("../testdata/source/stdlibmain"); err == nil ||
		!strings.Contains(err.Error(), `could not import "fmt"`) ||
		!strings.Contains(err.Error(), "only embed, math, and math/rand are allowed") {
		t.Fatalf("standard library error = %v", err)
	}
	if _, err := Load("../testdata/source/thirdpartymain"); err == nil || !strings.Contains(err.Error(), "invalid third party lib") {
		t.Fatalf("third-party error = %v", err)
	}
}
func TestASTTracerConcurrentEntryPoints(t *testing.T) {
	pkg := mustStaticPackage(t, map[string]string{
		"main.go": `package main

type Item struct {
	Value int
}

var First, Second = Item{Value: 1}, []int{2, 3}
`,
	})
	tracer := NewASTTracer(pkg)
	valueSpec := findValueSpec(t, pkg, "First")
	typeSpec := findTypeSpec(t, pkg, "Item")
	wantDigest := staticValueDigest(mustEvalBinding(t, tracer, "Second").Value)

	const goroutines = 24
	const iterations = 50
	errors := make(chan error, goroutines)
	var waitGroup sync.WaitGroup
	for worker := 0; worker < goroutines; worker++ {
		worker := worker
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			for iteration := 0; iteration < iterations; iteration++ {
				switch worker % 3 {
				case 0:
					binding, err := tracer.EvalPackageValue("Second")
					if err != nil {
						errors <- err
						return
					}
					if got := staticValueDigest(binding.Value); got != wantDigest {
						errors <- fmt.Errorf("digest changed: %s", got)
						return
					}
				case 1:
					bindings, err := tracer.EvalValueSpec(valueSpec)
					if err != nil || len(bindings) != 2 {
						errors <- fmt.Errorf("EvalValueSpec: count=%d err=%w", len(bindings), err)
						return
					}
				case 2:
					if tree := tracer.TraceTypeSpec(typeSpec); tree == nil || tree.Root == nil {
						errors <- fmt.Errorf("TraceTypeSpec returned nil")
						return
					}
				}
			}
		}()
	}
	waitGroup.Wait()
	close(errors)
	for err := range errors {
		t.Error(err)
	}
}

func TestASTTracerInstancesAreIsolated(t *testing.T) {
	pkg := mustStaticPackage(t, map[string]string{
		"main.go": `package main

var Value = []int{1, 2}
`,
	})
	first := mustEvalBinding(t, NewASTTracer(pkg), "Value")
	second := mustEvalBinding(t, NewASTTracer(pkg), "Value")
	if first.Storage == second.Storage {
		t.Fatal("package storage leaked between tracer instances")
	}
	if first.Value.Slice == nil || second.Value.Slice == nil || first.Value.Slice.Backing == second.Value.Slice.Backing {
		t.Fatal("allocated object leaked between tracer instances")
	}
	if first.Storage.ID != second.Storage.ID || first.Value.Slice.Backing.ID != second.Value.Slice.Backing.ID {
		t.Fatal("isolated tracers did not allocate deterministic instance-local IDs")
	}
}
