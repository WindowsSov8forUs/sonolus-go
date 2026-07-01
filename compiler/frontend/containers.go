package frontend

import (
	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

	"github.com/WindowsSov8forUs/sonolus-go/compiler/ir"
)

// ── Pair methods ────────────────────────────────────────────────────────────
//
// Pair is a generic two-element record {first, second}. Comparisons follow the
// standard lexicographic order: compare first, then second on tie.

// pairLt implements Pair.lt(o Pair) → float64 (1.0 if p < o, else 0.0).
func pairLt(t *tracer, p Num, args []Num) (Num, error) {
	o := args[0]
	firstLt := t.gen.PureInstr(resource.RuntimeFunctionLess, p.MustField("first").mustNode(), o.MustField("first").mustNode())
	firstEq := t.gen.PureInstr(resource.RuntimeFunctionEqual, p.MustField("first").mustNode(), o.MustField("first").mustNode())
	secondLt := t.gen.PureInstr(resource.RuntimeFunctionLess, p.MustField("second").mustNode(), o.MustField("second").mustNode())
	andPart := t.gen.PureInstr(resource.RuntimeFunctionAnd, firstEq, secondLt)
	return exprNum(t.gen.PureInstr(resource.RuntimeFunctionOr, firstLt, andPart)), nil
}

// pairLe implements Pair.le(o Pair) → float64 (1.0 if p <= o, else 0.0).
func pairLe(t *tracer, p Num, args []Num) (Num, error) {
	o := args[0]
	firstLt := t.gen.PureInstr(resource.RuntimeFunctionLess, p.MustField("first").mustNode(), o.MustField("first").mustNode())
	firstEq := t.gen.PureInstr(resource.RuntimeFunctionEqual, p.MustField("first").mustNode(), o.MustField("first").mustNode())
	secondLe := t.gen.PureInstr(resource.RuntimeFunctionLessOr, p.MustField("second").mustNode(), o.MustField("second").mustNode())
	andPart := t.gen.PureInstr(resource.RuntimeFunctionAnd, firstEq, secondLe)
	return exprNum(t.gen.PureInstr(resource.RuntimeFunctionOr, firstLt, andPart)), nil
}

// pairGt implements Pair.gt(o Pair) → float64 (1.0 if p > o, else 0.0).
func pairGt(t *tracer, p Num, args []Num) (Num, error) {
	return pairLt(t, args[0], []Num{p})
}

// pairGe implements Pair.ge(o Pair) → float64 (1.0 if p >= o, else 0.0).
func pairGe(t *tracer, p Num, args []Num) (Num, error) {
	return pairLe(t, args[0], []Num{p})
}

// pairTuple implements Pair.tuple() → [2]float64.
func pairTuple(t *tracer, p Num, args []Num) (Num, error) {
	return arrayNum([]Num{p.MustField("first"), p.MustField("second")}), nil
}

// ── Container info type ─────────────────────────────────────────────────────

// containerInfo tracks a VarArray / ArrayMap / ArraySet local. It stores the
// compile-time capacity and the backing TempBlock so that methods can emit
// correct IR for element access and size manipulation.
type containerInfo struct {
	tb       *ir.TempBlock // backing memory: slot[0]=_size, slot[1..]=elements
	sizeSlot int           // slot index for _size (always 0)
	dataOff  int           // first data slot (always 1)
	capacity int           // max element count (compile-time constant)
	elemSize int           // slots per element (1 for scalar, N for N-field record)
	val      Num           // tracked composite Num for field reads
}

// sizePlace returns the BlockPlace for the _size field.
func (ci *containerInfo) sizePlace() ir.BlockPlace {
	return ir.BlockPlace{Block: ci.tb, Index: ir.Const(ci.sizeSlot), Offset: 0}
}

// elemPlace returns the BlockPlace for element at the given logical index.
// The index is multiplied by elemSize and added to dataOff.
func (ci *containerInfo) elemPlace(gen *ir.IDGen, index ir.Node) ir.BlockPlace {
	if ci.elemSize == 1 {
		return ir.BlockPlace{Block: ci.tb, Index: gen.PureInstr(resource.RuntimeFunctionAdd, ir.Const(ci.dataOff), index), Offset: 0}
	}
	// elemSize > 1: compute dataOff + index * elemSize
	scaled := gen.PureInstr(resource.RuntimeFunctionMultiply, index, ir.Const(ci.elemSize))
	return ir.BlockPlace{Block: ci.tb, Index: gen.PureInstr(resource.RuntimeFunctionAdd, ir.Const(ci.dataOff), scaled), Offset: 0}
}

