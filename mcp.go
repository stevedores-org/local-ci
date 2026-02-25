package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// MCPTool represents a single MCP tool definition
type MCPTool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// MCPRequest represents an MCP tool call request
type MCPRequest struct {
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params"`
}

// MCPResponse represents an MCP tool call response
type MCPResponse struct {
	Content []map[string]interface{} `json:"content"`
}

// MCPServer provides agent-native access to local-ci via MCP protocol
type MCPServer struct {
	config    *Config
	workspace *Workspace
	cwd       string
}

// NewMCPServer creates a new MCP server instance
func NewMCPServer(cwd string) (*MCPServer, error) {
	config, err := LoadConfig(cwd, false)
	if err != nil {
		return nil, err
	}

	ws, _ := DetectWorkspace(cwd)

	return &MCPServer{
		config:    config,
		workspace: ws,
		cwd:       cwd,
	}, nil
}

// GetTools returns the list of available MCP tools
func (s *MCPServer) GetTools() []MCPTool {
	return []MCPTool{
		{
			Name:        "run_stage",
			Description: "Run a single CI stage and return its result",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"stage": map[string]interface{}{
						"type":        "string",
						"description": "Name of the stage to run",
					},
				},
				"required": []string{"stage"},
			},
		},
		{
			Name:        "run_all",
			Description: "Run all enabled stages and return complete pipeline report",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"no_cache": map[string]interface{}{
						"type":        "boolean",
						"description": "Disable caching for this run",
					},
				},
			},
		},
		{
			Name:        "get_stages",
			Description: "List all available stages with cache status",
			InputSchema: map[string]interface{}{
				"type": "object",
			},
		},
		{
			Name:        "get_stale_stages",
			Description: "Get stages that have cache misses and their reasons",
			InputSchema: map[string]interface{}{
				"type": "object",
			},
		},
		{
			Name:        "invalidate",
			Description: "Invalidate cache for a specific stage",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"stage": map[string]interface{}{
						"type":        "string",
						"description": "Name of the stage to invalidate",
					},
				},
				"required": []string{"stage"},
			},
		},
		{
			Name:        "get_source_hash",
			Description: "Get the current source hash for cache invalidation checks",
			InputSchema: map[string]interface{}{
				"type": "object",
			},
		},
	}
}

// HandleToolCall processes a tool call request
func (s *MCPServer) HandleToolCall(toolName string, params map[string]interface{}) (string, error) {
	switch toolName {
	case "run_stage":
		return s.runStage(params)
	case "run_all":
		return s.runAll(params)
	case "get_stages":
		return s.getStages(params)
	case "get_stale_stages":
		return s.getStaleStagess(params)
	case "invalidate":
		return s.invalidateCache(params)
	case "get_source_hash":
		return s.getSourceHash(params)
	default:
		return "", fmt.Errorf("unknown tool: %s", toolName)
	}
}

// runStage runs a single stage
func (s *MCPServer) runStage(params map[string]interface{}) (string, error) {
	stageName, ok := params["stage"].(string)
	if !ok {
		return "", fmt.Errorf("invalid stage parameter")
	}

	stage, ok := s.config.Stages[stageName]
	if !ok {
		return "", fmt.Errorf("stage not found: %s", stageName)
	}

	// Run the stage
	runner := &ParallelRunner{
		Stages:      []Stage{stage},
		Concurrency: 1,
		Cwd:         s.cwd,
		NoCache:     false,
		Cache:       make(map[string]string),
		SourceHash:  "",
		Verbose:     true,
		JSON:        true,
		FailFast:    false,
	}

	results := runner.Run()
	if len(results) > 0 {
		data, _ := json.MarshalIndent(results[0], "", "  ")
		return string(data), nil
	}
	return "{}", nil
}

// runAll runs all enabled stages
func (s *MCPServer) runAll(params map[string]interface{}) (string, error) {
	noCache := false
	if nc, ok := params["no_cache"].(bool); ok {
		noCache = nc
	}

	var stages []Stage
	for _, name := range s.config.GetEnabledStages() {
		stages = append(stages, s.config.Stages[name])
	}

	// Compute source hash
	sourceHash, _ := computeSourceHash(s.cwd, s.config, s.workspace)

	cache := make(map[string]string)
	if !noCache {
		cache, _ = loadCache(s.cwd)
	}

	runner := &ParallelRunner{
		Stages:      stages,
		Concurrency: 1,
		Cwd:         s.cwd,
		NoCache:     noCache,
		Cache:       cache,
		SourceHash:  sourceHash,
		Verbose:     true,
		JSON:        true,
		FailFast:    false,
	}

	results := runner.Run()
	data, _ := json.MarshalIndent(results, "", "  ")
	return string(data), nil
}

