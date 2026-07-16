package backend

import (
	"encoding/binary"
	"math"
)

func buildROM(user []byte) []byte {
	result := make([]byte, 12+len(user))
	binary.LittleEndian.PutUint32(result[0:4], math.Float32bits(float32(math.NaN())))
	binary.LittleEndian.PutUint32(result[4:8], math.Float32bits(float32(math.Inf(1))))
	binary.LittleEndian.PutUint32(result[8:12], math.Float32bits(float32(math.Inf(-1))))
	copy(result[12:], user)
	return result
}
