package goparse

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
)

// ParseFiles parses a set of Go source files that belong to the same package
// and returns a merged *Package. Files are keyed by their base name (e.g.
// "engine.go"). All files must share the same package declaration name.
//
// The returned Package retains the *token.FileSet used during parsing so
// downstream consumers can produce diagnostics with source locations.
func ParseFiles(files map[string]string) (*Package, error) {
	if len(files) == 0 {
		return nil, fmt.Errorf("no source files provided")
	}

	fset := token.NewFileSet()
	out := &Package{Fset: fset}

	// Sort filenames for deterministic iteration and error messages.
	names := make([]string, 0, len(files))
	for n := range files {
		names = append(names, n)
	}
	sort.Strings(names)

	for _, name := range names {
		src := files[name]
		f, err := parser.ParseFile(fset, name, src, 0)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", name, err)
		}

		// Validate and capture package name.
		if out.Name == "" {
			out.Name = f.Name.Name
		} else if f.Name.Name != out.Name {
			return nil, fmt.Errorf("conflicting package names: %q uses %q, expected %q",
				name, f.Name.Name, out.Name)
		}

		fileIR := classifyDecls(f.Decls)
		fileIR.Name = name
		out.Files = append(out.Files, fileIR)
	}

	return out, nil
}

// ParseFile parses a single Go source string and returns a *Package. This is
// a convenience wrapper for tests and single-file engine sources.
func ParseFile(src string) (*Package, error) {
	return ParseFiles(map[string]string{"engine.go": src})
}

// classifyDecls walks the top-level declarations of a Go source file and
// classifies them into types, functions, methods, and variables. No Sonolus
// semantics are applied — this is pure Go AST classification.
func classifyDecls(decls []ast.Decl) *File {
	out := &File{}

	for _, decl := range decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			classifyGenDecl(out, d)
		case *ast.FuncDecl:
			classifyFuncDecl(out, d)
		}
	}

	return out
}

// classifyGenDecl classifies a general declaration (type, var, const, import).
// Imports are skipped — import resolution is the caller's responsibility.
func classifyGenDecl(out *File, d *ast.GenDecl) {
	switch d.Tok {
	case token.TYPE:
		for _, spec := range d.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				// Non-struct types (e.g. type X float64, type Y int) —
				// the engine compiler doesn't use them but we record them
				// as a struct with zero fields for completeness.
				continue
			}
			td := &TypeDecl{Name: ts.Name.Name}
			for _, f := range st.Fields.List {
				field := classifyField(f)
				td.Fields = append(td.Fields, field)
			}
			out.Types = append(out.Types, td)
		}
	case token.VAR:
		for _, spec := range d.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok || len(vs.Names) == 0 {
				continue
			}
			vd := &VarDecl{
				Values: vs.Values,
			}
			for _, n := range vs.Names {
				vd.Names = append(vd.Names, n.Name)
			}
			if vs.Type != nil {
				vd.Type = renderTypeExpr(vs.Type)
			}
			out.Vars = append(out.Vars, vd)
		}
	}
}

// classifyFuncDecl classifies a function declaration as either a free function
// or a method based on whether it has a receiver.
func classifyFuncDecl(out *File, d *ast.FuncDecl) {
	if d.Recv != nil && len(d.Recv.List) > 0 {
		typeName, recvName := receiverInfo(d.Recv.List[0])
		if typeName == "" {
			return
		}
		out.Methods = append(out.Methods, &MethodDecl{
			ReceiverType: typeName,
			ReceiverName: recvName,
			MethodName:   d.Name.Name,
			Body:         d.Body,
		})
		return
	}

	// Free function.
	params := classifyParams(d.Type.Params)
	out.Funcs = append(out.Funcs, &FuncDecl{
		Name:   d.Name.Name,
		Body:   d.Body,
		Params: params,
	})
}

// classifyField converts an AST field to a Field IR.
func classifyField(f *ast.Field) *Field {
	field := &Field{
		TypeExpr: f.Type,
	}
	for _, n := range f.Names {
		field.Names = append(field.Names, n.Name)
	}
	field.Type = renderTypeExpr(f.Type)
	if f.Tag != nil {
		field.Tag = f.Tag.Value
	}
	return field
}

// classifyParams converts an AST field list to a slice of Param.
func classifyParams(fl *ast.FieldList) []*Param {
	if fl == nil || len(fl.List) == 0 {
		return nil
	}
	var params []*Param
	for _, f := range fl.List {
		p := &Param{Type: renderTypeExpr(f.Type)}
		for _, n := range f.Names {
			p.Names = append(p.Names, n.Name)
		}
		params = append(params, p)
	}
	return params
}

// receiverInfo extracts the receiver type name and variable name from a
// method receiver field. For pointer receivers like `*Note`, the `*` is
// stripped from the type name.
func receiverInfo(field *ast.Field) (typeName, recvName string) {
	if len(field.Names) > 0 {
		recvName = field.Names[0].Name
	}
	switch t := field.Type.(type) {
	case *ast.Ident:
		typeName = t.Name
	case *ast.StarExpr:
		if id, ok := t.X.(*ast.Ident); ok {
			typeName = id.Name
		}
	}
	return
}

// renderTypeExpr renders an AST type expression as a human-readable string
// by printing it via the token file set. This produces standard Go type
// notation (e.g. "float64", "*Note", "sonolus.Vec2").
func renderTypeExpr(expr ast.Expr) string {
	if expr == nil {
		return ""
	}
	var fset token.FileSet
	return string(renderTypeExprBytes(&fset, expr))
}

// renderTypeExprBytes renders a type expression to bytes.
func renderTypeExprBytes(fset *token.FileSet, expr ast.Expr) []byte {
	// Use a small buffer approach: we have the node, render with Fprint.
	// Since we don't need full fidelity, a simple type-switch rendering
	// is sufficient and avoids depending on go/format.
	switch t := expr.(type) {
	case *ast.Ident:
		return []byte(t.Name)
	case *ast.StarExpr:
		return append([]byte("*"), renderTypeExprBytes(fset, t.X)...)
	case *ast.SelectorExpr:
		prefix := renderTypeExprBytes(fset, t.X)
		return append(append(prefix, '.'), t.Sel.Name...)
	case *ast.ArrayType:
		if t.Len == nil {
			return append([]byte("[]"), renderTypeExprBytes(fset, t.Elt)...)
		}
		lenB := renderTypeExprBytes(fset, t.Len)
		result := append([]byte("["), lenB...)
		result = append(result, ']')
		return append(result, renderTypeExprBytes(fset, t.Elt)...)
	case *ast.MapType:
		result := append([]byte("map["), renderTypeExprBytes(fset, t.Key)...)
		result = append(result, ']')
		return append(result, renderTypeExprBytes(fset, t.Value)...)
	case *ast.InterfaceType:
		return []byte("interface{}")
	case *ast.FuncType:
		return []byte("func") // simplified
	case *ast.StructType:
		return []byte("struct{...}") // simplified
	case *ast.BasicLit:
		return []byte(t.Value)
	case *ast.Ellipsis:
		return append([]byte("..."), renderTypeExprBytes(fset, t.Elt)...)
	case *ast.ChanType:
		return []byte("chan")
	case *ast.ParenExpr:
		return renderTypeExprBytes(fset, t.X)
	default:
		// Fallback: use the node type name.
		return []byte(fmt.Sprintf("<%T>", expr))
	}
}
