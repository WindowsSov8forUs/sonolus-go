package golden

type Skin struct{ Note float64 }

type Note struct {
	Beat float64 `sonolus:"imported"`
	X    float64 `sonolus:"memory"`
}

func (n Note) Preprocess() {
	score := sonolus.Score()
	n.X = score.Base.Perfect +
		score.Consecutive.Perfect.Multiplier +
		score.Consecutive.Great.Cap
}
