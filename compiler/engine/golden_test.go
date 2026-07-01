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

// goldenMode describes one golden-test mode: the file suffix, the JSON suffix,
// and a compile function that returns (data, error). Play mode's extra
// *EngineConfiguration return is discarded.
type goldenMode struct {
	suffix    string
	jsonExt   string
	compileFn func(string) (any, error)
}

// TestGoldenEndToEnd compiles engine DSL sources from testdata/golden/*.{mode}.go
// and compares the resulting JSON against saved golden files.
// Use -update to regenerate golden outputs.
func TestGoldenEndToEnd(t *testing.T) {
	modes := []goldenMode{
		{
			suffix:  ".play.go",
			jsonExt: ".play.json",
			compileFn: func(src string) (any, error) {
				data, _, err := CompilePlayFile(src)
				return data, err
			},
		},
		{
			suffix:  ".watch.go",
			jsonExt: ".watch.json",
			compileFn: func(src string) (any, error) {
				return CompileWatchFile(src)
			},
		},
		{
			suffix:  ".preview.go",
			jsonExt: ".preview.json",
			compileFn: func(src string) (any, error) {
				return CompilePreviewFile(src)
			},
		},
		{
			suffix:  ".tutorial.go",
			jsonExt: ".tutorial.json",
			compileFn: func(src string) (any, error) {
				return CompileTutorialFile(src)
			},
		},
	}

	for _, m := range modes {
		sources, err := filepath.Glob("testdata/golden/*" + m.suffix)
		if err != nil {
			t.Fatalf("glob %s: %v", m.suffix, err)
		}
		if len(sources) == 0 {
			t.Fatal("no golden test sources found for " + m.suffix)
		}

		for _, srcPath := range sources {
			name := strings.TrimSuffix(filepath.Base(srcPath), ".go")
			goldenPath := filepath.Join("testdata", "golden", name+m.jsonExt)

			t.Run(name, func(t *testing.T) {
				src, err := os.ReadFile(srcPath)
				if err != nil {
					t.Fatalf("read source: %v", err)
				}

				data, err := m.compileFn(string(src))
				if err != nil {
					t.Fatalf("compile: %v", err)
				}

				got, err := json.MarshalIndent(data, "", "  ")
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
}
