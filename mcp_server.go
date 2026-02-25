package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// mcpContext holds shared state for MCP tool handlers.
type mcpContext struct {
	root   string
	config *Config
	ws     *Workspace
}

func cmdServe(root string) error {
	config, err := LoadConfig(root)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	ws, _ := DetectWorkspace(root)

	ctx := &mcpContext{root: root, config: config, ws: ws}

	s := server.NewMCPServer(
		"local-ci",
		version,
		server.WithToolCapabilities(false),
	)

	// run_stage: Run a single named stage
	s.AddTool(mcp.NewTool("run_stage",
		mcp.WithDescription("Run a single CI stage by name and return the result"),
		mcp.WithString("name", mcp.Required(), mcp.Description("Stage name to run (e.g. fmt, clippy, test)")),
	), ctx.handleRunStage)

	// run_all: Run all enabled stages
	s.AddTool(mcp.NewTool("run_all",
		mcp.WithDescription("Run all enabled CI stages and return results"),
	), ctx.handleRunAll)

	// get_stages: List stages with cache/enabled status
	s.AddTool(mcp.NewTool("get_stages",
		mcp.WithDescription("List all stages with their enabled status and cache state"),
	), ctx.handleGetStages)

	// get_stale_stages: List stages that would run (cache miss)
	s.AddTool(mcp.NewTool("get_stale_stages",
		mcp.WithDescription("List stages that need to run (cache miss or no cache)"),
	), ctx.handleGetStaleStages)

	// invalidate: Clear cache for a specific stage
	s.AddTool(mcp.NewTool("invalidate",
		mcp.WithDescription("Clear the cache for a specific stage, forcing it to re-run next time"),
		mcp.WithString("name", mcp.Required(), mcp.Description("Stage name to invalidate")),
	), ctx.handleInvalidate)

	// get_source_hash: Current source hash
	s.AddTool(mcp.NewTool("get_source_hash",
		mcp.WithDescription("Compute and return the current source hash used for cache keys"),
	), ctx.handleGetSourceHash)

	// get_workspace: Workspace structure
	s.AddTool(mcp.NewTool("get_workspace",
		mcp.WithDescription("Return the detected workspace structure (members, excludes, project type)"),
	), ctx.handleGetWorkspace)

	return server.ServeStdio(s)
}

func (mc *mcpContext) handleRunStage(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name, err := req.RequireString("name")
	if err != nil {
		return mcp.NewToolResultError("missing required parameter: name"), nil
	}

	stage, ok := mc.config.Stages[name]
	if !ok {
		return mcp.NewToolResultError(fmt.Sprintf("unknown stage: %s", name)), nil
	}
	stage.Name = name

	result := mc.executeStage(stage)
	return mc.resultToMCP(result), nil
}

func (mc *mcpContext) handleRunAll(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	enabledNames := mc.config.GetEnabledStages()
	var results []Result
	for _, name := range enabledNames {
		stage := mc.config.Stages[name]
		stage.Name = name
		r := mc.executeStage(stage)
		results = append(results, r)
	}
	return mc.resultsToMCP(results), nil
}

func (mc *mcpContext) handleGetStages(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	hash, _ := computeSourceHash(mc.root, mc.config, mc.ws)
	cache, _ := loadCache(mc.root)

	type stageInfo struct {
		Name     string `json:"name"`
		Enabled  bool   `json:"enabled"`
		CacheHit bool   `json:"cache_hit"`
		Command  string `json:"command"`
	}

	var stages []stageInfo
	for name, stage := range mc.config.Stages {
		cmdStr := strings.Join(stage.Cmd, " ")
		cacheKey := hash + "|" + cmdStr
		hit := cache[name] == cacheKey
		stages = append(stages, stageInfo{
			Name:     name,
			Enabled:  stage.Enabled,
			CacheHit: hit,
			Command:  cmdStr,
		})
	}

	data, _ := json.Marshal(stages)
	return mcp.NewToolResultText(string(data)), nil
}

