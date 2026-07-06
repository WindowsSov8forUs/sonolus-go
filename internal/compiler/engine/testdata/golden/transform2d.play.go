package golden

type Skin struct{ Note float64 }

type Note struct {
	Beat float64 `sonolus:"imported"`
	X    float64 `sonolus:"memory"`
}

func (n Note) Initialize() {
	t := sonolus.SkinTransform()
	r := t.Translate(sonolus.NewVec2(1, 2)).Rotate(0.5)
	n.X = r.TransformVec(sonolus.NewVec2(0, 0)).X
}
