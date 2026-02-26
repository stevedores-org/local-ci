package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/BurntSushi/toml"
)

// stagePriority defines the canonical execution order.
// Fast checks run first for best --fail-fast behavior.
// Stages not in this list sort alphabetically after known stages.
var stagePriority = map[string]int{
	"fmt":     0,
	"clippy":  1,
	"check":   2,
	"test":    3,
	"deny":    4,
	"audit":   5,
	"machete": 6,
	"taplo":   7,
	// TypeScript stages
	"lint":      0,
	"typecheck": 1,
	"build":     5,
}

// Profile defines a named set of overrides for stage selection and flags.
type Profile struct {
	Stages   []string `toml:"stages"`
	FailFast bool     `toml:"fail_fast"`
	NoCache  bool     `toml:"no_cache"`
	JSON     bool     `toml:"json"`
}

// Config represents the .local-ci.toml configuration file
type Config struct {
	Cache        CacheConfig        `toml:"cache"`
	Stages       map[string]Stage   `toml:"stages"`
	Profiles     map[string]Profile `toml:"profiles"`
	Dependencies DepsConfig         `toml:"dependencies"`
	Workspace    WorkspaceConfig    `toml:"workspace"`
}

// CacheConfig defines caching behavior
type CacheConfig struct {
	SkipDirs        []string `toml:"skip_dirs"`
	IncludePatterns []string `toml:"include_patterns"`
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

// LoadConfig loads configuration from .local-ci.toml or returns defaults.
// Auto-detects project type for language-specific defaults when no config file exists.
func LoadConfig(root string) (*Config, error) {
	configPath := filepath.Join(root, ".local-ci.toml")

	// Detect project type for smart defaults
	projectType := DetectProjectType(root)
	defaultStages := GetDefaultStagesForType(projectType)
	cachePatterns := GetCachePatternForType(projectType)
	skipDirs := GetSkipDirsForType(projectType)

	cfg := &Config{
		Cache: CacheConfig{
			SkipDirs:        skipDirs,
			IncludePatterns: cachePatterns,
		},
		Stages:   defaultStages,
		Profiles: map[string]Profile{},
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

	// Set stage names from map keys (Name is toml:"-")
	for name, stage := range cfg.Stages {
		stage.Name = name
		cfg.Stages[name] = stage
	}

	// Merge defaults for stages not specified (but keep file values if set)
	for name, defaultStage := range defaultStages {
		if _, exists := cfg.Stages[name]; !exists {
			cfg.Stages[name] = defaultStage
		}
	}

	// Populate Name field from map key (toml:"-" means it's not deserialized)
	for name, stage := range cfg.Stages {
		stage.Name = name
		cfg.Stages[name] = stage
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

	// Detect project type and get appropriate template
	projectType := DetectProjectType(root)
	defaultConfig := GetConfigTemplateForType(projectType)

	// Write to file
	if err := os.WriteFile(configPath, []byte(defaultConfig), 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	fmt.Printf("Generated .local-ci.toml for %s project\n", projectType)
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
	}
}

// GetProfile returns the named profile or an error if it doesn't exist.
func (c *Config) GetProfile(name string) (*Profile, error) {
	p, ok := c.Profiles[name]
	if !ok {
		available := make([]string, 0, len(c.Profiles))
		for k := range c.Profiles {
			available = append(available, k)
		}
		sort.Strings(available)
		return nil, fmt.Errorf("unknown profile %q (available: %v)", name, available)
	}
	// Validate that all referenced stages exist
	for _, s := range p.Stages {
		if _, ok := c.Stages[s]; !ok {
			return nil, fmt.Errorf("profile %q references unknown stage %q", name, s)
		}
	}
	return &p, nil
}

// GetProfileStages returns the stages for a profile in priority order.
func (c *Config) GetProfileStages(profile *Profile) []string {
	if len(profile.Stages) == 0 {
		return c.GetEnabledStages()
	}
	// Return profile stages in canonical priority order
	stages := make([]string, len(profile.Stages))
	copy(stages, profile.Stages)
	sort.Slice(stages, func(i, j int) bool {
		pi, oki := stagePriority[stages[i]]
		pj, okj := stagePriority[stages[j]]
		if oki && okj {
			return pi < pj
		}
		if oki {
			return true
		}
		if okj {
			return false
		}
		return stages[i] < stages[j]
	})
	return stages
}

// GetTimeout returns the timeout for a stage, with fallback to default
func (c *Config) GetTimeout(stageName string) time.Duration {
	if stage, ok := c.Stages[stageName]; ok && stage.Timeout > 0 {
		return time.Duration(stage.Timeout) * time.Second
	}
	return 30 * time.Second // Safe default
}

// GetEnabledStages returns the list of enabled stage names in deterministic
// priority order (fast checks first). Stages without an explicit priority
// sort alphabetically after known stages.
func (c *Config) GetEnabledStages() []string {
	var enabled []string
	for name, stage := range c.Stages {
		if stage.Enabled {
			enabled = append(enabled, name)
		}
	}
	sort.Slice(enabled, func(i, j int) bool {
		pi, oki := stagePriority[enabled[i]]
		pj, okj := stagePriority[enabled[j]]
		if oki && okj {
			return pi < pj
		}
		if oki {
			return true // known stages come first
		}
		if okj {
			return false
		}
		return enabled[i] < enabled[j] // alphabetical fallback
	})
	return enabled
}
