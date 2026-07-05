package frontend

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strconv"
	"strings"

	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/ir"
)

func (t *tracer) expr(e ast.Expr) (Num, error) {
	switch n := e.(type) {
	case *ast.BasicLit:
		return t.literal(n)
	case *ast.Ident:
		return t.ident(n)
	case *ast.ParenExpr:
		return t.expr(n.X)
	case *ast.UnaryExpr:
		x, err := t.expr(n.X)
		if err != nil {
			return Num{}, err
		}
		res, ok := applyUnary(t.gen, n.Op, x)
		if !ok {
			return Num{}, t.errf(n, "unsupported unary operator %s (only +, -, ! are supported; no ^, &, or <-)", n.Op)
		}
		return res, nil
	case *ast.BinaryExpr:
		x, err := t.expr(n.X)
		if err != nil {
			return Num{}, err
		}
		y, err := t.expr(n.Y)
		if err != nil {
			return Num{}, err
		}
		res, ok := applyBinary(t.gen, n.Op, x, y)
		if !ok {
			return Num{}, t.errf(n, "binary operator %s is not supported (only arithmetic +, -, *, /, %%, comparison, and logical &&, || operators are supported; no bitwise &, |, ^, <<, >>)", n.Op)
		}
		return res, nil
	case *ast.IndexExpr:
		place, err := t.indexPlace(n)
		if err != nil {
			return Num{}, err
		}
		return exprNum(ir.GetPlace(place)), nil
	case *ast.SelectorExpr:
		if b, isRecv, err := t.receiverBinding(n); err != nil {
			return Num{}, err
		} else if isRecv {
			return exprNum(ir.GetPlace(ir.Cell(b.Block, b.Index))), nil
		}
		// Record-typed struct field: n.pos.X → look up "pos.x" in bindings.
		if inner, ok := n.X.(*ast.SelectorExpr); ok {
			if base, ok2 := inner.X.(*ast.Ident); ok2 && t.env.Receiver != "" && base.Name == t.env.Receiver {
				fullName := inner.Sel.Name + "." + strings.ToLower(n.Sel.Name)
				if b, ok3 := t.env.Names[fullName]; ok3 {
					return exprNum(ir.GetPlace(ir.Cell(b.Block, b.Index))), nil
				}
			}
		}
		// sonolus-prefixed global variable: sonolus.Time → lookup "time" in bindings.
		if pkg, ok := n.X.(*ast.Ident); ok && pkg.Name == "sonolus" {
			bareName := lowerFirst(n.Sel.Name)
			if b, ok2 := t.env.Names[bareName]; ok2 {
				return exprNum(ir.GetPlace(ir.Cell(b.Block, b.Index))), nil
			}
		}
		// Bare composite: evaluate vec2(...).x → extract field from the composite.
		if _, isCall := n.X.(*ast.CallExpr); isCall {
			v, err := t.expr(n.X)
			if err != nil {
				return Num{}, err
			}
			if v.IsComposite() {
				if f, ok := v.TryField(n.Sel.Name); ok {
					return f, nil
				}
				// Try lowercase: Go field names are exported (uppercase)
				// but record composite fields are stored lowercase (t, r, b, l).
				if f, ok := v.TryField(strings.ToLower(n.Sel.Name)); ok {
					return f, nil
				}
				return Num{}, t.errf(n, "record has no field %q", n.Sel.Name)
			}
		}
		return t.fieldValue(n)
	case *ast.CompositeLit:
		return t.compositeLit(n)
	case *ast.CallExpr:
		return t.call(n)
	case *ast.FuncLit:
		// Collect captured outer variables via free-variable analysis.
		var captures []string
		seen := make(map[string]bool)
		ast.Inspect(n.Body, func(node ast.Node) bool {
			if id, ok := node.(*ast.Ident); ok {
				if seen[id.Name] {
					return true
				}
				if _, inVars := t.vars[id.Name]; inVars {
					captures = append(captures, id.Name)
					seen[id.Name] = true
				}
				if _, inNames := t.env.Names[id.Name]; inNames {
					captures = append(captures, id.Name)
					seen[id.Name] = true
				}
				if _, inRecs := t.records[id.Name]; inRecs {
					captures = append(captures, id.Name)
					seen[id.Name] = true
				}
			}
			return true
		})

		if len(captures) == 0 {
			// Non-capturing: register as a synthetic helper.
			synthName := fmt.Sprintf("$closure_%d", t.gen.Next())
			decl := &ast.FuncDecl{
				Name: &ast.Ident{Name: synthName},
				Type: n.Type,
				Body: n.Body,
			}
			t.env.Funcs[synthName] = decl
			return constNum(0), nil
		}

		// Capturing closure: allocate a capture frame and store captured values.
		synthName := fmt.Sprintf("$closure_%d", t.gen.Next())
		frame := &ir.TempBlock{Name: synthName + "_cap", Size: len(captures)}
		// Initialize each capture slot from the outer variable's current value.
		for i, capName := range captures {
			if tb, ok := t.vars[capName]; ok {
				val := exprNum(ir.GetPlace(t.cell(tb)))
				t.emit(t.gen.SetPlace(ir.BlockPlace{Block: frame, Index: ir.Const(i), Offset: 0}, val.mustNode()))
			} else if b, ok := t.env.Names[capName]; ok {
				val := exprNum(ir.GetPlace(ir.Cell(b.Block, b.Index)))
				t.emit(t.gen.SetPlace(ir.BlockPlace{Block: frame, Index: ir.Const(i), Offset: 0}, val.mustNode()))
			} else if rec, ok := t.records[capName]; ok {
				// For record captures, capture the full record by reading
				// its composite value and storing field-by-field.
				offset := i
				for j := range rec.order {
					val := exprNum(ir.GetPlace(ir.BlockPlace{Block: rec.tb, Index: ir.Const(j), Offset: 0}))
					t.emit(t.gen.SetPlace(ir.BlockPlace{Block: frame, Index: ir.Const(offset + j), Offset: 0}, val.mustNode()))
				}
			}
		}
		if t.closures == nil {
			t.closures = make(map[string]*closureInfo)
		}
		t.closures[synthName] = &closureInfo{
			fn:       n,
			captures: captures,
			frame:    frame,
		}
		// Store the closure name as a temp so it can be referenced for calls.
		tb := t.alloc(synthName)
		return exprNum(ir.GetPlace(t.cell(tb))), nil
	default:
		return Num{}, t.errf(e, "unsupported expression %T (closures, func literals, and channel receives are not supported because the engine has no heap and no concurrency)", e)
	}
}