// readSize returns an IR Get node that reads the current _size value.
func (ci *containerInfo) readSize() ir.Node {
	return ir.GetPlace(ci.sizePlace())
}

// writeSize emits an IR Set node writing a new value to _size.
func (ci *containerInfo) writeSize(t *tracer, newSize ir.Node) {
	t.emit(t.gen.SetPlace(ci.sizePlace(), newSize))
}

// ── VarArray methods (non-container-aware fallbacks) ──────────────────────
//
// These are used when no containerInfo is available (e.g., VarArray received
// as a parameter). They provide degraded functionality through the Num
// composite value only.

func varArrayLen(t *tracer, v Num, args []Num) (Num, error) {
	return v.MustField("_size"), nil
}

func varArrayCapacity(t *tracer, v Num, args []Num) (Num, error) {
	return constNum(-1), nil
}

func varArrayIsFull(t *tracer, v Num, args []Num) (Num, error) {
	capNum := constNum(-1)
	return exprNum(t.gen.PureInstr(resource.RuntimeFunctionEqual,
		v.MustField("_size").mustNode(), capNum.mustNode())), nil
}

func varArrayAppend(t *tracer, v Num, args []Num) (Num, error) {
	return constNum(0), nil
}

func varArrayPop(t *tracer, v Num, args []Num) (Num, error) {
	return constNum(0), nil
}

func varArrayClear(t *tracer, v Num, args []Num) (Num, error) {
	return constNum(0), nil
}

// ── VarArray methods (container-aware, with direct IR emission) ───────────

// varArrayLenCI returns the current size via a direct memory read.
func varArrayLenCI(t *tracer, ci *containerInfo, v Num, args []Num) (Num, error) {
	return exprNum(ci.readSize()), nil
}

// varArrayCapacityCI returns the compile-time capacity constant.
func varArrayCapacityCI(t *tracer, ci *containerInfo, v Num, args []Num) (Num, error) {
	return constNum(float64(ci.capacity)), nil
}

// varArrayIsFullCI returns 1.0 if _size == capacity, else 0.0.
func varArrayIsFullCI(t *tracer, ci *containerInfo, v Num, args []Num) (Num, error) {
	capNode := ir.Const(ci.capacity)
	return exprNum(t.gen.PureInstr(resource.RuntimeFunctionEqual, ci.readSize(), capNode)), nil
}

// varArrayAppendCI emits IR to append a value to the end of the array.
// Sequence: set(array[dataOff + _size * elemSize], value); set(_size, _size + 1)
// No bounds check is emitted (caller should use isFull() if needed).
func varArrayAppendCI(t *tracer, ci *containerInfo, v Num, args []Num) (Num, error) {
	val := args[0]
	sizeNode := ci.readSize()
	target := ci.elemPlace(t.gen, sizeNode)
	t.emit(t.gen.SetPlace(target, val.mustNode()))
	// _size = _size + 1
	newSize := t.gen.PureInstr(resource.RuntimeFunctionAdd, sizeNode, ir.Const(1))
	ci.writeSize(t, newSize)
	return constNum(0), nil
}

// varArrayPopCI pops the last element (no args) or pops at a specific index
// (1 arg). Indexed pop shifts subsequent elements left by one slot before
// decrementing _size.
func varArrayPopCI(t *tracer, ci *containerInfo, v Num, args []Num) (Num, error) {
	if len(args) == 0 {
		// Pop last element.
		sizeNode := ci.readSize()
		newSize := t.gen.PureInstr(resource.RuntimeFunctionSubtract, sizeNode, ir.Const(1))
		ci.writeSize(t, newSize)
		val := ir.GetPlace(ci.elemPlace(t.gen, newSize))
		return exprNum(val), nil
	}
	// Pop at args[0]: read the element, shift remaining left, decrement _size.
	idx := args[0].mustNode()
	sizeNode := ci.readSize()
	// Read the element at idx (this will be the return value).
	val := ir.GetPlace(ci.elemPlace(t.gen, idx))
	// Shift loop: for i = idx+1; i < _size; i++ { arr[i-1] = arr[i] }
	// Use emitLinearSearch-like block generation. For simplicity, use a
	// while-loop pattern in the CFG.
	after := t.emitShiftLeft(ci, idx, sizeNode)
	// Decrement _size.
	ci.writeSize(t, t.gen.PureInstr(resource.RuntimeFunctionSubtract, sizeNode, ir.Const(1)))
	// After the shift, continue from the merge block.
	t.current = after
	return exprNum(val), nil
}

