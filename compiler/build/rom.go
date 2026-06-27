package build

import (
	"encoding/binary"
	"math"

	"github.com/WindowsSov8forUs/sonolus-core-go/codec"
)

// BuildRom produces the gzipped EngineRom file from a list of float32 values.
// The minimum ROM has NaN at index 0, +Inf at 1, -Inf at 2.
func BuildRom(values []float32) ([]byte, error) {
	raw := make([]byte, 4*len(values))
	for i, v := range values {
		binary.LittleEndian.PutUint32(raw[4*i:], math.Float32bits(v))
	}
	return codec.Compress(raw)
}

// DefaultRom returns a ROM with the three required non-finite slots.
func DefaultRom() []float32 {
	return []float32{
		float32(math.NaN()),  // index 0
		float32(math.Inf(1)), // index 1
		float32(math.Inf(-1)), // index 2
	}
}
