package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Test Fixtures - Create temporary test directories and files
func createTestWorkspace(t testing.TB) string {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "local-ci-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Create basic Cargo.toml for single crate
	cargoContent := `[package]
name = "test-crate"
version = "0.1.0"
edition = "2021"
`
	cargoPath := filepath.Join(tmpDir, "Cargo.toml")
	if err := os.WriteFile(cargoPath, []byte(cargoContent), 0644); err != nil {
		t.Fatalf("Failed to create Cargo.toml: %v", err)
	}

	// Create a simple Rust source file
	srcDir := filepath.Join(tmpDir, "src")
	os.MkdirAll(srcDir, 0755)
	rsFile := filepath.Join(srcDir, "lib.rs")
	if err := os.WriteFile(rsFile, []byte("pub fn test() {}"), 0644); err != nil {
		t.Fatalf("Failed to create lib.rs: %v", err)
	}

	return tmpDir
}

func createTestWorkspaceWithMembers(t *testing.T) string {
	tmpDir, err := os.MkdirTemp("", "local-ci-ws-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Create workspace Cargo.toml
	wsCargoContent := `[workspace]
members = ["crate1", "crate2"]
exclude = ["crate3"]

[workspace.package]
version = "0.1.0"
edition = "2021"
`
	cargoPath := filepath.Join(tmpDir, "Cargo.toml")
	if err := os.WriteFile(cargoPath, []byte(wsCargoContent), 0644); err != nil {
		t.Fatalf("Failed to create workspace Cargo.toml: %v", err)
	}

	// Create member crates
	for _, crate := range []string{"crate1", "crate2", "crate3"} {
		crateDir := filepath.Join(tmpDir, crate)
		srcDir := filepath.Join(crateDir, "src")
		os.MkdirAll(srcDir, 0755)

		memberCargoContent := `[package]
name = "` + crate + `"
version = "0.1.0"
edition = "2021"
`
		memberCargoPath := filepath.Join(crateDir, "Cargo.toml")
		if err := os.WriteFile(memberCargoPath, []byte(memberCargoContent), 0644); err != nil {
			t.Fatalf("Failed to create member Cargo.toml: %v", err)
		}

		libPath := filepath.Join(srcDir, "lib.rs")
		if err := os.WriteFile(libPath, []byte("pub fn "+crate+"() {}"), 0644); err != nil {
			t.Fatalf("Failed to create lib.rs: %v", err)
		}
	}

	return tmpDir
}

// Test Cases - Code Configuration
func TestLoadConfigDefaults(t *testing.T) {
	tmpDir := createTestWorkspace(t)
	defer os.RemoveAll(tmpDir)

	config, err := LoadConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if len(config.Stages) < 7 {
		t.Errorf("Expected at least 7 stages, got %d", len(config.Stages))
	}

	// Verify default stages exist
	defaultStages := []string{"fmt", "clippy", "test"}
	for _, stageName := range defaultStages {
		if _, exists := config.Stages[stageName]; !exists {
			t.Errorf("Missing default stage: %s", stageName)
		}
	}

	// Verify TypeScript/Bun stages exist
	bunStages := []string{"bun-install", "typecheck-ts", "lint-ts", "test-ts"}
	for _, stageName := range bunStages {
		stage, exists := config.Stages[stageName]
		if !exists {
			t.Errorf("Missing Bun/TS stage: %s", stageName)
			continue
		}
		if len(stage.Cmd) == 0 || stage.Cmd[0] != "bun" {
			t.Errorf("Expected stage %s to use bun command, got %v", stageName, stage.Cmd)
		}
	}

	// Verify cache config
	if len(config.Cache.SkipDirs) == 0 {
		t.Error("Cache skip_dirs should have defaults")
	}
	if len(config.Cache.IncludePatterns) == 0 {
		t.Error("Cache include_patterns should have defaults")
	}
}

func TestLoadConfigFromFile(t *testing.T) {
	tmpDir := createTestWorkspace(t)
	defer os.RemoveAll(tmpDir)

	// Create custom config
	configContent := `[cache]
skip_dirs = [".git", "target"]
include_patterns = ["*.rs"]

[stages.fmt]
command = ["cargo", "fmt", "--all", "--", "--check"]
timeout = 60
enabled = true

[stages.test]
command = ["cargo", "test"]
enabled = false
`
	configPath := filepath.Join(tmpDir, ".local-ci.toml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	config, err := LoadConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Verify custom config loaded
	if config.Cache.SkipDirs[0] != ".git" {
		t.Errorf("Expected first skip dir to be .git, got %s", config.Cache.SkipDirs[0])
	}

	if !config.Stages["fmt"].Enabled {
		t.Error("fmt stage should be enabled")
	}

	if config.Stages["test"].Enabled {
		t.Error("test stage should be disabled")
	}
}

// Test Cases - Workspace Understanding
func TestDetectSingleCrate(t *testing.T) {
	tmpDir := createTestWorkspace(t)
	defer os.RemoveAll(tmpDir)

	ws, err := DetectWorkspace(tmpDir)
	if err != nil {
		t.Fatalf("DetectWorkspace failed: %v", err)
	}

	if !ws.IsSingle {
		t.Error("Expected single crate workspace")
	}

	if len(ws.Members) != 1 || ws.Members[0] != "." {
		t.Errorf("Expected single member [.], got %v", ws.Members)
	}
}

func TestDetectWorkspaceWithMembers(t *testing.T) {
	tmpDir := createTestWorkspaceWithMembers(t)
	defer os.RemoveAll(tmpDir)

	ws, err := DetectWorkspace(tmpDir)
	if err != nil {
		t.Fatalf("DetectWorkspace failed: %v", err)
	}

	if ws.IsSingle {
		t.Error("Expected workspace, not single crate")
	}

	if len(ws.Members) != 2 {
		t.Errorf("Expected 2 members, got %d", len(ws.Members))
	}

	if !strings.Contains(strings.Join(ws.Members, ","), "crate1") {
		t.Error("Expected crate1 in members")
	}

	// Verify excludes
	if len(ws.Excludes) != 1 || ws.Excludes[0] != "crate3" {
		t.Errorf("Expected exclude [crate3], got %v", ws.Excludes)
	}
}

func TestWorkspaceIsExcluded(t *testing.T) {
	tmpDir := createTestWorkspaceWithMembers(t)
	defer os.RemoveAll(tmpDir)

	ws, err := DetectWorkspace(tmpDir)
	if err != nil {
		t.Fatalf("DetectWorkspace failed: %v", err)
	}

	if !ws.IsExcluded("crate3") {
		t.Error("crate3 should be excluded")
	}

	if ws.IsExcluded("crate1") {
		t.Error("crate1 should not be excluded")
	}
}

// Test Cases - Source Hashing
func TestComputeSourceHash(t *testing.T) {
	tmpDir := createTestWorkspace(t)
	defer os.RemoveAll(tmpDir)

	config, _ := LoadConfig(tmpDir)
	ws, _ := DetectWorkspace(tmpDir)

	hash1, err := computeSourceHash(tmpDir, config, ws)
	if err != nil {
		t.Fatalf("computeSourceHash failed: %v", err)
	}

	if hash1 == "" {
		t.Error("Hash should not be empty")
	}

	// Hash should be consistent
	hash2, _ := computeSourceHash(tmpDir, config, ws)
	if hash1 != hash2 {
		t.Error("Hash should be deterministic")
	}

	// Modifying source should change hash
	rsFile := filepath.Join(tmpDir, "src", "lib.rs")
	if err := os.WriteFile(rsFile, []byte("pub fn test2() {}"), 0644); err != nil {
		t.Fatalf("Failed to modify source: %v", err)
	}

	hash3, _ := computeSourceHash(tmpDir, config, ws)
	if hash1 == hash3 {
		t.Error("Hash should change when source changes")
	}
}

func TestHashSkipsDirectories(t *testing.T) {
	tmpDir := createTestWorkspace(t)
	defer os.RemoveAll(tmpDir)

	config, _ := LoadConfig(tmpDir)
	ws, _ := DetectWorkspace(tmpDir)

	hash1, _ := computeSourceHash(tmpDir, config, ws)

	// Create .git directory (should be skipped)
	gitDir := filepath.Join(tmpDir, ".git")
	os.MkdirAll(gitDir, 0755)
	if err := os.WriteFile(filepath.Join(gitDir, "config"), []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create .git file: %v", err)
	}

	hash2, _ := computeSourceHash(tmpDir, config, ws)
	if hash1 != hash2 {
		t.Error("Hash should be unchanged when .git directory is modified")
	}
}

// Test Cases - Tool Detection
func TestToolChecking(t *testing.T) {
	// This test verifies tool detection works
	// It doesn't require tools to be installed

	missing := GetMissingTools()
	if !isStringSlice(missing) {
		t.Error("GetMissingTools should return string slice")
	}

	hints := GetMissingToolsWithHints()
	if hints == nil {
		t.Error("GetMissingToolsWithHints should return map")
	}
}

func TestMissingToolsMessage(t *testing.T) {
	hints := map[string]string{
		"cargo-deny":  "cargo install cargo-deny",
		"cargo-audit": "cargo install cargo-audit",
	}

	msg := FormatMissingToolsMessage(hints)
	if !strings.Contains(msg, "cargo-deny") {
		t.Error("Message should contain tool name")
	}

	if !strings.Contains(msg, "cargo install") {
		t.Error("Message should contain installation instruction")
	}
}

// Test Cases - Initialization
func TestInitCreateConfig(t *testing.T) {
	tmpDir := createTestWorkspace(t)
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, ".local-ci.toml")
	if _, err := os.Stat(configPath); err == nil {
		t.Fatal("Config should not exist before init")
	}

	ws, _ := DetectWorkspace(tmpDir)
	err := SaveDefaultConfig(tmpDir, ws)
	if err != nil {
		t.Fatalf("SaveDefaultConfig failed: %v", err)
	}

	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("Config file not created: %v", err)
	}

	// Verify config is valid TOML
	data, _ := os.ReadFile(configPath)
	if !strings.Contains(string(data), "[stages.fmt]") {
		t.Error("Config should contain stage definitions")
	}
	if !strings.Contains(string(data), "[stages.typecheck-ts]") {
		t.Error("Config should contain TypeScript stage definitions")
	}
}

