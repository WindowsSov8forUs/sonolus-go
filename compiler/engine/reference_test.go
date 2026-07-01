// Package engine — reference comparison tests against sonolus.js-compiler.
//
// These tests verify semantic equivalence between sonolus-go and
// sonolus.js-compiler by compiling equivalent engine logic in both
// and comparing normalized SNode output.
//
// Prerequisites (tests auto-skip if unmet):
//  1. sonolus.js-compiler repo at ../../sonolus.js-compiler
//  2. Node.js >= 18 available on PATH
//  3. cd ../../sonolus.js-compiler && npm install && npm run build
//
// Run with:
//
//	SONOLUS_JS_COMPILER=1 go test ./compiler/engine/ -run TestReference -v
package engine

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
)

// referenceTestEnv describes the runtime environment for reference comparison.
type referenceTestEnv struct {
	jsCompilerDir string // path to sonolus.js-compiler
	bridgeScript  string // path to compile.mjs bridge
	skipReason    string // if non-empty, why tests are skipped
}

// setupReferenceEnv checks prerequisites and returns the environment.
func setupReferenceEnv(t *testing.T) *referenceTestEnv {
	t.Helper()

	if os.Getenv("SONOLUS_JS_COMPILER") == "" {
		return &referenceTestEnv{skipReason: "SONOLUS_JS_COMPILER not set (opt-in via env var)"}
	}

	jsCompilerDir := filepath.Join("..", "..", "sonolus.js-compiler")
	if _, err := os.Stat(jsCompilerDir); os.IsNotExist(err) {
		return &referenceTestEnv{skipReason: "sonolus.js-compiler not found at " + jsCompilerDir}
	}

	// Check that dist/ exists (built).
	distPlay := filepath.Join(jsCompilerDir, "dist", "index.play.js")
	if _, err := os.Stat(distPlay); os.IsNotExist(err) {
		return &referenceTestEnv{skipReason: "sonolus.js-compiler dist/ not built; run: cd " + jsCompilerDir + " && npm install && npm run build"}
	}

	// Check Node.js availability.
	if _, err := exec.LookPath("node"); err != nil {
		return &referenceTestEnv{skipReason: "node not found on PATH"}
	}

	bridgeScript := filepath.Join("testdata", "reference", "compile.mjs")
	if _, err := os.Stat(bridgeScript); os.IsNotExist(err) {
		return &referenceTestEnv{skipReason: "bridge script not found at " + bridgeScript}
	}

	return &referenceTestEnv{jsCompilerDir: jsCompilerDir, bridgeScript: bridgeScript}
}

// compileJSBridge invokes the Node.js bridge script to compile TypeScript
// engine source and returns the resulting EnginePlayData.
func (env *referenceTestEnv) compileJSBridge(tsSource string) (*resource.EnginePlayData, error) {
	cmd := exec.Command("node", env.bridgeScript, env.jsCompilerDir)
	cmd.Dir = filepath.Join("testdata", "reference")
	cmd.Stdin = nil // source passed via temp file

	// Write source to temp file passed as argument.
	tmpFile, err := os.CreateTemp("", "sonolus-ref-*.ts")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmpFile.Name())
	if _, err := tmpFile.WriteString(tsSource); err != nil {
		tmpFile.Close()
		return nil, err
	}
	tmpFile.Close()

	cmd.Args = append(cmd.Args, tmpFile.Name())
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, &exec.ExitError{Stderr: exitErr.Stderr}
		}
		return nil, err
	}

	var data resource.EnginePlayData
	if err := json.Unmarshal(out, &data); err != nil {
		return nil, err
	}
	return &data, nil
}

// TestReferenceComparison compiles equivalent engine logic in both Go and
// TypeScript using sonolus.js-compiler, then compares normalized SNode output.
// It is an opt-in test controlled by the SONOLUS_JS_COMPILER env var.
func TestReferenceComparison(t *testing.T) {
	env := setupReferenceEnv(t)
	if env.skipReason != "" {
		t.Skip(env.skipReason)
	}

	// Load paired test cases from testdata/reference/.
	entries, err := os.ReadDir("testdata/reference")
	if err != nil {
		t.Fatal(err)
	}

	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == "compile.mjs" {
			continue
		}
		t.Run(entry.Name(), func(t *testing.T) {
			caseDir := filepath.Join("testdata", "reference", entry.Name())
			goSrc, err := os.ReadFile(filepath.Join(caseDir, "engine.go"))
			if err != nil {
				t.Fatalf("read engine.go: %v", err)
			}
			tsSrc, err := os.ReadFile(filepath.Join(caseDir, "engine.ts"))
			if err != nil {
				t.Fatalf("read engine.ts: %v", err)
			}

			// Compile Go.
			goData, _, err := CompilePlayFile(string(goSrc))
			if err != nil {
				t.Fatalf("go compile: %v", err)
			}

			// Compile TS via bridge.
			jsData, err := env.compileJSBridge(string(tsSrc))
			if err != nil {
				t.Fatalf("js compile: %v", err)
			}

			// Compare normalized SNode trees.
			diffs := compareSNodeTrees(goData.Nodes, jsData.Nodes)
			for _, d := range diffs {
				t.Error(d)
			}
			if len(diffs) == 0 {
				t.Logf("SNode trees match (%d nodes)", len(goData.Nodes))
			}
		})
	}
}
