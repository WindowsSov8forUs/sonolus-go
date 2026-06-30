package p

type N struct {
	a float64 `sonolus:"memory"`
	b float64 `sonolus:"memory"`
}

func lerp(a, b, t float64) float64 {
	return a + (b-a)*t
}

func (n N) Initialize() {
	n.a = 1 + 2*3
	n.b = lerp(n.a, 100, 0.05)
}
