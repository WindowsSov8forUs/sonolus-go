package snode

// --- SwitchWithDefault (src/snode/optimize/SwitchWithDefault.ts) ---

func optimizeSwitchWithDefault(s Func) SNode {
	if len(s.Args) < 2 {
		return s
	}
	discriminant := s.Args[0]
	cases := s.Args[1 : len(s.Args)-1]
	defaultCase := s.Args[len(s.Args)-1]

	removeDefault := isValueEq(defaultCase, 0)

	if a, d, ok := tryNormalize(cases); ok {
		normalizedDiscriminant := Peephole(Func{Op: rfDivide, Args: []SNode{
			Func{Op: rfSubtract, Args: []SNode{discriminant, Value(a)}},
			Value(d),
		}})

		var consequences []SNode
		for i := 1; i < len(cases); i += 2 {
			consequences = append(consequences, cases[i])
		}

		if removeDefault {
			return Func{Op: rfSwitchInteger, Args: append([]SNode{normalizedDiscriminant}, consequences...)}
		}
		return Func{Op: rfSwitchIntegerWithDefault, Args: append(append([]SNode{normalizedDiscriminant}, consequences...), defaultCase)}
	}

	if removeDefault {
		return Func{Op: rfSwitch, Args: append([]SNode{discriminant}, cases...)}
	}
	return s
}

// tryNormalize checks whether the case test values form an arithmetic sequence
// a, a+d, a+2d, ... of safe integers and returns (a, d, true) if so.
func tryNormalize(cases []SNode) (a, d float64, ok bool) {
	var tests []float64
	for i := 0; i < len(cases); i += 2 {
		v, isVal := asValue(cases[i])
		if !isVal {
			return 0, 0, false
		}
		tests = append(tests, v)
	}

	if len(tests) < 1 {
		return 0, 0, false
	}
	a = tests[0]
	if !isSafeInteger(a) {
		return 0, 0, false
	}
	if len(tests) < 2 {
		// d would be NaN in JS (tests[1] undefined), failing isSafeInteger.
		return 0, 0, false
	}
	d = tests[1] - a
	if !isSafeInteger(d) {
		return 0, 0, false
	}
	for i, value := range tests {
		if value != a+d*float64(i) {
			return 0, 0, false
		}
	}
	return a, d, true
}

// --- While (src/snode/optimize/While.ts) ---

func optimizeWhile(s Func) SNode {
	if len(s.Args) < 2 {
		return s
	}
	body, ok := asFunc(s.Args[1], rfExecute)
	if !ok || len(body.Args) == 0 {
		return s
	}
	if _, ok := asValue(body.Args[len(body.Args)-1]); !ok {
		return s
	}

	if len(body.Args) == 2 {
		return Func{Op: rfWhile, Args: []SNode{s.Args[0], body.Args[0]}}
	}
	return Func{Op: rfWhile, Args: []SNode{
		s.Args[0],
		Func{Op: rfExecute, Args: append([]SNode{}, body.Args[:len(body.Args)-1]...)},
	}}
}
