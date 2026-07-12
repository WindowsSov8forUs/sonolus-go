package frontend

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
	"github.com/WindowsSov8forUs/sonolus-go/internal/newcompiler/catalog"
	"github.com/WindowsSov8forUs/sonolus-go/internal/newcompiler/intrinsic"
	"github.com/WindowsSov8forUs/sonolus-go/internal/newcompiler/ir"
	"github.com/WindowsSov8forUs/sonolus-go/internal/newcompiler/mode"
)

type lowerValue struct {
	type_     types.Type
	slots     []ir.Expr
	places    []ir.Place
	container *containerValue
}

type containerValue struct {
	kind          string
	capacity      int
	stride        int
	keySize       int
	element       types.Type
	key           types.Type
	dataLocal     *ir.LocalPlace
	memoryStorage string
	memoryBase    int
	memoryRead    bool
	memoryWrite   bool
}

type archetypeBinding struct {
	id          int
	declaration *ArchetypeDeclaration
}

type inlineCallSite struct {
	function string
	pos      ir.SourcePos
}

type lowerFrame struct {
	vars        map[types.Object]lowerValue
	results     map[types.Object]bool
	result      lowerValue
	returnBlock *ir.Block
}

type lowerer struct {
	mode            mode.Mode
	phase           string
	packages        map[*types.Package]*packages.Package
	pkg             *packages.Package
	builder         *ir.Builder
	frames          []*lowerFrame
	callStack       map[*types.Func]bool
	resourceIDs     map[*types.Var][]int
	archetypeFields map[*types.Var]*FieldDeclaration
	archetypes      map[*types.Named]archetypeBinding
	breaks          []*ir.Block
	continues       []*ir.Block
	inlineCalls     []inlineCallSite
	dynamicDepth    int
	errs            []error
}

func sourcePos(pkg *packages.Package, pos token.Pos) ir.SourcePos {
	p := pkg.Fset.Position(pos)
	return ir.SourcePos{File: p.Filename, Line: p.Line, Column: p.Column}
}

func (l *lowerer) errorAt(node ast.Node, format string, args ...any) {
	p := l.pkg.Fset.Position(node.Pos())
	message := fmt.Sprintf("%s: callback %s", p, l.builder.Function().Name)
	for i := len(l.inlineCalls) - 1; i >= 0; i-- {
		call := l.inlineCalls[i]
		message += fmt.Sprintf(": inlined from %s:%d:%d (%s)", call.pos.File, call.pos.Line, call.pos.Column, call.function)
	}
	l.errs = append(l.errs, fmt.Errorf("%s: %s", message, fmt.Sprintf(format, args...)))
}

func (l *lowerer) newBlock() *ir.Block        { return l.builder.NewBlock() }
func (l *lowerer) current() *ir.Block         { return l.builder.Current() }
func (l *lowerer) setCurrent(block *ir.Block) { _ = l.builder.SetCurrent(block) }

func (l *lowerer) jump(target *ir.Block) {
	_ = l.builder.JumpIfOpen(target)
}

func (l *lowerer) dynamic(fn func()) {
	l.dynamicDepth++
	defer func() { l.dynamicDepth-- }()
	fn()
}

func irTypeOf(t types.Type) ir.Type {
	if t == nil {
		return ir.Type{Name: "void"}
	}
	t = types.Unalias(t)
	if pointer, ok := t.(*types.Pointer); ok {
		return irTypeOf(pointer.Elem())
	}
	if tuple, ok := t.(*types.Tuple); ok {
		out := ir.Type{Name: t.String()}
		for i := 0; i < tuple.Len(); i++ {
			item := irTypeOf(tuple.At(i).Type())
			out.Fields = append(out.Fields, ir.Field{Name: tuple.At(i).Name(), Offset: out.Slots, Type: item})
			out.Slots += item.Slots
		}
		return out
	}
	if basic, ok := t.Underlying().(*types.Basic); ok {
		return ir.Type{Name: basic.Name(), Slots: 1}
	}
	if array, ok := t.Underlying().(*types.Array); ok {
		e := irTypeOf(array.Elem())
		return ir.Type{Name: t.String(), Slots: int(array.Len()) * e.Slots}
	}
	if st, ok := t.Underlying().(*types.Struct); ok {
		if id := typeID(t); id == rootID("Sprite") || id == rootID("Clip") || id == rootID("Effect") || id == rootID("Text") || id == rootID("Icon") || id == rootID("Bucket") {
			return ir.Type{Name: t.String(), Slots: 1}
		}
		out := ir.Type{Name: t.String()}
		for i := 0; i < st.NumFields(); i++ {
			ft := irTypeOf(st.Field(i).Type())
			out.Fields = append(out.Fields, ir.Field{Name: st.Field(i).Name(), Offset: out.Slots, Type: ft})
			out.Slots += ft.Slots
		}
		return out
	}
	return ir.Type{Name: t.String(), Slots: 1}
}

func zeroValue(t types.Type) lowerValue {
	typ := irTypeOf(t)
	slots := make([]ir.Expr, typ.Slots)
	for i := range slots {
		slots[i] = ir.Const{}
	}
	return lowerValue{type_: t, slots: slots}
}

func namedResourceHandle(t types.Type) string {
	switch typeID(t) {
	case rootID("Sprite"):
		return "Sprite"
	case rootID("Clip"):
		return "Clip"
	case rootID("Effect"):
		return "Effect"
	case rootID("Text"):
		return "Text"
	case rootID("Icon"):
		return "Icon"
	default:
		return ""
	}
}

func containedResourceHandle(t types.Type, visiting map[types.Type]bool) string {
	t = types.Unalias(t)
	if kind := namedResourceHandle(t); kind != "" {
		return kind
	}
	if visiting[t] {
		return ""
	}
	visiting[t] = true
	defer delete(visiting, t)
	switch underlying := t.Underlying().(type) {
	case *types.Array:
		return containedResourceHandle(underlying.Elem(), visiting)
	case *types.Struct:
		for i := 0; i < underlying.NumFields(); i++ {
			if kind := containedResourceHandle(underlying.Field(i).Type(), visiting); kind != "" {
				return kind
			}
		}
	}
	return ""
}

func containsResourceHandle(t types.Type) string {
	return containedResourceHandle(t, map[types.Type]bool{})
}

func constantExpr(value constant.Value) (ir.Expr, bool) {
	if value == nil {
		return nil, false
	}
	if value.Kind() == constant.Bool {
		if constant.BoolVal(value) {
			return ir.Const{Value: 1}, true
		}
		return ir.Const{}, true
	}
	if value.Kind() == constant.Int || value.Kind() == constant.Float {
		f, _ := constant.Float64Val(constant.ToFloat(value))
		return ir.Const{Value: f}, true
	}
	return nil, false
}

func (l *lowerer) alloc(name string, t types.Type) lowerValue {
	typ := irTypeOf(t)
	value := l.builder.NewLocal(name, typ)
	return lowerValue{type_: t, slots: value.Slots, places: ir.Places(value)}
}

func (l *lowerer) allocZeroed(name string, t types.Type, node ast.Node) lowerValue {
	value := l.alloc(name, t)
	l.store(value, zeroValue(t), node)
	return value
}

func (l *lowerer) indexedLocal(base ir.LocalPlace, index ir.Expr, length, stride, offset int, node ast.Node) ir.Place {
	place, err := l.builder.IndexedLocal(base, index, length, stride, offset)
	if err != nil {
		l.errorAt(node, "%v", err)
		return nil
	}
	return place
}

func (l *lowerer) memory(storage string, index ir.Expr, stride, offset int, read, write bool, node ast.Node) ir.Place {
	place, err := l.builder.Memory(storage, index, stride, offset, read, write)
	if err != nil {
		l.errorAt(node, "%v", err)
		return nil
	}
	return place
}

func (l *lowerer) bind(obj types.Object, v lowerValue) { l.frames[len(l.frames)-1].vars[obj] = v }

func (l *lowerer) rebind(obj types.Object, v lowerValue) {
	for i := len(l.frames) - 1; i >= 0; i-- {
		if _, ok := l.frames[i].vars[obj]; ok {
			l.frames[i].vars[obj] = v
			if l.frames[i].results[obj] {
				l.frames[i].result = v
			}
			return
		}
	}
	l.bind(obj, v)
}

func (l *lowerer) bindParameter(obj *types.Var, value lowerValue, name string, node ast.Node) {
	if _, pointer := types.Unalias(obj.Type()).(*types.Pointer); pointer || value.container != nil {
		l.bind(obj, value)
		return
	}
	local := l.alloc(name, obj.Type())
	l.store(local, value, node)
	l.bind(obj, local)
}
func (l *lowerer) lookup(obj types.Object) (lowerValue, bool) {
	for i := len(l.frames) - 1; i >= 0; i-- {
		if v, ok := l.frames[i].vars[obj]; ok {
			return v, true
		}
	}
	return lowerValue{}, false
}

func (l *lowerer) store(dst, src lowerValue, node ast.Node) {
	if len(dst.places) != len(src.slots) {
		l.errorAt(node, "assignment layout mismatch: %d places for %d slots", len(dst.places), len(src.slots))
		return
	}
	for _, p := range dst.places {
		if p == nil {
			l.errorAt(node, "value is not assignable")
			return
		}
	}
	if err := l.builder.Store(dst.places, ir.Value{Type: irTypeOf(src.type_), Slots: src.slots}, sourcePos(l.pkg, node.Pos())); err != nil {
		l.errorAt(node, "%v", err)
	}
}

var binaryRuntime = map[token.Token]resource.RuntimeFunction{
	token.ADD: resource.RuntimeFunctionAdd, token.SUB: resource.RuntimeFunctionSubtract,
	token.MUL: resource.RuntimeFunctionMultiply, token.QUO: resource.RuntimeFunctionDivide,
	token.REM: resource.RuntimeFunctionMod, token.EQL: resource.RuntimeFunctionEqual,
	token.NEQ: resource.RuntimeFunctionNotEqual, token.LSS: resource.RuntimeFunctionLess,
	token.LEQ: resource.RuntimeFunctionLessOr, token.GTR: resource.RuntimeFunctionGreater,
	token.GEQ: resource.RuntimeFunctionGreaterOr,
}

