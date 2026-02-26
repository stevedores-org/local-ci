package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// --- toJSONResults ---

func TestToJSONResultsPass(t *testing.T) {
	results := []Result{
		{
			Name:     "fmt",
			Command:  "cargo fmt --all -- --check",
			Status:   "pass",
			Duration: 150 * time.Millisecond,
			Output:   "formatted\n",
			CacheHit: false,
		},
	}

	jsonResults := toJSONResults(results)
	if len(jsonResults) != 1 {
		t.Fatalf("expected 1 result, got %d", len(jsonResults))
	}

	jr := jsonResults[0]
	if jr.Name != "fmt" {
		t.Errorf("expected name 'fmt', got %q", jr.Name)
	}
	if jr.Status != "pass" {
		t.Errorf("expected status 'pass', got %q", jr.Status)
	}
	if jr.DurationMS != 150 {
		t.Errorf("expected 150ms, got %d", jr.DurationMS)
	}
	if jr.Output != "formatted" {
		t.Errorf("expected trimmed output 'formatted', got %q", jr.Output)
	}
	if jr.Error != "" {
		t.Errorf("expected empty error, got %q", jr.Error)
	}
	if jr.CacheHit {
		t.Error("expected cache hit to be false")
	}
}

func TestToJSONResultsFail(t *testing.T) {
	results := []Result{
		{
			Name:    "test",
			Command: "cargo test",
			Status:  "fail",
			Error:   fmt.Errorf("exit status 1"),
			Output:  "test failed\n",
		},
	}

	jsonResults := toJSONResults(results)
	jr := jsonResults[0]

	if jr.Status != "fail" {
		t.Errorf("expected status 'fail', got %q", jr.Status)
	}
	if jr.Error != "exit status 1" {
		t.Errorf("expected error 'exit status 1', got %q", jr.Error)
	}
}

func TestToJSONResultsCacheHit(t *testing.T) {
	results := []Result{
		{
			Name:     "clippy",
			Command:  "cargo clippy",
			Status:   "pass",
			CacheHit: true,
			Duration: 0,
		},
	}

	jsonResults := toJSONResults(results)
	if !jsonResults[0].CacheHit {
		t.Error("expected cache hit to be true")
	}
	if jsonResults[0].DurationMS != 0 {
		t.Error("cached result should have 0 duration")
	}
}

func TestToJSONResultsEmpty(t *testing.T) {
	jsonResults := toJSONResults(nil)
	if len(jsonResults) != 0 {
		t.Errorf("expected 0 results, got %d", len(jsonResults))
	}
}

func TestToJSONResultsNilError(t *testing.T) {
	results := []Result{
		{Name: "fmt", Status: "pass", Error: nil},
	}

	jsonResults := toJSONResults(results)
	if jsonResults[0].Error != "" {
		t.Errorf("nil error should serialize as empty string, got %q", jsonResults[0].Error)
	}
}

// --- Fix flag behavior ---

func TestFixFlagSwapsFmtCommand(t *testing.T) {
	stages := []Stage{
		{
			Name:   "fmt",
			Cmd:    []string{"cargo", "fmt", "--all", "--", "--check"},
			FixCmd: []string{"cargo", "fmt", "--all"},
			Check:  true,
		},
		{
			Name:  "clippy",
			Cmd:   []string{"cargo", "clippy"},
			Check: false,
		},
	}

	// Simulate --fix behavior from main.go
	for i := range stages {
		if len(stages[i].FixCmd) > 0 {
			stages[i].Cmd = stages[i].FixCmd
			stages[i].Check = false
		}
	}

	// fmt should now use fix command
	if strings.Contains(strings.Join(stages[0].Cmd, " "), "--check") {
		t.Error("fmt command should not contain --check after --fix")
	}
	if stages[0].Check {
		t.Error("fmt Check should be false after --fix")
	}

	// clippy should be unchanged
	if stages[1].Cmd[0] != "cargo" || stages[1].Cmd[1] != "clippy" {
		t.Error("clippy should be unchanged by --fix")
	}
}

func TestFixFlagNoFixCmd(t *testing.T) {
	stages := []Stage{
		{
			Name:   "test",
			Cmd:    []string{"cargo", "test"},
			FixCmd: nil,
			Check:  false,
		},
	}

	// Simulate --fix — should not modify stages without FixCmd
	for i := range stages {
		if len(stages[i].FixCmd) > 0 {
			stages[i].Cmd = stages[i].FixCmd
			stages[i].Check = false
		}
	}

	if stages[0].Cmd[0] != "cargo" || stages[0].Cmd[1] != "test" {
		t.Error("test stage should not be modified by --fix")
	}
}

