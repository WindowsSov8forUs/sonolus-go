// Golden test: EntityInfo structured record access in Preview mode.
// This is read as a plain-text string by golden_test.go — it is NOT compiled as Go.
package golden

type Note struct {
	X float64 `sonolus:"memory"`
}

func (n Note) Render() {
	// 1. EntityInfoAt in Preview: block 4102, stride 2
	n.X = sonolus.EntityInfoAt(0).Index

	// 2. SelfInfo in Preview: block 4002
	n.X = n.X + sonolus.SelfInfo().Archetype
}
