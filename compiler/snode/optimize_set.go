package snode

import "github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

// --- Set (src/snode/optimize/Set.ts) ---

func optimizeSet(s Func) SNode {
	if len(s.Args) < 3 {
		return s
	}
	id, index, value := s.Args[0], s.Args[1], s.Args[2]

	if add, ok := asFunc(index, rfAdd); ok && len(add.Args) == 2 {
		if mul, ok := asFunc(add.Args[1], rfMultiply); ok && len(mul.Args) == 2 {
			return Peephole(Func{Op: rfSetShifted, Args: []SNode{id, add.Args[0], mul.Args[0], mul.Args[1], value}})
		}
	}

	if vf, ok := asFuncs(value, rfAdd, rfSubtract, rfMultiply, rfDivide, rfRem, rfMod, rfPower); ok && len(vf.Args) == 2 {
		if g, ok := asFunc(vf.Args[0], rfGet); ok && len(g.Args) >= 2 &&
			isEquivalent(g.Args[0], id) && isEquivalent(g.Args[1], index) {
			return Func{Op: setFunc(vf.Op), Args: []SNode{id, index, vf.Args[1]}}
		}
	}

	if vf, ok := asFuncs(value, rfAdd, rfMultiply); ok && len(vf.Args) == 2 {
		if g, ok := asFunc(vf.Args[1], rfGet); ok && len(g.Args) >= 2 &&
			isEquivalent(g.Args[0], id) && isEquivalent(g.Args[1], index) {
			return Func{Op: setFunc(vf.Op), Args: []SNode{id, index, vf.Args[0]}}
		}
	}

	return s
}

// --- SetShifted (src/snode/optimize/SetShifted.ts) ---

func optimizeSetShifted(s Func) SNode {
	if len(s.Args) < 5 {
		return s
	}
	id, x, y, sh, value := s.Args[0], s.Args[1], s.Args[2], s.Args[3], s.Args[4]

	if yv, ok := asValue(y); ok {
		if sv, ok := asValue(sh); ok {
			if xv, ok := asValue(x); ok {
				return Peephole(Func{Op: rfSet, Args: []SNode{id, Value(xv + yv*sv), value}})
			}
			if yv == 0 && sv == 0 {
				return Peephole(Func{Op: rfSet, Args: []SNode{id, x, value}})
			}
		}
	}

	if vf, ok := asFuncs(value, rfAdd, rfSubtract, rfMultiply, rfDivide, rfRem, rfMod, rfPower); ok && len(vf.Args) == 2 {
		if g, ok := asFunc(vf.Args[0], rfGetShifted); ok && len(g.Args) >= 4 &&
			isEquivalent(g.Args[0], id) && isEquivalent(g.Args[1], x) &&
			isEquivalent(g.Args[2], y) && isEquivalent(g.Args[3], sh) {
			return Func{Op: setShiftedFunc(vf.Op), Args: []SNode{id, x, y, sh, vf.Args[1]}}
		}
		if g, ok := asFunc(vf.Args[1], rfGetShifted); ok && len(g.Args) >= 4 &&
			isEquivalent(g.Args[0], id) && isEquivalent(g.Args[1], x) &&
			isEquivalent(g.Args[2], y) && isEquivalent(g.Args[3], sh) {
			return Func{Op: setShiftedFunc(vf.Op), Args: []SNode{id, x, y, sh, vf.Args[0]}}
		}
	}

	return s
}

// setFuncMap maps arithmetic runtime functions to their compound-assignment
// counterparts (e.g. Add → SetAdd). The table is exhaustive for the arithmetic
// ops that appear in the Set/SetShifted optimizer paths; string concatenation
// (matching the JS reference) is kept as a fallback for future op additions.
var setFuncMap = map[resource.RuntimeFunction]resource.RuntimeFunction{
	resource.RuntimeFunctionAdd:      resource.RuntimeFunctionSetAdd,
	resource.RuntimeFunctionSubtract: resource.RuntimeFunctionSetSubtract,
	resource.RuntimeFunctionMultiply: resource.RuntimeFunctionSetMultiply,
	resource.RuntimeFunctionDivide:   resource.RuntimeFunctionSetDivide,
	resource.RuntimeFunctionPower:    resource.RuntimeFunctionSetPower,
	resource.RuntimeFunctionMod:      resource.RuntimeFunctionSetMod,
	resource.RuntimeFunctionRem:      resource.RuntimeFunctionSetRem,
}

// setShiftedFuncMap mirrors setFuncMap for the Shifted variants.
var setShiftedFuncMap = map[resource.RuntimeFunction]resource.RuntimeFunction{
	resource.RuntimeFunctionAdd:      resource.RuntimeFunctionSetAddShifted,
	resource.RuntimeFunctionSubtract: resource.RuntimeFunctionSetSubtractShifted,
	resource.RuntimeFunctionMultiply: resource.RuntimeFunctionSetMultiplyShifted,
	resource.RuntimeFunctionDivide:   resource.RuntimeFunctionSetDivideShifted,
	resource.RuntimeFunctionPower:    resource.RuntimeFunctionSetPowerShifted,
	resource.RuntimeFunctionMod:      resource.RuntimeFunctionSetModShifted,
	resource.RuntimeFunctionRem:      resource.RuntimeFunctionSetRemShifted,
}

// setFunc maps an arithmetic function to its compound-assignment counterpart,
// e.g. Add -> SetAdd. Uses an explicit mapping table with a string-concat
// fallback for forward compatibility.
func setFunc(fn resource.RuntimeFunction) resource.RuntimeFunction {
	if m, ok := setFuncMap[fn]; ok {
		return m
	}
	return resource.RuntimeFunction("Set" + string(fn))
}

// setShiftedFunc mirrors setFunc for the Shifted variants.
func setShiftedFunc(fn resource.RuntimeFunction) resource.RuntimeFunction {
	if m, ok := setShiftedFuncMap[fn]; ok {
		return m
	}
	return resource.RuntimeFunction("Set" + string(fn) + "Shifted")
}
