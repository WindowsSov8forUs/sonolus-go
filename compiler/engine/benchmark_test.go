package engine

import (
	"os"
	"testing"
)

// BenchmarkCompilePlay measures end-to-end Play-mode compilation throughput
// for a minimal engine (one archetype, one callback).
func BenchmarkCompilePlay(b *testing.B) {
	src := `package p
type Note struct {
	Beat float64 ` + "`sonolus:\"imported\"`" + `
	X    float64 ` + "`sonolus:\"memory\"`" + `
}
func (n *Note) Initialize() {
	debugPause()
}
func (n *Note) UpdateParallel() {
	debugPause()
}
`
	for range b.N {
		_, _, err := CompilePlayFile(src)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkCompilePlayWithArithmetic measures compilation throughput for
// a callback that exercises arithmetic and control flow.
func BenchmarkCompilePlayWithArithmetic(b *testing.B) {
	src := `package p
type Note struct {
	X float64 ` + "`sonolus:\"memory\"`" + `
	Y float64 ` + "`sonolus:\"memory\"`" + `
}
func (n *Note) UpdateParallel() {
	n.X = n.X * 2 + 1
	n.Y = sin(n.X)
	if n.X > 100 {
		n.X = 0
	}
}
`
	for range b.N {
		_, _, err := CompilePlayFile(src)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkCompilePlayWithLoop measures compilation for a callback containing
// a for loop with arithmetic.
func BenchmarkCompilePlayWithLoop(b *testing.B) {
	src := `package p
type Note struct {
	Beat  float64 ` + "`sonolus:\"imported\"`" + `
	Count float64 ` + "`sonolus:\"memory\"`" + `
}
func (n *Note) Initialize() {
	for i := 0; i < 10; i++ {
		n.Count = n.Count + 1
	}
}
func (n *Note) UpdateSequential() {
	if n.Beat > 100 {
		n.Count = n.Count + 1
	}
}
`
	for range b.N {
		_, _, err := CompilePlayFile(src)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkCompilePlayStages reports total Play-mode compilation time for a
// simple engine read from testdata/golden/simple_play.go.
func BenchmarkCompilePlayStages(b *testing.B) {
	src, err := os.ReadFile("testdata/golden/simple_play.go")
	if err != nil {
		b.Fatalf("read test source: %v", err)
	}
	srcStr := string(src)

	for range b.N {
		_, _, err := CompilePlayFile(srcStr)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkCompileAllModes measures compilation throughput for all four modes
// sequentially from a single engine source.
func BenchmarkCompileAllModes(b *testing.B) {
	src := `package p
type Note struct {
	Beat float64 ` + "`sonolus:\"imported\"`" + `
	X    float64 ` + "`sonolus:\"memory\"`" + `
}
func (n *Note) Initialize() {
	debugPause()
}
func (n *Note) UpdateParallel() {
	debugPause()
}
func UpdateSpawn() float64 { return 0 }
func Preprocess() {}
func Navigate() float64 { return 1 }
func Update() {}
`
	for range b.N {
		_, _, err := CompilePlayFile(src)
		if err != nil {
			b.Fatal(err)
		}
		_, err = CompileWatchFile(src)
		if err != nil {
			b.Fatal(err)
		}
		_, err = CompilePreviewFile(src)
		if err != nil {
			b.Fatal(err)
		}
		_, err = CompileTutorialFile(src)
		if err != nil {
			b.Fatal(err)
		}
	}
}
