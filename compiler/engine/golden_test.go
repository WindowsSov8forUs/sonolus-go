package engine

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// updateGolden controls whether golden files are regenerated instead of compared.
var updateGolden = flag.Bool("update", false, "update golden files")

// TestGoldenPlayEndToEnd compiles engine DSL sources from testdata/golden/*.go
// and compares the resulting EnginePlayData JSON against saved golden files.
// Use -update to regenerate golden outputs.
func TestGoldenPlayEndToEnd(t *testing.T) {
	sources, err := filepath.Glob("testdata/golden/*.play.go")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(sources) == 0 {
		t.Fatal("no golden test sources found in testdata/golden/")
	}

	for _, srcPath := range sources {
		name := strings.TrimSuffix(filepath.Base(srcPath), ".go")
		goldenPath := filepath.Join("testdata", "golden", name+".play.json")

		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(srcPath)
			if err != nil {
				t.Fatalf("read source: %v", err)
			}

			playData, _, err := CompilePlayFile(string(src))
			if err != nil {
				t.Fatalf("compile: %v", err)
			}

			// Marshal to indented JSON for human-readable diffs.
			got, err := json.MarshalIndent(playData, "", "  ")
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			got = append(got, '\n')

			if *updateGolden {
				if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
					t.Fatalf("write golden: %v", err)
				}
				t.Logf("updated %s", goldenPath)
				return
			}

			want, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("read golden %s: %v\n(hint: run with -update to generate)", goldenPath, err)
			}

			if string(got) != string(want) {
				t.Errorf("golden mismatch for %s:\n--- got ---\n%s\n--- want ---\n%s", name, string(got), string(want))
			}
		})
	}
}

// TestGoldenWatchEndToEnd compiles engine DSL sources from testdata/golden/*.watch.go
// and compares the resulting EngineWatchData JSON against saved golden files.
func TestGoldenWatchEndToEnd(t *testing.T) {
	sources, err := filepath.Glob("testdata/golden/*.watch.go")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(sources) == 0 {
		t.Fatal("no golden watch test sources found in testdata/golden/")
	}
	for _, srcPath := range sources {
		name := strings.TrimSuffix(filepath.Base(srcPath), ".go")
		goldenPath := filepath.Join("testdata", "golden", name+".watch.json")
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(srcPath)
			if err != nil {
				t.Fatalf("read source: %v", err)
			}
			watchData, err := CompileWatchFile(string(src))
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			got, err := json.MarshalIndent(watchData, "", "  ")
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			got = append(got, '\n')
			if *updateGolden {
				if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
					t.Fatalf("write golden: %v", err)
				}
				t.Logf("updated %s", goldenPath)
				return
			}
			want, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("read golden %s: %v\n(hint: run with -update to generate)", goldenPath, err)
			}
			if string(got) != string(want) {
				t.Errorf("golden mismatch for %s:\n--- got ---\n%s\n--- want ---\n%s", name, string(got), string(want))
			}
		})
	}
}

// TestGoldenPreviewEndToEnd compiles engine DSL sources from testdata/golden/*.preview.go
// and compares the resulting EnginePreviewData JSON against saved golden files.
func TestGoldenPreviewEndToEnd(t *testing.T) {
	sources, err := filepath.Glob("testdata/golden/*.preview.go")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(sources) == 0 {
		t.Fatal("no golden preview test sources found in testdata/golden/")
	}
	for _, srcPath := range sources {
		name := strings.TrimSuffix(filepath.Base(srcPath), ".go")
		goldenPath := filepath.Join("testdata", "golden", name+".preview.json")
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(srcPath)
			if err != nil {
				t.Fatalf("read source: %v", err)
			}
			previewData, err := CompilePreviewFile(string(src))
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			got, err := json.MarshalIndent(previewData, "", "  ")
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			got = append(got, '\n')
			if *updateGolden {
				if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
					t.Fatalf("write golden: %v", err)
				}
				t.Logf("updated %s", goldenPath)
				return
			}
			want, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("read golden %s: %v\n(hint: run with -update to generate)", goldenPath, err)
			}
			if string(got) != string(want) {
				t.Errorf("golden mismatch for %s:\n--- got ---\n%s\n--- want ---\n%s", name, string(got), string(want))
			}
		})
	}
}

// TestGoldenTutorialEndToEnd compiles engine DSL sources from testdata/golden/*.tutorial.go
// and compares the resulting EngineTutorialData JSON against saved golden files.
func TestGoldenTutorialEndToEnd(t *testing.T) {
	sources, err := filepath.Glob("testdata/golden/*.tutorial.go")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(sources) == 0 {
		t.Fatal("no golden tutorial test sources found in testdata/golden/")
	}
	for _, srcPath := range sources {
		name := strings.TrimSuffix(filepath.Base(srcPath), ".go")
		goldenPath := filepath.Join("testdata", "golden", name+".tutorial.json")
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(srcPath)
			if err != nil {
				t.Fatalf("read source: %v", err)
			}
			tutorialData, err := CompileTutorialFile(string(src))
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			got, err := json.MarshalIndent(tutorialData, "", "  ")
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			got = append(got, '\n')
			if *updateGolden {
				if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
					t.Fatalf("write golden: %v", err)
				}
				t.Logf("updated %s", goldenPath)
				return
			}
			want, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("read golden %s: %v\n(hint: run with -update to generate)", goldenPath, err)
			}
			if string(got) != string(want) {
				t.Errorf("golden mismatch for %s:\n--- got ---\n%s\n--- want ---\n%s", name, string(got), string(want))
			}
		})
	}
}