func (l *lowerer) expr(expr ast.Expr) lowerValue {
	switch n := expr.(type) {
	case *ast.ParenExpr:
		return l.expr(n.X)
	case *ast.BasicLit:
		v := l.pkg.TypesInfo.Types[n].Value
		expr, ok := constantExpr(v)
		if !ok {
			l.errorAt(n, "constant is not representable at runtime")
			return lowerValue{}
		}
		return lowerValue{type_: l.pkg.TypesInfo.TypeOf(n), slots: []ir.Expr{expr}}
	case *ast.Ident:
		obj := l.pkg.TypesInfo.ObjectOf(n)
		if v, ok := l.lookup(obj); ok {
			return v
		}
		if c, ok := obj.(*types.Const); ok {
			expr, representable := constantExpr(c.Val())
			if !representable {
				l.errorAt(n, "constant %s is not representable at runtime", c.Name())
				return lowerValue{}
			}
			return lowerValue{type_: c.Type(), slots: []ir.Expr{expr}}
		}
		l.errorAt(n, "unsupported identifier %s", n.Name)
	case *ast.StarExpr:
		v := l.expr(n.X)
		pointer, ok := types.Unalias(v.type_).(*types.Pointer)
		if !ok {
			l.errorAt(n, "dereference operand is not a pointer")
			return zeroValue(l.pkg.TypesInfo.TypeOf(n))
		}
		if len(v.places) != len(v.slots) {
			l.errorAt(n, "pointer alias has %d places for %d slots", len(v.places), len(v.slots))
			return zeroValue(l.pkg.TypesInfo.TypeOf(n))
		}
		v.type_ = pointer.Elem()
		return v
	case *ast.UnaryExpr:
		v := l.expr(n.X)
		if n.Op == token.AND {
			v = l.materializeAddressable("address", v, n.X)
			if len(v.places) != len(v.slots) || len(v.places) == 0 {
				l.errorAt(n, "address operand is not representable as a DSL place")
				return zeroValue(l.pkg.TypesInfo.TypeOf(n))
			}
			v.type_ = l.pkg.TypesInfo.TypeOf(n)
			return v
		}
		if len(v.slots) != 1 {
			l.errorAt(n, "unary operator requires scalar value")
			return lowerValue{}
		}
		switch n.Op {
		case token.ADD:
			return v
		case token.SUB:
			return lowerValue{type_: v.type_, slots: []ir.Expr{ir.RuntimeCall{Function: resource.RuntimeFunctionNegate, Args: v.slots, Result: irTypeOf(v.type_), Pure: true, Pos: sourcePos(l.pkg, n.Pos())}}}
		case token.NOT:
			return lowerValue{type_: v.type_, slots: []ir.Expr{ir.RuntimeCall{Function: resource.RuntimeFunctionNot, Args: v.slots, Result: irTypeOf(v.type_), Pure: true, Pos: sourcePos(l.pkg, n.Pos())}}}
		default:
			l.errorAt(n, "unsupported unary operation %s", n.Op)
			return zeroValue(l.pkg.TypesInfo.TypeOf(n))
		}
	case *ast.BinaryExpr:
		if n.Op == token.LAND || n.Op == token.LOR {
			return l.shortCircuit(n)
		}
		a, b := l.expr(n.X), l.expr(n.Y)
		op, ok := binaryRuntime[n.Op]
		if !ok || len(a.slots) != 1 || len(b.slots) != 1 {
			l.errorAt(n, "unsupported binary operation %s", n.Op)
			return lowerValue{}
		}
		resultType := l.pkg.TypesInfo.TypeOf(n)
		if irTypeOf(resultType).Slots != 1 && len(a.slots) == 1 {
			resultType = a.type_
		}
		return lowerValue{type_: resultType, slots: []ir.Expr{ir.RuntimeCall{Function: op, Args: []ir.Expr{a.slots[0], b.slots[0]}, Result: irTypeOf(resultType), Pure: true, Pos: sourcePos(l.pkg, n.Pos())}}}
	case *ast.CompositeLit:
		return l.composite(n)
	case *ast.SelectorExpr:
		sel := l.pkg.TypesInfo.Selections[n]
		if sel == nil {
			if c, ok := l.pkg.TypesInfo.ObjectOf(n.Sel).(*types.Const); ok {
				expr, representable := constantExpr(c.Val())
				if !representable {
					l.errorAt(n, "constant %s is not representable at runtime", c.Name())
					return lowerValue{}
				}
				return lowerValue{type_: c.Type(), slots: []ir.Expr{expr}}
			}
			l.errorAt(n, "package symbol %s must be called", n.Sel.Name)
			return lowerValue{}
		}
		if field, ok := sel.Obj().(*types.Var); ok {
			if ids, exists := l.resourceIDs[field]; exists {
				slots := make([]ir.Expr, len(ids))
				for i, id := range ids {
					slots[i] = ir.Const{Value: float64(id)}
				}
				return lowerValue{type_: l.pkg.TypesInfo.TypeOf(n), slots: slots}
			}
			if declaration, exists := l.archetypeFields[field]; exists {
				base := l.expr(n.X)
				start, size := declaration.ReceiverOffset, declaration.Size
				if start+size > len(base.slots) {
					l.errorAt(n, "archetype field layout exceeds receiver")
					return lowerValue{}
				}
				value := lowerValue{type_: declaration.Type, slots: base.slots[start : start+size], places: base.places[start : start+size]}
				if declaration.ContainerKind != "" {
					_, key, element, _ := containerTypes(declaration.Type)
					stride := declaration.KeySize + declaration.ElementSize
					storage, read, write := archetypeStorageAccess(declaration.Storage, l.mode, l.phase)
					value.container = &containerValue{kind: declaration.ContainerKind, capacity: declaration.Capacity, stride: stride, keySize: declaration.KeySize, element: element, key: key, memoryStorage: storage, memoryBase: declaration.Offset + 1, memoryRead: read, memoryWrite: write}
				}
				return value
			}
		}
		base := l.expr(n.X)
		if pointer, ok := types.Unalias(base.type_).(*types.Pointer); ok {
			base.type_ = pointer.Elem()
		}
		offset := 0
		st, _ := types.Unalias(base.type_).Underlying().(*types.Struct)
		for _, index := range sel.Index() {
			for i := 0; i < index; i++ {
				offset += irTypeOf(st.Field(i).Type()).Slots
			}
			base.type_ = st.Field(index).Type()
			st, _ = types.Unalias(base.type_).Underlying().(*types.Struct)
		}
		size := irTypeOf(base.type_).Slots
		base.slots = base.slots[offset : offset+size]
		if base.places != nil {
			base.places = base.places[offset : offset+size]
		}
		return base
	case *ast.IndexExpr:
		base, index := l.expr(n.X), l.expr(n.Index)
		arr, ok := types.Unalias(base.type_).Underlying().(*types.Array)
		if !ok || len(index.slots) != 1 {
			l.errorAt(n, "only array indexing is supported")
			return lowerValue{}
		}
		if c, ok := index.slots[0].(ir.Const); ok {
			size := irTypeOf(arr.Elem()).Slots
			start := int(c.Value) * size
			if start < 0 || start+size > len(base.slots) {
				l.errorAt(n, "array index is out of bounds")
				return lowerValue{}
			}
			v := lowerValue{type_: arr.Elem(), slots: base.slots[start : start+size]}
			if base.places != nil {
				v.places = base.places[start : start+size]
			}
			return v
		}
		index = l.materialize("array.index", index, n.Index)
		if len(base.places) != len(base.slots) || len(base.places) == 0 {
			base = l.materializeAddressable("array.index", base, n.X)
		}
		first, ok := base.places[0].(ir.LocalPlace)
		if !ok {
			l.errorAt(n.Index, "dynamic indexing is only supported for local arrays")
			return lowerValue{}
		}
		inBounds := l.pure(resource.RuntimeFunctionAnd, n,
			l.pure(resource.RuntimeFunctionGreaterOr, n, index.slots[0], ir.Const{}),
			l.pure(resource.RuntimeFunctionLess, n, index.slots[0], ir.Const{Value: float64(arr.Len())}))
		l.guard(n, inBounds)
		size := irTypeOf(arr.Elem()).Slots
		v := lowerValue{type_: arr.Elem(), slots: make([]ir.Expr, size), places: make([]ir.Place, size)}
		for offset := 0; offset < size; offset++ {
			place := l.indexedLocal(first, index.slots[0], int(arr.Len()), size, offset, n)
			v.places[offset], v.slots[offset] = place, ir.Load{Place: place}
		}
		return v
	case *ast.CallExpr:
		return l.call(n)
	}
	return zeroValue(l.pkg.TypesInfo.TypeOf(expr))
}

func (l *lowerer) composite(n *ast.CompositeLit) lowerValue {
	t := l.pkg.TypesInfo.TypeOf(n)
	if kind := containsResourceHandle(t); kind != "" {
		l.errorAt(n, "%s resource handle aggregates can only come from declared resource fields", kind)
		return zeroValue(t)
	}
	out := zeroValue(t)
	if st, ok := types.Unalias(t).Underlying().(*types.Struct); ok {
		values := make([]lowerValue, st.NumFields())
		for i := range values {
			values[i] = zeroValue(st.Field(i).Type())
		}
		pos := 0
		for _, elt := range n.Elts {
			if kv, ok := elt.(*ast.KeyValueExpr); ok {
				name := kv.Key.(*ast.Ident).Name
				for i := 0; i < st.NumFields(); i++ {
					if st.Field(i).Name() == name {
						values[i] = l.materialize("literal.field", l.expr(kv.Value), kv.Value)
						break
					}
				}
			} else if pos < len(values) {
				values[pos] = l.materialize("literal.field", l.expr(elt), elt)
				pos++
			}
		}
		out.slots = out.slots[:0]
		for _, v := range values {
			out.slots = append(out.slots, v.slots...)
		}
		return out
	}
	if array, ok := types.Unalias(t).Underlying().(*types.Array); ok {
		elementSlots := irTypeOf(array.Elem()).Slots
		index := 0
		for _, elt := range n.Elts {
			valueExpr := elt
			if kv, ok := elt.(*ast.KeyValueExpr); ok {
				constantIndex := l.pkg.TypesInfo.Types[kv.Key].Value
				if constantIndex == nil {
					l.errorAt(kv.Key, "array literal index must be constant")
					continue
				}
				index64, exact := constant.Int64Val(constantIndex)
				if !exact {
					l.errorAt(kv.Key, "array literal index is not representable")
					continue
				}
				index = int(index64)
				valueExpr = kv.Value
			}
			value := l.materialize("literal.element", l.expr(valueExpr), valueExpr)
			start := index * elementSlots
			if start < 0 || start+elementSlots > len(out.slots) || len(value.slots) != elementSlots {
				l.errorAt(valueExpr, "array literal element layout is invalid")
				continue
			}
			copy(out.slots[start:start+elementSlots], value.slots)
			index++
		}
		return out
	}
	l.errorAt(n, "unsupported composite literal type %s", t)
	return out
}

func (l *lowerer) shortCircuit(n *ast.BinaryExpr) lowerValue {
	result := l.alloc("shortcircuit", l.pkg.TypesInfo.TypeOf(n))
	left := l.materialize("shortcircuit.left", l.expr(n.X), n.X)
	rightBlock, shortBlock, merge := l.newBlock(), l.newBlock(), l.newBlock()
	if n.Op == token.LAND {
		_ = l.builder.Branch(left.slots[0], rightBlock, shortBlock)
	} else {
		_ = l.builder.Branch(left.slots[0], shortBlock, rightBlock)
	}
	l.setCurrent(shortBlock)
	l.store(result, left, n.X)
	l.jump(merge)
	l.setCurrent(rightBlock)
	var right lowerValue
	l.dynamic(func() { right = l.expr(n.Y) })
	l.store(result, right, n.Y)
	l.jump(merge)
	l.setCurrent(merge)
	return result
}

func calledFunc(pkg *packages.Package, call *ast.CallExpr) *types.Func {
	obj := calledObject(pkg, call.Fun)
	fn, _ := obj.(*types.Func)
	return fn
}

func (l *lowerer) call(n *ast.CallExpr) lowerValue {
	if tv := l.pkg.TypesInfo.Types[n.Fun]; tv.IsType() {
		value := l.expr(n.Args[0])
		target := l.pkg.TypesInfo.TypeOf(n)
		if len(value.slots) != irTypeOf(target).Slots {
			l.errorAt(n, "conversion from %s to %s changes runtime layout", value.type_, target)
			return zeroValue(target)
		}
		value.type_ = target
		return value
	}
	funExpr := n.Fun
	for {
		if paren, ok := funExpr.(*ast.ParenExpr); ok {
			funExpr = paren.X
			continue
		}
		break
	}
	if literal, ok := funExpr.(*ast.FuncLit); ok {
		args := make([]lowerValue, len(n.Args))
		for i, arg := range n.Args {
			args[i] = l.materialize("closure.arg", l.expr(arg), arg)
		}
		return l.inlineLiteral(n, literal, args)
	}
	if object := calledObject(l.pkg, n.Fun); object != nil {
		if builtin, ok := object.(*types.Builtin); ok {
			return l.builtinCall(n, builtin)
		}
	}
	fn := calledFunc(l.pkg, n)
	if fn == nil {
		l.errorAt(n, "callback call target must be a statically declared function")
		return zeroValue(l.pkg.TypesInfo.TypeOf(n))
	}
	if symbol, ok := catalog.LookupObject(fn); ok {
		if symbol.Internal {
			l.errorAt(n, "Sonolus API %s is not part of the public callback catalog", symbol.Key())
			return zeroValue(l.pkg.TypesInfo.TypeOf(n))
		}
		if !catalog.AllowsMode(symbol, string(l.mode)) {
			l.errorAt(n, "Sonolus API %s is not available in %s mode", symbol.Key(), l.mode)
			return zeroValue(l.pkg.TypesInfo.TypeOf(n))
		}
		if !catalog.AllowsPhase(symbol, l.phase) {
			l.errorAt(n, "Sonolus API %s cannot write during %s callback", symbol.Key(), l.phase)
			return zeroValue(l.pkg.TypesInfo.TypeOf(n))
		}
		if symbol.Key() == "sonolus.Ease" {
			return l.easeCall(n)
		}
		if symbol.Key() == "sonolus/play.debugAPI.Terminate" {
			if err := l.builder.MarkUnreachable(); err != nil {
				l.errorAt(n, "%v", err)
			}
			l.setCurrent(l.newBlock())
			return lowerValue{}
		}
	}
	args := make([]lowerValue, 0, len(n.Args)+1)
	if sel, ok := n.Fun.(*ast.SelectorExpr); ok && l.pkg.TypesInfo.Selections[sel] != nil {
		if !l.isFacadeReceiver(sel.X) {
			args = append(args, l.callReceiver(sel, fn))
		}
	}
	for _, arg := range n.Args {
		args = append(args, l.materialize("call.arg", l.expr(arg), arg))
	}
	if symbol, ok := catalog.LookupObject(fn); ok {
		recipe := catalog.LookupRecipe(symbol)
		if !supportsRecipe(recipe) {
			l.errorAt(n, "Sonolus API %s references unimplemented %s recipe %q", symbol.Key(), recipe.Kind, recipe.Operation)
			return zeroValue(l.pkg.TypesInfo.TypeOf(n))
		}
		if recipe.Kind == catalog.RecipeAggregate {
			return l.aggregateCall(n, recipe.Operation, args)
		}
		if recipe.Kind == catalog.RecipeResource {
			return l.resourceCall(n, recipe.Operation, args)
		}
		if recipe.Kind == catalog.RecipeContainer {
			return l.containerCall(n, recipe.Operation, args)
		}
		if recipe.Kind == catalog.RecipeMemory {
			return l.memoryCall(n, recipe, args)
		}
		if recipe.Kind != catalog.RecipeRuntime {
			l.errorAt(n, "Sonolus API %s cannot be lowered in callbacks: %s", symbol.Key(), recipe.Reason)
			return zeroValue(l.pkg.TypesInfo.TypeOf(n))
		}
		return l.runtimeCall(n, recipe.Runtime, symbol.Effect != catalog.EffectWrite, recipe.Prefix, args)
	}
	if symbol, ok := intrinsic.LookupObject(fn); ok {
		if symbol.Kind != intrinsic.RuntimeFunction {
			l.errorAt(n, "constant intrinsic cannot be called")
			return lowerValue{}
		}
		if symbol.Package == "math/rand" && symbol.Name == "Intn" && len(args) == 1 && len(args[0].slots) == 1 {
			if value, ok := args[0].slots[0].(ir.Const); ok && value.Value <= 0 {
				l.errorAt(n.Args[0], "rand.Intn constant bound must be positive")
				return zeroValue(l.pkg.TypesInfo.TypeOf(n))
			}
		}
		return l.runtimeCall(n, symbol.Runtime, true, symbol.Prefix, args)
	}
	if fn.Pkg() != nil && (fn.Pkg().Path() == "math" || fn.Pkg().Path() == "math/rand") {
		l.errorAt(n, "standard library symbol %s is not a Sonolus intrinsic", fn.FullName())
		return zeroValue(l.pkg.TypesInfo.TypeOf(n))
	}
	return l.inlineCall(n, fn, args)
}

