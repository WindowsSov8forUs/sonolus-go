package frontend

import (
	"fmt"
	"go/ast"
	"go/token"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/ir"
)

func (t *tracer) rangeStmt(n *ast.RangeStmt) error {
	// Lower for-range over a count/collection into a standard integer for-loop:
	//   for i := range counts → for i := 0; i < counts; i++ { ... }
	// Synthesize key variable if not provided: for range N {}.
	// Non-identifier keys (e.g., a.x, b[0]) are synthesized into a temp
	// and assigned to the user's LHS at the top of each iteration via writePlace.
	var synthKeyLHS ast.Expr
	hasSyntheticKey := false
	if n.Key == nil {
		hasSyntheticKey = true
		name := fmt.Sprintf("_rkey_%d", t.gen.Next())
		n.Key = &ast.Ident{Name: name}
		t.alloc(name)
	} else if _, ok := n.Key.(*ast.Ident); !ok {
		synthKeyLHS = n.Key
		name := fmt.Sprintf("_rk_%d", t.gen.Next())
		t.alloc(name)
		n.Key = &ast.Ident{Name: name}
	}
	keyName, ok := n.Key.(*ast.Ident)
	if !ok {
		return t.errf(n, "range key must be an identifier")
	}
	if n.Tok != token.DEFINE && n.Tok != token.ASSIGN {
		return t.errf(n, "range requires := or =")
	}
	// Extract value variable name. Non-identifier values (e.g., a.x, b[0])
	// are synthesized into a temp and assigned to the user's LHS at iteration top.
	var synthValLHS ast.Expr
	var valName string
	if n.Value != nil {
		if v, ok := n.Value.(*ast.Ident); !ok {
			synthValLHS = n.Value
			name := fmt.Sprintf("_rv_%d", t.gen.Next())
			t.alloc(name)
			n.Value = &ast.Ident{Name: name}
			valName = name
		} else {
			valName = v.Name
		}
	}

	// Container iteration: for i, v := range containerName
	if id, ok := n.X.(*ast.Ident); ok {
		if ci, ok2 := t.containers[id.Name]; ok2 {
			return t.containerIter(n, ci, keyName.Name, valName, synthKeyLHS, synthValLHS)
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
			// Assign synthesized temps to user LHS at top of each iteration.
			if err := t.emitRangeSynthAssignments(synthKeyLHS, iTB, synthValLHS, valName); err != nil {
				return err
			}
			if err := t.stmtList(n.Body.List); err != nil {
				return err
			}
		}
		// Remove synthetic key from vars after loop
		if hasSyntheticKey {
			if keyIdent, ok := n.Key.(*ast.Ident); ok {
				delete(t.vars, keyIdent.Name)
			}
		}
		t.cleanupLoopVars(keyName.Name, valName)
		t.enterMerge()
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
	// Assign synthesized temps to user LHS at top of each iteration.
	if err := t.emitRangeSynthAssignments(synthKeyLHS, iTB, synthValLHS, valName); err != nil {
		return err
	}
	t.loops = append(t.loops, loopCtx{breakTo: merge, continueTo: loopHead, label: t.stmtLabel})
	t.stmtLabel = ""
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

	// Remove synthetic key from vars after loop
	if hasSyntheticKey {
		if keyIdent, ok := n.Key.(*ast.Ident); ok {
			delete(t.vars, keyIdent.Name)
		}
	}
	t.cleanupLoopVars(keyName.Name, valName)
	return nil
}

// containerIter lowers `for i, v := range container` for VarArray/ArrayMap/ArraySet.
// It emits a runtime loop that reads elements from the backing array, using
// ci.readSize() as the dynamic bound and ci.elemPlace for element access.
func (t *tracer) containerIter(n *ast.RangeStmt, ci *containerInfo, keyName, valName string, synthKeyLHS, synthValLHS ast.Expr) error {
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
			// Assign synthesized temps to user LHS at top of each iteration.
			if err := t.emitRangeSynthAssignments(synthKeyLHS, iTB, synthValLHS, valName); err != nil {
				return err
			}
			if err := t.stmtList(n.Body.List); err != nil {
				return err
			}
			if iter > 0 {
				t.fallthroughTo(skipBlock)
				t.enter(skipBlock)
			}
		}
		t.cleanupLoopVars(keyName, valName)
		t.enterMerge()
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
	// Assign synthesized temps to user LHS at top of each iteration.
	if err := t.emitRangeSynthAssignments(synthKeyLHS, iTB, synthValLHS, valName); err != nil {
		return err
	}

	t.loops = append(t.loops, loopCtx{breakTo: merge, continueTo: loopHead, label: t.stmtLabel})
	t.stmtLabel = ""
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

	t.cleanupLoopVars(keyName, valName)
	return nil
}

// cleanupLoopVars removes the loop key and optional value variable from t.vars
// after a for-range completes.
func (t *tracer) cleanupLoopVars(keyName, valName string) {
	delete(t.vars, keyName)
	if valName != "" {
		delete(t.vars, valName)
	}
}

// enterMerge creates a merge block after an unrolled loop body and enters it.
// Subsequent statements will be reachable through the merge block (unless the
// loop body always terminates, in which case merge has no incoming edges and
// is treated as dead code).
func (t *tracer) enterMerge() *ir.BasicBlock {
	merge := ir.NewBlock()
	t.fallthroughTo(merge)
	t.enter(merge)
	t.terminated = len(merge.Incoming) == 0
	return merge
}

// emitRangeSynthAssignments emits writePlace calls to assign synthesized range
// key/value temps to the user's original LHS expressions at the top of each
// loop iteration. This supports range statements where the key or value is a
// non-identifier expression such as a.x or b[0].
func (t *tracer) emitRangeSynthAssignments(synthKeyLHS ast.Expr, keyTB *ir.TempBlock, synthValLHS ast.Expr, valName string) error {
	if synthKeyLHS != nil {
		if err := t.writePlace(synthKeyLHS, exprNum(ir.GetPlace(t.cell(keyTB)))); err != nil {
			return err
		}
	}
	if synthValLHS != nil && valName != "" {
		if valTB := t.vars[valName]; valTB != nil {
			if err := t.writePlace(synthValLHS, exprNum(ir.GetPlace(t.cell(valTB)))); err != nil {
				return err
			}
		}
	}
	return nil
}
