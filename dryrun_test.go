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
		"fmt":  "hash1|cargo fmt",
		"test": "hash1|cargo test",
	}

	report := BuildDryRunReport("/tmp/project", "hash1", nil, stages, cache, false)

	if !report.DryRun {
		t.Error("expected DryRun to be true")
	}
	if report.ToRun != 0 {
		t.Errorf("expected 0 to run, got %d", report.ToRun)
	}
	if report.Cached != 2 {
		t.Errorf("expected 2 cached, got %d", report.Cached)
	}
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
		"fmt":  "oldhash|cargo fmt",
		"test": "oldhash|cargo test",
	}

	report := BuildDryRunReport("/tmp/project", "newhash", nil, stages, cache, false)

	if report.ToRun != 2 {
		t.Errorf("expected 2 to run, got %d", report.ToRun)
	}
	if report.Cached != 0 {
		t.Errorf("expected 0 cached, got %d", report.Cached)
	}
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

	// Even with matching cache, noCache=true should force run
	cache := map[string]string{
		"fmt": "hash1|cargo fmt",
	}

	report := BuildDryRunReport("/tmp/project", "hash1", nil, stages, cache, true)

	if report.ToRun != 1 {
		t.Errorf("expected 1 to run with no-cache, got %d", report.ToRun)
	}
	if report.Stages[0].Reason != "no_cache" {
		t.Errorf("expected reason 'no_cache', got %q", report.Stages[0].Reason)
	}
}

func TestBuildDryRunReportDisabledStages(t *testing.T) {
	enabledStages := []Stage{
		{Name: "fmt", Cmd: []string{"cargo", "fmt"}, Enabled: true},
	}

	allStages := map[string]Stage{
		"fmt":   {Name: "fmt", Cmd: []string{"cargo", "fmt"}, Enabled: true},
		"deny":  {Name: "deny", Cmd: []string{"cargo", "deny"}, Enabled: false},
		"audit": {Name: "audit", Cmd: []string{"cargo", "audit"}, Enabled: false},
	}

	report := BuildDryRunReport("/tmp/project", "hash1", allStages, enabledStages, nil, true)

	if report.Disabled != 2 {
		t.Errorf("expected 2 disabled, got %d", report.Disabled)
	}

	disabledCount := 0
	for _, s := range report.Stages {
		if s.Reason == "disabled" {
			disabledCount++
			if s.WouldRun {
				t.Errorf("disabled stage %q should not run", s.Name)
			}
		}
	}
	if disabledCount != 2 {
		t.Errorf("expected 2 disabled stages in report, got %d", disabledCount)
	}
}

func TestBuildDryRunReportMixedStates(t *testing.T) {
	enabledStages := []Stage{
		{Name: "fmt", Cmd: []string{"cargo", "fmt"}, Enabled: true},
		{Name: "test", Cmd: []string{"cargo", "test"}, Enabled: true},
	}

	allStages := map[string]Stage{
		"fmt":  {Name: "fmt", Cmd: []string{"cargo", "fmt"}, Enabled: true},
		"test": {Name: "test", Cmd: []string{"cargo", "test"}, Enabled: true},
		"deny": {Name: "deny", Cmd: []string{"cargo", "deny"}, Enabled: false},
	}

	cache := map[string]string{
		"fmt": "hash1|cargo fmt", // cached
		// test not in cache → stale
	}

	report := BuildDryRunReport("/tmp/project", "hash1", allStages, enabledStages, cache, false)

	if report.ToRun != 1 {
		t.Errorf("expected 1 to run, got %d", report.ToRun)
	}
	if report.Cached != 1 {
		t.Errorf("expected 1 cached, got %d", report.Cached)
	}
	if report.Disabled != 1 {
		t.Errorf("expected 1 disabled, got %d", report.Disabled)
	}
	if len(report.Stages) != 3 {
		t.Errorf("expected 3 total stages, got %d", len(report.Stages))
	}
}

func TestBuildDryRunReportWorkspaceAndHash(t *testing.T) {
	report := BuildDryRunReport("/home/user/project", "abc123def", nil, nil, nil, false)

	if report.Workspace != "/home/user/project" {
		t.Errorf("expected workspace '/home/user/project', got %q", report.Workspace)
	}
	if report.SourceHash != "abc123def" {
		t.Errorf("expected source hash 'abc123def', got %q", report.SourceHash)
	}
}

func TestBuildDryRunReportEmptyStages(t *testing.T) {
	report := BuildDryRunReport("/tmp", "hash", nil, nil, nil, false)

	if report.ToRun != 0 || report.Cached != 0 || report.Disabled != 0 {
		t.Error("empty stages should have all zero counts")
	}
}
