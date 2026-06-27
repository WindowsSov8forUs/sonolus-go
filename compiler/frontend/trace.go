package frontend

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

	"github.com/WindowsSov8forUs/sonolus-go/compiler/ir"
)

// Binding is a named location accessible as a bare identifier in a callback: an
// archetype field or a runtime accessor. Writable bindings may be assigned.
type Binding struct {
	Block    int
	Index    int
	Writable bool
}

// Env is the compilation environment for a callback. Names maps bare
// identifiers (archetype fields, runtime accessors) to their locations. When
// Receiver is set (a callback authored as a method), accesses of the form
// Receiver.Field also resolve against Names. Funcs holds user-defined helper
// functions callable from the body (inlined when called); Accessors is the base
// binding set those inlined functions see (no archetype fields).
type Env struct {
	Names     map[string]Binding
	Receiver  string
	Funcs     map[string]*ast.FuncDecl // free helper functions
	Methods   map[string]*ast.FuncDecl // non-callback methods of the current archetype
	Accessors map[string]Binding
	Mode      ir.Mode
	Records   map[string][]string // user-defined record: name → field names
}

// loopCtx records the jump targets for break/continue inside a loop.
type loopCtx struct {
	breakTo    *ir.BasicBlock
	continueTo *ir.BasicBlock
}

// returnCtx records how a `return` is handled. For a callback (target==nil) a
// value return becomes Break(value, 1) on the enclosing JumpLoop. For an inlined
// function, the value is written to temp and control branches to target (the
// call's continuation block). compositeFields is set to the field names when the
// function returns a composite value.
type returnCtx struct {
	temp            *ir.TempBlock
	target          *ir.BasicBlock
	compositeFields []string
}

// tracer builds a CFG by interpreting a function body.
//
// Local variables are memory-backed by a TempBlock each, so assignments under
// control flow (if-branches, loops) merge correctly through memory without
// SSA/phi. AllocateTempBlocks later assigns concrete cells. The output is
// intentionally memory-heavy; constant/copy propagation and dead-store
// elimination are the optimizer's job.
type tracer struct {
	fset       *token.FileSet
	entry      *ir.BasicBlock
	current    *ir.BasicBlock
	terminated bool // current block already ended in break/continue (unreachable tail)
	env        Env
	vars       map[string]*ir.TempBlock
	arrays     map[string]*arrayInfo
	records    map[string]*recordInfo
	loops      []loopCtx
	returns    []returnCtx
	inlining   map[string]bool
}

// arrayInfo is a fixed-size scalar array local, backed by a multi-slot temp.
type arrayInfo struct {
	tb    *ir.TempBlock
	count int
}

// recordInfo is a record local with named scalar fields. Backed by a multi-slot
// temp for storage, but each field also tracked as an individual Num so reads
// can be constant-folded or SSA-folded without a memory read.
type recordInfo struct {
	tb     *ir.TempBlock
	fields map[string]int
	order  []string
	val    Num // composite Num for scalar-replaceable field reads
}

// vec2Fields is the field layout of the built-in Vec2 record.
var vec2Fields = []string{"x", "y"}

// quadFields is the field layout of the built-in Quad record.
var quadFields = []string{"blx", "bly", "tlx", "tly", "trx", "try", "brx", "bry"}

// enter makes b the current block and marks it reachable.
func (t *tracer) enter(b *ir.BasicBlock) {
	t.current = b
	t.terminated = false
}

// fallthroughTo adds an unconditional edge from the current block to b, unless
// the current block already terminated (break/continue), in which case the edge
// would be unreachable and is skipped.
func (t *tracer) fallthroughTo(b *ir.BasicBlock) {
	if !t.terminated {
		t.current.ConnectTo(b, nil)
	}
}

// Compile parses a Go source file and traces the FIRST function body into a CFG.
// All other functions in the file are collected as helpers in env.Funcs.
func Compile(src string, env Env) (*ir.BasicBlock, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "engine.go", src, 0)
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}

	if env.Funcs == nil {
		env.Funcs = map[string]*ast.FuncDecl{}
	}
	var fn *ast.FuncDecl
	for _, d := range file.Decls {
		if f, ok := d.(*ast.FuncDecl); ok && f.Body != nil {
			if fn == nil {
				fn = f
			} else {
				env.Funcs[f.Name.Name] = f
			}
		}
	}
	if fn == nil {
		return nil, fmt.Errorf("no function with a body found")
	}

	return CompileBlock(fset, fn.Body, env)
}

// CompileBlock traces an already-parsed function body into a CFG. It is used to
// compile callback methods directly from a parsed engine file (the body keeps
// its original positions in fset for error messages).
func CompileBlock(fset *token.FileSet, body *ast.BlockStmt, env Env) (*ir.BasicBlock, error) {
	t := &tracer{
		fset:     fset,
		env:      env,
		vars:     map[string]*ir.TempBlock{},
		arrays:   map[string]*arrayInfo{},
		records:  map[string]*recordInfo{},
		inlining: map[string]bool{},
	}
	t.entry = ir.NewBlock()
	t.current = t.entry
	// Callback-level return context: a value return becomes Break on the
	// callback's JumpLoop (target == nil).
	t.returns = append(t.returns, returnCtx{})
	if err := t.stmtList(body.List); err != nil {
		return nil, err
	}
	return t.entry, nil
}

func (t *tracer) errf(node ast.Node, format string, args ...any) error {
	pos := t.fset.Position(node.Pos())
	return fmt.Errorf("%d:%d: %s", pos.Line, pos.Column, fmt.Sprintf(format, args...))
}

// cell returns the place for a local's temp block.
func (t *tracer) cell(tb *ir.TempBlock) ir.BlockPlace { return ir.TempCell(tb) }

// emit appends a statement to the current block.
func (t *tracer) emit(stmt ir.Node) { t.current.Statements = append(t.current.Statements, stmt) }

func (t *tracer) stmtList(list []ast.Stmt) error {
	for _, s := range list {
		if t.terminated {
			// Remaining statements are unreachable (after break/continue).
			break
		}
		if err := t.stmt(s); err != nil {
			return err
		}
	}
	return nil
}