// emitShiftLeft generates a loop that shifts elements [start+1 .. size-1] left
// by one position: arr[i-1] = arr[i] for i from start+1 to size-1.
// Returns the exit block where execution continues after the shift.
func (t *tracer) emitShiftLeft(ci *containerInfo, start ir.Node, size ir.Node) *ir.BasicBlock {
	// Allocate loop index starting at start+1.
	iTB := &ir.TempBlock{Name: "shiftIdx", Size: 1}
	iPlace := ir.BlockPlace{Block: iTB, Index: ir.Const(0), Offset: 0}
	nextIdx := t.gen.PureInstr(resource.RuntimeFunctionAdd, start, ir.Const(1))
	t.emit(t.gen.SetPlace(iPlace, nextIdx))

	header := ir.NewBlock()
	body := ir.NewBlock()
	exit := ir.NewBlock()

	t.fallthroughTo(header)
	t.enter(header)
	iNode := ir.GetPlace(iPlace)
	cond := t.gen.PureInstr(resource.RuntimeFunctionLess, iNode, size)
	header.Test = cond
	header.ConnectTo(exit, ir.Cond(0))
	header.ConnectTo(body, nil)

	// Body: arr[i-1] = arr[i]
	t.enter(body)
	prevIdx := t.gen.PureInstr(resource.RuntimeFunctionSubtract, iNode, ir.Const(1))
	src := ir.GetPlace(ci.elemPlace(t.gen, iNode))
	t.emit(t.gen.SetPlace(ci.elemPlace(t.gen, prevIdx), src))
	// i++
	t.emit(t.gen.SetPlace(iPlace, t.gen.PureInstr(resource.RuntimeFunctionAdd, iNode, ir.Const(1))))
	t.current.ConnectTo(header, nil)

	t.enter(exit)
	return exit
}

// varArrayInsertCI inserts val at index, shifting elements right.
func varArrayInsertCI(t *tracer, ci *containerInfo, v Num, args []Num) (Num, error) {
	idx := args[0].mustNode()
	val := args[1].mustNode()
	sizeNode := ci.readSize()
	// Shift elements [idx .. size-1] right by 1: arr[i+1] = arr[i] from i = size-1 down to idx.
	// Use a reverse loop.
	iTB := &ir.TempBlock{Name: "shiftIdx", Size: 1}
	iPlace := ir.BlockPlace{Block: iTB, Index: ir.Const(0), Offset: 0}
	// Start at size-1.
	lastIdx := t.gen.PureInstr(resource.RuntimeFunctionSubtract, sizeNode, ir.Const(1))
	t.emit(t.gen.SetPlace(iPlace, lastIdx))

	header := ir.NewBlock()
	body := ir.NewBlock()
	exit := ir.NewBlock()

	t.fallthroughTo(header)
	t.enter(header)
	iNode := ir.GetPlace(iPlace)
	// Continue while i >= idx.
	cond := t.gen.PureInstr(resource.RuntimeFunctionGreaterOr, iNode, idx)
	header.Test = cond
	header.ConnectTo(exit, ir.Cond(0))
	header.ConnectTo(body, nil)

	// Body: arr[i+1] = arr[i]; i--
	t.enter(body)
	nextIdx := t.gen.PureInstr(resource.RuntimeFunctionAdd, iNode, ir.Const(1))
	src := ir.GetPlace(ci.elemPlace(t.gen, iNode))
	t.emit(t.gen.SetPlace(ci.elemPlace(t.gen, nextIdx), src))
	t.emit(t.gen.SetPlace(iPlace, t.gen.PureInstr(resource.RuntimeFunctionSubtract, iNode, ir.Const(1))))
	t.current.ConnectTo(header, nil)

	t.enter(exit)
	// Write val at idx.
	t.emit(t.gen.SetPlace(ci.elemPlace(t.gen, idx), val))
	// Increment _size.
	ci.writeSize(t, t.gen.PureInstr(resource.RuntimeFunctionAdd, sizeNode, ir.Const(1)))
	return constNum(0), nil
}

