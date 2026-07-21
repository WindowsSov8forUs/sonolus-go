package shared

type Timing struct {
	Beat float64 `archetype:"imported,name=#BEAT"`
	Time float64 `archetype:"data"`
}

type BasicNote struct {
	Timing
	Seen float64 `archetype:"shared"`
}

func (note *BasicNote) Preprocess() {
	note.Time = note.Beat
	note.Seen = note.Time
}
