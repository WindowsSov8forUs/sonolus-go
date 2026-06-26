package frontend

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"

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
// Receiver.Field also resolve against Names.
type Env struct {
	Names    map[string]Binding
	Receiver string
}

// loopCtx records the jump targets for break/continue inside a loop.
type loopCtx struct {
	breakTo    *ir.BasicBlock
	continueTo *ir.BasicBlock
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
}

// arrayInfo is a fixed-size scalar array local, backed by a multi-slot temp.
type arrayInfo struct {
	tb    *ir.TempBlock
	count int
}

// recordInfo is a record local with named scalar fields at fixed slot offsets,
// backed by a multi-slot temp.
type recordInfo struct {
	tb     *ir.TempBlock
	fields map[string]int
	order  []string // field names in slot order, for construction
}

// vec2Fields is the field layout of the built-in Vec2 record.
var vec2Fields = []string{"x", "y"}

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

// Compile parses a single Go function from src and traces its body into a CFG,
// returning the entry block. src must be a complete file with at least one
// function declaration; the first function's body is compiled. env supplies the
// bare-identifier bindings (archetype fields, runtime accessors) in scope.
func Compile(src string, env Env) (*ir.BasicBlock, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "engine.go", src, 0)
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}

	var fn *ast.FuncDecl
	for _, d := range file.Decls {
		if f, ok := d.(*ast.FuncDecl); ok && f.Body != nil {
			fn = f
			break
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
		fset:    fset,
		env:     env,
		vars:    map[string]*ir.TempBlock{},
		arrays:  map[string]*arrayInfo{},
		records: map[string]*recordInfo{},
	}
	t.entry = ir.NewBlock()
	t.current = t.entry
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
	case *ast.ForStmt:
		return t.forStmt(n)
	case *ast.BranchStmt:
		return t.branch(n)
	case *ast.BlockStmt:
		return t.stmtList(n.List)
	case *ast.EmptyStmt:
		return nil
	default:
		return t.errf(s, "unsupported statement %T", s)
	}
}

