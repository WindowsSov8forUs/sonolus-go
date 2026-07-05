package golden

type Skin struct{ Note float64 }

type Note struct {
	Beat float64 `sonolus:"imported"`
	X    float64 `sonolus:"memory"`
}

func (n Note) Preprocess() {
	n.X = sonolus.Life().Consecutive.Perfect.Increment +
		sonolus.Life().Initial
}
