package engine

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestCompileStats_RecordNilSafe(t *testing.T) {
	var s *CompileStats
	// Must not panic on nil receiver.
	s.Record("preprocess", time.Millisecond)
}

func TestCompileStats_Total_Empty(t *testing.T) {
	s := &CompileStats{}
	if got := s.Total(); got != 0 {
		t.Errorf("Total() on empty stats = %v, want 0", got)
	}
}

func TestCompileStats_Total_WithEntries(t *testing.T) {
	s := &CompileStats{}
	s.Record("a", 10*time.Millisecond)
	s.Record("b", 20*time.Millisecond)
	s.Record("c", 30*time.Millisecond)
	if got := s.Total(); got != 60*time.Millisecond {
		t.Errorf("Total() = %v, want 60ms", got)
	}
}

func TestCompileStats_WriteSummary_Empty(t *testing.T) {
	var buf bytes.Buffer

	// nil stats
	var s *CompileStats
	s.WriteSummary(&buf, "play")
	if buf.Len() != 0 {
		t.Errorf("nil stats should produce no output, got %q", buf.String())
	}

	// empty stats
	s2 := &CompileStats{}
	s2.WriteSummary(&buf, "play")
	if buf.Len() != 0 {
		t.Errorf("empty stats should produce no output, got %q", buf.String())
	}
}

func TestCompileStats_WriteSummary_Sorted(t *testing.T) {
	s := &CompileStats{}
	s.Record("fast", time.Microsecond)
	s.Record("slow", time.Second)
	s.Record("medium", time.Millisecond)

	var buf bytes.Buffer
	s.WriteSummary(&buf, "play")

	output := buf.String()
	// Must contain all three callbacks and a total line.
	for _, name := range []string{"fast", "slow", "medium", "total"} {
		if !strings.Contains(output, name) {
			t.Errorf("output missing %q: %s", name, output)
		}
	}
	// Must be sorted descending: slow before medium before fast before total.
	slowPos := strings.Index(output, "slow")
	mediumPos := strings.Index(output, "medium")
	fastPos := strings.Index(output, "fast")
	totalPos := strings.Index(output, "total")
	if slowPos == -1 || mediumPos == -1 || fastPos == -1 || totalPos == -1 {
		t.Fatal("missing expected output line")
	}
	if !(slowPos < mediumPos && mediumPos < fastPos && fastPos < totalPos) {
		t.Errorf("output not sorted descending: %s", output)
	}
}

func TestCompileStats_WriteSummary_NilSafe(t *testing.T) {
	var s *CompileStats
	var buf bytes.Buffer
	// Must not panic on nil receiver.
	s.WriteSummary(&buf, "watch")
}
