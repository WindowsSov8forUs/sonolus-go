package golden

type Skin struct{ Note float64 }

type Note struct {
	Beat float64 `sonolus:"imported"`
	X    float64 `sonolus:"memory"`
	V    Vec2   `sonolus:"memory"`
}

func (n Note) Initialize() {
	n.V = sonolus.Vec2_(1, 2)
}