func (t *tracer) literal(n *ast.BasicLit) (Num, error) {
	switch n.Kind {
	case token.INT:
		v, err := strconv.ParseInt(n.Value, 0, 64)
		if err != nil {
			return Num{}, t.errf(n, "invalid integer %q", n.Value)
		}
		return constNum(float64(v)), nil
	case token.FLOAT:
		v, err := strconv.ParseFloat(n.Value, 64)
		if err != nil {
			return Num{}, t.errf(n, "invalid float %q", n.Value)
		}
		return constNum(v), nil
	default:
		return Num{}, t.errf(n, "unsupported literal %s (only int and float literals are supported; all values are float64 at runtime)", n.Kind)
	}
}

func (t *tracer) ident(n *ast.Ident) (Num, error) {
	switch n.Name {
	case "true":
		return constNum(1), nil
	case "false":
		return constNum(0), nil
	case "_":
		return Num{}, t.errf(n, "cannot use _ as value (blank identifier is write-only)")
	}
	// D2: use go/types Info for type-driven local-variable resolution.
	if info := t.env.Info; info != nil {
		if obj, ok := info.Uses[n]; ok {
			if _, isVar := obj.(*types.Var); isVar {
				if tb, ok2 := t.vars[n.Name]; ok2 {
					return exprNum(ir.GetPlace(t.cell(tb))), nil
				}
				// Types.Var resolved but tracer doesn't have a temp block yet
				// — fall through to environment bindings.
			}
		}
	}
	if tb, ok := t.vars[n.Name]; ok {
		return exprNum(ir.GetPlace(t.cell(tb))), nil
	}
	// Record locals (vec2, pair, etc.) are stored in records, not vars.
	if rec, ok := t.records[n.Name]; ok {
		return rec.val, nil
	}
	// Environment bindings: archetype fields, runtime accessors (a user variable
	// of the same fnName shadows these).
	if b, ok := t.env.Names[n.Name]; ok {
		return exprNum(ir.GetPlace(ir.Cell(b.Block, b.Index))), nil
	}
	// Named compile-time constants (archetype indices, standard values).
	if v, ok := t.env.Constants[n.Name]; ok {
		return constNum(v), nil
	}
	return Num{}, t.errf(n, "undefined identifier %q", n.Name)
}

// indexPlace resolves arr[i] for both local arrays (bare ident) and
// struct-field container arrays (selector expression).
func (t *tracer) indexPlace(n *ast.IndexExpr) (ir.BlockPlace, error) {
	switch target := n.X.(type) {
	case *ast.Ident:
		return t.arrayElemPlace(target, n.Index)
	case *ast.SelectorExpr:
		// struct field: n.Arr[i] → resolve field binding and container.
		if base, ok := target.X.(*ast.Ident); ok && base.Name == t.env.Receiver {
			fieldName := target.Sel.Name
			if ci, ok2 := t.containers[fieldName]; ok2 {
				index, err := t.expr(n.Index)
				if err != nil {
					return ir.BlockPlace{}, err
				}
				return ci.elemPlace(t.gen, index.mustNode()), nil
			}
		}
		return ir.BlockPlace{}, t.errf(n, "array index target must be a local array or container struct field")
	default:
		return ir.BlockPlace{}, t.errf(n, "array index target must be an identifier or struct field")
	}
}

// call handles the memory builtins get(block, index) and set(block, index, value).
func (t *tracer) call(n *ast.CallExpr) (Num, error) {
	// Package-qualified call: sonolus.Draw(...) → runtime function.
	if sel, ok := n.Fun.(*ast.SelectorExpr); ok {
		if pkgName := t.resolvePkgName(sel.X); pkgName == "sonolus" {
			return t.sonolusCall(n, sel)
		}
		return t.methodCall(n, sel)
	}
	fn, ok := n.Fun.(*ast.Ident)
	if !ok {
		return Num{}, t.errf(n, "unsupported call target (function calls must use a plain identifier or selector expression, e.g. foo() or obj.Method())")
	}

	// Type-driven dispatch from go/types info.
	if r, handled, err := t.resolveTypeCall(fn, n); handled {
		return r, err
	}

	// Name-based dispatch that does not require pre-evaluated arguments.
	if r, handled, err := t.resolveBuiltinCall(fn, n); handled {
		return r, err
	}

	// Calling a local variable as a function — check if it's a captured closure.
	if _, ok := t.vars[fn.Name]; ok {
		if ci, ok2 := t.closures[fn.Name]; ok2 {
			return t.callClosure(fn, n, ci)
		}
		return Num{}, t.errf(n, "unsupported call through variable %q (closures and function values cannot be stored in variables; call functions directly instead)", fn.Name)
	}
	if _, ok := t.records[fn.Name]; ok {
		return Num{}, t.errf(n, "unsupported call through variable %q (closures and function values cannot be stored in variables; call functions directly instead)", fn.Name)
	}

	// Functions that need pre-evaluated numeric arguments.
	args := make([]Num, len(n.Args))
	for i, a := range n.Args {
		v, err := t.expr(a)
		if err != nil {
			return Num{}, err
		}
		args[i] = v
	}
	return t.callWithArgs(fn, n, args)
}

