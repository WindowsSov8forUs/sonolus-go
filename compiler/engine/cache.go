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
//
// MaxEntries controls the maximum number of entries per mode map. When a map
// exceeds this limit, the oldest entry (FIFO order) is evicted.
// Set to 0 to disable eviction (unbounded growth). Default: 256.
type CompileCache struct {
	mu         sync.RWMutex
	MaxEntries int
	play       map[CacheKey]*resource.EnginePlayData
	playOrder  []CacheKey // FIFO insertion order for eviction
	watch      map[CacheKey]*resource.EngineWatchData
	watchOrder []CacheKey
	preview    map[CacheKey]*resource.EnginePreviewData
	previewOrder []CacheKey
	tutorial   map[CacheKey]*resource.EngineTutorialData
	tutorialOrder []CacheKey
	config     map[CacheKey]*resource.EngineConfiguration
	configOrder []CacheKey
}

// NewCache creates an empty compile cache with the default max entries (256).
func NewCache() *CompileCache {
	return &CompileCache{
		MaxEntries: 256,
		play:       make(map[CacheKey]*resource.EnginePlayData),
		watch:      make(map[CacheKey]*resource.EngineWatchData),
		preview:    make(map[CacheKey]*resource.EnginePreviewData),
		tutorial:   make(map[CacheKey]*resource.EngineTutorialData),
		config:     make(map[CacheKey]*resource.EngineConfiguration),
	}
}

// GetPlay returns cached Play-mode compilation results for the given key, or nil
// if not cached.
func (c *CompileCache) GetPlay(key CacheKey) (*resource.EnginePlayData, *resource.EngineConfiguration) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	d, dok := c.play[key]
	cfg, cok := c.config[key]
	if !dok {
		return nil, nil
	}
	if !cok {
		return d, nil
	}
	return d, cfg
}

// PutPlay stores Play-mode compilation results in the cache. When MaxEntries
// is exceeded, the oldest entry (FIFO) is evicted from both play and config maps.
func (c *CompileCache) PutPlay(key CacheKey, data *resource.EnginePlayData, cfg *resource.EngineConfiguration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Evict oldest from play map if needed and key is new.
	if _, exists := c.play[key]; !exists {
		if c.MaxEntries > 0 && len(c.play) >= c.MaxEntries {
			oldest := c.playOrder[0]
			c.playOrder = c.playOrder[1:]
			delete(c.play, oldest)
			delete(c.config, oldest)
			c.configOrder = removeFromOrder(c.configOrder, oldest)
		}
		c.playOrder = append(c.playOrder, key)
	}
	c.play[key] = data
	if cfg != nil {
		if _, exists := c.config[key]; !exists {
			if c.MaxEntries > 0 && len(c.config) >= c.MaxEntries {
				oldest := c.configOrder[0]
				c.configOrder = c.configOrder[1:]
				delete(c.config, oldest)
				delete(c.play, oldest)
				c.playOrder = removeFromOrder(c.playOrder, oldest)
			}
			c.configOrder = append(c.configOrder, key)
		}
		c.config[key] = cfg
	}
}

// removeFromOrder removes the first occurrence of key from a FIFO order slice.
// It is used by both putKeyed and putPlayKeyed for eviction bookkeeping.
func removeFromOrder(order []CacheKey, key CacheKey) []CacheKey {
	for i, k := range order {
		if k == key {
			return append(order[:i], order[i+1:]...)
		}
	}
	return order
}

// putKeyed is a generic helper for map stores under the write lock.
// When MaxEntries > 0 and the map reaches the limit, the oldest entry
// (FIFO order) is evicted before inserting the new one.
func putKeyed[T any](c *CompileCache, m map[CacheKey]T, order *[]CacheKey, key CacheKey, data T) {
	// Re-insert of existing key: update value, keep original order.
	if _, exists := m[key]; exists {
		m[key] = data
		return
	}
	if c.MaxEntries > 0 && len(m) >= c.MaxEntries {
		oldest := (*order)[0]
		*order = (*order)[1:]
		delete(m, oldest)
	}
	*order = append(*order, key)
	m[key] = data
}

// GetWatch returns cached Watch-mode compilation results for the given key, or nil
// if not cached.
func (c *CompileCache) GetWatch(key CacheKey) *resource.EngineWatchData {
	d, ok := getKeyed(c, c.watch, key)
	if !ok {
		return nil
	}
	return d
}

// PutWatch stores Watch-mode compilation results in the cache.
func (c *CompileCache) PutWatch(key CacheKey, data *resource.EngineWatchData) {
	c.mu.Lock()
	defer c.mu.Unlock()
	putKeyed(c, c.watch, &c.watchOrder, key, data)
}

// GetPreview returns cached Preview-mode compilation results for the given key,
// or nil if not cached.
func (c *CompileCache) GetPreview(key CacheKey) *resource.EnginePreviewData {
	d, ok := getKeyed(c, c.preview, key)
	if !ok {
		return nil
	}
	return d
}

// PutPreview stores Preview-mode compilation results in the cache.
func (c *CompileCache) PutPreview(key CacheKey, data *resource.EnginePreviewData) {
	c.mu.Lock()
	defer c.mu.Unlock()
	putKeyed(c, c.preview, &c.previewOrder, key, data)
}

// GetTutorial returns cached Tutorial-mode compilation results for the given key,
// or nil if not cached.
func (c *CompileCache) GetTutorial(key CacheKey) *resource.EngineTutorialData {
	d, ok := getKeyed(c, c.tutorial, key)
	if !ok {
		return nil
	}
	return d
}

// PutTutorial stores Tutorial-mode compilation results in the cache.
func (c *CompileCache) PutTutorial(key CacheKey, data *resource.EngineTutorialData) {
	c.mu.Lock()
	defer c.mu.Unlock()
	putKeyed(c, c.tutorial, &c.tutorialOrder, key, data)
}

// getKeyed is a generic helper for map lookups under the read lock.
func getKeyed[T any](c *CompileCache, m map[CacheKey]T, key CacheKey) (T, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := m[key]
	return v, ok
}
