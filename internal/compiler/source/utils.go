package source

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"go/types"
	"slices"
	"strings"

	"golang.org/x/tools/go/packages"

	sourcetracer "github.com/WindowsSov8forUs/sonolus-go/internal/compiler/source/tracer"
)

type NodeChecker func(node *TypeSpecNode) bool

func isStructNode(node *TypeSpecNode) bool {
	if node == nil || node.Spec == nil {
		return false
	}
	if _, ok := node.Spec.Type.(*ast.StructType); ok {
		return true
	}
	return false
}

func isUnderlyingTypeWithVisited(node *TypeSpecNode, callback NodeChecker, visited map[*TypeSpecNode]bool) bool {
	if node == nil || visited[node] {
		return false
	}
	visited[node] = true

	if callback(node) {
		return true
	}
	return isUnderlyingTypeWithVisited(node.Underlying(), callback, visited)
}

func IsUnderlyingType(node *TypeSpecNode, callback NodeChecker) bool {
	if callback == nil {
		return false
	}
	visited := make(map[*TypeSpecNode]bool)
	return isUnderlyingTypeWithVisited(node, callback, visited)
}

func IsStruct(ts *ast.TypeSpec, pkg *packages.Package) bool {
	if ts == nil || pkg == nil || pkg.TypesInfo == nil {
		return false
	}
	object, ok := pkg.TypesInfo.Defs[ts.Name].(*types.TypeName)
	if !ok || object.Type() == nil {
		return false
	}
	_, ok = types.Unalias(object.Type()).Underlying().(*types.Struct)
	return ok
}

func extractExpr(node *TypeSpecNode, callback NodeChecker, visited map[*TypeSpecNode]bool) ast.Expr {
	if node == nil || node.Spec == nil || callback == nil || visited[node] {
		return nil
	}
	visited[node] = true

	if callback(node) {
		return node.Spec.Type
	}

	return extractExpr(node.Underlying(), callback, visited)
}

func ExtractExpr(node *TypeSpecNode, callback NodeChecker) ast.Expr {
	visited := make(map[*TypeSpecNode]bool)
	return extractExpr(node, callback, visited)
}

func ExtractStructType(node *TypeSpecNode) *ast.StructType {
	if node == nil || node.Spec == nil {
		return nil
	}
	if st, ok := node.Spec.Type.(*ast.StructType); ok {
		return st
	}

	result := ExtractExpr(node, isStructNode)
	if st, ok := result.(*ast.StructType); ok {
		return st
	}
	return nil
}

func TypeSpecString(ts *ast.TypeSpec) string {
	if ts == nil {
		return "<nil>"
	}
	var buffer bytes.Buffer
	declaration := &ast.GenDecl{Tok: token.TYPE, Specs: []ast.Spec{ts}}
	if err := format.Node(&buffer, token.NewFileSet(), declaration); err == nil {
		return buffer.String()
	}

	name := ts.Name.Name
	expr := ExprString(ts.Type)

	if ts.Assign.IsValid() {
		return fmt.Sprintf("type %s = %s", name, expr)
	} else {
		return fmt.Sprintf("type %s %s", name, expr)
	}
}

func ExprString(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name

	case *ast.SelectorExpr:
		return ExprString(e.X) + "." + e.Sel.Name

	case *ast.StarExpr:
		return "*" + ExprString(e.X)

	case *ast.ArrayType:
		if e.Len == nil {
			return "[]" + ExprString(e.Elt)
		}
		return "[" + ExprString(e.Len) + "]" + ExprString(e.Elt)

	case *ast.MapType:
		return "map[" + ExprString(e.Key) + "]" + ExprString(e.Value)

	case *ast.ChanType:
		switch e.Dir {
		case ast.RECV:
			return "<-chan " + ExprString(e.Value)
		case ast.SEND:
			return "chan<- " + ExprString(e.Value)
		default:
			return "chan " + ExprString(e.Value)
		}

	case *ast.StructType:
		if e.Fields == nil || len(e.Fields.List) == 0 {
			return "struct{}"
		}
		return "struct{...}"

	case *ast.InterfaceType:
		if e.Methods == nil || len(e.Methods.List) == 0 {
			return "interface{}"
		}
		return "interface{...}"

	case *ast.FuncType:
		return "func(...)" + simplifyFuncResults(e)

	case *ast.IndexExpr:
		return ExprString(e.X) + "[" + ExprString(e.Index) + "]"

	case *ast.IndexListExpr:
		indices := make([]string, len(e.Indices))
		for i, idx := range e.Indices {
			indices[i] = ExprString(idx)
		}
		return ExprString(e.X) + "[" + strings.Join(indices, ", ") + "]"

	case *ast.ParenExpr:
		return ExprString(e.X)

	case *ast.Ellipsis:
		return "..." + ExprString(e.Elt)

	case *ast.UnaryExpr:
		return e.Op.String() + ExprString(e.X)

	case *ast.BinaryExpr:
		return ExprString(e.X) + " " + e.Op.String() + " " + ExprString(e.Y)

	case *ast.BasicLit:
		return e.Value

	default:
		return fmt.Sprintf("<%T>", e)
	}
}