// getStages returns list of all stages with cache status
func (s *MCPServer) getStages(params map[string]interface{}) (string, error) {
	cache, _ := loadCache(s.cwd)
	if cache == nil {
		cache = make(map[string]string)
	}

	sourceHash, _ := computeSourceHash(s.cwd, s.config, s.workspace)

	type StageInfo struct {
		Name     string `json:"name"`
		Enabled  bool   `json:"enabled"`
		Cached   bool   `json:"cached"`
		Command  string `json:"command"`
		DependsOn []string `json:"depends_on"`
	}

	var infos []StageInfo
	for _, stage := range s.config.Stages {
		infos = append(infos, StageInfo{
			Name:      stage.Name,
			Enabled:   stage.Enabled,
			Cached:    cache[stage.Name] == sourceHash,
			Command:   strings.Join(stage.Cmd, " "),
			DependsOn: stage.DependsOn,
		})
	}

	data, _ := json.MarshalIndent(infos, "", "  ")
	return string(data), nil
}

// getStaleStagess returns stages with cache misses
func (s *MCPServer) getStaleStagess(params map[string]interface{}) (string, error) {
	cache, _ := loadCache(s.cwd)
	if cache == nil {
		cache = make(map[string]string)
	}

	sourceHash, _ := computeSourceHash(s.cwd, s.config, s.workspace)

	type StaleInfo struct {
		Name   string `json:"name"`
		Reason string `json:"reason"`
	}

	var stales []StaleInfo
	for _, stage := range s.config.Stages {
		if !stage.Enabled {
			stales = append(stales, StaleInfo{
				Name:   stage.Name,
				Reason: "disabled",
			})
		} else if cache[stage.Name] != sourceHash {
			stales = append(stales, StaleInfo{
				Name:   stage.Name,
				Reason: "hash_changed",
			})
		}
	}

	data, _ := json.MarshalIndent(stales, "", "  ")
	return string(data), nil
}

// invalidateCache removes a stage from cache
func (s *MCPServer) invalidateCache(params map[string]interface{}) (string, error) {
	stageName, ok := params["stage"].(string)
	if !ok {
		return "", fmt.Errorf("invalid stage parameter")
	}

	cache, _ := loadCache(s.cwd)
	if cache == nil {
		cache = make(map[string]string)
	}

	delete(cache, stageName)
	_ = saveCache(cache, s.cwd)

	return fmt.Sprintf(`{"status": "invalidated", "stage": "%s"}`, stageName), nil
}

// getSourceHash returns the current source hash
func (s *MCPServer) getSourceHash(params map[string]interface{}) (string, error) {
	sourceHash, err := computeSourceHash(s.cwd, s.config, s.workspace)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf(`{"hash": "%s"}`, sourceHash), nil
}

// cmdServe implements the "local-ci serve" subcommand
func cmdServe(cwd string) {
	server, err := NewMCPServer(cwd)
	if err != nil {
		fatalf("Failed to initialize MCP server: %v", err)
	}

	printf("🚀 Starting MCP server on stdio...\n")
	printf("📋 Available tools:\n")
	for _, tool := range server.GetTools() {
		printf("  - %s: %s\n", tool.Name, tool.Description)
	}
	printf("\n")

	// Simple stdio-based MCP server
	// This reads JSON requests from stdin and writes JSON responses to stdout
	decoder := json.NewDecoder(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)

	for {
		var req MCPRequest
		err := decoder.Decode(&req)
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			fmt.Fprintf(os.Stderr, "Error decoding request: %v\n", err)
			continue
		}

		// Handle different MCP methods
		if req.Method == "tools/list" {
			tools := server.GetTools()
			response := map[string]interface{}{
				"tools": tools,
			}
			encoder.Encode(response)
		} else if req.Method == "tools/call" {
			toolName, _ := req.Params["name"].(string)
			toolParams, _ := req.Params["arguments"].(map[string]interface{})

			result, err := server.HandleToolCall(toolName, toolParams)
			if err != nil {
				encoder.Encode(map[string]interface{}{
					"error": err.Error(),
				})
			} else {
				// Parse result as JSON if possible
				var resultObj interface{}
				_ = json.Unmarshal([]byte(result), &resultObj)
				if resultObj == nil {
					resultObj = result
				}

				encoder.Encode(map[string]interface{}{
					"content": []map[string]interface{}{
						{
							"type": "text",
							"text": result,
						},
					},
				})
			}
		}
	}
}
