// Package frontend is the frontend compilation stage. It translates Go AST
// (parsed engine source) into the IR via an environment-driven trace: field
// accessors, mode-specific builtin functions, type-driven dispatch (D2),
// and helper method inlining.
package frontend

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/ir"
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
	Names       map[string]Binding
	Receiver    string
	Funcs       map[string]*ast.FuncDecl // free helper functions
	Methods     map[string]*ast.FuncDecl // non-callback methods of the current archetype
	Accessors   map[string]Binding
	Mode        ir.Mode
	Records     map[string][]string // user-defined record: name → field names
	Info        *types.Info         // go/types type-check result (D1 diagnostic layer)
	Constants        map[string]float64        // named compile-time constants (e.g. archetype indices)
	SpriteIndex      map[string]float64        // sprite name → index (from Skin struct fields)
	EffectIndex      map[string]float64        // effect clip name → ID (from Effect struct fields)
	ParticleIndex    map[string]float64        // particle effect name → ID (from Particle struct fields)
	ContainerFields  []ContainerFieldMeta      // container-typed struct field metadata
	MaxUnroll        int                       // general loop unroll limit (0 = default 256)
	MaxUnrollCont    int                       // container unroll limit (0 = default 64)
}

// ContainerFieldMeta mirrors engine.ContainerFieldMeta in the frontend package
// to avoid a circular import. It stores compile-time metadata for a container-typed
// struct field so the tracer can emit container-aware IR.
type ContainerFieldMeta struct {
	Name     string // Go field name
	TypeName string // record type: "varArray", "arrayMap", "arraySet", "frozenNumSet"
	Capacity int    // max element count
	ElemSize int    // slots per element
}

// loopCtx records the jump targets for break/continue inside a loop.
type loopCtx struct {
	breakTo    *ir.BasicBlock
	continueTo *ir.BasicBlock
	label      string // for labeled break/continue, empty if unlabeled
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
// SSA/phi. allocateTempBlocks later assigns concrete cells. The output is
// intentionally memory-heavy; constant/copy propagation and dead-store
// elimination are the optimizer's job.
type tracer struct {
	fset       *token.FileSet
	gen        *ir.IDGen
	entry      *ir.BasicBlock
	current    *ir.BasicBlock
	terminated bool // current block already ended in break/continue (unreachable tail)
	env        Env
	vars       map[string]*ir.TempBlock
	arrays     map[string]*arrayInfo
	records    map[string]*recordInfo
	containers map[string]*containerInfo // VarArray/ArrayMap/ArraySet locals
	loops      []loopCtx
	returns    []returnCtx
	inlining   map[string]bool
	stmtLabel  string // label of the currently-being-traced labeled statement

	// fallthroughTarget is the CFG block created by the most recent
	// fallthrough statement in a switch case body. The next case picks it
	// up as its body block so that execution transfers directly into the
	// next case's body, bypassing the condition check.
	fallthroughTarget *ir.BasicBlock

	maxUnroll     int // general loop unroll limit (default 256)
	maxUnrollCont int // container iteration unroll limit (default 64)

	labels map[string]*ir.BasicBlock // label -> block mapping for goto

	defers   []deferCtx              // stack of defer scopes, one per function level
	closures map[string]*closureInfo // captured closures by synthetic name
}

// deferRecord stores a deferred call with arguments evaluated at defer-time.
type deferRecord struct {
	call *ast.CallExpr
	args []Num
}

// deferCtx holds the deferred calls for a single function scope.
type deferCtx struct {
	records []deferRecord
}

// closureInfo holds a capturing closure that has been lifted to a synthetic helper.
// The capture frame stores the captured variable values at closure creation time.
type closureInfo struct {
	fn       *ast.FuncLit    // the closure body
	captures []string        // captured variable names in order
	frame    *ir.TempBlock   // capture frame (one slot per capture)
}

// arrayInfo is a fixed-size array local, backed by a multi-slot temp.
// elemSize slots per element: 1 for scalars, N for N-field records.
type arrayInfo struct {
	tb       *ir.TempBlock
	count    int
	elemSize int
	elemNum  Num // tracked per-element values for scalar-replaceable reads
}

// recordInfo is a record local with named scalar fields. Backed by a multi-slot
// temp for storage, but each field also tracked as an individual Num so reads
// can be constant-folded or SSA-folded without a memory read.
type recordInfo struct {
	tb       *ir.TempBlock
	fields   map[string]int
	order    []string
	val      Num    // composite Num for scalar-replaceable field reads
	typeName string // record type name ("vec2", "quad", etc.) for method dispatch
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
func Compile(src string, env Env) (*ir.BasicBlock, *ir.IDGen, error) {
	gen := ir.NewIDGen()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "engine.go", src, 0)
	if err != nil {
		return nil, nil, fmt.Errorf("parse: %w", err)
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
		// Validate struct field types: reject map, chan, func, and interface
		// types that the Sonolus engine memory model cannot represent.
		if g, ok := d.(*ast.GenDecl); ok && g.Tok == token.TYPE {
			for _, spec := range g.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				st, ok := ts.Type.(*ast.StructType)
				if !ok {
					continue
				}
				if err := validateStructFields(st); err != nil {
					return nil, nil, fmt.Errorf("struct %s: %w", ts.Name.Name, err)
				}
			}
		}
	}

	// Expand embedded struct fields into parent struct records.
	// When env.Info is available (set by callers that type-check before compiling),
	// resolve each embedded field's underlying struct type and promote its fields
	// into the parent's field list. Named fields always take precedence
	// (shadowing) over promoted fields of the same name.
	if env.Info != nil {
		for _, d := range file.Decls {
			g, ok := d.(*ast.GenDecl)
			if !ok || g.Tok != token.TYPE {
				continue
			}
			for _, spec := range g.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				st, ok := ts.Type.(*ast.StructType)
				if !ok {
					continue
				}
				fullFields := make([]string, 0)
				seen := make(map[string]bool)
				for _, field := range st.Fields.List {
					if len(field.Names) == 1 {
						name := field.Names[0].Name
						fullFields = append(fullFields, name)
						seen[name] = true
					} else if len(field.Names) == 0 {
						// Embedded: expand fields from underlying struct type
						named, ok2 := env.Info.TypeOf(field.Type).(*types.Named)
						if ok2 {
							if embeddedSt, ok3 := named.Underlying().(*types.Struct); ok3 {
								for i := range embeddedSt.NumFields() {
									name := embeddedSt.Field(i).Name()
									if !seen[name] {
										fullFields = append(fullFields, name)
										seen[name] = true
									}
								}
							}
						}
					}
				}
				if len(fullFields) > 0 {
					if env.Records == nil {
						env.Records = make(map[string][]string)
					}
					env.Records[ts.Name.Name] = fullFields
				}
			}
		}
	}

	if fn == nil {
		return nil, nil, fmt.Errorf("no function with a body found")
	}

	entry, err := CompileBlock(fset, gen, fn.Body, env)
	return entry, gen, err
}

