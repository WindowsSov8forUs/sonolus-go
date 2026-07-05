package golden

type Skin struct{ Note float64 }

type Note struct {
	Beat float64 `sonolus:"imported"`
	X    float64 `sonolus:"memory"`
	Tr   Transform2d `sonolus:"memory"`
}

func (n Note) Initialize() {
	t := sonolus.SkinTransform()
	n.X = t.A00 + t.A33
	n.Tr = t
}
