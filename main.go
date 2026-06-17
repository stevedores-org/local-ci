// local-ci — Universal local CI runner for any project type.
//
// Provides a fast, cacheable local CI pipeline that mirrors GitHub Actions
// for Rust, Python, TypeScript/Bun, Go, Java, and other projects.
// Auto-detects project type and applies language-specific defaults.
//
// Supports file-hash caching, configuration files, and colored output.
//
// Usage:
//
//	local-ci                Run default stages for detected project type
//	local-ci fmt clippy     Run specific stages
//	local-ci init           Initialize .local-ci.toml in current project
//	local-ci --no-cache     Disable caching, force all stages
//	local-ci --fix          Auto-fix issues
//	local-ci --verbose      Show detailed output
//	local-ci --list         List available stages
//	local-ci --version      Print version
//
// Supported project types:
//   - Rust (Cargo.toml)
//   - Python (pyproject.toml, setup.py, requirements.txt)
//   - TypeScript/Bun (package.json)
//   - Go (go.mod)
//   - Java (pom.xml, build.gradle)
//   - Generic (custom commands via .local-ci.toml)
package main

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var (
	version  = "0.3.0" // Universal project support (Rust, Python, TypeScript, Go, Java, etc.)
	flagJSON *bool
)

type Stage struct {
	Name      string
	Cmd       []string
	FixCmd    []string // command to run with --fix flag
	Check     bool     // true if this is a --check command
	Timeout   int      // in seconds
	Enabled   bool
	DependsOn []string `toml:"depends_on"` // stage names this stage depends on
	Watch     []string // file patterns this stage cares about (for granular caching)
}

func (s *Stage) UnmarshalTOML(data interface{}) error {
	m, ok := data.(map[string]interface{})
	if !ok {
		return nil
	}

	// Helper to extract string slice
	getStringSlice := func(key string) []string {
		if val, exists := m[key]; exists {
			if list, ok := val.([]interface{}); ok {
				var res []string
				for _, item := range list {
					if str, ok := item.(string); ok {
						res = append(res, str)
					}
				}
				return res
			}
		}
		return nil
	}

	// Helper to extract bool
	getBool := func(key string) bool {
		if val, exists := m[key]; exists {
			if b, ok := val.(bool); ok {
				return b
			}
		}
		return false
	}

	// Helper to extract int
	getInt := func(key string) int {
		if val, exists := m[key]; exists {
			switch v := val.(type) {
			case int:
				return v
			case int64:
				return int(v)
			case float64:
				return int(v)
			}
		}
		return 0
	}

	// Helper to extract string
	getString := func(key string) string {
		if val, exists := m[key]; exists {
			if str, ok := val.(string); ok {
				return str
			}
		}
		return ""
	}

	if name := getString("name"); name != "" {
		s.Name = name
	}

	s.Cmd = getStringSlice("command")
	if len(s.Cmd) == 0 {
		s.Cmd = getStringSlice("cmd")
	}
	s.FixCmd = getStringSlice("fix_command")
	if len(s.FixCmd) == 0 {
		s.FixCmd = getStringSlice("fix_cmd")
	}
	s.Check = getBool("check")
	s.Timeout = getInt("timeout")
	if val, exists := m["enabled"]; exists {
		if b, ok := val.(bool); ok {
			s.Enabled = b
		}
	}
	s.DependsOn = getStringSlice("depends_on")
	s.Watch = getStringSlice("watch")

	return nil
}

type Result struct {
	Name     string
	Command  string
	Status   string
	Duration time.Duration
	Output   string
	CacheHit bool
	Error    error
}

// ResultJSON is the JSON-serializable form of Result.
type ResultJSON struct {
	Name       string `json:"name"`
	Command    string `json:"command"`
	Status     string `json:"status"`
	DurationMS int64  `json:"duration_ms"`
	CacheHit   bool   `json:"cache_hit"`
	Output     string `json:"output,omitempty"`
	Error      string `json:"error,omitempty"`
}

