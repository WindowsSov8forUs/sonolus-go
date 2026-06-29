package frontend

import (
	"go/ast"
	"go/token"
	"strconv"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

	"github.com/WindowsSov8forUs/sonolus-go/compiler/ir"
)

func (t *tracer) shortCircuitIf(n *ast.IfStmt, bin *ast.BinaryExpr) error {
	if bin.Op == token.LAND {
		return t.shortCircuitAnd(n, bin)
	}
	return t.shortCircuitOr(n, bin)
}

// shortCircuitAnd lowers `left && right` into CFG edges:
//
//	const false && _ → skip to else/merge
//	const true  && right → test right
//	dyn && right → test left, false→else, true→test right
func (t *tracer) shortCircuitAnd(n *ast.IfStmt, bin *ast.BinaryExpr) error {
	thenBlock := ir.NewBlock()
	merge := ir.NewBlock()
	falseTarget, elseBlock := t.setupShortCircuit(n, merge)

	left, err := t.expr(bin.X)
	if err != nil {
		return err
	}
	if left.isConst {
		if left.c == 0 {
			if elseBlock != nil {
				return t.stmtList(n.Else.(*ast.BlockStmt).List)
			}
			return nil
		}
		right, err := t.expr(bin.Y)
		if err != nil {
			return err
		}
		t.current.Test = right.mustNode()
		t.current.ConnectTo(falseTarget, ir.Cond(0))
		t.current.ConnectTo(thenBlock, nil)
	} else {
		t.current.Test = left.mustNode()
		rightBlock := ir.NewBlock()
		t.current.ConnectTo(falseTarget, ir.Cond(0))
		t.current.ConnectTo(rightBlock, nil)
		t.enter(rightBlock)
		right, err := t.expr(bin.Y)
		if err != nil {
			return err
		}
		rightBlock.Test = right.mustNode()
		rightBlock.ConnectTo(falseTarget, ir.Cond(0))
		rightBlock.ConnectTo(thenBlock, nil)
	}

	return t.finishShortCircuit(n, thenBlock, merge, elseBlock)
}

// shortCircuitOr lowers `left || right` into CFG edges:
//
//	const truthy || _ → execute body, jump to merge
//	const falsy  || right → test right
//	dyn || right → test left, true→body, false→test right
func (t *tracer) shortCircuitOr(n *ast.IfStmt, bin *ast.BinaryExpr) error {
	thenBlock := ir.NewBlock()
	merge := ir.NewBlock()
	falseTarget, elseBlock := t.setupShortCircuit(n, merge)

	left, err := t.expr(bin.X)
	if err != nil {
		return err
	}
	if left.isConst {
		if left.c != 0 {
			if err := t.stmtList(n.Body.List); err != nil {
				return err
			}
			t.fallthroughTo(merge)
			t.enter(merge)
			return nil
		}
		right, err := t.expr(bin.Y)
		if err != nil {
			return err
		}
		t.current.Test = right.mustNode()
		t.current.ConnectTo(falseTarget, ir.Cond(0))
		t.current.ConnectTo(thenBlock, nil)
	} else {
		t.current.Test = left.mustNode()
		rightBlock := ir.NewBlock()
		t.current.ConnectTo(thenBlock, nil)
		t.current.ConnectTo(rightBlock, ir.Cond(0))
		t.enter(rightBlock)
		right, err := t.expr(bin.Y)
		if err != nil {
			return err
		}
		rightBlock.Test = right.mustNode()
		rightBlock.ConnectTo(falseTarget, ir.Cond(0))
		rightBlock.ConnectTo(thenBlock, nil)
	}

	return t.finishShortCircuit(n, thenBlock, merge, elseBlock)
}

// setupShortCircuit creates the blocks shared by short-circuit lowering and
// returns the false-branch target (elseBlock if present, otherwise merge).
func (t *tracer) setupShortCircuit(n *ast.IfStmt, merge *ir.BasicBlock) (falseTarget *ir.BasicBlock, elseBlock *ir.BasicBlock) {
	if n.Else != nil {
		elseBlock = ir.NewBlock()
		falseTarget = elseBlock
	} else {
		falseTarget = merge
	}
	return
}

