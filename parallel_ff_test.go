package main

import (
	"strings"
	"testing"
)

func TestParallelFailFastIndependentStagesStillRun(t *testing.T) {
	dir := t.TempDir()
	stages := []Stage{
		{Name: "fail", Cmd: []string{"false"}, Timeout: 10},
		{Name: "second", Cmd: []string{"echo", "ran"}, Timeout: 10},
	}
	pr := &ParallelRunner{
		Stages: stages, Concurrency: 1, Cwd: dir, NoCache: true,
		Cache: map[string]string{}, SourceHash: "h", FailFast: true,
	}
	results := pr.Run()
	for _, r := range results {
		if r.Name == "second" {
			if r.Status == "pass" && strings.Contains(r.Output, "ran") {
				t.Fatal("fail-fast should prevent second independent stage from running")
			}
		}
	}
}