// PipelineReportJSON is the JSON-serializable execution report of the pipeline.
type PipelineReportJSON struct {
	Results    []ResultJSON `json:"results"`
	Passed     int          `json:"passed"`
	Failed     int          `json:"failed"`
	DurationMS int64        `json:"duration_ms"`
}

// toJSONResults converts a slice of Result to a slice of ResultJSON.
func toJSONResults(results []Result) []ResultJSON {
	out := make([]ResultJSON, 0, len(results))
	for _, r := range results {
		jr := ResultJSON{
			Name:       r.Name,
			Command:    r.Command,
			Status:     r.Status,
			DurationMS: r.Duration.Milliseconds(),
			CacheHit:   r.CacheHit,
			Output:     strings.TrimSpace(r.Output),
		}
		if r.Error != nil {
			jr.Error = r.Error.Error()
		}
		out = append(out, jr)
	}
	return out
}

func main() {
	var (
		flagNoCache         = flag.Bool("no-cache", false, "Disable file hash cache")
		flagVerbose         = flag.Bool("verbose", false, "Show detailed output")
		flagFix             = flag.Bool("fix", false, "Auto-fix issues (cargo fmt)")
		flagList            = flag.Bool("list", false, "List available stages")
		flagListRemoteHosts = flag.Bool("list-remote-hosts", false, "List named remote host presets from .local-ci-remote.toml")
		flagVersion         = flag.Bool("version", false, "Print version")
		flagAll             = flag.Bool("all", false, "Run all stages including disabled ones")
		flagRemote          = flag.String("remote", "", "Run remotely on specified SSH host (e.g., user@host)")
		flagRemoteHost      = flag.String("remote-host", "", "Run remotely using a named preset from .local-ci-remote.toml (`[hosts.<name>]`)")
		flagSession         = flag.String("session", "onion", "tmux session name for remote execution")
		flagRemoteTimeout   = flag.Int("remote-timeout", 30, "SSH operation timeout in seconds")
		flagRemoteDir       = flag.String("remote-dir", "", "Remote working directory (defaults to /tmp/<basename>)")
		flagProfile         = flag.String("profile", "", "Use a named profile from config")
		flagDryRun          = flag.Bool("dry-run", false, "Show what would run without executing")
		flagParallel        = flag.Int("parallel", 0, "Number of parallel jobs (0 = auto)")
		flagFailFast        = flag.Bool("fail-fast", false, "Stop on first failure")
	)
	flagJSON = flag.Bool("json", false, "Output in JSON format")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "local-ci v%s — Universal local CI for any project\n\n", version)
		fmt.Fprintf(os.Stderr, "Supports: Rust, Python, TypeScript/Bun, Go, Java, and custom projects\n\n")
		fmt.Fprintf(os.Stderr, "Usage: local-ci [flags] [stages...]\n\n")
		fmt.Fprintf(os.Stderr, "Commands:\n")
		fmt.Fprintf(os.Stderr, "  init      Initialize .local-ci.toml for detected project type\n\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  local-ci              Run enabled stages for your project\n")
		fmt.Fprintf(os.Stderr, "  local-ci test         Run only the test stage\n")
		fmt.Fprintf(os.Stderr, "  local-ci --fix        Auto-fix format/lint issues\n")
		fmt.Fprintf(os.Stderr, "  local-ci --profile ci Run stages from 'ci' profile\n")
		fmt.Fprintf(os.Stderr, "  local-ci --dry-run    Show what would run\n")
		fmt.Fprintf(os.Stderr, "  local-ci --list       List all available stages\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *flagVersion {
		fmt.Printf("local-ci v%s\n", version)
		return
	}

	cwd, err := os.Getwd()
	if err != nil {
		fatalf("Cannot get working directory: %v", err)
	}

	// Handle subcommands
	args := flag.Args()
	if len(args) > 0 {
		if args[0] == "init" {
			cmdInit(cwd)
			return
		} else if args[0] == "serve" {
			if err := cmdServe(cwd); err != nil {
				fatalf("MCP server error: %v", err)
			}
			return
		}
	}

	// Load configuration. Pull the remote overlay whenever remote mode or
	// preset listing is requested — `--remote-host` resolves [hosts.*].
	needRemoteCfg := *flagRemote != "" || *flagRemoteHost != "" || *flagListRemoteHosts
	config, err := LoadConfig(cwd, needRemoteCfg)
	if err != nil {
		fatalf("Failed to load config: %v", err)
	}

	if *flagListRemoteHosts {
		hosts := config.ListRemoteHosts()
		if len(hosts) == 0 {
			fmt.Println("No remote host presets defined. Add [hosts.<name>] to .local-ci-remote.toml")
			return
		}
		for _, h := range hosts {
			line := fmt.Sprintf("%s  host=%s", h.Name, h.Host)
			if h.Session != "" {
				line += fmt.Sprintf("  session=%s", h.Session)
			}
			if h.RemoteDir != "" {
				line += fmt.Sprintf("  remote_dir=%s", h.RemoteDir)
			}
			if h.Description != "" {
				line += fmt.Sprintf("  # %s", h.Description)
			}
			fmt.Println(line)
		}
		return
	}

	// Resolve `--remote-host <name>` into the underlying ssh/session/dir
	// values. Explicit CLI flags always win over preset fields, so e.g.
	// `--remote-host aivcs2 --session experiment` reuses the preset's host
	// but overrides its session for this one run.
	if *flagRemoteHost != "" {
		userSetSession := false
		userSetRemoteDir := false
		flag.Visit(func(f *flag.Flag) {
			switch f.Name {
			case "session":
				userSetSession = true
			case "remote-dir":
				userSetRemoteDir = true
			}
		})
		resolved, err := config.ResolveRemoteHost(
			*flagRemoteHost,
			*flagRemote, *flagSession, *flagRemoteDir,
			userSetSession, userSetRemoteDir,
		)
		if err != nil {
			fatalf("%v", err)
		}
		*flagRemote = resolved.Host
		*flagSession = resolved.Session
		*flagRemoteDir = resolved.RemoteDir
		if *flagVerbose {
			printf("📍 Using host preset %q → %s\n", *flagRemoteHost, *flagRemote)
		}
	}

	// Expand bare `--remote discovery` hostnames via ssh_defaults (macOS default).
	if *flagRemote != "" && !strings.Contains(*flagRemote, "@") {
		*flagRemote = NormalizeSSHHost(*flagRemote, remotePlatformMacOS, config.SSHDefaults)
	}

	// Apply profile if specified
	if *flagProfile != "" {
		profile, ok := config.Profiles[*flagProfile]
		if !ok {
			fatalf("Profile '%s' not found in config", *flagProfile)
		}

		// Override flag values with profile settings
		if profile.NoCache {
			*flagNoCache = true
		}
		if profile.FailFast {
			*flagFailFast = true
		}
		if profile.JSON {
			*flagJSON = true
		}

		// Disable all stages first, then enable only the ones in the profile
		for name := range config.Stages {
			stage := config.Stages[name]
			stage.Enabled = false
			config.Stages[name] = stage
		}

		// Enable only stages from the profile
		for _, stageName := range profile.Stages {
			if stage, ok := config.Stages[stageName]; ok {
				stage.Enabled = true
				config.Stages[stageName] = stage
			}
		}
	}

	// Detect workspace
	ws, err := DetectWorkspace(cwd)
	if err != nil && *flagVerbose {
		warnf("Workspace detection failed: %v", err)
	}

	if *flagList {
		fmt.Printf("Available stages:\n")
		for name := range config.Stages {
			stage := config.Stages[name]
			status := "enabled"
			if !stage.Enabled {
				status = "disabled"
			}
			fmt.Printf("  %s (%s)\n", name, status)
		}
		return
	}

	// Build stage list from config
	stageMap := config.Stages
	var stages []Stage
	for _, name := range flag.Args() {
		if stage, ok := stageMap[name]; ok {
			stages = append(stages, stage)
		}
	}

	// If no stages specified, use enabled defaults
	if len(stages) == 0 {
		for _, name := range config.GetEnabledStages() {
			stages = append(stages, stageMap[name])
		}
	}

	// If --all, run every configured stage including disabled ones.
	// (Disabled stages are never added above, so rebuild the list from all
	// configured stages and force them enabled.)
	if *flagAll {
		if len(flag.Args()) == 0 {
			stages = stages[:0]
			for _, name := range config.GetAllStages() {
				stage := stageMap[name]
				stage.Name = name
				stage.Enabled = true
				stages = append(stages, stage)
			}
		} else {
			for i := range stages {
				stages[i].Enabled = true
			}
		}
	}

	// If --fix, modify fmt stage
	if *flagFix {
		for i := range stages {
			if stages[i].Name == "fmt" && len(stages[i].FixCmd) > 0 {
				stages[i].Cmd = stages[i].FixCmd
				stages[i].Check = false
				break
			}
		}
	}

	// Compute source hash
	sourceHash, err := computeSourceHash(cwd, config, ws)
	if err != nil {
		warnf("Warning: hash computation failed: %v\n", err)
		*flagNoCache = true
	}

	// Load cache if enabled
	var cache map[string]string
	if !*flagNoCache {
		cache, _ = loadCache(cwd)
	}
	if cache == nil {
		cache = make(map[string]string)
	}

	stageHashes, stageHashErr := computeStageHashes(cwd, config, ws, stages)
	if stageHashErr != nil && *flagVerbose {
		warnf("Warning: per-stage hash computation failed: %v\n", stageHashErr)
	}

	// Handle dry-run mode
	if *flagDryRun {
		var remote *DryRunRemote
		if *flagRemote != "" {
			workDir := *flagRemoteDir
			if workDir == "" {
				workDir = filepath.Join("/tmp", filepath.Base(cwd))
			}
			remote = &DryRunRemote{
				Host:       *flagRemote,
				Session:    *flagSession,
				WorkDir:    workDir,
				HostPreset: *flagRemoteHost,
			}
		}
		report := BuildDryRunReport(stages, cache, stageHashes, sourceHash, *flagNoCache, remote)
		if *flagJSON {
			PrintDryRunJSON(report)
		} else {
			PrintDryRunHuman(report)
		}
		return
	}

	// Run stages
	var results []Result
	start := time.Now()

	// Use parallel runner if requested
	if *flagParallel > 0 {
		if *flagRemote != "" {
			fatalf("Cannot use --parallel and --remote together; run remote stages sequentially")
		}
		runner := &ParallelRunner{
			Stages:      stages,
			Concurrency: *flagParallel,
			Cwd:         cwd,
			NoCache:     *flagNoCache,
			Cache:       cache,
			SourceHash:  sourceHash,
			StageHashes: stageHashes,
			Verbose:     *flagVerbose,
			JSON:        *flagJSON,
			FailFast:    *flagFailFast,
		}
		results = runner.Run()
	} else if *flagRemote != "" {
		// Remote sequential execution via SSH+tmux
		workDir := *flagRemoteDir
		if workDir == "" {
			workDir = filepath.Join("/tmp", filepath.Base(cwd))
		}
		re := NewRemoteExecutor(*flagRemote, *flagSession, workDir, time.Duration(*flagRemoteTimeout)*time.Second, *flagVerbose)

		// Sync local workspace to remote
		printf("🔄 Synchronizing local workspace to remote...\n")
		syncCtx, syncCancel := context.WithTimeout(context.Background(), 2*time.Minute)
		if err := re.SyncWorkspace(syncCtx, cwd, config.Cache.SkipDirs); err != nil {
			syncCancel()
			fatalf("Failed to sync workspace to remote: %v", err)
		}
		syncCancel()

		printf("🚀 Running local CI pipeline remotely on %s...\n\n", *flagRemote)

		// Ensure remote session exists
		ctx, cancel := context.WithTimeout(context.Background(), re.Timeout)
		if err := re.EnsureRemoteSession(ctx); err != nil {
			cancel()
			fatalf("Failed to initialize remote tmux session: %v", err)
		}
		cancel()

		for _, stage := range stages {
			stageStart := time.Now()

			// Compute per-stage hash for granular caching
			stageHash := sourceHash
			if len(stage.Watch) > 0 {
				if h, ok := stageHashes[stage.Name]; ok {
					stageHash = h
				} else {
					var err error
					stageHash, err = computeStageHash(stage, cwd, config, ws)
					if err != nil {
						if *flagVerbose {
							warnf("Stage hash computation failed for %s: %v\n", stage.Name, err)
						}
						stageHash = ""
					}
				}
			}

			// Check local cache
			if !*flagNoCache && cacheHit(cache, stage, stageHash) {
				if *flagVerbose {
					printf("✓ %s (cached)\n", stage.Name)
				}
				results = append(results, Result{
					Name:     stage.Name,
					Status:   "pass",
					CacheHit: true,
					Duration: 0,
				})
				continue
			}

			// Print stage header
			printf("::group::%s\n", stage.Name)
			if len(stage.Cmd) == 0 {
				printf("Error: Stage has no command defined\n")
				printf("::endgroup::\n")
				printf("✗ %s (failed)\n", stage.Name)
				results = append(results, Result{
					Name:     stage.Name,
					Status:   "fail",
					Duration: 0,
					Error:    fmt.Errorf("no command defined"),
				})
				if *flagFailFast {
					break
				}
				continue
			}
			if *flagVerbose {
				cmdStr := strings.Join(stage.Cmd, " ")
				printf("$ %s\n", cmdStr)
			}

			// Run stage remotely
			result := re.ExecuteStage(stage)
			duration := time.Since(stageStart)
			result.Duration = duration // set actual local time spent

			if result.Status == "fail" {
				if result.Output != "" {
					printf("%s\n", result.Output)
				} else if result.Error != nil {
					printf("Error: %v\n", result.Error)
				}
				printf("::endgroup::\n")
				printf("✗ %s (failed)\n", stage.Name)
				results = append(results, result)
				if *flagFailFast {
					break
				}
			} else {
				if *flagVerbose && result.Output != "" {
					printf("%s\n", result.Output)
				}
				printf("::endgroup::\n")
				printf("✓ %s (%dms)\n", stage.Name, result.Duration.Milliseconds())
				results = append(results, result)
				// Update cache
				cache[stage.Name] = cacheKeyForStage(stage, stageHash)
			}
		}
	} else {
		// Sequential execution
		printf("🚀 Running local CI pipeline...\n\n")

		for _, stage := range stages {
			stageStart := time.Now()

			// Compute per-stage hash for granular caching
			stageHash := sourceHash
			if len(stage.Watch) > 0 {
				if h, ok := stageHashes[stage.Name]; ok {
					stageHash = h
				} else {
					var err error
					stageHash, err = computeStageHash(stage, cwd, config, ws)
					if err != nil {
						if *flagVerbose {
							warnf("Stage hash computation failed for %s: %v\n", stage.Name, err)
						}
						stageHash = ""
					}
				}
			}

			// Check cache using per-stage hash
			if !*flagNoCache && cacheHit(cache, stage, stageHash) {
				if *flagVerbose {
					printf("✓ %s (cached)\n", stage.Name)
				}
				results = append(results, Result{
					Name:     stage.Name,
					Status:   "pass",
					CacheHit: true,
					Duration: 0,
				})
				continue
			}

			// Print stage header
			printf("::group::%s\n", stage.Name)
			if len(stage.Cmd) == 0 {
				printf("Error: Stage has no command defined\n")
				printf("::endgroup::\n")
				printf("✗ %s (failed)\n", stage.Name)
				results = append(results, Result{
					Name:     stage.Name,
					Status:   "fail",
					Duration: 0,
					Error:    fmt.Errorf("no command defined"),
				})
				if *flagFailFast {
					break
				}
				continue
			}

			if *flagVerbose {
				cmdStr := strings.Join(stage.Cmd, " ")
				printf("$ %s\n", cmdStr)
			}

			// Run stage with timeout
			timeout := time.Duration(stage.Timeout) * time.Second
			if timeout == 0 {
				timeout = 30 * time.Second
			}

			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			cmd := exec.CommandContext(ctx, stage.Cmd[0], stage.Cmd[1:]...)
			var out bytes.Buffer
			cmd.Stdout = &out
			cmd.Stderr = &out
			cmd.Dir = cwd

			err := cmd.Run()
			cancel()
			duration := time.Since(stageStart)

			if err != nil {
				printf("%s\n", out.String()) // Show output even if not verbose
				printf("::endgroup::\n")
				printf("✗ %s (failed)\n", stage.Name)
				results = append(results, Result{
					Name:     stage.Name,
					Status:   "fail",
					Duration: duration,
					Error:    err,
					Output:   out.String(),
				})
			} else {
				if *flagVerbose {
					printf("%s\n", out.String())
				}
				printf("::endgroup::\n")
				printf("✓ %s (%dms)\n", stage.Name, duration.Milliseconds())
				results = append(results, Result{
					Name:     stage.Name,
					Status:   "pass",
					Duration: duration,
					Output:   out.String(),
				})
				// Update cache with per-stage hash
				cache[stage.Name] = cacheKeyForStage(stage, stageHash)
			}
		}
	}

	// Save cache
	if !*flagNoCache {
		saveCache(cache, cwd)
	}

	// Summary
	printf("\n")
	totalDuration := time.Since(start)
	passCount := 0
	failCount := 0
	cachedCount := 0
	executedCount := 0
	totalTime := time.Duration(0)

	for _, r := range results {
		if r.Status == "pass" {
			passCount++
			if r.CacheHit {
				cachedCount++
			} else {
				executedCount++
				totalTime += r.Duration
			}
		} else {
			failCount++
			totalTime += r.Duration
		}
	}

	// Summary line
	if failCount == 0 {
		successf("✅ All %d stage(s) passed in %dms\n", len(results), totalDuration.Milliseconds())
	} else {
		errorf("❌ %d/%d stages failed\n", failCount, len(results))
	}

	// Statistics
	printf("\n📊 Summary:\n")
	printf("  Total stages: %d\n", len(results))
	printf("  Passed: %d\n", passCount)
	if failCount > 0 {
		printf("  Failed: %d\n", failCount)
	}
	if cachedCount > 0 {
		printf("  Cached: %d (%.0f%%)\n", cachedCount, float64(cachedCount)*100/float64(len(results)))
	}
	if executedCount > 0 {
		printf("  Executed: %d\n", executedCount)
	}
	printf("  Total time: %dms\n", totalDuration.Milliseconds())

	// Show missing tools (optional)
	missingTools := GetMissingToolsWithHints(DetectProjectKind(cwd))
	if len(missingTools) > 0 {
		printf("%s", FormatMissingToolsMessage(missingTools))
	}

	if *flagJSON {
		report := PipelineReportJSON{
			Results:    toJSONResults(results),
			Passed:     passCount,
			Failed:     failCount,
			DurationMS: totalDuration.Milliseconds(),
		}
		data, err := json.MarshalIndent(report, "", "  ")
		if err == nil {
			fmt.Fprintln(os.Stdout, string(data))
		}
	}

	// Exit with error if any stage failed
	if failCount > 0 {
		os.Exit(1)
	}
}

