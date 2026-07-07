package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
)

const defaultMaxCacheEntries = 256

// CacheKey identifies a compiled artifact by hashing (mode, opt level, namespace, source).
// Different modes, opt levels, or namespaces with the same source produce different keys.
type CacheKey struct {
	mode string
	opt  int // optimization level (0=minimal, 1=fast, 2=standard)
	hash string
}

// NewCacheKey returns a deterministic cache key for the given mode, opt level,
// source, and optional namespace(s). The namespace distinguishes otherwise-
// identical source that belongs to different files or compilation contexts
// (e.g. dev server path). When no namespace is given, the key only depends on
// mode, opt level, and source content.
func NewCacheKey(mode string, optLevel int, src string, namespace ...string) CacheKey {
	h := sha256.New()
	h.Write([]byte(mode))
	h.Write([]byte{0})
	h.Write([]byte{byte(optLevel)})
	h.Write([]byte{0})
	for _, ns := range namespace {
		h.Write([]byte(ns))
		h.Write([]byte{0})
	}
	h.Write([]byte(src))
	sum := h.Sum(nil)
	return CacheKey{mode: mode, opt: optLevel, hash: hex.EncodeToString(sum)}
}

// CompileCache stores compiled engine data indexed by CacheKey, enabling
// fast recompilation when source has not changed (e.g. in the dev server).
// All methods are safe for concurrent use.
//
// MaxEntriesPerMode controls the maximum number of entries per mode map. When
// a map exceeds this limit, the oldest entry (FIFO order) is evicted. Total
// cache capacity is MaxEntriesPerMode × 5 (one map per mode + config).
// Set to 0 to disable eviction (unbounded growth). Default: 256.
type CompileCache struct {
	mu                sync.RWMutex
	MaxEntriesPerMode int
	play              map[CacheKey]*resource.EnginePlayData
	playOrder         []CacheKey // FIFO insertion order for eviction
	watch             map[CacheKey]*resource.EngineWatchData
	watchOrder        []CacheKey
	preview           map[CacheKey]*resource.EnginePreviewData
	previewOrder      []CacheKey
	tutorial          map[CacheKey]*resource.EngineTutorialData
	tutorialOrder     []CacheKey
	config            map[CacheKey]*resource.EngineConfiguration
	configOrder       []CacheKey
}

// NewCache creates an empty compile cache with the default max entries (256).
func NewCache() *CompileCache {
	return &CompileCache{
		MaxEntriesPerMode: defaultMaxCacheEntries,
		play:              make(map[CacheKey]*resource.EnginePlayData),
		watch:             make(map[CacheKey]*resource.EngineWatchData),
		preview:           make(map[CacheKey]*resource.EnginePreviewData),
		tutorial:          make(map[CacheKey]*resource.EngineTutorialData),
		config:            make(map[CacheKey]*resource.EngineConfiguration),
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

// PutPlay stores Play-mode compilation results in the cache. When MaxEntriesPerMode
// is exceeded, the oldest entry (FIFO) is evicted from both play and config maps.
func (c *CompileCache) PutPlay(key CacheKey, data *resource.EnginePlayData, cfg *resource.EngineConfiguration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Evict oldest from play map if needed and key is new.
	if _, exists := c.play[key]; !exists {
		if c.MaxEntriesPerMode > 0 && len(c.play) >= c.MaxEntriesPerMode {
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
		// Config shares the play eviction policy: when an entry is evicted
		// from play, its config is also removed. No independent eviction here.
		if _, exists := c.config[key]; !exists {
			c.configOrder = append(c.configOrder, key)
		}
		c.config[key] = cfg
	}
}

// removeFromOrder removes the first occurrence of key from a FIFO order slice.
// It is used by PutPlay for config-order eviction bookkeeping (putKeyed uses
// inline eviction since its map and order are generic type parameters).
func removeFromOrder(order []CacheKey, key CacheKey) []CacheKey {
	for i, k := range order {
		if k == key {
			return append(order[:i], order[i+1:]...)
		}
	}
	return order
}

// putKeyed is a generic helper for map stores under the write lock.
// When MaxEntriesPerMode > 0 and the map reaches the limit, the oldest entry
// (FIFO order) is evicted before inserting the new one.
func putKeyed[T any](c *CompileCache, m map[CacheKey]T, order *[]CacheKey, key CacheKey, data T) {
	// Re-insert of existing key: update value, keep original order.
	if _, exists := m[key]; exists {
		m[key] = data
		return
	}
	if c.MaxEntriesPerMode > 0 && len(m) >= c.MaxEntriesPerMode {
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
