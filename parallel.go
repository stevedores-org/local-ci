package main

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// ParallelRunner executes stages concurrently, respecting DependsOn constraints.
type ParallelRunner struct {
	Stages      []Stage
	Concurrency int // max parallel goroutines (0 = unlimited)
	Cwd         string
	NoCache     bool
	Cache       map[string]string
	StageHashes map[string]string // per-stage content hashes
	Verbose     bool
	JSON        bool
	FailFast    bool
	cacheMu     sync.RWMutex // protects Cache reads during parallel execution
}

// buildDepGraph returns a map from stage name to its dependency name (or "").
// It also validates that all DependsOn targets exist in the stage list.
func buildDepGraph(stages []Stage) (map[string]string, error) {
	names := make(map[string]bool, len(stages))
	for _, s := range stages {
		names[s.Name] = true
	}

	deps := make(map[string]string, len(stages))
	for _, s := range stages {
		if s.DependsOn != "" {
			if !names[s.DependsOn] {
				return nil, fmt.Errorf("stage %q depends on %q which is not in the stage list", s.Name, s.DependsOn)
			}
			deps[s.Name] = s.DependsOn
		}
	}

	// Check for cycles using a simple visited/recursion check
	if err := detectCycles(deps); err != nil {
		return nil, err
	}

	return deps, nil
}

// detectCycles checks the dependency map for cycles.
func detectCycles(deps map[string]string) error {
	visited := make(map[string]bool)
	for name := range deps {
		if visited[name] {
			continue
		}
		chain := make(map[string]bool)
		cur := name
		for cur != "" {
			if chain[cur] {
				return fmt.Errorf("circular dependency detected involving %q", cur)
			}
			chain[cur] = true
			visited[cur] = true
			cur = deps[cur]
		}
	}
	return nil
}

// resolveOrder returns stage execution layers: stages in each layer can run in parallel.
// Stages with no dependencies are in layer 0. Stages depending on layer-N stages are in layer N+1.
func resolveOrder(stages []Stage) ([][]Stage, error) {
	deps, err := buildDepGraph(stages)
	if err != nil {
		return nil, err
	}

	stageMap := make(map[string]Stage, len(stages))
	for _, s := range stages {
		stageMap[s.Name] = s
	}

	// Compute layer for each stage
	layerOf := make(map[string]int)
	var getLayer func(name string) int
	getLayer = func(name string) int {
		if l, ok := layerOf[name]; ok {
			return l
		}
		dep := deps[name]
		if dep == "" {
			layerOf[name] = 0
			return 0
		}
		l := getLayer(dep) + 1
		layerOf[name] = l
		return l
	}

	maxLayer := 0
	for _, s := range stages {
		l := getLayer(s.Name)
		if l > maxLayer {
			maxLayer = l
		}
	}

	layers := make([][]Stage, maxLayer+1)
	for _, s := range stages {
		l := layerOf[s.Name]
		layers[l] = append(layers[l], s)
	}

	return layers, nil
}

// RunParallel executes stages in dependency-order layers, with concurrency within each layer.
func (pr *ParallelRunner) RunParallel() ([]Result, error) {
	layers, err := resolveOrder(pr.Stages)
	if err != nil {
		return nil, err
	}

	var allResults []Result
	var mu sync.Mutex
	failed := false

	for _, layer := range layers {
		if pr.FailFast && failed {
			break
		}

		sem := make(chan struct{}, pr.concurrency())
		var wg sync.WaitGroup
		var layerResults []Result

		for _, stage := range layer {
			if pr.FailFast && failed {
				break
			}

			wg.Add(1)
			go func(s Stage) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				result := pr.executeStage(s)

				mu.Lock()
				layerResults = append(layerResults, result)
				if result.Status == "fail" {
					failed = true
				}
				mu.Unlock()
			}(stage)
		}

		wg.Wait()
		allResults = append(allResults, layerResults...)
	}

	return allResults, nil
}

func (pr *ParallelRunner) concurrency() int {
	if pr.Concurrency <= 0 {
		return len(pr.Stages) // effectively unlimited
	}
	return pr.Concurrency
}

func (pr *ParallelRunner) executeStage(stage Stage) Result {
	stageStart := time.Now()
	cmdStr := strings.Join(stage.Cmd, " ")
	stageCacheKey := pr.StageHashes[stage.Name] + "|" + cmdStr

	// Check cache (protected for concurrent reads during parallel execution)
	pr.cacheMu.RLock()
	cacheHit := !pr.NoCache && pr.Cache[stage.Name] == stageCacheKey
	pr.cacheMu.RUnlock()
	if cacheHit {
		return Result{
			Name:     stage.Name,
			Command:  cmdStr,
			Status:   "pass",
			CacheHit: true,
			Duration: 0,
		}
	}

	timeout := time.Duration(stage.Timeout) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, stage.Cmd[0], stage.Cmd[1:]...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	cmd.Dir = pr.Cwd

	err := cmd.Run()
	duration := time.Since(stageStart)

	result := Result{
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

	return result
}
