package main

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"sync"
	"time"
)

// ParallelRunner executes stages concurrently while respecting dependencies
type ParallelRunner struct {
	Stages      []Stage
	Concurrency int
	Cwd         string
	NoCache     bool
	Cache       map[string]string
	SourceHash  string
	StageHashes map[string]string // Per-stage hashes for cache validation
	Verbose     bool
	JSON        bool
	FailFast    bool
}

// Run executes all stages concurrently with dependency management
func (r *ParallelRunner) Run() []Result {
	// Adjust concurrency
	if r.Concurrency <= 0 {
		r.Concurrency = runtime.NumCPU()
	}

	// Create a semaphore to limit concurrent executions
	sem := make(chan struct{}, r.Concurrency)

	// Track stage completion for dependency resolution
	completed := make(map[string]bool)
	failed := false // set once any stage fails (for fail-fast)
	var mu sync.Mutex

	// Result channel for collecting results
	resultChan := make(chan Result, len(r.Stages))
	var wg sync.WaitGroup

	// Build dependency graph
	stageDeps := make(map[string][]string)
	for _, stage := range r.Stages {
		stageDeps[stage.Name] = stage.DependsOn
	}

	// Process each stage
	for _, stage := range r.Stages {
		wg.Add(1)
		go func(s Stage) {
			defer wg.Done()

			// Wait for dependencies
			for {
				mu.Lock()
				allDone := true
				for _, dep := range stageDeps[s.Name] {
					if !completed[dep] {
						allDone = false
						break
					}
				}
				mu.Unlock()

				if allDone {
					break
				}
				time.Sleep(10 * time.Millisecond)
			}

			// Fail-fast: if a prior stage already failed, skip this one.
			// Mark it completed so dependents don't wait forever, and emit
			// no result (a non-"pass" result would be counted as a failure
			// by the summary).
			if r.FailFast {
				mu.Lock()
				skip := failed
				mu.Unlock()
				if skip {
					mu.Lock()
					completed[s.Name] = true
					mu.Unlock()
					return
				}
			}

			// Acquire semaphore
			sem <- struct{}{}
			defer func() { <-sem }()

			// Execute stage
			result := r.executeStage(s)
			resultChan <- result

			// Record completion and failure state
			mu.Lock()
			if result.Status != "pass" {
				failed = true
			}
			completed[s.Name] = true
			mu.Unlock()
		}(stage)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(resultChan)

	// Collect results in order
	resultMap := make(map[string]Result)
	for result := range resultChan {
		resultMap[result.Name] = result
	}

	var results []Result
	for _, stage := range r.Stages {
		if result, ok := resultMap[stage.Name]; ok {
			results = append(results, result)
		}
	}

	return results
}

// executeStage runs a single stage
func (r *ParallelRunner) executeStage(stage Stage) Result {
	stageStart := time.Now()

	// Guard against a stage with no command (mirrors the sequential path);
	// indexing stage.Cmd[0] below would otherwise panic.
	if len(stage.Cmd) == 0 {
		return Result{
			Name:   stage.Name,
			Status: "fail",
			Error:  fmt.Errorf("no command defined for stage %q", stage.Name),
		}
	}

	// Check cache
	if !r.NoCache && r.Cache[stage.Name] == r.SourceHash {
		return Result{
			Name:     stage.Name,
			Status:   "pass",
			CacheHit: true,
			Duration: 0,
		}
	}

	// Create context with timeout
	timeout := time.Duration(stage.Timeout) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Execute command
	cmd := exec.CommandContext(ctx, stage.Cmd[0], stage.Cmd[1:]...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	cmd.Dir = r.Cwd

	err := cmd.Run()
	duration := time.Since(stageStart)

	if err != nil {
		return Result{
			Name:     stage.Name,
			Status:   "fail",
			Duration: duration,
			Error:    err,
			Output:   out.String(),
		}
	}

	// Note: cache update is done by the caller after collecting results

	return Result{
		Name:     stage.Name,
		Status:   "pass",
		Duration: duration,
		Output:   out.String(),
	}
}