// matchesPatterns checks if a filename matches any of the given patterns
func matchesPatterns(filename string, patterns []string) bool {
	for _, pattern := range patterns {
		// Simple pattern matching: *.rs, *.toml
		if strings.HasPrefix(pattern, "*.") {
			ext := pattern[1:] // Get .rs or .toml
			if strings.HasSuffix(filename, ext) {
				return true
			}
		} else if pattern == "*" {
			// Match all files
			return true
		} else if filename == pattern {
			// Exact filename match
			return true
		}
	}
	return false
}

// computeStageHashes computes hashes for multiple stages in one pass
func computeStageHashes(root string, config *Config, ws *Workspace, stages []Stage) (map[string]string, error) {
	result := make(map[string]string)
	for _, stage := range stages {
		hash, err := computeStageHash(stage, root, config, ws)
		if err != nil {
			return nil, err
		}
		result[stage.Name] = hash
	}
	return result, nil
}

// computeSourceHash computes MD5 hash of Rust source files
func computeSourceHash(root string, config *Config, ws *Workspace) (string, error) {
	h := md5.New()

	// Build skip set from config
	skipDirs := make(map[string]bool)
	for _, dir := range config.Cache.SkipDirs {
		skipDirs[dir] = true
	}

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories in config
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
		}

		// Skip excluded workspace members
		if ws != nil && !ws.IsSingle {
			relPath, err := filepath.Rel(root, path)
			if err == nil && ws.IsExcluded(relPath) {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		// Hash files matching include patterns
		if !d.IsDir() {
			if matchesPatterns(d.Name(), config.Cache.IncludePatterns) {
				data, err := os.ReadFile(path)
				if err != nil {
					return nil // Skip unreadable files
				}
				h.Write(data)
			}
		}

		return nil
	})

	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// computeStageHash computes MD5 hash for a specific stage based on its watch patterns