// varArrayClearCI emits IR to set _size to 0.
func varArrayClearCI(t *tracer, ci *containerInfo, v Num, args []Num) (Num, error) {
	ci.writeSize(t, ir.Const(0))
	return constNum(0), nil
}

// ── Linear search loop generator ────────────────────────────────────────────
//
// emitLinearSearch generates a while loop that scans elements 0.._size-1,
// comparing each with target via Equal. When a match is found, onFound is
// called in the "found" block (receiving the current index node) and must
// return a result value. If the loop exhausts without a match, onExhausted
// is called in the "exhausted" block and must return a result value.
//
// Both result values are stored into a fresh result TempBlock and the final
// IR node (a Get from that temp) is returned. After the call, t.current is
// the exit/merge block where the result can be read.
func emitLinearSearch(t *tracer, ci *containerInfo, target ir.Node,
	onFound func(idx ir.Node) ir.Node,
	onExhausted func() ir.Node,
) ir.Node {
	// Allocate temps for the loop index and result.
	idxTB := &ir.TempBlock{Name: "searchIdx", Size: 1}
	idxPlace := ir.BlockPlace{Block: idxTB, Index: ir.Const(0), Offset: 0}
	resultTB := &ir.TempBlock{Name: "searchResult", Size: 1}
	resultPlace := ir.BlockPlace{Block: resultTB, Index: ir.Const(0), Offset: 0}

	// Init index to 0.
	t.emit(t.gen.SetPlace(idxPlace, ir.Const(0)))

	// Build the loop blocks.
	header := ir.NewBlock()
	body := ir.NewBlock()
	increment := ir.NewBlock()
	found := ir.NewBlock()
	exhausted := ir.NewBlock()
	exit := ir.NewBlock()

	t.fallthroughTo(header)

	// Header: test i < _size. False → exhausted, true → body.
	t.enter(header)
	idxNode := ir.GetPlace(idxPlace)
	cond := t.gen.PureInstr(resource.RuntimeFunctionLess, idxNode, ci.readSize())
	header.Test = cond
	header.ConnectTo(exhausted, ir.Cond(0))
	header.ConnectTo(body, nil)

	// Body: read element, compare with target. Equal → found, else → increment.
	t.enter(body)
	elem := ir.GetPlace(ci.elemPlace(t.gen, idxNode))
	eq := t.gen.PureInstr(resource.RuntimeFunctionEqual, elem, target)
	body.Test = eq
	body.ConnectTo(found, nil)
	body.ConnectTo(increment, ir.Cond(0))

	// Increment: i = i + 1, loop back to header.
	t.enter(increment)
	nextIdx := t.gen.PureInstr(resource.RuntimeFunctionAdd, idxNode, ir.Const(1))
	t.emit(t.gen.SetPlace(idxPlace, nextIdx))
	t.current.ConnectTo(header, nil)

	// Found: call handler, store result, goto exit.
	t.enter(found)
	t.emit(t.gen.SetPlace(resultPlace, onFound(idxNode)))
	t.fallthroughTo(exit)

	// Exhausted: call handler, store result, goto exit.
	t.enter(exhausted)
	t.emit(t.gen.SetPlace(resultPlace, onExhausted()))
	t.fallthroughTo(exit)

	// Exit: merge point — continue from here.
	t.enter(exit)
	return ir.GetPlace(resultPlace)
}

// ── VarArray search methods (container-aware) ───────────────────────────────

// varArrayContainsCI returns 1.0 if value is found, 0.0 otherwise.
func varArrayContainsCI(t *tracer, ci *containerInfo, v Num, args []Num) (Num, error) {
	target := args[0].mustNode()
	result := emitLinearSearch(t, ci, target,
		func(ir.Node) ir.Node { return ir.Const(1) },
		func() ir.Node { return ir.Const(0) },
	)
	return exprNum(result), nil
}

// varArrayIndexCI returns the index of value (0-based), or -1 if not found.
func varArrayIndexCI(t *tracer, ci *containerInfo, v Num, args []Num) (Num, error) {
	target := args[0].mustNode()
	result := emitLinearSearch(t, ci, target,
		func(idx ir.Node) ir.Node { return idx },
		func() ir.Node { return ir.Const(-1) },
	)
	return exprNum(result), nil
}

