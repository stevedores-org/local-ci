package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

// Profile represents a named collection of settings
type Profile struct {
	Stages   []string `toml:"stages"` // stage names to enable (overrides enabled)
	FailFast bool     `toml:"fail_fast"`
	NoCache  bool     `toml:"no_cache"`
	JSON     bool     `toml:"json"`
}

// Config represents the .local-ci.toml configuration file
type Config struct {
	SSHDefaults  RemoteSSHDefaults     `toml:"ssh_defaults"`
	Cache        CacheConfig           `toml:"cache"`
	Stages       map[string]Stage      `toml:"stages"`
	Dependencies DepsConfig            `toml:"dependencies"`
	Workspace    WorkspaceConfig       `toml:"workspace"`
	Profiles     map[string]Profile    `toml:"profiles"`
	Hosts        map[string]RemoteHost `toml:"hosts"`
}

// RemoteHost is a named SSH+tmux target loaded from .local-ci-remote.toml.
// Lets users say `--remote-host sparky` instead of repeating
// `--remote aivcs2@spark-bde7 --session sparky-onion --remote-dir /data/builds`.
type RemoteHost struct {
	// Tailscale name or full user@host. Bare names expand via [ssh_defaults].
	Host string `toml:"host"`
	// macos (default) | linux_spark — picks user from [ssh_defaults].
	Platform string `toml:"platform"`
	// tmux session name; if empty, the --session flag (or its default) wins.
	Session string `toml:"session"`
	// Remote working directory; if empty, falls back to --remote-dir or /tmp/<basename>.
	RemoteDir string `toml:"remote_dir"`
	// Human-readable note shown by tooling. Optional.
	Description string `toml:"description"`
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

// LoadConfig loads configuration from .local-ci.toml (and .local-ci-remote.toml if remote=true)
func LoadConfig(root string, remote bool) (*Config, error) {
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
		Stages: defaultStages,
		Dependencies: DepsConfig{
			Required: []string{},
			Optional: []string{},
		},
		Workspace: WorkspaceConfig{
			Exclude: []string{},
		},
		Profiles: make(map[string]Profile),
	}

	// Try to load from file
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Fall through to remote config loading or return defaults
		} else {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	} else {
		// Parse TOML
		if err := toml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("failed to parse .local-ci.toml: %w", err)
		}
	}

	// If --remote flag is set, load and merge .local-ci-remote.toml
	if remote {
		remoteConfigPath := filepath.Join(root, ".local-ci-remote.toml")
		remoteData, err := os.ReadFile(remoteConfigPath)
		if err != nil {
			if !os.IsNotExist(err) {
				return nil, fmt.Errorf("failed to read remote config file: %w", err)
			}
			// If remote config doesn't exist, that's okay - just continue with local config
		} else {
			// Parse and merge remote TOML
			remoteCfg := &Config{}
			if err := toml.Unmarshal(remoteData, remoteCfg); err != nil {
				return nil, fmt.Errorf("failed to parse .local-ci-remote.toml: %w", err)
			}

			// Merge remote stages (override local stages if specified in remote)
			for name, stage := range remoteCfg.Stages {
				cfg.Stages[name] = stage
			}

			// Merge remote cache config if specified
			if len(remoteCfg.Cache.SkipDirs) > 0 {
				cfg.Cache.SkipDirs = remoteCfg.Cache.SkipDirs
			}
			if len(remoteCfg.Cache.IncludePatterns) > 0 {
				cfg.Cache.IncludePatterns = remoteCfg.Cache.IncludePatterns
			}

			// Merge remote dependencies if specified
			if len(remoteCfg.Dependencies.Required) > 0 {
				cfg.Dependencies.Required = remoteCfg.Dependencies.Required
			}
			if len(remoteCfg.Dependencies.Optional) > 0 {
				cfg.Dependencies.Optional = remoteCfg.Dependencies.Optional
			}

			// Merge remote workspace config if specified
			if len(remoteCfg.Workspace.Exclude) > 0 {
				cfg.Workspace.Exclude = remoteCfg.Workspace.Exclude
			}

			// Carry over named host presets ([hosts.*]) verbatim. These only
			// live in the remote config file — they have no analogue in the
			// per-project local config.
			if remoteCfg.SSHDefaults.MacOSUser != "" || remoteCfg.SSHDefaults.LinuxSparkUser != "" || remoteCfg.SSHDefaults.WindowsUser != "" {
				cfg.SSHDefaults = remoteCfg.SSHDefaults
			}
			if len(remoteCfg.Hosts) > 0 {
				if cfg.Hosts == nil {
					cfg.Hosts = make(map[string]RemoteHost, len(remoteCfg.Hosts))
				}
				for name, h := range remoteCfg.Hosts {
					cfg.Hosts[name] = h
				}
			}
		}
	}

	// Merge defaults for stages not specified (but keep file values if set)
	for name, defaultStage := range defaultStages {
		if _, exists := cfg.Stages[name]; !exists {
			cfg.Stages[name] = defaultStage
		}
	}

	// Ensure Name field is set for all stages from the map key
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
			Name:      "fmt",
			Cmd:       []string{"cargo", "fmt", "--all", "--", "--check"},
			FixCmd:    []string{"cargo", "fmt", "--all"},
			Check:     true,
			Timeout:   120,
			Enabled:   true,
			DependsOn: []string{},
			Watch:     []string{"*.rs"},
		},
		"clippy": {
			Name:      "clippy",
			Cmd:       []string{"cargo", "clippy", "--workspace", "--all-targets", "--", "-D", "warnings"},
			FixCmd:    nil,
			Check:     false,
			Timeout:   600,
			Enabled:   true,
			DependsOn: []string{"fmt"},
			Watch:     []string{"*.rs", "Cargo.toml", "Cargo.lock"},
		},
		"test": {
			Name:      "test",
			Cmd:       []string{"cargo", "test", "--workspace"},
			FixCmd:    nil,
			Check:     false,
			Timeout:   1200,
			Enabled:   true,
			DependsOn: []string{"fmt"},
			Watch:     []string{"*.rs", "Cargo.toml", "Cargo.lock"},
		},
		"check": {
			Name:      "check",
			Cmd:       []string{"cargo", "check", "--workspace"},
			FixCmd:    nil,
			Check:     false,
			Timeout:   600,
			Enabled:   false,
			DependsOn: []string{},
			Watch:     []string{"*.rs", "Cargo.toml", "Cargo.lock"},
		},
		"deny": {
			Name:      "deny",
			Cmd:       []string{"cargo", "deny", "check"},
			FixCmd:    nil,
			Check:     false,
			Timeout:   300,
			Enabled:   false,
			DependsOn: []string{},
			Watch:     []string{"Cargo.toml", "Cargo.lock", "deny.toml"},
		},
		"audit": {
			Name:      "audit",
			Cmd:       []string{"cargo", "audit"},
			FixCmd:    nil,
			Check:     false,
			Timeout:   300,
			Enabled:   false,
			DependsOn: []string{},
			Watch:     []string{"Cargo.toml", "Cargo.lock"},
		},
		"machete": {
			Name:      "machete",
			Cmd:       []string{"cargo", "machete"},
			FixCmd:    nil,
			Check:     false,
			Timeout:   300,
			Enabled:   false,
			DependsOn: []string{},
			Watch:     []string{"*.rs", "Cargo.toml"},
		},
		"taplo": {
			Name:      "taplo",
			Cmd:       []string{"taplo", "format", "--check", "."},
			FixCmd:    []string{"taplo", "format", "."},
			Check:     true,
			Timeout:   300,
			Enabled:   false,
			DependsOn: []string{},
			Watch:     []string{"*.toml"},
		},
	}
}

