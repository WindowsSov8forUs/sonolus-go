package frontend

import (
	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

	"github.com/WindowsSov8forUs/sonolus-go/compiler/ir"
)

// ── VarArray.sort() — in-place insertion sort on backing array ──────────────
//
// varArraySortCI emits an in-place insertion sort on the container's backing
// array, sorting elements 0.._size-1 in ascending order. The sort is stable
// and uses insertion sort (O(n²) comparisons) which is efficient for the
// small arrays typical in Sonolus engines.

// varArraySortCI sorts the container in-place.
func varArraySortCI(t *tracer, ci *containerInfo, v Num, args []Num) (Num, error) {
	// Insertion sort:
	// for i := 1; i < _size; i++ {
	//     key := arr[i]
	//     j := i - 1
	//     while j >= 0 && arr[j] > key {
	//         arr[j+1] = arr[j]
	//         j--
	//     }
	//     arr[j+1] = key
	// }
	_ = v
	_ = args

	// Allocate temps.
	iTB := &ir.TempBlock{Name: "sortI", Size: 1}
	iPlace := ir.BlockPlace{Block: iTB, Index: ir.Const(0), Offset: 0}
	jTB := &ir.TempBlock{Name: "sortJ", Size: 1}
	jPlace := ir.BlockPlace{Block: jTB, Index: ir.Const(0), Offset: 0}
	keyTB := &ir.TempBlock{Name: "sortKey", Size: 1}
	keyPlace := ir.BlockPlace{Block: keyTB, Index: ir.Const(0), Offset: 0}

	// i = 1
	t.emit(t.gen.SetPlace(iPlace, ir.Const(1)))

	// Outer loop header
	outerHead := ir.NewBlock()
	outerBody := ir.NewBlock()
	outerExit := ir.NewBlock()

	t.fallthroughTo(outerHead)

	// Outer: test i < _size
	t.enter(outerHead)
	iNode := ir.GetPlace(iPlace)
	outerCond := t.gen.PureInstr(resource.RuntimeFunctionLess, iNode, ci.readSize())
	outerHead.Test = outerCond
	outerHead.ConnectTo(outerExit, ir.Cond(0))
	outerHead.ConnectTo(outerBody, nil)

	// Outer body
	t.enter(outerBody)
	// key = arr[i]
	keyVal := ir.GetPlace(ci.elemPlace(t.gen, iNode))
	t.emit(t.gen.SetPlace(keyPlace, keyVal))
	// j = i - 1
	jStart := t.gen.PureInstr(resource.RuntimeFunctionSubtract, iNode, ir.Const(1))
	t.emit(t.gen.SetPlace(jPlace, jStart))

	// Inner loop header
	innerHead := ir.NewBlock()
	innerBody := ir.NewBlock()
	innerExit := ir.NewBlock()
	t.current.ConnectTo(innerHead, nil)

	t.enter(innerHead)
	jNode := ir.GetPlace(jPlace)
	// j >= 0 && arr[j] > key
	jGe0 := t.gen.PureInstr(resource.RuntimeFunctionGreaterOr, jNode, ir.Const(-1))
	arrJ := ir.GetPlace(ci.elemPlace(t.gen, jNode))
	jGtKey := t.gen.PureInstr(resource.RuntimeFunctionGreater, arrJ, ir.GetPlace(keyPlace))
	innerCond := t.gen.PureInstr(resource.RuntimeFunctionAnd, jGe0, jGtKey)
	innerHead.Test = innerCond
	innerHead.ConnectTo(innerExit, ir.Cond(0))
	innerHead.ConnectTo(innerBody, nil)

	// Inner body: arr[j+1] = arr[j]; j--
	t.enter(innerBody)
	jPlus1 := t.gen.PureInstr(resource.RuntimeFunctionAdd, jNode, ir.Const(1))
	t.emit(t.gen.SetPlace(ci.elemPlace(t.gen, jPlus1), arrJ))
	jMinus1 := t.gen.PureInstr(resource.RuntimeFunctionSubtract, jNode, ir.Const(1))
	t.emit(t.gen.SetPlace(jPlace, jMinus1))
	t.current.ConnectTo(innerHead, nil)

	// Inner exit: arr[j+1] = key; i++
	t.enter(innerExit)
	jFinal := ir.GetPlace(jPlace)
	jPlus1Final := t.gen.PureInstr(resource.RuntimeFunctionAdd, jFinal, ir.Const(1))
	t.emit(t.gen.SetPlace(ci.elemPlace(t.gen, jPlus1Final), ir.GetPlace(keyPlace)))
	iPlus1 := t.gen.PureInstr(resource.RuntimeFunctionAdd, iNode, ir.Const(1))
	t.emit(t.gen.SetPlace(iPlace, iPlus1))
	t.current.ConnectTo(outerHead, nil)

	// Outer exit
	t.enter(outerExit)
	return constNum(0), nil
}