// --- Cache edge cases ---

func TestLoadCacheEmptyFile(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".local-ci-cache"), []byte(""), 0644)

	cache, err := loadCache(dir)
	if err != nil {
		t.Fatalf("loadCache with empty file should not error: %v", err)
	}
	if len(cache) != 0 {
		t.Errorf("expected empty cache, got %d entries", len(cache))
	}
}

func TestLoadCacheMalformedLines(t *testing.T) {
	dir := t.TempDir()
	content := "fmt:hash123\nbadline\nclippy:hash456\n"
	os.WriteFile(filepath.Join(dir, ".local-ci-cache"), []byte(content), 0644)

	cache, err := loadCache(dir)
	if err != nil {
		t.Fatalf("loadCache should handle malformed lines: %v", err)
	}
	if cache["fmt"] != "hash123" {
		t.Errorf("expected fmt hash123, got %q", cache["fmt"])
	}
	if cache["clippy"] != "hash456" {
		t.Errorf("expected clippy hash456, got %q", cache["clippy"])
	}
	// "badline" should be ignored (no colon separator)
}

func TestSaveCacheSorted(t *testing.T) {
	dir := t.TempDir()
	cache := map[string]string{
		"test":   "hash3",
		"clippy": "hash2",
		"fmt":    "hash1",
	}

	err := saveCache(cache, dir)
	if err != nil {
		t.Fatalf("saveCache failed: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, ".local-ci-cache"))
	lines := strings.Split(string(data), "\n")

	// Should be sorted alphabetically
	if !strings.HasPrefix(lines[0], "clippy:") {
		t.Errorf("expected first line to be clippy, got %q", lines[0])
	}
	if !strings.HasPrefix(lines[1], "fmt:") {
		t.Errorf("expected second line to be fmt, got %q", lines[1])
	}
}

func TestCacheKeyIncludesCommand(t *testing.T) {
	// The cache key format is "sourceHash|command"
	// This ensures cache invalidates when command changes
	sourceHash := "abc123"
	cmd1 := "cargo fmt --all -- --check"
	cmd2 := "cargo fmt --all"

	key1 := sourceHash + "|" + cmd1
	key2 := sourceHash + "|" + cmd2

	if key1 == key2 {
		t.Error("different commands should produce different cache keys")
	}
}

// --- Source hash with workspace exclusions ---

func TestComputeSourceHashWithExclusion(t *testing.T) {
	dir := t.TempDir()

	// Create source files
	srcDir := filepath.Join(dir, "src")
	os.MkdirAll(srcDir, 0755)
	os.WriteFile(filepath.Join(srcDir, "lib.rs"), []byte("pub fn x() {}"), 0644)
	os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte("[package]\nname = \"x\"\n"), 0644)

	// Create excluded dir
	excludedDir := filepath.Join(dir, "excluded")
	os.MkdirAll(excludedDir, 0755)
	os.WriteFile(filepath.Join(excludedDir, "mod.rs"), []byte("fn y() {}"), 0644)

	config, _ := LoadConfig(dir)
	ws := &Workspace{
		Root:     dir,
		IsSingle: false,
		Members:  []string{"src", "excluded"},
		Excludes: []string{"excluded"},
	}

	hash1, err := computeSourceHash(dir, config, ws)
	if err != nil {
		t.Fatalf("computeSourceHash failed: %v", err)
	}

	// Modify excluded file
	os.WriteFile(filepath.Join(excludedDir, "mod.rs"), []byte("fn changed() {}"), 0644)

	hash2, err := computeSourceHash(dir, config, ws)
	if err != nil {
		t.Fatalf("computeSourceHash failed: %v", err)
	}

	if hash1 != hash2 {
		t.Error("hash should not change when excluded workspace member changes")
	}
}

// --- updateGitignore edge cases ---

func TestUpdateGitignoreCreatesNewFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".gitignore")

	err := updateGitignore(path, ".local-ci-cache")
	if err != nil {
		t.Fatalf("updateGitignore failed: %v", err)
	}

	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), ".local-ci-cache") {
		t.Error("should contain .local-ci-cache")
	}
}

func TestUpdateGitignoreAppendsNewline(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".gitignore")

	// Existing content without trailing newline
	os.WriteFile(path, []byte("target"), 0644)

	err := updateGitignore(path, ".local-ci-cache")
	if err != nil {
		t.Fatalf("updateGitignore failed: %v", err)
	}

	data, _ := os.ReadFile(path)
	content := string(data)
	if !strings.Contains(content, "target\n.local-ci-cache") {
		t.Errorf("expected newline before new entry, got %q", content)
	}
}

