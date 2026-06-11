package main

import (
	"testing"
	"time"
)

func TestParallelRunnerFailFastPassingDeadlock(t *testing.T) {
	dir := t.TempDir()
	stages := []Stage{
		{Name: "s1", Cmd: []string{"echo", "1"}, Timeout: 10},
		{Name: "s2", Cmd: []string{"echo", "2"}, Timeout: 10},
	}
	pr := &ParallelRunner{
		Stages: stages, Concurrency: 1, Cwd: dir, NoCache: true,
		Cache: map[string]string{}, SourceHash: "h", FailFast: true,
	}
	done := make(chan struct{})
	go func() { pr.Run(); close(done) }()
	select {
	case <-done:
		// expected: fail-fast must not deadlock when all stages pass
	case <-time.After(2 * time.Second):
		t.Fatal("parallel fail-fast hung with all-passing stages")
	}
}