func computeStageHash(stage Stage, root string, config *Config, ws *Workspace) (string, error) {
	h := md5.New()

	// If no watch patterns, use global hash
	if len(stage.Watch) == 0 {
		return computeSourceHash(root, config, ws)
	}

	// Build skip set from config
	skipDirs := make(map[string]bool)
	for _, dir := range config.Cache.SkipDirs {
		skipDirs[dir] = true
	}

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories in config
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
		}

		// Skip excluded workspace members
		if ws != nil && !ws.IsSingle {
			relPath, err := filepath.Rel(root, path)
			if err == nil && ws.IsExcluded(relPath) {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		// Hash files matching watch patterns
		if !d.IsDir() {
			if matchesPatterns(d.Name(), stage.Watch) {
				data, err := os.ReadFile(path)
				if err != nil {
					return nil // Skip unreadable files
				}
				h.Write(data)
			}
		}

		return nil
	})

	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// loadCache loads the cache from .local-ci-cache
func loadCache(root string) (map[string]string, error) {
	cache := make(map[string]string)
	cachePath := filepath.Join(root, ".local-ci-cache")

	data, err := os.ReadFile(cachePath)
	if err != nil {
		return cache, nil // Cache doesn't exist, return empty
	}

	// Simple format: stage:hash\n
	for _, line := range strings.Split(string(data), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			cache[parts[0]] = parts[1]
		}
	}

	return cache, nil
}