func (l *lowerer) callReceiver(selector *ast.SelectorExpr, fn *types.Func) lowerValue {
	receiver := l.expr(selector.X)
	if receiver.container != nil {
		return receiver
	}
	signature, _ := fn.Type().(*types.Signature)
	if signature == nil || signature.Recv() == nil {
		return l.materialize("call.receiver", receiver, selector.X)
	}
	declared := types.Unalias(signature.Recv().Type())
	if pointer, ok := declared.(*types.Pointer); ok {
		if _, alreadyPointer := types.Unalias(receiver.type_).(*types.Pointer); !alreadyPointer {
			receiver = l.materializeAddressable("call.receiver", receiver, selector.X)
			receiver.type_ = pointer
		}
		return receiver
	}
	if pointer, ok := types.Unalias(receiver.type_).(*types.Pointer); ok {
		receiver.type_ = pointer.Elem()
	}
	copy := l.alloc("call.receiver", signature.Recv().Type())
	l.store(copy, receiver, selector.X)
	return copy
}

func (l *lowerer) builtinCall(n *ast.CallExpr, builtin *types.Builtin) lowerValue {
	if builtin.Name() != "len" && builtin.Name() != "cap" {
		l.errorAt(n, "Go builtin %s is not supported by the frozen callback subset", builtin.Name())
		return zeroValue(l.pkg.TypesInfo.TypeOf(n))
	}
	if len(n.Args) != 1 {
		l.errorAt(n, "Go builtin %s requires one argument", builtin.Name())
		return zeroValue(l.pkg.TypesInfo.TypeOf(n))
	}
	value := l.expr(n.Args[0])
	result := lowerValue{type_: l.pkg.TypesInfo.TypeOf(n)}
	if array, ok := types.Unalias(value.type_).Underlying().(*types.Array); ok {
		result.slots = []ir.Expr{ir.Const{Value: float64(array.Len())}}
		return result
	}
	if value.container != nil {
		if builtin.Name() == "len" {
			result.slots = []ir.Expr{value.slots[0]}
		} else {
			result.slots = []ir.Expr{ir.Const{Value: float64(value.container.capacity)}}
		}
		return result
	}
	l.errorAt(n, "Go builtin %s is only supported for fixed arrays and catalog containers", builtin.Name())
	return zeroValue(l.pkg.TypesInfo.TypeOf(n))
}

func (l *lowerer) materialize(name string, value lowerValue, node ast.Node) lowerValue {
	if value.container != nil || len(value.slots) == 0 {
		return value
	}
	if _, ok := types.Unalias(value.type_).(*types.Pointer); ok {
		return value
	}
	allConstant := true
	for _, slot := range value.slots {
		if _, ok := slot.(ir.Const); !ok {
			allConstant = false
			break
		}
	}
	if allConstant {
		return value
	}
	local := l.alloc(name, value.type_)
	l.store(local, value, node)
	return local
}

func (l *lowerer) materializeAddressable(name string, value lowerValue, node ast.Node) lowerValue {
	if len(value.places) == len(value.slots) && len(value.places) != 0 {
		return value
	}
	if value.container != nil || len(value.slots) == 0 {
		return value
	}
	local := l.alloc(name, value.type_)
	l.store(local, value, node)
	return local
}

func (l *lowerer) easeCall(n *ast.CallExpr) lowerValue {
	if len(n.Args) != 3 {
		l.errorAt(n, "Ease requires direction, curve, and value")
		return zeroValue(l.pkg.TypesInfo.TypeOf(n))
	}
	constantString := func(expr ast.Expr) (string, bool) {
		value := l.pkg.TypesInfo.Types[expr].Value
		if value == nil || value.Kind() != constant.String {
			return "", false
		}
		return constant.StringVal(value), true
	}
	direction, okDirection := constantString(n.Args[0])
	curve, okCurve := constantString(n.Args[1])
	if !okDirection || !okCurve {
		l.errorAt(n, "Ease direction and curve must be compile-time constants")
		return zeroValue(l.pkg.TypesInfo.TypeOf(n))
	}
	name := "Ease" + direction + curve
	var runtime resource.RuntimeFunction
	for _, symbol := range catalog.Symbols {
		if symbol.Package == "sonolus/native" && symbol.Name == name && symbol.Runtime != "" {
			runtime = symbol.Runtime
			break
		}
	}
	if runtime == "" {
		l.errorAt(n, "unsupported Ease combination %s/%s", direction, curve)
		return zeroValue(l.pkg.TypesInfo.TypeOf(n))
	}
	value := l.expr(n.Args[2])
	return l.runtimeCall(n, runtime, true, nil, []lowerValue{value})
}

func (l *lowerer) isFacadeReceiver(expr ast.Expr) bool {
	var object types.Object
	switch value := expr.(type) {
	case *ast.Ident:
		object = l.pkg.TypesInfo.ObjectOf(value)
	case *ast.SelectorExpr:
		if l.pkg.TypesInfo.Selections[value] == nil {
			object = l.pkg.TypesInfo.ObjectOf(value.Sel)
		}
	}
	if object == nil {
		return false
	}
	symbol, ok := catalog.LookupObject(object)
	return ok && symbol.Kind == catalog.KindVariable && irTypeOf(object.Type()).Slots == 0
}

func facadeReceiverName(expr ast.Expr) string {
	switch value := expr.(type) {
	case *ast.Ident:
		return value.Name
	case *ast.SelectorExpr:
		return value.Sel.Name
	default:
		return ""
	}
}

func (l *lowerer) memoryCall(n *ast.CallExpr, recipe catalog.Recipe, args []lowerValue) lowerValue {
	storage, offset := recipe.Storage, recipe.Offset
	if selector, ok := n.Fun.(*ast.SelectorExpr); ok {
		if receiver := facadeReceiverName(selector.X); receiver != "" {
			switch receiver {
			case "SafeArea":
				switch l.mode {
				case mode.ModePlay, mode.ModeWatch:
					offset = 5
				case mode.ModePreview:
					offset = 2
				case mode.ModeTutorial:
					offset = 3
				}
			case "TutorialData":
				storage = "TutorialData"
			case "SkinTransform":
				storage = "SkinTransform"
			case "ParticleTransform":
				storage = "ParticleTransform"
			}
		}
	}
	index := ir.Expr(ir.Const{})
	if recipe.IndexArg >= 0 {
		if recipe.IndexArg >= len(args) || len(args[recipe.IndexArg].slots) != 1 {
			l.errorAt(n, "memory recipe requires a scalar index argument")
			return zeroValue(l.pkg.TypesInfo.TypeOf(n))
		}
		index = args[recipe.IndexArg].slots[0]
	}
	if recipe.Write {
		valueArg := recipe.IndexArg + 1
		if valueArg < 0 {
			valueArg = 0
		}
		if valueArg >= len(args) {
			l.errorAt(n, "memory write recipe requires a value argument")
			return lowerValue{}
		}
		value := args[valueArg]
		places := make([]ir.Place, len(value.slots))
		for i := range places {
			places[i] = l.memory(storage, index, recipe.Stride, memorySlotOffset(storage, offset, i), true, true, n)
		}
		if err := l.builder.Store(places, ir.Value{Type: irTypeOf(value.type_), Slots: value.slots}, sourcePos(l.pkg, n.Pos())); err != nil {
			l.errorAt(n, "%v", err)
		}
		return lowerValue{}
	}
	t := l.pkg.TypesInfo.TypeOf(n)
	result := zeroValue(t)
	result.places = make([]ir.Place, len(result.slots))
	for i := range result.slots {
		place := l.memory(storage, index, recipe.Stride, memorySlotOffset(storage, offset, i), true, false, n)
		result.places[i] = place
		result.slots[i] = ir.Load{Place: place}
	}
	return result
}

func memorySlotOffset(storage string, base, slot int) int {
	if storage == "SkinTransform" || storage == "ParticleTransform" {
		offsets := [...]int{0, 1, 3, 4, 5, 7}
		if slot < len(offsets) {
			return base + offsets[slot]
		}
	}
	return base + slot
}

func containerTypes(t types.Type) (kind string, key, element types.Type, ok bool) {
	named, ok := types.Unalias(t).(*types.Named)
	if !ok {
		return "", nil, nil, false
	}
	name := named.Obj().Name()
	args := named.TypeArgs()
	switch name {
	case "VarArray", "ArraySet":
		if args.Len() == 1 {
			return name, nil, args.At(0), true
		}
	case "ArrayMap":
		if args.Len() == 2 {
			return name, args.At(0), args.At(1), true
		}
	}
	return "", nil, nil, false
}

func isContainerType(t types.Type) bool {
	_, _, _, ok := containerTypes(t)
	return ok
}

func sameContainerBacking(a, b lowerValue) bool {
	if a.container == nil || b.container == nil || a.container.dataLocal == nil || b.container.dataLocal == nil {
		return false
	}
	if a.container.kind != b.container.kind || a.container.capacity != b.container.capacity || a.container.stride != b.container.stride || a.container.dataLocal.ID != b.container.dataLocal.ID {
		return false
	}
	if len(a.places) == 0 || len(b.places) == 0 {
		return false
	}
	ap, aok := a.places[0].(ir.LocalPlace)
	bp, bok := b.places[0].(ir.LocalPlace)
	return aok && bok && ap.ID == bp.ID && ap.Offset == bp.Offset
}

