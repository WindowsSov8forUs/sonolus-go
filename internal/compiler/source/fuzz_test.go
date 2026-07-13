package source

import (
	"go/ast"
	"go/token"
	"reflect"
	"testing"
)

func FuzzStaticValueEval(f *testing.F) {
	for _, seed := range []string{
		"1 + 2*3",
		"float32(0.1)",
		"[3]int{1, 2, 3}",
		"[]int{1, 2: 3}",
		`map[string]int{"a": 1}`,
		"struct{ A int }{A: 1}",
		"new(int)",
		"make([]int, 2, 3)",
		`len("hello")`,
		"min(3, 1, 2)",
		"(*int)(nil)",
		"func() {}",
		"[]int{1}[2]",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, expression string) {
		if len(expression) > 4096 {
			return
		}
		pkg, err := checkStaticPackage(map[string]string{
			"fuzz.go": "package main\nvar Value = " + expression + "\n",
		})
		if err != nil {
			return
		}

		tracer := NewASTTracer(pkg)
		first, firstErr := tracer.EvalPackageValue("Value")
		second, secondErr := tracer.EvalPackageValue("Value")
		if firstErr != nil || secondErr != nil {
			if firstErr == nil || secondErr == nil || firstErr.Error() != secondErr.Error() {
				t.Fatalf("nondeterministic errors: first=%v second=%v", firstErr, secondErr)
			}
			return
		}
		if firstDigest, secondDigest := staticValueDigest(first.Value), staticValueDigest(second.Value); firstDigest != secondDigest {
			t.Fatalf("nondeterministic values:\nfirst:  %s\nsecond: %s", firstDigest, secondDigest)
		}
	})
}
func FuzzTypeTrace(f *testing.F) {
	for _, seed := range []string{
		"type Item struct { Value int }",
		"type Node struct { Next *Node }",
		"type Box[T any] struct { Value T }; type Item struct { Box[int] }",
		"type Number interface { ~int | ~int64 }; type Item[T Number] struct { Value T }",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, declarations string) {
		if len(declarations) > 8192 {
			t.Skip()
		}
		pkg, err := checkStaticPackage(map[string]string{
			"fuzz.go": "package main\n" + declarations,
		})
		if err != nil {
			return
		}
		tracer := NewASTTracer(pkg)
		for _, file := range pkg.Syntax {
			for _, declaration := range file.Decls {
				general, ok := declaration.(*ast.GenDecl)
				if !ok || general.Tok != token.TYPE {
					continue
				}
				for _, specification := range general.Specs {
					typeSpec := specification.(*ast.TypeSpec)
					first := tracer.TraceTypeSpec(typeSpec)
					second := tracer.TraceTypeSpec(typeSpec)
					if (first == nil) != (second == nil) {
						t.Fatal("trace result changed between calls")
					}
					if first == nil || ExtractStructType(first.Root) == nil {
						continue
					}
					firstNames := projectedFieldNames(t, tracer, typeSpec)
					secondNames := projectedFieldNames(t, tracer, typeSpec)
					if !reflect.DeepEqual(firstNames, secondNames) {
						t.Fatalf("field order changed: %v != %v", firstNames, secondNames)
					}
				}
			}
		}
	})
}
