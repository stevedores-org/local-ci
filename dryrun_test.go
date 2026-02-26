package main

import (
	"testing"
)

func TestBuildDryRunReportAllCached(t *testing.T) {
	stages := []Stage{
		{Name: "fmt", Cmd: []string{"cargo", "fmt"}, Enabled: true},
		{Name: "test", Cmd: []string{"cargo", "test"}, Enabled: true},
	}

	cache := map[string]string{
		"fmt":  "hash1",
		"test": "hash1",
	}

	report := BuildDryRunReport(stages, cache, "hash1", false)

	for _, s := range report.Stages {
		if s.WouldRun {
			t.Errorf("stage %q should not run (cached)", s.Name)
		}
		if s.Reason != "cached" {
			t.Errorf("stage %q reason should be 'cached', got %q", s.Name, s.Reason)
		}
	}
}

func TestBuildDryRunReportHashChanged(t *testing.T) {
	stages := []Stage{
		{Name: "fmt", Cmd: []string{"cargo", "fmt"}, Enabled: true},
		{Name: "test", Cmd: []string{"cargo", "test"}, Enabled: true},
	}

	cache := map[string]string{
		"fmt":  "oldhash",
		"test": "oldhash",
	}

	report := BuildDryRunReport(stages, cache, "newhash", false)

	for _, s := range report.Stages {
		if !s.WouldRun {
			t.Errorf("stage %q should run (hash changed)", s.Name)
		}
		if s.Reason != "hash_changed" {
			t.Errorf("stage %q reason should be 'hash_changed', got %q", s.Name, s.Reason)
		}
	}
}

func TestBuildDryRunReportNoCache(t *testing.T) {
	stages := []Stage{
		{Name: "fmt", Cmd: []string{"cargo", "fmt"}, Enabled: true},
	}

	cache := map[string]string{
		"fmt": "hash1",
	}

	report := BuildDryRunReport(stages, cache, "hash1", true)

	if len(report.Stages) != 1 {
		t.Fatalf("expected 1 stage, got %d", len(report.Stages))
	}
	if !report.Stages[0].WouldRun {
		t.Error("stage should run with no-cache flag")
	}
	if report.Stages[0].Reason != "no_cache_flag" {
		t.Errorf("expected reason 'no_cache_flag', got %q", report.Stages[0].Reason)
	}
}

func TestBuildDryRunReportDisabledStages(t *testing.T) {
	stages := []Stage{
		{Name: "fmt", Cmd: []string{"cargo", "fmt"}, Enabled: true},
		{Name: "deny", Cmd: []string{"cargo", "deny"}, Enabled: false},
	}

	report := BuildDryRunReport(stages, nil, "hash1", true)

	disabledCount := 0
	for _, s := range report.Stages {
		if s.Reason == "disabled" {
			disabledCount++
			if s.WouldRun {
				t.Errorf("disabled stage %q should not run", s.Name)
			}
		}
	}
	if disabledCount != 1 {
		t.Errorf("expected 1 disabled stage, got %d", disabledCount)
	}
}

func TestBuildDryRunReportMixedStates(t *testing.T) {
	stages := []Stage{
		{Name: "fmt", Cmd: []string{"cargo", "fmt"}, Enabled: true},
		{Name: "test", Cmd: []string{"cargo", "test"}, Enabled: true},
		{Name: "deny", Cmd: []string{"cargo", "deny"}, Enabled: false},
	}

	cache := map[string]string{
		"fmt": "hash1", // cached
		// test not cached
	}

	report := BuildDryRunReport(stages, cache, "hash1", false)

	if len(report.Stages) != 3 {
		t.Errorf("expected 3 stages, got %d", len(report.Stages))
	}
}

func TestBuildDryRunReportSourceHash(t *testing.T) {
	report := BuildDryRunReport(nil, nil, "abc123def", false)

	if report.SourceHash != "abc123def" {
		t.Errorf("expected source hash 'abc123def', got %q", report.SourceHash)
	}
}

func TestBuildDryRunReportEmptyStages(t *testing.T) {
	report := BuildDryRunReport(nil, nil, "hash", false)

	if len(report.Stages) != 0 {
		t.Errorf("expected 0 stages, got %d", len(report.Stages))
	}
}