func (mc *mcpContext) handleGetStaleStages(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	hash, _ := computeSourceHash(mc.root, mc.config, mc.ws)
	cache, _ := loadCache(mc.root)

	type staleStage struct {
		Name   string `json:"name"`
		Reason string `json:"reason"`
	}

	var stale []staleStage
	for name, stage := range mc.config.Stages {
		if !stage.Enabled {
			continue
		}
		cmdStr := strings.Join(stage.Cmd, " ")
		cacheKey := hash + "|" + cmdStr
		if cache[name] != cacheKey {
			reason := "cache miss"
			if _, exists := cache[name]; !exists {
				reason = "never run"
			} else {
				reason = "source changed"
			}
			stale = append(stale, staleStage{Name: name, Reason: reason})
		}
	}

	data, _ := json.Marshal(stale)
	return mcp.NewToolResultText(string(data)), nil
}

func (mc *mcpContext) handleInvalidate(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name, err := req.RequireString("name")
	if err != nil {
		return mcp.NewToolResultError("missing required parameter: name"), nil
	}

	if _, ok := mc.config.Stages[name]; !ok {
		return mcp.NewToolResultError(fmt.Sprintf("unknown stage: %s", name)), nil
	}

	cache, _ := loadCache(mc.root)
	if _, exists := cache[name]; !exists {
		return mcp.NewToolResultText(fmt.Sprintf(`{"stage":"%s","status":"no_cache_entry"}`, name)), nil
	}
	delete(cache, name)
	if err := saveCache(cache, mc.root); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to save cache: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf(`{"stage":"%s","status":"invalidated"}`, name)), nil
}

func (mc *mcpContext) handleGetSourceHash(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	hash, err := computeSourceHash(mc.root, mc.config, mc.ws)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to compute hash: %v", err)), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf(`{"hash":"%s"}`, hash)), nil
}

func (mc *mcpContext) handleGetWorkspace(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	projectType := DetectProjectType(mc.root)

	type wsInfo struct {
		Root        string   `json:"root"`
		ProjectType string   `json:"project_type"`
		Members     []string `json:"members"`
		IsSingle    bool     `json:"is_single"`
	}

	info := wsInfo{
		Root:        mc.root,
		ProjectType: string(projectType),
		Members:     []string{},
		IsSingle:    true,
	}

	if mc.ws != nil {
		info.Members = mc.ws.GetMembers()
		info.IsSingle = mc.ws.IsSingle
	}

	data, _ := json.Marshal(info)
	return mcp.NewToolResultText(string(data)), nil
}

// executeStage runs a single stage locally and returns the result.
func (mc *mcpContext) executeStage(stage Stage) Result {
	cmdStr := strings.Join(stage.Cmd, " ")

	// Check cache
	hash, _ := computeSourceHash(mc.root, mc.config, mc.ws)
	cache, _ := loadCache(mc.root)
	cacheKey := hash + "|" + cmdStr
	if cache[stage.Name] == cacheKey {
		return Result{
			Name:     stage.Name,
			Command:  cmdStr,
			Status:   "pass",
			Duration: 0,
			CacheHit: true,
		}
	}

	// Execute
	timeout := time.Duration(stage.Timeout) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, stage.Cmd[0], stage.Cmd[1:]...)
	cmd.Dir = mc.root
	cmd.Env = os.Environ()
	output, err := cmd.CombinedOutput()

	result := Result{
		Name:    stage.Name,
		Command: cmdStr,
		Output:  string(output),
	}

	if err != nil {
		result.Status = "fail"
		result.Error = err
	} else {
		result.Status = "pass"
		// Update cache on success
		cache[stage.Name] = cacheKey
		_ = saveCache(cache, mc.root)
	}

	return result
}

func (mc *mcpContext) resultToMCP(r Result) *mcp.CallToolResult {
	rj := ResultJSON{
		Name:       r.Name,
		Command:    r.Command,
		Status:     r.Status,
		DurationMS: r.Duration.Milliseconds(),
		CacheHit:   r.CacheHit,
		Output:     r.Output,
	}
	if r.Error != nil {
		rj.Error = r.Error.Error()
	}
	data, _ := json.Marshal(rj)
	return mcp.NewToolResultText(string(data))
}

func (mc *mcpContext) resultsToMCP(results []Result) *mcp.CallToolResult {
	var rjs []ResultJSON
	for _, r := range results {
		rj := ResultJSON{
			Name:       r.Name,
			Command:    r.Command,
			Status:     r.Status,
			DurationMS: r.Duration.Milliseconds(),
			CacheHit:   r.CacheHit,
			Output:     r.Output,
		}
		if r.Error != nil {
			rj.Error = r.Error.Error()
		}
		rjs = append(rjs, rj)
	}
	data, _ := json.Marshal(rjs)
	return mcp.NewToolResultText(string(data))
}
