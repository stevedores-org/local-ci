package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
)

// PackageJSON represents the fields we care about from package.json.
type PackageJSON struct {
	Name       string            `json:"name"`
	Workspaces []string          `json:"workspaces"`
	Scripts    map[string]string `json:"scripts"`
}

// DetectTypeScriptWorkspace reads package.json and resolves workspace members.
// Returns a Workspace compatible with the existing Rust workspace type.
func DetectTypeScriptWorkspace(root string) (*Workspace, error) {
	data, err := os.ReadFile(filepath.Join(root, "package.json"))
	if err != nil {
		return nil, err
	}

	var pkg PackageJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, err
	}

	ws := &Workspace{
		Root: root,
	}

	if len(pkg.Workspaces) == 0 {
		ws.IsSingle = true
		name := pkg.Name
		if name == "" {
			name = filepath.Base(root)
		}
		ws.Members = []string{name}
		return ws, nil
	}

	// Expand workspace globs
	for _, pattern := range pkg.Workspaces {
		absPattern := filepath.Join(root, pattern)
		matches, err := filepath.Glob(absPattern)
		if err != nil {
			continue
		}
		for _, m := range matches {
			if info, err := os.Stat(m); err == nil && info.IsDir() {
				if fileExistsAt(filepath.Join(m, "package.json")) {
					rel, _ := filepath.Rel(root, m)
					ws.Members = append(ws.Members, rel)
				}
			}
		}
	}

	sort.Strings(ws.Members)
	return ws, nil
}

// defaultTypeScriptStages returns the built-in TS stage definitions.
// Commands delegate to package.json scripts (bun run <script>) where possible,
// so projects can use eslint, biome, prettier, or any other tool.
func defaultTypeScriptStages() map[string]Stage {
	return map[string]Stage{
		"typecheck": {
			Name:    "typecheck",
			Cmd:     []string{"bun", "x", "tsc", "--noEmit"},
			FixCmd:  nil,
			Check:   false,
			Timeout: 120,
			Enabled: true,
		},
		"lint": {
			Name:    "lint",
			Cmd:     []string{"bun", "run", "lint"},
			FixCmd:  []string{"bun", "run", "lint", "--", "--fix"},
			Check:   false,
			Timeout: 300,
			Enabled: true,
		},
		"test": {
			Name:    "test",
			Cmd:     []string{"bun", "test"},
			FixCmd:  nil,
			Check:   false,
			Timeout: 600,
			Enabled: true,
		},
		"format": {
			Name:    "format",
			Cmd:     []string{"bun", "run", "format", "--check"},
			FixCmd:  []string{"bun", "run", "format"},
			Check:   true,
			Timeout: 120,
			Enabled: false, // disabled by default, optional
		},
	}
}

// defaultTSCacheConfig returns the default cache settings for TS projects.
func defaultTSCacheConfig() CacheConfig {
	return CacheConfig{
		SkipDirs:        []string{".git", "node_modules", "dist", ".next", "coverage", ".claude"},
		IncludePatterns: []string{"*.ts", "*.tsx", "*.js", "*.jsx", "*.json"},
	}
}

// defaultTypeScriptConfig returns a full Config for TypeScript projects.
func defaultTypeScriptConfig() *Config {
	return &Config{
		Cache:  defaultTSCacheConfig(),
		Stages: defaultTypeScriptStages(),
		Dependencies: DepsConfig{
			Required: []string{},
			Optional: []string{},
		},
		Workspace: WorkspaceConfig{
			Exclude: []string{},
		},
	}
}

// SaveDefaultTypeScriptConfig writes a .local-ci.toml for TypeScript/Bun projects.
func SaveDefaultTypeScriptConfig(root string) error {
	configPath := filepath.Join(root, ".local-ci.toml")

	if _, err := os.Stat(configPath); err == nil {
		return nil // Don't overwrite existing config
	}

	content := `# local-ci configuration for TypeScript/Bun projects
# See: https://github.com/stevedores-org/local-ci
# Runtime: bun
#
# Stages delegate to package.json scripts where possible.
# Customize the "lint" and "format" scripts in your package.json
# to use eslint, biome, prettier, or any other tool.

[cache]
# Directories to skip when computing source hash
skip_dirs = [".git", "node_modules", "dist", ".next", "coverage", ".claude"]
# File patterns to include in hash
include_patterns = ["*.ts", "*.tsx", "*.js", "*.jsx", "*.json"]

[stages.typecheck]
# Uses bun x to auto-resolve tsc (requires typescript as a dependency)
command = ["bun", "x", "tsc", "--noEmit"]
timeout = 120
enabled = true

[stages.lint]
# Delegates to package.json "lint" script
command = ["bun", "run", "lint"]
fix_command = ["bun", "run", "lint", "--", "--fix"]
timeout = 300
enabled = true

[stages.test]
command = ["bun", "test"]
timeout = 600
enabled = true

[stages.format]
# Delegates to package.json "format" script
command = ["bun", "run", "format", "--check"]
fix_command = ["bun", "run", "format"]
timeout = 120
enabled = false

[dependencies]
required = []
optional = []

[workspace]
exclude = []
`
	return os.WriteFile(configPath, []byte(content), 0644)
}
