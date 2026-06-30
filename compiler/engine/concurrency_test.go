package engine

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"testing"
)

// TestConcurrentCompilation verifies that compiling engine sources concurrently
// from multiple goroutines produces deterministic, race-free results. Each
// goroutine compiles the same source; we assert that all outputs are byte-identical
// to a single-threaded baseline.
func TestConcurrentCompilation(t *testing.T) {
	const src = `package test

type Skin struct {
	Note float64
}

type Note struct {
	Beat float64 ` + "`sonolus:\"imported\"`" + `
	T    float64 ` + "`sonolus:\"memory\"`" + `
}

func (n Note) Initialize() {
	n.T = n.Beat * 0.5
}

func (n Note) UpdateParallel() {
	v := vec2(sin(n.T), cos(n.T))
	draw(1, v.x, v.y, 1, 1, 0, 1, 0, 0)
}
`
	// Single-threaded baseline.
	const N = 8
	baseline, baselineCfg, err := CompilePlayFile(src)
	if err != nil {
		t.Fatalf("baseline compile: %v", err)
	}
	baselineB := mustJSON(t, baseline)
	baselineCfgB := mustJSON(t, baselineCfg)

	// Concurrent compilations.
	var wg sync.WaitGroup
	errs := make(chan error, N)
	for range N {
		wg.Add(1)
		go func() {
			defer wg.Done()
			data, cfg, compileErr := CompilePlayFile(src)
			if compileErr != nil {
				errs <- compileErr
				return
			}
			if gotB := mustJSON(t, data); !bytes.Equal(gotB, baselineB) {
				errs <- fmt.Errorf("play data mismatch: baseline %d bytes, got %d bytes",
					len(baselineB), len(gotB))
				return
			}
			if gotCfgB := mustJSON(t, cfg); !bytes.Equal(gotCfgB, baselineCfgB) {
				errs <- fmt.Errorf("configuration mismatch")
				return
			}
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Error(e)
	}

	// Also test non-play modes concurrently.
	watchBase, watchErr := CompileWatchFile(src)
	if watchErr != nil {
		t.Skipf("watch baseline: %v", watchErr)
	}
	previewBase, previewErr := CompilePreviewFile(src)
	if previewErr != nil {
		t.Skipf("preview baseline: %v", previewErr)
	}
	tutorialBase, tutorialErr := CompileTutorialFile(src)
	if tutorialErr != nil {
		t.Skipf("tutorial baseline: %v", tutorialErr)
	}

	watchB := mustJSON(t, watchBase)
	previewB := mustJSON(t, previewBase)
	tutorialB := mustJSON(t, tutorialBase)

	errs2 := make(chan error, N)
	var wg2 sync.WaitGroup
	for range 4 {
		wg2.Add(1)
		go func() {
			defer wg2.Done()
			w, wErr := CompileWatchFile(src)
			if wErr != nil {
				errs2 <- wErr
				return
			}
			if wb := mustJSON(t, w); !bytes.Equal(wb, watchB) {
				errs2 <- fmt.Errorf("watch mismatch")
			}
			p, pErr := CompilePreviewFile(src)
			if pErr != nil {
				errs2 <- pErr
				return
			}
			if pb := mustJSON(t, p); !bytes.Equal(pb, previewB) {
				errs2 <- fmt.Errorf("preview mismatch")
			}
			tu, tuErr := CompileTutorialFile(src)
			if tuErr != nil {
				errs2 <- tuErr
				return
			}
			if tub := mustJSON(t, tu); !bytes.Equal(tub, tutorialB) {
				errs2 <- fmt.Errorf("tutorial mismatch")
			}
		}()
	}
	wg2.Wait()
	close(errs2)
	for e := range errs2 {
		t.Error(e)
	}
}

// TestConcurrentCompilationNonPlay verifies deterministic output when compiling
// Watch, Preview, and Tutorial mode engine sources concurrently from multiple
// goroutines. Each mode uses a source that is valid for that mode so that no
// compilation is skipped.
func TestConcurrentCompilationNonPlay(t *testing.T) {
	const watchSrc = `package test

type Skin struct {
	Note float64
}

type Note struct {
	Time float64 ` + "`sonolus:\"memory\"`" + `
}

func (n Note) Initialize() {
	n.Time = 1
}

func (n Note) UpdateSequential() {}
`
	const previewSrc = `package test

type Skin struct {
	Note float64
}

type Note struct {
	T float64 ` + "`sonolus:\"memory\"`" + `
}

func (n Note) Render() {}
`
	const tutorialSrc = `package test

type Skin struct {
	Note float64
}

func Preprocess() {}
func Navigate()   {}
func Update()     {}
`

	const N = 4

	// Single-threaded baselines.
	watchBase, watchErr := CompileWatchFile(watchSrc)
	if watchErr != nil {
		t.Fatalf("watch baseline: %v", watchErr)
	}
	previewBase, previewErr := CompilePreviewFile(previewSrc)
	if previewErr != nil {
		t.Fatalf("preview baseline: %v", previewErr)
	}
	tutorialBase, tutorialErr := CompileTutorialFile(tutorialSrc)
	if tutorialErr != nil {
		t.Fatalf("tutorial baseline: %v", tutorialErr)
	}
	watchB := mustJSON(t, watchBase)
	previewB := mustJSON(t, previewBase)
	tutorialB := mustJSON(t, tutorialBase)

	errs := make(chan error, N*3)
	var wg sync.WaitGroup
	for range N {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w, wErr := CompileWatchFile(watchSrc)
			if wErr != nil {
				errs <- wErr
				return
			}
			if wb := mustJSON(t, w); !bytes.Equal(wb, watchB) {
				errs <- fmt.Errorf("watch mismatch")
			}
			p, pErr := CompilePreviewFile(previewSrc)
			if pErr != nil {
				errs <- pErr
				return
			}
			if pb := mustJSON(t, p); !bytes.Equal(pb, previewB) {
				errs <- fmt.Errorf("preview mismatch")
			}
			tu, tuErr := CompileTutorialFile(tutorialSrc)
			if tuErr != nil {
				errs <- tuErr
				return
			}
			if tub := mustJSON(t, tu); !bytes.Equal(tub, tutorialB) {
				errs <- fmt.Errorf("tutorial mismatch")
			}
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Error(e)
	}
}

// TestConcurrentCompilationComplex verifies deterministic output when compiling
// a complex engine source (multiple archetypes, control flow, arithmetic) from
// many concurrent goroutines.
func TestConcurrentCompilationComplex(t *testing.T) {
	src, err := os.ReadFile("testdata/concurrency_complex.go")
	if err != nil {
		t.Fatalf("read test source: %v", err)
	}
	srcStr := string(src)

	const N = 16
	baseline, baselineCfg, err := CompilePlayFile(srcStr)
	if err != nil {
		t.Fatalf("baseline compile: %v", err)
	}
	baselineB := mustJSON(t, baseline)
	baselineCfgB := mustJSON(t, baselineCfg)

	var wg sync.WaitGroup
	errs := make(chan error, N)
	for range N {
		wg.Add(1)
		go func() {
			defer wg.Done()
			data, cfg, compileErr := CompilePlayFile(srcStr)
			if compileErr != nil {
				errs <- compileErr
				return
			}
			if gotB := mustJSON(t, data); !bytes.Equal(gotB, baselineB) {
				errs <- fmt.Errorf("complex: play data mismatch")
				return
			}
			if gotCfgB := mustJSON(t, cfg); !bytes.Equal(gotCfgB, baselineCfgB) {
				errs <- fmt.Errorf("complex: configuration mismatch")
				return
			}
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Error(e)
	}
}

func mustJSON[T any](t *testing.T, v *T) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
