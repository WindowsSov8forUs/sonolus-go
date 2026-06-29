package engine

import (
	"testing"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
)

func TestCacheKeyDeterministic(t *testing.T) {
	k1 := NewCacheKey("play", "package main\nfunc main() {}")
	k2 := NewCacheKey("play", "package main\nfunc main() {}")
	if k1 != k2 {
		t.Errorf("same input produced different keys: %v vs %v", k1, k2)
	}
}

func TestCacheKeyModeDiffers(t *testing.T) {
	src := "package main\nfunc main() {}"
	k1 := NewCacheKey("play", src)
	k2 := NewCacheKey("watch", src)
	if k1 == k2 {
		t.Errorf("different modes produced same key: %v", k1)
	}
}

func TestCacheKeySourceDiffers(t *testing.T) {
	k1 := NewCacheKey("play", "a")
	k2 := NewCacheKey("play", "b")
	if k1 == k2 {
		t.Errorf("different sources produced same key: %v", k1)
	}
}

func TestCompileCachePutGet(t *testing.T) {
	c := NewCache()
	key := NewCacheKey("play", "test")
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
