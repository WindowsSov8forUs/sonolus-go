package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
)

// CacheKey identifies a compiled artifact by hashing (mode, source).
// Different modes with the same source produce different keys.
type CacheKey struct {
	mode string
	hash string
}

// NewCacheKey returns a deterministic cache key for the given mode and source.
func NewCacheKey(mode, src string) CacheKey {
	h := sha256.Sum256([]byte(src))
	return CacheKey{mode: mode, hash: hex.EncodeToString(h[:])}
}

// CompileCache stores compiled engine data indexed by CacheKey, enabling
// fast recompilation when source has not changed (e.g. in the dev server).
// All methods are safe for concurrent use.
type CompileCache struct {
	mu       sync.RWMutex
	play     map[CacheKey]*resource.EnginePlayData
	watch    map[CacheKey]*resource.EngineWatchData
	preview  map[CacheKey]*resource.EnginePreviewData
	tutorial map[CacheKey]*resource.EngineTutorialData
	config   map[CacheKey]*resource.EngineConfiguration
}

// NewCache creates an empty compile cache.
func NewCache() *CompileCache {
	return &CompileCache{
		play:     make(map[CacheKey]*resource.EnginePlayData),
		watch:    make(map[CacheKey]*resource.EngineWatchData),
		preview:  make(map[CacheKey]*resource.EnginePreviewData),
		tutorial: make(map[CacheKey]*resource.EngineTutorialData),
		config:   make(map[CacheKey]*resource.EngineConfiguration),
	}
}

func (c *CompileCache) GetPlay(key CacheKey) (*resource.EnginePlayData, *resource.EngineConfiguration) {
	d, dok := getKeyed(c, c.play, key)
	cfg, cok := getKeyed(c, c.config, key)
	if !dok {
		return nil, nil
	}
	if !cok {
		return d, nil
	}
	return d, cfg
}

func (c *CompileCache) PutPlay(key CacheKey, data *resource.EnginePlayData, cfg *resource.EngineConfiguration) {
	putKeyed(c, c.play, key, data)
	if cfg != nil {
		c.mu.Lock()
		defer c.mu.Unlock()
		c.config[key] = cfg
	}
}

// getKeyed is a generic helper for map lookups under the read lock.
func getKeyed[T any](c *CompileCache, m map[CacheKey]T, key CacheKey) (T, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := m[key]
	return v, ok
}

// putKeyed is a generic helper for map stores under the write lock.
func putKeyed[T any](c *CompileCache, m map[CacheKey]T, key CacheKey, data T) {
	c.mu.Lock()
	defer c.mu.Unlock()
	m[key] = data
}

func (c *CompileCache) GetWatch(key CacheKey) *resource.EngineWatchData {
	d, ok := getKeyed(c, c.watch, key)
	if !ok {
		return nil
	}
	return d
}

func (c *CompileCache) PutWatch(key CacheKey, data *resource.EngineWatchData) {
	putKeyed(c, c.watch, key, data)
}

func (c *CompileCache) GetPreview(key CacheKey) *resource.EnginePreviewData {
	d, ok := getKeyed(c, c.preview, key)
	if !ok {
		return nil
	}
	return d
}

func (c *CompileCache) PutPreview(key CacheKey, data *resource.EnginePreviewData) {
	putKeyed(c, c.preview, key, data)
}

func (c *CompileCache) GetTutorial(key CacheKey) *resource.EngineTutorialData {
	d, ok := getKeyed(c, c.tutorial, key)
	if !ok {
		return nil
	}
	return d
}

func (c *CompileCache) PutTutorial(key CacheKey, data *resource.EngineTutorialData) {
	putKeyed(c, c.tutorial, key, data)
}
