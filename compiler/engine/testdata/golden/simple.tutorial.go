// Engine DSL source for Tutorial-mode golden regression testing.
// This is read as a plain-text string by golden_test.go — it is NOT compiled as Go.
package golden

type Instruction struct {
	Step float64 `sonolus:"imported"`
	X    float64 `sonolus:"memory"`
}

func Preprocess() {
	set(3000, 0, 0)
}

func Navigate() {
	x := get(3000, 0)
	set(3000, 0, x+x)
}

func Update() {
	set(3000, 0, deltaTime)
}
