// Engine DSL source for golden regression testing — entity info in Preview mode.
// This is read as a plain-text string by golden_test.go — it is NOT compiled as Go.
package golden

type Note struct {
	X float64 `sonolus:"memory"`
}

func (n Note) Render() {
	// Cross-entity in Preview: block 4102, stride 2.
	n.X = sonolus.EntityInfoIndex(0)
}
