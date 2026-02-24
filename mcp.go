// MCP (Model Context Protocol) server for local-ci.
//
// Exposes local-ci functionality as MCP tools over stdio (JSON-RPC 2.0).
// AI agents can query cache state, run individual stages, and get
// structured feedback without shelling out.
//
// Usage:
//
//	local-ci serve
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// JSON-RPC 2.0 types
type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *rpcError   `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// MCP types
type mcpToolDef struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	InputSchema mcpInputSchema    `json:"inputSchema"`
}

type mcpInputSchema struct {
	Type       string                       `json:"type"`
	Properties map[string]mcpPropertySchema `json:"properties,omitempty"`
	Required   []string                     `json:"required,omitempty"`
}

type mcpPropertySchema struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

type mcpToolResult struct {
	Content []mcpContent `json:"content"`
}

type mcpContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// MCPServer holds the state for the MCP server
type MCPServer struct {
	root   string
	config *Config
	ws     *Workspace
	cache  map[string]string
}

func serveMCP() {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot get working directory: %v\n", err)
		os.Exit(1)
	}

	kind := DetectProjectKind(cwd)
	if kind == ProjectKindUnknown {
		fmt.Fprintf(os.Stderr, "No project detected in %s\n", cwd)
		os.Exit(1)
	}

	config, err := LoadConfig(cwd, kind)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	var ws *Workspace
	switch kind {
	case ProjectKindTypeScript:
		ws, _ = DetectTypeScriptWorkspace(cwd)
	default:
		ws, _ = DetectWorkspace(cwd)
	}

	cache, _ := loadCache(cwd)
	if cache == nil {
		cache = make(map[string]string)
	}

	server := &MCPServer{
		root:   cwd,
		config: config,
		ws:     ws,
		cache:  cache,
	}

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req jsonRPCRequest
		if err := json.Unmarshal(line, &req); err != nil {
			continue
		}

		resp := server.handleRequest(req)
		if resp != nil {
			data, _ := json.Marshal(resp)
			fmt.Fprintf(os.Stdout, "%s\n", data)
		}
	}
}

func (s *MCPServer) handleRequest(req jsonRPCRequest) *jsonRPCResponse {
	switch req.Method {
	case "initialize":
		return &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]interface{}{
				"protocolVersion": "2024-11-05",
				"capabilities": map[string]interface{}{
					"tools": map[string]interface{}{},
				},
				"serverInfo": map[string]interface{}{
					"name":    "local-ci",
					"version": version,
				},
			},
		}

	case "notifications/initialized":
		return nil // no response for notifications

	case "tools/list":
		return &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]interface{}{
				"tools": s.toolDefinitions(),
			},
		}

	case "tools/call":
		var params struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return s.errorResponse(req.ID, -32602, "Invalid params")
		}
		return s.callTool(req.ID, params.Name, params.Arguments)

	default:
		return s.errorResponse(req.ID, -32601, "Method not found: "+req.Method)
	}
}

func (s *MCPServer) toolDefinitions() []mcpToolDef {
	return []mcpToolDef{
		{
			Name:        "run_stage",
			Description: "Run a single CI stage and return the result",
			InputSchema: mcpInputSchema{
				Type: "object",
				Properties: map[string]mcpPropertySchema{
					"name": {Type: "string", Description: "Stage name (e.g. fmt, clippy, test)"},
				},
				Required: []string{"name"},
			},
		},
		{
			Name:        "run_all",
			Description: "Run all enabled CI stages and return results",
			InputSchema: mcpInputSchema{Type: "object"},
		},
		{
			Name:        "get_stages",
			Description: "List all stages with their cache and enabled status",
			InputSchema: mcpInputSchema{Type: "object"},
		},
		{
			Name:        "get_stale_stages",
			Description: "List stages that need to run (cache miss or hash changed)",
			InputSchema: mcpInputSchema{Type: "object"},
		},
		{
			Name:        "invalidate",
			Description: "Clear cache for a specific stage",
			InputSchema: mcpInputSchema{
				Type: "object",
				Properties: map[string]mcpPropertySchema{
					"name": {Type: "string", Description: "Stage name to invalidate"},
				},
				Required: []string{"name"},
			},
		},
		{
			Name:        "get_source_hash",
			Description: "Get the current source hash used for caching",
			InputSchema: mcpInputSchema{Type: "object"},
		},
		{
			Name:        "get_workspace",
			Description: "Get workspace structure information",
			InputSchema: mcpInputSchema{Type: "object"},
		},
	}
}

func (s *MCPServer) callTool(id json.RawMessage, name string, args json.RawMessage) *jsonRPCResponse {
	switch name {
	case "run_stage":
		return s.toolRunStage(id, args)
	case "run_all":
		return s.toolRunAll(id)
	case "get_stages":
		return s.toolGetStages(id)
	case "get_stale_stages":
		return s.toolGetStaleStages(id)
	case "invalidate":
		return s.toolInvalidate(id, args)
	case "get_source_hash":
		return s.toolGetSourceHash(id)
	case "get_workspace":
		return s.toolGetWorkspace(id)
	default:
		return s.errorResponse(id, -32602, "Unknown tool: "+name)
	}
}

func (s *MCPServer) toolRunStage(id json.RawMessage, args json.RawMessage) *jsonRPCResponse {
	var params struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return s.errorResponse(id, -32602, "Invalid arguments")
	}

	stage, ok := s.config.Stages[params.Name]
	if !ok {
		return s.errorResponse(id, -32602, "Unknown stage: "+params.Name)
	}

	sourceHash, _ := computeSourceHash(s.root, s.config, s.ws)
	cmdStr := strings.Join(stage.Cmd, " ")
	stageCacheKey := sourceHash + "|" + cmdStr

	// Check cache
	if s.cache[stage.Name] == stageCacheKey {
		return s.toolResult(id, map[string]interface{}{
			"name":      stage.Name,
			"status":    "pass",
			"cache_hit": true,
		})
	}

	// Execute
	result := s.executeStage(stage)

	if result.Status == "pass" {
		s.cache[stage.Name] = stageCacheKey
		saveCache(s.cache, s.root)
	}

	out := map[string]interface{}{
		"name":        result.Name,
		"status":      result.Status,
		"cache_hit":   false,
		"duration_ms": result.Duration.Milliseconds(),
	}
	if result.Output != "" {
		out["output"] = strings.TrimSpace(result.Output)
	}
	if result.Error != nil {
		out["error"] = result.Error.Error()
	}
	return s.toolResult(id, out)
}

func (s *MCPServer) toolRunAll(id json.RawMessage) *jsonRPCResponse {
	sourceHash, _ := computeSourceHash(s.root, s.config, s.ws)
	var results []map[string]interface{}

	for _, name := range s.config.GetEnabledStages() {
		stage := s.config.Stages[name]
		cmdStr := strings.Join(stage.Cmd, " ")
		stageCacheKey := sourceHash + "|" + cmdStr

		if s.cache[stage.Name] == stageCacheKey {
			results = append(results, map[string]interface{}{
				"name": stage.Name, "status": "pass", "cache_hit": true,
			})
			continue
		}

		result := s.executeStage(stage)
		if result.Status == "pass" {
			s.cache[stage.Name] = stageCacheKey
		}

		entry := map[string]interface{}{
			"name":        result.Name,
			"status":      result.Status,
			"cache_hit":   false,
			"duration_ms": result.Duration.Milliseconds(),
		}
		if result.Output != "" {
			entry["output"] = strings.TrimSpace(result.Output)
		}
		results = append(results, entry)
	}

	saveCache(s.cache, s.root)
	return s.toolResult(id, map[string]interface{}{"results": results})
}

func (s *MCPServer) toolGetStages(id json.RawMessage) *jsonRPCResponse {
	sourceHash, _ := computeSourceHash(s.root, s.config, s.ws)
	var stages []map[string]interface{}

	for name, stage := range s.config.Stages {
		cmdStr := strings.Join(stage.Cmd, " ")
		stageCacheKey := sourceHash + "|" + cmdStr
		cached := s.cache[name] == stageCacheKey

		stages = append(stages, map[string]interface{}{
			"name":    name,
			"command": cmdStr,
			"enabled": stage.Enabled,
			"cached":  cached,
		})
	}
	return s.toolResult(id, map[string]interface{}{"stages": stages})
}

func (s *MCPServer) toolGetStaleStages(id json.RawMessage) *jsonRPCResponse {
	sourceHash, _ := computeSourceHash(s.root, s.config, s.ws)
	var stale []map[string]interface{}

	for _, name := range s.config.GetEnabledStages() {
		stage := s.config.Stages[name]
		cmdStr := strings.Join(stage.Cmd, " ")
		stageCacheKey := sourceHash + "|" + cmdStr

		if s.cache[name] != stageCacheKey {
			reason := "hash_changed"
			if s.cache[name] == "" {
				reason = "not_cached"
			}
			stale = append(stale, map[string]interface{}{
				"name":   name,
				"reason": reason,
			})
		}
	}
	return s.toolResult(id, map[string]interface{}{"stale_stages": stale})
}

func (s *MCPServer) toolInvalidate(id json.RawMessage, args json.RawMessage) *jsonRPCResponse {
	var params struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return s.errorResponse(id, -32602, "Invalid arguments")
	}

	if _, ok := s.config.Stages[params.Name]; !ok {
		return s.errorResponse(id, -32602, "Unknown stage: "+params.Name)
	}

	delete(s.cache, params.Name)
	saveCache(s.cache, s.root)

	return s.toolResult(id, map[string]interface{}{
		"invalidated": params.Name,
	})
}

func (s *MCPServer) toolGetSourceHash(id json.RawMessage) *jsonRPCResponse {
	hash, err := computeSourceHash(s.root, s.config, s.ws)
	if err != nil {
		return s.toolResult(id, map[string]interface{}{
			"hash":  "",
			"error": err.Error(),
		})
	}
	return s.toolResult(id, map[string]interface{}{"hash": hash})
}

func (s *MCPServer) toolGetWorkspace(id json.RawMessage) *jsonRPCResponse {
	result := map[string]interface{}{
		"root": s.root,
	}
	if s.ws != nil {
		result["is_single"] = s.ws.IsSingle
		result["members"] = s.ws.Members
	}
	return s.toolResult(id, result)
}

func (s *MCPServer) executeStage(stage Stage) Result {
	stageStart := time.Now()
	cmdStr := strings.Join(stage.Cmd, " ")

	timeout := time.Duration(stage.Timeout) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, stage.Cmd[0], stage.Cmd[1:]...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	cmd.Dir = s.root

	err := cmd.Run()

	result := Result{
		Name:     stage.Name,
		Command:  cmdStr,
		Status:   "pass",
		Duration: time.Since(stageStart),
		Output:   out.String(),
	}
	if err != nil {
		result.Status = "fail"
		result.Error = err
	}
	return result
}

// Helper methods

func (s *MCPServer) toolResult(id json.RawMessage, data interface{}) *jsonRPCResponse {
	text, _ := json.Marshal(data)
	return &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: mcpToolResult{
			Content: []mcpContent{{Type: "text", Text: string(text)}},
		},
	}
}

func (s *MCPServer) errorResponse(id json.RawMessage, code int, message string) *jsonRPCResponse {
	return &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &rpcError{Code: code, Message: message},
	}
}