// resolveTypeCall uses go/types Info to dispatch record constructors and static
// compositeLit handles Go composite literal expressions like Vec2{x: 1, y: 2}
// or Rect{0, 0, 100, 50}. It resolves the type name to a known record type and
// constructs the record from the key-value or positional elements.
func (t *tracer) compositeLit(n *ast.CompositeLit) (Num, error) {
	typeName, ok := n.Type.(*ast.Ident)
	if !ok {
		return Num{}, t.errf(n, "composite literal type must be an identifier (use TypeName{...} directly, not a qualified or parameterized type)")
	}
	fields, known := knownRecordFields(typeName.Name, t.env.Records)
	if !known {
		return Num{}, t.errf(n, "unknown record type %q in composite literal (composite literals are only supported for known record types: Vec2, Quad, Mat, Rect, Trans, Pair)", typeName.Name)
	}

	// Build field → value map from the composite literal elements.
	vals := make(map[string]Num, len(fields))
	for i, elt := range n.Elts {
		switch kv := elt.(type) {
		case *ast.KeyValueExpr:
			key, ok := kv.Key.(*ast.Ident)
			if !ok {
				return Num{}, t.errf(kv, "composite literal key must be an identifier (use FieldName: value, not a string or expression key)")
			}
			v, err := t.expr(kv.Value)
			if err != nil {
				return Num{}, err
			}
			vals[key.Name] = v
		default:
			// Positional: map to field order.
			if i >= len(fields) {
				return Num{}, t.errf(n, "too many positional elements for %q (has %d fields)", typeName.Name, len(fields))
			}
			v, err := t.expr(elt)
			if err != nil {
				return Num{}, err
			}
			vals[fields[i]] = v
		}
	}

	return compNumFromFields(typeName.Name, fields, vals), nil
}

// compNumFromFields builds a composite Num for a known record type. Missing
// fields default to const 0.
func compNumFromFields(typeName string, fields []string, vals map[string]Num) Num {
	fieldMap := make(map[string]Num, len(fields))
	for _, f := range fields {
		if v, ok := vals[f]; ok {
			fieldMap[f] = v
		} else {
			fieldMap[f] = constNum(0)
		}
	}
	return compNum(fieldMap)
}

// constructors via the type system. Returns handled=true when the call was
// resolved (either a value or an error); handled=false means keep going.
func (t *tracer) resolveTypeCall(fn *ast.Ident, n *ast.CallExpr) (Num, bool, error) {
	if t.env.Info == nil {
		return Num{}, false, nil
	}
	obj, ok := t.env.Info.Uses[fn]
	if !ok {
		return Num{}, false, nil
	}
	fobj, ok := obj.(*types.Func)
	if !ok {
		return Num{}, false, nil
	}
	sig, ok := fobj.Type().(*types.Signature)
	if !ok || sig.Results().Len() == 0 {
		return Num{}, false, nil
	}

	rt := sig.Results().At(0).Type()
	if rt == nil {
		return Num{}, false, nil
	}

	// Struct-returning constructors (user records, vec2, quad, etc.).
	if named, ok := rt.(*types.Named); ok {
		if st, ok := named.Underlying().(*types.Struct); ok {
			fields := make([]string, st.NumFields())
			for i := range st.NumFields() {
				fields[i] = st.Field(i).Name()
			}
			r, err := t.inlineComposite(fn, n, fields)
			return r, true, err
		}
		// Static constructors: vec2Zero, vec2One, etc.
		if methods, ok := recordStatics[named.Obj().Name()]; ok {
			if f, ok := methods[fn.Name]; ok {
				r, err := f()
				return r, true, err
			}
		}
	}
	return Num{}, false, nil
}