func simplifyFuncResults(ft *ast.FuncType) string {
	if ft.Results == nil || len(ft.Results.List) == 0 {
		return ""
	}
	if len(ft.Results.List) == 1 && len(ft.Results.List[0].Names) == 0 {
		return " " + ExprString(ft.Results.List[0].Type)
	}
	parts := make([]string, len(ft.Results.List))
	for i, f := range ft.Results.List {
		parts[i] = ExprString(f.Type)
	}
	return " (" + strings.Join(parts, ", ") + ")"
}

func ExprToImplicitName(expr ast.Expr) *ast.Ident {
	visited := make(map[ast.Expr]bool)
	return exprToImplicitName(expr, visited)
}

func exprToImplicitName(expr ast.Expr, visited map[ast.Expr]bool) *ast.Ident {
	if visited[expr] {
		return nil
	}
	visited[expr] = true

	switch e := expr.(type) {
	case *ast.Ident:
		return e
	case *ast.SelectorExpr:
		return e.Sel
	case *ast.StarExpr:
		return exprToImplicitName(e.X, visited)
	case *ast.IndexExpr:
		return exprToImplicitName(e.X, visited)
	case *ast.IndexListExpr:
		return exprToImplicitName(e.X, visited)
	case *ast.ParenExpr:
		return exprToImplicitName(e.X, visited)
	default:
		return nil
	}
}

func GetTypeMethodSet(pkg *packages.Package, ts *ast.TypeSpec) []*types.Func {
	if pkg == nil || pkg.TypesInfo == nil || ts == nil {
		return nil
	}
	info := pkg.TypesInfo
	obj := info.Defs[ts.Name]
	if obj == nil || obj.Type() == nil {
		return nil
	}
	typ := obj.Type()

	msetPtr := types.NewMethodSet(types.NewPointer(typ))
	fnset := make([]*types.Func, 0)

	for i := 0; i < msetPtr.Len(); i++ {
		sel := msetPtr.At(i)
		fn := sel.Obj().(*types.Func)
		fnset = append(fnset, fn)
	}

	return fnset
}

func ASTFuncDeclToTypesFunc(pkg *packages.Package, fd *ast.FuncDecl) *types.Func {
	if pkg == nil || pkg.TypesInfo == nil || fd == nil {
		return nil
	}
	function, _ := pkg.TypesInfo.Defs[fd.Name].(*types.Func)
	return function
}

// Field 承担 *ast.Field 与其承载的默认值，默认值是否可用由字段语境决定；
// 默认值由 struct tag 设置，当前语境下仅可为 float64
type Field struct {
	Field   *ast.Field
	Default float64
}

func ASTFieldToField(f *ast.Field) *Field {
	return &Field{Field: f}
}

func (f *Field) SetDefault(def float64) *Field {
	f.Default = def
	return f
}

func IsExportIdent(i *ast.Ident) bool {
	return i != nil && ast.IsExported(i.Name)
}

type FieldFilter func(f *ast.Field) bool

func StructFieldFilter(st *ast.StructType, node *TypeSpecNode, callback FieldFilter) ([]*ast.Field, error) {
	fields, err := sourcetracer.OrderedStructFields(st, node)
	if err != nil {
		return nil, err
	}
	result := make([]*ast.Field, 0, len(fields))
	for _, field := range fields {
		if callback == nil || callback(field) {
			result = append(result, field)
		}
	}

	return result, nil
}

func RemoveValueFromSlice[T comparable](s []T, v T) []T {
	s = slices.DeleteFunc(s, func(n T) bool {
		return n == v
	})
	return s
}