func (l *lowerer) containerCall(n *ast.CallExpr, operation string, args []lowerValue) lowerValue {
	if strings.HasSuffix(operation, ".new") {
		if len(args) != 1 || len(args[0].slots) != 1 {
			l.errorAt(n, "container constructor requires one constant capacity")
			return lowerValue{}
		}
		capacityValue, constantCapacity := args[0].slots[0].(ir.Const)
		capacity := int(capacityValue.Value)
		if !constantCapacity || capacity <= 0 || float64(capacity) != capacityValue.Value {
			l.errorAt(n.Args[0], "container capacity must be a positive integer constant")
			return lowerValue{}
		}
		resultType := l.pkg.TypesInfo.TypeOf(n)
		kind, key, element, ok := containerTypes(resultType)
		if !ok {
			l.errorAt(n, "invalid container result type %s", resultType)
			return lowerValue{}
		}
		keySize := 0
		if key != nil {
			keySize = irTypeOf(key).Slots
		}
		stride := keySize + irTypeOf(element).Slots
		size := l.allocZeroed("container.size", types.Typ[types.Int], n)
		backingType := ir.Type{Name: resultType.String() + ".backing", Slots: capacity * stride}
		backing := l.builder.NewLocal("container.data", backingType)
		result := lowerValue{type_: resultType, slots: append(append([]ir.Expr{}, size.slots...), backing.Slots...), places: append(append([]ir.Place{}, size.places...), ir.Places(backing)...)}
		first, _ := ir.Places(backing)[0].(ir.LocalPlace)
		result.container = &containerValue{kind: kind, capacity: capacity, stride: stride, keySize: keySize, element: element, key: key, dataLocal: &first}
		return result
	}
	if len(args) == 0 || args[0].container == nil {
		l.errorAt(n, "container method receiver has no backing layout")
		return lowerValue{}
	}
	receiver, c := args[0], args[0].container
	size := receiver.slots[0]
	scalar := func(expr ir.Expr, t types.Type) lowerValue { return lowerValue{type_: t, slots: []ir.Expr{expr}} }
	indexInRange := func(index ir.Expr, allowEnd bool) ir.Expr {
		upper := resource.RuntimeFunctionLess
		if allowEnd {
			upper = resource.RuntimeFunctionLessOr
		}
		return l.pure(resource.RuntimeFunctionAnd, n,
			l.pure(resource.RuntimeFunctionGreaterOr, n, index, ir.Const{}),
			l.pure(upper, n, index, size))
	}
	hasCapacity := func() ir.Expr {
		return l.pure(resource.RuntimeFunctionLess, n, size, ir.Const{Value: float64(c.capacity)})
	}
	indexValue := func(index ir.Expr, offset, count int, t types.Type) lowerValue {
		return l.containerElement(n, c, index, offset, count, t)
	}
	switch operation {
	case "varArray.len", "arrayMap.len", "arraySet.len":
		return scalar(size, l.pkg.TypesInfo.TypeOf(n))
	case "varArray.capacity", "arrayMap.capacity", "arraySet.capacity":
		return scalar(ir.Const{Value: float64(c.capacity)}, l.pkg.TypesInfo.TypeOf(n))
	case "varArray.isFull":
		return scalar(l.pure(resource.RuntimeFunctionEqual, n, size, ir.Const{Value: float64(c.capacity)}), l.pkg.TypesInfo.TypeOf(n))
	case "varArray.get":
		if len(args) == 2 {
			l.guard(n, indexInRange(args[1].slots[0], false))
			return indexValue(args[1].slots[0], 0, c.stride, c.element)
		}
	case "varArray.set":
		if len(args) == 3 {
			l.guard(n, indexInRange(args[1].slots[0], false))
			l.store(indexValue(args[1].slots[0], 0, c.stride, c.element), args[2], n)
			return lowerValue{}
		}
	case "varArray.append":
		if len(args) == 2 {
			l.guard(n, hasCapacity())
			l.store(indexValue(size, 0, c.stride, c.element), args[1], n)
			l.store(lowerValue{type_: types.Typ[types.Int], slots: []ir.Expr{size}, places: receiver.places[:1]}, scalar(l.pure(resource.RuntimeFunctionAdd, n, size, ir.Const{Value: 1}), types.Typ[types.Int]), n)
			return lowerValue{}
		}
	case "varArray.pop":
		l.guard(n, l.pure(resource.RuntimeFunctionGreater, n, size, ir.Const{}))
		newSize := l.pure(resource.RuntimeFunctionSubtract, n, size, ir.Const{Value: 1})
		l.store(lowerValue{type_: types.Typ[types.Int], slots: []ir.Expr{size}, places: receiver.places[:1]}, scalar(newSize, types.Typ[types.Int]), n)
		return indexValue(newSize, 0, c.stride, c.element)
	case "varArray.clear", "arrayMap.clear", "arraySet.clear":
		l.store(lowerValue{type_: types.Typ[types.Int], slots: []ir.Expr{size}, places: receiver.places[:1]}, scalar(ir.Const{}, types.Typ[types.Int]), n)
		return lowerValue{}
	case "varArray.contains", "arraySet.contains":
		if len(args) == 2 {
			_, found := l.containerFind(n, receiver, args[1], 0, c.stride)
			return found
		}
	case "arrayMap.contains":
		if len(args) == 2 {
			_, found := l.containerFind(n, receiver, args[1], 0, c.keySize)
			return found
		}
	case "arrayMap.get":
		if len(args) == 2 {
			index, found := l.containerFind(n, receiver, args[1], 0, c.keySize)
			l.guard(n, found.slots[0])
			return indexValue(index.slots[0], c.keySize, c.stride-c.keySize, c.element)
		}
	case "arrayMap.set":
		if len(args) == 3 {
			index, found := l.containerFind(n, receiver, args[1], 0, c.keySize)
			existing, newEntry, merge := l.newBlock(), l.newBlock(), l.newBlock()
			_ = l.builder.Branch(found.slots[0], existing, newEntry)
			l.setCurrent(existing)
			l.store(indexValue(index.slots[0], c.keySize, c.stride-c.keySize, c.element), args[2], n)
			l.jump(merge)
			l.setCurrent(newEntry)
			l.guard(n, hasCapacity())
			l.store(indexValue(size, 0, c.keySize, c.key), args[1], n)
			l.store(indexValue(size, c.keySize, c.stride-c.keySize, c.element), args[2], n)
			l.store(lowerValue{type_: types.Typ[types.Int], slots: []ir.Expr{size}, places: receiver.places[:1]}, scalar(l.pure(resource.RuntimeFunctionAdd, n, size, ir.Const{Value: 1}), types.Typ[types.Int]), n)
			l.jump(merge)
			l.setCurrent(merge)
			return lowerValue{}
		}
	case "arraySet.add":
		if len(args) == 2 {
			_, found := l.containerFind(n, receiver, args[1], 0, c.stride)
			result := l.alloc("set.added", types.Typ[types.Bool])
			exists, add, merge := l.newBlock(), l.newBlock(), l.newBlock()
			_ = l.builder.Branch(found.slots[0], exists, add)
			l.setCurrent(exists)
			l.store(result, scalar(ir.Const{}, types.Typ[types.Bool]), n)
			l.jump(merge)
			l.setCurrent(add)
			l.guard(n, hasCapacity())
			l.store(indexValue(size, 0, c.stride, c.element), args[1], n)
			l.store(lowerValue{type_: types.Typ[types.Int], slots: []ir.Expr{size}, places: receiver.places[:1]}, scalar(l.pure(resource.RuntimeFunctionAdd, n, size, ir.Const{Value: 1}), types.Typ[types.Int]), n)
			l.store(result, scalar(ir.Const{Value: 1}, types.Typ[types.Bool]), n)
			l.jump(merge)
			l.setCurrent(merge)
			return result
		}
	case "varArray.insert":
		if len(args) == 3 {
			l.guard(n, l.pure(resource.RuntimeFunctionAnd, n, indexInRange(args[1].slots[0], true), hasCapacity()))
			l.containerInsert(n, receiver, args[1], args[2])
			return lowerValue{}
		}
	case "arraySet.remove":
		if len(args) == 2 {
			index, found := l.containerFind(n, receiver, args[1], 0, c.stride)
			result := l.alloc("set.removed", types.Typ[types.Bool])
			remove, missing, merge := l.newBlock(), l.newBlock(), l.newBlock()
			_ = l.builder.Branch(found.slots[0], remove, missing)
			l.setCurrent(remove)
			l.containerRemoveAt(n, receiver, index)
			l.store(result, scalar(ir.Const{Value: 1}, types.Typ[types.Bool]), n)
			l.jump(merge)
			l.setCurrent(missing)
			l.store(result, scalar(ir.Const{}, types.Typ[types.Bool]), n)
			l.jump(merge)
			l.setCurrent(merge)
			return result
		}
	case "arrayMap.delete":
		if len(args) == 2 {
			index, found := l.containerFind(n, receiver, args[1], 0, c.keySize)
			result := l.alloc("map.deleted", types.Typ[types.Bool])
			remove, missing, merge := l.newBlock(), l.newBlock(), l.newBlock()
			_ = l.builder.Branch(found.slots[0], remove, missing)
			l.setCurrent(remove)
			l.containerRemoveAt(n, receiver, index)
			l.store(result, scalar(ir.Const{Value: 1}, types.Typ[types.Bool]), n)
			l.jump(merge)
			l.setCurrent(missing)
			l.store(result, scalar(ir.Const{}, types.Typ[types.Bool]), n)
			l.jump(merge)
			l.setCurrent(merge)
			return result
		}
	}
	l.errorAt(n, "container operation %s is not implemented for this layout", operation)
	return zeroValue(l.pkg.TypesInfo.TypeOf(n))
}

func (l *lowerer) guard(n ast.Node, condition ir.Expr) {
	valid, invalid := l.newBlock(), l.newBlock()
	_ = l.builder.Branch(condition, valid, invalid)
	l.setCurrent(invalid)
	_ = l.builder.MarkUnreachable()
	l.setCurrent(valid)
}

func (l *lowerer) containerSizeValue(receiver lowerValue) lowerValue {
	return lowerValue{type_: types.Typ[types.Int], slots: receiver.slots[:1], places: receiver.places[:1]}
}

func (l *lowerer) containerRemoveAt(n ast.Node, receiver, index lowerValue) {
	c := receiver.container
	cursor := l.alloc("container.remove.index", types.Typ[types.Int])
	l.store(cursor, index, n)
	header, body, exit := l.newBlock(), l.newBlock(), l.newBlock()
	l.jump(header)
	l.setCurrent(header)
	last := l.pure(resource.RuntimeFunctionSubtract, n, receiver.slots[0], ir.Const{Value: 1})
	_ = l.builder.Branch(l.pure(resource.RuntimeFunctionLess, n, cursor.slots[0], last), body, exit)
	l.setCurrent(body)
	next := l.pure(resource.RuntimeFunctionAdd, n, cursor.slots[0], ir.Const{Value: 1})
	source := l.containerElement(n, c, next, 0, c.stride, types.Typ[types.Float64])
	target := l.containerElement(n, c, cursor.slots[0], 0, c.stride, types.Typ[types.Float64])
	if err := l.builder.Store(target.places, ir.Value{Type: ir.Type{Name: "container.entry", Slots: c.stride}, Slots: source.slots}, sourcePos(l.pkg, n.Pos())); err != nil {
		l.errorAt(n, "%v", err)
	}
	l.store(cursor, lowerValue{type_: types.Typ[types.Int], slots: []ir.Expr{next}}, n)
	l.jump(header)
	l.setCurrent(exit)
	l.store(l.containerSizeValue(receiver), lowerValue{type_: types.Typ[types.Int], slots: []ir.Expr{last}}, n)
}

func (l *lowerer) containerInsert(n ast.Node, receiver, index, value lowerValue) {
	c := receiver.container
	cursor := l.alloc("container.insert.index", types.Typ[types.Int])
	l.store(cursor, l.containerSizeValue(receiver), n)
	header, body, exit := l.newBlock(), l.newBlock(), l.newBlock()
	l.jump(header)
	l.setCurrent(header)
	_ = l.builder.Branch(l.pure(resource.RuntimeFunctionGreater, n, cursor.slots[0], index.slots[0]), body, exit)
	l.setCurrent(body)
	previous := l.pure(resource.RuntimeFunctionSubtract, n, cursor.slots[0], ir.Const{Value: 1})
	source := l.containerElement(n, c, previous, 0, c.stride, c.element)
	target := l.containerElement(n, c, cursor.slots[0], 0, c.stride, c.element)
	l.store(target, source, n)
	l.store(cursor, lowerValue{type_: types.Typ[types.Int], slots: []ir.Expr{previous}}, n)
	l.jump(header)
	l.setCurrent(exit)
	l.store(l.containerElement(n, c, index.slots[0], 0, c.stride, c.element), value, n)
	l.store(l.containerSizeValue(receiver), lowerValue{type_: types.Typ[types.Int], slots: []ir.Expr{l.pure(resource.RuntimeFunctionAdd, n, receiver.slots[0], ir.Const{Value: 1})}}, n)
}

func (l *lowerer) containerElement(n ast.Node, c *containerValue, index ir.Expr, offset, count int, t types.Type) lowerValue {
	v := lowerValue{type_: t, slots: make([]ir.Expr, count), places: make([]ir.Place, count)}
	for i := 0; i < count; i++ {
		var p ir.Place
		if c.dataLocal != nil {
			p = l.indexedLocal(*c.dataLocal, index, c.capacity, c.stride, offset+i, n)
		} else {
			p = l.memory(c.memoryStorage, index, c.stride, c.memoryBase+offset+i, c.memoryRead, c.memoryWrite, n)
		}
		v.places[i], v.slots[i] = p, ir.Load{Place: p}
	}
	return v
}

func (l *lowerer) containerFind(n ast.Node, receiver, needle lowerValue, offset, count int) (lowerValue, lowerValue) {
	c := receiver.container
	index := l.alloc("container.find.index", types.Typ[types.Int])
	found := l.alloc("container.find.found", types.Typ[types.Bool])
	l.store(index, lowerValue{type_: types.Typ[types.Int], slots: []ir.Expr{ir.Const{}}}, n)
	l.store(found, lowerValue{type_: types.Typ[types.Bool], slots: []ir.Expr{ir.Const{}}}, n)
	header, body, match, next, exit := l.newBlock(), l.newBlock(), l.newBlock(), l.newBlock(), l.newBlock()
	l.jump(header)
	l.setCurrent(header)
	_ = l.builder.Branch(l.pure(resource.RuntimeFunctionLess, n, index.slots[0], receiver.slots[0]), body, exit)
	l.setCurrent(body)
	candidate := l.containerElement(n, c, index.slots[0], offset, count, needle.type_)
	var equal ir.Expr = ir.Const{Value: 1}
	for i := 0; i < count; i++ {
		equal = l.pure(resource.RuntimeFunctionAnd, n, equal, l.pure(resource.RuntimeFunctionEqual, n, candidate.slots[i], needle.slots[i]))
	}
	_ = l.builder.Branch(equal, match, next)
	l.setCurrent(match)
	l.store(found, lowerValue{type_: types.Typ[types.Bool], slots: []ir.Expr{ir.Const{Value: 1}}}, n)
	l.jump(exit)
	l.setCurrent(next)
	l.store(index, lowerValue{type_: types.Typ[types.Int], slots: []ir.Expr{l.pure(resource.RuntimeFunctionAdd, n, index.slots[0], ir.Const{Value: 1})}}, n)
	l.jump(header)
	l.setCurrent(exit)
	return index, found
}

