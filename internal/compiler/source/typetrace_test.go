package source

import (
	"go/ast"
	"reflect"
	"testing"
)

func projectedFieldNames(t testing.TB, tracer *ASTTracer, typeSpec *ast.TypeSpec) []string {
	t.Helper()
	tree := tracer.TraceTypeSpec(typeSpec)
	if tree == nil || tree.Root == nil {
		t.Fatalf("TraceTypeSpec(%s) returned nil", typeSpec.Name.Name)
	}
	structure := ExtractStructType(tree.Root)
	if structure == nil {
		t.Fatalf("ExtractStructType(%s) returned nil", typeSpec.Name.Name)
	}
	fields, err := StructFieldFilter(structure, tree.Root, nil)
	if err != nil {
		t.Fatalf("StructFieldFilter(%s): %v", typeSpec.Name.Name, err)
	}
	names := make([]string, len(fields))
	for index, field := range fields {
		if len(field.Names) != 1 {
			t.Fatalf("field %d has %d names", index, len(field.Names))
		}
		names[index] = field.Names[0].Name
	}
	return names
}

func TestTraceTypeSpecRecursiveAliasGenericAndConstraint(t *testing.T) {
	pkg := mustStaticPackage(t, map[string]string{
		"types.go": `package main

type T int

type Node struct {
	Next *Node
}

type Base struct {
	Value int
}

type Alias = Base
type Defined Base

type Box[T any] struct {
	Value T
}

type Number interface {
	~int | ~int64
}

type Generic[T Number] struct {
	Value T
}
`,
	})
	tracer := NewASTTracer(pkg)

	nodeSpec := findTypeSpec(t, pkg, "Node")
	nodeTree := tracer.TraceTypeSpec(nodeSpec)
	if nodeTree == nil || nodeTree.Root == nil {
		t.Fatal("recursive Node trace returned nil")
	}
	foundSelf := false
	for ident, child := range nodeTree.Root.Children {
		if ident.Name == "Node" && child == nodeTree.Root {
			foundSelf = true
		}
	}
	if !foundSelf {
		t.Fatal("recursive Node reference did not point to the cached root node")
	}

	for _, name := range []string{"Alias", "Defined"} {
		typeSpec := findTypeSpec(t, pkg, name)
		if !IsStruct(typeSpec, pkg) {
			t.Fatalf("IsStruct(%s) = false", name)
		}
		tree := tracer.TraceTypeSpec(typeSpec)
		if tree == nil || ExtractStructType(tree.Root) == nil {
			t.Fatalf("%s did not resolve to Base's struct declaration", name)
		}
	}

	boxTree := tracer.TraceTypeSpec(findTypeSpec(t, pkg, "Box"))
	if boxTree == nil {
		t.Fatal("generic Box trace returned nil")
	}
	for ident, child := range boxTree.Root.Children {
		if ident.Name == "T" && child != nil {
			t.Fatal("type parameter T was confused with the package type T")
		}
	}
	if tree := tracer.TraceTypeSpec(findTypeSpec(t, pkg, "Generic")); tree == nil {
		t.Fatal("type-set constraint trace returned nil")
	}
	if got := TypeSpecString(findTypeSpec(t, pkg, "Generic")); got != "type Generic[T Number] struct{ Value T }" {
		t.Fatalf("TypeSpecString(Generic) = %q", got)
	}
}

func TestStructFieldProjectionUsesGoPromotionRulesAndStableOrder(t *testing.T) {
	pkg := mustStaticPackage(t, map[string]string{
		"fields.go": `package main

type Number int

type Inner struct {
	X int
	Y int
}

type Outer struct {
	Inner
	X string
	Z int
}

type Holder struct {
	Number
}

type Leaf struct {
	A int
}

type Left struct { Leaf }
type Right struct { Leaf }
type Diamond struct {
	Left
	Right
}

type Deep struct { Leaf }
type Shallow struct { A int }
type Root struct {
	Deep
	Shallow
}

type Box[T any] struct { Value T }
type Wrapped struct {
	Box[int]
	Local int
}

type Grouped struct { B, A int }
`,
	})
	tracer := NewASTTracer(pkg)
	tests := []struct {
		name string
		want []string
	}{
		{name: "Outer", want: []string{"Inner", "X", "Z", "Y"}},
		{name: "Holder", want: []string{"Number"}},
		{name: "Diamond", want: []string{"Left", "Right"}},
		{name: "Root", want: []string{"Deep", "Shallow", "Leaf", "A"}},
		{name: "Wrapped", want: []string{"Box", "Local", "Value"}},
		{name: "Grouped", want: []string{"B", "A"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			typeSpec := findTypeSpec(t, pkg, test.name)
			for iteration := 0; iteration < 50; iteration++ {
				if got := projectedFieldNames(t, tracer, typeSpec); !reflect.DeepEqual(got, test.want) {
					t.Fatalf("projection = %v, want %v", got, test.want)
				}
			}
		})
	}

	numberTree := tracer.TraceTypeSpec(findTypeSpec(t, pkg, "Number"))
	if numberTree == nil {
		t.Fatal("Number trace returned nil")
	}
	if structure := ExtractStructType(numberTree.Root); structure != nil {
		t.Fatalf("ExtractStructType(Number) = %#v", structure)
	}
}

func TestTraceTypeSpecAcrossPackageAndRenamedImport(t *testing.T) {
	packages, err := Load("./testdata/staticmain")
	if err != nil {
		t.Fatalf("Load staticmain: %v", err)
	}
	pkg := packages[0]
	typeSpec := findTypeSpec(t, pkg, "CrossType")
	got := projectedFieldNames(t, NewASTTracer(pkg), typeSpec)
	want := []string{"Pair", "Local", "Number", "Text"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("cross-package projection = %v, want %v", got, want)
	}
}
