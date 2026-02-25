package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// newTestMCPContext creates an mcpContext with a temp dir and minimal config.
func newTestMCPContext(t *testing.T, stages map[string]Stage) *mcpContext {
	t.Helper()
	tmpDir := t.TempDir()

	// Create a minimal Cargo.toml so project detection works
	os.WriteFile(filepath.Join(tmpDir, "Cargo.toml"), []byte(`[package]
name = "test"
version = "0.1.0"
`), 0644)

	srcDir := filepath.Join(tmpDir, "src")
	os.MkdirAll(srcDir, 0755)
	os.WriteFile(filepath.Join(srcDir, "lib.rs"), []byte("pub fn x() {}"), 0644)

	cfg := &Config{
		Stages: stages,
		Cache: CacheConfig{
			IncludePatterns: []string{"*.rs"},
			SkipDirs:        []string{"target"},
		},
	}

	ws, _ := DetectWorkspace(tmpDir)
	return &mcpContext{root: tmpDir, config: cfg, ws: ws}
}

func makeCallToolRequest(args map[string]interface{}) mcp.CallToolRequest {
	return mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: args,
		},
	}
}

// --- get_stages tests ---

func TestHandleGetStages_ReturnsAllStages(t *testing.T) {
	mc := newTestMCPContext(t, map[string]Stage{
		"fmt":  {Cmd: []string{"echo", "fmt"}, Enabled: true},
		"test": {Cmd: []string{"echo", "test"}, Enabled: false},
	})

	result, err := mc.handleGetStages(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var stages []struct {
		Name     string `json:"name"`
		Enabled  bool   `json:"enabled"`
		CacheHit bool   `json:"cache_hit"`
		Command  string `json:"command"`
	}
	text := result.Content[0].(mcp.TextContent).Text
	if err := json.Unmarshal([]byte(text), &stages); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if len(stages) != 2 {
		t.Fatalf("expected 2 stages, got %d", len(stages))
	}

	found := map[string]bool{}
	for _, s := range stages {
		found[s.Name] = true
	}
	if !found["fmt"] || !found["test"] {
		t.Errorf("missing expected stages: %v", found)
	}
}

func TestHandleGetStages_CacheHitAfterRun(t *testing.T) {
	mc := newTestMCPContext(t, map[string]Stage{
		"echo": {Cmd: []string{"echo", "hello"}, Enabled: true, Timeout: 10},
	})

	// Run the stage to populate cache
	stage := mc.config.Stages["echo"]
	stage.Name = "echo"
	mc.executeStage(context.Background(), stage)

	result, err := mc.handleGetStages(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var stages []struct {
		Name     string `json:"name"`
		CacheHit bool   `json:"cache_hit"`
	}
	text := result.Content[0].(mcp.TextContent).Text
	json.Unmarshal([]byte(text), &stages)

	if len(stages) != 1 {
		t.Fatalf("expected 1 stage, got %d", len(stages))
	}
	if !stages[0].CacheHit {
		t.Errorf("expected cache hit after successful run")
	}
}

// --- get_stale_stages tests ---

func TestHandleGetStaleStages_AllStaleInitially(t *testing.T) {
	mc := newTestMCPContext(t, map[string]Stage{
		"fmt":  {Cmd: []string{"echo", "fmt"}, Enabled: true},
		"test": {Cmd: []string{"echo", "test"}, Enabled: true},
	})

	result, err := mc.handleGetStaleStages(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var stale []struct {
		Name   string `json:"name"`
		Reason string `json:"reason"`
	}
	text := result.Content[0].(mcp.TextContent).Text
	json.Unmarshal([]byte(text), &stale)

	if len(stale) != 2 {
		t.Fatalf("expected 2 stale stages, got %d", len(stale))
	}
	for _, s := range stale {
		if s.Reason != "never run" {
			t.Errorf("expected reason 'never run', got %q for %s", s.Reason, s.Name)
		}
	}
}

func TestHandleGetStaleStages_ExcludesDisabled(t *testing.T) {
	mc := newTestMCPContext(t, map[string]Stage{
		"fmt":  {Cmd: []string{"echo", "fmt"}, Enabled: true},
		"deny": {Cmd: []string{"echo", "deny"}, Enabled: false},
	})

	result, _ := mc.handleGetStaleStages(context.Background(), mcp.CallToolRequest{})
	var stale []struct {
		Name string `json:"name"`
	}
	text := result.Content[0].(mcp.TextContent).Text
	json.Unmarshal([]byte(text), &stale)

	if len(stale) != 1 {
		t.Fatalf("expected 1 stale stage, got %d", len(stale))
	}
	if stale[0].Name != "fmt" {
		t.Errorf("expected stale stage 'fmt', got %q", stale[0].Name)
	}
}

func TestHandleGetStaleStages_NoneAfterRun(t *testing.T) {
	mc := newTestMCPContext(t, map[string]Stage{
		"echo": {Cmd: []string{"echo", "ok"}, Enabled: true, Timeout: 10},
	})

	stage := mc.config.Stages["echo"]
	stage.Name = "echo"
	mc.executeStage(context.Background(), stage)

	result, _ := mc.handleGetStaleStages(context.Background(), mcp.CallToolRequest{})
	var stale []struct {
		Name string `json:"name"`
	}
	text := result.Content[0].(mcp.TextContent).Text
	json.Unmarshal([]byte(text), &stale)

	if len(stale) != 0 {
		t.Errorf("expected 0 stale stages after run, got %d", len(stale))
	}
}

// --- run_stage tests ---

func TestHandleRunStage_Success(t *testing.T) {
	mc := newTestMCPContext(t, map[string]Stage{
		"echo": {Cmd: []string{"echo", "hello"}, Enabled: true, Timeout: 10},
	})

	req := makeCallToolRequest(map[string]interface{}{"name": "echo"})
	result, err := mc.handleRunStage(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var rj ResultJSON
	text := result.Content[0].(mcp.TextContent).Text
	json.Unmarshal([]byte(text), &rj)

	if rj.Status != "pass" {
		t.Errorf("expected pass, got %q", rj.Status)
	}
	if rj.Name != "echo" {
		t.Errorf("expected name 'echo', got %q", rj.Name)
	}
}

func TestHandleRunStage_Failure(t *testing.T) {
	mc := newTestMCPContext(t, map[string]Stage{
		"fail": {Cmd: []string{"false"}, Enabled: true, Timeout: 10},
	})

	req := makeCallToolRequest(map[string]interface{}{"name": "fail"})
	result, _ := mc.handleRunStage(context.Background(), req)

	var rj ResultJSON
	text := result.Content[0].(mcp.TextContent).Text
	json.Unmarshal([]byte(text), &rj)

	if rj.Status != "fail" {
		t.Errorf("expected fail, got %q", rj.Status)
	}
}

func TestHandleRunStage_UnknownStage(t *testing.T) {
	mc := newTestMCPContext(t, map[string]Stage{})

	req := makeCallToolRequest(map[string]interface{}{"name": "nonexistent"})
	result, _ := mc.handleRunStage(context.Background(), req)

	text := result.Content[0].(mcp.TextContent).Text
	if text == "" {
		t.Error("expected error message for unknown stage")
	}
	if !result.IsError {
		t.Error("expected IsError=true for unknown stage")
	}
}

func TestHandleRunStage_CacheHitOnSecondRun(t *testing.T) {
	mc := newTestMCPContext(t, map[string]Stage{
		"echo": {Cmd: []string{"echo", "cached"}, Enabled: true, Timeout: 10},
	})

	req := makeCallToolRequest(map[string]interface{}{"name": "echo"})
	mc.handleRunStage(context.Background(), req)

	// Second run should be cache hit
	result, _ := mc.handleRunStage(context.Background(), req)
	var rj ResultJSON
	text := result.Content[0].(mcp.TextContent).Text
	json.Unmarshal([]byte(text), &rj)

	if !rj.CacheHit {
		t.Error("expected cache hit on second run")
	}
	if rj.Status != "pass" {
		t.Errorf("expected pass on cache hit, got %q", rj.Status)
	}
}

func TestHandleRunStage_DisabledStage(t *testing.T) {
	mc := newTestMCPContext(t, map[string]Stage{
		"deny": {Cmd: []string{"echo", "deny"}, Enabled: false, Timeout: 10},
	})

	req := makeCallToolRequest(map[string]interface{}{"name": "deny"})
	result, _ := mc.handleRunStage(context.Background(), req)

	if !result.IsError {
		t.Error("expected IsError=true for disabled stage")
	}
	text := result.Content[0].(mcp.TextContent).Text
	if text == "" {
		t.Error("expected error message for disabled stage")
	}
}

// --- run_all tests ---

func TestHandleRunAll_RunsEnabledOnly(t *testing.T) {
	mc := newTestMCPContext(t, map[string]Stage{
		"a": {Cmd: []string{"echo", "a"}, Enabled: true, Timeout: 10},
		"b": {Cmd: []string{"echo", "b"}, Enabled: false, Timeout: 10},
		"c": {Cmd: []string{"echo", "c"}, Enabled: true, Timeout: 10},
	})

	result, err := mc.handleRunAll(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var results []ResultJSON
	text := result.Content[0].(mcp.TextContent).Text
	json.Unmarshal([]byte(text), &results)

	if len(results) != 2 {
		t.Fatalf("expected 2 results (enabled only), got %d", len(results))
	}
}

func TestHandleRunAll_ReportsFailure(t *testing.T) {
	mc := newTestMCPContext(t, map[string]Stage{
		"ok":   {Cmd: []string{"echo", "ok"}, Enabled: true, Timeout: 10},
		"fail": {Cmd: []string{"false"}, Enabled: true, Timeout: 10},
	})

	result, _ := mc.handleRunAll(context.Background(), mcp.CallToolRequest{})

	var results []ResultJSON
	text := result.Content[0].(mcp.TextContent).Text
	json.Unmarshal([]byte(text), &results)

	failCount := 0
	for _, r := range results {
		if r.Status == "fail" {
			failCount++
		}
	}
	if failCount == 0 {
		t.Error("expected at least one failed result")
	}
}

// --- invalidate tests ---

func TestHandleInvalidate_ClearsCache(t *testing.T) {
	mc := newTestMCPContext(t, map[string]Stage{
		"echo": {Cmd: []string{"echo", "hi"}, Enabled: true, Timeout: 10},
	})

	// Run to populate cache
	stage := mc.config.Stages["echo"]
	stage.Name = "echo"
	mc.executeStage(context.Background(), stage)

	// Verify cache hit
	req := makeCallToolRequest(map[string]interface{}{"name": "echo"})
	r1, _ := mc.handleRunStage(context.Background(), req)
	var rj1 ResultJSON
	json.Unmarshal([]byte(r1.Content[0].(mcp.TextContent).Text), &rj1)
	if !rj1.CacheHit {
		t.Fatal("expected cache hit before invalidation")
	}

	// Invalidate
	invReq := makeCallToolRequest(map[string]interface{}{"name": "echo"})
	invResult, _ := mc.handleInvalidate(context.Background(), invReq)
	invText := invResult.Content[0].(mcp.TextContent).Text
	if invResult.IsError {
		t.Fatalf("invalidate failed: %s", invText)
	}

	// Now stage should be stale
	staleResult, _ := mc.handleGetStaleStages(context.Background(), mcp.CallToolRequest{})
	var stale []struct {
		Name string `json:"name"`
	}
	json.Unmarshal([]byte(staleResult.Content[0].(mcp.TextContent).Text), &stale)
	if len(stale) != 1 || stale[0].Name != "echo" {
		t.Errorf("expected echo to be stale after invalidation, got %v", stale)
	}
}

func TestHandleInvalidate_UnknownStage(t *testing.T) {
	mc := newTestMCPContext(t, map[string]Stage{})

	req := makeCallToolRequest(map[string]interface{}{"name": "nonexistent"})
	result, _ := mc.handleInvalidate(context.Background(), req)

	if !result.IsError {
		t.Error("expected error for unknown stage")
	}
}

func TestHandleInvalidate_NoCacheEntry(t *testing.T) {
	mc := newTestMCPContext(t, map[string]Stage{
		"echo": {Cmd: []string{"echo", "hi"}, Enabled: true},
	})

	req := makeCallToolRequest(map[string]interface{}{"name": "echo"})
	result, _ := mc.handleInvalidate(context.Background(), req)

	text := result.Content[0].(mcp.TextContent).Text
	var resp struct {
		Status string `json:"status"`
	}
	json.Unmarshal([]byte(text), &resp)
	if resp.Status != "no_cache_entry" {
		t.Errorf("expected no_cache_entry status, got %q", resp.Status)
	}
}

// --- get_source_hash tests ---

func TestHandleGetSourceHash_ReturnsHash(t *testing.T) {
	mc := newTestMCPContext(t, map[string]Stage{})

	result, err := mc.handleGetSourceHash(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var resp struct {
		Hash string `json:"hash"`
	}
	text := result.Content[0].(mcp.TextContent).Text
	json.Unmarshal([]byte(text), &resp)

	if resp.Hash == "" {
		t.Error("expected non-empty hash")
	}
}

func TestHandleGetSourceHash_DeterministicForSameContent(t *testing.T) {
	mc := newTestMCPContext(t, map[string]Stage{})

	r1, _ := mc.handleGetSourceHash(context.Background(), mcp.CallToolRequest{})
	r2, _ := mc.handleGetSourceHash(context.Background(), mcp.CallToolRequest{})

	t1 := r1.Content[0].(mcp.TextContent).Text
	t2 := r2.Content[0].(mcp.TextContent).Text

	if t1 != t2 {
		t.Errorf("hash not deterministic: %q vs %q", t1, t2)
	}
}

func TestHandleGetSourceHash_ChangesWithSource(t *testing.T) {
	mc := newTestMCPContext(t, map[string]Stage{})

	r1, _ := mc.handleGetSourceHash(context.Background(), mcp.CallToolRequest{})
	t1 := r1.Content[0].(mcp.TextContent).Text

	// Modify source
	os.WriteFile(filepath.Join(mc.root, "src", "lib.rs"), []byte("pub fn changed() {}"), 0644)

	r2, _ := mc.handleGetSourceHash(context.Background(), mcp.CallToolRequest{})
	t2 := r2.Content[0].(mcp.TextContent).Text

	if t1 == t2 {
		t.Error("hash should change when source changes")
	}
}

// --- get_workspace tests ---

func TestHandleGetWorkspace_SingleProject(t *testing.T) {
	mc := newTestMCPContext(t, map[string]Stage{})

	result, err := mc.handleGetWorkspace(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var info struct {
		Root        string   `json:"root"`
		ProjectType string   `json:"project_type"`
		Members     []string `json:"members"`
		IsSingle    bool     `json:"is_single"`
	}
	text := result.Content[0].(mcp.TextContent).Text
	json.Unmarshal([]byte(text), &info)

	if info.Root != mc.root {
		t.Errorf("expected root %q, got %q", mc.root, info.Root)
	}
	if info.ProjectType != "rust" {
		t.Errorf("expected project_type 'rust', got %q", info.ProjectType)
	}
	if !info.IsSingle {
		t.Error("expected is_single=true for single crate")
	}
}

// --- executeStage tests ---

func TestExecuteStage_PassingCommand(t *testing.T) {
	mc := newTestMCPContext(t, map[string]Stage{})
	stage := Stage{Name: "echo", Cmd: []string{"echo", "hello world"}, Timeout: 10}

	r := mc.executeStage(context.Background(), stage)
	if r.Status != "pass" {
		t.Errorf("expected pass, got %q", r.Status)
	}
	if r.CacheHit {
		t.Error("first run should not be cache hit")
	}
}

func TestExecuteStage_RecordsDuration(t *testing.T) {
	mc := newTestMCPContext(t, map[string]Stage{})
	stage := Stage{Name: "sleep", Cmd: []string{"sleep", "0.05"}, Timeout: 10}

	r := mc.executeStage(context.Background(), stage)
	if r.Duration == 0 {
		t.Error("expected non-zero duration for executed stage")
	}
	if r.Duration.Milliseconds() < 40 {
		t.Errorf("duration too short: %v", r.Duration)
	}
}

func TestExecuteStage_FailingCommand(t *testing.T) {
	mc := newTestMCPContext(t, map[string]Stage{})
	stage := Stage{Name: "fail", Cmd: []string{"false"}, Timeout: 10}

	r := mc.executeStage(context.Background(), stage)
	if r.Status != "fail" {
		t.Errorf("expected fail, got %q", r.Status)
	}
	if r.Error == nil {
		t.Error("expected non-nil error on failure")
	}
}

func TestExecuteStage_CacheOnSuccess(t *testing.T) {
	mc := newTestMCPContext(t, map[string]Stage{})
	stage := Stage{Name: "echo", Cmd: []string{"echo", "cache me"}, Timeout: 10}

	r1 := mc.executeStage(context.Background(), stage)
	if r1.CacheHit {
		t.Error("first run should not be cache hit")
	}

	r2 := mc.executeStage(context.Background(), stage)
	if !r2.CacheHit {
		t.Error("second run should be cache hit")
	}
	if r2.Status != "pass" {
		t.Errorf("cache hit should have pass status, got %q", r2.Status)
	}
}

func TestExecuteStage_NoCacheOnFailure(t *testing.T) {
	mc := newTestMCPContext(t, map[string]Stage{})
	stage := Stage{Name: "fail", Cmd: []string{"false"}, Timeout: 10}

	mc.executeStage(context.Background(), stage)
	r2 := mc.executeStage(context.Background(), stage)

	if r2.CacheHit {
		t.Error("failed commands should not be cached")
	}
}

// --- resultToMCP / resultsToMCP tests ---

func TestResultToMCP_IncludesAllFields(t *testing.T) {
	mc := newTestMCPContext(t, map[string]Stage{})
	r := Result{
		Name:    "test",
		Command: "echo test",
		Status:  "pass",
		Output:  "test output",
	}

	result := mc.resultToMCP(r)
	var rj ResultJSON
	text := result.Content[0].(mcp.TextContent).Text
	json.Unmarshal([]byte(text), &rj)

	if rj.Name != "test" {
		t.Errorf("expected name 'test', got %q", rj.Name)
	}
	if rj.Status != "pass" {
		t.Errorf("expected status 'pass', got %q", rj.Status)
	}
	if rj.Output != "test output" {
		t.Errorf("expected output 'test output', got %q", rj.Output)
	}
}

func TestResultsToMCP_MultipleResults(t *testing.T) {
	mc := newTestMCPContext(t, map[string]Stage{})
	results := []Result{
		{Name: "a", Status: "pass"},
		{Name: "b", Status: "fail"},
	}

	result := mc.resultsToMCP(results)
	var rjs []ResultJSON
	text := result.Content[0].(mcp.TextContent).Text
	json.Unmarshal([]byte(text), &rjs)

	if len(rjs) != 2 {
		t.Fatalf("expected 2 results, got %d", len(rjs))
	}
}
