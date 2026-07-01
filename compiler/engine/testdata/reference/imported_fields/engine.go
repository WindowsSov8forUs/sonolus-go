package p

type N struct {
	Beat float64 `sonolus:"imported"`
	Bpm  float64 `sonolus:"imported"`
	Sum  float64 `sonolus:"memory"`
}

func (n N) Initialize() {
	n.Sum = n.Beat + n.Bpm
}

func (n N) UpdateSequential() {
	n.Sum = n.Beat*2 + n.Bpm
}