func (l *lowerer) resourceCall(n *ast.CallExpr, operation string, args []lowerValue) lowerValue {
	if operation == "archetype.spawn" {
		if len(args) != 1 {
			l.errorAt(n, "Spawn requires one archetype value")
			return lowerValue{}
		}
		named, ok := namedType(args[0].type_)
		binding, exists := l.archetypes[named]
		if !ok || !exists {
			l.errorAt(n, "Spawn argument type %s is not an archetype declared in %s mode", args[0].type_, l.mode)
			return lowerValue{}
		}
		callArgs := []ir.Expr{ir.Const{Value: float64(binding.id)}}
		for _, field := range binding.declaration.Fields {
			if field.Storage != "memory" {
				continue
			}
			start, end := field.ReceiverOffset, field.ReceiverOffset+field.Size
			if start < 0 || end > len(args[0].slots) {
				l.errorAt(n, "Spawn data layout for %s is incomplete", binding.declaration.Name)
				return lowerValue{}
			}
			callArgs = append(callArgs, args[0].slots[start:end]...)
		}
		call := l.builder.RuntimeCall(resource.RuntimeFunctionSpawn, callArgs, ir.Type{Name: "void"}, false, sourcePos(l.pkg, n.Pos()))
		_ = l.builder.Eval(call)
		return lowerValue{}
	}
	if operation == "instruction.show" || operation == "instruction.clear" {
		value := ir.Expr(ir.Const{Value: -1})
		if operation == "instruction.show" {
			if len(args) != 1 || len(args[0].slots) != 1 {
				l.errorAt(n, "instruction show requires one text handle")
				return lowerValue{}
			}
			value = args[0].slots[0]
		}
		place := l.memory("TutorialInstruction", ir.Const{}, 0, 0, true, true, n)
		if err := l.builder.Store([]ir.Place{place}, ir.Value{Type: ir.Type{Name: "instruction", Slots: 1}, Slots: []ir.Expr{value}}, sourcePos(l.pkg, n.Pos())); err != nil {
			l.errorAt(n, "%v", err)
		}
		return lowerValue{}
	}
	flat := func(indices ...int) ([]ir.Expr, bool) {
		var out []ir.Expr
		for _, index := range indices {
			if index >= len(args) {
				l.errorAt(n, "resource operation %s argument mismatch", operation)
				return nil, false
			}
			out = append(out, args[index].slots...)
		}
		return out, true
	}
	var runtime resource.RuntimeFunction
	pure := false
	var callArgs []ir.Expr
	var returns bool
	switch operation {
	case "sprite.draw":
		values, ok := flat(0, 1)
		if !ok || len(values) != 9 || len(args) < 4 {
			l.errorAt(n, "sprite draw layout mismatch")
			return lowerValue{}
		}
		z, alpha := args[2].slots[0], args[3].slots[0]
		callArgs = append(values, z, alpha, z, z, z)
		runtime = resource.RuntimeFunctionDraw
	case "sprite.drawCurvedB", "sprite.drawCurvedT", "sprite.drawCurvedL", "sprite.drawCurvedR":
		values, ok := flat(0, 1)
		if !ok || len(values) != 9 || len(args) != 6 || len(args[2].slots) != 2 {
			l.errorAt(n, "single-edge curved sprite draw layout mismatch")
			return lowerValue{}
		}
		control, segments, z, alpha := args[2].slots, args[3].slots[0], args[4].slots[0], args[5].slots[0]
		callArgs = append(values, z, alpha, segments, control[0], control[1], z, z, z)
		runtime = map[string]resource.RuntimeFunction{
			"sprite.drawCurvedB": resource.RuntimeFunctionDrawCurvedB,
			"sprite.drawCurvedT": resource.RuntimeFunctionDrawCurvedT,
			"sprite.drawCurvedL": resource.RuntimeFunctionDrawCurvedL,
			"sprite.drawCurvedR": resource.RuntimeFunctionDrawCurvedR,
		}[operation]
	case "sprite.drawCurvedBT", "sprite.drawCurvedLR":
		values, ok := flat(0, 1)
		if !ok || len(values) != 9 || len(args) != 7 || len(args[2].slots) != 2 || len(args[3].slots) != 2 {
			l.errorAt(n, "two-edge curved sprite draw layout mismatch")
			return lowerValue{}
		}
		first, second := args[2].slots, args[3].slots
		segments, z, alpha := args[4].slots[0], args[5].slots[0], args[6].slots[0]
		callArgs = append(values, z, alpha, segments, first[0], first[1], second[0], second[1], z, z, z)
		if operation == "sprite.drawCurvedBT" {
			runtime = resource.RuntimeFunctionDrawCurvedBT
		} else {
			runtime = resource.RuntimeFunctionDrawCurvedLR
		}
	case "judgment.judge":
		values, ok := flat(1, 2, 0)
		if !ok || len(values) != 8 {
			l.errorAt(n, "judgment window layout mismatch")
			return lowerValue{}
		}
		callArgs = values
		runtime = resource.RuntimeFunctionJudge
		pure, returns = true, true
	case "sprite.exists":
		callArgs, _ = flat(0)
		runtime = resource.RuntimeFunctionHasSkinSprite
		pure, returns = true, true
	case "clip.play":
		callArgs, _ = flat(0, 1)
		runtime = resource.RuntimeFunctionPlay
	case "clip.playScheduled":
		callArgs, _ = flat(0, 1, 2)
		runtime = resource.RuntimeFunctionPlayScheduled
	case "clip.playLooped":
		callArgs, _ = flat(0)
		runtime = resource.RuntimeFunctionPlayLooped
		returns = true
	case "clip.playLoopedScheduled":
		callArgs, _ = flat(0, 1)
		runtime = resource.RuntimeFunctionPlayLoopedScheduled
		returns = true
	case "loop.stop":
		callArgs, _ = flat(0)
		runtime = resource.RuntimeFunctionStopLooped
	case "loop.stopScheduled":
		callArgs, _ = flat(0, 1)
		runtime = resource.RuntimeFunctionStopLoopedScheduled
	case "particle.spawn":
		callArgs, _ = flat(0, 1, 2, 3)
		runtime = resource.RuntimeFunctionSpawnParticleEffect
		returns = true
	case "particle.move":
		callArgs, _ = flat(0, 1)
		runtime = resource.RuntimeFunctionMoveParticleEffect
	case "particle.destroy":
		callArgs, _ = flat(0)
		runtime = resource.RuntimeFunctionDestroyParticleEffect
	default:
		l.errorAt(n, "unknown resource recipe %s", operation)
		return lowerValue{}
	}
	callResult := ir.Type{Name: "void"}
	if returns {
		callResult = irTypeOf(l.pkg.TypesInfo.TypeOf(n))
	}
	call := l.builder.RuntimeCall(runtime, callArgs, callResult, pure, sourcePos(l.pkg, n.Pos()))
	if !returns {
		if !pure {
			_ = l.builder.Eval(call)
		}
		return lowerValue{}
	}
	resultType := l.pkg.TypesInfo.TypeOf(n)
	if pure {
		return lowerValue{type_: resultType, slots: []ir.Expr{call}}
	}
	result := l.alloc("runtime.result", resultType)
	if err := l.builder.Store(result.places, ir.Value{Type: irTypeOf(resultType), Slots: []ir.Expr{call}}, sourcePos(l.pkg, n.Pos())); err != nil {
		l.errorAt(n, "%v", err)
	}
	return result
}

func (l *lowerer) pure(op resource.RuntimeFunction, pos ast.Node, args ...ir.Expr) ir.Expr {
	return l.builder.RuntimeCall(op, args, ir.Type{Name: "number", Slots: 1}, true, sourcePos(l.pkg, pos.Pos()))
}

