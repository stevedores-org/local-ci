package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSelectStagesKnown(t *testing.T) {
	stages := availableStages()
	got, err := selectStages([]string{"fmt", "test"}, stages)
	if err != nil {
		t.Fatalf("selectStages returned error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 stages, got %d", len(got))
	}
	if got[0].Name != "fmt" || got[1].Name != "test" {
		t.Fatalf("unexpected stage order: %s, %s", got[0].Name, got[1].Name)
	}
}

func TestSelectStagesUnknown(t *testing.T) {
	_, err := selectStages([]string{"fmt", "nope"}, availableStages())
	if err == nil {
		t.Fatal("expected error for unknown stage, got nil")
	}
}

func TestCacheRoundTrip(t *testing.T) {
	dir := t.TempDir()
	cache := map[string]string{
		"fmt":    "hash-a",
		"clippy": "hash-b",
	}

	if err := saveCache(cache, dir); err != nil {
		t.Fatalf("saveCache failed: %v", err)
	}
	loaded, err := loadCache(dir)
	if err != nil {
		t.Fatalf("loadCache failed: %v", err)
	}
	if loaded["fmt"] != "hash-a" || loaded["clippy"] != "hash-b" {
		t.Fatalf("unexpected cache roundtrip contents: %#v", loaded)
	}
}

func TestComputeSourceHashIgnoresTarget(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "main.rs")
	targetDir := filepath.Join(dir, "target")
	targetFile := filepath.Join(targetDir, "junk.rs")

	if err := os.WriteFile(srcPath, []byte("fn main() {}"), 0o644); err != nil {
		t.Fatalf("write src failed: %v", err)
	}
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatalf("mkdir target failed: %v", err)
	}
	if err := os.WriteFile(targetFile, []byte("changed"), 0o644); err != nil {
		t.Fatalf("write target failed: %v", err)
	}

	h1, err := computeSourceHash(dir)
	if err != nil {
		t.Fatalf("computeSourceHash #1 failed: %v", err)
	}

	// Changing files under target/ should not affect hash.
	if err := os.WriteFile(targetFile, []byte("changed again"), 0o644); err != nil {
		t.Fatalf("rewrite target failed: %v", err)
	}
	h2, err := computeSourceHash(dir)
	if err != nil {
		t.Fatalf("computeSourceHash #2 failed: %v", err)
	}

	if h1 != h2 {
		t.Fatalf("expected stable hash when target changes: %s vs %s", h1, h2)
	}
}
