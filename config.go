package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"
)

// Config represents the .local-ci.toml configuration file
type Config struct {
	Cache        CacheConfig      `toml:"cache"`
	Stages       map[string]Stage `toml:"stages"`
	Dependencies DepsConfig       `toml:"dependencies"`
	Workspace    WorkspaceConfig  `toml:"workspace"`
}

// CacheConfig defines caching behavior
type CacheConfig struct {
	SkipDirs        []string `toml:"skip_dirs"`
	IncludePatterns []string `toml:"include_patterns"`
}

// StageConfig defines a CI stage
type StageConfig struct {
	Command                  []string `toml:"command"`
	FixCommand               []string `toml:"fix_command"`
	Timeout                  int      `toml:"timeout"` // seconds
	Enabled                  bool     `toml:"enabled"`
	RespectWorkspaceExcludes bool     `toml:"respect_workspace_excludes"`
}

// DepsConfig defines system dependencies
type DepsConfig struct {
	Required []string `toml:"required"`
	Optional []string `toml:"optional"`
}

// WorkspaceConfig defines workspace settings
type WorkspaceConfig struct {
	Exclude []string `toml:"exclude"`
}

// LoadConfig loads configuration from .local-ci.toml or returns defaults
func LoadConfig(root string) (*Config, error) {
	configPath := filepath.Join(root, ".local-ci.toml")

	cfg := &Config{
		Cache: CacheConfig{
			SkipDirs:        []string{".git", "target", ".github", "scripts", ".claude"},
			IncludePatterns: []string{"*.rs", "*.toml", "*.ts", "*.tsx", "*.js", "*.jsx", "*.mjs", "*.cjs", "*.json"},
		},
		Stages: defaultStages(),
		Dependencies: DepsConfig{
			Required: []string{},
			Optional: []string{},
		},
		Workspace: WorkspaceConfig{
			Exclude: []string{},
		},
	}

	// Try to load from file
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil // Return defaults if file doesn't exist
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse TOML
	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse .local-ci.toml: %w", err)
	}

	// Merge defaults for stages not specified
	defaults := defaultStages()
	for name, defaultStage := range defaults {
		if _, exists := cfg.Stages[name]; !exists {
			cfg.Stages[name] = defaultStage
		}
	}

	return cfg, nil
}