// validateStructFields checks that all fields in a struct have types supported
// by the Sonolus engine memory model. The engine only supports float64 scalar
// fields and named record/container types. Map, chan, func, and interface types
// are rejected.
func validateStructFields(st *ast.StructType) error {
	for _, field := range st.Fields.List {
		if len(field.Names) == 0 {
			// Embedded field: validate the type itself
			if err := validateFieldType(field.Type, "(embedded)"); err != nil {
				return err
			}
			continue
		}
		if err := validateFieldType(field.Type, field.Names[0].Name); err != nil {
			return err
		}
	}
	return nil
}

// validateFieldType checks a single field's AST type expression. It rejects
// types that cannot be represented in the Sonolus engine memory model.
func validateFieldType(expr ast.Expr, fieldName string) error {
	switch t := expr.(type) {
	case *ast.MapType:
		return fmt.Errorf("map type in field %q is not supported (engine memory model only supports float64 scalars)", fieldName)
	case *ast.ChanType:
		return fmt.Errorf("chan type in field %q is not supported (engine memory model only supports float64 scalars)", fieldName)
	case *ast.FuncType:
		return fmt.Errorf("func type in field %q is not supported (engine memory model only supports float64 scalars)", fieldName)
	case *ast.InterfaceType:
		return fmt.Errorf("interface type in field %q is not supported (engine memory model only supports float64 scalars)", fieldName)
	case *ast.ArrayType:
		// Slices ([]float64) are not supported; arrays ([N]float64) are
		// handled by the container system and are OK.
		if _, isEllipsis := t.Len.(*ast.Ellipsis); t.Len == nil || isEllipsis {
			return fmt.Errorf("slice type in field %q is not supported (use fixed-size array or varArray)", fieldName)
		}
	}
	return nil
}