// finishShortCircuit traces the then/else bodies and rejoins at merge.
func (t *tracer) finishShortCircuit(n *ast.IfStmt, thenBlock, merge, elseBlock *ir.BasicBlock) error {
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
	keyName, ok := n.Key.(*ast.Ident)
	if !ok {
		return t.errf(n, "range key must be an identifier")
	}
	if n.Tok != token.DEFINE && n.Tok != token.ASSIGN {
		return t.errf(n, "range requires := or =")
	}
	// Extract value variable name if present.
	var valName string
	if n.Value != nil {
		v, ok := n.Value.(*ast.Ident)
		if !ok {
			return t.errf(n, "range value must be an identifier")
		}
		valName = v.Name
	}

	// Container iteration: for i, v := range containerName
	if id, ok := n.X.(*ast.Ident); ok {
		if ci, ok2 := t.containers[id.Name]; ok2 {
			return t.containerIter(n, ci, keyName.Name, valName)
		}
	}

	// Capture array name before evaluation (needed for value variable read).
	var arrName string
	if valName != "" {
		if id, isID := n.X.(*ast.Ident); isID {
			arrName = id.Name
		}
	}

	// Evaluate the bound expression.
	bound, err := t.expr(n.X)
	if err != nil {
		return err
	}

	// Declare loop variable i := 0
	iTB := ir.NewTemp("range")
	t.vars[keyName.Name] = iTB
	t.emit(t.gen.SetPlace(t.cell(iTB), ir.Const(0)))

	// Compile-time loop unrolling: if the bound is a constant integer, expand the
	// body N times instead of emitting a runtime loop. This matches sonolus.py and
	// sonolus.js-compiler behaviour where loops over comptime-known iterables are
	// always unrolled (flat code runs faster than handshake-branch loops).
	const maxUnroll = 256
	if bound.isConst && bound.c > 0 && bound.c <= maxUnroll && bound.c == float64(int(bound.c)) {
		count := int(bound.c)
		for iter := 0; iter < count; iter++ {
			// Set i = iter at the start of each unrolled copy.
			t.emit(t.gen.SetPlace(t.cell(iTB), ir.Const(iter)))
			// If value variable, read array[iter].
			if valName != "" {
				if arr, ok := t.arrays[arrName]; ok {
					valTB := t.vars[valName]
					if arr.elemSize == 1 {
						t.emit(t.gen.SetPlace(ir.TempCell(valTB),
							ir.GetPlace(ir.BlockPlace{Block: arr.tb, Index: ir.Const(iter), Offset: 0})))
					} else {
						offset := int(iter * arr.elemSize)
						t.emit(t.gen.SetPlace(ir.TempCell(valTB),
							ir.GetPlace(ir.BlockPlace{Block: arr.tb, Index: ir.Const(offset), Offset: 0})))
					}
				}
			}
			if err := t.stmtList(n.Body.List); err != nil {
				return err
			}
		}
		// Clean up loop variables.
		delete(t.vars, keyName.Name)
		if valName != "" {
			delete(t.vars, valName)
		}
		return nil
	}

	loopHead := ir.NewBlock()
	loopBody := ir.NewBlock()
	merge := ir.NewBlock()

	t.current.ConnectTo(loopHead, nil)
	t.enter(loopHead)

	// Test: i < bound
	loopHead.Test = t.gen.PureInstr(resource.RuntimeFunctionLess, ir.GetPlace(t.cell(iTB)), bound.mustNode())
	loopHead.ConnectTo(merge, ir.Cond(0)) // false → exit
	loopHead.ConnectTo(loopBody, nil)     // true → body

	// Loop body
	t.enter(loopBody)
	// If value variable, read array[i] at start of each iteration.
	if valName != "" {
		if arr, ok := t.arrays[arrName]; ok {
			valTB := t.alloc(valName)
			idxVal := ir.GetPlace(t.cell(iTB))
			// Read array element: index * elemSize into the array temp.
			if arr.elemSize == 1 {
				t.emit(t.gen.SetPlace(ir.TempCell(valTB),
					ir.GetPlace(ir.BlockPlace{Block: arr.tb, Index: idxVal, Offset: 0})))
			} else {
				// Multi-slot element: read slot 0 (the scalar or first field).
				offset := t.gen.PureInstr(resource.RuntimeFunctionMultiply, idxVal, ir.Const(arr.elemSize))
				t.emit(t.gen.SetPlace(ir.TempCell(valTB),
					ir.GetPlace(ir.BlockPlace{Block: arr.tb, Index: offset, Offset: 0})))
			}
		}
	}
	t.loops = append(t.loops, loopCtx{breakTo: merge, continueTo: loopHead})
	if err := t.stmtList(n.Body.List); err != nil {
		return err
	}
	t.loops = t.loops[:len(t.loops)-1]

	// Increment i++
	t.emit(t.gen.SetPlace(t.cell(iTB),
		t.gen.PureInstr(resource.RuntimeFunctionAdd, ir.GetPlace(t.cell(iTB)), ir.Const(1))))
	t.fallthroughTo(loopHead)

	t.enter(merge)
	t.terminated = len(merge.Incoming) == 0

	// Clean up loop variables
	delete(t.vars, keyName.Name)
	if valName != "" {
		delete(t.vars, valName)
	}
	return nil
}

