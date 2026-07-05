package golden

type Skin struct{ Note float64 }

type Note struct {
	Beat float64 `sonolus:"imported"`
	X    float64 `sonolus:"memory"`
}

func (n Note) Preprocess() {
	life := sonolus.Life()
	n.X = life.Consecutive.Perfect.Increment +
		life.Initial +
		life.Max
}
