package backend

import (
	"math"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
)

const maxSafeInteger = 9007199254740991

func simplify(node snode) snode {
	function, ok := node.(functionNode)
	if !ok {
		return node
	}
	args := make([]snode, len(function.args))
	for i, arg := range function.args {
		args[i] = simplify(arg)
	}
	function.args = args
	switch function.function {
	case resource.RuntimeFunctionAdd:
		return simplifyCommutative(function, 0, func(a, b float64) float64 { return a + b }, false)
	case resource.RuntimeFunctionSubtract:
		return simplifyNonCommutative(function, 0, func(a, b float64) float64 { return a + b })
	case resource.RuntimeFunctionMultiply:
		return simplifyCommutative(function, 1, func(a, b float64) float64 { return a * b }, true)
	case resource.RuntimeFunctionDivide:
		return simplifyNonCommutative(function, 1, func(a, b float64) float64 { return a * b })
	case resource.RuntimeFunctionMod, resource.RuntimeFunctionRem, resource.RuntimeFunctionPower:
		return simplifyHead(function)
	case resource.RuntimeFunctionGet:
		return simplifyGet(function)
	case resource.RuntimeFunctionGetShifted:
		return simplifyGetShifted(function)
	case resource.RuntimeFunctionSet:
		return simplifySet(function, false)
	case resource.RuntimeFunctionSetShifted:
		return simplifySet(function, true)
	case resource.RuntimeFunctionIf:
		if len(args) > 2 && isValue(args[2], 0) {
			return simplify(call(resource.RuntimeFunctionAnd, args[0], args[1]))
		}
	case resource.RuntimeFunctionSwitchWithDefault:
		return simplifySwitch(function)
	case resource.RuntimeFunctionWhile:
		if len(args) >= 2 {
			if body, ok := asCall(args[1], resource.RuntimeFunctionExecute); ok && len(body.args) > 0 {
				if _, ok := body.args[len(body.args)-1].(valueNode); ok {
					body.args = body.args[:len(body.args)-1]
					if len(body.args) == 1 {
						function.args[1] = body.args[0]
					} else if len(body.args) > 1 {
						function.args[1] = body
					}
				}
			}
		}
	case resource.RuntimeFunctionExecute:
		if len(args) > 0 && isValue(args[len(args)-1], 0) {
			args = args[:len(args)-1]
			if len(args) == 0 {
				return valueNode(0)
			}
			if len(args) == 1 {
				return args[0]
			}
			function.args = args
		}
	}
	return function
}

func simplifyCommutative(function functionNode, identity float64, combine func(float64, float64) float64, zero bool) snode {
	flat := make([]snode, 0, len(function.args))
	for _, arg := range function.args {
		if nested, ok := asCall(arg, function.function); ok {
			flat = append(flat, nested.args...)
		} else {
			flat = append(flat, arg)
		}
	}
	constant := identity
	dynamic := make([]snode, 0, len(flat))
	for _, arg := range flat {
		if value, ok := arg.(valueNode); ok {
			constant = combine(constant, float64(value))
		} else {
			dynamic = append(dynamic, arg)
		}
	}
	if len(dynamic) == 0 {
		return valueNode(constant)
	}
	if zero && constant == 0 {
		return execute(append(dynamic, valueNode(0))...)
	}
	if constant != identity {
		dynamic = append([]snode{valueNode(constant)}, dynamic...)
	}
	if len(dynamic) == 1 {
		return dynamic[0]
	}
	return call(function.function, dynamic...)
}

func simplifyNonCommutative(function functionNode, identity float64, combine func(float64, float64) float64) snode {
	if len(function.args) == 0 {
		return valueNode(identity)
	}
	args := append([]snode(nil), function.args...)
	if nested, ok := asCall(args[0], function.function); ok {
		args = append(append([]snode(nil), nested.args...), args[1:]...)
	}
	head := args[0]
	constant := identity
	dynamic := make([]snode, 0, len(args))
	for _, arg := range args[1:] {
		if value, ok := arg.(valueNode); ok {
			constant = combine(constant, float64(value))
		} else {
			dynamic = append(dynamic, arg)
		}
	}
	if constant != identity {
		dynamic = append([]snode{valueNode(constant)}, dynamic...)
	}
	if len(dynamic) == 0 {
		return head
	}
	return call(function.function, append([]snode{head}, dynamic...)...)
}

