package engine

import (
	"testing"
)

// FuzzCompilePlay verifies that CompilePlayFile never panics on arbitrary
// Go source inputs. It does not assert on error values — the goal is crash
// safety, not correctness of output for malformed input.
func FuzzCompilePlay(f *testing.F) {
	seeds := []string{
		`package p
type N struct {}
func (n *N) Initialize() {}`,
		`package p
type N struct { t float64 ` + "`sonolus:\"imported\"`" + ` }
func (n *N) UpdateSequential(dt float64) { n.t = n.t + dt }`,
		`package p
type A struct { x float64 ` + "`sonolus:\"memory\"`" + ` }
func (a *A) Preprocess() {}
func (a *A) Initialize() { a.x = 42 }
func (a *A) UpdateParallel(dt float64) { a.x = a.x + dt }`,
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, src string) {
		// Must not panic on any input.
		data, cfg, err := CompilePlayFile(src)
		_ = data
		_ = cfg
		_ = err
	})
}

// FuzzCompileWatch verifies crash safety for Watch mode compilation.
func FuzzCompileWatch(f *testing.F) {
	seeds := []string{
		`package p
type N struct {}
func (n *N) Initialize() {}`,
		`package p
type N struct { t float64 ` + "`sonolus:\"imported\"`" + ` }
func (n *N) UpdateSequential(dt float64) { n.t = n.t + dt }`,
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, src string) {
		data, err := CompileWatchFile(src)
		_ = data
		_ = err
	})
}

// FuzzCompilePreview verifies crash safety for Preview mode compilation.
func FuzzCompilePreview(f *testing.F) {
	seeds := []string{
		`package p
type N struct {}
func (n *N) Preprocess() {}`,
		`package p
type N struct { t float64 ` + "`sonolus:\"data\"`" + ` }
func (n *N) Render() { DebugPause() }`,
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, src string) {
		data, err := CompilePreviewFile(src)
		_ = data
		_ = err
	})
}

// FuzzCompileTutorial verifies crash safety for Tutorial mode compilation.
func FuzzCompileTutorial(f *testing.F) {
	seeds := []string{
		`package p
func Preprocess() {}
func Navigate() float64 { return 1 }
func Update() {}`,
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, src string) {
		data, err := CompileTutorialFile(src)
		_ = data
		_ = err
	})
}
