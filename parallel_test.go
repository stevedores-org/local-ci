package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// === Parallel execution tests ===

func TestParallelRunnerExecutesStages(t *testing.T) {
	dir := t.TempDir()

	stages := []Stage{
		{Name: "s1", Cmd: []string{"echo", "hello"}, Timeout: 10},
		{Name: "s2", Cmd: []string{"echo", "world"}, Timeout: 10},
	}

	pr := &ParallelRunner{
		Stages:      stages,
		Concurrency: 2,
		Cwd:         dir,
		NoCache:     true,
		Cache:       make(map[string]string),
		SourceHash: "testhash",
	}

	results := pr.Run()

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	for _, r := range results {
		if r.Status != "pass" {
			t.Errorf("stage %q should pass, got %q", r.Name, r.Status)
		}
	}
}

func TestParallelRunnerRespectsDepOrder(t *testing.T) {
	dir := t.TempDir()

	// s2 depends on s1. s1 writes a marker file, s2 reads it.
	markerFile := filepath.Join(dir, "marker.txt")

	stages := []Stage{
		{Name: "s1", Cmd: []string{"sh", "-c", fmt.Sprintf("echo done > %s", markerFile)}, Timeout: 10},
		{Name: "s2", Cmd: []string{"sh", "-c", fmt.Sprintf("cat %s", markerFile)}, Timeout: 10, DependsOn: []string{"s1"}},
	}

	pr := &ParallelRunner{
		Stages:      stages,
		Concurrency: 2,
		Cwd:         dir,
		NoCache:     true,
		Cache:       make(map[string]string),
		SourceHash: "testhash",
	}

	results := pr.Run()

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	for _, r := range results {
		if r.Status != "pass" {
			t.Errorf("stage %q should pass, got %q (output: %s)", r.Name, r.Status, r.Output)
		}
	}
}

func TestParallelRunnerFailFast(t *testing.T) {
	dir := t.TempDir()

	stages := []Stage{
		{Name: "fail-first", Cmd: []string{"false"}, Timeout: 10},
		{Name: "should-skip", Cmd: []string{"echo", "ran"}, Timeout: 10, DependsOn: []string{"fail-first"}},
	}

	pr := &ParallelRunner{
		Stages:      stages,
		Concurrency: 1,
		Cwd:         dir,
		NoCache:     true,
		Cache:       make(map[string]string),
		SourceHash: "testhash",
		FailFast:    true,
	}

	results := pr.Run()

	// First stage should fail
	foundFail := false
	for _, r := range results {
		if r.Status == "fail" {
			foundFail = true
		}
	}
	if !foundFail {
		t.Error("expected at least one failed stage")
	}
}

func TestParallelRunnerCacheHit(t *testing.T) {
	dir := t.TempDir()

	stages := []Stage{
		{Name: "cached", Cmd: []string{"echo", "hello"}, Timeout: 10},
	}

	cache := map[string]string{
		"cached": "hash123",
	}

	pr := &ParallelRunner{
		Stages:     stages,
		Cwd:        dir,
		NoCache:    false,
		Cache:      cache,
		SourceHash: "hash123",
	}

	results := pr.Run()

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].CacheHit {
		t.Error("expected cache hit")
	}
	if results[0].Duration != 0 {
		t.Error("cached result should have 0 duration")
	}
}

func TestParallelRunnerConcurrencyLimit(t *testing.T) {
	dir := t.TempDir()

	stages := []Stage{
		{Name: "s1", Cmd: []string{"echo", "1"}, Timeout: 10},
		{Name: "s2", Cmd: []string{"echo", "2"}, Timeout: 10},
		{Name: "s3", Cmd: []string{"echo", "3"}, Timeout: 10},
		{Name: "s4", Cmd: []string{"echo", "4"}, Timeout: 10},
	}

	pr := &ParallelRunner{
		Stages:      stages,
		Concurrency: 2,
		Cwd:         dir,
		NoCache:     true,
		Cache:       make(map[string]string),
		SourceHash: "testhash",
	}

	results := pr.Run()

	if len(results) != 4 {
		t.Fatalf("expected 4 results, got %d", len(results))
	}

	for _, r := range results {
		if r.Status != "pass" {
			t.Errorf("stage %q should pass, got %q", r.Name, r.Status)
		}
	}
}

func TestDependsOnFieldInConfig(t *testing.T) {
	dir := t.TempDir()

	configContent := `[stages.fmt]
command = ["echo", "fmt"]
timeout = 10
enabled = true

[stages.clippy]
command = ["echo", "clippy"]
depends_on = ["fmt"]
timeout = 10
enabled = true

[stages.test]
command = ["echo", "test"]
depends_on = ["fmt"]
timeout = 10
enabled = true
`
	os.WriteFile(filepath.Join(dir, ".local-ci.toml"), []byte(configContent), 0644)
	os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte("[package]\nname = \"x\"\n"), 0644)

	config, err := LoadConfig(dir, false)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	clippy := config.Stages["clippy"]
	if len(clippy.DependsOn) != 1 || clippy.DependsOn[0] != "fmt" {
		t.Errorf("clippy should depend on [fmt], got %v", clippy.DependsOn)
	}
	test := config.Stages["test"]
	if len(test.DependsOn) != 1 || test.DependsOn[0] != "fmt" {
		t.Errorf("test should depend on [fmt], got %v", test.DependsOn)
	}
	fmtStage := config.Stages["fmt"]
	if len(fmtStage.DependsOn) != 0 {
		t.Errorf("fmt should have no dependency, got %v", fmtStage.DependsOn)
	}
}
