// Package frontend — sentinel errors for programmatic error discrimination.
package frontend

import "errors"

// Sentinel errors returned by CompileBlock and related functions. Callers can
// use errors.Is to distinguish error categories without string-matching.
var (
	// ErrNilBody is returned when CompileBlock receives a nil function body.
	ErrNilBody = errors.New("frontend: nil function body")

	// ErrUnsupported is returned when the engine source uses a Go construct that
	// is not supported by the Sonolus Go subset (e.g. defer, go, channel, map).
	ErrUnsupported = errors.New("frontend: unsupported Go construct")
)
