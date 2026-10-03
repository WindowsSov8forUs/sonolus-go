package compiler

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"hash"
	"math"
	"reflect"
	"slices"
	"sync"

	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/ir"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/optimize"
)

// CallbackCache reuses optimized, source-independent IR between Compiler
// instances. Its zero value is ready for use. Successful compilations replace
// the previous generation; failed compilations do not publish cache entries.
// It is safe to share between concurrent compilers.
type CallbackCache struct {
	mu      sync.Mutex
	entries map[[32]byte]cachedCallback
}

type cachedCallback struct {
	function *ir.Function
	size     int
}

type callbackCacheSession struct {
	cache   *CallbackCache
	level   optimize.Level
	checks  RuntimeChecks
	base    map[[32]byte]cachedCallback // Immutable after publication.
	mu      sync.Mutex
	pending map[[32]byte]cachedCallback
	hits    int
}

func (cache *CallbackCache) begin(level optimize.Level, checks RuntimeChecks) *callbackCacheSession {
	if level == 0 {
		level = optimize.LevelStandard
	}
	cache.mu.Lock()
	base := cache.entries
	cache.mu.Unlock()
	return &callbackCacheSession{cache: cache, level: level, checks: checks, base: base, pending: make(map[[32]byte]cachedCallback)}
}

func (session *callbackCacheSession) optimize(optimizer *optimize.Optimizer, context optimize.Context, function *ir.Function) (*ir.Function, error) {
	if session == nil {
		return optimizer.Optimize(context, function)
	}
	// Include the complete typed IR: diagnostics, source positions, memory
	// layouts, purity, local types, and IEEE float bits all affect identity.
	encoder := cacheEncoder{hash: sha256.New()}
	ok := encoder.value(reflect.ValueOf(struct {
		Mode, Callback string
		Level          optimize.Level
		Checks         RuntimeChecks
		Function       *ir.Function
	}{string(context.Mode), context.Callback, session.level, session.checks, function}))
	if !ok {
		return optimizer.Optimize(context, function)
	}
	encoder.flush()
	var key [32]byte
	encoder.hash.Sum(key[:0])
	if entry, exists := session.base[key]; exists {
		session.mu.Lock()
		session.pending[key] = entry
		session.hits++
		session.mu.Unlock()
		return optimize.CloneFunction(entry.function), nil
	}
	result, err := optimizer.Optimize(context, function)
	if err != nil {
		return nil, err
	}
	measure := cacheEncoder{}
	if measure.value(reflect.ValueOf(result)) && measure.size <= maxCallbackCacheBytes {
		entry := cachedCallback{function: optimize.CloneFunction(result), size: measure.size}
		session.mu.Lock()
		session.pending[key] = entry
		session.mu.Unlock()
	}
	return result, nil
}

// Limits apply to entry count and encoded IR size, not process resident memory.
const maxCallbackCacheBytes = 32 << 20
const maxCallbackCacheEntries = 512

func (session *callbackCacheSession) commit() {
	keys := make([][32]byte, 0, len(session.pending))
	for key := range session.pending {
		keys = append(keys, key)
	}
	// Worker completion order must not determine which entries survive limits.
	slices.SortFunc(keys, func(a, b [32]byte) int { return bytes.Compare(a[:], b[:]) })
	entries := make(map[[32]byte]cachedCallback)
	size := 0
	for _, key := range keys {
		entry := session.pending[key]
		if len(entries) >= maxCallbackCacheEntries || size+entry.size > maxCallbackCacheBytes {
			continue
		}
		entries[key] = entry
		size += entry.size
	}
	session.cache.mu.Lock()
	session.cache.entries = entries
	session.cache.mu.Unlock()
}

// cacheEncoder is an in-process structural encoding, not a wire format.
// Concrete interface types are tagged; integer map keys are sorted. Reflection
// includes every IR field so adding metadata cannot silently omit it from the
// key. Unsupported future field kinds disable caching instead of losing data.
// A fixed buffer avoids materializing entire callback encodings. With no hash,
// the same traversal measures retained IR size without encoding any bytes.
type cacheEncoder struct {
	hash       hash.Hash
	buffer     [4096]byte
	used, size int
}

func (encoder *cacheEncoder) value(value reflect.Value) bool {
	switch value.Kind() {
	case reflect.Pointer, reflect.Interface:
		if value.IsNil() {
			encoder.word(0)
			return true
		}
		encoder.word(1)
		if value.Kind() == reflect.Interface {
			t := value.Elem().Type()
			encoder.text(t.PkgPath())
			encoder.text(t.String())
		}
		return encoder.value(value.Elem())
	case reflect.Struct:
		for i := 0; i < value.NumField(); i++ {
			if !encoder.value(value.Field(i)) {
				return false
			}
		}
	case reflect.Slice, reflect.Array:
		encoder.word(uint64(value.Len()))
		for i := 0; i < value.Len(); i++ {
			if !encoder.value(value.Index(i)) {
				return false
			}
		}
	case reflect.Map:
		if value.Type().Key().Kind() != reflect.Int {
			return false
		}
		keys := value.MapKeys()
		slices.SortFunc(keys, func(a, b reflect.Value) int {
			if a.Int() < b.Int() {
				return -1
			}
			if a.Int() > b.Int() {
				return 1
			}
			return 0
		})
		encoder.word(uint64(len(keys)))
		for _, key := range keys {
			encoder.word(uint64(key.Int()))
			if !encoder.value(value.MapIndex(key)) {
				return false
			}
		}
	case reflect.String:
		encoder.text(value.String())
	case reflect.Bool:
		if value.Bool() {
			encoder.word(1)
		} else {
			encoder.word(0)
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		encoder.word(uint64(value.Int()))
	case reflect.Float64:
		encoder.word(math.Float64bits(value.Float()))
	default:
		return false
	}
	return true
}

func (encoder *cacheEncoder) word(value uint64) {
	encoder.size += 8
	if encoder.hash == nil {
		return
	}
	if len(encoder.buffer)-encoder.used < 8 {
		encoder.flush()
	}
	binary.LittleEndian.PutUint64(encoder.buffer[encoder.used:], value)
	encoder.used += 8
}

func (encoder *cacheEncoder) text(value string) {
	encoder.word(uint64(len(value)))
	encoder.size += len(value)
	if encoder.hash == nil {
		return
	}
	for len(value) != 0 {
		if encoder.used == len(encoder.buffer) {
			encoder.flush()
		}
		n := copy(encoder.buffer[encoder.used:], value)
		encoder.used += n
		value = value[n:]
	}
}

func (encoder *cacheEncoder) flush() {
	_, _ = encoder.hash.Write(encoder.buffer[:encoder.used])
	encoder.used = 0
}
