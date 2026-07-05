package golden

type Skin struct{ Note float64 }

type Note struct {
	Beat  float64     `sonolus:"imported"`
	Score EntityScore `sonolus:"scored"`
	Life  EntityLife  `sonolus:"lifed"`
}

func (n Note) Touch() {
	n.Score.Perfect = 100
	n.Life.Miss = -50
}