func (t *tracer) stmt(s ast.Stmt) error {
	switch n := s.(type) {
	case *ast.AssignStmt:
		return t.assign(n)
	case *ast.IncDecStmt:
		return t.incDec(n)
	case *ast.ExprStmt:
		_, err := t.expr(n.X)
		return err
	case *ast.IfStmt:
		return t.ifStmt(n)
	case *ast.SwitchStmt:
		return t.switchStmt(n)
	case *ast.ForStmt:
		return t.forStmt(n)
	case *ast.RangeStmt:
		return t.rangeStmt(n)
	case *ast.BranchStmt:
		return t.branch(n)
	case *ast.ReturnStmt:
		return t.returnStmt(n)
	case *ast.BlockStmt:
		return t.stmtList(n.List)
	case *ast.EmptyStmt:
		return nil
	default:
		return t.errf(s, "unsupported statement %T", s)
	}
}

var compoundOps = map[token.Token]token.Token{
	token.ADD_ASSIGN: token.ADD, token.SUB_ASSIGN: token.SUB,
	token.MUL_ASSIGN: token.MUL, token.QUO_ASSIGN: token.QUO,
	token.REM_ASSIGN: token.REM,
}

func (t *tracer) assign(n *ast.AssignStmt) error {
	// Compound assignment: x += y → x = x + y
	if nt := n.Tok; nt != token.ASSIGN && nt != token.DEFINE {
		if binOp, ok := compoundOps[nt]; ok {
			return t.compoundAssign(n, binOp)
		}
		return t.errf(n, "unsupported assignment %s", n.Tok)
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

	fnName, ok := n.Lhs[0].(*ast.Ident)
	if !ok {
		return t.errf(n, "assignment target must be an identifier")
	}

	// Field write: `=` to an env binding not shadowed by a local. (`:=` always
	// declares a fresh local, shadowing any binding.)
	if _, isLocal := t.vars[fnName.Name]; !isLocal && n.Tok == token.ASSIGN {
		if b, ok := t.env.Names[fnName.Name]; ok {
			if !b.Writable {
				return t.errf(n, "cannot assign to read-only %q", fnName.Name)
			}
			val, err := t.expr(n.Rhs[0])
			if err != nil {
				return err
			}
			if b.Block < 0 { // exported field: emit ExportValue(index, value)
				t.emit(ir.ImpureInstr(resource.RuntimeFunctionExportValue, ir.Const(b.Index), val.node()))
			} else {
				t.emit(ir.SetPlace(ir.Cell(b.Block, b.Index), val.node()))
			}
			return nil
		}
	}

	// Composite declarations: a := array(n) / v := vec2(x, y).
	if call, ok := n.Rhs[0].(*ast.CallExpr); ok {
		if fn, ok := call.Fun.(*ast.Ident); ok {
			switch fn.Name {
			case "array":
				return t.arrayDecl(fnName, call)
			case "vec2":
				return t.recordDecl(fnName, call, vec2Fields)
			case "quad":
				return t.recordDecl(fnName, call, quadFields)
			case "mat":
				return t.recordDecl(fnName, call, matFields)
			case "rect":
				return t.recordDecl(fnName, call, rectFields)
			case "trans":
				return t.recordDecl(fnName, call, transFields)
			}
		}
	}

	val, err := t.expr(n.Rhs[0])
	if err != nil {
		return err
	}

	// Composite value from a function return or constructor: register as a record.
	if n.Tok == token.DEFINE && val.IsComposite() {
		rec := &recordInfo{
			tb:     &ir.TempBlock{Name: fnName.Name, Size: val.CompositeSize()},
			fields: map[string]int{},
			order:  CompositeFieldOrder(&val),
			val:    val,
		}
		for i, f := range rec.order {
			rec.fields[f] = i
		}
		t.records[fnName.Name] = rec
		return nil
	}

	tb, ok := t.vars[fnName.Name]
	if !ok {
		if n.Tok != token.DEFINE {
			return t.errf(n, "assignment to undefined variable %q", fnName.Name)
		}
		tb = t.alloc(fnName.Name)
	}
	t.emit(ir.SetPlace(t.cell(tb), val.node()))
	return nil
}

func (t *tracer) incDec(n *ast.IncDecStmt) error {
	fnName, ok := n.X.(*ast.Ident)
	if !ok {
		return t.errf(n, "increment target must be an identifier")
	}
	tb, ok := t.vars[fnName.Name]
	if !ok {
		return t.errf(n, "increment of undefined variable %q", fnName.Name)
	}
	op := binOps[token.ADD]
	if n.Tok == token.DEC {
		op = binOps[token.SUB]
	}
	cur := ir.GetPlace(t.cell(tb))
	t.emit(ir.SetPlace(t.cell(tb), ir.PureInstr(op, cur, ir.Const(1))))
	return nil
}

func (t *tracer) alloc(fnName string) *ir.TempBlock {
	tb := ir.NewTemp(fnName)
	t.vars[fnName] = tb
	return tb
}

// arrayDecl handles `fnName := array(count)`: it reserves a multi-slot temp.
func (t *tracer) arrayDecl(fnName *ast.Ident, call *ast.CallExpr) error {
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
	t.arrays[fnName.Name] = &arrayInfo{tb: &ir.TempBlock{Name: fnName.Name, Size: int(size.c)}, count: int(size.c)}
	return nil
}

// arrayElemPlace builds the place for a[index].
func (t *tracer) arrayElemPlace(fnName *ast.Ident, indexExpr ast.Expr) (ir.BlockPlace, error) {
	arr, ok := t.arrays[fnName.Name]
	if !ok {
		return ir.BlockPlace{}, t.errf(fnName, "undefined array %q", fnName.Name)
	}
	index, err := t.expr(indexExpr)
	if err != nil {
		return ir.BlockPlace{}, err
	}
	return ir.BlockPlace{Block: arr.tb, Index: index.node(), Offset: 0}, nil
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
		// Array element read
		id, ok := l.X.(*ast.Ident)
		if !ok {
			return t.errf(n, "only named array variables supported for += etc.")
		}
		arr, ok := t.arrays[id.Name]
		if !ok {
			return t.errf(n, "undefined array %q", id.Name)
		}
		idx, err := t.expr(l.Index)
		if err != nil {
			return err
		}
		cur = exprNum(ir.GetPlace(ir.BlockPlace{Block: arr.tb, Index: idx.node(), Offset: 0}))
	default:
		return t.errf(n, "unsupported compound assign target %T", lhs)
	}

	result, ok := applyBinary(binOp, cur, rhs)
	if !ok {
		return t.errf(n, "unsupported compound operation")
	}
	return t.writePlace(lhs, result)
}

