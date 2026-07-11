package source

import (
	"testing"

	sourcetracer "github.com/WindowsSov8forUs/sonolus-go/internal/newcompiler/source/tracer"
)

func TestDirectTracerPackage(t *testing.T) {
	pkg := mustStaticPackage(t, map[string]string{
		"main.go": `package main

type Pair struct { Value int }
var Value = Pair{Value: 7}
`,
	})

	direct := sourcetracer.NewASTTracer(pkg)
	var compatible *ASTTracer = direct
	if compatible.Package() != pkg {
		t.Fatal("source facade and tracer package use different ASTTracer types")
	}

	binding, err := direct.EvalPackageValue("Value")
	if err != nil {
		t.Fatalf("direct EvalPackageValue: %v", err)
	}
	if binding.Value.Kind != sourcetracer.StaticStruct || staticInt64(t, binding.Value.Fields[0].Value) != 7 {
		t.Fatalf("direct Value = %#v", binding.Value)
	}

	tree := direct.TraceTypeSpec(findTypeSpec(t, pkg, "Pair"))
	if tree == nil || tree.Root == nil {
		t.Fatal("direct TraceTypeSpec returned nil")
	}
	fields, err := sourcetracer.OrderedStructFields(ExtractStructType(tree.Root), tree.Root)
	if err != nil || len(fields) != 1 || fields[0].Names[0].Name != "Value" {
		t.Fatalf("direct OrderedStructFields = %#v, %v", fields, err)
	}
}
