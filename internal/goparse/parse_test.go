package goparse

import (
	"testing"
)

func TestParseFile_BasicStruct(t *testing.T) {
	src := `package test
type Note struct {
	Beat float64 ` + "`" + `sonolus:"imported"` + "`" + `
	X    float64 ` + "`" + `sonolus:"memory"` + "`" + `
}
`

	pkg, err := ParseFile(src)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if pkg.Name != "test" {
		t.Errorf("pkg.Name = %q, want %q", pkg.Name, "test")
	}
	if len(pkg.Files) != 1 {
		t.Fatalf("len(pkg.Files) = %d, want 1", len(pkg.Files))
	}
	if pkg.Fset == nil {
		t.Error("pkg.Fset is nil")
	}

	f := pkg.Files[0]
	if f.Name != "engine.go" {
		t.Errorf("file name = %q, want %q", f.Name, "engine.go")
	}
	if len(f.Types) != 1 {
		t.Fatalf("len(f.Types) = %d, want 1", len(f.Types))
	}

	td := f.Types[0]
	if td.Name != "Note" {
		t.Errorf("type name = %q, want %q", td.Name, "Note")
	}
	if len(td.Fields) != 2 {
		t.Fatalf("len(td.Fields) = %d, want 2", len(td.Fields))
	}
	if td.Fields[0].Names[0] != "Beat" {
		t.Errorf("field[0].name = %q, want %q", td.Fields[0].Names[0], "Beat")
	}
	if td.Fields[1].Names[0] != "X" {
		t.Errorf("field[1].name = %q, want %q", td.Fields[1].Names[0], "X")
	}

	// Tag is preserved as raw string.
	if td.Fields[0].Tag == "" {
		t.Error("field[0].tag is empty, expected sonolus tag")
	}
}

func TestParseFile_MethodsAndFunctions(t *testing.T) {
	src := `package p
type Note struct {
	Beat float64
}
func (n *Note) Initialize() { debugPause() }
func UpdateSpawn() float64 { return 0 }
`

	pkg, err := ParseFile(src)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}

	f := pkg.Files[0]

	// Methods.
	if len(f.Methods) != 1 {
		t.Fatalf("len(f.Methods) = %d, want 1", len(f.Methods))
	}
	m := f.Methods[0]
	if m.ReceiverType != "Note" {
		t.Errorf("receiver type = %q, want %q", m.ReceiverType, "Note")
	}
	if m.ReceiverName != "n" {
		t.Errorf("receiver name = %q, want %q", m.ReceiverName, "n")
	}
	if m.MethodName != "Initialize" {
		t.Errorf("method name = %q, want %q", m.MethodName, "Initialize")
	}
	if m.Body == nil {
		t.Error("method body is nil")
	}

	// Free functions.
	if len(f.Funcs) != 1 {
		t.Fatalf("len(f.Funcs) = %d, want 1", len(f.Funcs))
	}
	fn := f.Funcs[0]
	if fn.Name != "UpdateSpawn" {
		t.Errorf("func name = %q, want %q", fn.Name, "UpdateSpawn")
	}
	if fn.Body == nil {
		t.Error("func body is nil")
	}
}

func TestParseFile_PointerReceiver(t *testing.T) {
	src := `package p
type Note struct { Beat float64 }
func (n *Note) Initialize() {}
`

	pkg, err := ParseFile(src)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}

	m := pkg.Files[0].Methods[0]
	if m.ReceiverType != "Note" {
		t.Errorf("receiver type = %q, want %q (pointer stripped)", m.ReceiverType, "Note")
	}
}

func TestParseFile_VarDeclarations(t *testing.T) {
	src := `package p
var ui = UI{PrimaryMetric: MetricArcade}
var x int = 42
`

	pkg, err := ParseFile(src)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}

	f := pkg.Files[0]
	if len(f.Vars) != 2 {
		t.Fatalf("len(f.Vars) = %d, want 2", len(f.Vars))
	}

	if f.Vars[0].Names[0] != "ui" {
		t.Errorf("var[0].name = %q, want %q", f.Vars[0].Names[0], "ui")
	}
	if f.Vars[0].Type != "" {
		t.Errorf("var[0].type = %q, want empty (inferred)", f.Vars[0].Type)
	}
	if len(f.Vars[0].Values) != 1 {
		t.Error("var[0] should have 1 value")
	}

	if f.Vars[1].Names[0] != "x" {
		t.Errorf("var[1].name = %q, want %q", f.Vars[1].Names[0], "x")
	}
	if f.Vars[1].Type != "int" {
		t.Errorf("var[1].type = %q, want %q", f.Vars[1].Type, "int")
	}
}

