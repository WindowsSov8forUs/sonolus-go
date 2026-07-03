package simple

type Skin struct {
	Note  float64 `json:"note"`
	Stage float64 `json:"stage"`
}

type Note struct {
	Beat       float64 `sonolus:"imported"`
	TargetTime float64 `sonolus:"memory"`
}

func (n Note) Initialize() {
	n.TargetTime = n.Beat * 0.5
}

func (n Note) UpdateParallel() {
	draw(1, n.TargetTime, 0, n.TargetTime, 1, 0, 1, 0, 0)
}
