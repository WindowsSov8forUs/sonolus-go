package engine

import (
	"strings"
	"testing"
)

// TestCompilePlayFileParseError verifies that invalid Go syntax produces a
// parse error from CompilePlayFile.
func TestCompilePlayFileParseError(t *testing.T) {
	_, _, err := CompilePlayFile("package p\nfunc {")
	if err == nil {
		t.Fatal("expected parse error for invalid syntax")
	}
	if !strings.Contains(err.Error(), "expected") && !strings.Contains(err.Error(), "parse") {
		t.Errorf("error should be a syntax error: %v", err)
	}
}

// TestCompileWatchFileParseError verifies parse error propagation for Watch.
func TestCompileWatchFileParseError(t *testing.T) {
	_, err := CompileWatchFile("package p\nfunc {")
	if err == nil {
		t.Fatal("expected parse error for invalid syntax")
	}
	if !strings.Contains(err.Error(), "expected") && !strings.Contains(err.Error(), "parse") {
		t.Errorf("error should be a syntax error: %v", err)
	}
}

// TestCompilePreviewFileParseError verifies parse error propagation for Preview.
func TestCompilePreviewFileParseError(t *testing.T) {
	_, err := CompilePreviewFile("package p\nfunc {")
	if err == nil {
		t.Fatal("expected parse error for invalid syntax")
	}
	if !strings.Contains(err.Error(), "expected") && !strings.Contains(err.Error(), "parse") {
		t.Errorf("error should be a syntax error: %v", err)
	}
}

// TestCompileTutorialFileParseError verifies parse error propagation for Tutorial.
func TestCompileTutorialFileParseError(t *testing.T) {
	_, err := CompileTutorialFile("package p\nfunc {")
	if err == nil {
		t.Fatal("expected parse error for invalid syntax")
	}
	if !strings.Contains(err.Error(), "expected") && !strings.Contains(err.Error(), "parse") {
		t.Errorf("error should be a syntax error: %v", err)
	}
}

// TestCompilePlayFileUndefinedIdent verifies that an undefined identifier in a
// callback body produces an error from the frontend trace (not TypeCheck).
func TestCompilePlayFileUndefinedIdent(t *testing.T) {
	src := "package p\n" +
		"type Note struct {\n\tBeat float64 `sonolus:\"imported\"`\n}\n" +
		"func (n Note) Preprocess() { set(2000, 0, undefinedVar) }\n"
	_, _, err := CompilePlayFile(src)
	if err == nil {
		t.Fatal("expected error for undefined identifier")
	}
	if !strings.Contains(err.Error(), "typecheck") && !strings.Contains(err.Error(), "undefined") {
		t.Errorf("error should mention 'typecheck' or 'undefined': %v", err)
	}
}

// TestCompileWatchFileUndefinedIdent verifies that undefined identifiers in
// Watch callbacks are caught by the frontend trace.
func TestCompileWatchFileUndefinedIdent(t *testing.T) {
	src := "package p\n" +
		"type Note struct {\n\tBeat float64 `sonolus:\"imported\"`\n}\n" +
		"func (n Note) Preprocess() { set(2000, 0, undefinedVar) }\n"
	_, err := CompileWatchFile(src)
	if err == nil {
		t.Fatal("expected error for undefined identifier")
	}
	if !strings.Contains(err.Error(), "typecheck") && !strings.Contains(err.Error(), "undefined") {
		t.Errorf("error should mention 'typecheck' or 'undefined': %v", err)
	}
}

// TestCompilePreviewFileUndefinedIdent verifies that undefined identifiers in
// Preview callbacks are caught by the frontend trace.
func TestCompilePreviewFileUndefinedIdent(t *testing.T) {
	src := "package p\n" +
		"type Line struct {\n\tBeat float64 `sonolus:\"imported\"`\n}\n" +
		"func (l Line) Preprocess() { set(2000, 0, undefinedVar) }\n"
	_, err := CompilePreviewFile(src)
	if err == nil {
		t.Fatal("expected error for undefined identifier")
	}
	if !strings.Contains(err.Error(), "typecheck") && !strings.Contains(err.Error(), "undefined") {
		t.Errorf("error should mention 'typecheck' or 'undefined': %v", err)
	}
}

// TestCompileTutorialFileUndefinedIdent verifies that undefined identifiers in
// Tutorial functions are caught by the frontend trace.
func TestCompileTutorialFileUndefinedIdent(t *testing.T) {
	src := "package p\n" +
		"func Preprocess() { set(2000, 0, undefinedVar) }\n"
	_, err := CompileTutorialFile(src)
	if err == nil {
		t.Fatal("expected error for undefined identifier")
	}
	if !strings.Contains(err.Error(), "typecheck") && !strings.Contains(err.Error(), "undefined") {
		t.Errorf("error should mention 'typecheck' or 'undefined': %v", err)
	}
}

// TypeCheck errors are now propagated for hard errors (undefined identifiers
// and wrong argument counts) while soft errors (float64 indices, implicit
// callback returns — Go DSL mismatches) are filtered via filterHardErrors.
// The prelude declares engine builtins (time, array, entityPerfect, etc.)
// so TypeCheck only reports genuinely undeclared user identifiers.