func (l *lowerer) aggregateCall(n *ast.CallExpr, operation string, args []lowerValue) lowerValue {
	result := lowerValue{type_: l.pkg.TypesInfo.TypeOf(n)}
	if result.type_ == nil {
		l.errorAt(n, "aggregate operation %s has no result", operation)
		return result
	}
	require := func(index, slots int) bool {
		if index >= len(args) || len(args[index].slots) != slots {
			l.errorAt(n, "aggregate operation %s received an invalid layout", operation)
			return false
		}
		return true
	}
	op := func(fn resource.RuntimeFunction, values ...ir.Expr) ir.Expr { return l.pure(fn, n, values...) }
	add := func(a, b ir.Expr) ir.Expr { return op(resource.RuntimeFunctionAdd, a, b) }
	sub := func(a, b ir.Expr) ir.Expr { return op(resource.RuntimeFunctionSubtract, a, b) }
	mul := func(a, b ir.Expr) ir.Expr { return op(resource.RuntimeFunctionMultiply, a, b) }
	div := func(a, b ir.Expr) ir.Expr { return op(resource.RuntimeFunctionDivide, a, b) }
	rotate := func(x, y, angle ir.Expr) []ir.Expr {
		c, s := op(resource.RuntimeFunctionCos, angle), op(resource.RuntimeFunctionSin, angle)
		return []ir.Expr{sub(mul(x, c), mul(y, s)), add(mul(x, s), mul(y, c))}
	}
	compose := func(a, b []ir.Expr) []ir.Expr {
		return []ir.Expr{
			add(mul(b[0], a[0]), mul(b[1], a[3])), add(mul(b[0], a[1]), mul(b[1], a[4])), add(add(mul(b[0], a[2]), mul(b[1], a[5])), b[2]),
			add(mul(b[3], a[0]), mul(b[4], a[3])), add(mul(b[3], a[1]), mul(b[4], a[4])), add(add(mul(b[3], a[2]), mul(b[4], a[5])), b[5]),
		}
	}
	switch operation {
	case "touch.get":
		if !require(0, 1) {
			break
		}
		index := args[0].slots[0]
		count := ir.Load{Place: l.memory("RuntimeUpdate", ir.Const{}, 0, 3, true, false, n)}
		lowerBound := op(resource.RuntimeFunctionGreaterOr, index, ir.Const{})
		upperBound := op(resource.RuntimeFunctionLess, index, count)
		l.guard(n, op(resource.RuntimeFunctionAnd, lowerBound, upperBound))
		result.slots = make([]ir.Expr, 15)
		result.places = make([]ir.Place, 15)
		for i := range result.slots {
			place := l.memory("RuntimeTouch", index, 15, i, true, false, n)
			result.places[i] = place
			result.slots[i] = ir.Load{Place: place}
		}
	case "screen.rect":
		aspect := ir.Load{Place: l.memory("RuntimeEnvironment", ir.Const{}, 0, 1, true, false, n)}
		receiver := ""
		if selector, ok := n.Fun.(*ast.SelectorExpr); ok {
			receiver = facadeReceiverName(selector.X)
		}
		if receiver == "Screen" {
			result.slots = []ir.Expr{ir.Const{Value: 1}, aspect, ir.Const{Value: -1}, op(resource.RuntimeFunctionNegate, aspect)}
			break
		}
		base := 0
		switch l.mode {
		case mode.ModePlay, mode.ModeWatch:
			base = 5
		case mode.ModePreview:
			base = 2
		case mode.ModeTutorial:
			base = 3
		}
		xMin := ir.Load{Place: l.memory("RuntimeEnvironment", ir.Const{}, 0, base, true, false, n)}
		xMax := ir.Load{Place: l.memory("RuntimeEnvironment", ir.Const{}, 0, base+1, true, false, n)}
		yMin := ir.Load{Place: l.memory("RuntimeEnvironment", ir.Const{}, 0, base+2, true, false, n)}
		yMax := ir.Load{Place: l.memory("RuntimeEnvironment", ir.Const{}, 0, base+3, true, false, n)}
		result.slots = []ir.Expr{yMax, xMax, yMin, xMin}
	case "time.previous", "time.offsetAdjusted":
		now := ir.Load{Place: l.memory("RuntimeUpdate", ir.Const{}, 0, 0, true, false, n)}
		offset := 1
		storage := "RuntimeUpdate"
		if operation == "time.offsetAdjusted" {
			offset = 3
			storage = "RuntimeEnvironment"
		}
		other := ir.Load{Place: l.memory(storage, ir.Const{}, 0, offset, true, false, n)}
		result.slots = []ir.Expr{op(resource.RuntimeFunctionSubtract, now, other)}
	case "vec2.new":
		if require(0, 1) && require(1, 1) {
			result.slots = []ir.Expr{args[0].slots[0], args[1].slots[0]}
		}
	case "vec2.add", "vec2.sub":
		if require(0, 2) && require(1, 2) {
			op := resource.RuntimeFunctionAdd
			if operation == "vec2.sub" {
				op = resource.RuntimeFunctionSubtract
			}
			result.slots = []ir.Expr{l.pure(op, n, args[0].slots[0], args[1].slots[0]), l.pure(op, n, args[0].slots[1], args[1].slots[1])}
		}
	case "vec2.mul", "vec2.div":
		if require(0, 2) && require(1, 1) {
			op := resource.RuntimeFunctionMultiply
			if operation == "vec2.div" {
				op = resource.RuntimeFunctionDivide
			}
			result.slots = []ir.Expr{l.pure(op, n, args[0].slots[0], args[1].slots[0]), l.pure(op, n, args[0].slots[1], args[1].slots[0])}
		}
	case "vec2.dot":
		if require(0, 2) && require(1, 2) {
			result.slots = []ir.Expr{l.pure(resource.RuntimeFunctionAdd, n, l.pure(resource.RuntimeFunctionMultiply, n, args[0].slots[0], args[1].slots[0]), l.pure(resource.RuntimeFunctionMultiply, n, args[0].slots[1], args[1].slots[1]))}
		}
	case "vec2.magnitudeSquared", "vec2.magnitude":
		if require(0, 2) {
			square := l.pure(resource.RuntimeFunctionAdd, n, l.pure(resource.RuntimeFunctionMultiply, n, args[0].slots[0], args[0].slots[0]), l.pure(resource.RuntimeFunctionMultiply, n, args[0].slots[1], args[0].slots[1]))
			if operation == "vec2.magnitude" {
				square = l.pure(resource.RuntimeFunctionPower, n, square, ir.Const{Value: 0.5})
			}
			result.slots = []ir.Expr{square}
		}
	case "vec2.angle":
		if require(0, 2) {
			result.slots = []ir.Expr{l.pure(resource.RuntimeFunctionArctan2, n, args[0].slots[1], args[0].slots[0])}
		}
	case "vec2.orthogonal":
		if require(0, 2) {
			result.slots = []ir.Expr{l.pure(resource.RuntimeFunctionNegate, n, args[0].slots[1]), args[0].slots[0]}
		}
	case "vec2.normalize", "vec2.normalizeOrZero":
		if require(0, 2) {
			x, y := args[0].slots[0], args[0].slots[1]
			magnitude := op(resource.RuntimeFunctionPower, add(mul(x, x), mul(y, y)), ir.Const{Value: 0.5})
			zero := op(resource.RuntimeFunctionAnd, op(resource.RuntimeFunctionEqual, x, ir.Const{}), op(resource.RuntimeFunctionEqual, y, ir.Const{}))
			if operation == "vec2.normalizeOrZero" {
				magnitude = add(magnitude, zero)
			} else {
				l.guard(n, op(resource.RuntimeFunctionNot, zero))
			}
			result.slots = []ir.Expr{div(x, magnitude), div(y, magnitude)}
		}
	case "vec2.rotate":
		if require(0, 2) && require(1, 1) {
			result.slots = rotate(args[0].slots[0], args[0].slots[1], args[1].slots[0])
		}
	case "vec2.rotateAbout":
		if require(0, 2) && require(1, 2) && require(2, 1) {
			rotated := rotate(sub(args[0].slots[0], args[1].slots[0]), sub(args[0].slots[1], args[1].slots[1]), args[2].slots[0])
			result.slots = []ir.Expr{add(rotated[0], args[1].slots[0]), add(rotated[1], args[1].slots[1])}
		}
	case "vec2.angleDiff", "vec2.signedAngleDiff":
		if require(0, 2) && require(1, 2) {
			a := op(resource.RuntimeFunctionArctan2, args[0].slots[1], args[0].slots[0])
			b := op(resource.RuntimeFunctionArctan2, args[1].slots[1], args[1].slots[0])
			diff := sub(op(resource.RuntimeFunctionMod, add(sub(a, b), ir.Const{Value: 3.141592653589793}), ir.Const{Value: 6.283185307179586}), ir.Const{Value: 3.141592653589793})
			if operation == "vec2.angleDiff" {
				diff = op(resource.RuntimeFunctionAbs, diff)
			}
			result.slots = []ir.Expr{diff}
		}
	case "rect.width":
		if require(0, 4) {
			result.slots = []ir.Expr{l.pure(resource.RuntimeFunctionSubtract, n, args[0].slots[1], args[0].slots[3])}
		}
	case "rect.height":
		if require(0, 4) {
			result.slots = []ir.Expr{l.pure(resource.RuntimeFunctionSubtract, n, args[0].slots[0], args[0].slots[2])}
		}
	case "rect.center":
		if require(0, 4) {
			result.slots = []ir.Expr{l.pure(resource.RuntimeFunctionDivide, n, l.pure(resource.RuntimeFunctionAdd, n, args[0].slots[3], args[0].slots[1]), ir.Const{Value: 2}), l.pure(resource.RuntimeFunctionDivide, n, l.pure(resource.RuntimeFunctionAdd, n, args[0].slots[2], args[0].slots[0]), ir.Const{Value: 2})}
		}
	case "rect.translate":
		if require(0, 4) && require(1, 2) {
			result.slots = []ir.Expr{l.pure(resource.RuntimeFunctionAdd, n, args[0].slots[0], args[1].slots[1]), l.pure(resource.RuntimeFunctionAdd, n, args[0].slots[1], args[1].slots[0]), l.pure(resource.RuntimeFunctionAdd, n, args[0].slots[2], args[1].slots[1]), l.pure(resource.RuntimeFunctionAdd, n, args[0].slots[3], args[1].slots[0])}
		}
	case "rect.scale":
		if require(0, 4) && require(1, 1) {
			for _, slot := range args[0].slots {
				result.slots = append(result.slots, l.pure(resource.RuntimeFunctionMultiply, n, slot, args[1].slots[0]))
			}
		}
	case "rect.toQuad":
		if require(0, 4) {
			t, r, b, left := args[0].slots[0], args[0].slots[1], args[0].slots[2], args[0].slots[3]
			result.slots = []ir.Expr{left, b, left, t, r, t, r, b}
		}
	case "rect.contains":
		if require(0, 4) && require(1, 2) {
			x, y := args[1].slots[0], args[1].slots[1]
			result.slots = []ir.Expr{op(resource.RuntimeFunctionAnd,
				op(resource.RuntimeFunctionAnd, op(resource.RuntimeFunctionGreaterOr, x, args[0].slots[3]), op(resource.RuntimeFunctionLessOr, x, args[0].slots[1])),
				op(resource.RuntimeFunctionAnd, op(resource.RuntimeFunctionGreaterOr, y, args[0].slots[2]), op(resource.RuntimeFunctionLessOr, y, args[0].slots[0])))}
		}
	case "quad.center":
		if require(0, 8) {
			result.slots = []ir.Expr{div(add(add(args[0].slots[0], args[0].slots[2]), add(args[0].slots[4], args[0].slots[6])), ir.Const{Value: 4}), div(add(add(args[0].slots[1], args[0].slots[3]), add(args[0].slots[5], args[0].slots[7])), ir.Const{Value: 4})}
		}
	case "quad.translate":
		if require(0, 8) && require(1, 2) {
			for i, value := range args[0].slots {
				result.slots = append(result.slots, add(value, args[1].slots[i%2]))
			}
		}
	case "quad.scale":
		if require(0, 8) && require(1, 1) {
			for _, value := range args[0].slots {
				result.slots = append(result.slots, mul(value, args[1].slots[0]))
			}
		}
	case "quad.rotate":
		if require(0, 8) && require(1, 1) {
			for i := 0; i < 8; i += 2 {
				result.slots = append(result.slots, rotate(args[0].slots[i], args[0].slots[i+1], args[1].slots[0])...)
			}
		}
	case "quad.permute":
		if require(0, 8) && require(1, 1) {
			value := l.alloc("quad.permute", result.type_)
			exit := l.newBlock()
			blocks := []*ir.Block{l.newBlock(), l.newBlock(), l.newBlock(), l.newBlock()}
			cases := make([]ir.SwitchCase, 4)
			for i := range blocks {
				cases[i] = ir.SwitchCase{Value: float64(i), Target: blocks[i].ID}
			}
			rotation := op(resource.RuntimeFunctionMod, args[1].slots[0], ir.Const{Value: 4})
			_ = l.builder.Switch(rotation, cases, blocks[0])
			orders := [4][4]int{{0, 1, 2, 3}, {3, 0, 1, 2}, {2, 3, 0, 1}, {1, 2, 3, 0}}
			for i, block := range blocks {
				l.setCurrent(block)
				var slots []ir.Expr
				for _, corner := range orders[i] {
					slots = append(slots, args[0].slots[corner*2:corner*2+2]...)
				}
				l.store(value, lowerValue{type_: result.type_, slots: slots}, n)
				l.jump(exit)
			}
			l.setCurrent(exit)
			result = value
		}
	case "quad.contains":
		if require(0, 8) && require(1, 2) {
			px, py := args[1].slots[0], args[1].slots[1]
			var inside ir.Expr = ir.Const{}
			for i := 0; i < 4; i++ {
				j := (i + 3) % 4
				xi, yi := args[0].slots[i*2], args[0].slots[i*2+1]
				xj, yj := args[0].slots[j*2], args[0].slots[j*2+1]
				straddles := op(resource.RuntimeFunctionNotEqual, op(resource.RuntimeFunctionGreater, yi, py), op(resource.RuntimeFunctionGreater, yj, py))
				intersection := add(div(mul(sub(xj, xi), sub(py, yi)), sub(yj, yi)), xi)
				crosses := op(resource.RuntimeFunctionAnd, straddles, op(resource.RuntimeFunctionLess, px, intersection))
				inside = op(resource.RuntimeFunctionNotEqual, inside, crosses)
			}
			result.slots = []ir.Expr{inside}
		}
	case "quad.top":
		if require(0, 8) {
			result.slots = []ir.Expr{div(add(args[0].slots[2], args[0].slots[4]), ir.Const{Value: 2}), div(add(args[0].slots[3], args[0].slots[5]), ir.Const{Value: 2})}
		}
	case "quad.right":
		if require(0, 8) {
			result.slots = []ir.Expr{div(add(args[0].slots[4], args[0].slots[6]), ir.Const{Value: 2}), div(add(args[0].slots[5], args[0].slots[7]), ir.Const{Value: 2})}
		}
	case "quad.bottom":
		if require(0, 8) {
			result.slots = []ir.Expr{div(add(args[0].slots[0], args[0].slots[6]), ir.Const{Value: 2}), div(add(args[0].slots[1], args[0].slots[7]), ir.Const{Value: 2})}
		}
	case "quad.left":
		if require(0, 8) {
			result.slots = []ir.Expr{div(add(args[0].slots[0], args[0].slots[2]), ir.Const{Value: 2}), div(add(args[0].slots[1], args[0].slots[3]), ir.Const{Value: 2})}
		}
	case "transform.translate":
		if require(0, 6) && require(1, 2) {
			result.slots = append([]ir.Expr(nil), args[0].slots...)
			result.slots[2] = add(result.slots[2], args[1].slots[0])
			result.slots[5] = add(result.slots[5], args[1].slots[1])
		}
	case "transform.scale":
		if require(0, 6) && require(1, 2) {
			result.slots = []ir.Expr{mul(args[1].slots[0], args[0].slots[0]), mul(args[1].slots[0], args[0].slots[1]), mul(args[1].slots[0], args[0].slots[2]), mul(args[1].slots[1], args[0].slots[3]), mul(args[1].slots[1], args[0].slots[4]), mul(args[1].slots[1], args[0].slots[5])}
		}
	case "transform.rotate":
		if require(0, 6) && require(1, 1) {
			c, s := op(resource.RuntimeFunctionCos, args[1].slots[0]), op(resource.RuntimeFunctionSin, args[1].slots[0])
			result.slots = compose(args[0].slots, []ir.Expr{c, op(resource.RuntimeFunctionNegate, s), ir.Const{}, s, c, ir.Const{}})
		}
	case "transform.compose", "transform.composeBefore":
		if require(0, 6) && require(1, 6) {
			if operation == "transform.compose" {
				result.slots = compose(args[0].slots, args[1].slots)
			} else {
				result.slots = compose(args[1].slots, args[0].slots)
			}
		}
	case "transform.scaleAbout":
		if require(0, 6) && require(1, 2) && require(2, 2) {
			sx, sy, px, py := args[1].slots[0], args[1].slots[1], args[2].slots[0], args[2].slots[1]
			result.slots = compose(args[0].slots, []ir.Expr{sx, ir.Const{}, mul(px, sub(ir.Const{Value: 1}, sx)), ir.Const{}, sy, mul(py, sub(ir.Const{Value: 1}, sy))})
		}
	case "transform.rotateAbout":
		if require(0, 6) && require(1, 1) && require(2, 2) {
			angle, px, py := args[1].slots[0], args[2].slots[0], args[2].slots[1]
			c, s := op(resource.RuntimeFunctionCos, angle), op(resource.RuntimeFunctionSin, angle)
			tx := add(sub(px, mul(c, px)), mul(s, py))
			ty := sub(sub(py, mul(s, px)), mul(c, py))
			result.slots = compose(args[0].slots, []ir.Expr{c, op(resource.RuntimeFunctionNegate, s), tx, s, c, ty})
		}
	case "transform.vec":
		if require(0, 6) && require(1, 2) {
			result.slots = []ir.Expr{add(add(mul(args[0].slots[0], args[1].slots[0]), mul(args[0].slots[1], args[1].slots[1])), args[0].slots[2]), add(add(mul(args[0].slots[3], args[1].slots[0]), mul(args[0].slots[4], args[1].slots[1])), args[0].slots[5])}
		}
	case "transform.quad":
		if require(0, 6) && require(1, 8) {
			for i := 0; i < 8; i += 2 {
				result.slots = append(result.slots, add(add(mul(args[0].slots[0], args[1].slots[i]), mul(args[0].slots[1], args[1].slots[i+1])), args[0].slots[2]), add(add(mul(args[0].slots[3], args[1].slots[i]), mul(args[0].slots[4], args[1].slots[i+1])), args[0].slots[5]))
			}
		}
	default:
		l.errorAt(n, "unknown aggregate recipe %s", operation)
	}
	if len(result.slots) != irTypeOf(result.type_).Slots {
		l.errorAt(n, "aggregate recipe %s returned %d slots; expected %d", operation, len(result.slots), irTypeOf(result.type_).Slots)
	}
	return result
}

func (l *lowerer) inlineLiteral(call *ast.CallExpr, literal *ast.FuncLit, args []lowerValue) lowerValue {
	sig, ok := l.pkg.TypesInfo.TypeOf(literal).Underlying().(*types.Signature)
	if !ok {
		l.errorAt(call, "immediate closure has no static signature")
		return lowerValue{}
	}
	frame := &lowerFrame{vars: map[types.Object]lowerValue{}, results: map[types.Object]bool{}, returnBlock: l.newBlock()}
	resultType := l.pkg.TypesInfo.TypeOf(call)
	if resultType != nil {
		if isContainerType(resultType) {
			frame.result = lowerValue{type_: resultType}
		} else {
			frame.result = l.allocZeroed("closure.result", resultType, call)
		}
	}
	l.frames = append(l.frames, frame)
	l.inlineCalls = append(l.inlineCalls, inlineCallSite{function: "immediate closure", pos: sourcePos(l.pkg, call.Pos())})
	defer func() {
		l.inlineCalls = l.inlineCalls[:len(l.inlineCalls)-1]
		l.frames = l.frames[:len(l.frames)-1]
	}()
	arg := 0
	for _, field := range literal.Type.Params.List {
		if len(field.Names) == 0 {
			arg++
			continue
		}
		for _, name := range field.Names {
			if arg >= len(args) {
				l.errorAt(call, "immediate closure argument count mismatch")
				break
			}
			if obj, ok := l.pkg.TypesInfo.Defs[name].(*types.Var); ok {
				l.bindParameter(obj, args[arg], name.Name, call)
			}
			arg++
		}
	}
	if literal.Type.Results != nil && frame.result.slots != nil {
		offset := 0
		for _, field := range literal.Type.Results.List {
			size := irTypeOf(l.pkg.TypesInfo.TypeOf(field.Type)).Slots
			if len(field.Names) == 0 {
				offset += size
				continue
			}
			for _, name := range field.Names {
				if obj := l.pkg.TypesInfo.Defs[name]; obj != nil {
					frame.results[obj] = true
					if isContainerType(obj.Type()) {
						frame.vars[obj] = lowerValue{type_: obj.Type()}
					} else {
						frame.vars[obj] = lowerValue{type_: obj.Type(), slots: frame.result.slots[offset : offset+size], places: frame.result.places[offset : offset+size]}
					}
				}
				offset += size
			}
		}
	}
	if arg != len(args) || sig.Params().Len() != len(args) {
		l.errorAt(call, "immediate closure argument count mismatch")
	}
	l.stmts(literal.Body.List)
	l.jump(frame.returnBlock)
	l.setCurrent(frame.returnBlock)
	return frame.result
}

