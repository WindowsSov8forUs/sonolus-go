// Engine DSL source for Preview-mode golden regression testing.
// This is read as a plain-text string by golden_test.go — it is NOT compiled as Go.
package golden

type Skin struct {
	Note float64
}

type Note struct {
	Beat float64 `sonolus:"imported"`
	X    float64 `sonolus:"data"`
}

func (n Note) Initialize() {
	n.X = n.Beat * 0.5
}

func (n Note) UpdateParallel() {
	n.X = n.X + deltaTime
}
