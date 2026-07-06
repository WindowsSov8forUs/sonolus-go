package notegarupa

type SlideNote struct {
	Beat float64 `sonolus:"imported"`
	X    float64 `sonolus:"memory"`
}

func (n *SlideNote) UpdateSequential() {
	n.X = n.Beat * 2
}