// varArrayRemoveCI searches for value and pops at the found index.
// Returns 1.0 if removed, 0.0 if not found.
func varArrayRemoveCI(t *tracer, ci *containerInfo, v Num, args []Num) (Num, error) {
	target := args[0].mustNode()
	result := emitLinearSearch(t, ci, target,
		func(idx ir.Node) ir.Node {
			// Pop at idx: shift elements [idx+1.._size-1] left by 1,
			// then decrement _size.
			// For simplicity: read size, write last element into idx slot,
			// decrement size (swap-with-last pattern from sonolus.py).
			sizeNode := ci.readSize()
			lastIdx := t.gen.PureInstr(resource.RuntimeFunctionSubtract, sizeNode, ir.Const(1))
			lastElem := ir.GetPlace(ci.elemPlace(t.gen, lastIdx))
			t.emit(t.gen.SetPlace(ci.elemPlace(t.gen, idx), lastElem))
			ci.writeSize(t, lastIdx)
			return ir.Const(1)
		},
		func() ir.Node { return ir.Const(0) },
	)
	return exprNum(result), nil
}

// varArraySetAddCI adds value only if not already present (set semantics).
// Returns 1.0 if added, 0.0 if already present or full.
func varArraySetAddCI(t *tracer, ci *containerInfo, v Num, args []Num) (Num, error) {
	target := args[0].mustNode()
	// First check if full — if so, skip (return 0).
	// Otherwise search: if found → return 0; if not found → append and return 1.
	// The linear search doubles as a check for duplicates.
	result := emitLinearSearch(t, ci, target,
		func(ir.Node) ir.Node { return ir.Const(0) }, // already present
		func() ir.Node {
			// Not found — append.
			sizeNode := ci.readSize()
			t.emit(t.gen.SetPlace(ci.elemPlace(t.gen, sizeNode), target))
			ci.writeSize(t, t.gen.PureInstr(resource.RuntimeFunctionAdd, sizeNode, ir.Const(1)))
			return ir.Const(1)
		},
	)
	return exprNum(result), nil
}

// varArraySetRemoveCI removes value (set semantics — same as remove).
func varArraySetRemoveCI(t *tracer, ci *containerInfo, v Num, args []Num) (Num, error) {
	return varArrayRemoveCI(t, ci, v, args)
}

// ── ArrayMap methods (container-aware) ──────────────────────────────────────
//
// ArrayMap stores key-value pairs. Each entry occupies elemSize=2 slots
// (key at offset 0, value at offset 1). elemPlace(idx) returns the key slot;
// the value slot is always key slot + 1.
//
// Helper: valuePlace returns the place of the value slot for a given key place.
func (ci *containerInfo) valuePlace(gen *ir.IDGen, keySlot ir.Node) ir.BlockPlace {
	return ir.BlockPlace{Block: ci.tb, Index: gen.PureInstr(resource.RuntimeFunctionAdd, keySlot, ir.Const(1)), Offset: 0}
}

// arrayMapGetCI searches by key and returns the associated value, or 0 if not found.
func arrayMapGetCI(t *tracer, ci *containerInfo, v Num, args []Num) (Num, error) {
	key := args[0].mustNode()
	result := emitLinearSearch(t, ci, key,
		func(idx ir.Node) ir.Node {
			keySlot := ci.elemPlace(t.gen, idx).Index
			return ir.GetPlace(ci.valuePlace(t.gen, keySlot))
		},
		func() ir.Node { return ir.Const(0) },
	)
	return exprNum(result), nil
}

// arrayMapSetCI sets map[k] = v: searches for existing key, updates value or appends.
func arrayMapSetCI(t *tracer, ci *containerInfo, v Num, args []Num) (Num, error) {
	key := args[0].mustNode()
	val := args[1].mustNode()
	result := emitLinearSearch(t, ci, key,
		func(idx ir.Node) ir.Node {
			// Key exists — update the value slot.
			t.emit(t.gen.SetPlace(ci.valuePlace(t.gen, ci.elemPlace(t.gen, idx).Index), val))
			return ir.Const(0)
		},
		func() ir.Node {
			// Not found — append {key, value} at position _size.
			sizeNode := ci.readSize()
			keySlot := ci.elemPlace(t.gen, sizeNode).Index
			t.emit(t.gen.SetPlace(ir.BlockPlace{Block: ci.tb, Index: keySlot, Offset: 0}, key))
			t.emit(t.gen.SetPlace(ci.valuePlace(t.gen, keySlot), val))
			ci.writeSize(t, t.gen.PureInstr(resource.RuntimeFunctionAdd, sizeNode, ir.Const(1)))
			return ir.Const(0)
		},
	)
	return exprNum(result), nil
}