// CompileBlock traces an already-parsed function body into a CFG. It is used to
// compile callback methods directly from a parsed engine file (the body keeps
// its original positions in fset for error messages).
func CompileBlock(fset *token.FileSet, gen *ir.IDGen, body *ast.BlockStmt, env Env) (*ir.BasicBlock, error) {
	if body == nil {
		return nil, fmt.Errorf("frontend: nil function body")
	}
	t := &tracer{
		fset:         fset,
		gen:          gen,
		env:          env,
		vars:         map[string]*ir.TempBlock{},
		arrays:       map[string]*arrayInfo{},
		records:      map[string]*recordInfo{},
		containers:   map[string]*containerInfo{},
		inlining:     map[string]bool{},
		maxUnroll:    env.MaxUnroll,
		maxUnrollCont: env.MaxUnrollCont,
	}
	if t.maxUnroll == 0 {
		t.maxUnroll = 256
	}
	if t.maxUnrollCont == 0 {
		t.maxUnrollCont = 64
	}
	t.populateFieldContainers()
	t.entry = ir.NewBlock()
	t.current = t.entry
	// Callback-level return context: a value return becomes Break on the
	// callback's JumpLoop (target == nil).
	t.returns = append(t.returns, returnCtx{})
	// Push defer scope for this function.
	t.defers = append(t.defers, deferCtx{})
	if err := t.stmtList(body.List); err != nil {
		return nil, err
	}
	// Emit deferred calls at function exit (implicit return with no value).
	if err := t.emitDefers(); err != nil {
		return nil, err
	}
	return t.entry, nil
}

// emitDefers emits all deferred calls for the current function scope in LIFO order.
func (t *tracer) emitDefers() error {
	if len(t.defers) == 0 {
		return nil
	}
	ctx := &t.defers[len(t.defers)-1]
	for i := len(ctx.records) - 1; i >= 0; i-- {
		d := ctx.records[i]
		// Re-evaluate the deferred call with stored arguments.
		// We need to call via the normal call path with pre-evaluated args.
		fn, ok := d.call.Fun.(*ast.Ident)
		if !ok {
			return t.errf(d.call, "deferred call must use a plain function name")
		}
		if _, err := t.callWithArgs(fn, d.call, d.args); err != nil {
			return err
		}
	}
	t.defers = t.defers[:len(t.defers)-1]
	return nil
}

func (t *tracer) errf(node ast.Node, format string, args ...any) error {
	pos := t.fset.Position(node.Pos())
	return fmt.Errorf("%d:%d: "+format, append([]any{pos.Line, pos.Column}, args...)...)
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
	case *ast.LabeledStmt:
		prev := t.stmtLabel
		t.stmtLabel = n.Label.Name
		labelBlock := ir.NewBlock()
		t.fallthroughTo(labelBlock)
		if t.labels == nil {
			t.labels = make(map[string]*ir.BasicBlock)
		}
		t.labels[n.Label.Name] = labelBlock
		t.enter(labelBlock)
		err := t.stmt(n.Stmt)
		t.stmtLabel = prev
		return err
	case *ast.DeferStmt:
		if len(t.defers) == 0 {
			return t.errf(n, "defer outside of a function")
		}
		call := n.Call
		args := make([]Num, len(call.Args))
		for i, a := range call.Args {
			v, err := t.expr(a)
			if err != nil {
				return err
			}
			args[i] = v
		}
		ctx := &t.defers[len(t.defers)-1]
		ctx.records = append(ctx.records, deferRecord{call: call, args: args})
		return nil
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
		return t.errf(s, "unsupported statement %T (the Sonolus engine is single-threaded with a float64-only runtime; defer, go, select, and type switches are not supported)", s)
	}
}

func (t *tracer) alloc(fnName string) *ir.TempBlock {
	tb := ir.NewTemp(fnName)
	t.vars[fnName] = tb
	return tb
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

// addNode emits a PureInstr Add node with the given operands.
func (t *tracer) addNode(a, b ir.Node) ir.Node {
	return t.gen.PureInstr(resource.RuntimeFunctionAdd, a, b)
}

// mulNode emits a PureInstr Multiply node with the given operands.
func (t *tracer) mulNode(a, b ir.Node) ir.Node {
	return t.gen.PureInstr(resource.RuntimeFunctionMultiply, a, b)
}
