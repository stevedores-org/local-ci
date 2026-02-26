// local-ci — Universal local CI runner for any project type.
//
// Provides a fast, cacheable local CI pipeline that mirrors GitHub Actions
// for Rust, Python, Node.js, Go, Java, and other projects.
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
//	local-ci --remote HOST  Run stages on remote machine via SSH (e.g., aivcs@100.90.209.9)
//	local-ci --session NAME Use specific tmux session name (default: onion)
//
// Supported project types:
//   - Rust (Cargo.toml)
//   - Python (pyproject.toml, setup.py, requirements.txt)
//   - Node.js (package.json)
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

var version = "0.3.0" // Universal project support (Rust, Python, Node, Go, Java, etc.)

type Stage struct {
	Name    string   `toml:"-"`
	Cmd     []string `toml:"command"`
	FixCmd  []string `toml:"fix_command"` // command to run with --fix flag
	Check   bool     `toml:"check"`       // true if this is a --check command
	Timeout int      `toml:"timeout"`     // in seconds
	Enabled bool     `toml:"enabled"`
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

type PipelineReport struct {
	Version    string       `json:"version"`
	DurationMS int64        `json:"duration_ms"`
	Passed     int          `json:"passed"`
	Failed     int          `json:"failed"`
	Results    []ResultJSON `json:"results"`
}

type ResultJSON struct {
	Name       string `json:"name"`
	Command    string `json:"command"`
	Status     string `json:"status"`
	DurationMS int64  `json:"duration_ms"`
	CacheHit   bool   `json:"cache_hit"`
	Output     string `json:"output,omitempty"`
	Error      string `json:"error,omitempty"`
}

func main() {
	var (
		flagNoCache  = flag.Bool("no-cache", false, "Disable file hash cache")
		flagVerbose  = flag.Bool("verbose", false, "Show detailed output")
		flagFix      = flag.Bool("fix", false, "Auto-fix issues (cargo fmt)")
		flagFailFast = flag.Bool("fail-fast", false, "Stop on first failed stage")
		flagJSON     = flag.Bool("json", false, "Emit machine-readable JSON report")
		flagList     = flag.Bool("list", false, "List available stages")
		flagVersion  = flag.Bool("version", false, "Print version")
		flagAll      = flag.Bool("all", false, "Run all stages including disabled ones")
		flagRemote   = flag.String("remote", "", "Run stages on remote machine via SSH (e.g., aivcs@100.90.209.9)")
		flagSession  = flag.String("session", "onion", "tmux session name for remote execution")
	)

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "local-ci v%s — Universal local CI for any project\n\n", version)
		fmt.Fprintf(os.Stderr, "Supports: Rust, Python, Node.js, Go, Java, and custom projects\n\n")
		fmt.Fprintf(os.Stderr, "Usage: local-ci [flags] [stages...]\n\n")
		fmt.Fprintf(os.Stderr, "Commands:\n")
		fmt.Fprintf(os.Stderr, "  init      Initialize .local-ci.toml for detected project type\n")
		fmt.Fprintf(os.Stderr, "  serve     Start MCP server on stdio for AI agent integration\n\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  local-ci              Run enabled stages for your project\n")
		fmt.Fprintf(os.Stderr, "  local-ci test         Run only the test stage\n")
		fmt.Fprintf(os.Stderr, "  local-ci --fix        Auto-fix format/lint issues\n")
		fmt.Fprintf(os.Stderr, "  local-ci --list       List all available stages\n\n")
		fmt.Fprintf(os.Stderr, "Remote execution:\n")
		fmt.Fprintf(os.Stderr, "  --remote HOST  Run stages on remote machine via SSH (e.g., aivcs@100.90.209.9)\n")
		fmt.Fprintf(os.Stderr, "  --session NAME Use specific tmux session name (default: onion)\n\n")
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
	if len(args) > 0 && args[0] == "init" {
		cmdInit(cwd)
		return
	}
	if len(args) > 0 && args[0] == "serve" {
		if err := cmdServe(cwd); err != nil {
			fatalf("MCP server error: %v", err)
		}
		return
	}

	// Load configuration
	config, err := LoadConfig(cwd)
	if err != nil {
		fatalf("Failed to load config: %v", err)
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
		} else {
			fatalf("unknown stage %q (use --list)", name)
		}
	}

	// If no stages specified, use enabled defaults
	if len(stages) == 0 {
		for _, name := range config.GetEnabledStages() {
			stages = append(stages, stageMap[name])
		}
	}

	// If --all, rebuild stage list from all stages (including disabled)
	if *flagAll {
		stages = nil
		for name, stage := range stageMap {
			stage.Name = name
			stage.Enabled = true
			stages = append(stages, stage)
		}
	}

	// Validate that commands for selected stages are installed
	if err := validateStageCommands(stages); err != nil {
		fatalf("%v", err)
	}

	// If --fix, swap in fix commands for stages that have them
	if *flagFix {
		for i := range stages {
			if len(stages[i].FixCmd) > 0 {
				stages[i].Cmd = stages[i].FixCmd
				stages[i].Check = false
			}
		}
	}

	// Compute source hash
	sourceHash, err := computeSourceHash(cwd, config, ws)
	if err != nil && *flagVerbose {
		warnf("Hash computation failed: %v", err)
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

	// Run stages
	var results []Result
	start := time.Now()

	// Initialize remote executor if --remote flag is set
	var remoteExec *RemoteExecutor
	if *flagRemote != "" {
		if !*flagJSON {
			printf("🚀 Running local CI pipeline remotely on %s...\n\n", *flagRemote)
		}
		remoteExec = NewRemoteExecutor(*flagRemote, *flagSession, cwd, 30*time.Second, *flagVerbose)

		// Test SSH connection
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := remoteExec.TestSSHConnection(ctx); err != nil {
			cancel()
			fatalf("Cannot connect to remote host %s: %v", *flagRemote, err)
		}
		cancel()

		// Ensure remote session exists
		ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
		if err := remoteExec.EnsureRemoteSession(ctx); err != nil {
			cancel()
			fatalf("Cannot create remote tmux session: %v", err)
		}
		cancel()
	} else {
		if !*flagJSON {
			printf("🚀 Running local CI pipeline...\n\n")
		}
	}

	for _, stage := range stages {
		stageStart := time.Now()
		cmdStr := strings.Join(stage.Cmd, " ")
		stageCacheKey := sourceHash + "|" + cmdStr

		// Check cache
		if !*flagNoCache && cache[stage.Name] == stageCacheKey {
			if *flagVerbose && !*flagJSON {
				printf("✓ %s (cached)\n", stage.Name)
			}
			results = append(results, Result{
				Name:     stage.Name,
				Command:  cmdStr,
				Status:   "pass",
				CacheHit: true,
				Duration: 0,
			})
			continue
		}

		// Print stage header
		if !*flagJSON {
			printf("::group::%s\n", stage.Name)
			if *flagVerbose {
				printf("$ %s\n", cmdStr)
			}
		}

		var result Result

		// Execute stage (local or remote)
		if remoteExec != nil {
			// Remote execution
			result = remoteExec.ExecuteStage(stage)
		} else {
			// Local execution
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

			result = Result{
				Name:     stage.Name,
				Command:  cmdStr,
				Status:   "pass",
				Duration: duration,
				Output:   out.String(),
			}
			if err != nil {
				result.Status = "fail"
				result.Error = err
			}
		}

		// Post-process and output result
		if result.Status == "fail" {
			if !*flagJSON {
				if result.Output != "" {
					printf("%s\n", result.Output)
				}
				printf("::endgroup::\n")
				printf("✗ %s (failed)\n", result.Name)
			}
			results = append(results, result)
			if *flagFailFast {
				break
			}
		} else {
			if !*flagJSON {
				if *flagVerbose && result.Output != "" {
					printf("%s\n", result.Output)
				}
				printf("::endgroup::\n")
				printf("✓ %s (%dms)\n", result.Name, result.Duration.Milliseconds())
			}
			results = append(results, result)
			// Update cache
			cache[stage.Name] = stageCacheKey
		}
	}

	// Save cache
	if !*flagNoCache {
		saveCache(cache, cwd)
	}

	// Summary
	totalDuration := time.Since(start)
	passCount := 0
	failCount := 0
	cachedCount := 0
	executedCount := 0

	for _, r := range results {
		if r.Status == "pass" {
			passCount++
			if r.CacheHit {
				cachedCount++
			} else {
				executedCount++
			}
		} else {
			failCount++
		}
	}

	// JSON output mode
	if *flagJSON {
		report := PipelineReport{
			Version:    version,
			DurationMS: totalDuration.Milliseconds(),
			Passed:     passCount,
			Failed:     failCount,
			Results:    toJSONResults(results),
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(report)
		if failCount > 0 {
			os.Exit(1)
		}
		return
	}

	// Human-readable summary
	printf("\n")
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

	// Exit with error if any stage failed
	if failCount > 0 {
		os.Exit(1)
	}
}

func requireCommand(name string) error {
	if _, err := exec.LookPath(name); err != nil {
		return fmt.Errorf("required command %q not found in PATH", name)
	}
	return nil
}

// validateStageCommands checks that the binary for each stage is available in PATH.
func validateStageCommands(stages []Stage) error {
	seen := make(map[string]bool)
	for _, stage := range stages {
		if len(stage.Cmd) == 0 {
			return fmt.Errorf("stage %q has empty command", stage.Name)
		}

		cmd := stage.Cmd[0]
		if seen[cmd] {
			continue
		}

		if err := requireCommand(cmd); err != nil {
			return fmt.Errorf("stage %q requires command %q: %w", stage.Name, cmd, err)
		}
		seen[cmd] = true
	}
	return nil
}

func toJSONResults(results []Result) []ResultJSON {
	out := make([]ResultJSON, 0, len(results))
	for _, r := range results {
		item := ResultJSON{
			Name:       r.Name,
			Command:    r.Command,
			Status:     r.Status,
			DurationMS: r.Duration.Milliseconds(),
			CacheHit:   r.CacheHit,
			Output:     strings.TrimSpace(r.Output),
		}
		if r.Error != nil {
			item.Error = r.Error.Error()
		}
		out = append(out, item)
	}
	return out
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
			shouldHash := false
			for _, pattern := range config.Cache.IncludePatterns {
				// Simple pattern matching: *.rs, *.toml
				if strings.HasPrefix(pattern, "*.") {
					ext := pattern[1:] // Get .rs or .toml
					if strings.HasSuffix(d.Name(), ext) {
						shouldHash = true
						break
					}
				}
			}

			if shouldHash {
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
	var lines []string
	keys := make([]string, 0, len(cache))
	for stage := range cache {
		keys = append(keys, stage)
	}
	sort.Strings(keys)
	for _, stage := range keys {
		hash := cache[stage]
		lines = append(lines, fmt.Sprintf("%s:%s", stage, hash))
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
	fmt.Fprintf(os.Stdout, format, args...)
}

func successf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stdout, "\033[32m"+format+"\033[0m", args...)
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
