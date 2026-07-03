package p

type N struct {
	Count float64 `sonolus:"memory"`
	Flag  float64 `sonolus:"memory"`
}

func (n N) Initialize() {
	n.Count = 0
	n.Flag = 0
}

func (n N) UpdateSequential() {
	for i := 0; i < 5; i++ {
		if i > 2 {
			n.Count = n.Count + 1
		}
	}
	if n.Count > 3 {
		n.Flag = 1
	}
}