func simplifyHead(function functionNode) snode {
	if len(function.args) == 0 {
		return valueNode(0)
	}
	if len(function.args) == 1 {
		return function.args[0]
	}
	if nested, ok := asCall(function.args[0], function.function); ok {
		return call(function.function, append(append([]snode(nil), nested.args...), function.args[1:]...)...)
	}
	return function
}

func simplifyGet(function functionNode) snode {
	if len(function.args) < 2 {
		return function
	}
	if add, ok := asCall(function.args[1], resource.RuntimeFunctionAdd); ok && len(add.args) == 2 {
		if multiply, ok := asCall(add.args[1], resource.RuntimeFunctionMultiply); ok && len(multiply.args) == 2 {
			return simplify(call(resource.RuntimeFunctionGetShifted, function.args[0], add.args[0], multiply.args[0], multiply.args[1]))
		}
	}
	return function
}

func simplifyGetShifted(function functionNode) snode {
	if len(function.args) < 4 {
		return function
	}
	id, x, y, shift := function.args[0], function.args[1], function.args[2], function.args[3]
	yv, yok := y.(valueNode)
	sv, sok := shift.(valueNode)
	if !yok || !sok {
		return function
	}
	if xv, ok := x.(valueNode); ok {
		return simplify(call(resource.RuntimeFunctionGet, id, valueNode(float64(xv)+float64(yv)*float64(sv))))
	}
	if yv == 0 && sv == 0 {
		return simplify(call(resource.RuntimeFunctionGet, id, x))
	}
	return function
}

func simplifySet(function functionNode, shifted bool) snode {
	minimum := 3
	valueIndex := 2
	getFunction := resource.RuntimeFunctionGet
	if shifted {
		minimum, valueIndex, getFunction = 5, 4, resource.RuntimeFunctionGetShifted
	}
	if len(function.args) < minimum {
		return function
	}
	if shifted {
		id, x, y, shift, value := function.args[0], function.args[1], function.args[2], function.args[3], function.args[4]
		if yv, ok := y.(valueNode); ok {
			if sv, ok := shift.(valueNode); ok {
				if xv, ok := x.(valueNode); ok {
					return simplify(call(resource.RuntimeFunctionSet, id, valueNode(float64(xv)+float64(yv)*float64(sv)), value))
				}
				if yv == 0 && sv == 0 {
					return simplify(call(resource.RuntimeFunctionSet, id, x, value))
				}
			}
		}
	} else if add, ok := asCall(function.args[1], resource.RuntimeFunctionAdd); ok && len(add.args) == 2 {
		if multiply, ok := asCall(add.args[1], resource.RuntimeFunctionMultiply); ok && len(multiply.args) == 2 {
			return simplify(call(resource.RuntimeFunctionSetShifted, function.args[0], add.args[0], multiply.args[0], multiply.args[1], function.args[2]))
		}
	}
	value, ok := function.args[valueIndex].(functionNode)
	if !ok || len(value.args) != 2 {
		return function
	}
	compound := compoundSet(value.function, shifted)
	if compound == "" {
		return function
	}
	address := function.args[:valueIndex]
	for replacement, operand := range []snode{value.args[0], value.args[1]} {
		get, ok := asCall(operand, getFunction)
		if !ok || len(get.args) != len(address) || !equivalentSlice(get.args, address) {
			continue
		}
		if replacement == 1 && value.function != resource.RuntimeFunctionAdd && value.function != resource.RuntimeFunctionMultiply {
			continue
		}
		args := append(append([]snode(nil), address...), value.args[1-replacement])
		return call(compound, args...)
	}
	return function
}

