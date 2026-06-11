package main

import "testing"

func TestCacheKeyForStage(t *testing.T) {
	stage := Stage{Name: "fmt", Cmd: []string{"cargo", "fmt"}}
	got := cacheKeyForStage(stage, "abc123")
	want := "abc123|cargo fmt"
	if got != want {
		t.Fatalf("cacheKeyForStage = %q, want %q", got, want)
	}
}

func TestCacheHitCanonicalAndLegacy(t *testing.T) {
	stage := Stage{Name: "fmt", Cmd: []string{"cargo", "fmt"}}
	cache := map[string]string{
		"fmt": cacheKeyForStage(stage, "abc123"),
	}
	if !cacheHit(cache, stage, "abc123") {
		t.Fatal("expected canonical cache hit")
	}

	legacy := map[string]string{"fmt": "abc123"}
	if !cacheHit(legacy, stage, "abc123") {
		t.Fatal("expected legacy hash-only cache hit")
	}
}

func TestCacheHitMiss(t *testing.T) {
	stage := Stage{Name: "fmt", Cmd: []string{"cargo", "fmt"}}
	cache := map[string]string{"fmt": cacheKeyForStage(stage, "old")}
	if cacheHit(cache, stage, "new") {
		t.Fatal("expected cache miss")
	}
}