// resolveBuiltinCall dispatches calls that don't need pre-evaluated arguments
// (arrays, mode checks, composite constructors, etc.).
func (t *tracer) resolveBuiltinCall(fn *ast.Ident, n *ast.CallExpr) (Num, bool, error) {
	switch fn.Name {
	case "sprite":
		if len(n.Args) == 1 {
			if lit, ok := n.Args[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
				name, err := strconv.Unquote(lit.Value)
				if err == nil {
					if id, ok2 := t.env.SpriteIndex[name]; ok2 {
						return constNum(id), true, nil
					}
					return Num{}, true, t.errf(n, "unknown sprite name %q", name)
				}
			}
		}
		return Num{}, true, t.errf(n, "sprite expects a string literal")
	case "len":
		if len(n.Args) == 1 {
			if id, ok := n.Args[0].(*ast.Ident); ok {
				if arr, ok := t.arrays[id.Name]; ok {
					return constNum(float64(arr.count)), true, nil
				}
			}
		}
		return Num{}, true, t.errf(n, "len expects an array")
	case "array":
		return Num{}, true, t.errf(n, "array() may only appear in a declaration (a := array(n))")
	case "screen":
		r, err := t.screenFunc(n)
		return r, true, err
	case "safeArea":
		r, err := t.safeAreaFunc(n)
		return r, true, err
	case "offsetAdjustedTime":
		r, err := t.offsetAdjustedTimeFunc(n)
		return r, true, err
	case "prevTime":
		r, err := t.prevTimeFunc(n)
		return r, true, err
	case "isPlay":
		return boolNum(t.env.Mode == ir.ModePlay), true, nil
	case "isWatch":
		return boolNum(t.env.Mode == ir.ModeWatch), true, nil
	case "isPreview":
		return boolNum(t.env.Mode == ir.ModePreview), true, nil
	case "isTutorial":
		return boolNum(t.env.Mode == ir.ModeTutorial), true, nil
	case "varArray", "arrayMap":
		// varArray(capacity) / arrayMap(capacity) — capacity must be constant.
		if len(n.Args) != 1 {
			return Num{}, true, t.errf(n, "%s expects exactly 1 argument (capacity)", fn.Name)
		}
		capVal, err := t.expr(n.Args[0])
		if err != nil {
			return Num{}, true, err
		}
		if !capVal.isConst {
			return Num{}, true, t.errf(n, "%s capacity must be a compile-time constant", fn.Name)
		}
		return compNum(map[string]Num{
			"_size":  constNum(0),
			"_array": constNum(float64(capVal.c)),
		}), true, nil
	case "frozenNumSet":
		return compNum(map[string]Num{"_size": constNum(0), "_array": constNum(0)}), true, nil
	case "arraySet":
		// arraySet(capacity) wraps a VarArray internally.
		if len(n.Args) != 1 {
			return Num{}, true, t.errf(n, "arraySet expects exactly 1 argument (capacity)")
		}
		capVal, err := t.expr(n.Args[0])
		if err != nil {
			return Num{}, true, err
		}
		if !capVal.isConst {
			return Num{}, true, t.errf(n, "arraySet capacity must be a compile-time constant")
		}
		return compNum(map[string]Num{
			"_values": constNum(0), // placeholder — wraps VarArray internally
		}), true, nil
	case "debugTerminate":
		t.terminated = true
		return constNum(0), true, nil
	case "entityInfo":
		if len(n.Args) == 1 {
			indexVal, err := t.expr(n.Args[0])
			if err != nil {
				return Num{}, true, err
			}
			return exprNum(ir.GetPlace(ir.NewBlockPlace(ir.Const(4103), indexVal.mustNode(), 0))), true, nil
		}
		return Num{}, true, t.errf(n, "entityInfo expects 1 argument (index)")
	case "skinTransform":
		return t.builtinGetBlock(n, 1003)
	case "setSkinTransform":
		return t.builtinSetBlock(n, 1003)
	case "particleTransform":
		return t.builtinGetBlock(n, 1004)
	case "setParticleTransform":
		return t.builtinSetBlock(n, 1004)
	case "background":
		return t.builtinGetBlock(n, 1005)
	case "setBackground":
		return t.builtinSetBlock(n, 1005)
	case "levelScore":
		return t.builtinGetBlock(n, 2004)
	case "setLevelScore":
		return t.builtinSetBlock(n, 2004)
	case "levelLife":
		return t.builtinGetBlock(n, 2005)
	case "setLevelLife":
		return t.builtinSetBlock(n, 2005)
	default:
		if strings.HasPrefix(fn.Name, "vec2") {
			key := strings.ToLower(fn.Name[4:])
			if f, ok := vec2Statics[key]; ok {
				return f(), true, nil
			}
		}
		if fields, ok := builtinRecordFields(fn.Name); ok {
			r, err := t.inlineComposite(fn, n, fields)
			return r, true, err
		}
	}
	return Num{}, false, nil
}

