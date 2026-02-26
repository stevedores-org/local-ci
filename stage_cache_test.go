package main

import (
	"os"
	"path/filepath"
	"testing"
)

// createRustWorkspace creates a temp dir with Cargo.toml, *.rs, and *.toml files
// for testing per-stage hash computation.
func createRustWorkspace(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// Cargo.toml (dependency file)
	os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte(`[package]
name = "test"
version = "0.1.0"
`), 0o644)

	// Cargo.lock
	os.WriteFile(filepath.Join(dir, "Cargo.lock"), []byte("lockfile-contents\n"), 0o644)

	// deny.toml
	os.WriteFile(filepath.Join(dir, "deny.toml"), []byte("[bans]\nmultiple-versions = \"deny\"\n"), 0o644)

	// src/lib.rs
	srcDir := filepath.Join(dir, "src")
	os.MkdirAll(srcDir, 0o755)
	os.WriteFile(filepath.Join(srcDir, "lib.rs"), []byte("pub fn hello() {}\n"), 0o644)

	// target/ should be skipped
	targetDir := filepath.Join(dir, "target")
	os.MkdirAll(targetDir, 0o755)
	os.WriteFile(filepath.Join(targetDir, "junk.rs"), []byte("junk"), 0o644)

	return dir
}

func TestStageHashDiffersWithWatchPatterns(t *testing.T) {
	dir := createRustWorkspace(t)

	config := &Config{
		Cache: CacheConfig{
			SkipDirs:        []string{"target", ".git"},
			IncludePatterns: []string{"*.rs", "*.toml", "*.lock"},
		},
	}

	// Stage watching only *.rs files
	fmtStage := Stage{Name: "fmt", Watch: []string{"*.rs"}}
	fmtHash, err := computeStageHash(fmtStage, dir, config, nil)
	if err != nil {
		t.Fatalf("computeStageHash for fmt failed: %v", err)
	}

	// Stage watching *.rs + Cargo.toml + Cargo.lock
	clippyStage := Stage{Name: "clippy", Watch: []string{"*.rs", "Cargo.toml", "Cargo.lock"}}
	clippyHash, err := computeStageHash(clippyStage, dir, config, nil)
	if err != nil {
		t.Fatalf("computeStageHash for clippy failed: %v", err)
	}

	// Stage watching only dep files
	denyStage := Stage{Name: "deny", Watch: []string{"Cargo.toml", "Cargo.lock", "deny.toml"}}
	denyHash, err := computeStageHash(denyStage, dir, config, nil)
	if err != nil {
		t.Fatalf("computeStageHash for deny failed: %v", err)
	}

	// All three should be different
	if fmtHash == clippyHash {
		t.Fatal("fmt and clippy hashes should differ (clippy includes Cargo.toml/Cargo.lock)")
	}
	if fmtHash == denyHash {
		t.Fatal("fmt and deny hashes should differ")
	}
	if clippyHash == denyHash {
		t.Fatal("clippy and deny hashes should differ (clippy includes *.rs, deny doesn't)")
	}
}

func TestStageHashFallsBackToGlobalPatterns(t *testing.T) {
	dir := createRustWorkspace(t)

	config := &Config{
		Cache: CacheConfig{
			SkipDirs:        []string{"target", ".git"},
			IncludePatterns: []string{"*.rs", "*.toml", "*.lock"},
		},
	}

	// Empty watch → falls back to global include patterns
	fallbackStage := Stage{Name: "test", Watch: nil}
	fallbackHash, err := computeStageHash(fallbackStage, dir, config, nil)
	if err != nil {
		t.Fatalf("computeStageHash with nil patterns failed: %v", err)
	}

	// Explicit global patterns should match
	explicitStage := Stage{Name: "test", Watch: []string{"*.rs", "*.toml", "*.lock"}}
	explicitHash, err := computeStageHash(explicitStage, dir, config, nil)
	if err != nil {
		t.Fatalf("computeStageHash with explicit patterns failed: %v", err)
	}

	if fallbackHash != explicitHash {
		t.Fatalf("empty watch should fall back to global include_patterns: %s vs %s", fallbackHash, explicitHash)
	}
}

