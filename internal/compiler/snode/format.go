package snode

import (
	"math"
	"strconv"
	"strings"
)

// FormatNumber renders a float64 the way JavaScript's Number.prototype.toString
// (and therefore JSON.stringify for finite numbers) would.
//
// The reference compiler (sonolus.js-compiler) builds its node dedup keys with
// the JS template literal `${value}` and emits value nodes via JSON.stringify.
// To reproduce byte-identical output we must match JS number formatting here,
// rather than relying on Go's default float formatting which differs in
// exponent thresholds, exponent padding, and negative-zero handling.
//
// Rules replicated:
//   - +0 and -0 both render as "0".
//   - Magnitudes in [1e-6, 1e21) use plain decimal notation ('f').
//   - Magnitudes outside that range use exponential notation with a sign and no
//     leading zero in the exponent (e.g. "1e-7", "1e+21"), matching JS.
//   - The shortest round-tripping mantissa is used (strconv precision -1).
func FormatNumber(f float64) string {
	if f == 0 {
		// Covers both +0 and -0; JS prints "0" for negative zero.
		return "0"
	}
	if math.IsNaN(f) {
		// JSON.stringify(NaN) is "null"; this only matters for dedup keys.
		return "NaN"
	}
	if math.IsInf(f, 1) {
		return "Infinity"
	}
	if math.IsInf(f, -1) {
		return "-Infinity"
	}

	abs := math.Abs(f)
	if abs >= 1e21 || abs < 1e-6 {
		return jsExponential(f)
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// jsExponential formats f in JS exponential style: a shortest mantissa, the
// letter 'e', an explicit sign, and an exponent with no leading zeros.
//
// Go's 'e' format pads the exponent to at least two digits ("1e-07") and always
// includes a sign, so we only need to strip the leading zeros from the exponent.
func jsExponential(f float64) string {
	s := strconv.FormatFloat(f, 'e', -1, 64)

	i := strings.IndexByte(s, 'e')
	if i < 0 {
		return s
	}
	mantissa, exp := s[:i], s[i+1:]

	sign := "+"
	if len(exp) > 0 && (exp[0] == '+' || exp[0] == '-') {
		if exp[0] == '-' {
			sign = "-"
		}
		exp = exp[1:]
	}
	exp = strings.TrimLeft(exp, "0")
	if exp == "" {
		exp = "0"
	}

	return mantissa + "e" + sign + exp
}
