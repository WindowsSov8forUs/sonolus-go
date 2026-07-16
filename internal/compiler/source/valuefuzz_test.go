package source

import "testing"

func FuzzStaticValueEval(f *testing.F) {
	for _, seed := range []string{
		"1 + 2*3",
		"float32(0.1)",
		"[3]int{1, 2, 3}",
		"[]int{1, 2: 3}",
		`map[string]int{"a": 1}`,
		"struct{ A int }{A: 1}",
		"new(int)",
		"make([]int, 2, 3)",
		`len("hello")`,
		"min(3, 1, 2)",
		"(*int)(nil)",
		"func() {}",
		"[]int{1}[2]",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, expression string) {
		if len(expression) > 4096 {
			return
		}
		pkg, err := checkStaticPackage(map[string]string{
			"fuzz.go": "package main\nvar Value = " + expression + "\n",
		})
		if err != nil {
			return
		}

		tracer := NewASTTracer(pkg)
		first, firstErr := tracer.EvalPackageValue("Value")
		second, secondErr := tracer.EvalPackageValue("Value")
		if firstErr != nil || secondErr != nil {
			if firstErr == nil || secondErr == nil || firstErr.Error() != secondErr.Error() {
				t.Fatalf("nondeterministic errors: first=%v second=%v", firstErr, secondErr)
			}
			return
		}
		if firstDigest, secondDigest := staticValueDigest(first.Value), staticValueDigest(second.Value); firstDigest != secondDigest {
			t.Fatalf("nondeterministic values:\nfirst:  %s\nsecond: %s", firstDigest, secondDigest)
		}
	})
}