func TestStageHashChangesOnlyForAffectedStage(t *testing.T) {
	dir := createRustWorkspace(t)

	config := &Config{
		Cache: CacheConfig{
			SkipDirs:        []string{"target", ".git"},
			IncludePatterns: []string{"*.rs", "*.toml", "*.lock"},
		},
	}

	stages := []Stage{
		{Name: "fmt", Watch: []string{"*.rs"}},
		{Name: "deny", Watch: []string{"Cargo.toml", "Cargo.lock", "deny.toml"}},
	}

	// Get initial hashes
	hashes1, err := computeStageHashes(dir, config, nil, stages)
	if err != nil {
		t.Fatalf("computeStageHashes #1 failed: %v", err)
	}

	// Modify only deny.toml — should only affect deny, not fmt
	os.WriteFile(filepath.Join(dir, "deny.toml"), []byte("[bans]\nmultiple-versions = \"allow\"\n"), 0o644)

	hashes2, err := computeStageHashes(dir, config, nil, stages)
	if err != nil {
		t.Fatalf("computeStageHashes #2 failed: %v", err)
	}

	if hashes1["fmt"] != hashes2["fmt"] {
		t.Fatal("fmt hash should NOT change when only deny.toml was modified")
	}
	if hashes1["deny"] == hashes2["deny"] {
		t.Fatal("deny hash SHOULD change when deny.toml was modified")
	}
}

func TestStageHashChangesOnlyForRsStage(t *testing.T) {
	dir := createRustWorkspace(t)

	config := &Config{
		Cache: CacheConfig{
			SkipDirs:        []string{"target", ".git"},
			IncludePatterns: []string{"*.rs", "*.toml", "*.lock"},
		},
	}

	stages := []Stage{
		{Name: "fmt", Watch: []string{"*.rs"}},
		{Name: "deny", Watch: []string{"Cargo.toml", "Cargo.lock", "deny.toml"}},
	}

	hashes1, err := computeStageHashes(dir, config, nil, stages)
	if err != nil {
		t.Fatalf("computeStageHashes #1 failed: %v", err)
	}

	// Modify only lib.rs — should only affect fmt, not deny
	os.WriteFile(filepath.Join(dir, "src", "lib.rs"), []byte("pub fn goodbye() {}\n"), 0o644)

	hashes2, err := computeStageHashes(dir, config, nil, stages)
	if err != nil {
		t.Fatalf("computeStageHashes #2 failed: %v", err)
	}

	if hashes1["fmt"] == hashes2["fmt"] {
		t.Fatal("fmt hash SHOULD change when lib.rs was modified")
	}
	if hashes1["deny"] != hashes2["deny"] {
		t.Fatal("deny hash should NOT change when only lib.rs was modified")
	}
}

func TestStageHashesDeduplicateWalks(t *testing.T) {
	dir := createRustWorkspace(t)

	config := &Config{
		Cache: CacheConfig{
			SkipDirs:        []string{"target", ".git"},
			IncludePatterns: []string{"*.rs", "*.toml", "*.lock"},
		},
	}

	// Two stages with identical watch patterns should get identical hashes
	stages := []Stage{
		{Name: "clippy", Watch: []string{"*.rs", "Cargo.toml", "Cargo.lock"}},
		{Name: "test", Watch: []string{"*.rs", "Cargo.toml", "Cargo.lock"}},
	}

	hashes, err := computeStageHashes(dir, config, nil, stages)
	if err != nil {
		t.Fatalf("computeStageHashes failed: %v", err)
	}

	if hashes["clippy"] != hashes["test"] {
		t.Fatalf("stages with identical watch patterns should get same hash: clippy=%s test=%s",
			hashes["clippy"], hashes["test"])
	}
}