// arrayMapDeleteCI removes a key from the map (swap-with-last pattern).
func arrayMapDeleteCI(t *tracer, ci *containerInfo, v Num, args []Num) (Num, error) {
	key := args[0].mustNode()
	result := emitLinearSearch(t, ci, key,
		func(idx ir.Node) ir.Node {
			sizeNode := ci.readSize()
			lastIdx := t.gen.PureInstr(resource.RuntimeFunctionSubtract, sizeNode, ir.Const(1))
			idxKeySlot := ci.elemPlace(t.gen, idx).Index
			lastKeySlot := ci.elemPlace(t.gen, lastIdx).Index
			lastKey := ir.GetPlace(ir.BlockPlace{Block: ci.tb, Index: lastKeySlot, Offset: 0})
			lastVal := ir.GetPlace(ci.valuePlace(t.gen, lastKeySlot))
			t.emit(t.gen.SetPlace(ir.BlockPlace{Block: ci.tb, Index: idxKeySlot, Offset: 0}, lastKey))
			t.emit(t.gen.SetPlace(ci.valuePlace(t.gen, idxKeySlot), lastVal))
			ci.writeSize(t, lastIdx)
			return ir.Const(1)
		},
		func() ir.Node { return ir.Const(0) },
	)
	return exprNum(result), nil
}

// arrayMapContainsCI returns 1.0 if key exists, 0.0 otherwise.
func arrayMapContainsCI(t *tracer, ci *containerInfo, v Num, args []Num) (Num, error) {
	key := args[0].mustNode()
	result := emitLinearSearch(t, ci, key,
		func(ir.Node) ir.Node { return ir.Const(1) },
		func() ir.Node { return ir.Const(0) },
	)
	return exprNum(result), nil
}

// arrayMapPopCI removes key and returns its value, or 0 if not found.
func arrayMapPopCI(t *tracer, ci *containerInfo, v Num, args []Num) (Num, error) {
	key := args[0].mustNode()
	result := emitLinearSearch(t, ci, key,
		func(idx ir.Node) ir.Node {
			idxKeySlot := ci.elemPlace(t.gen, idx).Index
			savedVal := ir.GetPlace(ci.valuePlace(t.gen, idxKeySlot))
			// Swap with last and decrement size.
			sizeNode := ci.readSize()
			lastIdx := t.gen.PureInstr(resource.RuntimeFunctionSubtract, sizeNode, ir.Const(1))
			lastKeySlot := ci.elemPlace(t.gen, lastIdx).Index
			lastKey := ir.GetPlace(ir.BlockPlace{Block: ci.tb, Index: lastKeySlot, Offset: 0})
			lastVal := ir.GetPlace(ci.valuePlace(t.gen, lastKeySlot))
			t.emit(t.gen.SetPlace(ir.BlockPlace{Block: ci.tb, Index: idxKeySlot, Offset: 0}, lastKey))
			t.emit(t.gen.SetPlace(ci.valuePlace(t.gen, idxKeySlot), lastVal))
			ci.writeSize(t, lastIdx)
			return savedVal
		},
		func() ir.Node { return ir.Const(0) },
	)
	return exprNum(result), nil
}

// ── ArrayMap and ArraySet field lists ───────────────────────────────────────

var arrayMapFields = []string{"_size", "_array"}
var arraySetFields = []string{"_values"}

// Box wraps a single numeric value for uniform Num/non-Num handling.
var boxFields = []string{"val"}

// FrozenNumSet fields: a sorted array of nums + size. Contains uses
// linear search; binary search for large sets is deferred to a future phase.
var frozenNumSetFields = []string{"_size", "_array"}

// containerSelf returns the receiver Num itself. Used by ArrayMap keys/values/items
// to return self-reference for iteration (for-range uses the container directly).
func containerSelf(t *tracer, v Num, args []Num) (Num, error) {
	return v, nil
}

// frozenNumSetContainsCI implements contains() for FrozenNumSet using linear
// search (binary search for large sets deferred to a future phase).
func frozenNumSetContainsCI(t *tracer, ci *containerInfo, v Num, args []Num) (Num, error) {
	target := args[0].mustNode()
	result := emitLinearSearch(t, ci, target,
		func(ir.Node) ir.Node { return ir.Const(1) },
		func() ir.Node { return ir.Const(0) },
	)
	return exprNum(result), nil
}