func (t *tracer) writePlace(lhs ast.Expr, val Num) error {
	switch l := lhs.(type) {
	case *ast.Ident:
		if tb, ok := t.vars[l.Name]; ok {
			t.emit(ir.SetPlace(t.cell(tb), val.node()))
			return nil
		}
		if b, ok := t.env.Names[l.Name]; ok {
			if !b.Writable {
				return t.errf(lhs, "cannot write %q: read-only", l.Name)
			}
			if b.Block < 0 {
				t.emit(ir.ImpureInstr(resource.RuntimeFunctionExportValue, ir.Const(b.Index), val.node()))
			} else {
				t.emit(ir.SetPlace(ir.Cell(b.Block, b.Index), val.node()))
			}
			return nil
		}
	case *ast.SelectorExpr:
		place, err := t.fieldPlace(l)
		if err != nil {
			return err
		}
		t.emit(ir.SetPlace(place, val.node()))
		if base, ok2 := l.X.(*ast.Ident); ok2 {
			if rec, ok3 := t.records[base.Name]; ok3 {
				rec.val.SetField(l.Sel.Name, val)
			}
		}
		return nil
	case *ast.IndexExpr:
		return t.errf(lhs, "compound assign to array elements is not yet supported")
	}
	return t.errf(lhs, "cannot write compound assign to %T", lhs)
}

func (t *tracer) arrayStore(idx *ast.IndexExpr, rhs ast.Expr) error {
	fnName, ok := idx.X.(*ast.Ident)
	if !ok {
		return t.errf(idx, "array index target must be an identifier")
	}
	place, err := t.arrayElemPlace(fnName, idx.Index)
	if err != nil {
		return err
	}
	val, err := t.expr(rhs)
	if err != nil {
		return err
	}
	t.emit(ir.SetPlace(place, val.node()))
	return nil
}

// recordDecl handles `fnName := vec2(x, y)` (and future record constructors): it
// reserves a temp with one slot per field, stores the initializers, and tracks
// each field as an individual Num for scalar-replaceable reads.
func (t *tracer) recordDecl(fnName *ast.Ident, call *ast.CallExpr, fields []string) error {
	if len(call.Args) != len(fields) {
		return t.errf(call, "%s expects %d arguments", call.Fun.(*ast.Ident).Name, len(fields))
	}
	rec := &recordInfo{
		tb:     &ir.TempBlock{Name: fnName.Name, Size: len(fields)},
		fields: map[string]int{},
		order:  fields,
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
		t.emit(ir.SetPlace(ir.BlockPlace{Block: rec.tb, Index: ir.Const(i), Offset: 0}, val.node()))
		fieldVals[fields[i]] = val
	}
	rec.val = compNum(fieldVals)
	t.records[fnName.Name] = rec
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
		// Check if this is a bare composite Num in a local variable.
		if tb, ok2 := t.vars[base.Name]; ok2 {
			_ = tb
			return Num{}, t.errf(sel, "scalar variable %q has no fields", base.Name)
		}
		return Num{}, t.errf(sel, "undefined record %q", base.Name)
	}
	off, ok := rec.fields[sel.Sel.Name]
	if !ok {
		return Num{}, t.errf(sel, "record %q has no field %q", base.Name, sel.Sel.Name)
	}
	// Return the tracked field Num if it's a constant or pure expression; fall
	// back to a memory read for mutable-backed fields.
	if v, ok := rec.val.fields[sel.Sel.Name]; ok && (v.isConst || v.e != nil) {
		return v, nil
	}
	return exprNum(ir.GetPlace(ir.BlockPlace{Block: rec.tb, Index: ir.Const(off), Offset: 0})), nil
}

// fieldPlace builds the place for v.field (used for writes).
func (t *tracer) fieldPlace(sel *ast.SelectorExpr) (ir.BlockPlace, error) {
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
		return ir.BlockPlace{}, t.errf(sel, "record %q has no field %q", base.Name, sel.Sel.Name)
	}
	return ir.BlockPlace{Block: rec.tb, Index: ir.Const(off), Offset: 0}, nil
}

func (t *tracer) fieldStore(sel *ast.SelectorExpr, rhs ast.Expr) error {
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
		if b.Block < 0 {
			t.emit(ir.ImpureInstr(resource.RuntimeFunctionExportValue, ir.Const(b.Index), val.node()))
		} else {
			t.emit(ir.SetPlace(ir.Cell(b.Block, b.Index), val.node()))
		}
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
	t.emit(ir.SetPlace(place, val.node()))
	// Update the tracked composite Num so subsequent reads fold the new value.
	if base, ok := sel.X.(*ast.Ident); ok {
		if rec, ok2 := t.records[base.Name]; ok2 {
			rec.val.SetField(sel.Sel.Name, val)
		}
	}
	return nil
}

