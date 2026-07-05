// Golden test: EntityInfo structured record access in Play mode.
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
	// 1. Inline field access: EntityInfoAt(idx).State → GetShifted(4103, 2, idx, 3)
	n.X = sonolus.EntityInfoAt(1).State

	// 2. Self-info: SelfInfo().Index → Get(4003, 0)
	n.X = n.X + sonolus.SelfInfo().Index

	// 3. Constant reference: sonolus.EntityStateActive → const 1
	if n.X == sonolus.EntityStateActive {
		n.X = 0
	}

	// 4. Method call: info.IsActive() → state == 1
	info := sonolus.EntityInfoAt(0)
	if info.IsActive() {
		n.X = 2
	}
}