// callWithArgs dispatches calls that require pre-evaluated numeric arguments.
func (t *tracer) callWithArgs(fn *ast.Ident, n *ast.CallExpr, args []Num) (Num, error) {
	switch fn.Name {
	case "get":
		if len(args) != 2 {
			return Num{}, t.errf(n, "get expects (block, index)")
		}
		place := ir.NewBlockPlace(args[0].mustNode(), args[1].mustNode(), 0)
		return exprNum(ir.GetPlace(place)), nil
	case "touchId", "touchID": // touchID: lowerFirst("TouchID") produces "touchID"
		return t.touchField(n, args, 0)
	case "touchStarted":
		return t.touchField(n, args, 1)
	case "touchEnded":
		return t.touchField(n, args, 2)
	case "touchX":
		return t.touchField(n, args, 3)
	case "touchY":
		return t.touchField(n, args, 4)
	case "pnpoly":
		return t.pnpolyFunc(n, args)
	case "perspectiveApproach":
		return t.perspectiveApproachFunc(n, args)
	case "set":
		if len(args) != 3 {
			return Num{}, t.errf(n, "set expects (block, index, value)")
		}
		place := ir.NewBlockPlace(args[0].mustNode(), args[1].mustNode(), 0)
		t.emit(t.gen.SetPlace(place, args[2].mustNode()))
		return constNum(0), nil
	case "sortLinkedEntities":
		if len(args) < 3 {
			return Num{}, t.errf(n, "sortLinkedEntities expects (head, sortKeyOffset, nextOffset[, prevOffset])")
		}
		return sortLinkedEntitiesCall(t, args)
	case "debugError":
		if len(args) != 1 {
			return Num{}, t.errf(n, "debugError expects 1 argument (message)")
		}
		msgNode, err := args[0].Node()
		if err != nil {
			return Num{}, t.errf(n, "debugError: %v", err)
		}
		t.debugError(msgNode)
		return constNum(0), nil
	case "debugRequire", "debugAssertTrue":
		if len(args) != 2 {
			return Num{}, t.errf(n, "%s expects 2 arguments (condition, message)", fn.Name)
		}
		if _, err := args[1].Node(); err != nil {
			return Num{}, t.errf(n, "%s: %v", fn.Name, err)
		}
		t.debugRequire(args[0], args[1])
		return constNum(0), nil
	case "debugAssertFalse":
		if len(args) != 2 {
			return Num{}, t.errf(n, "debugAssertFalse expects 2 arguments (condition, message)")
		}
		// assertFalse(cond, msg) ≡ assertTrue(cond == 0, msg)
		zero := constNum(0)
		eq, ok := applyBinary(t.gen, token.EQL, args[0], zero)
		if !ok {
			return Num{}, t.errf(n, "debugAssertFalse: cannot compare condition")
		}
		if _, err := args[1].Node(); err != nil {
			return Num{}, t.errf(n, "debugAssertFalse: %v", err)
		}
		t.debugRequire(eq, args[1])
		return constNum(0), nil
	default:
		// User-defined record constructor: TypeName(val1, val2, ...)
		if fields, ok := t.env.Records[fn.Name]; ok {
			return t.inlineComposite(fn, n, fields)
		}
		if rf, ok := runtimeFns[fn.Name]; ok {
			if rf.arity >= 0 && len(args) != rf.arity {
				return Num{}, t.errf(n, "%s expects %d arguments, got %d", fn.Name, rf.arity, len(args))
			}
			nodes := make([]ir.Node, len(args))
			for i, a := range args {
				nd, err := a.Node()
				if err != nil {
					return Num{}, t.errf(n, "argument %d: %v", i, err)
				}
				nodes[i] = nd
			}
			if rf.returns {
				return exprNum(t.gen.PureInstr(rf.op, nodes...)), nil
			}
			t.emit(t.gen.ImpureInstr(rf.op, nodes...))
			return constNum(0), nil
		}
		if decl, ok := t.env.Funcs[fn.Name]; ok {
			return t.callUserFunc(fn, decl, args)
		}
		return Num{}, t.errf(n, "unknown function %q", fn.Name)
	}
}

// inlineComposite evaluates a vec2/quad constructor outside a declaration
// (e.g. in a return or argument position) and returns a composite Num.
func (t *tracer) inlineComposite(fnName *ast.Ident, call *ast.CallExpr, fields []string) (Num, error) {
	args := make([]Num, len(call.Args))
	for i, a := range call.Args {
		v, err := t.expr(a)
		if err != nil {
			return Num{}, err
		}
		args[i] = v
	}
	if len(args) != len(fields) {
		return Num{}, t.errf(call, "%s expects %d arguments, got %d", fnName, len(fields), len(args))
	}
	fv := map[string]Num{}
	for i, f := range fields {
		fv[f] = args[i]
	}
	return compNum(fv), nil
}

// callUserFunc inlines a free helper function: it sees only accessors and other
// functions (no archetype fields).
func (t *tracer) callUserFunc(fnName *ast.Ident, decl *ast.FuncDecl, args []Num) (Num, error) {
	child := Env{Names: t.env.Names, Accessors: t.env.Accessors, Funcs: t.env.Funcs, Methods: t.env.Methods, Constants: t.env.Constants}
	return t.inlineFunc(fnName, decl, args, child)
}

// sonolusCall handles sonolus.Function(args) calls by mapping the PascalCase
// function name to its camelCase runtimeFns entry.
// e.g. sonolus.Draw → draw, sonolus.DebugPause → debugPause, sonolus.GetShifted → getShifted.
// Constructor names with trailing underscore (Vec2_, Quad_) are stripped:
// sonolus.Vec2_ → vec2_ → vec2.
func (t *tracer) sonolusCall(n *ast.CallExpr, sel *ast.SelectorExpr) (Num, error) {
	fnName := lowerFirst(sel.Sel.Name)
	// Strip trailing underscore from constructor names (Vec2_ → vec2).
	fnName = strings.TrimSuffix(fnName, "_")
	fn := &ast.Ident{Name: fnName, NamePos: sel.Sel.NamePos}

	// Type-driven dispatch.
	if r, handled, err := t.resolveTypeCall(fn, n); handled {
		return r, err
	}
	// Builtin dispatch without pre-evaluated args.
	if r, handled, err := t.resolveBuiltinCall(fn, n); handled {
		return r, err
	}
	// Evaluate args and dispatch.
	args := make([]Num, len(n.Args))
	for i, a := range n.Args {
		v, err := t.expr(a)
		if err != nil {
			return Num{}, err
		}
		args[i] = v
	}
	return t.callWithArgs(fn, n, args)
}

// resolvePkgName returns the Go package name for an identifier, using go/types
// info when available. Returns "" if the identifier is not a package reference.
func (t *tracer) resolvePkgName(x ast.Expr) string {
	ident, ok := x.(*ast.Ident)
	if !ok {
		return ""
	}
	if info := t.env.Info; info != nil {
		if obj, ok2 := info.Uses[ident]; ok2 {
			if pkgName, ok3 := obj.(*types.PkgName); ok3 {
				return pkgName.Imported().Name()
			}
		}
	}
	// Fallback: known package names.
	if ident.Name == "sonolus" {
		return "sonolus"
	}
	return ""
}