func (t *tracer) shortCircuitIf(n *ast.IfStmt, bin *ast.BinaryExpr) error {
	thenBlock := ir.NewBlock()
	merge := ir.NewBlock()
	var elseBlock *ir.BasicBlock
	if n.Else != nil {
		elseBlock = ir.NewBlock()
	}
	falseTarget := merge
	if elseBlock != nil {
		falseTarget = elseBlock
	}

	if bin.Op == token.LAND {
		// Left must be truthy, THEN evaluate right.
		left, err := t.expr(bin.X)
		if err != nil {
			return err
		}
		if left.isConst {
			if left.c == 0 {
				// Left is constant false: short-circuit, skip to else/merge.
				if elseBlock != nil {
					return t.stmtList(n.Else.(*ast.BlockStmt).List)
				}
				return nil
			}
			// Left is constant true: just evaluate right as the condition.
			right, err := t.expr(bin.Y)
			if err != nil {
				return err
			}
			t.current.Test = right.node()
			if elseBlock != nil {
				t.current.ConnectTo(falseTarget, ir.Cond(0))
			} else {
				t.current.ConnectTo(merge, ir.Cond(0))
			}
			t.current.ConnectTo(thenBlock, nil)
		} else {
			t.current.Test = left.node()
			rightBlock := ir.NewBlock()
			t.current.ConnectTo(falseTarget, ir.Cond(0))
			t.current.ConnectTo(rightBlock, nil)

			t.enter(rightBlock)
			right, err := t.expr(bin.Y)
			if err != nil {
				return err
			}
			rightBlock.Test = right.node()
			rightBlock.ConnectTo(falseTarget, ir.Cond(0))
			rightBlock.ConnectTo(thenBlock, nil)
		}
	} else {
		// `||`: left truthy → thenBlock; else → evaluate right.
		left, err := t.expr(bin.X)
		if err != nil {
			return err
		}
		if left.isConst {
			if left.c != 0 {
				t.stmtList(n.Body.List)
				t.fallthroughTo(merge)
				t.enter(merge)
				return nil
			}
			right, err := t.expr(bin.Y)
			if err != nil {
				return err
			}
			t.current.Test = right.node()
			if elseBlock != nil {
				t.current.ConnectTo(falseTarget, ir.Cond(0))
			} else {
				t.current.ConnectTo(merge, ir.Cond(0))
			}
			t.current.ConnectTo(thenBlock, nil)
		} else {
			t.current.Test = left.node()
			rightBlock := ir.NewBlock()
			t.current.ConnectTo(thenBlock, nil)         // true → thenBlock
			t.current.ConnectTo(rightBlock, ir.Cond(0)) // false → evaluate right

			t.enter(rightBlock)
			right, err := t.expr(bin.Y)
			if err != nil {
				return err
			}
			rightBlock.Test = right.node()
			rightBlock.ConnectTo(falseTarget, ir.Cond(0))
			rightBlock.ConnectTo(thenBlock, nil)
		}
	}

	// then branch
	t.enter(thenBlock)
	if err := t.stmtList(n.Body.List); err != nil {
		return err
	}
	t.fallthroughTo(merge)

	// else branch
	if elseBlock != nil {
		t.enter(elseBlock)
		switch e := n.Else.(type) {
		case *ast.BlockStmt:
			if err := t.stmtList(e.List); err != nil {
				return err
			}
		case *ast.IfStmt:
			if err := t.ifStmt(e); err != nil {
				return err
			}
		default:
			return t.errf(n.Else, "unsupported else %T", n.Else)
		}
		t.fallthroughTo(merge)
	}

	t.enter(merge)
	t.terminated = len(merge.Incoming) == 0
	return nil
}

func (t *tracer) rangeStmt(n *ast.RangeStmt) error {
	// Lower for-range over a count/collection into a standard integer for-loop:
	//   for i := range counts → for i := 0; i < counts; i++ { ... }
	if n.Key == nil {
		return t.errf(n, "range statement requires a key variable")
	}
	if n.Value != nil {
		return t.errf(n, "range value variable is not supported yet")
	}
	keyName, ok := n.Key.(*ast.Ident)
	if !ok {
		return t.errf(n, "range key must be an identifier")
	}
	if n.Tok != token.DEFINE && n.Tok != token.ASSIGN {
		return t.errf(n, "range requires := or =")
	}

	// Evaluate the bound expression.
	bound, err := t.expr(n.X)
	if err != nil {
		return err
	}

	// Declare loop variable i := 0
	iTB := ir.NewTemp("range")
	t.vars[keyName.Name] = iTB
	t.emit(ir.SetPlace(t.cell(iTB), ir.Const(0)))

	loopHead := ir.NewBlock()
	loopBody := ir.NewBlock()
	merge := ir.NewBlock()

	t.current.ConnectTo(loopHead, nil)
	t.enter(loopHead)

	// Test: i < bound
	loopHead.Test = ir.PureInstr("Less", ir.GetPlace(t.cell(iTB)), bound.node())
	loopHead.ConnectTo(merge, ir.Cond(0)) // false → exit
	loopHead.ConnectTo(loopBody, nil)     // true → body

	// Loop body
	t.enter(loopBody)
	t.loops = append(t.loops, loopCtx{breakTo: merge, continueTo: loopHead})
	if err := t.stmtList(n.Body.List); err != nil {
		return err
	}
	t.loops = t.loops[:len(t.loops)-1]

	// Increment i++
	t.emit(ir.SetPlace(t.cell(iTB),
		ir.PureInstr("Add", ir.GetPlace(t.cell(iTB)), ir.Const(1))))
	t.fallthroughTo(loopHead)

	t.enter(merge)
	t.terminated = len(merge.Incoming) == 0

	// Clean up loop variable
	delete(t.vars, keyName.Name)
	return nil
}

