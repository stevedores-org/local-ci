package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindYAMLFiles(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "yamllint-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test directories and files
	dirsToCreate := []string{
		"sub1",
		"sub2",
		"ignored_dir",
		".git",
	}
	for _, d := range dirsToCreate {
		if err := os.MkdirAll(filepath.Join(tmpDir, d), 0755); err != nil {
			t.Fatalf("failed to create dir: %v", err)
		}
	}

	filesToCreate := []string{
		"file1.yml",
		"file2.yaml",
		filepath.Join("sub1", "file3.yml"),
		filepath.Join("sub2", "file4.yaml"),
		filepath.Join("ignored_dir", "file5.yml"),
		filepath.Join(".git", "config.yml"),
		"not_yaml.txt",
	}
	for _, f := range filesToCreate {
		if err := os.WriteFile(filepath.Join(tmpDir, f), []byte(""), 0644); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}
	}

	skipDirs := []string{"ignored_dir"}
	found, err := findYAMLFiles(tmpDir, skipDirs)
	if err != nil {
		t.Fatalf("findYAMLFiles failed: %v", err)
	}

	expected := map[string]bool{
		"file1.yml":       true,
		"file2.yaml":      true,
		"sub1/file3.yml":  true,
		"sub2/file4.yaml": true,
	}

	if len(found) != len(expected) {
		t.Errorf("expected %d files, got %d: %v", len(expected), len(found), found)
	}

	for _, f := range found {
		if !expected[f] {
			t.Errorf("unexpected file found: %s", f)
		}
	}
}

func TestCmdYamllintNoFiles(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "yamllint-test-empty-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Should not return an error when no files are found
	err = cmdYamllint(tmpDir)
	if err != nil {
		t.Errorf("expected no error when no yaml files found, got: %v", err)
	}
}

func TestCmdYamllintSkipsWhenToolMissing(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "config.yml"), []byte("name: test\n"), 0644); err != nil {
		t.Fatalf("failed to write yaml file: %v", err)
	}

	t.Setenv("PATH", tmpDir)

	if err := cmdYamllint(tmpDir); err != nil {
		t.Fatalf("expected missing optional yamllint to skip without error, got: %v", err)
	}
}

func TestCmdYamllintReturnsFailure(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "config.yml"), []byte("name: test\n"), 0644); err != nil {
		t.Fatalf("failed to write yaml file: %v", err)
	}

	binDir := t.TempDir()
	fakeYamllint := filepath.Join(binDir, "yamllint")
	if err := os.WriteFile(fakeYamllint, []byte("#!/bin/sh\nexit 7\n"), 0755); err != nil {
		t.Fatalf("failed to write fake yamllint: %v", err)
	}
	t.Setenv("PATH", binDir)

	err := cmdYamllint(tmpDir)
	if err == nil {
		t.Fatal("expected yamllint failure to return an error")
	}
	if !strings.Contains(err.Error(), "exit code 7") {
		t.Fatalf("expected exit code in error, got: %v", err)
	}
}
