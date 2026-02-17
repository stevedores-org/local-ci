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
package main

import (
	"bytes"
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

var version = "0.1.0"

type Stage struct {
	Name  string
	Cmd   []string
	Check bool // true if this is a --check command
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
	)

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "local-ci v%s — Local CI for Rust workspaces\n\n", version)
		fmt.Fprintf(os.Stderr, "Usage: local-ci [flags] [stages...]\n\n")
		fmt.Fprintf(os.Stderr, "Stages:\n")
		fmt.Fprintf(os.Stderr, "  fmt       Format check (cargo fmt --check)\n")
		fmt.Fprintf(os.Stderr, "  clippy    Linter (cargo clippy -D warnings)\n")
		fmt.Fprintf(os.Stderr, "  test      Tests (cargo test --workspace)\n")
		fmt.Fprintf(os.Stderr, "  check     Compile check (cargo check)\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *flagVersion {
		fmt.Printf("local-ci v%s\n", version)
		return
	}

	if err := requireCommand("cargo"); err != nil {
		fatalf("%v", err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		fatalf("Cannot get working directory: %v", err)
	}

	if *flagList {
		fmt.Printf("Available stages: fmt, clippy, test, check\n")
		return
	}

	// Define stages
	stages := availableStages()
	if *flagFix {
		for i := range stages {
			if stages[i].Name == "fmt" {
				stages[i].Cmd = []string{"cargo", "fmt", "--all"}
				stages[i].Check = false
				break
			}
		}
	}

	requestedStages := flag.Args()
	if len(requestedStages) == 0 {
		requestedStages = []string{"fmt", "clippy", "test"}
	}

	toRun, err := selectStages(requestedStages, stages)
	if err != nil {
		fatalf("%v", err)
	}

	// Compute source hash
	sourceHash, err := computeSourceHash(cwd)
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

	if !*flagJSON {
		printf("🚀 Running local CI pipeline...\n\n")
	}

	for _, stage := range toRun {
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

		// Run stage
		cmd := exec.Command(stage.Cmd[0], stage.Cmd[1:]...)
		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &out
		cmd.Dir = cwd

		err := cmd.Run()
		duration := time.Since(stageStart)

		if err != nil {
			if !*flagJSON {
				printf("✗ %s (failed)\n", stage.Name)
				if *flagVerbose {
					printf("%s\n", out.String())
				}
			}
			results = append(results, Result{
				Name:     stage.Name,
				Command:  cmdStr,
				Status:   "fail",
				Duration: duration,
				Error:    err,
				Output:   out.String(),
			})
			if *flagFailFast {
				break
			}
		} else {
			if !*flagJSON {
				printf("✓ %s (%dms)\n", stage.Name, duration.Milliseconds())
			}
			results = append(results, Result{
				Name:     stage.Name,
				Command:  cmdStr,
				Status:   "pass",
				Duration: duration,
				Output:   out.String(),
			})
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
	for _, r := range results {
		if r.Status == "pass" {
			passCount++
		}
	}
	failCount := len(results) - passCount

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

	printf("\n")
	if passCount == len(results) {
		successf("✅ All %d stage(s) passed in %dms\n", len(results), totalDuration.Milliseconds())
	} else {
		errorf("❌ %d/%d stages failed\n", len(results)-passCount, len(results))
		os.Exit(1)
	}
}

func availableStages() []Stage {
	return []Stage{
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
}

func selectStages(requested []string, stages []Stage) ([]Stage, error) {
	stageMap := make(map[string]Stage, len(stages))
	for _, stage := range stages {
		stageMap[stage.Name] = stage
	}

	var out []Stage
	for _, name := range requested {
		stage, ok := stageMap[name]
		if !ok {
			return nil, fmt.Errorf("unknown stage %q (use --list)", name)
		}
		out = append(out, stage)
	}
	return out, nil
}

func requireCommand(name string) error {
	if _, err := exec.LookPath(name); err != nil {
		return fmt.Errorf("required command %q not found in PATH", name)
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