func (t *tracer) switchStmt(n *ast.SwitchStmt) error {
	if n.Init != nil {
		return t.errf(n, "switch init statement is not supported yet")
	}

	// Evaluate the tag expression (or use true for switch{} with no tag).
	var tag Num
	if n.Tag != nil {
		v, err := t.expr(n.Tag)
		if err != nil {
			return err
		}
		tag = v
	} else {

	}

	merge := ir.NewBlock()

	for _, clause := range n.Body.List {
		cc, ok := clause.(*ast.CaseClause)
		if !ok {
			continue
		}

		if cc.List == nil {
			// default case.
			t.enter(ir.NewBlock())
			if err := t.stmtList(cc.Body); err != nil {
				return err
			}
			t.fallthroughTo(merge)
			continue
		}

		// Build condition: if tagged, `tag == val` for each val; if untagged, use case expr directly.
		var cond Num
		for j, expr := range cc.List {
			cv, err := t.expr(expr)
			if err != nil {
				return err
			}
			if j == 0 {
				cond = cv
			} else {
				cond, _ = applyBinary(token.LOR, cond, cv)
			}
		}

		if n.Tag != nil {
			// Tagged: need `tag == caseVal` for each val.
			var eq Num
			for j, expr := range cc.List {
				cv, err := t.expr(expr)
				if err != nil {
					return err
				}
				eq2, _ := applyBinary(token.EQL, tag, cv)
				if j == 0 {
					eq = eq2
				} else {
					eq, _ = applyBinary(token.LOR, eq, eq2)
				}
			}
			cond = eq
		}

		caseBlock := ir.NewBlock()

		if cond.isConst {
			if cond.c != 0 {
				// This case is always true: execute body and skip everything after.
				t.enter(caseBlock)
				if err := t.stmtList(cc.Body); err != nil {
					return err
				}
				t.fallthroughTo(merge)
				break
			}
			// Constant false: skip this case entirely.
			continue
		}

		// Non-constant: generate Branch.
		nextBlock := ir.NewBlock()
		t.current.Test = cond.node()
		t.current.ConnectTo(nextBlock, ir.Cond(0)) // false → next case
		t.current.ConnectTo(caseBlock, nil)        // true → this case

		t.enter(caseBlock)
		if err := t.stmtList(cc.Body); err != nil {
			return err
		}
		t.fallthroughTo(merge)

		t.enter(nextBlock)
	}

	t.enter(merge)
	return nil
}

func (t *tracer) ifStmt(n *ast.IfStmt) error {
	if n.Init != nil {
		return t.errf(n, "if init statement is not supported yet")
	}
	// Short-circuit logical operators: generate CFG branches instead of a single
	// expression node. `a && b` → test a, true→test b, false→falseTarget.
	// `a || b` → test a, true→thenBlock, false→test b.
	if bin, ok := n.Cond.(*ast.BinaryExpr); ok && (bin.Op == token.LAND || bin.Op == token.LOR) {
		return t.shortCircuitIf(n, bin)
	}

	cond, err := t.expr(n.Cond)
	if err != nil {
		return err
	}

	// A compile-time-constant condition is resolved here: only the taken branch
	// is traced (mirrors sonolus.py visit_If's _is_py_ handling). This avoids
	// emitting — or even tracing — the dead branch.
	if cond.isConst {
		if cond.c != 0 {
			return t.stmtList(n.Body.List)
		}
		switch e := n.Else.(type) {
		case nil:
			return nil
		case *ast.BlockStmt:
			return t.stmtList(e.List)
		case *ast.IfStmt:
			return t.ifStmt(e)
		default:
			return t.errf(n.Else, "unsupported else %T", n.Else)
		}
	}

	condBlock := t.current
	condBlock.Test = cond.node()

	thenBlock := ir.NewBlock()
	merge := ir.NewBlock()
	var elseBlock *ir.BasicBlock
	if n.Else != nil {
		elseBlock = ir.NewBlock()
	}

	falseTarget := merge
	if elseBlock != nil {
		falseTarget = elseBlock
	}
	condBlock.ConnectTo(falseTarget, ir.Cond(0)) // false branch
	condBlock.ConnectTo(thenBlock, nil)          // true branch

	t.enter(thenBlock)
	if err := t.stmtList(n.Body.List); err != nil {
		return err
	}
	t.fallthroughTo(merge)

	if elseBlock != nil {
		t.enter(elseBlock)
		switch e := n.Else.(type) {
		case *ast.BlockStmt:
			if err := t.stmtList(e.List); err != nil {
				return err
			}
		case *ast.IfStmt:
			if err := t.ifStmt(e); err != nil {
				return err
			}
		default:
			return t.errf(n.Else, "unsupported else %T", n.Else)
		}
		t.fallthroughTo(merge)
	}

	// If neither branch reaches the merge, code after the if is unreachable.
	t.enter(merge)
	t.terminated = len(merge.Incoming) == 0
	return nil
}

// forStmt lowers Go's for in its three shapes: for{}, for cond {}, and
// for init; cond; post {}. break/continue jump to the loop exit / latch.
func (t *tracer) forStmt(n *ast.ForStmt) error {
	if n.Init != nil {
		if err := t.stmt(n.Init); err != nil {
			return err
		}
	}

	header := ir.NewBlock()
	body := ir.NewBlock()
	exit := ir.NewBlock()

	// The latch is where the back edge and `continue` go; with a post statement
	// it is a distinct block that runs post then loops to the header.
	latch := header
	if n.Post != nil {
		latch = ir.NewBlock()
	}

	t.fallthroughTo(header)
	t.enter(header)
	if n.Cond != nil {
		cond, err := t.expr(n.Cond)
		if err != nil {
			return err
		}
		header.Test = cond.node()
		header.ConnectTo(exit, ir.Cond(0)) // false -> exit
		header.ConnectTo(body, nil)        // true  -> body
	} else {
		header.ConnectTo(body, nil)
	}

	t.loops = append(t.loops, loopCtx{breakTo: exit, continueTo: latch})
	t.enter(body)
	if err := t.stmtList(n.Body.List); err != nil {
		return err
	}
	t.fallthroughTo(latch)
	t.loops = t.loops[:len(t.loops)-1]

	if n.Post != nil {
		t.enter(latch)
		if err := t.stmt(n.Post); err != nil {
			return err
		}
		t.current.ConnectTo(header, nil)
	}

	// If nothing breaks out of the loop, code after it is unreachable.
	t.enter(exit)
	t.terminated = len(exit.Incoming) == 0
	return nil
}

