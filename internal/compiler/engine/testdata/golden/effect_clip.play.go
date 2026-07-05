package golden

type Skin struct{ Note float64 }

type Effect struct {
	Hit  float64
	Miss float64
}

type Note struct {
	Beat float64 `sonolus:"imported"`
	X    float64 `sonolus:"memory"`
}

func (n Note) Initialize() {
	sonolus.EffectClip("Hit").Schedule(n.Beat, 0.1)
}
