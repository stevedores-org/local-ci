// local-ci — Local CI runner for Rust workspaces.
//
// Provides a fast, cacheable local CI pipeline that mirrors GitHub Actions
// for Rust projects. Supports file-hash caching and colored output.
//
// Usage:
//
//	local-ci                Run default stages (fmt, clippy, test)
//	local-ci fmt clippy     Run specific stages
//	local-ci --no-cache     Disable caching, force all stages
//	local-ci --fix          Auto-fix formatting (cargo fmt without --check)
//	local-ci --verbose      Show detailed output
//	local-ci --list         List available stages
//	local-ci --version      Print version
//
package main

import (
	"bytes"
	"crypto/md5"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

var version = "0.1.0"

type Stage struct {
<<<<<<< Updated upstream
	Name  string
	Cmd   []string
	Check bool // true if this is a --check command
=======
	Name      string
	Cmd       []string
	FixCmd    []string   // command to run with --fix flag
	Check     bool       // true if this is a --check command
	Timeout   int        // in seconds
	Enabled   bool
	DependsOn []string   // stage names this stage depends on
	Watch     []string   // file patterns this stage cares about (for granular caching)
>>>>>>> Stashed changes
}

type Result struct {
	Name      string
	Status    string
	Duration  time.Duration
	Output    string
	CacheHit  bool
	Error     error
}

func main() {
	var (
		flagNoCache  = flag.Bool("no-cache", false, "Disable file hash cache")
		flagVerbose  = flag.Bool("verbose", false, "Show detailed output")
		flagFix      = flag.Bool("fix", false, "Auto-fix issues (cargo fmt)")
		flagList     = flag.Bool("list", false, "List available stages")
		flagVersion  = flag.Bool("version", false, "Print version")
<<<<<<< Updated upstream
=======
		flagAll      = flag.Bool("all", false, "Run all stages including disabled ones")
		flagRemote   = flag.Bool("remote", false, "Load remote config from .local-ci-remote.toml")
		flagProfile  = flag.String("profile", "", "Use a named profile from config")
		flagDryRun   = flag.Bool("dry-run", false, "Show what would run without executing")
		flagParallel = flag.Int("parallel", 0, "Number of parallel jobs (0 = auto)")
		flagJSON     = flag.Bool("json", false, "Output in JSON format")
		flagFailFast = flag.Bool("fail-fast", false, "Stop on first failure")
>>>>>>> Stashed changes
	)

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "local-ci v%s — Local CI for Rust workspaces\n\n", version)
		fmt.Fprintf(os.Stderr, "Usage: local-ci [flags] [stages...]\n\n")
<<<<<<< Updated upstream
		fmt.Fprintf(os.Stderr, "Stages:\n")
		fmt.Fprintf(os.Stderr, "  fmt       Format check (cargo fmt --check)\n")
		fmt.Fprintf(os.Stderr, "  clippy    Linter (cargo clippy -D warnings)\n")
		fmt.Fprintf(os.Stderr, "  test      Tests (cargo test --workspace)\n")
		fmt.Fprintf(os.Stderr, "  check     Compile check (cargo check)\n\n")
=======
		fmt.Fprintf(os.Stderr, "Commands:\n")
		fmt.Fprintf(os.Stderr, "  init      Initialize .local-ci.toml for detected project type\n\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  local-ci              Run enabled stages for your project\n")
		fmt.Fprintf(os.Stderr, "  local-ci test         Run only the test stage\n")
		fmt.Fprintf(os.Stderr, "  local-ci --fix        Auto-fix format/lint issues\n")
		fmt.Fprintf(os.Stderr, "  local-ci --profile ci Run stages from 'ci' profile\n")
		fmt.Fprintf(os.Stderr, "  local-ci --dry-run    Show what would run\n")
		fmt.Fprintf(os.Stderr, "  local-ci --list       List all available stages\n\n")
>>>>>>> Stashed changes
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

<<<<<<< Updated upstream
=======
	// Handle subcommands
	args := flag.Args()
	if len(args) > 0 {
		if args[0] == "init" {
			cmdInit(cwd)
			return
		} else if args[0] == "serve" {
			cmdServe(cwd)
			return
		}
	}

	// Load configuration
	config, err := LoadConfig(cwd, *flagRemote)
	if err != nil {
		fatalf("Failed to load config: %v", err)
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

>>>>>>> Stashed changes
	if *flagList {
		fmt.Printf("Available stages: fmt, clippy, test, check\n")
		return
	}

	// Define stages
	stages := []Stage{
		{
			Name:  "fmt",
			Cmd:   []string{"cargo", "fmt", "--all", "--", "--check"},
			Check: true,
		},
		{
			Name:  "clippy",
			Cmd:   []string{"cargo", "clippy", "--workspace", "--", "-D", "warnings"},
			Check: false,
		},
		{
			Name:  "test",
			Cmd:   []string{"cargo", "test", "--workspace"},
			Check: false,
		},
		{
			Name:  "check",
			Cmd:   []string{"cargo", "check", "--workspace"},
			Check: false,
		},
	}

	// If --fix, modify fmt stage
	if *flagFix {
		for i := range stages {
			if stages[i].Name == "fmt" {
				stages[i].Cmd = []string{"cargo", "fmt", "--all"}
				stages[i].Check = false
				break
			}
		}
	}

	// Determine which stages to run
	requestedStages := flag.Args()
	if len(requestedStages) == 0 {
		requestedStages = []string{"fmt", "clippy", "test"}
	}

	// Filter stages
	var toRun []Stage
	for _, requested := range requestedStages {
		for _, stage := range stages {
			if stage.Name == requested {
				toRun = append(toRun, stage)
				break
			}
		}
	}

	// Compute source hash
<<<<<<< Updated upstream
	sourceHash, err := computeSourceHash(cwd)
	if err != nil && *flagVerbose {
		warnf("Hash computation failed: %v", err)
=======
	sourceHash, err := computeSourceHash(cwd, config, ws)
	if err != nil {
		warnf("Warning: hash computation failed: %v\n", err)
>>>>>>> Stashed changes
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

	// Handle dry-run mode
	if *flagDryRun {
		report := BuildDryRunReport(stages, cache, sourceHash, *flagNoCache)
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
		runner := &ParallelRunner{
			Stages:      stages,
			Concurrency: *flagParallel,
			Cwd:         cwd,
			NoCache:     *flagNoCache,
			Cache:       cache,
			SourceHash:  sourceHash,
			Verbose:     *flagVerbose,
			JSON:        *flagJSON,
			FailFast:    *flagFailFast,
		}
		results = runner.Run()
	} else {
		// Sequential execution
		printf("🚀 Running local CI pipeline...\n\n")

<<<<<<< Updated upstream
	for _, stage := range toRun {
=======
		for _, stage := range stages {
>>>>>>> Stashed changes
		stageStart := time.Now()

		// Compute per-stage hash for granular caching
		stageHash := sourceHash
		if len(stage.Watch) > 0 {
			var err error
			stageHash, err = computeStageHash(stage, cwd, config, ws)
			if err != nil && *flagVerbose {
				warnf("Stage hash computation failed for %s: %v\n", stage.Name, err)
			}
		}

		// Check cache using per-stage hash
		if !*flagNoCache && cache[stage.Name] == stageHash {
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

		// Run stage
		cmd := exec.Command(stage.Cmd[0], stage.Cmd[1:]...)
		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &out
		cmd.Dir = cwd

		err := cmd.Run()
		duration := time.Since(stageStart)

		if err != nil {
			printf("✗ %s (failed)\n", stage.Name)
			if *flagVerbose {
				printf("%s\n", out.String())
			}
			results = append(results, Result{
				Name:     stage.Name,
				Status:   "fail",
				Duration: duration,
				Error:    err,
				Output:   out.String(),
			})
		} else {
			printf("✓ %s (%dms)\n", stage.Name, duration.Milliseconds())
			results = append(results, Result{
				Name:     stage.Name,
				Status:   "pass",
				Duration: duration,
				Output:   out.String(),
			})
			// Update cache with per-stage hash
			cache[stage.Name] = stageHash
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
	for _, r := range results {
		if r.Status == "pass" {
			passCount++
		}
	}

	if passCount == len(results) {
		successf("✅ All %d stage(s) passed in %dms\n", len(results), totalDuration.Milliseconds())
	} else {
		errorf("❌ %d/%d stages failed\n", len(results)-passCount, len(results))
		os.Exit(1)
	}
}

// computeSourceHash computes MD5 hash of Rust source files
func computeSourceHash(root string) (string, error) {
	h := md5.New()

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip .git, target, .github, scripts
		if d.IsDir() {
			switch d.Name() {
			case ".git", "target", ".github", "scripts", ".claude":
				return filepath.SkipDir
			}
		}

		// Only hash Rust files and Cargo files
		if !d.IsDir() && (strings.HasSuffix(d.Name(), ".rs") ||
			strings.HasSuffix(d.Name(), ".toml")) {
			data, err := os.ReadFile(path)
			if err != nil {
				return nil // Skip unreadable files
			}
			h.Write(data)
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
			shouldHash := false
			for _, pattern := range stage.Watch {
				// Simple pattern matching: *.rs, *.toml
				if strings.HasPrefix(pattern, "*.") {
					ext := pattern[1:] // Get .rs or .toml
					if strings.HasSuffix(d.Name(), ext) {
						shouldHash = true
						break
					}
				} else if pattern == "*" {
					// Match all files
					shouldHash = true
					break
				} else if d.Name() == pattern {
					// Exact filename match
					shouldHash = true
					break
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
		parts := strings.Split(line, ":")
		if len(parts) == 2 {
			cache[parts[0]] = parts[1]
		}
	}

	return cache, nil
}

// saveCache saves the cache to .local-ci-cache
func saveCache(cache map[string]string, root string) error {
	var lines []string
	for stage, hash := range cache {
		lines = append(lines, fmt.Sprintf("%s:%s", stage, hash))
	}

	cachePath := filepath.Join(root, ".local-ci-cache")
	return os.WriteFile(cachePath, []byte(strings.Join(lines, "\n")), 0644)
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