func (t *tracer) returnStmt(n *ast.ReturnStmt) error {
	if len(t.returns) == 0 {
		return t.errf(n, "return outside of a function")
	}
	rc := t.returns[len(t.returns)-1]

	var val *Num
	if len(n.Results) == 1 {
		v, err := t.expr(n.Results[0])
		if err != nil {
			return err
		}
		val = &v
	} else if len(n.Results) > 1 {
		return t.errf(n, "multiple return values are not supported")
	}

	if rc.target == nil {
		// Callback: a value return breaks the JumpLoop yielding the value.
		// Composite returns are not supported at the callback level (callbacks
		// return a single float via Break).
		if val != nil {
			if val.IsComposite() {
				return t.errf(n, "cannot return a composite value from a callback; return individual fields instead")
			}
			t.emit(ir.ImpureInstr(resource.RuntimeFunctionBreak, val.node(), ir.Const(1)))
		}
	} else {
		// Inlined function: write the return value to the ret temp.
		if val != nil {
			if val.IsComposite() {
				rc.compositeFields = CompositeFieldOrder(val)
				for i, f := range rc.compositeFields {
					t.emit(ir.SetPlace(ir.BlockPlace{Block: rc.temp, Index: ir.Const(i), Offset: 0}, val.Field(f).node()))
				}
			} else {
				t.emit(ir.SetPlace(ir.TempCell(rc.temp), val.node()))
			}
		}
		t.current.ConnectTo(rc.target, nil)
	}
	t.terminated = true
	return nil
}

func (t *tracer) branch(n *ast.BranchStmt) error {
	if len(t.loops) == 0 {
		return t.errf(n, "%s outside of a loop", n.Tok)
	}
	loop := t.loops[len(t.loops)-1]
	switch n.Tok {
	case token.BREAK:
		t.current.ConnectTo(loop.breakTo, nil)
	case token.CONTINUE:
		t.current.ConnectTo(loop.continueTo, nil)
	default:
		return t.errf(n, "unsupported branch statement %s", n.Tok)
	}
	// The rest of this block is unreachable; stmtList stops here.
	t.terminated = true
	return nil
}

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
		res, ok := applyUnary(n.Op, x)
		if !ok {
			return Num{}, t.errf(n, "unsupported unary operator %s", n.Op)
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
		res, ok := applyBinary(n.Op, x, y)
		if !ok {
			return Num{}, t.errf(n, "unsupported binary operator %s", n.Op)
		}
		return res, nil
	case *ast.IndexExpr:
		fnName, ok := n.X.(*ast.Ident)
		if !ok {
			return Num{}, t.errf(n, "array index target must be an identifier")
		}
		place, err := t.arrayElemPlace(fnName, n.Index)
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
		// Bare composite: evaluate vec2(...).x → extract field from the composite.
		if _, isCall := n.X.(*ast.CallExpr); isCall {
			v, err := t.expr(n.X)
			if err != nil {
				return Num{}, err
			}
			if v.IsComposite() {
				return v.Field(n.Sel.Name), nil
			}
		}
		return t.fieldValue(n)
	case *ast.CallExpr:
		return t.call(n)
	default:
		return Num{}, t.errf(e, "unsupported expression %T", e)
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
		return Num{}, t.errf(n, "unsupported literal %s", n.Kind)
	}
}

func (t *tracer) ident(n *ast.Ident) (Num, error) {
	switch n.Name {
	case "true":
		return constNum(1), nil
	case "false":
		return constNum(0), nil
	}
	if tb, ok := t.vars[n.Name]; ok {
		return exprNum(ir.GetPlace(t.cell(tb))), nil
	}
	// Environment bindings: archetype fields, runtime accessors (a user variable
	// of the same fnName shadows these).
	if b, ok := t.env.Names[n.Name]; ok {
		return exprNum(ir.GetPlace(ir.Cell(b.Block, b.Index))), nil
	}
	return Num{}, t.errf(n, "undefined identifier %q", n.Name)
}

// call handles the memory builtins get(block, index) and set(block, index, value).
func (t *tracer) call(n *ast.CallExpr) (Num, error) {
	// Method helper call: receiver.Method(args).
	if sel, ok := n.Fun.(*ast.SelectorExpr); ok {
		return t.methodCall(n, sel)
	}
	fn, ok := n.Fun.(*ast.Ident)
	if !ok {
		return Num{}, t.errf(n, "unsupported call target")
	}

	// len(array) and array(n) take an array operand, not a numeric one.
	switch fn.Name {
	case "len":
		if len(n.Args) == 1 {
			if id, ok := n.Args[0].(*ast.Ident); ok {
				if arr, ok := t.arrays[id.Name]; ok {
					return constNum(float64(arr.count)), nil
				}
			}
		}
		return Num{}, t.errf(n, "len expects an array")
	case "array":
		return Num{}, t.errf(n, "array() may only appear in a declaration (a := array(n))")
	case "screen":
		return t.screenFunc(n)
	case "safeArea":
		return t.safeAreaFunc(n)
	case "offsetAdjustedTime":
		return t.offsetAdjustedTimeFunc(n)
	case "prevTime":
		return t.prevTimeFunc(n)
	case "isPlay":
		return boolNum(t.env.Mode == ir.ModePlay), nil
	case "isWatch":
		return boolNum(t.env.Mode == ir.ModeWatch), nil
	case "isPreview":
		return boolNum(t.env.Mode == ir.ModePreview), nil
	case "isTutorial":
		return boolNum(t.env.Mode == ir.ModeTutorial), nil
	case "vec2":
		return t.inlineComposite(fn, n, vec2Fields)
	case "vec2Zero":
		return vec2Statics["zero"](), nil
	case "vec2One":
		return vec2Statics["one"](), nil
	case "vec2Up":
		return vec2Statics["up"](), nil
	case "vec2Down":
		return vec2Statics["down"](), nil
	case "vec2Left":
		return vec2Statics["left"](), nil
	case "vec2Right":
		return vec2Statics["right"](), nil
	case "quad":
		return t.inlineComposite(fn, n, quadFields)
	case "mat":
		return t.inlineComposite(fn, n, matFields)
	case "rect":
		return t.inlineComposite(fn, n, rectFields)
	case "trans":
		return t.inlineComposite(fn, n, transFields)
	}

	args := make([]Num, len(n.Args))
	for i, a := range n.Args {
		v, err := t.expr(a)
		if err != nil {
			return Num{}, err
		}
		args[i] = v
	}

	switch fn.Name {
	case "get":
		if len(args) != 2 {
			return Num{}, t.errf(n, "get expects (block, index)")
		}
		place := ir.NewBlockPlace(args[0].node(), args[1].node(), 0)
		return exprNum(ir.GetPlace(place)), nil
	case "touchId":
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
		place := ir.NewBlockPlace(args[0].node(), args[1].node(), 0)
		t.emit(ir.SetPlace(place, args[2].node()))
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
				nodes[i] = a.node()
			}
			if rf.pure {
				return exprNum(ir.PureInstr(rf.op, nodes...)), nil
			}
			t.emit(ir.ImpureInstr(rf.op, nodes...))
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
	child := Env{Names: t.env.Accessors, Accessors: t.env.Accessors, Funcs: t.env.Funcs, Methods: t.env.Methods}
	return t.inlineFunc(fnName, decl, args, child)
}

