package main

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"sync"
	"sync/atomic"
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
	if r.Concurrency <= 0 {
		r.Concurrency = runtime.NumCPU()
	}

	sem := make(chan struct{}, r.Concurrency)
	completed := make(map[string]bool)
	var mu sync.Mutex
	var failed atomic.Bool

	resultChan := make(chan Result, len(r.Stages))
	var wg sync.WaitGroup

	// Resolve dependencies to only those present in the selected stage set,
	// and detect cycles. Without this, a stage that depends on an unselected
	// stage (e.g. `local-ci --parallel clippy`, where clippy depends on the
	// unselected fmt) or a dependency cycle would wait forever and hang Run().
	stageDeps, cyclic := resolveDeps(r.Stages)
	stageIndex := make(map[string]int, len(r.Stages))
	for i, stage := range r.Stages {
		stageIndex[stage.Name] = i
	}

	for _, stage := range r.Stages {
		wg.Add(1)
		go func(s Stage) {
			defer wg.Done()

			// A stage caught in a dependency cycle can never become runnable;
			// fail it instead of spinning forever.
			if cyclic[s.Name] {
				resultChan <- Result{
					Name:   s.Name,
					Status: "fail",
					Error:  fmt.Errorf("dependency cycle involving stage %q", s.Name),
				}
				failed.Store(true)
				mu.Lock()
				completed[s.Name] = true
				mu.Unlock()
				return
			}

			for {
				mu.Lock()
				allDone := true
				for _, dep := range stageDeps[s.Name] {
					if !completed[dep] {
						allDone = false
						break
					}
				}
				if allDone && r.FailFast {
					myIdx := stageIndex[s.Name]
					for _, earlier := range r.Stages {
						if stageIndex[earlier.Name] >= myIdx {
							break
						}
						if !completed[earlier.Name] {
							allDone = false
							break
						}
					}
				}
				mu.Unlock()
				if allDone {
					break
				}
				time.Sleep(10 * time.Millisecond)
			}

			skip := func() {
				resultChan <- Result{
					Name:   s.Name,
					Status: "skip",
				}
				mu.Lock()
				completed[s.Name] = true
				mu.Unlock()
			}

			if r.FailFast && failed.Load() {
				skip()
				return
			}

			sem <- struct{}{}
			defer func() { <-sem }()

			if r.FailFast && failed.Load() {
				skip()
				return
			}

			result := r.executeStage(s)
			if result.Status != "pass" {
				failed.Store(true)
			} else if !result.CacheHit {
				mu.Lock()
				r.Cache[s.Name] = cacheKeyForStage(s, r.stageHash(s))
				mu.Unlock()
			}
			resultChan <- result

			mu.Lock()
			completed[s.Name] = true
			mu.Unlock()
		}(stage)
	}

	wg.Wait()
	close(resultChan)

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

// resolveDeps returns each stage's dependency list pruned to dependencies that
// are actually present in the selected stage set (and not self-referential),
// plus the set of stage names involved in a dependency cycle. Pruning prevents
// an indefinite wait on an unselected dependency; the cycle set lets Run fail
// those stages instead of deadlocking.
func resolveDeps(stages []Stage) (deps map[string][]string, cyclic map[string]bool) {
	inSet := make(map[string]bool, len(stages))
	for _, s := range stages {
		inSet[s.Name] = true
	}

	deps = make(map[string][]string, len(stages))
	indeg := make(map[string]int, len(stages))
	dependents := make(map[string][]string) // dep -> stages that depend on it
	for _, s := range stages {
		var d []string
		for _, dep := range s.DependsOn {
			if dep != s.Name && inSet[dep] {
				d = append(d, dep)
			}
		}
		deps[s.Name] = d
		indeg[s.Name] = len(d)
		for _, dep := range d {
			dependents[dep] = append(dependents[dep], s.Name)
		}
	}

	// Kahn's algorithm: anything left with indegree > 0 is part of a cycle.
	var queue []string
	for _, s := range stages {
		if indeg[s.Name] == 0 {
			queue = append(queue, s.Name)
		}
	}
	resolved := 0
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		resolved++
		for _, m := range dependents[n] {
			indeg[m]--
			if indeg[m] == 0 {
				queue = append(queue, m)
			}
		}
	}

	cyclic = make(map[string]bool)
	if resolved < len(stages) {
		for _, s := range stages {
			if indeg[s.Name] > 0 {
				cyclic[s.Name] = true
			}
		}
	}
	return deps, cyclic
}

func (r *ParallelRunner) stageHash(stage Stage) string {
	if r.StageHashes != nil {
		if h, ok := r.StageHashes[stage.Name]; ok {
			return h
		}
	}
	return r.SourceHash
}

// executeStage runs a single stage
func (r *ParallelRunner) executeStage(stage Stage) Result {
	stageStart := time.Now()
	hash := r.stageHash(stage)

	if !r.NoCache && cacheHit(r.Cache, stage, hash) {
		return Result{
			Name:     stage.Name,
			Status:   "pass",
			CacheHit: true,
			Duration: 0,
		}
	}

	if len(stage.Cmd) == 0 {
		return Result{
			Name:     stage.Name,
			Status:   "fail",
			Duration: time.Since(stageStart),
			Error:    fmt.Errorf("no command defined"),
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

	return Result{
		Name:     stage.Name,
		Status:   "pass",
		Duration: duration,
		Output:   out.String(),
	}
}
