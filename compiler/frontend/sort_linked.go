package frontend

import (
	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

	"github.com/WindowsSov8forUs/sonolus-go/compiler/ir"
)

// ── Shared insertion sort skeleton ───────────────────────────────────────────

// sortElemAccess abstracts element access for the insertion sort skeleton.
// getCompare returns the comparison value at the given index (for Greater check).
// getRaw returns the raw element to store as the sort key (defaults to getCompare
// if nil). set emits a Set to store a raw element at the given index.
type sortElemAccess struct {
	getCompare func(gen *ir.IDGen, idx ir.Node) ir.Node
	getRaw     func(gen *ir.IDGen, idx ir.Node) ir.Node // optional, defaults to getCompare
	set        func(gen *ir.IDGen, idx ir.Node, val ir.Node) ir.Node
}

func (a sortElemAccess) get(gen *ir.IDGen, idx ir.Node) ir.Node {
	if a.getRaw != nil {
		return a.getRaw(gen, idx)
	}
	return a.getCompare(gen, idx)
}

// emitInsertionSort emits an in-place insertion sort on the given element
// range, comparing elements with Greater. The size node must be a readable
// value (e.g. container size or count variable). Used by both varArray sort
// and linked entity sort.
func emitInsertionSort(t *tracer, sizeNode ir.Node, elem sortElemAccess) {
	// Allocate temps: i, j, key, key value.
	iTB := &ir.TempBlock{Name: "sortI", Size: 1}
	iPlace := ir.BlockPlace{Block: iTB, Index: ir.Const(0), Offset: 0}
	jTB := &ir.TempBlock{Name: "sortJ", Size: 1}
	jPlace := ir.BlockPlace{Block: jTB, Index: ir.Const(0), Offset: 0}
	keyTB := &ir.TempBlock{Name: "sortKey", Size: 1}
	keyPlace := ir.BlockPlace{Block: keyTB, Index: ir.Const(0), Offset: 0}
	keyValTB := &ir.TempBlock{Name: "sortKeyVal", Size: 1}
	keyValPlace := ir.BlockPlace{Block: keyValTB, Index: ir.Const(0), Offset: 0}
	// i = 1
	t.emit(t.gen.SetPlace(iPlace, ir.Const(1)))

	outerHead := ir.NewBlock()
	outerBody := ir.NewBlock()
	outerExit := ir.NewBlock()
	t.fallthroughTo(outerHead)

	t.enter(outerHead)
	iNode := ir.GetPlace(iPlace)
	outerCond := t.gen.PureInstr(resource.RuntimeFunctionLess, iNode, sizeNode)
	outerHead.Test = outerCond
	outerHead.ConnectTo(outerExit, ir.Cond(0))
	outerHead.ConnectTo(outerBody, nil)

	t.enter(outerBody)
	// key = raw element, keyVal = comparison value
	keyRaw := elem.get(t.gen, iNode)
	keyCmp := elem.getCompare(t.gen, iNode)
	t.emit(t.gen.SetPlace(keyPlace, keyRaw))
	t.emit(t.gen.SetPlace(keyValPlace, keyCmp))
	jStart := t.gen.PureInstr(resource.RuntimeFunctionSubtract, iNode, ir.Const(1))
	t.emit(t.gen.SetPlace(jPlace, jStart))

	innerHead := ir.NewBlock()
	innerBody := ir.NewBlock()
	innerExit := ir.NewBlock()
	t.current.ConnectTo(innerHead, nil)

	t.enter(innerHead)
	jNode := ir.GetPlace(jPlace)
	jGe0 := t.gen.PureInstr(resource.RuntimeFunctionGreaterOr, jNode, ir.Const(-1))
	cmpJ := elem.getCompare(t.gen, jNode)
	jGtKey := t.gen.PureInstr(resource.RuntimeFunctionGreater, cmpJ, ir.GetPlace(keyValPlace))
	innerCond := t.gen.PureInstr(resource.RuntimeFunctionAnd, jGe0, jGtKey)
	innerHead.Test = innerCond
	innerHead.ConnectTo(innerExit, ir.Cond(0))
	innerHead.ConnectTo(innerBody, nil)

	t.enter(innerBody)
	jPlus1 := t.gen.PureInstr(resource.RuntimeFunctionAdd, jNode, ir.Const(1))
	t.emit(elem.set(t.gen, jPlus1, cmpJ))
	jMinus1 := t.gen.PureInstr(resource.RuntimeFunctionSubtract, jNode, ir.Const(1))
	t.emit(t.gen.SetPlace(jPlace, jMinus1))
	t.current.ConnectTo(innerHead, nil)

	t.enter(innerExit)
	jFinal := ir.GetPlace(jPlace)
	jPlus1Final := t.gen.PureInstr(resource.RuntimeFunctionAdd, jFinal, ir.Const(1))
	t.emit(elem.set(t.gen, jPlus1Final, ir.GetPlace(keyPlace)))
	iPlus1 := t.gen.PureInstr(resource.RuntimeFunctionAdd, iNode, ir.Const(1))
	t.emit(t.gen.SetPlace(iPlace, iPlus1))
	t.current.ConnectTo(outerHead, nil)

	t.enter(outerExit)
}

// ── VarArray.sort() — in-place insertion sort on backing array ──────────────

// varArraySortCI sorts the container in-place using the shared insertion sort
// skeleton. Elements are accessed through the container's backing temp block.
func varArraySortCI(t *tracer, ci *containerInfo, v Num, args []Num) (Num, error) {
	_ = v
	_ = args
	emitInsertionSort(t, ci.readSize(), sortElemAccess{
		getCompare: func(gen *ir.IDGen, idx ir.Node) ir.Node {
			return ir.GetPlace(ci.elemPlace(gen, idx))
		},
		set: func(gen *ir.IDGen, idx ir.Node, val ir.Node) ir.Node {
			return gen.SetPlace(ci.elemPlace(gen, idx), val)
		},
	})
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
	return exprNum(ir.GetPlace(ir.BlockPlace{Block: idxTB, Index: ir.Const(0), Offset: 0})), nil
}

// emitEntitySort performs insertion sort on the entity index array, comparing
// entities by their sort key field value (read from EntityMemoryBlock). Uses
// the shared insertion sort skeleton with distinct raw/comparison accessors.
func emitEntitySort(t *tracer, idxTB *ir.TempBlock, countTB *ir.TempBlock, sortKeyOff int) error {
	countNode := ir.GetPlace(ir.BlockPlace{Block: countTB, Index: ir.Const(0), Offset: 0})
	emitInsertionSort(t, countNode, sortElemAccess{
		getCompare: func(gen *ir.IDGen, idx ir.Node) ir.Node {
			idxVal := ir.GetPlace(ir.BlockPlace{Block: idxTB, Index: idx, Offset: 0})
			return ir.GetPlace(ir.BlockPlace{
				Block: ir.Const(ir.BlockEntityMemory),
				Index: gen.PureInstr(resource.RuntimeFunctionAdd, idxVal, ir.Const(sortKeyOff)),
				Offset: 0,
			})
		},
		getRaw: func(gen *ir.IDGen, idx ir.Node) ir.Node {
			return ir.GetPlace(ir.BlockPlace{Block: idxTB, Index: idx, Offset: 0})
		},
		set: func(gen *ir.IDGen, idx ir.Node, val ir.Node) ir.Node {
			return gen.SetPlace(ir.BlockPlace{Block: idxTB, Index: idx, Offset: 0}, val)
		},
	})
	return nil
}