// containerIter lowers `for i, v := range container` for VarArray/ArrayMap/ArraySet.
// It emits a runtime loop that reads elements from the backing array, using
// ci.readSize() as the dynamic bound and ci.elemPlace for element access.
func (t *tracer) containerIter(n *ast.RangeStmt, ci *containerInfo, keyName, valName string) error {
	// Allocate index variable (i).
	iTB := ir.NewTemp("range")
	t.vars[keyName] = iTB
	t.emit(t.gen.SetPlace(t.cell(iTB), ir.Const(0)))

	// If capacity is small and constant, unroll the loop.
	const maxUnroll = 64
	if ci.capacity <= maxUnroll {
		for iter := 0; iter < ci.capacity; iter++ {
			// If _size <= iter, skip remaining iterations.
			var skipBlock, bodyBlock *ir.BasicBlock
			if iter > 0 {
				sizeNode := ci.readSize()
				cond := t.gen.PureInstr(resource.RuntimeFunctionLess, ir.Const(iter), sizeNode)
				skipBlock = ir.NewBlock()
				bodyBlock = ir.NewBlock()
				t.current.Test = cond
				t.current.ConnectTo(skipBlock, ir.Cond(0))
				t.current.ConnectTo(bodyBlock, nil)
				t.enter(bodyBlock)
			}
			// Set index = iter.
			t.emit(t.gen.SetPlace(t.cell(iTB), ir.Const(iter)))
			// If value variable, read element at index.
			if valName != "" {
				valTB := t.alloc(valName)
				elemNode := ir.GetPlace(ci.elemPlace(t.gen, ir.Const(iter)))
				t.emit(t.gen.SetPlace(ir.TempCell(valTB), elemNode))
			}
			if err := t.stmtList(n.Body.List); err != nil {
				return err
			}
			if iter > 0 {
				t.fallthroughTo(skipBlock)
				t.enter(skipBlock)
			}
		}
		delete(t.vars, keyName)
		if valName != "" {
			delete(t.vars, valName)
		}
		return nil
	}

	// Runtime loop: for i := 0; i < _size; i++ { body }
	loopHead := ir.NewBlock()
	loopBody := ir.NewBlock()
	merge := ir.NewBlock()

	t.current.ConnectTo(loopHead, nil)
	t.enter(loopHead)

	// Test: i < _size
	loopHead.Test = t.gen.PureInstr(resource.RuntimeFunctionLess,
		ir.GetPlace(t.cell(iTB)), ci.readSize())
	loopHead.ConnectTo(merge, ir.Cond(0))
	loopHead.ConnectTo(loopBody, nil)

	// Body: read element if value variable
	t.enter(loopBody)
	if valName != "" {
		valTB := t.alloc(valName)
		idxNode := ir.GetPlace(t.cell(iTB))
		elemNode := ir.GetPlace(ci.elemPlace(t.gen, idxNode))
		t.emit(t.gen.SetPlace(ir.TempCell(valTB), elemNode))
	}

	t.loops = append(t.loops, loopCtx{breakTo: merge, continueTo: loopHead})
	if err := t.stmtList(n.Body.List); err != nil {
		return err
	}
	t.loops = t.loops[:len(t.loops)-1]

	// Increment i++
	t.emit(t.gen.SetPlace(t.cell(iTB),
		t.gen.PureInstr(resource.RuntimeFunctionAdd, ir.GetPlace(t.cell(iTB)), ir.Const(1))))
	t.fallthroughTo(loopHead)

	t.enter(merge)
	t.terminated = len(merge.Incoming) == 0

	delete(t.vars, keyName)
	if valName != "" {
		delete(t.vars, valName)
	}
	return nil
}