// resolveRecordField resolves a struct field access like n.Ref (where n is the
// receiver and Ref is a record-typed field) into a composite Num and the record
// type name. It matches expanded field bindings (e.g. "Ref.index") against known
// record types to determine the type, then reads each field slot from memory.
func (t *tracer) resolveRecordField(sel *ast.SelectorExpr) (Num, string, bool) {
	fieldName := sel.Sel.Name

	// Check container struct fields first.
	for _, cf := range t.env.ContainerFields {
		if cf.Name == fieldName {
			return t.resolveContainerField(fieldName, cf)
		}
	}

	// Match expanded sub-field bindings against builtin and user record types.
	var recordName string
	var recordFields []string
FieldLoop:
	for _, rd := range builtinRecords {
		for _, f := range rd.fields {
			fullName := fieldName + "." + strings.ToLower(f)
			if _, ok := t.env.Names[fullName]; !ok {
				continue FieldLoop
			}
		}
		recordName = rd.name
		recordFields = rd.fields
		break
	}
	if recordName == "" {
	UserLoop:
		for rn, fields := range t.env.Records {
			for _, f := range fields {
				fullName := fieldName + "." + strings.ToLower(f)
				if _, ok := t.env.Names[fullName]; !ok {
					continue UserLoop
				}
			}
			recordName = rn
			recordFields = fields
			break
		}
	}
	if recordName == "" {
		return Num{}, "", false
	}

	// Derive base memory location from the first sub-field binding.
	firstField := fieldName + "." + strings.ToLower(recordFields[0])
	if _, ok := t.env.Names[firstField]; !ok {
		return Num{}, "", false
	}

	// Build a composite Num by reading each field slot from memory.
	// Slots are contiguous: field.index occupies the same slot as the sub-field binding.
	fieldVals := make(map[string]Num, len(recordFields))
	for i, f := range recordFields {
		fullName := fieldName + "." + strings.ToLower(f)
		bf, _ := t.env.Names[fullName]
		fieldVals[f] = exprNum(ir.GetPlace(ir.Cell(bf.Block, bf.Index)))
		_ = i
	}
	return compNum(fieldVals), recordName, true
}

// resolveContainerField builds a composite Num and a containerInfo for a
// container-typed struct field (VarArray, ArrayMap, ArraySet, FrozenNumSet).
// The field is backed by EntityMemory slots and uses the containerFn dispatch.
func (t *tracer) resolveContainerField(fieldName string, cf ContainerFieldMeta) (Num, string, bool) {
	// Look up the _size binding to get the base block and slot.
	sizeBinding, ok := t.env.Names[fieldName+".__size__"]
	if !ok {
		return Num{}, "", false
	}
	blockID := sizeBinding.Block
	baseIdx := sizeBinding.Index

	// Build composite Num matching the container's record type.
	fieldVals := map[string]Num{
		"_size":  exprNum(ir.GetPlace(ir.Cell(blockID, baseIdx))),
		"_array": constNum(float64(cf.Capacity)),
	}
	recordNum := compNum(fieldVals)

	// Register a field-backed containerInfo so containerFn dispatch finds it.
	ci := newContainerInfoField(blockID, baseIdx, cf.Capacity, cf.ElemSize, recordNum)
	t.containers[fieldName] = ci

	return recordNum, cf.TypeName, true
}

// populateFieldContainers creates containerInfo entries for container-typed
// struct fields declared in the archetype. Called once at the start of each
// callback compilation.
func (t *tracer) populateFieldContainers() {
	for _, cf := range t.env.ContainerFields {
		t.resolveContainerField(cf.Name, cf)
	}
}

// builtinGetBlock emits Get(block, index) for a fixed block ID.
func (t *tracer) builtinGetBlock(n *ast.CallExpr, blockID int) (Num, bool, error) {
	if len(n.Args) != 1 {
		return Num{}, true, t.errf(n, "expects 1 argument (index)")
	}
	indexVal, err := t.expr(n.Args[0])
	if err != nil {
		return Num{}, true, err
	}
	return exprNum(ir.GetPlace(ir.NewBlockPlace(ir.Const(blockID), indexVal.mustNode(), 0))), true, nil
}

// builtinSetBlock emits Set(block, index, value) for a fixed block ID.
func (t *tracer) builtinSetBlock(n *ast.CallExpr, blockID int) (Num, bool, error) {
	if len(n.Args) != 2 {
		return Num{}, true, t.errf(n, "expects 2 arguments (index, value)")
	}
	indexVal, err := t.expr(n.Args[0])
	if err != nil {
		return Num{}, true, err
	}
	val, err := t.expr(n.Args[1])
	if err != nil {
		return Num{}, true, err
	}
	t.emit(t.gen.SetPlace(ir.NewBlockPlace(ir.Const(blockID), indexVal.mustNode(), 0), val.mustNode()))
	return constNum(0), true, nil
}

// lowerFirst returns s with the first character lowercased.
// e.g. "Draw" → "draw", "DebugPause" → "debugPause".
func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}