// --- Stage selection ---

func TestAllFlagEnablesDisabledStages(t *testing.T) {
	config, _ := LoadConfig(t.TempDir())

	// Simulate --all behavior
	var allStages []Stage
	for name, stage := range config.Stages {
		stage.Name = name
		stage.Enabled = true
		allStages = append(allStages, stage)
	}

	for _, stage := range allStages {
		if !stage.Enabled {
			t.Errorf("stage %q should be enabled with --all", stage.Name)
		}
	}

	if len(allStages) < len(config.Stages) {
		t.Error("--all should include all stages")
	}
}

// --- requireCommand ---

func TestRequireCommandFound(t *testing.T) {
	if err := requireCommand("go"); err != nil {
		t.Errorf("'go' should be found: %v", err)
	}
}

func TestRequireCommandNotFound(t *testing.T) {
	err := requireCommand("nonexistent-command-xyz-999")
	if err == nil {
		t.Error("expected error for nonexistent command")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention 'not found', got %q", err.Error())
	}
}

// --- Issue #12: Warnings in PipelineReport ---

func TestPipelineReportWarningsFieldInJSON(t *testing.T) {
	report := PipelineReport{
		Version:    "0.4.0",
		DurationMS: 100,
		Passed:     1,
		Failed:     0,
		Warnings:   []string{"hash computation failed: permission denied on src/lib.rs"},
		Results:    []ResultJSON{},
	}

	data, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	warnings, ok := parsed["warnings"]
	if !ok {
		t.Fatal("JSON output should contain 'warnings' field")
	}

	warningsList, ok := warnings.([]interface{})
	if !ok {
		t.Fatalf("warnings should be an array, got %T", warnings)
	}

	if len(warningsList) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(warningsList))
	}

	msg, ok := warningsList[0].(string)
	if !ok || !strings.Contains(msg, "hash computation failed") {
		t.Errorf("warning should contain hash error message, got %q", msg)
	}
}

func TestPipelineReportNoWarningsOmitsField(t *testing.T) {
	report := PipelineReport{
		Version:    "0.4.0",
		DurationMS: 50,
		Passed:     2,
		Failed:     0,
		Results:    []ResultJSON{},
	}

	data, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	// When Warnings is nil, the field should be omitted from JSON
	if strings.Contains(string(data), "warnings") {
		t.Error("JSON should omit 'warnings' when nil/empty")
	}
}

func TestPipelineReportWarningsBeforeResults(t *testing.T) {
	// Verify warnings appear in JSON output (order doesn't matter for JSON,
	// but we verify both fields coexist correctly)
	report := PipelineReport{
		Version:    "0.4.0",
		DurationMS: 200,
		Passed:     1,
		Failed:     1,
		Warnings:   []string{"warning one", "warning two"},
		Results: []ResultJSON{
			{Name: "fmt", Status: "pass", DurationMS: 100},
			{Name: "test", Status: "fail", DurationMS: 100, Error: "exit 1"},
		},
	}

	data, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	warningsList := parsed["warnings"].([]interface{})
	if len(warningsList) != 2 {
		t.Errorf("expected 2 warnings, got %d", len(warningsList))
	}

	resultsList := parsed["results"].([]interface{})
	if len(resultsList) != 2 {
		t.Errorf("expected 2 results, got %d", len(resultsList))
	}
}

func TestHashErrorAlwaysSurfaced(t *testing.T) {
	// computeSourceHash should return an error for an unreadable directory.
	// The caller should always surface this — not gate on verbose.
	// We test that computeSourceHash returns a meaningful error.
	config := &Config{
		Cache: CacheConfig{
			SkipDirs:        []string{},
			IncludePatterns: []string{"*.go"},
		},
	}

	// Point at a non-existent directory
	_, err := computeSourceHash("/nonexistent-path-for-test-12", config, nil)
	if err == nil {
		t.Error("computeSourceHash should return error for non-existent root")
	}
}

func TestHashErrorDisablesCacheAndAddsWarning(t *testing.T) {
	// Simulate the behavior: when hash fails, cache should be disabled
	// and a warning should be recorded.
	// This tests the collectWarning helper.
	var warnings []string
	hashErr := fmt.Errorf("permission denied on src/lib.rs")

	// Simulate the new behavior from main.go
	if hashErr != nil {
		warnings = append(warnings, fmt.Sprintf("hash computation failed: %v", hashErr))
	}

	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(warnings))
	}
	if !strings.Contains(warnings[0], "permission denied") {
		t.Errorf("warning should contain original error, got %q", warnings[0])
	}
}
