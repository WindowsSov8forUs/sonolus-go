package golden

type Skin struct{ Note float64 }

type Note struct {
	Beat float64 `sonolus:"imported"`
	X    float64 `sonolus:"memory"`
}

func (n Note) Preprocess() {
	// Combo-scaling life: perfect increment + step
	n.X = sonolus.ConsecutiveLife("perfect").Increment +
		sonolus.ConsecutiveLife("great").Step
}
