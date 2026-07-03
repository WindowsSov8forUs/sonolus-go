package verify

type Skin struct {
	TapHead   float64
	TapBody   float64
	Connector float64
}

type TapNote struct {
	Beat  float64 `sonolus:"imported"`
	Lane  float64 `sonolus:"imported"`
	Width float64 `sonolus:"imported"`
	Time  float64 `sonolus:"memory"`
	X     float64 `sonolus:"memory"`
}

func (n TapNote) Preprocess()  { n.Time = n.Beat; n.X = n.Lane }
func (n TapNote) ShouldSpawn() { return time > n.Time }
func (n TapNote) Initialize()  { n.X = n.Lane*2 - 1 }
func (n TapNote) UpdateParallel() {
	p := vec2(n.X, 0)
	q := quad(p.x-0.5, -0.5, p.x-0.5, 0.5, p.x+0.5, 0.5, p.x+0.5, -0.5)
	draw(1, q.blx, q.bly, q.tlx, q.tly, q.trx, q.try, q.brx, q.bry, 0, 1, 0, 1, 0)
}
