package engine

import (
	"fmt"
	"io"
	"sort"
	"sync"
	"time"
)

// CompileStats records per-callback compilation timing for a single mode.
// All methods are safe for concurrent use.
type CompileStats struct {
	mu      sync.Mutex
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
	s.mu.Lock()
	s.entries = append(s.entries, compileStatsEntry{callback: callback, duration: d})
	s.mu.Unlock()
}

// Total returns the sum of all recorded durations.
func (s *CompileStats) Total() time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	var total time.Duration
	for _, e := range s.entries {
		total += e.duration
	}
	return total
}

// WriteSummary prints a sorted summary of compilation times to w.
// Each line has the format "  callback  duration".
func (s *CompileStats) WriteSummary(w io.Writer, mode string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.entries) == 0 {
		return
	}
	sort.Slice(s.entries, func(i, j int) bool {
		return s.entries[i].duration > s.entries[j].duration
	})
	// Compute total inline to avoid reentrant lock on s.Total().
	var total time.Duration
	for _, e := range s.entries {
		total += e.duration
	}
	fmt.Fprintf(w, "Compilation stats for %s:\n", mode)
	for _, e := range s.entries {
		fmt.Fprintf(w, "  %-30s %s\n", e.callback, e.duration.Round(time.Microsecond))
	}
	fmt.Fprintf(w, "  %-30s %s\n", "total", total.Round(time.Microsecond))
}
