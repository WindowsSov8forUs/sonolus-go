package build

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"os"

	"github.com/WindowsSov8forUs/sonolus-core-go/codec"
)

var (
	// ErrROMBadSize is returned when a ROM file size is not a multiple of 4 (expected raw float32).
	ErrROMBadSize = errors.New("ROM file size is not a multiple of 4 (expected raw float32)")
	// ErrROMRead is returned when a ROM file cannot be read.
	ErrROMRead = errors.New("failed to read ROM file")
)

// BuildROM produces the gzipped EngineRom file from a list of float32 values.
// The minimum ROM has NaN at index 0, +Inf at 1, -Inf at 2.
func BuildROM(values []float32) ([]byte, error) {
	raw := make([]byte, 4*len(values))
	for i, v := range values {
		binary.LittleEndian.PutUint32(raw[4*i:], math.Float32bits(v))
	}
	return codec.Compress(raw)
}

// BuildROMFromFile reads a binary file of raw float32 values and builds a ROM.
// The file must be a multiple of 4 bytes. On success the returned byte slice is
// gzipped and ready for packaging.
func BuildROMFromFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("%w: %q: %w", ErrROMRead, path, err)
	}
	if len(data)%4 != 0 {
		return nil, fmt.Errorf("%w: %q size %d", ErrROMBadSize, path, len(data))
	}
	values := make([]float32, len(data)/4)
	for i := range values {
		values[i] = math.Float32frombits(binary.LittleEndian.Uint32(data[4*i:]))
	}
	return BuildROM(values)
}

// DefaultROM returns a ROM with the three required non-finite slots.
func DefaultROM() []float32 {
	return []float32{
		float32(math.NaN()),   // index 0
		float32(math.Inf(1)),  // index 1
		float32(math.Inf(-1)), // index 2
	}
}

// DefaultROMBytes builds and returns the gzipped default ROM in one call.
func DefaultROMBytes() ([]byte, error) {
	return BuildROM(DefaultROM())
}
