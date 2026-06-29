package test

type Skin struct {
	TapNote   float64
	FlickNote float64
	Connector float64
}

type TapNote struct {
	Beat  float64 `sonolus:"imported"`
	Lane  float64 `sonolus:"imported"`
	Width float64 `sonolus:"imported"`
	T     float64 `sonolus:"memory"`
	X     float64 `sonolus:"memory"`
}

func (n *TapNote) Preprocess()      { n.T = n.Beat }
func (n *TapNote) Initialize()      { n.X = n.Lane*2 - 1 }
func (n *TapNote) UpdateSequential() {
	n.T = n.T + deltaTime
	if n.T > 100 {
		n.X = n.X + 0.1
	}
}
func (n *TapNote) UpdateParallel() {
	p := vec2(n.X, 0)
	draw(1, p.x-0.5, -0.5, p.x-0.5, 0.5, p.x+0.5, 0.5, p.x+0.5, -0.5, 0, 1, 0, 1, 0)
}

type FlickNote struct {
	Beat float64 `sonolus:"imported"`
	Lane float64 `sonolus:"imported"`
	T    float64 `sonolus:"memory"`
}

func (n *FlickNote) Initialize()    { n.T = n.Beat }
func (n *FlickNote) UpdateParallel() {
	v := vec2(n.Lane*2-1, n.T*0.01)
	draw(1, v.x-0.3, -0.3, v.x, 0.3, v.x+0.3, -0.3, 1, 1, 0, 1, 0)
}
