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
	return parseEngineSourceFiles(map[string]string{"engine.go": src}, true)
}

// parseEngineSourceFiles parses multiple source files of the same package and
// merges their declarations into a single parsedEngineSource. Files are keyed
// by filename. If allowResources is false, struct types matching resource roles
// (Skin, Effect, Particle, etc.) are treated as errors — this prevents imported
// packages from defining engine resources.
func parseEngineSourceFiles(files map[string]string, allowResources bool) (*parsedEngineSource, error) {
	fset := token.NewFileSet()

	out := &parsedEngineSource{
		fset:      fset,
		funcs:     map[string]*ast.FuncDecl{},
		resources: map[string]*ast.StructType{},
	}

	for name, src := range files {
		file, err := parser.ParseFile(fset, name, src, 0)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", name, err)
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

// parseImportedPackage parses an imported sub-package's source files. It is
// like parseEngineSourceFiles but always disallows resource definitions and
// returns the package name for cross-package type resolution.
func parseImportedPackage(files map[string]string) (pkgName string, _ *parsedEngineSource, _ error) {
	pes, err := parseEngineSourceFiles(files, false)
	if err != nil {
		return "", nil, err
	}
	// Extract package name from the first file.
	fset := token.NewFileSet()
	for name, src := range files {
		f, err := parser.ParseFile(fset, name, src, parser.PackageClauseOnly)
		if err != nil {
			return "", nil, fmt.Errorf("parse package name from %s: %w", name, err)
		}
		return f.Name.Name, pes, nil
	}
	return "", pes, nil
}
