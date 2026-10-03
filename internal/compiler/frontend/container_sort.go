package frontend

import (
	"go/ast"
	"go/types"
	"math/bits"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/ir"
)

// mergeSortContainerValue emits a stable, in-place divide-and-rotate merge
// sort. Processing the smaller subproblem first bounds the pending stack by
// log2(capacity), independent of element width. No full-array scratch is used.
func (l *lowerer) mergeSortContainerValue(n *ast.CallExpr, receiver lowerValue, comparator *staticCallable) lowerValue {
	c := receiver.container
	integer := types.Typ[types.Int]
	constant := func(v int) ir.Expr { return ir.Const{Value: float64(v)} }
	op := func(fn resource.RuntimeFunction, args ...ir.Expr) ir.Expr { return l.pure(fn, n, args...) }
	add := func(a, b ir.Expr) ir.Expr { return op(resource.RuntimeFunctionAdd, a, b) }
	sub := func(a, b ir.Expr) ir.Expr { return op(resource.RuntimeFunctionSubtract, a, b) }
	half := func(v ir.Expr) ir.Expr {
		return op(resource.RuntimeFunctionFloor, op(resource.RuntimeFunctionDivide, v, constant(2)))
	}
	set := func(v lowerValue, expr ir.Expr) { l.store(v, scalarValue(expr, integer), n) }
	local := func(name string) lowerValue { return l.alloc("container.sort."+name, integer) }
	element := func(index ir.Expr) lowerValue { return l.containerElement(n, c, index, 0, c.stride, c.element) }
	less := func(left, right ir.Expr) ir.Expr {
		a := l.materialize("sort.left", element(left), n)
		b := l.materialize("sort.right", element(right), n)
		typed := *comparator
		typed.resultType = types.Typ[types.Bool]
		result := l.inlineStaticCallable(n, &typed, []callArgument{{value: a}, {value: b}})
		if len(result.slots) != 1 {
			l.errorAt(n, "VarArray.SortFunc comparator must return bool")
			return ir.Const{}
		}
		return result.slots[0]
	}
	swap := func(a, b ir.Expr) {
		value := l.materialize("sort.swap", element(a), n)
		l.store(element(a), element(b), n)
		l.store(element(b), value, n)
	}
	width, start, size := local("width"), local("start"), local("size")
	a, middle, end := local("first"), local("middle"), local("end")
	cut1, cut2, newMiddle := local("cut1"), local("cut2"), local("newMiddle")
	low, high, probe := local("low"), local("high"), local("probe")
	depth := local("depth")
	stackCapacity := bits.Len(uint(c.capacity)) + 1
	stack := l.builder.NewLocal("sort.stack", ir.Type{Name: "sort.stack", Slots: 3 * stackCapacity})
	base := ir.Places(stack)[0].(ir.LocalPlace)
	stackCell := func(offset int) lowerValue {
		place := l.indexedLocal(base, depth.slots[0], stackCapacity, 3, offset, n)
		return lowerValue{type_: integer, slots: []ir.Expr{ir.Load{Place: place}}, places: []ir.Place{place}}
	}
	push := func(first, mid, last ir.Expr) {
		set(stackCell(0), first)
		set(stackCell(1), mid)
		set(stackCell(2), last)
		set(depth, add(depth.slots[0], constant(1)))
	}
	set(size, receiver.slots[0])
	set(width, constant(1))
	passes, pass, runs, run, work, inspect, pair, split := l.newBlock(), l.newBlock(), l.newBlock(), l.newBlock(), l.newBlock(), l.newBlock(), l.newBlock(), l.newBlock()
	leftSplit, rightSplit, rotate, choose, keepLeft, keepRight := l.newBlock(), l.newBlock(), l.newBlock(), l.newBlock(), l.newBlock(), l.newBlock()
	pop, take, nextRun, nextPass, exit := l.newBlock(), l.newBlock(), l.newBlock(), l.newBlock(), l.newBlock()
	l.jump(passes)
	l.setCurrent(passes)
	_ = l.builder.Branch(op(resource.RuntimeFunctionLess, width.slots[0], size.slots[0]), pass, exit)
	l.setCurrent(pass)
	set(start, constant(0))
	l.jump(runs)
	l.setCurrent(runs)
	_ = l.builder.Branch(op(resource.RuntimeFunctionLess, add(start.slots[0], width.slots[0]), size.slots[0]), run, nextPass)
	l.setCurrent(run)
	set(a, start.slots[0])
	set(middle, add(start.slots[0], width.slots[0]))
	set(end, op(resource.RuntimeFunctionMin, add(middle.slots[0], width.slots[0]), size.slots[0]))
	set(depth, constant(0))
	l.jump(work)
	l.setCurrent(work)
	_ = l.builder.Branch(op(resource.RuntimeFunctionAnd,
		op(resource.RuntimeFunctionLess, a.slots[0], middle.slots[0]),
		op(resource.RuntimeFunctionLess, middle.slots[0], end.slots[0])), inspect, pop)
	l.setCurrent(inspect)
	// Already ordered runs need neither searches nor rotations.
	ordered := less(middle.slots[0], sub(middle.slots[0], constant(1)))
	checkPair := l.newBlock()
	_ = l.builder.Branch(ordered, checkPair, pop)
	l.setCurrent(checkPair)
	_ = l.builder.Branch(op(resource.RuntimeFunctionEqual, sub(end.slots[0], a.slots[0]), constant(2)), pair, split)
	l.setCurrent(pair)
	swap(a.slots[0], middle.slots[0])
	l.jump(pop)
	l.setCurrent(split)
	_ = l.builder.Branch(op(resource.RuntimeFunctionGreater, sub(middle.slots[0], a.slots[0]), sub(end.slots[0], middle.slots[0])), leftSplit, rightSplit)
	// A pivot from the left run uses lower_bound in the right run. A pivot
	// from the right uses upper_bound in the left, preserving equal-key order.
	search := func(leftPivot bool) {
		header, body, advance, retreat, done := l.newBlock(), l.newBlock(), l.newBlock(), l.newBlock(), l.newBlock()
		l.jump(header)
		l.setCurrent(header)
		_ = l.builder.Branch(op(resource.RuntimeFunctionLess, low.slots[0], high.slots[0]), body, done)
		l.setCurrent(body)
		set(probe, add(low.slots[0], half(sub(high.slots[0], low.slots[0]))))
		if leftPivot {
			_ = l.builder.Branch(less(probe.slots[0], cut1.slots[0]), advance, retreat)
		} else {
			_ = l.builder.Branch(less(cut2.slots[0], probe.slots[0]), retreat, advance)
		}
		l.setCurrent(advance)
		set(low, add(probe.slots[0], constant(1)))
		l.jump(header)
		l.setCurrent(retreat)
		set(high, probe.slots[0])
		l.jump(header)
		l.setCurrent(done)
	}
	l.setCurrent(leftSplit)
	set(cut1, add(a.slots[0], half(sub(middle.slots[0], a.slots[0]))))
	set(low, middle.slots[0])
	set(high, end.slots[0])
	search(true)
	set(cut2, low.slots[0])
	l.jump(rotate)
	l.setCurrent(rightSplit)
	set(cut2, add(middle.slots[0], half(sub(end.slots[0], middle.slots[0]))))
	set(low, a.slots[0])
	set(high, middle.slots[0])
	search(false)
	set(cut1, low.slots[0])
	l.jump(rotate)
	l.setCurrent(rotate)
	set(newMiddle, add(cut1.slots[0], sub(cut2.slots[0], middle.slots[0])))
	rotation := l.newBlock()
	_ = l.builder.Branch(op(resource.RuntimeFunctionAnd,
		op(resource.RuntimeFunctionLess, cut1.slots[0], middle.slots[0]),
		op(resource.RuntimeFunctionLess, middle.slots[0], cut2.slots[0])), rotation, choose)
	l.setCurrent(rotation)
	reverse := func(first, last ir.Expr) {
		set(low, first)
		set(high, sub(last, constant(1)))
		header, body, done := l.newBlock(), l.newBlock(), l.newBlock()
		l.jump(header)
		l.setCurrent(header)
		_ = l.builder.Branch(op(resource.RuntimeFunctionLess, low.slots[0], high.slots[0]), body, done)
		l.setCurrent(body)
		swap(low.slots[0], high.slots[0])
		set(low, add(low.slots[0], constant(1)))
		set(high, sub(high.slots[0], constant(1)))
		l.jump(header)
		l.setCurrent(done)
	}
	reverse(cut1.slots[0], middle.slots[0])
	reverse(middle.slots[0], cut2.slots[0])
	reverse(cut1.slots[0], cut2.slots[0])
	l.jump(choose)
	l.setCurrent(choose)
	_ = l.builder.Branch(op(resource.RuntimeFunctionLessOr, sub(newMiddle.slots[0], a.slots[0]), sub(end.slots[0], newMiddle.slots[0])), keepLeft, keepRight)
	l.setCurrent(keepLeft)
	push(newMiddle.slots[0], cut2.slots[0], end.slots[0])
	set(middle, cut1.slots[0])
	set(end, newMiddle.slots[0])
	l.jump(work)
	l.setCurrent(keepRight)
	push(a.slots[0], cut1.slots[0], newMiddle.slots[0])
	set(a, newMiddle.slots[0])
	set(middle, cut2.slots[0])
	l.jump(work)
	l.setCurrent(pop)
	_ = l.builder.Branch(op(resource.RuntimeFunctionGreater, depth.slots[0], constant(0)), take, nextRun)
	l.setCurrent(take)
	set(depth, sub(depth.slots[0], constant(1)))
	set(a, stackCell(0).slots[0])
	set(middle, stackCell(1).slots[0])
	set(end, stackCell(2).slots[0])
	l.jump(work)
	l.setCurrent(nextRun)
	set(start, add(start.slots[0], op(resource.RuntimeFunctionMultiply, width.slots[0], constant(2))))
	l.jump(runs)
	l.setCurrent(nextPass)
	set(width, op(resource.RuntimeFunctionMultiply, width.slots[0], constant(2)))
	l.jump(passes)
	l.setCurrent(exit)
	return lowerValue{}
}
