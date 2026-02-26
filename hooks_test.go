package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreatePreCommitHookNewRepo(t *testing.T) {
	dir := t.TempDir()
	gitDir := filepath.Join(dir, ".git")
	os.MkdirAll(gitDir, 0755)

	err := CreatePreCommitHook(dir)
	if err != nil {
		t.Fatalf("CreatePreCommitHook failed: %v", err)
	}

	hookPath := filepath.Join(gitDir, "hooks", "pre-commit")
	data, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("Hook file not created: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "local-ci") {
		t.Error("Hook should contain local-ci invocation")
	}
	if !strings.HasPrefix(content, "#!/bin/bash") {
		t.Error("Hook should start with bash shebang")
	}

	// Verify executable permission
	info, _ := os.Stat(hookPath)
	if info.Mode()&0111 == 0 {
		t.Error("Hook should be executable")
	}
}

func TestCreatePreCommitHookIdempotent(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".git"), 0755)

	// Create twice
	if err := CreatePreCommitHook(dir); err != nil {
		t.Fatal(err)
	}
	if err := CreatePreCommitHook(dir); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, ".git", "hooks", "pre-commit"))
	count := strings.Count(string(data), "local-ci pre-commit hook")
	if count != 1 {
		t.Errorf("expected 1 local-ci section, got %d", count)
	}
}

func TestCreatePreCommitHookHooksDirCreated(t *testing.T) {
	dir := t.TempDir()
	// .git exists but .git/hooks does not
	os.MkdirAll(filepath.Join(dir, ".git"), 0755)

	err := CreatePreCommitHook(dir)
	if err != nil {
		t.Fatalf("Should create hooks dir: %v", err)
	}

	info, err := os.Stat(filepath.Join(dir, ".git", "hooks"))
	if err != nil || !info.IsDir() {
		t.Error("hooks directory should be created")
	}
}

func TestRemovePreCommitHookFullRemoval(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".git", "hooks"), 0755)

	// Write the exact template
	hookPath := filepath.Join(dir, ".git", "hooks", "pre-commit")
	os.WriteFile(hookPath, []byte(getPreCommitHookTemplate(dir)), 0755)

	err := RemovePreCommitHook(dir)
	if err != nil {
		t.Fatalf("RemovePreCommitHook failed: %v", err)
	}

	// File should be gone
	if _, err := os.Stat(hookPath); !os.IsNotExist(err) {
		t.Error("Hook file should be removed entirely when it only contains local-ci")
	}
}

func TestRemovePreCommitHookNonExistent(t *testing.T) {
	dir := t.TempDir()
	// No .git/hooks/pre-commit

	err := RemovePreCommitHook(dir)
	if err != nil {
		t.Fatalf("Should not error for nonexistent hook: %v", err)
	}
}

func TestRemovePreCommitHookNoGitDir(t *testing.T) {
	dir := t.TempDir()

	err := RemovePreCommitHook(dir)
	if err != nil {
		t.Fatalf("Should not error when .git doesn't exist: %v", err)
	}
}
