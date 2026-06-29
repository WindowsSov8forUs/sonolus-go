package engine

import (
	"fmt"
	"io"
	"sort"
	"time"
)

// CompileStats records per-callback compilation timing for a single mode.
type CompileStats struct {
	entries []compileStatsEntry
}

type compileStatsEntry struct {
	callback string
	duration time.Duration
}

// Record adds a timing entry for a compiled callback.
func (s *CompileStats) Record(callback string, d time.Duration) {
	if s == nil {
		return
	}
	s.entries = append(s.entries, compileStatsEntry{callback: callback, duration: d})
}

// Total returns the sum of all recorded durations.
func (s *CompileStats) Total() time.Duration {
	var total time.Duration
	for _, e := range s.entries {
		total += e.duration
	}
	return total
}

// WriteSummary prints a sorted summary of compilation times to w.
// Each line has the format "  callback  duration".
func (s *CompileStats) WriteSummary(w io.Writer, mode string) {
	if s == nil || len(s.entries) == 0 {
		return
	}
	sort.Slice(s.entries, func(i, j int) bool {
		return s.entries[i].duration > s.entries[j].duration
	})
	fmt.Fprintf(w, "Compilation stats for %s:\n", mode)
	for _, e := range s.entries {
		fmt.Fprintf(w, "  %-30s %s\n", e.callback, e.duration.Round(time.Microsecond))
	}
	fmt.Fprintf(w, "  %-30s %s\n", "total", s.Total().Round(time.Microsecond))
}