// GetTimeout returns the timeout for a stage, with fallback to default
func (c *Config) GetTimeout(stageName string) time.Duration {
	if stage, ok := c.Stages[stageName]; ok && stage.Timeout > 0 {
		return time.Duration(stage.Timeout) * time.Second
	}
	return 30 * time.Second // Safe default
}

// GetEnabledStages returns the list of enabled stage names in deterministic order
func (c *Config) GetEnabledStages() []string {
	// Define default order for common stages to ensure deterministic output
	order := []string{"fmt", "check", "clippy", "test", "lint", "vet", "types", "build", "audit", "deny", "machete", "taplo"}

	var enabled []string
	// First add stages in predefined order if they exist and are enabled
	for _, name := range order {
		if stage, ok := c.Stages[name]; ok && stage.Enabled {
			enabled = append(enabled, name)
		}
	}

	// Then add any remaining enabled stages not in the predefined order, sorted alphabetically
	seen := make(map[string]bool)
	for _, name := range order {
		seen[name] = true
	}
	var extra []string
	for name, stage := range c.Stages {
		if !seen[name] && stage.Enabled {
			extra = append(extra, name)
		}
	}
	sort.Strings(extra)
	enabled = append(enabled, extra...)

	return enabled
}

// GetAllStages returns every configured stage name (enabled and disabled) in
// deterministic order — the same ordering as GetEnabledStages, but without the
// enabled filter. Used by the --all flag.
func (c *Config) GetAllStages() []string {
	order := []string{"fmt", "check", "clippy", "test", "lint", "vet", "types", "build", "audit", "deny", "machete", "taplo"}

	var all []string
	seen := make(map[string]bool)
	for _, name := range order {
		if _, ok := c.Stages[name]; ok {
			all = append(all, name)
			seen[name] = true
		}
	}

	var extra []string
	for name := range c.Stages {
		if !seen[name] {
			extra = append(extra, name)
		}
	}
	sort.Strings(extra)
	return append(all, extra...)
}

