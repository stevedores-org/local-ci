package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestComputeSourceHashDistinguishesPath(t *testing.T) {
	cfg := &Config{Cache: CacheConfig{IncludePatterns: []string{"*.rs"}}}

	// Same content, different filename -> must hash differently (rename detection).
	a := t.TempDir()
	os.WriteFile(filepath.Join(a, "a.rs"), []byte("fn main(){}"), 0644)
	b := t.TempDir()
	os.WriteFile(filepath.Join(b, "b.rs"), []byte("fn main(){}"), 0644)

	ha, err := computeSourceHash(a, cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	hb, err := computeSourceHash(b, cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if ha == hb {
		t.Errorf("rename (a.rs -> b.rs, same content) should change the hash, both = %s", ha)
	}

	// Splitting content across a file boundary must change the hash too.
	c := t.TempDir()
	os.WriteFile(filepath.Join(c, "a.rs"), []byte("AB"), 0644)
	d := t.TempDir()
	os.WriteFile(filepath.Join(d, "a.rs"), []byte("A"), 0644)
	os.WriteFile(filepath.Join(d, "b.rs"), []byte("B"), 0644)
	hc, _ := computeSourceHash(c, cfg, nil)
	hd, _ := computeSourceHash(d, cfg, nil)
	if hc == hd {
		t.Errorf("file-boundary shift (AB vs A|B) should change the hash, both = %s", hc)
	}
}

func TestSaveCacheAtomicRoundTrip(t *testing.T) {
	dir := t.TempDir()
	want := map[string]string{"fmt": "hash1", "clippy": "hash2"}
	if err := saveCache(want, dir); err != nil {
		t.Fatalf("saveCache: %v", err)
	}
	got, err := loadCache(dir)
	if err != nil {
		t.Fatalf("loadCache: %v", err)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("cache[%q] = %q, want %q", k, got[k], v)
		}
	}
	// No temp files should be left behind.
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".local-ci-cache.tmp-") {
			t.Errorf("leftover temp file: %s", e.Name())
		}
	}
}

func TestSelectStagesFromConfig(t *testing.T) {
	dir := createTestWorkspace(t)
	defer os.RemoveAll(dir)

	config, err := LoadConfig(dir, false)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Request specific stages that exist in defaults
	requested := []string{"fmt", "test"}
	var stages []Stage
	for _, name := range requested {
		if stage, ok := config.Stages[name]; ok {
			stages = append(stages, stage)
		} else {
			t.Fatalf("stage %q not found in config", name)
		}
	}

	if len(stages) != 2 {
		t.Fatalf("expected 2 stages, got %d", len(stages))
	}
}

func TestSelectStagesUnknown(t *testing.T) {
	dir := createTestWorkspace(t)
	defer os.RemoveAll(dir)

	config, err := LoadConfig(dir, false)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	_, ok := config.Stages["nope"]
	if ok {
		t.Fatal("expected unknown stage 'nope' to not exist in config")
	}
}

func TestCacheRoundTrip(t *testing.T) {
	dir := t.TempDir()
	cache := map[string]string{
		"fmt":    "hash-a|cargo fmt --all -- --check",
		"clippy": "hash-b|cargo clippy --workspace -- -D warnings",
	}

	if err := saveCache(cache, dir); err != nil {
		t.Fatalf("saveCache failed: %v", err)
	}
	loaded, err := loadCache(dir)
	if err != nil {
		t.Fatalf("loadCache failed: %v", err)
	}
	if loaded["fmt"] != cache["fmt"] || loaded["clippy"] != cache["clippy"] {
		t.Fatalf("unexpected cache roundtrip contents: %#v", loaded)
	}
}

func TestComputeSourceHashIgnoresTarget(t *testing.T) {
	dir := createTestWorkspace(t)
	defer os.RemoveAll(dir)

	targetDir := filepath.Join(dir, "target")
	targetFile := filepath.Join(targetDir, "junk.rs")

	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatalf("mkdir target failed: %v", err)
	}
	if err := os.WriteFile(targetFile, []byte("changed"), 0o644); err != nil {
		t.Fatalf("write target failed: %v", err)
	}

	config, _ := LoadConfig(dir, false)
	ws, _ := DetectWorkspace(dir)

	h1, err := computeSourceHash(dir, config, ws)
	if err != nil {
		t.Fatalf("computeSourceHash #1 failed: %v", err)
	}

	// Changing files under target/ should not affect hash.
	if err := os.WriteFile(targetFile, []byte("changed again"), 0o644); err != nil {
		t.Fatalf("rewrite target failed: %v", err)
	}
	h2, err := computeSourceHash(dir, config, ws)
	if err != nil {
		t.Fatalf("computeSourceHash #2 failed: %v", err)
	}

	if h1 != h2 {
		t.Fatalf("expected stable hash when target changes: %s vs %s", h1, h2)
	}
}

func TestRequireCommand(t *testing.T) {
	if err := requireCommand("go"); err != nil {
		t.Fatalf("expected go to be found: %v", err)
	}
	if err := requireCommand("nonexistent-tool-xyz"); err == nil {
		t.Fatal("expected error for nonexistent command")
	}
}

func TestValidateStageCommands(t *testing.T) {
	stageWithGo := Stage{
		Name: "go-version",
		Cmd:  []string{"go", "version"},
	}

	if err := validateStageCommands([]Stage{stageWithGo}); err != nil {
		t.Fatalf("expected go command to be valid: %v", err)
	}
}

func TestValidateStageCommandsMissingCommand(t *testing.T) {
	tool := "definitely-missing-local-ci-tool-xyz"
	if _, err := exec.LookPath(tool); err == nil {
		t.Skipf("expected %q to be missing from PATH for this test", tool)
	}

	stage := Stage{
		Name: "missing-tool",
		Cmd:  []string{tool, "run"},
	}

	err := validateStageCommands([]Stage{stage})
	if err == nil {
		t.Fatal("expected error for missing command")
	}
	if !strings.Contains(err.Error(), fmt.Sprintf("stage %q", stage.Name)) {
		t.Fatalf("expected stage name in error, got: %v", err)
	}
	if !strings.Contains(err.Error(), tool) {
		t.Fatalf("expected missing tool in error, got: %v", err)
	}
}

func TestValidateStageCommandsEmptyCmd(t *testing.T) {
	stage := Stage{
		Name: "empty",
		Cmd:  []string{},
	}

	err := validateStageCommands([]Stage{stage})
	if err == nil {
		t.Fatal("expected error for empty command")
	}
}
