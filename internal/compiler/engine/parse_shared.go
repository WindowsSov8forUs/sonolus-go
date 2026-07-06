package engine

import (
	"fmt"
	"go/ast"
	"go/token"

	"github.com/WindowsSov8forUs/sonolus-go/internal/goparse"
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
	pkgName   string // package declaration name (e.g. "notes")
	typeDecls []parsedTypeDecl
	methods   []parsedMethodDecl
	funcs     map[string]*ast.FuncDecl
	resources map[string]*ast.StructType
	uiVar     *ast.CompositeLit // non-nil when var ui/var uiConfig found
}

// parseEngineSource parses an engine Go source file and returns the shared
// intermediate representation. Callers then walk the type declarations and
// methods to build mode-specific archetype structures (modeArch for non-Play
// modes, parsedArchetype for Play mode).
func parseEngineSource(src string) (*parsedEngineSource, error) {
	pkg, err := goparse.ParseFiles(map[string]string{"engine.go": src})
	if err != nil {
		return nil, err
	}
	return souceToParsed(pkg, true)
}

// sourceToParsed converts a goparse.Package to a parsedEngineSource, applying
// Sonolus-specific classification (resource roles, UI var detection).
// If allowResources is false, struct types matching resource roles are treated
// as errors — this prevents imported packages from defining engine resources.
func souceToParsed(pkg *goparse.Package, allowResources bool) (*parsedEngineSource, error) {

	out := &parsedEngineSource{
		fset:      pkg.Fset,
		pkgName:   pkg.Name,
		funcs:     map[string]*ast.FuncDecl{},
		resources: map[string]*ast.StructType{},
	}

	for _, file := range pkg.Files {
		// Sonolus-specific: detect UI config variables.
		if out.uiVar == nil {
			out.uiVar = findUIVarFromPackage(file.Vars)
		}

		// Classify struct types: resource roles → resources, others → typeDecls.
		for _, td := range file.Types {
			role := resourceRole(td.Name)
			if role != "" {
				if !allowResources {
					return nil, fmt.Errorf("resource type %q (role %s) is not allowed in imported package; define it in the main package", td.Name, role)
				}
				if _, dup := out.resources[td.Name]; dup {
					return nil, fmt.Errorf("duplicate resource type %q", td.Name)
				}
				// Reconstruct AST struct type from raw fields so downstream
				// tag parsing has access to *ast.StructType.
				st := reconstructStructType(td)
				out.resources[td.Name] = st
			} else {
				// Non-resource struct — archetype candidate.
				st := reconstructStructType(td)
				out.typeDecls = append(out.typeDecls, parsedTypeDecl{
					name: td.Name, structType: st,
				})
			}
		}

		// Methods.
		for _, md := range file.Methods {
			// Reconstruct a minimal *ast.FuncDecl so downstream callers work.
			fd := reconstructFuncDecl(md.MethodName, nil, md.Body)
			if md.ReceiverName != "" {
				fd.Recv = &ast.FieldList{List: []*ast.Field{
					{Names: []*ast.Ident{ast.NewIdent(md.ReceiverName)}, Type: ast.NewIdent(md.ReceiverType)},
				}}
			}
			out.methods = append(out.methods, parsedMethodDecl{
				receiverType: md.ReceiverType,
				receiverName: md.ReceiverName,
				methodName:   md.MethodName,
				funcDecl:     fd,
			})
		}

		// Free functions.
		for _, fn := range file.Funcs {
			if fn.Body != nil {
				if _, dup := out.funcs[fn.Name]; dup {
					return nil, fmt.Errorf("duplicate function %q", fn.Name)
				}
				out.funcs[fn.Name] = reconstructFuncDecl(fn.Name, fn.Params, fn.Body)
			}
		}
	}
	return out, nil
}

// findUIVarFromPackage scans parsed variable declarations for `var ui` or
// `var uiConfig` with a UI-typed composite literal initializer.
func findUIVarFromPackage(vars []*goparse.VarDecl) *ast.CompositeLit {
	for _, vd := range vars {
		for _, name := range vd.Names {
			if name != "ui" && name != "uiConfig" {
				continue
			}
			if len(vd.Values) == 0 {
				continue
			}
			// Inferred type: var ui = UI{...}
			if lit, ok := vd.Values[0].(*ast.CompositeLit); ok {
				if typeIdent, ok2 := lit.Type.(*ast.Ident); ok2 && typeIdent.Name == "UI" {
					return lit
				}
			}
			// Explicit type: var ui UI = UI{...}
			if vd.Type == "UI" {
				if lit, ok := vd.Values[0].(*ast.CompositeLit); ok {
					return lit
				}
			}
		}
	}
	return nil
}

// reconstructStructType builds a minimal *ast.StructType from a goparse.TypeDecl
// so that downstream tag parsing (tagCollector, buildResources) has access to
// the AST representation it expects.
func reconstructStructType(td *goparse.TypeDecl) *ast.StructType {
	st := &ast.StructType{Fields: &ast.FieldList{}}
	for _, f := range td.Fields {
		af := &ast.Field{Type: f.TypeExpr}
		for _, n := range f.Names {
			af.Names = append(af.Names, ast.NewIdent(n))
		}
		if f.Tag != "" {
			af.Tag = &ast.BasicLit{Kind: token.STRING, Value: f.Tag}
		}
		st.Fields.List = append(st.Fields.List, af)
	}
	return st
}

// reconstructFuncDecl builds a minimal *ast.FuncDecl so that downstream
// callers which expect the standard Go AST node work unchanged.
func reconstructFuncDecl(name string, params []*goparse.Param, body *ast.BlockStmt) *ast.FuncDecl {
	decl := &ast.FuncDecl{
		Name: ast.NewIdent(name),
		Type: &ast.FuncType{
			Params: &ast.FieldList{},
		},
		Body: body,
	}
	for _, p := range params {
		f := &ast.Field{Type: ast.NewIdent(p.Type)}
		for _, n := range p.Names {
			f.Names = append(f.Names, ast.NewIdent(n))
		}
		decl.Type.Params.List = append(decl.Type.Params.List, f)
	}
	return decl
}



// parseImportedPackage converts an already-parsed imported package to a
// parsedEngineSource with resource definitions disallowed.
func parseImportedPackage(pkg *goparse.Package) (pkgName string, _ *parsedEngineSource, _ error) {
	pes, err := souceToParsed(pkg, false)
	if err != nil {
		return "", nil, err
	}
	return pes.pkgName, pes, nil
}
