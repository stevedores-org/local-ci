package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// PackageJSON represents relevant fields in package.json
type PackageJSON struct {
	Name       string            `json:"name"`
	Workspaces interface{}       `json:"workspaces"` // string[] or {packages: string[]}
	Scripts    map[string]string `json:"scripts"`
}

// TSWorkspace represents a TypeScript/JS workspace structure
type TSWorkspace struct {
	Root     string
	Members  []string
	IsSingle bool
}

// DetectTSWorkspace detects TypeScript workspace structure from package.json
func DetectTSWorkspace(root string) (*TSWorkspace, error) {
	pkgPath := filepath.Join(root, "package.json")
	data, err := os.ReadFile(pkgPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read package.json: %w", err)
	}

	var pkg PackageJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, fmt.Errorf("failed to parse package.json: %w", err)
	}

	ws := &TSWorkspace{Root: root}

	// Parse workspaces field (can be string[] or {packages: string[]})
	patterns := parseWorkspaces(pkg.Workspaces)
	if len(patterns) == 0 {
		ws.IsSingle = true
		ws.Members = []string{"."}
		return ws, nil
	}

	// Expand glob patterns
	expanded, err := expandGlobPatterns(root, patterns)
	if err != nil {
		return nil, fmt.Errorf("failed to expand workspace patterns: %w", err)
	}

	// Filter to directories that contain package.json
	var members []string
	for _, dir := range expanded {
		memberPkg := filepath.Join(root, dir, "package.json")
		if fileExists(memberPkg) {
			members = append(members, dir)
		}
	}

	if len(members) == 0 {
		ws.IsSingle = true
		ws.Members = []string{"."}
	} else {
		ws.Members = members
	}

	return ws, nil
}

// parseWorkspaces extracts workspace patterns from the workspaces field.
// Handles both string[] and {packages: string[]} formats.
func parseWorkspaces(raw interface{}) []string {
	if raw == nil {
		return nil
	}

	// Try string array (["packages/*", "apps/*"])
	if arr, ok := raw.([]interface{}); ok {
		var result []string
		for _, item := range arr {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}

	// Try object with packages field ({packages: ["packages/*"]})
	if obj, ok := raw.(map[string]interface{}); ok {
		if pkgs, ok := obj["packages"]; ok {
			return parseWorkspaces(pkgs) // recurse on the packages array
		}
	}

	return nil
}

// HasBun returns true if the bun runtime is available
func HasBun() bool {
	return ToolIsAvailable("bun")
}

// defaultTSStages returns the default stage definitions for TypeScript projects
func defaultTSStages() map[string]Stage {
	return map[string]Stage{
		"install": {
			Name:    "install",
			Cmd:     []string{"bun", "install", "--frozen-lockfile"},
			Timeout: 120,
			Enabled: true,
		},
		"typecheck": {
			Name:    "typecheck",
			Cmd:     []string{"bun", "run", "typecheck"},
			Timeout: 300,
			Enabled: true,
		},
		"lint": {
			Name:    "lint",
			Cmd:     []string{"bun", "run", "lint"},
			FixCmd:  []string{"bun", "run", "lint", "--fix"},
			Check:   true,
			Timeout: 300,
			Enabled: true,
		},
		"test": {
			Name:    "test",
			Cmd:     []string{"bun", "run", "test"},
			Timeout: 600,
			Enabled: true,
		},
		"build": {
			Name:    "build",
			Cmd:     []string{"bun", "run", "build"},
			Timeout: 600,
			Enabled: false,
		},
	}
}

// defaultTSCacheConfig returns cache config for TypeScript projects
func defaultTSCacheConfig() CacheConfig {
	return CacheConfig{
		SkipDirs:        []string{".git", "node_modules", "dist", ".next", ".turbo", ".claude"},
		IncludePatterns: []string{"*.ts", "*.tsx", "*.js", "*.jsx", "*.json"},
	}
}

// defaultTSConfigTemplate returns the TOML template for TS projects
func defaultTSConfigTemplate() string {
	return `# local-ci configuration (TypeScript / Bun)
# See: https://github.com/stevedores-org/local-ci

[cache]
skip_dirs = [".git", "node_modules", "dist", ".next", ".turbo", ".claude"]
include_patterns = ["*.ts", "*.tsx", "*.js", "*.jsx", "*.json"]

[stages.install]
command = ["bun", "install", "--frozen-lockfile"]
timeout = 120
enabled = true

[stages.typecheck]
command = ["bun", "run", "typecheck"]
timeout = 300
enabled = true

[stages.lint]
command = ["bun", "run", "lint"]
fix_command = ["bun", "run", "lint", "--fix"]
timeout = 300
enabled = true

[stages.test]
command = ["bun", "run", "test"]
timeout = 600
enabled = true

# Optional: build stage (disabled by default)
[stages.build]
command = ["bun", "run", "build"]
timeout = 600
enabled = false

[dependencies]
required = []
optional = []

[workspace]
exclude = []
`
}
