package snode

// --- If (src/snode/optimize/If.ts) ---

func optimizeIf(s Func) SNode {
	if len(s.Args) > 2 && isValueEq(s.Args[2], 0) {
		return Peephole(Func{Op: OpAnd, Args: []SNode{s.Args[0], s.Args[1]}})
	}
	return s
}

// --- Get (src/snode/optimize/Get.ts) ---

func optimizeGet(s Func) SNode {
	if len(s.Args) < 2 {
		return s
	}
	id, index := s.Args[0], s.Args[1]

	if add, ok := asFunc(index, OpAdd); ok && len(add.Args) == 2 {
		if mul, ok := asFunc(add.Args[1], OpMultiply); ok && len(mul.Args) == 2 {
			return Peephole(Func{Op: OpGetShifted, Args: []SNode{id, add.Args[0], mul.Args[0], mul.Args[1]}})
		}
	}
	return s
}

// --- GetShifted (src/snode/optimize/GetShifted.ts) ---

func optimizeGetShifted(s Func) SNode {
	if len(s.Args) < 4 {
		return s
	}
	id, x, y, sh := s.Args[0], s.Args[1], s.Args[2], s.Args[3]

	if yv, ok := asValue(y); ok {
		if sv, ok := asValue(sh); ok {
			if xv, ok := asValue(x); ok {
				return Peephole(Func{Op: OpGet, Args: []SNode{id, Value(xv + yv*sv)}})
			}
			if yv == 0 && sv == 0 {
				return Peephole(Func{Op: OpGet, Args: []SNode{id, x}})
			}
		}
	}
	return s
}