// saveCache saves the cache to .local-ci-cache
func saveCache(cache map[string]string, root string) error {
	keys := make([]string, 0, len(cache))
	for stage := range cache {
		keys = append(keys, stage)
	}
	sort.Strings(keys)

	var lines []string
	for _, stage := range keys {
		lines = append(lines, fmt.Sprintf("%s:%s", stage, cache[stage]))
	}

	cachePath := filepath.Join(root, ".local-ci-cache")
	return os.WriteFile(cachePath, []byte(strings.Join(lines, "\n")), 0644)
}

// cmdInit initializes a new .local-ci.toml configuration
func cmdInit(root string) {
	// Detect workspace
	ws, err := DetectWorkspace(root)
	if err != nil {
		fatalf("Failed to detect workspace: %v", err)
	}

	printf("📦 Initializing local-ci for %s\n", root)

	if ws.IsSingle {
		printf("  Single crate: %s\n", ws.Members[0])
	} else {
		printf("  Workspace with %d members\n", len(ws.Members))
		for _, member := range ws.Members {
			if !ws.IsExcluded(member) {
				printf("    ✓ %s\n", member)
			} else {
				printf("    ✗ %s (excluded)\n", member)
			}
		}
	}

	// Create .local-ci.toml
	if err := SaveDefaultConfig(root, ws); err != nil {
		fatalf("Failed to save config: %v", err)
	}
	successf("✅ Created .local-ci.toml\n")

	// Update .gitignore
	gitignorePath := filepath.Join(root, ".gitignore")
	updateGitignore(gitignorePath, ".local-ci-cache")
	successf("✅ Updated .gitignore\n")

	// Try to create pre-commit hook if .git exists
	gitDir := filepath.Join(root, ".git")
	if _, err := os.Stat(gitDir); err == nil {
		if err := CreatePreCommitHook(root); err == nil {
			successf("✅ Created pre-commit hook\n")
		} else if !os.IsNotExist(err) {
			warnf("Could not create pre-commit hook: %v\n", err)
		}
	}

	printf("\n💡 Next steps:\n")
	printf("  1. Run 'local-ci' to test the setup\n")
	printf("  2. Customize .local-ci.toml as needed\n")
	printf("  3. Consider installing cargo tools:\n")
	printf("     - cargo install cargo-deny\n")
	printf("     - cargo install cargo-audit\n")
	printf("     - cargo install cargo-machete\n")
}

