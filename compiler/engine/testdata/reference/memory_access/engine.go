package p

type N struct {
	Beat  float64 `sonolus:"imported"`
	Value float64 `sonolus:"memory"`
	Saved float64 `sonolus:"memory"`
}

func (n N) Initialize() {
	n.Value = 0
	n.Saved = 0
}

func (n N) UpdateSequential() {
	n.Value = n.Beat * 2
	if n.Value > 100 {
		n.Saved = n.Value
		n.Value = 0
	}
}