// methodCall inlines a non-callback method of the current archetype, invoked as
// receiver.Method(args); the method body sees the archetype's fields via its own
// receiver.
func (t *tracer) methodCall(n *ast.CallExpr, sel *ast.SelectorExpr) (Num, error) {
	// Record method call: v.mul(s) where v is a vec2/quad.
	if base, ok := sel.X.(*ast.Ident); ok {
		if rec, ok := t.records[base.Name]; ok {
			if method, ok2 := vec2Methods[sel.Sel.Name]; ok2 {
				args := make([]Num, len(n.Args))
				for i, a := range n.Args {
					v, err := t.expr(a)
					if err != nil {
						return Num{}, err
					}
					args[i] = v
				}
				return method(t, rec.val, args)
			}
			if method, ok2 := matMethods[sel.Sel.Name]; ok2 {
				args := make([]Num, len(n.Args))
				for i, a := range n.Args {
					v, err := t.expr(a)
					if err != nil {
						return Num{}, err
					}
					args[i] = v
				}
				return method(t, rec.val, args)
			}
			if method, ok2 := rectMethods[sel.Sel.Name]; ok2 {
				args := make([]Num, len(n.Args))
				for i, a := range n.Args {
					v, err := t.expr(a)
					if err != nil {
						return Num{}, err
					}
					args[i] = v
				}
				return method(t, rec.val, args)
			}
			if method, ok2 := quadMethods[sel.Sel.Name]; ok2 {
				args := make([]Num, len(n.Args))
				for i, a := range n.Args {
					v, err := t.expr(a)
					if err != nil {
						return Num{}, err
					}
					args[i] = v
				}
				return method(t, rec.val, args)
			}
			if method, ok2 := transMethods[sel.Sel.Name]; ok2 {
				args := make([]Num, len(n.Args))
				for i, a := range n.Args {
					v, err := t.expr(a)
					if err != nil {
						return Num{}, err
					}
					args[i] = v
				}
				return method(t, rec.val, args)
			}
		}
	}

	base, ok := sel.X.(*ast.Ident)
	if !ok || t.env.Receiver == "" || base.Name != t.env.Receiver {
		return Num{}, t.errf(sel, "unsupported method call")
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
	if decl.Recv != nil && len(decl.Recv.List[0].Names) > 0 {
		recv = decl.Recv.List[0].Names[0].Name
	}
	child := Env{Names: t.env.Names, Receiver: recv, Funcs: t.env.Funcs, Methods: t.env.Methods, Accessors: t.env.Accessors}
	return t.inlineFunc(sel, decl, args, child)
}

// inlineFunc inlines a function/method body: it binds parameters, traces the
// body in a fresh scope under childEnv, routes returns to a continuation block,
// and yields the return value.
func (t *tracer) inlineFunc(node ast.Node, decl *ast.FuncDecl, args []Num, childEnv Env) (Num, error) {
	if t.inlining[decl.Name.Name] {
		return Num{}, t.errf(node, "recursive call to %q is not supported", decl.Name.Name)
	}
	params := funcParams(decl)
	if len(args) != len(params) {
		return Num{}, t.errf(node, "%s expects %d arguments, got %d", decl.Name.Name, len(params), len(args))
	}

	retTemp := &ir.TempBlock{Name: decl.Name.Name + ".$ret", Size: 1}
	cont := ir.NewBlock()

	savedVars, savedArrays, savedRecords, savedEnv := t.vars, t.arrays, t.records, t.env
	t.vars = map[string]*ir.TempBlock{}
	t.arrays = map[string]*arrayInfo{}
	t.records = map[string]*recordInfo{}
	t.env = childEnv

	for i, p := range params {
		if args[i].IsComposite() {
			// Bind each composite field as a separate local, plus a composite
			// record in the function's scope. This emulates recordDecl.
			rec := &recordInfo{
				tb:     &ir.TempBlock{Name: p, Size: args[i].CompositeSize()},
				fields: map[string]int{},
				order:  CompositeFieldOrder(&args[i]),
				val:    args[i],
			}
			for j, f := range rec.order {
				rec.fields[f] = j
			}
			t.records[p] = rec
		} else {
			pt := t.alloc(p)
			t.emit(ir.SetPlace(ir.TempCell(pt), args[i].node()))
		}
	}

	t.returns = append(t.returns, returnCtx{temp: retTemp, target: cont})
	t.inlining[decl.Name.Name] = true
	err := t.stmtList(decl.Body.List)
	t.fallthroughTo(cont)
	addRet := t.returns[len(t.returns)-1]
	compositeFields := addRet.compositeFields
	delete(t.inlining, decl.Name.Name)
	t.returns = t.returns[:len(t.returns)-1]

	t.vars, t.arrays, t.records, t.env = savedVars, savedArrays, savedRecords, savedEnv
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

// funcParams flattens a function's parameter names in order.
func funcParams(decl *ast.FuncDecl) []string {
	var out []string
	if decl.Type.Params == nil {
		return out
	}
	for _, field := range decl.Type.Params.List {
		for _, fnName := range field.Names {
			out = append(out, fnName.Name)
		}
	}
	return out
}

func (t *tracer) touchField(n *ast.CallExpr, args []Num, fieldOffset int) (Num, error) {
	if len(args) != 1 {
		return Num{}, t.errf(n, "touch field expects 1 argument (touch index)")
	}
	const touchBlock, touchStride = 1002, 9
	index := ir.PureInstr(resource.RuntimeFunctionAdd,
		ir.PureInstr(resource.RuntimeFunctionMultiply, args[0].node(), ir.Const(touchStride)),
		ir.Const(fieldOffset))
	return exprNum(ir.GetPlace(ir.NewBlockPlace(ir.Const(touchBlock), index, 0))), nil
}

func (t *tracer) screenFunc(n *ast.CallExpr) (Num, error) {
	if len(n.Args) != 0 {
		return Num{}, t.errf(n, "screen() takes no arguments")
	}
	ar := exprNum(ir.GetPlace(ir.Cell(1000, 1)))
	negAr := exprNum(ir.PureInstr(resource.RuntimeFunctionNegate, ar.node()))
	return compNum(map[string]Num{"t": constNum(1), "r": ar, "b": constNum(-1), "l": negAr}), nil
}

func (t *tracer) safeAreaFunc(n *ast.CallExpr) (Num, error) {
	if len(n.Args) != 0 {
		return Num{}, t.errf(n, "safeArea() takes no arguments")
	}
	return compNum(map[string]Num{
		"t": exprNum(ir.GetPlace(ir.Cell(1000, 8))),
		"r": exprNum(ir.GetPlace(ir.Cell(1000, 6))),
		"b": exprNum(ir.GetPlace(ir.Cell(1000, 7))),
		"l": exprNum(ir.GetPlace(ir.Cell(1000, 5))),
	}), nil
}

// pnpolyFunc emits a point-in-quad test: 1 if point is inside the convex quad.
func (t *tracer) pnpolyFunc(n *ast.CallExpr, args []Num) (Num, error) {
	if len(args) != 2 {
		return Num{}, t.errf(n, "pnpoly expects (point, quad)")
	}
	point, quad := args[0], args[1]
	px, py := point.Field("x").node(), point.Field("y").node()

	cross := func(ax, ay, bx, by ir.Node) ir.Node {
		dx := ir.PureInstr(resource.RuntimeFunctionSubtract, bx, ax)
		dy := ir.PureInstr(resource.RuntimeFunctionSubtract, by, ay)
		return ir.PureInstr(resource.RuntimeFunctionSubtract,
			ir.PureInstr(resource.RuntimeFunctionMultiply, dx, ir.PureInstr(resource.RuntimeFunctionSubtract, py, ay)),
			ir.PureInstr(resource.RuntimeFunctionMultiply, dy, ir.PureInstr(resource.RuntimeFunctionSubtract, px, ax)))
	}

	v0 := cross(quad.Field("blx").node(), quad.Field("bly").node(), quad.Field("tlx").node(), quad.Field("tly").node())
	v1 := cross(quad.Field("tlx").node(), quad.Field("tly").node(), quad.Field("trx").node(), quad.Field("try").node())
	v2 := cross(quad.Field("trx").node(), quad.Field("try").node(), quad.Field("brx").node(), quad.Field("bry").node())
	v3 := cross(quad.Field("brx").node(), quad.Field("bry").node(), quad.Field("blx").node(), quad.Field("bly").node())

	inside := ir.PureInstr(resource.RuntimeFunctionAnd,
		ir.PureInstr(resource.RuntimeFunctionAnd,
			ir.PureInstr(resource.RuntimeFunctionGreaterOr, ir.PureInstr(resource.RuntimeFunctionMultiply, v0, v1), ir.Const(0)),
			ir.PureInstr(resource.RuntimeFunctionGreaterOr, ir.PureInstr(resource.RuntimeFunctionMultiply, v1, v2), ir.Const(0))),
		ir.PureInstr(resource.RuntimeFunctionGreaterOr, ir.PureInstr(resource.RuntimeFunctionMultiply, v2, v3), ir.Const(0)))

	return exprNum(inside), nil
}

func (t *tracer) offsetAdjustedTimeFunc(n *ast.CallExpr) (Num, error) {
	if len(n.Args) != 0 {
		return Num{}, t.errf(n, "offsetAdjustedTime() takes no arguments")
	}
	tm := exprNum(ir.GetPlace(ir.Cell(1000, 0)))
	ao := exprNum(ir.GetPlace(ir.Cell(1000, 2)))
	return exprNum(ir.PureInstr(resource.RuntimeFunctionSubtract, tm.node(), ao.node())), nil
}

func (t *tracer) prevTimeFunc(n *ast.CallExpr) (Num, error) {
	if len(n.Args) != 0 {
		return Num{}, t.errf(n, "prevTime() takes no arguments")
	}
	tm := exprNum(ir.GetPlace(ir.Cell(1000, 0)))
	dt := exprNum(ir.GetPlace(ir.Cell(1000, 1)))
	return exprNum(ir.PureInstr(resource.RuntimeFunctionSubtract, tm.node(), dt.node())), nil
}

func (t *tracer) perspectiveApproachFunc(n *ast.CallExpr, args []Num) (Num, error) {
	if len(args) != 2 {
		return Num{}, t.errf(n, "perspectiveApproach expects (x, y)")
	}
	// perspectiveApproach(x, y) = 1 / (1 - x * y)
	denom := ir.PureInstr(resource.RuntimeFunctionSubtract,
		ir.Const(1),
		ir.PureInstr(resource.RuntimeFunctionMultiply, args[0].node(), args[1].node()))
	return exprNum(ir.PureInstr(resource.RuntimeFunctionDivide, ir.Const(1), denom)), nil
}