// methodCall inlines a non-callback method of the current archetype, invoked as
// receiver.Method(args); the method body sees the archetype's fields via its own
// receiver.
func (t *tracer) methodCall(n *ast.CallExpr, sel *ast.SelectorExpr) (Num, error) {
	// Record method on a struct field: n.Ref.get(...) where Ref is a
	// record-typed field (EntityRef, Vec2, etc.).
	if inner, ok := sel.X.(*ast.SelectorExpr); ok {
		if base, ok2 := inner.X.(*ast.Ident); ok2 && base.Name == t.env.Receiver {
			if recordNum, recordType, ok3 := t.resolveRecordField(inner); ok3 {
				if methods, ok4 := recordMethods[recordType]; ok4 {
					entry, ok5 := methods[sel.Sel.Name]
					if !ok5 {
						entry, ok5 = methods[lowerFirst(sel.Sel.Name)]
					}
					if ok5 {
						args := make([]Num, len(n.Args))
						for i, a := range n.Args {
							v, err := t.expr(a)
							if err != nil {
								return Num{}, err
							}
							args[i] = v
						}
						for _, idx := range entry.compositeArgAt {
							if idx < len(args) && !args[idx].IsComposite() {
								return Num{}, t.errf(sel, "method %q arg %d must be a composite value", sel.Sel.Name, idx+1)
							}
						}
						if entry.containerFn != nil {
							if ci, ok := t.containers[inner.Sel.Name]; ok {
								return entry.containerFn(t, ci, recordNum, args)
							}
						}
						return entry.fn(t, recordNum, args)
					}
				}
			}
		}
		// Continue to next dispatch if not resolved as a record field method.
	}

	// Record method call: v.mul(s) where v is a vec2/quad/mat/rect/trans.
	// Uses the unified recordMethods registry (value.go) for table-driven dispatch.
	if base, ok := sel.X.(*ast.Ident); ok {
		if rec, ok := t.records[base.Name]; ok && rec.typeName != "" {
			if methods, ok2 := recordMethods[rec.typeName]; ok2 {
				entry, ok3 := methods[sel.Sel.Name]
				if !ok3 {
					entry, ok3 = methods[lowerFirst(sel.Sel.Name)]
				}
				if ok3 {
					if len(n.Args) < entry.minArity {
						return Num{}, t.errf(sel, "method %q expects at least %d args, got %d", sel.Sel.Name, entry.minArity, len(n.Args))
					}
					args := make([]Num, len(n.Args))
					for i, a := range n.Args {
						v, err := t.expr(a)
						if err != nil {
							return Num{}, err
						}
						args[i] = v
					}

					for _, idx := range entry.compositeArgAt {
						if idx < len(args) && !args[idx].IsComposite() {
							return Num{}, t.errf(sel, "method %q arg %d must be a composite value", sel.Sel.Name, idx+1)
						}
					}
					if entry.containerFn != nil {
						if ci, ok := t.containers[base.Name]; ok {
							return entry.containerFn(t, ci, rec.val, args)
						}
						return Num{}, t.errf(sel, "method %q on %q requires a container-backed local", sel.Sel.Name, rec.typeName)
					}
					return entry.fn(t, rec.val, args)
				}
			}
		}
	}

	base, ok := sel.X.(*ast.Ident)
	if !ok || t.env.Receiver == "" || base.Name != t.env.Receiver {
		return Num{}, t.errf(sel, "unsupported method call (only methods on archetype receivers and known record types like Vec2, Quad are supported)")
	}
	decl, ok := t.env.Methods[sel.Sel.Name]
	if !ok {
		return Num{}, t.errf(sel, "unknown method %q", sel.Sel.Name)
	}
	args := make([]Num, len(n.Args))
	for i, a := range n.Args {
		v, err := t.expr(a)
		if err != nil {
			return Num{}, err
		}
		args[i] = v
	}
	recv := ""
	if decl.Recv != nil && len(decl.Recv.List) > 0 && len(decl.Recv.List[0].Names) > 0 {
		recv = decl.Recv.List[0].Names[0].Name
	}
	child := Env{Names: t.env.Names, Receiver: recv, Funcs: t.env.Funcs, Methods: t.env.Methods, Accessors: t.env.Accessors, Constants: t.env.Constants}
	return t.inlineFunc(sel, decl, args, child)
}