func (t *tracer) switchStmt(n *ast.SwitchStmt) error {
	if n.Init != nil {
		if err := t.stmt(n.Init); err != nil {
			return err
		}
	}

	// Evaluate the tag expression (or treat as untagged-switch with tautology).
	var tag Num
	if n.Tag != nil {
		v, err := t.expr(n.Tag)
		if err != nil {
			return err
		}
		tag = v
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

		// Evaluate every case expression once, then build the condition.
		caseValues := make([]Num, len(cc.List))
		for j, expr := range cc.List {
			cv, err := t.expr(expr)
			if err != nil {
				return err
			}
			caseValues[j] = cv
		}

		var cond Num
		if n.Tag == nil {
			// Untagged: expressions are boolean conditions directly.
			cond = caseValues[0]
			for _, cv := range caseValues[1:] {
				var ok bool
				cond, ok = applyBinary(t.gen, token.LOR, cond, cv)
				if !ok {
					return t.errf(cc, "unsupported || in switch case")
				}
			}
		} else {
			// Tagged: build tag == caseVal for each.
			eq2, ok := applyBinary(t.gen, token.EQL, tag, caseValues[0])
			if !ok {
				return t.errf(cc, "unsupported == in switch case")
			}
			cond = eq2
			for _, cv := range caseValues[1:] {
				eq2, ok = applyBinary(t.gen, token.EQL, tag, cv)
				if !ok {
					return t.errf(cc, "unsupported == in switch case")
				}
				cond, ok = applyBinary(t.gen, token.LOR, cond, eq2)
				if !ok {
					return t.errf(cc, "unsupported || in switch case")
				}
			}
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
		t.current.Test = cond.mustNode()
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
		if err := t.stmt(n.Init); err != nil {
			return err
		}
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
	condBlock.Test = cond.mustNode()

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
		header.Test = cond.mustNode()
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
	rc := &t.returns[len(t.returns)-1]

	// Multi-return: evaluate all results into vals slice.
	var vals []Num
	for _, r := range n.Results {
		v, err := t.expr(r)
		if err != nil {
			return err
		}
		vals = append(vals, v)
	}

	if rc.target == nil {
		// Callback: only single scalar return is valid (callback yields a float
		// via Break). Composite / multi returns are rejected at the callback level.
		if len(vals) == 0 {
			// void return — no break necessary
		} else if len(vals) == 1 {
			v := vals[0]
			if v.IsComposite() {
				return t.errf(n, "cannot return a composite value from a callback; return individual fields instead")
			}
			t.emit(t.gen.ImpureInstr(resource.RuntimeFunctionBreak, v.mustNode(), ir.Const(1)))
		} else {
			return t.errf(n, "cannot return multiple values from a callback; callbacks yield a single float")
		}
	} else {
		// Inlined function: write return values to consecutive slots in retTemp.
		// Expand rc.temp size if needed.
		if len(vals) > rc.temp.Size {
			rc.temp.Size = len(vals)
		}
		if len(vals) == 1 && vals[0].IsComposite() {
			// Single composite return — use the composite's own field names.
			order, err := vals[0].CompositeFieldOrder()
			if err != nil {
				return err
			}
			rc.compositeFields = order
			for i, f := range rc.compositeFields {
				t.emit(t.gen.SetPlace(ir.BlockPlace{Block: rc.temp, Index: ir.Const(i), Offset: 0}, vals[0].Field(f).mustNode()))
			}
		} else if len(vals) == 1 {
			// Single scalar return.
			t.emit(t.gen.SetPlace(ir.TempCell(rc.temp), vals[0].mustNode()))
		} else if len(vals) > 1 {
			// Multi-return: auto-generate field names _0, _1, ...
			fields := make([]string, len(vals))
			for i := range vals {
				fields[i] = "_" + strconv.Itoa(i)
			}
			rc.compositeFields = fields
			for i, v := range vals {
				t.emit(t.gen.SetPlace(ir.BlockPlace{Block: rc.temp, Index: ir.Const(i), Offset: 0}, v.mustNode()))
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
