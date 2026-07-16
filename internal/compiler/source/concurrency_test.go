package source

import (
	"fmt"
	"sync"
	"testing"
)

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