func TestParseFile_VarWithExplicitType(t *testing.T) {
	src := `package p
var ui UI = UI{PrimaryMetric: MetricArcade}
`

	pkg, err := ParseFile(src)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}

	f := pkg.Files[0]
	if len(f.Vars) != 1 {
		t.Fatalf("len(f.Vars) = %d, want 1", len(f.Vars))
	}
	vd := f.Vars[0]
	if vd.Type != "UI" {
		t.Errorf("type = %q, want %q", vd.Type, "UI")
	}
	if len(vd.Values) != 1 {
		t.Errorf("len(values) = %d, want 1", len(vd.Values))
	}
}

func TestParseFiles_MultipleFiles(t *testing.T) {
	files := map[string]string{
		"engine.go": `package test
type Skin struct { Note float64 }
type Note struct { Beat float64 }
func (n *Note) Initialize() { debugPause() }
`,
		"helpers.go": `package test
type Hold struct { Beat float64 }
func (h *Hold) UpdateParallel(dt float64) { debugPause() }
func Preprocess() {}
`,
	}

	pkg, err := ParseFiles(files)
	if err != nil {
		t.Fatalf("ParseFiles: %v", err)
	}

	if pkg.Name != "test" {
		t.Errorf("pkg.Name = %q, want %q", pkg.Name, "test")
	}
	if len(pkg.Files) != 2 {
		t.Fatalf("len(pkg.Files) = %d, want 2", len(pkg.Files))
	}

	// Collect all types across files.
	var typeNames []string
	var methodNames []string
	var funcNames []string
	for _, f := range pkg.Files {
		for _, td := range f.Types {
			typeNames = append(typeNames, td.Name)
		}
		for _, md := range f.Methods {
			methodNames = append(methodNames, md.MethodName)
		}
		for _, fn := range f.Funcs {
			funcNames = append(funcNames, fn.Name)
		}
	}

	if len(typeNames) != 3 {
		t.Errorf("total types = %d, want 3 (%v)", len(typeNames), typeNames)
	}
	if len(methodNames) != 2 {
		t.Errorf("total methods = %d, want 2 (%v)", len(methodNames), methodNames)
	}
	if len(funcNames) != 1 {
		t.Errorf("total funcs = %d, want 1 (%v)", len(funcNames), funcNames)
	}
}

func TestParseFiles_ConflictingPackageNames(t *testing.T) {
	files := map[string]string{
		"a.go": `package foo
type A struct{}`,
		"b.go": `package bar
type B struct{}`,
	}

	_, err := ParseFiles(files)
	if err == nil {
		t.Fatal("expected error for conflicting package names, got nil")
	}
}

func TestParseFiles_EmptyFiles(t *testing.T) {
	_, err := ParseFiles(map[string]string{})
	if err == nil {
		t.Fatal("expected error for empty files map")
	}
}

func TestParseFile_MultipleTypesInGenDecl(t *testing.T) {
	src := `package p
type (
	A struct { X float64 }
	B struct { Y float64 }
)
`

	pkg, err := ParseFile(src)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}

	f := pkg.Files[0]
	if len(f.Types) != 2 {
		t.Fatalf("len(f.Types) = %d, want 2", len(f.Types))
	}
	if f.Types[0].Name != "A" {
		t.Errorf("type[0] = %q, want %q", f.Types[0].Name, "A")
	}
	if f.Types[1].Name != "B" {
		t.Errorf("type[1] = %q, want %q", f.Types[1].Name, "B")
	}
}

func TestParseFile_EmptyStruct(t *testing.T) {
	src := `package p
type Empty struct{}
`

	pkg, err := ParseFile(src)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}

	f := pkg.Files[0]
	if len(f.Types) != 1 {
		t.Fatalf("len(f.Types) = %d, want 1", len(f.Types))
	}
	if f.Types[0].Name != "Empty" {
		t.Errorf("name = %q, want %q", f.Types[0].Name, "Empty")
	}
	if len(f.Types[0].Fields) != 0 {
		t.Errorf("len(fields) = %d, want 0", len(f.Types[0].Fields))
	}
}