// GetRemoteHost looks up a named host preset (loaded from
// `.local-ci-remote.toml`) by name. Returns an actionable error when the
// name is unknown — listing the names that *are* defined, or saying that
// no presets exist at all — so users don't have to grep the config file.
func (c *Config) GetRemoteHost(name string) (*RemoteHost, error) {
	if len(c.Hosts) == 0 {
		return nil, fmt.Errorf(
			"no remote host presets defined; add `[hosts.%s]` to .local-ci-remote.toml",
			name,
		)
	}
	h, ok := c.Hosts[name]
	if !ok {
		available := make([]string, 0, len(c.Hosts))
		for n := range c.Hosts {
			available = append(available, n)
		}
		sort.Strings(available)
		return nil, fmt.Errorf(
			"remote host preset %q not found; available: %s",
			name,
			strings.Join(available, ", "),
		)
	}
	h = c.normalizeRemoteHost(name, h)
	if strings.TrimSpace(h.Host) == "" {
		return nil, fmt.Errorf(
			"remote host preset %q has empty `host` field in .local-ci-remote.toml",
			name,
		)
	}
	return &h, nil
}

// ListRemoteHosts returns named presets sorted for display.
func (c *Config) ListRemoteHosts() []struct {
	Name        string
	Host        string
	Session     string
	RemoteDir   string
	Description string
} {
	if len(c.Hosts) == 0 {
		return nil
	}
	names := make([]string, 0, len(c.Hosts))
	for n := range c.Hosts {
		names = append(names, n)
	}
	sort.Strings(names)
	out := make([]struct {
		Name        string
		Host        string
		Session     string
		RemoteDir   string
		Description string
	}, 0, len(names))
	for _, n := range names {
		h := c.normalizeRemoteHost(n, c.Hosts[n])
		out = append(out, struct {
			Name        string
			Host        string
			Session     string
			RemoteDir   string
			Description string
		}{n, h.Host, h.Session, h.RemoteDir, h.Description})
	}
	return out
}

// ResolvedRemoteTarget is the result of merging a `[hosts.<name>]` preset
// with command-line overrides. Empty fields mean "use the caller's default".
type ResolvedRemoteTarget struct {
	Host      string
	Session   string
	RemoteDir string
}

// ResolveRemoteHost applies a named preset onto flag-derived defaults using
// flag > preset > default precedence. Each `userSet*` argument indicates
// whether the CLI flag was explicitly passed (vs. left at its default), so
// that an explicit `--session onion` still beats a preset with a non-default
// session value.
//
// Pulled out of main() so it can be unit-tested.
func (c *Config) ResolveRemoteHost(
	name, hostFlag, sessionFlag, remoteDirFlag string,
	userSetSession, userSetRemoteDir bool,
) (ResolvedRemoteTarget, error) {
	preset, err := c.GetRemoteHost(name)
	if err != nil {
		return ResolvedRemoteTarget{}, err
	}
	out := ResolvedRemoteTarget{
		Host:      hostFlag,
		Session:   sessionFlag,
		RemoteDir: remoteDirFlag,
	}
	if out.Host == "" {
		out.Host = preset.Host
	} else if !strings.Contains(out.Host, "@") {
		out.Host = NormalizeSSHHost(out.Host, preset.effectivePlatform(name), c.SSHDefaults)
	}
	if !userSetSession && preset.Session != "" {
		out.Session = preset.Session
	}
	if !userSetRemoteDir && preset.RemoteDir != "" {
		out.RemoteDir = preset.RemoteDir
	}
	return out, nil
}

// GetProfile returns a profile by name, validating that all referenced stages exist.
func (c *Config) GetProfile(name string) (*Profile, error) {
	p, ok := c.Profiles[name]
	if !ok {
		return nil, fmt.Errorf("profile %q not found", name)
	}
	for _, stageName := range p.Stages {
		if _, exists := c.Stages[stageName]; !exists {
			return nil, fmt.Errorf("profile %q references unknown stage %q", name, stageName)
		}
	}
	return &p, nil
}

// GetProfileStages returns the stages for a profile in deterministic order.
// If the profile has no stages, falls back to enabled stages.
func (c *Config) GetProfileStages(p *Profile) []string {
	if len(p.Stages) == 0 {
		return c.GetEnabledStages()
	}

	// Use the same ordering as GetEnabledStages
	order := []string{"fmt", "check", "clippy", "test", "lint", "vet", "types", "build", "audit", "deny", "machete", "taplo"}
	inProfile := make(map[string]bool)
	for _, s := range p.Stages {
		inProfile[s] = true
	}

	var result []string
	seen := make(map[string]bool)
	for _, name := range order {
		if inProfile[name] {
			result = append(result, name)
			seen[name] = true
		}
	}
	for _, name := range p.Stages {
		if !seen[name] {
			result = append(result, name)
		}
	}
	return result
}