func (t *tracer) assign(n *ast.AssignStmt) error {
	if n.Tok != token.ASSIGN && n.Tok != token.DEFINE {
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

	name, ok := n.Lhs[0].(*ast.Ident)
	if !ok {
		return t.errf(n, "assignment target must be an identifier")
	}

	// Field write: `=` to an env binding not shadowed by a local. (`:=` always
	// declares a fresh local, shadowing any binding.)
	if _, isLocal := t.vars[name.Name]; !isLocal && n.Tok == token.ASSIGN {
		if b, ok := t.env.Names[name.Name]; ok {
			if !b.Writable {
				return t.errf(n, "cannot assign to read-only %q", name.Name)
			}
			val, err := t.expr(n.Rhs[0])
			if err != nil {
				return err
			}
			t.emit(ir.SetPlace(ir.Cell(b.Block, b.Index), val.node()))
			return nil
		}
	}

	// Composite declarations: a := array(n) / v := vec2(x, y).
	if call, ok := n.Rhs[0].(*ast.CallExpr); ok {
		if fn, ok := call.Fun.(*ast.Ident); ok {
			switch fn.Name {
			case "array":
				return t.arrayDecl(name, call)
			case "vec2":
				return t.recordDecl(name, call, vec2Fields)
			}
		}
	}

	val, err := t.expr(n.Rhs[0])
	if err != nil {
		return err
	}

	tb, ok := t.vars[name.Name]
	if !ok {
		if n.Tok != token.DEFINE {
			return t.errf(n, "assignment to undefined variable %q", name.Name)
		}
		tb = t.alloc(name.Name)
	}
	t.emit(ir.SetPlace(t.cell(tb), val.node()))
	return nil
}

func (t *tracer) incDec(n *ast.IncDecStmt) error {
	name, ok := n.X.(*ast.Ident)
	if !ok {
		return t.errf(n, "increment target must be an identifier")
	}
	tb, ok := t.vars[name.Name]
	if !ok {
		return t.errf(n, "increment of undefined variable %q", name.Name)
	}
	op := binOps[token.ADD]
	if n.Tok == token.DEC {
		op = binOps[token.SUB]
	}
	cur := ir.GetPlace(t.cell(tb))
	t.emit(ir.SetPlace(t.cell(tb), ir.PureInstr(op, cur, ir.Const(1))))
	return nil
}

func (t *tracer) alloc(name string) *ir.TempBlock {
	tb := ir.NewTemp(name)
	t.vars[name] = tb
	return tb
}

// arrayDecl handles `name := array(count)`: it reserves a multi-slot temp.
func (t *tracer) arrayDecl(name *ast.Ident, call *ast.CallExpr) error {
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
	t.arrays[name.Name] = &arrayInfo{tb: &ir.TempBlock{Name: name.Name, Size: int(size.c)}, count: int(size.c)}
	return nil
}

// arrayElemPlace builds the place for a[index].
func (t *tracer) arrayElemPlace(name *ast.Ident, indexExpr ast.Expr) (ir.BlockPlace, error) {
	arr, ok := t.arrays[name.Name]
	if !ok {
		return ir.BlockPlace{}, t.errf(name, "undefined array %q", name.Name)
	}
	index, err := t.expr(indexExpr)
	if err != nil {
		return ir.BlockPlace{}, err
	}
	return ir.BlockPlace{Block: arr.tb, Index: index.node(), Offset: 0}, nil
}

func (t *tracer) arrayStore(idx *ast.IndexExpr, rhs ast.Expr) error {
	name, ok := idx.X.(*ast.Ident)
	if !ok {
		return t.errf(idx, "array index target must be an identifier")
	}
	place, err := t.arrayElemPlace(name, idx.Index)
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

// recordDecl handles `name := vec2(x, y)` (and future record constructors): it
// reserves a temp with one slot per field and stores the initializers.
func (t *tracer) recordDecl(name *ast.Ident, call *ast.CallExpr, fields []string) error {
	if len(call.Args) != len(fields) {
		return t.errf(call, "%s expects %d arguments", call.Fun.(*ast.Ident).Name, len(fields))
	}
	rec := &recordInfo{
		tb:     &ir.TempBlock{Name: name.Name, Size: len(fields)},
		fields: map[string]int{},
		order:  fields,
	}
	for i, f := range fields {
		rec.fields[f] = i
	}
	t.records[name.Name] = rec
	for i, arg := range call.Args {
		val, err := t.expr(arg)
		if err != nil {
			return err
		}
		t.emit(ir.SetPlace(ir.BlockPlace{Block: rec.tb, Index: ir.Const(i), Offset: 0}, val.node()))
	}
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

// fieldPlace builds the place for v.field.
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
		t.emit(ir.SetPlace(ir.Cell(b.Block, b.Index), val.node()))
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
	return nil
}

func (t *tracer) ifStmt(n *ast.IfStmt) error {
	if n.Init != nil {
		return t.errf(n, "if init statement is not supported yet")
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
		name, ok := n.X.(*ast.Ident)
		if !ok {
			return Num{}, t.errf(n, "array index target must be an identifier")
		}
		place, err := t.arrayElemPlace(name, n.Index)
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
		place, err := t.fieldPlace(n)
		if err != nil {
			return Num{}, err
		}
		return exprNum(ir.GetPlace(place)), nil
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
	// of the same name shadows these).
	if b, ok := t.env.Names[n.Name]; ok {
		return exprNum(ir.GetPlace(ir.Cell(b.Block, b.Index))), nil
	}
	return Num{}, t.errf(n, "undefined identifier %q", n.Name)
}

// call handles the memory builtins get(block, index) and set(block, index, value).
func (t *tracer) call(n *ast.CallExpr) (Num, error) {
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
	case "vec2":
		return Num{}, t.errf(n, "vec2() may only appear in a declaration (v := vec2(x, y))")
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
	case "set":
		if len(args) != 3 {
			return Num{}, t.errf(n, "set expects (block, index, value)")
		}
		place := ir.NewBlockPlace(args[0].node(), args[1].node(), 0)
		t.emit(ir.SetPlace(place, args[2].node()))
		return constNum(0), nil
	default:
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
		return Num{}, t.errf(n, "unknown function %q", fn.Name)
	}
}
