package engine

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"

	"golang.org/x/tools/go/packages"
)

// parsedTypeDecl is a parsed struct type declaration from an engine source file.
type parsedTypeDecl struct {
	name       string
	structType *ast.StructType
}

// parsedMethodDecl is a parsed method with receiver info.
type parsedMethodDecl struct {
	receiverType string
	receiverName string
	methodName   string
	funcDecl     *ast.FuncDecl
}

// parsedEngineSource holds the shared AST-parsed state of an engine source file.
type parsedEngineSource struct {
	fset      *token.FileSet
	pkgName   string
	typeDecls []parsedTypeDecl
	methods   []parsedMethodDecl
	funcs     map[string]*ast.FuncDecl
	resources map[string]*ast.StructType
	uiVar     *ast.CompositeLit
}

// parseEngineSource parses a single Go source string.
func parseEngineSource(src string) (*parsedEngineSource, error) {
	return souceToParsed(src, true)
}

// sourceToParsed converts a Go source string or *packages.Package to a
// parsedEngineSource, applying Sonolus-specific classification.
// If allowResources is false, resource types in the package are rejected.
//
// When a non-empty src string is given, it is parsed directly (test/compat path).
// When pkg is non-nil, its Syntax AST files are used (production path).
func souceToParsed(src string, allowResources bool) (*parsedEngineSource, error) {
	// Parse the source string into AST files.
	fset := token.NewFileSet()
	f, err := parseSourceFile(fset, "engine.go", src)
	if err != nil {
		return nil, err
	}

	return astFilesToSource(fset, []*ast.File{f}, f.Name.Name, allowResources)
}

// packageToSource converts a *packages.Package to a parsedEngineSource.
func packageToSource(pkg *packages.Package, allowResources bool) (*parsedEngineSource, error) {
	return astFilesToSource(pkg.Fset, pkg.Syntax, pkg.Name, allowResources)
}

// astFilesToSource classifies a set of *ast.File nodes into a parsedEngineSource.
func astFilesToSource(fset *token.FileSet, files []*ast.File, pkgName string, allowResources bool) (*parsedEngineSource, error) {
	out := &parsedEngineSource{
		fset:      fset,
		pkgName:   pkgName,
		funcs:     map[string]*ast.FuncDecl{},
		resources: map[string]*ast.StructType{},
	}

	for _, file := range files {
		// Sonolus-specific: detect UI config variables.
		if out.uiVar == nil {
			out.uiVar = findUIVar(file.Decls)
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
					role := resourceRole(ts.Name.Name)
					if role != "" {
						if !allowResources {
							return nil, fmt.Errorf("resource type %q (role %s) is not allowed in imported package; define it in the main package", ts.Name.Name, role)
						}
						if _, dup := out.resources[ts.Name.Name]; dup {
							return nil, fmt.Errorf("duplicate resource type %q", ts.Name.Name)
						}
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
						if _, dup := out.funcs[d.Name.Name]; dup {
							return nil, fmt.Errorf("duplicate function %q", d.Name.Name)
						}
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
	}
	return out, nil
}

// parseImportedPackage converts a *packages.Package to a parsedEngineSource
// with resource definitions disallowed.
func parseImportedPackage(pkg *packages.Package) (pkgName string, _ *parsedEngineSource, _ error) {
	pes, err := packageToSource(pkg, false)
	if err != nil {
		return "", nil, err
	}
	return pes.pkgName, pes, nil
}

// ── helpers ──────────────────────────────────────────────────────────────────

// parseSourceFile parses a single Go source string into an *ast.File.
func parseSourceFile(fset *token.FileSet, name, src string) (*ast.File, error) {
	return parser.ParseFile(fset, name, src, 0)
}

// receiverInfo extracts receiver type and variable name from a method receiver.
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