// ── sortLinkedEntities builtin ──────────────────────────────────────────────
//
// sortLinkedEntities(headRef, sortKeyOffset, nextOffset, prevOffset?) sorts a
// linked list of entities. It walks the linked list starting from headRef,
// collects entity indices, sorts them by the sort key field value, and re-wires
// the next (and optionally prev) pointers.
//
// The field offsets are compile-time constants: sortKeyOffset is the memory slot
// offset for the sort key field within the entity data block, nextOffset is the
// offset for the next-pointer field, and prevOffset is for the prev-pointer.

// sortLinkedEntitiesCall implements the sortLinkedEntities builtin.
// Called from callWithArgs with:
//   args[0] = head EntityRef value (entity index)
//   args[1] = sortKeyOffset (compile-time constant)
//   args[2] = nextOffset (compile-time constant)
//   args[3] = prevOffset (optional, 0 means no prev pointer)
func sortLinkedEntitiesCall(t *tracer, args []Num) (Num, error) {
	headIdx := args[0]
	sortKeyOff := int(args[1].c)
	nextOff := int(args[2].c)
	prevOff := 0
	if len(args) >= 4 {
		prevOff = int(args[3].c)
	}

	const maxEntities = 128
	// Allocate a temp array for entity indices (max 128 entities).
	idxTB := &ir.TempBlock{Name: "sortIndices", Size: maxEntities}
	// Walk the linked list, collecting indices.
	countTB := &ir.TempBlock{Name: "sortCount", Size: 1}
	countPlace := ir.BlockPlace{Block: countTB, Index: ir.Const(0), Offset: 0}
	curTB := &ir.TempBlock{Name: "sortCur", Size: 1}
	curPlace := ir.BlockPlace{Block: curTB, Index: ir.Const(0), Offset: 0}

	t.emit(t.gen.SetPlace(countPlace, ir.Const(0)))
	t.emit(t.gen.SetPlace(curPlace, headIdx.mustNode()))

	// Walk loop
	walkHead := ir.NewBlock()
	walkBody := ir.NewBlock()
	walkExit := ir.NewBlock()

	t.fallthroughTo(walkHead)
	t.enter(walkHead)
	curNode := ir.GetPlace(curPlace)
	// while cur > 0
	walkCond := t.gen.PureInstr(resource.RuntimeFunctionGreater, curNode, ir.Const(0))
	walkHead.Test = walkCond
	walkHead.ConnectTo(walkExit, ir.Cond(0))
	walkHead.ConnectTo(walkBody, nil)

	// Body: store index, read next
	t.enter(walkBody)
	countNode := ir.GetPlace(countPlace)
	// indices[count] = cur
	t.emit(t.gen.SetPlace(ir.BlockPlace{Block: idxTB, Index: countNode, Offset: 0}, curNode))
	// count++
	newCount := t.gen.PureInstr(resource.RuntimeFunctionAdd, countNode, ir.Const(1))
	t.emit(t.gen.SetPlace(countPlace, newCount))
	// cur = EntityMemory[cur * entitySize + nextOff]
	nextVal := ir.GetPlace(ir.BlockPlace{Block: ir.Const(ir.BlockEntityMemory), Index: t.gen.PureInstr(resource.RuntimeFunctionAdd, curNode, ir.Const(nextOff)), Offset: 0})
	t.emit(t.gen.SetPlace(curPlace, nextVal))
	t.current.ConnectTo(walkHead, nil)

	t.enter(walkExit)

	// Now sort the collected indices by their sort key values.
	// Use insertion sort on the index array, comparing entity values.
	if err := emitEntitySort(t, idxTB, countTB, sortKeyOff); err != nil {
		return Num{}, err
	}

	// Re-wire next pointers: for i = 0; i < count-1; i++ { entity[indices[i] + nextOff] = indices[i+1] }
	rewireHead := ir.NewBlock()
	rewireBody := ir.NewBlock()
	rewireExit := ir.NewBlock()
	iTB := &ir.TempBlock{Name: "rewireI", Size: 1}
	iPlace := ir.BlockPlace{Block: iTB, Index: ir.Const(0), Offset: 0}
	t.emit(t.gen.SetPlace(iPlace, ir.Const(0)))

	t.fallthroughTo(rewireHead)
	t.enter(rewireHead)
	iNode := ir.GetPlace(iPlace)
	countMinus1 := t.gen.PureInstr(resource.RuntimeFunctionSubtract, ir.GetPlace(countPlace), ir.Const(1))
	rewireCond := t.gen.PureInstr(resource.RuntimeFunctionLess, iNode, countMinus1)
	rewireHead.Test = rewireCond
	rewireHead.ConnectTo(rewireExit, ir.Cond(0))
	rewireHead.ConnectTo(rewireBody, nil)

	t.enter(rewireBody)
	thisIdx := ir.GetPlace(ir.BlockPlace{Block: idxTB, Index: iNode, Offset: 0})
	nextIdx := ir.GetPlace(ir.BlockPlace{Block: idxTB, Index: t.gen.PureInstr(resource.RuntimeFunctionAdd, iNode, ir.Const(1)), Offset: 0})
	t.emit(t.gen.SetPlace(ir.BlockPlace{Block: ir.Const(ir.BlockEntityMemory), Index: t.gen.PureInstr(resource.RuntimeFunctionAdd, thisIdx, ir.Const(nextOff)), Offset: 0}, nextIdx))
	if prevOff != 0 {
		// Set prev pointer: entity[nextIdx + prevOff] = thisIdx
		t.emit(t.gen.SetPlace(ir.BlockPlace{Block: ir.Const(ir.BlockEntityMemory), Index: t.gen.PureInstr(resource.RuntimeFunctionAdd, nextIdx, ir.Const(prevOff)), Offset: 0}, thisIdx))
	}
	// i++
	t.emit(t.gen.SetPlace(iPlace, t.gen.PureInstr(resource.RuntimeFunctionAdd, iNode, ir.Const(1))))
	t.current.ConnectTo(rewireHead, nil)

	t.enter(rewireExit)
	// Return the new head (indices[0]).
	if countNode := ir.GetPlace(countPlace); true {
		_ = countNode
	}
	return exprNum(ir.GetPlace(ir.BlockPlace{Block: idxTB, Index: ir.Const(0), Offset: 0})), nil
}

