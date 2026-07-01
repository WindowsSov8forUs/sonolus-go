package engine

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
)

// parsedTypeDecl is a parsed struct type declaration from an engine source file.
type parsedTypeDecl struct {
	name       string
	structType *ast.StructType
}

// parsedMethodDecl is a parsed method with receiver info. The FuncDecl is
// preserved so callers can access the full AST node (body, type params, etc.).
type parsedMethodDecl struct {
	receiverType string
	receiverName string
	methodName   string
	funcDecl     *ast.FuncDecl
}

// parsedEngineSource holds the shared AST-parsed state of an engine source file:
// type declarations, free functions, resource struct ASTs, and receiver methods.
// Both parseModeFile (Watch/Preview/Tutorial) and CompilePlayFileWithStats (Play)
// build on this common structure.
type parsedEngineSource struct {
	fset      *token.FileSet
	typeDecls []parsedTypeDecl
	methods   []parsedMethodDecl
	funcs     map[string]*ast.FuncDecl
	resources map[string]*ast.StructType
}

// parseEngineSource parses an engine Go source file and returns the shared
// intermediate representation. Callers then walk the type declarations and
// methods to build mode-specific archetype structures (modeArch for non-Play
// modes, parsedArchetype for Play mode).
func parseEngineSource(src string) (*parsedEngineSource, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "engine.go", src, 0)
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}

	out := &parsedEngineSource{
		fset:      fset,
		funcs:     map[string]*ast.FuncDecl{},
		resources: map[string]*ast.StructType{},
	}

	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				st, ok := ts.Type.(*ast.StructType)
				if !ok {
					continue
				}
				if resourceRole(ts.Name.Name) != "" {
					out.resources[ts.Name.Name] = st
				} else {
					out.typeDecls = append(out.typeDecls, parsedTypeDecl{
						name: ts.Name.Name, structType: st,
					})
				}
			}
		case *ast.FuncDecl:
			if d.Recv == nil || len(d.Recv.List) == 0 {
				if d.Body != nil {
					out.funcs[d.Name.Name] = d
				}
				continue
			}
			typeName, recvName := receiverInfo(d.Recv.List[0])
			if typeName == "" {
				continue
			}
			out.methods = append(out.methods, parsedMethodDecl{
				receiverType: typeName,
				receiverName: recvName,
				methodName:   d.Name.Name,
				funcDecl:     d,
			})
		}
	}
	return out, nil
}
