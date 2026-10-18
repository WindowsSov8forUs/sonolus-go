package ir

import (
	"encoding/binary"
	"encoding/hex"
	"math"
	"strings"
	"sync"
)

// ConstantTablePrefix prefixes the placeholder storage of a read-only constant
// table destined for EngineRom. The suffix is the table's little-endian
// binary32 bytes in hex, so equal tables share a placeholder, the optimizer can
// fold reads at constant indexes, and the final ROM layout depends only on the
// tables still read after optimization.
const ConstantTablePrefix = "EngineRom@"

// ConstantTableStorage returns the placeholder storage for values, which must
// all be exactly representable in binary32.
func ConstantTableStorage(values []float64) string {
	bytes := make([]byte, 4*len(values))
	for position, value := range values {
		binary.LittleEndian.PutUint32(bytes[4*position:], math.Float32bits(float32(value)))
	}
	return ConstantTablePrefix + hex.EncodeToString(bytes)
}

var constantTables sync.Map

// ConstantTable decodes the values of a placeholder storage.
func ConstantTable(storage string) ([]float64, bool) {
	key, ok := strings.CutPrefix(storage, ConstantTablePrefix)
	if !ok {
		return nil, false
	}
	if values, ok := constantTables.Load(key); ok {
		return values.([]float64), true
	}
	bytes, err := hex.DecodeString(key)
	if err != nil || len(bytes)%4 != 0 {
		return nil, false
	}
	values := make([]float64, len(bytes)/4)
	for position := range values {
		values[position] = float64(math.Float32frombits(binary.LittleEndian.Uint32(bytes[4*position:])))
	}
	constantTables.Store(key, values)
	return values, true
}

// ConstantTableValue returns the value a constant-table place reads at a
// constant index.
func ConstantTableValue(place MemoryPlace, index float64) (float64, bool) {
	values, ok := ConstantTable(place.Storage)
	if !ok || index != math.Trunc(index) {
		return 0, false
	}
	position := int(index)*place.Stride + place.Offset
	if index < 0 || position < 0 || position >= len(values) {
		return 0, false
	}
	return values[position], true
}