func TestParseFile_MultiNameField(t *testing.T) {
	src := `package p
type Vec struct {
	X, Y float64 ` + "`" + `json:"coords"` + "`" + `
}
`

	pkg, err := ParseFile(src)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}

	f := pkg.Files[0]
	td := f.Types[0]
	if len(td.Fields) != 1 {
		t.Fatalf("len(td.Fields) = %d, want 1", len(td.Fields))
	}
	if len(td.Fields[0].Names) != 2 {
		t.Fatalf("len(field.Names) = %d, want 2", len(td.Fields[0].Names))
	}
	if td.Fields[0].Names[0] != "X" {
		t.Errorf("names[0] = %q, want %q", td.Fields[0].Names[0], "X")
	}
	if td.Fields[0].Names[1] != "Y" {
		t.Errorf("names[1] = %q, want %q", td.Fields[0].Names[1], "Y")
	}
	if td.Fields[0].Tag != "`json:\"coords\"`" {
		t.Errorf("tag = %q", td.Fields[0].Tag)
	}
}

func TestParseFile_NonStructTypeSkipped(t *testing.T) {
	src := `package p
type ID int
type Alias = string
type Note struct { Beat float64 }
`

	pkg, err := ParseFile(src)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}

	f := pkg.Files[0]
	if len(f.Types) != 1 {
		t.Fatalf("len(f.Types) = %d, want 1 (non-struct types skipped)", len(f.Types))
	}
	if f.Types[0].Name != "Note" {
		t.Errorf("name = %q, want %q", f.Types[0].Name, "Note")
	}
}

func TestParseFile_SelectorTypeField(t *testing.T) {
	src := `package p
type Note struct {
	pos sonolus.Vec2 ` + "`" + `sonolus:"memory"` + "`" + `
}
`

	pkg, err := ParseFile(src)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}

	f := pkg.Files[0]
	td := f.Types[0]
	if len(td.Fields) != 1 {
		t.Fatalf("len(td.Fields) = %d, want 1", len(td.Fields))
	}
	if td.Fields[0].Type != "sonolus.Vec2" {
		t.Errorf("field type = %q, want %q", td.Fields[0].Type, "sonolus.Vec2")
	}
}

func TestParseFile_FunctionParams(t *testing.T) {
	src := `package p
func UpdateParallel(dt float64) {}
func Draw(id float64, x, y float64, z float64) {}
`

	pkg, err := ParseFile(src)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}

	f := pkg.Files[0]
	if len(f.Funcs) != 2 {
		t.Fatalf("len(f.Funcs) = %d, want 2", len(f.Funcs))
	}

	// Single param.
	fn1 := f.Funcs[0]
	if fn1.Name != "UpdateParallel" {
		t.Errorf("func[0].name = %q", fn1.Name)
	}
	if len(fn1.Params) != 1 {
		t.Fatalf("len(params) = %d, want 1", len(fn1.Params))
	}
	if fn1.Params[0].Names[0] != "dt" {
		t.Errorf("param.name = %q, want %q", fn1.Params[0].Names[0], "dt")
	}

	// Multi-param with shared type.
	fn2 := f.Funcs[1]
	if len(fn2.Params) != 3 {
		t.Fatalf("len(params) = %d, want 3", len(fn2.Params))
	}
	if len(fn2.Params[1].Names) != 2 {
		t.Errorf("param[1] names = %d, want 2 (x, y share type)", len(fn2.Params[1].Names))
	}
}

func TestParseFile_MethodWithoutBody(t *testing.T) {
	src := `package p
type Skin struct { Note float64 }
func (s Skin) Magic()
`

	pkg, err := ParseFile(src)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}

	f := pkg.Files[0]
	if len(f.Methods) != 1 {
		t.Fatalf("len(f.Methods) = %d, want 1", len(f.Methods))
	}
	if f.Methods[0].Body != nil {
		t.Error("method body should be nil for declaration without body")
	}
}
