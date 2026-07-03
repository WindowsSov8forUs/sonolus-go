package engine

import (
	"sync"
	"testing"

	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/ir/optimize"
)

// benchAllModesSrc is a shared engine source used by the multi-mode benchmarks.
const benchAllModesSrc = `package p
type Note struct {
	Beat float64 ` + "`" + `sonolus:"imported"` + "`" + `
	X    float64 ` + "`" + `sonolus:"memory"` + "`" + `
}
func (n *Note) Initialize()    { debugPause() }
func (n *Note) UpdateParallel() { debugPause() }
func UpdateSpawn() float64 { return 0 }
func Preprocess()              {}
func Render()                  {}
func Navigate() float64        { return 1 }
func Update()                  {}
`

// BenchmarkCompileAllModesParallel measures parallel compilation of all four
// modes using goroutines, matching sonolus.py's ThreadPoolExecutor pattern.
// Go's lack of a GIL means this scales to available CPU cores.
func BenchmarkCompileAllModesParallel(b *testing.B) {
	b.ReportAllocs()
	for range b.N {
		var wg sync.WaitGroup
		wg.Add(4)
		errs := make(chan error, 4)
		go func() { defer wg.Done(); _, _, e := CompilePlayFile(benchAllModesSrc); errs <- e }()
		go func() { defer wg.Done(); _, e := CompileWatchFile(benchAllModesSrc); errs <- e }()
		go func() { defer wg.Done(); _, e := CompilePreviewFile(benchAllModesSrc); errs <- e }()
		go func() { defer wg.Done(); _, e := CompileTutorialFile(benchAllModesSrc); errs <- e }()
		wg.Wait()
		close(errs)
		for e := range errs {
			if e != nil {
				b.Fatal(e)
			}
		}
	}
}

// BenchmarkOptLevels compares the three optimization levels for a callback
// with arithmetic and control flow, profiling the compile-time vs optimization
// quality trade-off.
func BenchmarkOptLevels(b *testing.B) {
	src := `package p
type Note struct {
	X float64 ` + "`" + `sonolus:"memory"` + "`" + `
	Y float64 ` + "`" + `sonolus:"memory"` + "`" + `
}
func (n *Note) UpdateParallel() {
	n.X = n.X*2 + 1
	n.Y = sin(n.X)
	if n.X > 100 { n.X = 0 }
}
`
	levels := []struct {
		name  string
		level optimize.Level
	}{
		{"Minimal", optimize.LevelMinimal},
		{"Fast", optimize.LevelFast},
		{"Standard", optimize.LevelStandard},
	}
	for _, l := range levels {
		b.Run(l.name, func(b *testing.B) {
			b.ReportAllocs()
			opts := &CompileOptions{Opt: l.level}
			for range b.N {
				_, _, err := CompilePlayFileWithStats(src, opts)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
