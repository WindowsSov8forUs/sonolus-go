package p

type N struct {
	X float64 `sonolus:"memory"`
	Y float64 `sonolus:"memory"`
	Z float64 `sonolus:"memory"`
}

func (n N) Initialize() {
	n.X = 0
	n.Y = 0
	n.Z = 0
}

func (n N) UpdateSequential() {
	n.X = (3 + 5) * (2 + 1)
	n.Y = n.X * 2 - 4
	n.Z = n.X + n.Y
}