func (l *lowerer) runtimeCall(n *ast.CallExpr, op resource.RuntimeFunction, pure bool, prefix []float64, args []lowerValue) lowerValue {
	flat := make([]ir.Expr, 0)
	for _, p := range prefix {
		flat = append(flat, ir.Const{Value: p})
	}
	for _, arg := range args {
		flat = append(flat, arg.slots...)
	}
	t := l.pkg.TypesInfo.TypeOf(n)
	resultType := irTypeOf(t)
	call := l.builder.RuntimeCall(op, flat, resultType, pure, sourcePos(l.pkg, n.Pos()))
	if t == nil || resultType.Slots == 0 {
		_ = l.builder.Eval(call)
		return lowerValue{}
	}
	out := zeroValue(t)
	if len(out.slots) != 1 {
		l.errorAt(n, "runtime call %s cannot directly return a multi-slot value", op)
		return out
	}
	if pure {
		out.slots[0] = call
		return out
	}
	result := l.alloc("runtime.result", t)
	if err := l.builder.Store(result.places, ir.Value{Type: resultType, Slots: []ir.Expr{call}}, sourcePos(l.pkg, n.Pos())); err != nil {
		l.errorAt(n, "%v", err)
	}
	return result
}

func (l *lowerer) inlineCall(n *ast.CallExpr, fn *types.Func, args []lowerValue) lowerValue {
	if l.callStack[fn] {
		l.errorAt(n, "recursive helper call to %s is not supported", fn.FullName())
		return zeroValue(l.pkg.TypesInfo.TypeOf(n))
	}
	pkg := l.packages[fn.Pkg()]
	if pkg == nil {
		l.errorAt(n, "function %s has no source body", fn.FullName())
		return zeroValue(l.pkg.TypesInfo.TypeOf(n))
	}
	decl := findFuncDecl(pkg, fn)
	if decl == nil || decl.Body == nil {
		l.errorAt(n, "function %s has no source body", fn.FullName())
		return zeroValue(l.pkg.TypesInfo.TypeOf(n))
	}
	resultType := l.pkg.TypesInfo.TypeOf(n)
	sig := fn.Type().(*types.Signature)
	if sig.Variadic() {
		l.errorAt(n, "variadic user helper %s is not supported by the frozen callback subset", fn.FullName())
		return zeroValue(resultType)
	}
	expectedArgs := sig.Params().Len()
	if sig.Recv() != nil {
		expectedArgs++
	}
	if len(args) != expectedArgs {
		l.errorAt(n, "helper %s received %d arguments; expected %d", fn.FullName(), len(args), expectedArgs)
		return zeroValue(resultType)
	}
	callSig, _ := l.pkg.TypesInfo.TypeOf(n.Fun).Underlying().(*types.Signature)
	if callSig == nil {
		callSig = sig
	}
	oldPkg := l.pkg
	l.pkg = pkg
	defer func() { l.pkg = oldPkg }()
	frame := &lowerFrame{vars: map[types.Object]lowerValue{}, results: map[types.Object]bool{}}
	if resultType != nil {
		if isContainerType(resultType) {
			frame.result = lowerValue{type_: resultType}
		} else {
			frame.result = l.allocZeroed(fn.Name()+".result", resultType, n)
		}
	}
	frame.returnBlock = l.newBlock()
	l.frames = append(l.frames, frame)
	l.callStack[fn] = true
	l.inlineCalls = append(l.inlineCalls, inlineCallSite{function: fn.FullName(), pos: sourcePos(oldPkg, n.Pos())})
	defer func() {
		l.inlineCalls = l.inlineCalls[:len(l.inlineCalls)-1]
		delete(l.callStack, fn)
		l.frames = l.frames[:len(l.frames)-1]
	}()
	arg := 0
	if sig.Recv() != nil {
		frame.vars[sig.Recv()] = args[arg]
		arg++
	}
	for i := 0; i < sig.Params().Len() && arg < len(args); i, arg = i+1, arg+1 {
		parameter := sig.Params().At(i)
		l.bindParameter(parameter, args[arg], parameter.Name(), n)
	}
	if decl.Type.Results != nil {
		offset, resultIndex := 0, 0
		for _, field := range decl.Type.Results.List {
			count := len(field.Names)
			if count == 0 {
				count = 1
			}
			for i := 0; i < count; i++ {
				actualType := callSig.Results().At(resultIndex).Type()
				size := irTypeOf(actualType).Slots
				if len(field.Names) != 0 {
					result := pkg.TypesInfo.Defs[field.Names[i]]
					frame.results[result] = true
					if isContainerType(actualType) {
						frame.vars[result] = lowerValue{type_: actualType}
					} else {
						frame.vars[result] = lowerValue{type_: actualType, slots: frame.result.slots[offset : offset+size], places: frame.result.places[offset : offset+size]}
					}
				}
				offset += size
				resultIndex++
			}
		}
	}
	l.stmts(decl.Body.List)
	l.jump(frame.returnBlock)
	l.setCurrent(frame.returnBlock)
	return frame.result
}

func (l *lowerer) stmt(stmt ast.Stmt) {
	if l.current() == nil || l.current().Terminator != nil {
		return
	}
	switch n := stmt.(type) {
	case *ast.BlockStmt:
		l.stmts(n.List)
	case *ast.ExprStmt:
		v := l.expr(n.X)
		for _, e := range v.slots {
			if c, ok := e.(ir.RuntimeCall); ok && !c.Pure {
				_ = l.builder.Eval(e)
			}
		}
	case *ast.DeclStmt:
		decl, ok := n.Decl.(*ast.GenDecl)
		if !ok || decl.Tok != token.VAR {
			l.errorAt(n, "only local var declarations are supported by the frozen callback subset")
			return
		}
		for _, spec := range decl.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				l.errorAt(spec, "unsupported local declaration %T", spec)
				continue
			}
			if len(vs.Values) != 0 {
				l.assign(&ast.AssignStmt{Lhs: identifiersAsExprs(vs.Names), Tok: token.DEFINE, Rhs: vs.Values})
				continue
			}
			for _, name := range vs.Names {
				obj := l.pkg.TypesInfo.Defs[name]
				if kind := containsResourceHandle(obj.Type()); kind != "" {
					l.errorAt(name, "%s resource handle aggregates cannot be declared without a resource value", kind)
					continue
				}
				l.bind(obj, l.allocZeroed(name.Name, obj.Type(), name))
			}
		}
	case *ast.AssignStmt:
		l.assign(n)
	case *ast.IncDecStmt:
		dst := l.expr(n.X)
		op := resource.RuntimeFunctionAdd
		if n.Tok == token.DEC {
			op = resource.RuntimeFunctionSubtract
		}
		src := lowerValue{type_: dst.type_, slots: []ir.Expr{ir.RuntimeCall{Function: op, Args: []ir.Expr{dst.slots[0], ir.Const{Value: 1}}, Result: irTypeOf(dst.type_), Pure: true, Pos: sourcePos(l.pkg, n.Pos())}}}
		l.store(dst, src, n)
	case *ast.IfStmt:
		l.ifStmt(n)
	case *ast.ForStmt:
		l.forStmt(n)
	case *ast.RangeStmt:
		l.rangeStmt(n)
	case *ast.SwitchStmt:
		l.switchStmt(n)
	case *ast.BranchStmt:
		if n.Tok == token.BREAK && len(l.breaks) > 0 {
			l.jump(l.breaks[len(l.breaks)-1])
		} else if n.Tok == token.CONTINUE && len(l.continues) > 0 {
			l.jump(l.continues[len(l.continues)-1])
		} else {
			l.errorAt(n, "unsupported branch %s", n.Tok)
		}
	case *ast.ReturnStmt:
		frame := l.frames[len(l.frames)-1]
		if isContainerType(frame.result.type_) {
			if len(n.Results) > 1 {
				l.errorAt(n, "container helper return requires exactly one result")
			} else if len(n.Results) == 1 {
				value := l.expr(n.Results[0])
				if value.container == nil {
					l.errorAt(n.Results[0], "container helper must return a catalog container with static backing storage")
				} else if frame.result.container != nil && !sameContainerBacking(frame.result, value) {
					l.errorAt(n.Results[0], "container helper cannot select between different backing stores at runtime")
				} else {
					frame.result = value
				}
			} else if frame.result.container == nil {
				l.errorAt(n, "named container result has no static backing storage")
			}
			l.jump(frame.returnBlock)
			return
		}
		if len(n.Results) > 0 {
			combined := lowerValue{type_: frame.result.type_}
			for _, result := range n.Results {
				value := l.materialize("return.value", l.expr(result), result)
				combined.slots = append(combined.slots, value.slots...)
			}
			l.store(frame.result, combined, n)
		}
		l.jump(frame.returnBlock)
	case *ast.EmptyStmt:
	default:
		l.errorAt(n, "unsupported callback statement %T", n)
	}
}

func (l *lowerer) switchStmt(n *ast.SwitchStmt) {
	if n.Init != nil {
		l.stmt(n.Init)
	}
	allConstant := n.Tag != nil
	for _, raw := range n.Body.List {
		for _, expression := range raw.(*ast.CaseClause).List {
			if l.pkg.TypesInfo.Types[expression].Value == nil {
				allConstant = false
			}
		}
	}
	if !allConstant {
		l.dynamicSwitchStmt(n)
		return
	}
	value := l.materialize("switch.tag", l.expr(n.Tag), n.Tag)
	exit := l.newBlock()
	term := ir.Switch{Value: value.slots[0], Default: exit.ID}
	caseBlocks := make([]*ir.Block, len(n.Body.List))
	for i, raw := range n.Body.List {
		clause := raw.(*ast.CaseClause)
		caseBlocks[i] = l.newBlock()
		if clause.List == nil {
			term.Default = caseBlocks[i].ID
			continue
		}
		for _, expression := range clause.List {
			caseValue := l.expr(expression)
			if len(caseValue.slots) != 1 {
				l.errorAt(expression, "switch case must be scalar")
				continue
			}
			constantValue, ok := caseValue.slots[0].(ir.Const)
			if !ok {
				l.errorAt(expression, "switch case must be constant")
				continue
			}
			term.Cases = append(term.Cases, ir.SwitchCase{Value: constantValue.Value, Target: caseBlocks[i].ID})
		}
	}
	defaultBlock := exit
	for _, block := range caseBlocks {
		if block.ID == term.Default {
			defaultBlock = block
			break
		}
	}
	_ = l.builder.Switch(term.Value, term.Cases, defaultBlock)
	l.breaks = append(l.breaks, exit)
	for i, raw := range n.Body.List {
		l.setCurrent(caseBlocks[i])
		l.dynamic(func() { l.stmts(raw.(*ast.CaseClause).Body) })
		l.jump(exit)
	}
	l.breaks = l.breaks[:len(l.breaks)-1]
	l.setCurrent(exit)
}

func (l *lowerer) dynamicSwitchStmt(n *ast.SwitchStmt) {
	exit := l.newBlock()
	caseBlocks := make([]*ir.Block, len(n.Body.List))
	var defaultBlock *ir.Block
	for i, raw := range n.Body.List {
		caseBlocks[i] = l.newBlock()
		if raw.(*ast.CaseClause).List == nil {
			defaultBlock = caseBlocks[i]
		}
	}
	if defaultBlock == nil {
		defaultBlock = exit
	}
	var tag lowerValue
	if n.Tag != nil {
		tag = l.materialize("switch.tag", l.expr(n.Tag), n.Tag)
	}
	for i, raw := range n.Body.List {
		clause := raw.(*ast.CaseClause)
		if clause.List == nil {
			continue
		}
		for _, expression := range clause.List {
			condition := l.expr(expression)
			if n.Tag != nil {
				if len(tag.slots) != 1 || len(condition.slots) != 1 {
					l.errorAt(expression, "switch comparison requires scalar values")
					continue
				}
				condition = lowerValue{type_: types.Typ[types.Bool], slots: []ir.Expr{ir.RuntimeCall{Function: resource.RuntimeFunctionEqual, Args: []ir.Expr{tag.slots[0], condition.slots[0]}, Result: irTypeOf(types.Typ[types.Bool]), Pure: true, Pos: sourcePos(l.pkg, expression.Pos())}}}
			}
			next := l.newBlock()
			_ = l.builder.Branch(condition.slots[0], caseBlocks[i], next)
			l.setCurrent(next)
		}
	}
	l.jump(defaultBlock)
	l.breaks = append(l.breaks, exit)
	for i, raw := range n.Body.List {
		l.setCurrent(caseBlocks[i])
		l.dynamic(func() { l.stmts(raw.(*ast.CaseClause).Body) })
		l.jump(exit)
	}
	l.breaks = l.breaks[:len(l.breaks)-1]
	l.setCurrent(exit)
}

func (l *lowerer) stmts(stmts []ast.Stmt) {
	for _, stmt := range stmts {
		l.stmt(stmt)
	}
}

