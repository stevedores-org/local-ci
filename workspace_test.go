package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExpandGlobPatterns(t *testing.T) {
	dir := t.TempDir()

	// Create directories matching crates/*
	for _, name := range []string{"alpha", "beta", "gamma"} {
		os.MkdirAll(filepath.Join(dir, "crates", name), 0755)
	}

	patterns := []string{"crates/*"}
	result, err := expandGlobPatterns(dir, patterns)
	if err != nil {
		t.Fatalf("expandGlobPatterns failed: %v", err)
	}

	if len(result) != 3 {
		t.Errorf("expected 3 matches, got %d: %v", len(result), result)
	}

	// Results should be relative paths
	for _, r := range result {
		if filepath.IsAbs(r) {
			t.Errorf("expected relative path, got absolute: %s", r)
		}
	}
}

func TestExpandGlobPatternsNoGlob(t *testing.T) {
	dir := t.TempDir()

	patterns := []string{"crates/alpha", "crates/beta"}
	result, err := expandGlobPatterns(dir, patterns)
	if err != nil {
		t.Fatalf("expandGlobPatterns failed: %v", err)
	}

	// Non-glob patterns should be passed through as-is
	if len(result) != 2 {
		t.Errorf("expected 2 results, got %d: %v", len(result), result)
	}
	if result[0] != "crates/alpha" || result[1] != "crates/beta" {
		t.Errorf("unexpected result: %v", result)
	}
}

func TestExpandGlobPatternsNoMatches(t *testing.T) {
	dir := t.TempDir()

	patterns := []string{"nonexistent/*"}
	result, err := expandGlobPatterns(dir, patterns)
	if err != nil {
		t.Fatalf("expandGlobPatterns failed: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("expected 0 matches, got %d: %v", len(result), result)
	}
}

func TestExpandGlobPatternsMixed(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "crates", "alpha"), 0755)

	patterns := []string{"explicit/path", "crates/*"}
	result, err := expandGlobPatterns(dir, patterns)
	if err != nil {
		t.Fatalf("expandGlobPatterns failed: %v", err)
	}

	// Should have 1 literal + 1 glob match
	if len(result) != 2 {
		t.Errorf("expected 2 results, got %d: %v", len(result), result)
	}
}

func TestGetMembersWorkspace(t *testing.T) {
	ws := &Workspace{
		IsSingle: false,
		Members:  []string{"crate1", "crate2"},
	}
	members := ws.GetMembers()
	if len(members) != 2 || members[0] != "crate1" {
		t.Errorf("unexpected members: %v", members)
	}
}

func TestGetMembersSingleCrate(t *testing.T) {
	ws := &Workspace{
		IsSingle: true,
		Members:  []string{"my-crate"},
	}
	members := ws.GetMembers()
	if len(members) != 1 || members[0] != "." {
		t.Errorf("single crate GetMembers should return [.], got %v", members)
	}
}

func TestGetIncludedMembers(t *testing.T) {
	ws := &Workspace{
		IsSingle: false,
		Members:  []string{"crate1", "crate2", "crate3"},
		Excludes: []string{"crate2"},
	}
	included := ws.GetIncludedMembers()
	if len(included) != 2 {
		t.Errorf("expected 2 included members, got %d: %v", len(included), included)
	}
	for _, m := range included {
		if m == "crate2" {
			t.Error("crate2 should be excluded")
		}
	}
}

func TestGetIncludedMembersAllExcluded(t *testing.T) {
	ws := &Workspace{
		IsSingle: false,
		Members:  []string{"crate1"},
		Excludes: []string{"crate1"},
	}
	included := ws.GetIncludedMembers()
	if len(included) != 1 || included[0] != "." {
		t.Errorf("all excluded should return [.], got %v", included)
	}
}

func TestIsExcludedChildPath(t *testing.T) {
	ws := &Workspace{
		Excludes: []string{"legacy"},
	}

	if !ws.IsExcluded("legacy") {
		t.Error("legacy should be excluded")
	}
	if !ws.IsExcluded("legacy" + string(filepath.Separator) + "old") {
		t.Error("legacy/old child path should be excluded")
	}
	if ws.IsExcluded("not-legacy") {
		t.Error("not-legacy should not be excluded")
	}
}

func TestDetectWorkspaceNoCargoToml(t *testing.T) {
	dir := t.TempDir()

	_, err := DetectWorkspace(dir)
	if err == nil {
		t.Error("expected error when Cargo.toml is missing")
	}
}

func TestDetectWorkspaceInvalidToml(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte("not valid toml {{{}}}"), 0644)

	_, err := DetectWorkspace(dir)
	if err == nil {
		t.Error("expected error for invalid TOML")
	}
}

func TestDetectWorkspaceNoPackageOrWorkspace(t *testing.T) {
	dir := t.TempDir()
	// Valid TOML but no [package] or [workspace]
	os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte("[dependencies]\ntokio = \"1\"\n"), 0644)

	_, err := DetectWorkspace(dir)
	if err == nil {
		t.Error("expected error for Cargo.toml without [package] or [workspace]")
	}
}
