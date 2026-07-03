package engine

import (
	"fmt"
	"sync"
	"testing"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
)

func TestCacheKeyDeterministic(t *testing.T) {
	k1 := NewCacheKey("play", 0, "package main\nfunc main() {}")
	k2 := NewCacheKey("play", 0, "package main\nfunc main() {}")
	if k1 != k2 {
		t.Errorf("same input produced different keys: %v vs %v", k1, k2)
	}
}

func TestCacheKeyModeDiffers(t *testing.T) {
	src := "package main\nfunc main() {}"
	k1 := NewCacheKey("play", 0, src)
	k2 := NewCacheKey("watch", 0, src)
	if k1 == k2 {
		t.Errorf("different modes produced same key: %v", k1)
	}
}

func TestCacheKeySourceDiffers(t *testing.T) {
	k1 := NewCacheKey("play", 0, "a")
	k2 := NewCacheKey("play", 0, "b")
	if k1 == k2 {
		t.Errorf("different sources produced same key: %v", k1)
	}
}

func TestCompileCachePutGet(t *testing.T) {
	c := NewCache()
	key := NewCacheKey("play", 0, "test")
	if d, cfg := c.GetPlay(key); d != nil || cfg != nil {
		t.Error("empty cache returned non-nil")
	}
	dummy := &resource.EnginePlayData{}
	c.PutPlay(key, dummy, nil)
	d, cfg := c.GetPlay(key)
	if d != dummy || cfg != nil {
		t.Error("cache returned wrong value after put")
	}
}

func TestCompileCacheConcurrent(t *testing.T) {
	c := NewCache()
	c.MaxEntriesPerMode = 16

	const N = 20
	var wg sync.WaitGroup
	errs := make(chan error, N)
	for i := range N {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			key := NewCacheKey("play", 0, fmt.Sprintf("src-%d", n))
			data := &resource.EnginePlayData{}
			cfg := &resource.EngineConfiguration{}
			c.PutPlay(key, data, cfg)
			got, gotCfg := c.GetPlay(key)
			if got == nil {
				errs <- fmt.Errorf("concurrent: get returned nil for %d", n)
				return
			}
			if gotCfg == nil {
				errs <- fmt.Errorf("concurrent: config returned nil for %d", n)
				return
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Error(e)
	}
}

func TestCompileCacheEviction(t *testing.T) {
	c := NewCache()
	c.MaxEntriesPerMode = 2

	c.PutPlay(NewCacheKey("play", 0, "a"), &resource.EnginePlayData{}, &resource.EngineConfiguration{})
	c.PutPlay(NewCacheKey("play", 0, "b"), &resource.EnginePlayData{}, &resource.EngineConfiguration{})
	c.PutPlay(NewCacheKey("play", 0, "c"), &resource.EnginePlayData{}, &resource.EngineConfiguration{})

	// The most recently inserted key should still be present.
	if _, cfg := c.GetPlay(NewCacheKey("play", 0, "c")); cfg == nil {
		t.Error("most recently inserted key should be present after eviction")
	}
}

func TestCompileCacheNonPlayEviction(t *testing.T) {
	c := NewCache()
	c.MaxEntriesPerMode = 2

	// Watch eviction.
	for i := range 3 {
		c.PutWatch(NewCacheKey("watch", 0, fmt.Sprintf("w%d", i)), &resource.EngineWatchData{})
	}
	if d := c.GetWatch(NewCacheKey("watch", 0, "w0")); d != nil {
		t.Error("watch: oldest entry should have been evicted")
	}
	if d := c.GetWatch(NewCacheKey("watch", 0, "w2")); d == nil {
		t.Error("watch: most recent entry should still be present")
	}

	// Preview eviction.
	for i := range 3 {
		c.PutPreview(NewCacheKey("preview", 0, fmt.Sprintf("p%d", i)), &resource.EnginePreviewData{})
	}
	if d := c.GetPreview(NewCacheKey("preview", 0, "p0")); d != nil {
		t.Error("preview: oldest entry should have been evicted")
	}
	if d := c.GetPreview(NewCacheKey("preview", 0, "p2")); d == nil {
		t.Error("preview: most recent entry should still be present")
	}

	// Tutorial eviction.
	for i := range 3 {
		c.PutTutorial(NewCacheKey("tutorial", 0, fmt.Sprintf("t%d", i)), &resource.EngineTutorialData{})
	}
	if d := c.GetTutorial(NewCacheKey("tutorial", 0, "t0")); d != nil {
		t.Error("tutorial: oldest entry should have been evicted")
	}
	if d := c.GetTutorial(NewCacheKey("tutorial", 0, "t2")); d == nil {
		t.Error("tutorial: most recent entry should still be present")
	}
}

func TestCompileCacheNonPlayRoundTrip(t *testing.T) {
	c := NewCache()

	watchKey := NewCacheKey("watch", 0, "test")
	if d := c.GetWatch(watchKey); d != nil {
		t.Error("empty cache returned non-nil for watch")
	}
	dummy := &resource.EngineWatchData{}
	c.PutWatch(watchKey, dummy)
	if got := c.GetWatch(watchKey); got != dummy {
		t.Error("watch cache returned wrong value")
	}

	previewKey := NewCacheKey("preview", 0, "test")
	dummyP := &resource.EnginePreviewData{}
	c.PutPreview(previewKey, dummyP)
	if got := c.GetPreview(previewKey); got != dummyP {
		t.Error("preview cache returned wrong value")
	}

	tutorialKey := NewCacheKey("tutorial", 0, "test")
	dummyT := &resource.EngineTutorialData{}
	c.PutTutorial(tutorialKey, dummyT)
	if got := c.GetTutorial(tutorialKey); got != dummyT {
		t.Error("tutorial cache returned wrong value")
	}
}