// emitEntitySort performs insertion sort on the entity index array, comparing
// entities by their sort key field value (read from EntityMemoryBlock).
func emitEntitySort(t *tracer, idxTB *ir.TempBlock, countTB *ir.TempBlock, sortKeyOff int) error {
	// Insertion sort on the index array
	iTB := &ir.TempBlock{Name: "esI", Size: 1}
	iPlace := ir.BlockPlace{Block: iTB, Index: ir.Const(0), Offset: 0}
	t.emit(t.gen.SetPlace(iPlace, ir.Const(1)))

	outerHead := ir.NewBlock()
	outerBody := ir.NewBlock()
	outerExit := ir.NewBlock()

	t.fallthroughTo(outerHead)
	t.enter(outerHead)
	iNode := ir.GetPlace(iPlace)
	countNode := ir.GetPlace(ir.BlockPlace{Block: countTB, Index: ir.Const(0), Offset: 0})
	cond := t.gen.PureInstr(resource.RuntimeFunctionLess, iNode, countNode)
	outerHead.Test = cond
	outerHead.ConnectTo(outerExit, ir.Cond(0))
	outerHead.ConnectTo(outerBody, nil)

	t.enter(outerBody)
	// keyIdx = indices[i]
	keyIdx := ir.GetPlace(ir.BlockPlace{Block: idxTB, Index: iNode, Offset: 0})
	// keyVal = EntityMemory[keyIdx + sortKeyOff]
	keyVal := ir.GetPlace(ir.BlockPlace{Block: ir.Const(ir.BlockEntityMemory), Index: t.gen.PureInstr(resource.RuntimeFunctionAdd, keyIdx, ir.Const(sortKeyOff)), Offset: 0})
	keyTB := &ir.TempBlock{Name: "esKey", Size: 1}
	keyPlace := ir.BlockPlace{Block: keyTB, Index: ir.Const(0), Offset: 0}
	t.emit(t.gen.SetPlace(keyPlace, keyIdx))
	keyValTB := &ir.TempBlock{Name: "esKeyVal", Size: 1}
	keyValPlace := ir.BlockPlace{Block: keyValTB, Index: ir.Const(0), Offset: 0}
	t.emit(t.gen.SetPlace(keyValPlace, keyVal))

	// j = i - 1
	jTB := &ir.TempBlock{Name: "esJ", Size: 1}
	jPlace := ir.BlockPlace{Block: jTB, Index: ir.Const(0), Offset: 0}
	t.emit(t.gen.SetPlace(jPlace, t.gen.PureInstr(resource.RuntimeFunctionSubtract, iNode, ir.Const(1))))

	// Inner loop
	innerHead := ir.NewBlock()
	innerBody := ir.NewBlock()
	innerExit := ir.NewBlock()
	t.current.ConnectTo(innerHead, nil)

	t.enter(innerHead)
	jNode := ir.GetPlace(jPlace)
	jGe0 := t.gen.PureInstr(resource.RuntimeFunctionGreaterOr, jNode, ir.Const(0))
	jIdx := ir.GetPlace(ir.BlockPlace{Block: idxTB, Index: jNode, Offset: 0})
	jVal := ir.GetPlace(ir.BlockPlace{Block: ir.Const(ir.BlockEntityMemory), Index: t.gen.PureInstr(resource.RuntimeFunctionAdd, jIdx, ir.Const(sortKeyOff)), Offset: 0})
	jGtKey := t.gen.PureInstr(resource.RuntimeFunctionGreater, jVal, ir.GetPlace(keyValPlace))
	innerCond := t.gen.PureInstr(resource.RuntimeFunctionAnd, jGe0, jGtKey)
	innerHead.Test = innerCond
	innerHead.ConnectTo(innerExit, ir.Cond(0))
	innerHead.ConnectTo(innerBody, nil)

	t.enter(innerBody)
	// indices[j+1] = indices[j]
	jPlus1 := t.gen.PureInstr(resource.RuntimeFunctionAdd, jNode, ir.Const(1))
	t.emit(t.gen.SetPlace(ir.BlockPlace{Block: idxTB, Index: jPlus1, Offset: 0}, jIdx))
	// j--
	t.emit(t.gen.SetPlace(jPlace, t.gen.PureInstr(resource.RuntimeFunctionSubtract, jNode, ir.Const(1))))
	t.current.ConnectTo(innerHead, nil)

	// Inner exit
	t.enter(innerExit)
	jFinal := ir.GetPlace(jPlace)
	jPlus1Final := t.gen.PureInstr(resource.RuntimeFunctionAdd, jFinal, ir.Const(1))
	t.emit(t.gen.SetPlace(ir.BlockPlace{Block: idxTB, Index: jPlus1Final, Offset: 0}, ir.GetPlace(keyPlace)))
	iPlus1 := t.gen.PureInstr(resource.RuntimeFunctionAdd, iNode, ir.Const(1))
	t.emit(t.gen.SetPlace(iPlace, iPlus1))
	t.current.ConnectTo(outerHead, nil)

	t.enter(outerExit)
	return nil
}