// SaveDefaultConfig writes a default .local-ci.toml file
func SaveDefaultConfig(root string, wsConfig *Workspace) error {
	configPath := filepath.Join(root, ".local-ci.toml")

	// Check if config already exists
	if _, err := os.Stat(configPath); err == nil {
		return fmt.Errorf("config file already exists at %s", configPath)
	}

	// Write default config as text (template-based)
	defaultConfig := `# local-ci configuration
# See: https://github.com/stevedores-org/local-ci
#
# Core stages run by default: fmt, clippy, test (Rust)
# Optional tool stages available below (requires installation):
#   - deny: cargo-deny (security/license checking)
#   - audit: cargo-audit (CVE vulnerability scanning)
#   - machete: cargo-machete (unused dependencies)
#   - taplo: taplo-cli (TOML formatting)
#   - bun-install/typecheck-ts/lint-ts/test-ts: Bun + TypeScript/JS workflows

[cache]
# Directories to skip when computing source hash
skip_dirs = [".git", "target", ".github", "scripts", ".claude", "node_modules"]
# File patterns to include in hash
include_patterns = ["*.rs", "*.toml", "*.ts", "*.tsx", "*.js", "*.jsx", "*.mjs", "*.cjs", "*.json"]

[stages.fmt]
command = ["cargo", "fmt", "--all", "--", "--check"]
fix_command = ["cargo", "fmt", "--all"]
timeout = 120
enabled = true

[stages.clippy]
command = ["cargo", "clippy", "--workspace", "--all-targets", "--", "-D", "warnings"]
timeout = 600
enabled = true

[stages.test]
command = ["cargo", "test", "--workspace"]
timeout = 1200
enabled = true

[stages.check]
command = ["cargo", "check", "--workspace"]
timeout = 600
enabled = false

# Optional: Cargo tools (uncomment to enable)
# Install with: cargo install cargo-deny
[stages.deny]
command = ["cargo", "deny", "check"]
timeout = 300
enabled = false

# Install with: cargo install cargo-audit
[stages.audit]
command = ["cargo", "audit"]
timeout = 300
enabled = false

# Install with: cargo install cargo-machete
[stages.machete]
command = ["cargo", "machete"]
timeout = 300
enabled = false

# Install with: cargo install taplo-cli
[stages.taplo]
command = ["taplo", "format", "--check", "."]
fix_command = ["taplo", "format", "."]
timeout = 300
enabled = false

# Optional: Bun + TypeScript/JavaScript stages (uncomment to enable)
# Install Bun with: brew install oven-sh/bun/bun
[stages.bun-install]
command = ["bun", "install", "--frozen-lockfile"]
timeout = 300
enabled = false

[stages.typecheck-ts]
command = ["bun", "x", "tsc", "--noEmit"]
timeout = 300
enabled = false

[stages.lint-ts]
command = ["bun", "run", "lint"]
timeout = 300
enabled = false

[stages.test-ts]
command = ["bun", "test"]
timeout = 600
enabled = false

[dependencies]
# System dependencies required (uncomment if needed)
# required = ["protoc", "clang"]
optional = []

[workspace]
# Workspace members to exclude (auto-detected from Cargo.toml)
exclude = []
`

	// Write to file
	if err := os.WriteFile(configPath, []byte(defaultConfig), 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// defaultStages returns the default stage definitions
func defaultStages() map[string]Stage {
	return map[string]Stage{
		"fmt": {
			Name:    "fmt",
			Cmd:     []string{"cargo", "fmt", "--all", "--", "--check"},
			FixCmd:  []string{"cargo", "fmt", "--all"},
			Check:   true,
			Timeout: 120,
			Enabled: true,
		},
		"clippy": {
			Name:    "clippy",
			Cmd:     []string{"cargo", "clippy", "--workspace", "--all-targets", "--", "-D", "warnings"},
			FixCmd:  nil,
			Check:   false,
			Timeout: 600,
			Enabled: true,
		},
		"test": {
			Name:    "test",
			Cmd:     []string{"cargo", "test", "--workspace"},
			FixCmd:  nil,
			Check:   false,
			Timeout: 1200,
			Enabled: true,
		},
		"check": {
			Name:    "check",
			Cmd:     []string{"cargo", "check", "--workspace"},
			FixCmd:  nil,
			Check:   false,
			Timeout: 600,
			Enabled: false, // Disabled by default, redundant with clippy
		},
		"deny": {
			Name:    "deny",
			Cmd:     []string{"cargo", "deny", "check"},
			FixCmd:  nil,
			Check:   false,
			Timeout: 300,
			Enabled: false, // Requires cargo-deny to be installed
		},
		"audit": {
			Name:    "audit",
			Cmd:     []string{"cargo", "audit"},
			FixCmd:  nil,
			Check:   false,
			Timeout: 300,
			Enabled: false, // Requires cargo-audit to be installed
		},
		"machete": {
			Name:    "machete",
			Cmd:     []string{"cargo", "machete"},
			FixCmd:  nil,
			Check:   false,
			Timeout: 300,
			Enabled: false, // Requires cargo-machete to be installed
		},
		"taplo": {
			Name:    "taplo",
			Cmd:     []string{"taplo", "format", "--check", "."},
			FixCmd:  []string{"taplo", "format", "."},
			Check:   true,
			Timeout: 300,
			Enabled: false, // Requires taplo to be installed
		},
		"bun-install": {
			Name:    "bun-install",
			Cmd:     []string{"bun", "install", "--frozen-lockfile"},
			FixCmd:  nil,
			Check:   false,
			Timeout: 300,
			Enabled: false,
		},
		"typecheck-ts": {
			Name:    "typecheck-ts",
			Cmd:     []string{"bun", "x", "tsc", "--noEmit"},
			FixCmd:  nil,
			Check:   true,
			Timeout: 300,
			Enabled: false,
		},
		"lint-ts": {
			Name:    "lint-ts",
			Cmd:     []string{"bun", "run", "lint"},
			FixCmd:  nil,
			Check:   false,
			Timeout: 300,
			Enabled: false,
		},
		"test-ts": {
			Name:    "test-ts",
			Cmd:     []string{"bun", "test"},
			FixCmd:  nil,
			Check:   false,
			Timeout: 600,
			Enabled: false,
		},
	}
}

// ToStageConfigs converts the config stages map to Stage structs
func (c *Config) ToStageConfigs() map[string]StageConfig {
	result := make(map[string]StageConfig)
	for name, stage := range c.Stages {
		result[name] = StageConfig{
			Command:                  stage.Cmd,
			FixCommand:               stage.FixCmd,
			Timeout:                  stage.Timeout,
			Enabled:                  stage.Enabled,
			RespectWorkspaceExcludes: false,
		}
	}
	return result
}

// GetTimeout returns the timeout for a stage, with fallback to default
func (c *Config) GetTimeout(stageName string) time.Duration {
	if stage, ok := c.Stages[stageName]; ok && stage.Timeout > 0 {
		return time.Duration(stage.Timeout) * time.Second
	}
	return 30 * time.Second // Safe default
}

// GetEnabledStages returns the list of enabled stage names
func (c *Config) GetEnabledStages() []string {
	var enabled []string
	for name, stage := range c.Stages {
		if stage.Enabled {
			enabled = append(enabled, name)
		}
	}
	return enabled
}