// updateGitignore adds an entry to .gitignore if not already present
func updateGitignore(path string, entry string) error {
	data, err := os.ReadFile(path)
	var content string
	if err == nil {
		content = string(data)
	}

	// Check if entry already exists
	if strings.Contains(content, entry) {
		return nil
	}

	// Append entry
	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += entry + "\n"

	return os.WriteFile(path, []byte(content), 0644)
}

// Printing helpers
func printf(format string, args ...interface{}) {
	if flagJSON != nil && *flagJSON {
		fmt.Fprintf(os.Stderr, format, args...)
	} else {
		fmt.Fprintf(os.Stdout, format, args...)
	}
}

func successf(format string, args ...interface{}) {
	if flagJSON != nil && *flagJSON {
		fmt.Fprintf(os.Stderr, "\033[32m"+format+"\033[0m", args...)
	} else {
		fmt.Fprintf(os.Stdout, "\033[32m"+format+"\033[0m", args...)
	}
}

func errorf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "\033[31m"+format+"\033[0m", args...)
}

func warnf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "\033[33m"+format+"\033[0m", args...)
}

func fatalf(format string, args ...interface{}) {
	errorf(format+"\n", args...)
	os.Exit(1)
}

// requireCommand checks if a command is available in PATH
func requireCommand(name string) error {
	if _, err := exec.LookPath(name); err != nil {
		return fmt.Errorf("%q not found in PATH", name)
	}
	return nil
}

// validateStageCommands checks that all stage commands are available
func validateStageCommands(stages []Stage) error {
	for _, s := range stages {
		if len(s.Cmd) == 0 {
			return fmt.Errorf("stage %q has empty command", s.Name)
		}
		if err := requireCommand(s.Cmd[0]); err != nil {
			return fmt.Errorf("stage %q requires %q: %w", s.Name, s.Cmd[0], err)
		}
	}
	return nil
}
