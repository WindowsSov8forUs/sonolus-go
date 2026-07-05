// Engine DSL source for golden regression testing — cross-entity info.
// This is read as a plain-text string by golden_test.go — it is NOT compiled as Go.
package golden

type Skin struct {
	Note float64
}

type Note struct {
	Beat float64 `sonolus:"imported"`
	X    float64 `sonolus:"memory"`
}

func (n Note) Initialize() {
	// Cross-entity: read index (field 0) of entity 1.
	// With stride=3: 4103[0 + 1*3] = 4103[3].
	n.X = sonolus.EntityInfoIndex(1)
}