func compoundSet(function resource.RuntimeFunction, shifted bool) resource.RuntimeFunction {
	plain := map[resource.RuntimeFunction]resource.RuntimeFunction{
		resource.RuntimeFunctionAdd: resource.RuntimeFunctionSetAdd, resource.RuntimeFunctionSubtract: resource.RuntimeFunctionSetSubtract,
		resource.RuntimeFunctionMultiply: resource.RuntimeFunctionSetMultiply, resource.RuntimeFunctionDivide: resource.RuntimeFunctionSetDivide,
		resource.RuntimeFunctionMod: resource.RuntimeFunctionSetMod, resource.RuntimeFunctionRem: resource.RuntimeFunctionSetRem,
		resource.RuntimeFunctionPower: resource.RuntimeFunctionSetPower,
	}
	shift := map[resource.RuntimeFunction]resource.RuntimeFunction{
		resource.RuntimeFunctionAdd: resource.RuntimeFunctionSetAddShifted, resource.RuntimeFunctionSubtract: resource.RuntimeFunctionSetSubtractShifted,
		resource.RuntimeFunctionMultiply: resource.RuntimeFunctionSetMultiplyShifted, resource.RuntimeFunctionDivide: resource.RuntimeFunctionSetDivideShifted,
		resource.RuntimeFunctionMod: resource.RuntimeFunctionSetModShifted, resource.RuntimeFunctionRem: resource.RuntimeFunctionSetRemShifted,
		resource.RuntimeFunctionPower: resource.RuntimeFunctionSetPowerShifted,
	}
	if shifted {
		return shift[function]
	}
	return plain[function]
}

func simplifySwitch(function functionNode) snode {
	if len(function.args) < 4 || len(function.args[1:len(function.args)-1])%2 != 0 {
		return function
	}
	cases, fallback := function.args[1:len(function.args)-1], function.args[len(function.args)-1]
	tests := make([]float64, 0, len(cases)/2)
	for i := 0; i < len(cases); i += 2 {
		value, ok := cases[i].(valueNode)
		if !ok {
			return function
		}
		tests = append(tests, float64(value))
	}
	if len(tests) >= 2 {
		a, d := tests[0], tests[1]-tests[0]
		valid := safeInteger(a) && safeInteger(d)
		for i, value := range tests {
			valid = valid && value == a+d*float64(i)
		}
		if valid {
			discriminant := simplify(call(resource.RuntimeFunctionDivide, call(resource.RuntimeFunctionSubtract, function.args[0], valueNode(a)), valueNode(d)))
			args := []snode{discriminant}
			for i := 1; i < len(cases); i += 2 {
				args = append(args, cases[i])
			}
			if isValue(fallback, 0) {
				return call(resource.RuntimeFunctionSwitchInteger, args...)
			}
			return call(resource.RuntimeFunctionSwitchIntegerWithDefault, append(args, fallback)...)
		}
	}
	if isValue(fallback, 0) {
		return call(resource.RuntimeFunctionSwitch, append([]snode{function.args[0]}, cases...)...)
	}
	return function
}

func asCall(node snode, function resource.RuntimeFunction) (functionNode, bool) {
	value, ok := node.(functionNode)
	return value, ok && value.function == function
}

func isValue(node snode, expected float64) bool {
	value, ok := node.(valueNode)
	return ok && float64(value) == expected
}

func equivalent(a, b snode) bool {
	av, aok := a.(valueNode)
	bv, bok := b.(valueNode)
	if aok || bok {
		return aok && bok && av == bv
	}
	af, aok := a.(functionNode)
	bf, bok := b.(functionNode)
	if !aok || !bok || af.function != bf.function || len(af.args) != len(bf.args) || (af.function != resource.RuntimeFunctionGet && af.function != resource.RuntimeFunctionGetShifted) {
		return false
	}
	return equivalentSlice(af.args, bf.args)
}

func equivalentSlice(a, b []snode) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !equivalent(a[i], b[i]) {
			return false
		}
	}
	return true
}

func safeInteger(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value == math.Trunc(value) && math.Abs(value) <= maxSafeInteger
}
