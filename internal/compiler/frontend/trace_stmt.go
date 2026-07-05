package frontend

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/ir"
)

var compoundOps = map[token.Token]token.Token{
	token.ADD_ASSIGN: token.ADD, token.SUB_ASSIGN: token.SUB,
	token.MUL_ASSIGN: token.MUL, token.QUO_ASSIGN: token.QUO,
	token.REM_ASSIGN: token.REM,
}

// multiAssign handles tuple assignment: a, b, c := f() where f() returns a
// composite (multi-return or record). Each LHS identifier is bound to the
// corresponding field of the composite value.
func (t *tracer) multiAssign(n *ast.AssignStmt) error {
	if len(n.Rhs) != 1 {
		return t.errf(n, "multi-assignment requires exactly one RHS expression")
	}
	rhs, err := t.expr(n.Rhs[0])
	if err != nil {
		return err
	}
	if !rhs.IsComposite() {
		return t.errf(n, "multi-assignment RHS must be a composite (multi-return or record)")
	}
	fields, err := rhs.CompositeFieldOrder()
	if err != nil {
		return err
	}
	for i, lhs := range n.Lhs {
		// Blank identifier: discard this field value.
		if id, ok := lhs.(*ast.Ident); ok && id.Name == "_" {
			continue
		}
		// Resolve field: prefer positional (fields[i]), fall back to lhs name.
		var fval Num
		var okField bool
		if i < len(fields) {
			fval, okField = rhs.TryField(fields[i])
		}
		if !okField {
			// For identifier LHS, try name-based lookup.
			if id, okID := lhs.(*ast.Ident); okID {
				fval, okField = rhs.TryField(id.Name)
			}
		}
		if !okField {
			return t.errf(lhs, "multi-assignment: no field for position %d in composite (available: %v)", i, fields)
		}
		if n.Tok == token.DEFINE {
			// := requires identifier LHS
			id, okIdent := lhs.(*ast.Ident)
			if !okIdent {
				return t.errf(lhs, "multi-assignment := requires identifier targets (use = for field/index targets)")
			}
			tb := t.alloc(id.Name)
			t.emit(t.gen.SetPlace(ir.TempCell(tb), fval.mustNode()))
		} else {
			// = can target identifiers, fields, or array elements
			if id, okIdent := lhs.(*ast.Ident); okIdent {
				if tb, ok2 := t.vars[id.Name]; ok2 {
					t.emit(t.gen.SetPlace(ir.TempCell(tb), fval.mustNode()))
				} else if b, ok2 := t.env.Names[id.Name]; ok2 && b.Writable {
					t.emitBindingStore(b, fval)
				} else {
					return t.errf(lhs, "cannot assign to %q (variable has no local or writable binding; use := to declare a new variable)", id.Name)
				}
			} else {
				// Non-identifier LHS: use writePlace
				if err := t.writePlace(lhs, fval); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (t *tracer) assign(n *ast.AssignStmt) error {
	// Compound assignment: x += y → x = x + y
	if nt := n.Tok; nt != token.ASSIGN && nt != token.DEFINE {
		if binOp, ok := compoundOps[nt]; ok {
			return t.compoundAssign(n, binOp)
		}
		return t.errf(n, "unsupported assignment %s (only =, :=, +=, -=, *=, /=, %%= are supported; no bitwise &=, |=, <<=, >>=, etc.)", n.Tok)
	}
	// Multi-LHS (tuple assignment): a, b := f() where f() returns a composite.
	if len(n.Lhs) > 1 && len(n.Rhs) == 1 {
		return t.multiAssign(n)
	}
	if len(n.Lhs) != 1 || len(n.Rhs) != 1 {
		return t.errf(n, "only single assignment is supported")
	}

	// Array element write: a[i] = expr.
	if idx, ok := n.Lhs[0].(*ast.IndexExpr); ok {
		return t.arrayStore(idx, n.Rhs[0])
	}
	// Record field write: v.field = expr.
	if sel, ok := n.Lhs[0].(*ast.SelectorExpr); ok {
		return t.fieldStore(sel, n.Rhs[0])
	}

	lhsName, ok := n.Lhs[0].(*ast.Ident)
	if !ok {
		return t.errf(n, "assignment target must be an identifier")
	}

	// Blank identifier: evaluate RHS for side effects and discard the value.
	if lhsName.Name == "_" {
		_, err := t.expr(n.Rhs[0])
		return err
	}

	// Field write: `=` to an env binding not shadowed by a local. (`:=` always
	// declares a fresh local, shadowing any binding.)
	if _, isLocal := t.vars[lhsName.Name]; !isLocal && n.Tok == token.ASSIGN {
		if b, ok := t.env.Names[lhsName.Name]; ok {
			if !b.Writable {
				return t.errf(n, "cannot assign to read-only %q", lhsName.Name)
			}
			val, err := t.expr(n.Rhs[0])
			if err != nil {
				return err
			}
			t.emitBindingStore(b, val)
			return nil
		}
	}

	// Composite declarations: a := array(n) / v := vec2(x, y).
	// If the constructor is not recognized, fall through to regular assignment.
	if call, ok := n.Rhs[0].(*ast.CallExpr); ok {
		if fn, ok := unwrapCallFun(call.Fun).(*ast.Ident); ok {
			handled, err := t.compositeDecl(lhsName, fn, call)
			if handled {
				return err
			}
		}
	}

	val, err := t.expr(n.Rhs[0])
	if err != nil {
		return err
	}

	// Composite value from a function return or constructor: register as a record.
	if n.Tok == token.DEFINE && val.IsComposite() {
		order, err := val.CompositeFieldOrder()
		if err != nil {
			return err
		}
		rec := &recordInfo{
			tb:     &ir.TempBlock{Name: lhsName.Name, Size: val.CompositeSize()},
			fields: map[string]int{},
			order:  order,
			val:    val,
		}
		for i, f := range rec.order {
			rec.fields[f] = i
		}
		// Infer record type name from field list for method dispatch.
		if val.typeName != "" {
			rec.typeName = val.typeName
		} else {
			rec.typeName = recordTypeNameFromFields(order)
		}
		t.records[lhsName.Name] = rec
		return nil
	}

	tb, ok := t.vars[lhsName.Name]
	if !ok {
		if n.Tok != token.DEFINE {
			return t.errf(n, "assignment to undefined variable %q (use := to declare a new variable, or declare it before assigning with =)", lhsName.Name)
		}
		tb = t.alloc(lhsName.Name)
	}
	t.emit(t.gen.SetPlace(t.cell(tb), val.mustNode()))
	return nil
}

// compositeDecl handles `x := constructor(args)` declarations where the RHS is
// a call to a known record/container constructor. It dispatches via D3 type
// resolution (types.Info) when available, falling back to name-based lookup.
// The first return value reports whether the constructor was recognized (handled).
// If false, the caller should fall through to regular assignment.
func (t *tracer) compositeDecl(varName *ast.Ident, fn *ast.Ident, call *ast.CallExpr) (handled bool, _ error) {
	// D3: use types.Info to resolve record constructors by return type.
	if st, typeName, ok := t.resolveRecordType(fn); ok {
		if typeName == "VarArray" || typeName == "ArrayMap" || typeName == "FrozenNumSet" {
			return true, t.varArrayDecl(varName, call)
		}
		if typeName == "ArraySet" {
			return true, t.arraySetDecl(varName, call)
		}
		fields := make([]string, st.NumFields())
		for i := range st.NumFields() {
			fields[i] = st.Field(i).Name()
		}
		return true, t.recordDecl(varName, call, fields)
	}
	// Fallback: name-based dispatch (when Info is nil or type lookup fails).
	switch fn.Name {
	case "array":
		return true, t.arrayDecl(varName, call)
	case "vec2":
		return true, t.recordDecl(varName, call, vec2Fields)
	case "quad":
		return true, t.recordDecl(varName, call, quadFields)
	case "mat":
		return true, t.recordDecl(varName, call, matFields)
	case "rect":
		return true, t.recordDecl(varName, call, rectFields)
	case "trans":
		return true, t.recordDecl(varName, call, transFields)
	case "judgmentWindow":
		return true, t.recordDecl(varName, call, judgmentWindowFields)
	case "sprite":
		return true, t.recordDecl(varName, call, spriteFields)
	case "effect":
		return true, t.recordDecl(varName, call, effectFields)
	case "particle":
		return true, t.recordDecl(varName, call, particleFields)
	case "entityRef":
		return true, t.recordDecl(varName, call, entityRefFields)
	case "pair":
		return true, t.recordDecl(varName, call, pairFields)
	case "box":
		return true, t.recordDecl(varName, call, boxFields)
	case "frozenNumSet":
		return true, t.varArrayDecl(varName, call)
	case "varArray", "arrayMap":
		return true, t.varArrayDecl(varName, call)
	case "arraySet":
		return true, t.arraySetDecl(varName, call)
	}
	return false, nil
}

func (t *tracer) incDec(n *ast.IncDecStmt) error {
	binOp := token.ADD
	if n.Tok == token.DEC {
		binOp = token.SUB
	}

	// Read the current value from the LHS.
	var cur Num
	switch lhs := n.X.(type) {
	case *ast.Ident:
		tb, ok := t.vars[lhs.Name]
		if !ok {
			return t.errf(n, "increment of undefined variable %q", lhs.Name)
		}
		cur = exprNum(ir.GetPlace(t.cell(tb)))
	case *ast.SelectorExpr:
		place, err := t.fieldPlace(lhs)
		if err != nil {
			return err
		}
		cur = exprNum(ir.GetPlace(place))
	case *ast.IndexExpr:
		varName, ok := lhs.X.(*ast.Ident)
		if !ok {
			return t.errf(n, "increment target must be an identifier (++ and -- only work on named variables, not field access or array indexing)")
		}
		place, err := t.arrayElemPlace(varName, lhs.Index)
		if err != nil {
			return err
		}
		cur = exprNum(ir.GetPlace(place))
	default:
		return t.errf(n, "increment target must be an identifier (++ and -- only work on named variables, not field access or array indexing)")
	}

	// Apply the increment/decrement.
	result, ok := applyBinary(t.gen, binOp, cur, constNum(1))
	if !ok {
		return t.errf(n, "cannot apply increment/decrement")
	}

	return t.writePlace(n.X, result)
}

// arrayDecl handles `varName := array(count)` and `varName := array[Type](count)`:
// it reserves a multi-slot temp. For record types, each element occupies elemSize
// slots and can be indexed with `.Field` access.
func (t *tracer) arrayDecl(arrName *ast.Ident, call *ast.CallExpr) error {
	if len(call.Args) != 1 {
		return t.errf(call, "array expects a constant size")
	}
	size, err := t.expr(call.Args[0])
	if err != nil {
		return err
	}
	if !size.isConst || size.c < 1 || size.c != float64(int(size.c)) {
		return t.errf(call, "array size must be a positive integer constant")
	}
	elemSize := 1
	// Check if this is array[RecordType](count) via type info.
	if idx, ok := call.Fun.(*ast.IndexExpr); ok {
		if id, ok := idx.X.(*ast.Ident); ok && id.Name == "array" {
			if st, _, ok3 := t.resolveRecordType(id); ok3 {
				elemSize = st.NumFields()
			}
		}
	}
	ai := &arrayInfo{
		tb:       &ir.TempBlock{Name: arrName.Name, Size: int(size.c) * elemSize},
		count:    int(size.c),
		elemSize: elemSize,
	}
	// Pre-populate element Nums as expression reads so optimizer can fold them.
	elems := make([]Num, ai.count)
	for i := range ai.count {
		elems[i] = exprNum(ir.GetPlace(ir.BlockPlace{Block: ai.tb, Index: ir.Const(i * elemSize), Offset: 0}))
	}
	ai.elemNum = arrayNum(elems)
	t.arrays[arrName.Name] = ai
	return nil
}

// varArrayDecl handles `arr := varArray(capacity)`. It allocates a backing
// TempBlock of size 1+capacity (slot 0 = _size, slots 1..capacity = elements),
// creates a recordInfo for field tracking (so method dispatch works), and stores
// a containerInfo for methods that need the backing array.
func (t *tracer) varArrayDecl(arrName *ast.Ident, call *ast.CallExpr) error {
	fnIdent, ok := unwrapCallFun(call.Fun).(*ast.Ident)
	if !ok {
		return t.errf(call, "varArray/arrayMap constructor must be called by name, not expression (use varArray(n) directly, not via a variable or function return)")
	}
	if len(call.Args) != 1 {
		return t.errf(call, "varArray expects exactly 1 argument (capacity)")
	}
	capVal, err := t.expr(call.Args[0])
	if err != nil {
		return err
	}
	if !capVal.isConst || capVal.c < 1 || capVal.c != float64(int(capVal.c)) {
		return t.errf(call, "varArray capacity must be a positive integer constant")
	}
	capacity := int(capVal.c)
	elemSize := 1
	if fnIdent.Name == "arrayMap" {
		elemSize = 2 // key + value per entry
	}
	totalSlots := 1 + capacity*elemSize // slot 0 = _size, rest = elements

	tb := &ir.TempBlock{Name: arrName.Name, Size: totalSlots}

	// Build the composite Num: _size=0, _array as a placeholder (indexed access
	// will use the TempBlock directly via BlockPlace).
	fields := map[string]Num{
		"_size": constNum(0),
		// _array is not a regular scalar — store as a sentinel.
		// Methods that need the backing array will look up the containerInfo.
		"_array": constNum(float64(capacity)),
	}

	ri := &recordInfo{
		tb:       tb,
		fields:   map[string]int{"_size": 0, "_array": 1},
		order:    []string{"_size", "_array"},
		val:      compNum(fields),
		typeName: fnIdent.Name, // "varArray" or "arrayMap"
	}
	t.records[arrName.Name] = ri

	ci := newContainerInfoLocal(tb, capacity, elemSize, ri.val)
	t.containers[arrName.Name] = ci

	return nil
}

// arraySetDecl handles `s := arraySet(capacity)`. It uses the same backing layout
// as varArrayDecl (1+capacity slots) but sets typeName to "arraySet" so method
// dispatch routes to arraySet methods.
func (t *tracer) arraySetDecl(arrName *ast.Ident, call *ast.CallExpr) error {
	if err := t.varArrayDecl(arrName, call); err != nil {
		return err
	}
	// Override the typeName so method dispatch uses recordMethods["arraySet"].
	if ri, ok := t.records[arrName.Name]; ok {
		ri.typeName = "arraySet"
	}
	return nil
}

// arrayElemPlace builds the place for a[index]. For scalar arrays the place is
// `Block(arr.tb, index)`. For record arrays (elemSize > 1), the slot is
// `index * elemSize + fieldOffset` with fieldOffset in the Place.Offset.
func (t *tracer) arrayElemPlace(arrName *ast.Ident, indexExpr ast.Expr) (ir.BlockPlace, error) {
	arr, ok := t.arrays[arrName.Name]
	if !ok {
		return ir.BlockPlace{}, t.errf(arrName, "undefined array %q", arrName.Name)
	}
	index, err := t.expr(indexExpr)
	if err != nil {
		return ir.BlockPlace{}, err
	}
	idx := index.mustNode()
	if arr.elemSize > 1 {
		idx = t.gen.PureInstr(resource.RuntimeFunctionMultiply, idx, ir.Const(arr.elemSize))
	}
	return ir.BlockPlace{Block: arr.tb, Index: idx, Offset: 0}, nil
}

func (t *tracer) compoundAssign(n *ast.AssignStmt, binOp token.Token) error {
	lhs := n.Lhs[0]
	rhs, err := t.expr(n.Rhs[0])
	if err != nil {
		return err
	}

	// Read the current value of the LHS.
	var cur Num
	switch l := lhs.(type) {
	case *ast.Ident:
		if tb, ok := t.vars[l.Name]; ok {
			cur = exprNum(ir.GetPlace(t.cell(tb)))
		} else if b, ok := t.env.Names[l.Name]; ok {
			cur = exprNum(ir.GetPlace(ir.Cell(b.Block, b.Index)))
		} else {
			return t.errf(n, "undefined variable %q in compound assignment", l.Name)
		}
	case *ast.SelectorExpr:
		place, err := t.fieldPlace(l)
		if err != nil {
			return err
		}
		cur = exprNum(ir.GetPlace(place))
	case *ast.IndexExpr:
		id, ok := l.X.(*ast.Ident)
		if !ok {
			return t.errf(n, "only named array variables supported for += etc.")
		}
		place, err := t.arrayElemPlace(id, l.Index)
		if err != nil {
			return err
		}
		cur = exprNum(ir.GetPlace(place))
	default:
		return t.errf(n, "unsupported compound assign target %T (compound assignment +=, -=, etc. requires a variable, struct field, or array element)", lhs)
	}

	result, ok := applyBinary(t.gen, binOp, cur, rhs)
	if !ok {
		return t.errf(n, "unsupported compound operation (only +, -, *, /, %% are supported in compound assignments; no bitwise or logical operators)")
	}
	return t.writePlace(lhs, result)
}

func (t *tracer) writePlace(lhs ast.Expr, val Num) error {
	switch l := lhs.(type) {
	case *ast.Ident:
		// Blank identifier: discard the value.
		if l.Name == "_" {
			return nil
		}
		if tb, ok := t.vars[l.Name]; ok {
			t.emit(t.gen.SetPlace(t.cell(tb), val.mustNode()))
			return nil
		}
		if b, ok := t.env.Names[l.Name]; ok {
			if !b.Writable {
				return t.errf(lhs, "cannot write %q: read-only", l.Name)
			}
			t.emitBindingStore(b, val)
			return nil
		}
	case *ast.SelectorExpr:
		place, err := t.fieldPlace(l)
		if err != nil {
			return err
		}
		t.emit(t.gen.SetPlace(place, val.mustNode()))
		if base, ok2 := l.X.(*ast.Ident); ok2 {
			if rec, ok3 := t.records[base.Name]; ok3 {
				if err := rec.val.SetField(l.Sel.Name, val); err != nil {
					return err
				}
			}
		}
		return nil
	case *ast.IndexExpr:
		varName, ok := l.X.(*ast.Ident)
		if !ok {
			return t.errf(lhs, "array index target must be an identifier")
		}
		place, err := t.arrayElemPlace(varName, l.Index)
		if err != nil {
			return err
		}
		t.emit(t.gen.SetPlace(place, val.mustNode()))
		return nil
	}
	return t.errf(lhs, "cannot write compound assign to %T (compound assignment requires a variable, struct field, or array element)", lhs)
}

func (t *tracer) arrayStore(idx *ast.IndexExpr, rhs ast.Expr) error {
	place, err := t.indexPlace(idx)
	if err != nil {
		return err
	}
	val, err := t.expr(rhs)
	if err != nil {
		return err
	}
	t.emit(t.gen.SetPlace(place, val.mustNode()))
	return nil
}

// recordDecl handles `varName := vec2(x, y)` (and future record constructors): it
// reserves a temp with one slot per field, stores the initializers, and tracks
// each field as an individual Num for scalar-replaceable reads.
func (t *tracer) recordDecl(varName *ast.Ident, call *ast.CallExpr, fields []string) error {
	fnIdent, ok := unwrapCallFun(call.Fun).(*ast.Ident)
	if !ok {
		return t.errf(call, "record constructor must be called by name, not expression (use vec2(x, y) directly, not via a variable or function return)")
	}
	if len(call.Args) != len(fields) {
		return t.errf(call, "%s expects %d arguments", fnIdent.Name, len(fields))
	}
	typeName := fnIdent.Name
	rec := &recordInfo{
		tb:       &ir.TempBlock{Name: varName.Name, Size: len(fields)},
		fields:   map[string]int{},
		order:    fields,
		typeName: typeName,
	}
	fieldVals := map[string]Num{}
	for i, f := range fields {
		rec.fields[f] = i
	}
	for i, arg := range call.Args {
		val, err := t.expr(arg)
		if err != nil {
			return err
		}
		t.emit(t.gen.SetPlace(ir.BlockPlace{Block: rec.tb, Index: ir.Const(i), Offset: 0}, val.mustNode()))
		fieldVals[fields[i]] = val
	}
	rec.val = compNum(fieldVals)
	t.records[varName.Name] = rec
	return nil
}

// receiverBinding resolves Receiver.Field against the env. isRecv reports
// whether sel is a receiver access (so callers don't fall through to records).
func (t *tracer) receiverBinding(sel *ast.SelectorExpr) (b Binding, isRecv bool, err error) {
	base, ok := sel.X.(*ast.Ident)
	if !ok || t.env.Receiver == "" || base.Name != t.env.Receiver {
		return Binding{}, false, nil
	}
	binding, ok := t.env.Names[sel.Sel.Name]
	if !ok {
		return Binding{}, true, t.errf(sel, "unknown field %q", sel.Sel.Name)
	}
	return binding, true, nil
}

// fieldValue returns the traced Num for a record field read, using the tracked
// composite value for scalar-replaceable folding.
func (t *tracer) fieldValue(sel *ast.SelectorExpr) (Num, error) {
	base, ok := sel.X.(*ast.Ident)
	if !ok {
		return Num{}, t.errf(sel, "field access requires a record identifier")
	}
	rec, ok := t.records[base.Name]
	if !ok {
		if _, isScalar := t.vars[base.Name]; isScalar {
			return Num{}, t.errf(sel, "scalar variable %q has no fields", base.Name)
		}
		return Num{}, t.errf(sel, "undefined record %q", base.Name)
	}
	off, ok := rec.fields[sel.Sel.Name]
		if !ok {
			off, ok = rec.fields[lowerFirst(sel.Sel.Name)]
		}
	if !ok {
		off, ok = rec.fields[lowerFirst(sel.Sel.Name)]
	}
	if !ok {
		return Num{}, t.errf(sel, "record %q has no field %q", base.Name, sel.Sel.Name)
	}
	// Return the tracked field Num if it's a constant or pure expression; fall
	// back to a memory read for mutable-backed fields.
	if v, ok := rec.val.fields[sel.Sel.Name]; ok && (v.isConst || v.e != nil || v.IsComposite()) {
		return v, nil
	}
	if v, ok := rec.val.fields[lowerFirst(sel.Sel.Name)]; ok && (v.isConst || v.e != nil || v.IsComposite()) {
		return v, nil
	}
	return exprNum(ir.GetPlace(ir.BlockPlace{Block: rec.tb, Index: ir.Const(off), Offset: 0})), nil
}

// fieldPlace builds the place for v.field (used for writes).
func (t *tracer) fieldPlace(sel *ast.SelectorExpr) (ir.BlockPlace, error) {
	// Record-typed struct field: n.pos.X → look up "pos.x" in bindings.
	if inner, ok := sel.X.(*ast.SelectorExpr); ok {
		if base, ok2 := inner.X.(*ast.Ident); ok2 && t.env.Receiver != "" && base.Name == t.env.Receiver {
			fullName := inner.Sel.Name + "." + lowerFirst(sel.Sel.Name)
			if b, ok3 := t.env.Names[fullName]; ok3 {
				return ir.NewBlockPlace(ir.Const(b.Block), ir.Const(b.Index), 0), nil
			}
		}
	}

	base, ok := sel.X.(*ast.Ident)
	if !ok {
		return ir.BlockPlace{}, t.errf(sel, "field access requires a record identifier")
	}
	rec, ok := t.records[base.Name]
	if !ok {
		return ir.BlockPlace{}, t.errf(sel, "undefined record %q", base.Name)
	}
	off, ok := rec.fields[sel.Sel.Name]
		if !ok {
			off, ok = rec.fields[lowerFirst(sel.Sel.Name)]
		}
	if !ok {
		return ir.BlockPlace{}, t.errf(sel, "record %q has no field %q", base.Name, sel.Sel.Name)
	}
	return ir.BlockPlace{Block: rec.tb, Index: ir.Const(off), Offset: 0}, nil
}

func (t *tracer) fieldStore(sel *ast.SelectorExpr, rhs ast.Expr) error {
	// Record-typed struct field write: n.pos.X = value → look up "pos.x" in bindings.
	if inner, ok := sel.X.(*ast.SelectorExpr); ok {
		if base, ok2 := inner.X.(*ast.Ident); ok2 && t.env.Receiver != "" && base.Name == t.env.Receiver {
			fullName := inner.Sel.Name + "." + lowerFirst(sel.Sel.Name)
			if b, ok3 := t.env.Names[fullName]; ok3 {
				if !b.Writable {
					return t.errf(sel, "cannot assign to read-only field %q", fullName)
				}
				val, err := t.expr(rhs)
				if err != nil {
					return err
				}
				t.emitBindingStore(b, val)
				return nil
			}
		}
	}
	// Composite writeback: decompose a record value into sub-field writes.
	if base, ok := sel.X.(*ast.Ident); ok && base.Name == t.env.Receiver {
		if _, recordType, ok2 := t.resolveRecordField(sel); ok2 {
			val, err := t.expr(rhs)
			if err != nil {
				return err
			}
			if val.IsComposite() {
				fields, ok3 := builtinRecordFields(recordType)
				if !ok3 {
					var ferr error
					fields, ferr = val.CompositeFieldOrder()
					if ferr != nil {
						return ferr
					}
				}
				for _, f := range fields {
					fullName := sel.Sel.Name + "." + f
					b, ok4 := t.env.Names[fullName]
					if !ok4 {
						continue
					}
					fv := val.MustField(f)
					t.emitBindingStore(b, fv)
				}
				return nil
			}
		}
	}


	// Receiver field write (method-authored callback).
	if b, isRecv, err := t.receiverBinding(sel); err != nil {
		return err
	} else if isRecv {
		if !b.Writable {
			return t.errf(sel, "cannot assign to read-only field %q", sel.Sel.Name)
		}
		val, err := t.expr(rhs)
		if err != nil {
			return err
		}
		t.emitBindingStore(b, val)
		return nil
	}

	place, err := t.fieldPlace(sel)
	if err != nil {
		return err
	}
	val, err := t.expr(rhs)
	if err != nil {
		return err
	}
	t.emit(t.gen.SetPlace(place, val.mustNode()))
	// Update the tracked composite Num so subsequent reads fold the new value.
	if base, ok := sel.X.(*ast.Ident); ok {
		if rec, ok2 := t.records[base.Name]; ok2 {
			if err := rec.val.SetField(sel.Sel.Name, val); err != nil {
				return err
			}
		}
	}
	return nil
}

// emitBindingStore writes val to binding b — either as an ExportValue (for
// blocks with negative block ids, i.e. exported fields) or a SetPlace.
func (t *tracer) emitBindingStore(b Binding, val Num) {
	if b.Block < 0 { // exported field: emit ExportValue(index, value)
		t.emit(t.gen.ImpureInstr(resource.RuntimeFunctionExportValue, ir.Const(b.Index), val.mustNode()))
	} else {
		t.emit(t.gen.SetPlace(ir.Cell(b.Block, b.Index), val.mustNode()))
	}
}

// resolveRecordType resolves an identifier to its underlying struct type via the
// types.Info chain: Ident → types.Func → types.Signature → types.Named → types.Struct.
// Returns the struct type, the named type name, and whether resolution succeeded.
// Only works for functions that return a single struct-typed result (constructors).
func (t *tracer) resolveRecordType(fn *ast.Ident) (st *types.Struct, typeName string, _ bool) {
	info := t.env.Info
	if info == nil {
		return nil, "", false
	}
	obj, ok := info.Uses[fn]
	if !ok {
		return nil, "", false
	}
	fobj, ok := obj.(*types.Func)
	if !ok {
		return nil, "", false
	}
	sig, ok := fobj.Type().(*types.Signature)
	if !ok {
		return nil, "", false
	}
	if sig.Results().Len() == 0 {
		return nil, "", false
	}
	named, ok := sig.Results().At(0).Type().(*types.Named)
	if !ok {
		return nil, "", false
	}
	st, ok = named.Underlying().(*types.Struct)
	if !ok {
		return nil, "", false
	}
	return st, named.Obj().Name(), true
}

// unwrapCallFun strips parenthesized expressions from a call's function target.
// (vec2)(x, y) -> vec2(x, y)
func unwrapCallFun(fun ast.Expr) ast.Expr {
	for {
		if p, ok := fun.(*ast.ParenExpr); ok {
			fun = p.X
		} else {
			break
		}
	}
	return fun
}
