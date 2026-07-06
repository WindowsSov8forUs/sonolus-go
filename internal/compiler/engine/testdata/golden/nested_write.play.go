package golden

type Skin struct{ Note float64 }

type Note struct {
	Beat float64 `sonolus:"imported"`
	X    float64 `sonolus:"memory"`
}

func (n Note) Preprocess() {
	s := sonolus.Score()

	// Nested composite chain write
	s.Base.Perfect = 1000000
	s.Base.Great = 750000
	s.Consecutive.Perfect.Multiplier = 1
	n.X = s.Base.Perfect
}
