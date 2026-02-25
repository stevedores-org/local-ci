package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// === Parallel execution tests (Issue #6) ===

func TestBuildDepGraphNoDeps(t *testing.T) {
	stages := []Stage{
		{Name: "fmt", Cmd: []string{"echo", "fmt"}},
		{Name: "clippy", Cmd: []string{"echo", "clippy"}},
		{Name: "test", Cmd: []string{"echo", "test"}},
	}

	deps, err := buildDepGraph(stages)
	if err != nil {
		t.Fatalf("buildDepGraph failed: %v", err)
	}

	for _, s := range stages {
		if deps[s.Name] != "" {
			t.Errorf("stage %q should have no dependency, got %q", s.Name, deps[s.Name])
		}
	}
}

func TestBuildDepGraphWithDeps(t *testing.T) {
	stages := []Stage{
		{Name: "fmt", Cmd: []string{"echo", "fmt"}},
		{Name: "clippy", Cmd: []string{"echo", "clippy"}, DependsOn: "fmt"},
		{Name: "test", Cmd: []string{"echo", "test"}, DependsOn: "fmt"},
		{Name: "deny", Cmd: []string{"echo", "deny"}},
	}

	deps, err := buildDepGraph(stages)
	if err != nil {
		t.Fatalf("buildDepGraph failed: %v", err)
	}

	if deps["clippy"] != "fmt" {
		t.Errorf("clippy should depend on fmt, got %q", deps["clippy"])
	}
	if deps["test"] != "fmt" {
		t.Errorf("test should depend on fmt, got %q", deps["test"])
	}
	if deps["deny"] != "" {
		t.Errorf("deny should have no dependency, got %q", deps["deny"])
	}
}

func TestBuildDepGraphMissingTarget(t *testing.T) {
	stages := []Stage{
		{Name: "test", Cmd: []string{"echo", "test"}, DependsOn: "nonexistent"},
	}

	_, err := buildDepGraph(stages)
	if err == nil {
		t.Fatal("expected error for missing dependency target")
	}
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("error should mention missing dep, got %q", err.Error())
	}
}

func TestBuildDepGraphCyclicDeps(t *testing.T) {
	stages := []Stage{
		{Name: "a", Cmd: []string{"echo", "a"}, DependsOn: "b"},
		{Name: "b", Cmd: []string{"echo", "b"}, DependsOn: "a"},
	}

	_, err := buildDepGraph(stages)
	if err == nil {
		t.Fatal("expected error for circular dependency")
	}
	if !strings.Contains(err.Error(), "circular") {
		t.Errorf("error should mention circular, got %q", err.Error())
	}
}

func TestDetectCyclesSelfRef(t *testing.T) {
	stages := []Stage{
		{Name: "a", Cmd: []string{"echo"}, DependsOn: "a"},
	}

	_, err := buildDepGraph(stages)
	if err == nil {
		t.Fatal("expected error for self-referencing dependency")
	}
	if !strings.Contains(err.Error(), "circular") {
		t.Errorf("error should mention circular, got %q", err.Error())
	}
}

func TestResolveOrderLayers(t *testing.T) {
	stages := []Stage{
		{Name: "fmt", Cmd: []string{"echo", "fmt"}},
		{Name: "clippy", Cmd: []string{"echo", "clippy"}, DependsOn: "fmt"},
		{Name: "test", Cmd: []string{"echo", "test"}, DependsOn: "fmt"},
		{Name: "deny", Cmd: []string{"echo", "deny"}},
	}

	layers, err := resolveOrder(stages)
	if err != nil {
		t.Fatalf("resolveOrder failed: %v", err)
	}

	if len(layers) != 2 {
		t.Fatalf("expected 2 layers, got %d", len(layers))
	}

	// Layer 0: fmt and deny (no deps)
	layer0Names := make(map[string]bool)
	for _, s := range layers[0] {
		layer0Names[s.Name] = true
	}
	if !layer0Names["fmt"] || !layer0Names["deny"] {
		t.Errorf("layer 0 should contain fmt and deny, got %v", layers[0])
	}

	// Layer 1: clippy and test (depend on fmt)
	layer1Names := make(map[string]bool)
	for _, s := range layers[1] {
		layer1Names[s.Name] = true
	}
	if !layer1Names["clippy"] || !layer1Names["test"] {
		t.Errorf("layer 1 should contain clippy and test, got %v", layers[1])
	}
}