func TestInitUpdateGitignore(t *testing.T) {
	tmpDir := createTestWorkspace(t)
	defer os.RemoveAll(tmpDir)

	gitignorePath := filepath.Join(tmpDir, ".gitignore")
	err := updateGitignore(gitignorePath, ".local-ci-cache")
	if err != nil {
		t.Fatalf("updateGitignore failed: %v", err)
	}

	data, _ := os.ReadFile(gitignorePath)
	if !strings.Contains(string(data), ".local-ci-cache") {
		t.Error(".gitignore should contain .local-ci-cache")
	}

	// Second call should be idempotent
	err = updateGitignore(gitignorePath, ".local-ci-cache")
	if err != nil {
		t.Fatalf("Second updateGitignore failed: %v", err)
	}

	data, _ = os.ReadFile(gitignorePath)
	lines := strings.Split(string(data), "\n")
	count := 0
	for _, line := range lines {
		if strings.TrimSpace(line) == ".local-ci-cache" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("Expected 1 .local-ci-cache entry, got %d", count)
	}
}

// Test Cases - Caching
func TestSaveLoadCache(t *testing.T) {
	tmpDir := createTestWorkspace(t)
	defer os.RemoveAll(tmpDir)

	cache := map[string]string{
		"fmt":    "abc123",
		"clippy": "def456",
	}

	err := saveCache(cache, tmpDir)
	if err != nil {
		t.Fatalf("saveCache failed: %v", err)
	}

	loaded, err := loadCache(tmpDir)
	if err != nil {
		t.Fatalf("loadCache failed: %v", err)
	}

	if loaded["fmt"] != "abc123" {
		t.Errorf("Expected fmt hash abc123, got %s", loaded["fmt"])
	}

	if loaded["clippy"] != "def456" {
		t.Errorf("Expected clippy hash def456, got %s", loaded["clippy"])
	}
}