// inlineFunc inlines a function/method body: it binds parameters, traces the
// body in a fresh scope under childEnv, routes returns to a continuation block,
// and yields the return value.
func (t *tracer) inlineFunc(node ast.Node, decl *ast.FuncDecl, args []Num, childEnv Env) (Num, error) {
	if t.inlining[decl.Name.Name] {
		return Num{}, t.errf(node, "recursive call to %q is not supported (helper functions are inlined at every call site, so recursion would cause infinite expansion)", decl.Name.Name)
	}
	params := funcParams(decl)

	// Detect variadic function: last parameter has ...Type (*ast.Ellipsis).
	isVariadic := false
	var variadicName string
	if decl.Type.Params != nil && len(decl.Type.Params.List) > 0 {
		lastParam := decl.Type.Params.List[len(decl.Type.Params.List)-1]
		if _, isVar := lastParam.Type.(*ast.Ellipsis); isVar && len(lastParam.Names) > 0 {
			isVariadic = true
			variadicName = lastParam.Names[0].Name
		}
	}

	if isVariadic {
		minArgs := len(params) - 1
		if len(args) < minArgs {
			return Num{}, t.errf(node, "%s expects at least %d arguments, got %d", decl.Name.Name, minArgs, len(args))
		}
	} else if len(args) != len(params) {
		return Num{}, t.errf(node, "%s expects %d arguments, got %d", decl.Name.Name, len(params), len(args))
	}

	retTemp := &ir.TempBlock{Name: decl.Name.Name + ".$ret", Size: 1}
	cont := ir.NewBlock()

	savedVars, savedArrays, savedRecords, savedContainers, savedEnv := t.vars, t.arrays, t.records, t.containers, t.env
	t.vars = map[string]*ir.TempBlock{}
	t.arrays = map[string]*arrayInfo{}
	t.records = map[string]*recordInfo{}
	t.containers = map[string]*containerInfo{}
	t.env = childEnv

	// Bind regular (non-variadic) parameters.
	boundParams := params
	if isVariadic {
		boundParams = params[:len(params)-1]
	}
	for i, p := range boundParams {
		// D3: try type-driven composite detection via declared parameter type.
		isComp := args[i].IsComposite()
		if !isComp && t.env.Info != nil && i < len(decl.Type.Params.List) {
			field := decl.Type.Params.List[i]
			if len(field.Names) > 0 {
				if id, ok := field.Type.(*ast.Ident); ok {
					if _, isRecord := recordMethods[id.Name]; isRecord {
						isComp = true
					}
				}
			}
		}
		if isComp {
			// Bind each composite field as a separate local, plus a composite
			// record in the function's scope. This emulates recordDecl.
			order, err := args[i].CompositeFieldOrder()
			if err != nil {
				return Num{}, err
			}
			rec := &recordInfo{
				tb:     &ir.TempBlock{Name: p, Size: args[i].CompositeSize()},
				fields: map[string]int{},
				order:  order,
				val:    args[i],
			}
			for j, f := range rec.order {
				rec.fields[f] = j
			}
			t.records[p] = rec
		} else {
			pt := t.alloc(p)
			t.emit(t.gen.SetPlace(ir.TempCell(pt), args[i].mustNode()))
		}
	}

	// Bind variadic extra args as a compile-time-sized array.
	if isVariadic {
		extraArgs := args[len(params)-1:]
		varTB := &ir.TempBlock{Name: variadicName, Size: len(extraArgs)}
		for i, a := range extraArgs {
			t.emit(t.gen.SetPlace(ir.BlockPlace{Block: varTB, Index: ir.Const(i), Offset: 0}, a.mustNode()))
		}
		elems := make([]Num, len(extraArgs))
		for i := range extraArgs {
			elems[i] = exprNum(ir.GetPlace(ir.BlockPlace{Block: varTB, Index: ir.Const(i), Offset: 0}))
		}
		t.arrays[variadicName] = &arrayInfo{
			tb:       varTB,
			count:    len(extraArgs),
			elemSize: 1,
			elemNum:  arrayNum(elems),
		}
	}

	t.returns = append(t.returns, returnCtx{temp: retTemp, target: cont})
	t.defers = append(t.defers, deferCtx{})
	t.inlining[decl.Name.Name] = true
	err := t.stmtList(decl.Body.List)
	t.fallthroughTo(cont)
	// Emit deferred calls at function exit before restoring scope.
	if defErr := t.emitDefers(); defErr != nil && err == nil {
		err = defErr
	}
	addRet := t.returns[len(t.returns)-1]
	compositeFields := addRet.compositeFields
	delete(t.inlining, decl.Name.Name)
	t.returns = t.returns[:len(t.returns)-1]

	t.vars, t.arrays, t.records, t.containers, t.env = savedVars, savedArrays, savedRecords, savedContainers, savedEnv
	t.enter(cont)
	if err != nil {
		return Num{}, err
	}

	if compositeFields != nil {
		// Build a composite Num by reading each field from retTemp slots.
		fieldVals := map[string]Num{}
		for i, f := range compositeFields {
			fieldVals[f] = exprNum(ir.GetPlace(ir.BlockPlace{Block: retTemp, Index: ir.Const(i), Offset: 0}))
		}
		return compNum(fieldVals), nil
	}
	return exprNum(ir.GetPlace(ir.TempCell(retTemp))), nil
}

// callClosure invokes a captured closure. It sets up a fresh scope with
// capture bindings, then traces the closure body inline.
func (t *tracer) callClosure(fn *ast.Ident, call *ast.CallExpr, ci *closureInfo) (Num, error) {
	// Evaluate call arguments.
	args := make([]Num, len(call.Args))
	for i, a := range call.Args {
		v, err := t.expr(a)
		if err != nil {
			return Num{}, err
		}
		args[i] = v
	}

	// Build a synthetic FuncDecl for inlineFunc.
	decl := &ast.FuncDecl{
		Name: &ast.Ident{Name: fn.Name},
		Type: ci.fn.Type,
		Body: ci.fn.Body,
	}

	// Create a child environment — captures are bound as writable accessors
	// via t.vars (not env.Names, since captures are scope locals).
	childEnv := Env{
		Funcs:     t.env.Funcs,
		Accessors: t.env.Accessors,
		Mode:      t.env.Mode,
		Info:      t.env.Info,
	}
	if childEnv.Funcs == nil {
		childEnv.Funcs = map[string]*ast.FuncDecl{}
	}
	delete(childEnv.Funcs, fn.Name)

	// Save scope, create fresh scope with capture bindings.
	savedVars := t.vars
	savedRecords := t.records
	t.vars = make(map[string]*ir.TempBlock)
	t.records = make(map[string]*recordInfo)

	// Bind captures into the fresh scope by reading from the capture frame.
	for i, capName := range ci.captures {
		tb := t.alloc(capName)
		capVal := exprNum(ir.GetPlace(ir.BlockPlace{Block: ci.frame, Index: ir.Const(i), Offset: 0}))
		t.emit(t.gen.SetPlace(t.cell(tb), capVal.mustNode()))
	}

	// inlineFunc will save/restore scope and bind parameters.
	result, err := t.inlineFunc(call, decl, args, childEnv)

	// Restore outer scope.
	t.vars = savedVars
	t.records = savedRecords
	return result, err
}