func (l *lowerer) assign(n *ast.AssignStmt) {
	compound := map[token.Token]resource.RuntimeFunction{
		token.ADD_ASSIGN: resource.RuntimeFunctionAdd, token.SUB_ASSIGN: resource.RuntimeFunctionSubtract,
		token.MUL_ASSIGN: resource.RuntimeFunctionMultiply, token.QUO_ASSIGN: resource.RuntimeFunctionDivide,
		token.REM_ASSIGN: resource.RuntimeFunctionMod,
	}
	if op, ok := compound[n.Tok]; ok {
		if len(n.Lhs) != 1 || len(n.Rhs) != 1 {
			l.errorAt(n, "compound assignment requires one value")
			return
		}
		dst, rhs := l.expr(n.Lhs[0]), l.expr(n.Rhs[0])
		if len(dst.slots) != 1 || len(rhs.slots) != 1 {
			l.errorAt(n, "compound assignment requires scalar values")
			return
		}
		value := lowerValue{type_: dst.type_, slots: []ir.Expr{ir.RuntimeCall{Function: op, Args: []ir.Expr{dst.slots[0], rhs.slots[0]}, Result: irTypeOf(dst.type_), Pure: true, Pos: sourcePos(l.pkg, n.Pos())}}}
		l.store(dst, value, n)
		return
	}
	destinations := make([]lowerValue, len(n.Lhs))
	skipStore := make([]bool, len(n.Lhs))
	newObjects := make([]types.Object, len(n.Lhs))
	for i, lhs := range n.Lhs {
		if id, ok := lhs.(*ast.Ident); ok && id.Name == "_" {
			skipStore[i] = true
			continue
		}
		if id, ok := lhs.(*ast.Ident); ok && n.Tok == token.DEFINE {
			obj := l.pkg.TypesInfo.Defs[id]
			if obj == nil {
				destinations[i] = l.expr(id)
				continue
			}
			newObjects[i] = obj
		} else if _, ok := lhs.(*ast.Ident); !ok {
			destinations[i] = l.expr(lhs)
		} else {
			// Identifier destinations have no evaluation side effects. Resolve
			// them after the RHS so container assignments can rebind backing.
		}
	}
	values := make([]lowerValue, len(n.Rhs))
	for i, rhs := range n.Rhs {
		values[i] = l.expr(rhs)
		if len(n.Lhs) > 1 {
			values[i] = l.materialize("assignment.rhs", values[i], rhs)
		}
	}
	if len(values) == 1 && len(n.Lhs) > 1 {
		combined := values[0]
		values = make([]lowerValue, len(n.Lhs))
		offset := 0
		for i, lhs := range n.Lhs {
			typ := l.pkg.TypesInfo.TypeOf(lhs)
			size := irTypeOf(typ).Slots
			if offset+size > len(combined.slots) {
				l.errorAt(n, "tuple assignment layout mismatch")
				return
			}
			values[i] = lowerValue{type_: typ, slots: combined.slots[offset : offset+size]}
			offset += size
		}
		if offset != len(combined.slots) {
			l.errorAt(n, "tuple assignment layout mismatch")
			return
		}
	}
	if len(values) != len(n.Lhs) {
		l.errorAt(n, "tuple assignment is not supported")
		return
	}
	for i, object := range newObjects {
		if object == nil {
			continue
		}
		if values[i].container != nil {
			l.bind(object, values[i])
			skipStore[i] = true
			continue
		}
		identifier := n.Lhs[i].(*ast.Ident)
		destination := l.alloc(identifier.Name, object.Type())
		l.bind(object, destination)
		destinations[i] = destination
	}
	for i, lhs := range n.Lhs {
		if skipStore[i] {
			continue
		}
		if values[i].container != nil {
			identifier, ok := lhs.(*ast.Ident)
			if !ok {
				l.errorAt(lhs, "container values can only be assigned to local variables")
				continue
			}
			object := l.pkg.TypesInfo.ObjectOf(identifier)
			if object == nil {
				object = l.pkg.TypesInfo.Defs[identifier]
			}
			if object == nil {
				l.errorAt(lhs, "container assignment target has no object identity")
				continue
			}
			if l.dynamicDepth > 0 {
				existing, exists := l.lookup(object)
				if !exists || existing.container == nil || !sameContainerBacking(existing, values[i]) {
					l.errorAt(lhs, "container assignment cannot select a different backing store in runtime control flow")
					continue
				}
			}
			l.rebind(object, values[i])
			continue
		}
		if len(destinations[i].places) == 0 {
			destinations[i] = l.expr(lhs)
		}
		l.store(destinations[i], values[i], lhs)
	}
}

func identifiersAsExprs(ids []*ast.Ident) []ast.Expr {
	result := make([]ast.Expr, len(ids))
	for i, id := range ids {
		result[i] = id
	}
	return result
}

func (l *lowerer) ifStmt(n *ast.IfStmt) {
	if n.Init != nil {
		l.stmt(n.Init)
	}
	cond := l.expr(n.Cond)
	thenBlock, elseBlock, merge := l.newBlock(), l.newBlock(), l.newBlock()
	_ = l.builder.Branch(cond.slots[0], thenBlock, elseBlock)
	l.setCurrent(thenBlock)
	l.dynamic(func() { l.stmts(n.Body.List) })
	l.jump(merge)
	l.setCurrent(elseBlock)
	if n.Else != nil {
		l.dynamic(func() { l.stmt(n.Else) })
	}
	l.jump(merge)
	l.setCurrent(merge)
}

func (l *lowerer) forStmt(n *ast.ForStmt) {
	if n.Init != nil {
		l.stmt(n.Init)
	}
	header, body, post, exit := l.newBlock(), l.newBlock(), l.newBlock(), l.newBlock()
	l.jump(header)
	l.setCurrent(header)
	if n.Cond == nil {
		l.jump(body)
	} else {
		c := l.expr(n.Cond)
		_ = l.builder.Branch(c.slots[0], body, exit)
	}
	l.breaks = append(l.breaks, exit)
	l.continues = append(l.continues, post)
	l.setCurrent(body)
	l.dynamic(func() { l.stmts(n.Body.List) })
	l.jump(post)
	l.setCurrent(post)
	if n.Post != nil {
		l.dynamic(func() { l.stmt(n.Post) })
	}
	l.jump(header)
	l.breaks = l.breaks[:len(l.breaks)-1]
	l.continues = l.continues[:len(l.continues)-1]
	l.setCurrent(exit)
}

func (l *lowerer) rangeStmt(n *ast.RangeStmt) {
	collection := l.expr(n.X)
	array, ok := types.Unalias(collection.type_).Underlying().(*types.Array)
	if !ok && collection.container == nil {
		l.errorAt(n.X, "range is only supported for fixed arrays and catalog containers")
		return
	}
	var length ir.Expr
	var elementType types.Type
	if ok {
		collection = l.materializeAddressable("range.array", collection, n.X)
		length = ir.Const{Value: float64(array.Len())}
		elementType = array.Elem()
	} else {
		lengthValue := l.materialize("range.length", lowerValue{type_: types.Typ[types.Int], slots: []ir.Expr{collection.slots[0]}}, n.X)
		length = lengthValue.slots[0]
		slice, sliceOK := types.Unalias(collection.type_).Underlying().(*types.Slice)
		if !sliceOK {
			l.errorAt(n.X, "catalog container has no slice element layout")
			return
		}
		elementType = slice.Elem()
	}
	index := l.alloc("range.index", types.Typ[types.Int])
	l.store(index, lowerValue{type_: types.Typ[types.Int], slots: []ir.Expr{ir.Const{}}}, n)
	header, body, post, exit := l.newBlock(), l.newBlock(), l.newBlock(), l.newBlock()
	l.jump(header)
	l.setCurrent(header)
	condition := ir.RuntimeCall{Function: resource.RuntimeFunctionLess, Args: []ir.Expr{index.slots[0], length}, Result: irTypeOf(types.Typ[types.Bool]), Pure: true, Pos: sourcePos(l.pkg, n.Pos())}
	_ = l.builder.Branch(condition, body, exit)
	l.setCurrent(body)
	if !blankExpr(n.Key) {
		l.assignRangeValue(n.Key, n.Tok, index)
	}
	if !blankExpr(n.Value) {
		size := irTypeOf(elementType).Slots
		var element lowerValue
		if collection.container != nil {
			element = l.containerElement(n, collection.container, index.slots[0], 0, size, elementType)
		} else {
			element = lowerValue{type_: elementType, slots: make([]ir.Expr, size), places: make([]ir.Place, size)}
			if len(collection.places) == 0 {
				l.errorAt(n.X, "ranged array must be addressable")
				return
			}
			switch first := collection.places[0].(type) {
			case ir.LocalPlace:
				for offset := 0; offset < size; offset++ {
					place := l.indexedLocal(first, index.slots[0], int(array.Len()), size, offset, n)
					element.places[offset], element.slots[offset] = place, ir.Load{Place: place}
				}
			case ir.MemoryPlace:
				baseIndex, staticBase := first.Index.(ir.Const)
				if !staticBase || baseIndex.Value != 0 || first.Stride != 0 {
					l.errorAt(n.X, "range cannot combine an array index with a dynamic memory record index")
					return
				}
				for offset := 0; offset < size; offset++ {
					place := l.memory(first.Storage, index.slots[0], size, first.Offset+offset, first.Read, first.Write, n)
					element.places[offset], element.slots[offset] = place, ir.Load{Place: place}
				}
			default:
				l.errorAt(n.X, "range does not support array place %T", collection.places[0])
				return
			}
		}
		l.assignRangeValue(n.Value, n.Tok, element)
	}
	l.breaks = append(l.breaks, exit)
	l.continues = append(l.continues, post)
	l.dynamic(func() { l.stmts(n.Body.List) })
	l.jump(post)
	l.setCurrent(post)
	next := lowerValue{type_: types.Typ[types.Int], slots: []ir.Expr{ir.RuntimeCall{Function: resource.RuntimeFunctionAdd, Args: []ir.Expr{index.slots[0], ir.Const{Value: 1}}, Result: irTypeOf(types.Typ[types.Int]), Pure: true, Pos: sourcePos(l.pkg, n.Pos())}}}
	l.store(index, next, n)
	l.jump(header)
	l.breaks = l.breaks[:len(l.breaks)-1]
	l.continues = l.continues[:len(l.continues)-1]
	l.setCurrent(exit)
}

func blankExpr(expr ast.Expr) bool {
	identifier, ok := expr.(*ast.Ident)
	return expr == nil || (ok && identifier.Name == "_")
}

func (l *lowerer) assignRangeValue(target ast.Expr, op token.Token, value lowerValue) {
	if op != token.DEFINE {
		l.store(l.expr(target), value, target)
		return
	}
	identifier, ok := target.(*ast.Ident)
	if !ok {
		l.errorAt(target, "range declaration target must be an identifier")
		return
	}
	object := l.pkg.TypesInfo.Defs[identifier]
	if object == nil {
		l.errorAt(target, "range declaration has no type object")
		return
	}
	local := l.alloc(identifier.Name, object.Type())
	l.bind(object, local)
	l.store(local, value, target)
}

func lowerCallback(packagesByTypes map[*types.Package]*packages.Package, pkg *packages.Package, decl *ast.FuncDecl, fn *types.Func, fields []*FieldDeclaration, resources *ModeResources, archetypes map[*types.Named]archetypeBinding, m mode.Mode, phase string) (*ir.Function, []error) {
	if decl == nil {
		return nil, nil
	}
	resultType := ir.Type{Name: "void"}
	sig := fn.Type().(*types.Signature)
	if sig.Results().Len() == 1 {
		resultType = irTypeOf(sig.Results().At(0).Type())
	}
	builder := ir.NewBuilder(fn.FullName(), resultType)
	resourceIDs := map[*types.Var][]int{}
	if resources != nil {
		resourceIDs = resources.FieldIDs
	}
	archetypeFields := map[*types.Var]*FieldDeclaration{}
	for _, field := range fields {
		archetypeFields[field.Object] = field
	}
	l := &lowerer{mode: m, phase: phase, packages: packagesByTypes, pkg: pkg, builder: builder, callStack: map[*types.Func]bool{fn: true}, resourceIDs: resourceIDs, archetypeFields: archetypeFields, archetypes: archetypes}
	entry := l.newBlock()
	_ = builder.SetEntry(entry)
	l.setCurrent(entry)
	frame := &lowerFrame{vars: map[types.Object]lowerValue{}, returnBlock: l.newBlock()}
	l.frames = append(l.frames, frame)
	if sig.Recv() != nil {
		recv := l.alloc("receiver", sig.Recv().Type())
		if len(fields) != 0 {
			recv.slots = recv.slots[:0]
			recv.places = recv.places[:0]
			for _, field := range fields {
				storage, read, write := archetypeStorageAccess(field.Storage, m, phase)
				for offset := 0; offset < field.Size; offset++ {
					place, err := builder.Memory(storage, ir.Const{}, 0, field.Offset+offset, read, write)
					if err != nil {
						l.errorAt(decl, "%v", err)
						continue
					}
					recv.places = append(recv.places, place)
					recv.slots = append(recv.slots, ir.Load{Place: place})
				}
			}
		}
		frame.vars[sig.Recv()] = recv
	}
	if sig.Results().Len() == 1 {
		frame.result = l.allocZeroed("result", sig.Results().At(0).Type(), decl)
	}
	l.stmts(decl.Body.List)
	l.jump(frame.returnBlock)
	l.setCurrent(frame.returnBlock)
	if frame.result.slots != nil {
		_ = builder.Return(ir.Value{Type: resultType, Slots: frame.result.slots})
	} else {
		_ = builder.Return(ir.Value{Type: resultType})
	}
	sort.SliceStable(l.errs, func(i, j int) bool { return l.errs[i].Error() < l.errs[j].Error() })
	if len(l.errs) > 0 {
		return nil, l.errs
	}
	result, err := builder.Finish()
	if err != nil {
		return nil, []error{fmt.Errorf("callback %s: %w", fn.FullName(), err)}
	}
	if runtimeErrs := validateRuntimeCalls(result); len(runtimeErrs) != 0 {
		for i := range runtimeErrs {
			runtimeErrs[i] = fmt.Errorf("callback %s: %w", fn.FullName(), runtimeErrs[i])
		}
		return nil, runtimeErrs
	}
	return result, nil
}

func archetypeStorageAccess(storage string, m mode.Mode, phase string) (string, bool, bool) {
	switch storage {
	case "imported", "data":
		return "data", true, phase == "preprocess"
	case "exported":
		return "exported", false, true
	case "shared":
		write := phase == "preprocess" || phase == "updateSequential"
		if m == mode.ModePlay {
			write = write || phase == "touch"
		}
		return "shared", true, write
	default:
		return storage, true, true
	}
}