func TestCacheConsistency(t *testing.T) {
	tmpDir := createTestWorkspace(t)
	defer os.RemoveAll(tmpDir)

	cache1 := map[string]string{"test": "hash1"}
	saveCache(cache1, tmpDir)

	loaded, _ := loadCache(tmpDir)
	if loaded["test"] != "hash1" {
		t.Error("Cache not loaded correctly")
	}

	cache2 := map[string]string{"test": "hash1", "other": "hash2"}
	saveCache(cache2, tmpDir)

	loaded, _ = loadCache(tmpDir)
	if loaded["test"] != "hash1" || loaded["other"] != "hash2" {
		t.Error("Cache update failed")
	}
}

// Helpers
func isStringSlice(val interface{}) bool {
	_, ok := val.([]string)
	return ok
}

// Benchmark Tests
func BenchmarkComputeSourceHash(b *testing.B) {
	tmpDir := createTestWorkspace(b)
	defer os.RemoveAll(tmpDir)

	config, _ := LoadConfig(tmpDir)
	ws, _ := DetectWorkspace(tmpDir)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		computeSourceHash(tmpDir, config, ws)
	}
}

func BenchmarkLoadConfig(b *testing.B) {
	tmpDir := createTestWorkspace(b)
	defer os.RemoveAll(tmpDir)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		LoadConfig(tmpDir)
	}
}

// Performance Test - Measure critical paths
func TestPerformanceMetrics(t *testing.T) {
	tmpDir := createTestWorkspace(t)
	defer os.RemoveAll(tmpDir)

	// Measure config load time
	start := time.Now()
	for i := 0; i < 100; i++ {
		LoadConfig(tmpDir)
	}
	duration := time.Since(start)
	avgLoadTime := duration / 100

	if avgLoadTime > time.Millisecond*10 {
		t.Logf("Warning: LoadConfig averaging %v per call", avgLoadTime)
	}

	// Measure hash computation
	config, _ := LoadConfig(tmpDir)
	ws, _ := DetectWorkspace(tmpDir)

	start = time.Now()
	computeSourceHash(tmpDir, config, ws)
	hashDuration := time.Since(start)

	if hashDuration > time.Second {
		t.Logf("Warning: computeSourceHash took %v", hashDuration)
	}
}