func TestResolveOrderChainedDeps(t *testing.T) {
	stages := []Stage{
		{Name: "fmt", Cmd: []string{"echo", "fmt"}},
		{Name: "clippy", Cmd: []string{"echo", "clippy"}, DependsOn: "fmt"},
		{Name: "test", Cmd: []string{"echo", "test"}, DependsOn: "clippy"},
	}

	layers, err := resolveOrder(stages)
	if err != nil {
		t.Fatalf("resolveOrder failed: %v", err)
	}

	if len(layers) != 3 {
		t.Fatalf("expected 3 layers for chained deps, got %d", len(layers))
	}

	if layers[0][0].Name != "fmt" {
		t.Errorf("layer 0 should be fmt, got %q", layers[0][0].Name)
	}
	if layers[1][0].Name != "clippy" {
		t.Errorf("layer 1 should be clippy, got %q", layers[1][0].Name)
	}
	if layers[2][0].Name != "test" {
		t.Errorf("layer 2 should be test, got %q", layers[2][0].Name)
	}
}

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
		SourceHash:  "testhash",
	}

	results, err := pr.RunParallel()
	if err != nil {
		t.Fatalf("RunParallel failed: %v", err)
	}

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
		{Name: "s2", Cmd: []string{"sh", "-c", fmt.Sprintf("cat %s", markerFile)}, Timeout: 10, DependsOn: "s1"},
	}

	pr := &ParallelRunner{
		Stages:      stages,
		Concurrency: 2,
		Cwd:         dir,
		NoCache:     true,
		Cache:       make(map[string]string),
		SourceHash:  "testhash",
	}

	results, err := pr.RunParallel()
	if err != nil {
		t.Fatalf("RunParallel failed: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// Both should pass because s1 ran before s2
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
		{Name: "should-skip", Cmd: []string{"echo", "ran"}, Timeout: 10, DependsOn: "fail-first"},
	}

	pr := &ParallelRunner{
		Stages:      stages,
		Concurrency: 1,
		Cwd:         dir,
		NoCache:     true,
		Cache:       make(map[string]string),
		SourceHash:  "testhash",
		FailFast:    true,
	}

	results, err := pr.RunParallel()
	if err != nil {
		t.Fatalf("RunParallel failed: %v", err)
	}

	// With fail-fast, only the first stage should have run
	if len(results) != 1 {
		t.Fatalf("expected 1 result with fail-fast, got %d", len(results))
	}
	if results[0].Status != "fail" {
		t.Errorf("first stage should fail, got %q", results[0].Status)
	}
}

func TestParallelRunnerCacheHit(t *testing.T) {
	dir := t.TempDir()

	stages := []Stage{
		{Name: "cached", Cmd: []string{"echo", "hello"}, Timeout: 10},
	}

	cache := map[string]string{
		"cached": "hash123|echo hello",
	}

	pr := &ParallelRunner{
		Stages:     stages,
		Cwd:        dir,
		NoCache:    false,
		Cache:      cache,
		SourceHash: "hash123",
	}

	results, err := pr.RunParallel()
	if err != nil {
		t.Fatalf("RunParallel failed: %v", err)
	}

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

	// 4 independent stages with concurrency=2
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
		SourceHash:  "testhash",
	}

	results, err := pr.RunParallel()
	if err != nil {
		t.Fatalf("RunParallel failed: %v", err)
	}

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
depends_on = "fmt"
timeout = 10
enabled = true

[stages.test]
command = ["echo", "test"]
depends_on = "fmt"
timeout = 10
enabled = true
`
	os.WriteFile(filepath.Join(dir, ".local-ci.toml"), []byte(configContent), 0644)
	os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte("[package]\nname = \"x\"\n"), 0644)

	config, err := LoadConfig(dir, ProjectKindRust)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if config.Stages["clippy"].DependsOn != "fmt" {
		t.Errorf("clippy should depend on fmt, got %q", config.Stages["clippy"].DependsOn)
	}
	if config.Stages["test"].DependsOn != "fmt" {
		t.Errorf("test should depend on fmt, got %q", config.Stages["test"].DependsOn)
	}
	if config.Stages["fmt"].DependsOn != "" {
		t.Errorf("fmt should have no dependency, got %q", config.Stages["fmt"].DependsOn)
	}
}