func TestStageHashIgnoresTargetDir(t *testing.T) {
	dir := createRustWorkspace(t)

	config := &Config{
		Cache: CacheConfig{
			SkipDirs:        []string{"target", ".git"},
			IncludePatterns: []string{"*.rs", "*.toml", "*.lock"},
		},
	}

	stage := Stage{Name: "fmt", Watch: []string{"*.rs"}}
	hash1, err := computeStageHash(stage, dir, config, nil)
	if err != nil {
		t.Fatalf("computeStageHash #1 failed: %v", err)
	}

	// Modify file in target/ — should not affect hash
	os.WriteFile(filepath.Join(dir, "target", "junk.rs"), []byte("changed junk"), 0o644)

	hash2, err := computeStageHash(stage, dir, config, nil)
	if err != nil {
		t.Fatalf("computeStageHash #2 failed: %v", err)
	}

	if hash1 != hash2 {
		t.Fatalf("hash should be stable when target/ changes: %s vs %s", hash1, hash2)
	}
}

func TestMatchesPatterns(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		patterns []string
		want     bool
	}{
		{"extension match", "lib.rs", []string{"*.rs"}, true},
		{"extension no match", "lib.rs", []string{"*.toml"}, false},
		{"exact match", "Cargo.toml", []string{"Cargo.toml"}, true},
		{"exact no match", "Cargo.lock", []string{"Cargo.toml"}, false},
		{"multiple patterns first", "lib.rs", []string{"*.rs", "*.toml"}, true},
		{"multiple patterns second", "Cargo.toml", []string{"*.rs", "Cargo.toml"}, true},
		{"no patterns", "lib.rs", []string{}, false},
		{"deny.toml exact", "deny.toml", []string{"deny.toml"}, true},
		{"lock extension", "Cargo.lock", []string{"*.lock"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesPatterns(tt.filename, tt.patterns)
			if got != tt.want {
				t.Errorf("matchesPatterns(%q, %v) = %v, want %v", tt.filename, tt.patterns, got, tt.want)
			}
		})
	}
}

func TestCacheKeyUsesPerStageHash(t *testing.T) {
	dir := createRustWorkspace(t)

	config := &Config{
		Cache: CacheConfig{
			SkipDirs:        []string{"target", ".git"},
			IncludePatterns: []string{"*.rs", "*.toml", "*.lock"},
		},
	}

	fmtStage := Stage{Name: "fmt", Cmd: []string{"cargo", "fmt"}, Watch: []string{"*.rs"}}
	denyStage := Stage{Name: "deny", Cmd: []string{"cargo", "deny"}, Watch: []string{"Cargo.toml", "Cargo.lock", "deny.toml"}}

	hashes, err := computeStageHashes(dir, config, nil, []Stage{fmtStage, denyStage})
	if err != nil {
		t.Fatal(err)
	}

	// Simulate cache: save fmt as passing with its hash
	cache := make(map[string]string)
	fmtCacheKey := hashes["fmt"] + "|cargo fmt"
	cache["fmt"] = fmtCacheKey

	// Modify deny.toml only
	os.WriteFile(filepath.Join(dir, "deny.toml"), []byte("changed"), 0o644)

	// Recompute hashes
	hashes2, err := computeStageHashes(dir, config, nil, []Stage{fmtStage, denyStage})
	if err != nil {
		t.Fatal(err)
	}

	// fmt should still be cached
	fmtCacheKey2 := hashes2["fmt"] + "|cargo fmt"
	if cache["fmt"] != fmtCacheKey2 {
		t.Fatal("fmt should still be cached after changing only deny.toml")
	}

	// deny should not be cached (hash changed)
	denyCacheKey2 := hashes2["deny"] + "|cargo deny"
	if cache["deny"] == denyCacheKey2 {
		t.Fatal("deny should not be cached after changing deny.toml")
	}
}

func TestDefaultRustStagesHaveWatchPatterns(t *testing.T) {
	stages := getRustStages()

	fmtStage := stages["fmt"]
	if len(fmtStage.Watch) == 0 {
		t.Fatal("fmt stage should have watch patterns")
	}
	if fmtStage.Watch[0] != "*.rs" {
		t.Fatalf("fmt should watch *.rs, got %v", fmtStage.Watch)
	}

	clippyStage := stages["clippy"]
	if len(clippyStage.Watch) < 2 {
		t.Fatal("clippy stage should watch *.rs and Cargo.toml")
	}
}
