package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDetectProjectTypeRust verifies Rust project detection
func TestDetectProjectTypeRust(t *testing.T) {
	tmpdir := t.TempDir()
	// Create Cargo.toml
	if err := os.WriteFile(filepath.Join(tmpdir, "Cargo.toml"), []byte("[package]"), 0644); err != nil {
		t.Fatal(err)
	}

	projectType := DetectProjectType(tmpdir)
	if projectType != ProjectTypeRust {
		t.Errorf("Expected ProjectTypeRust, got %s", projectType)
	}
}

// TestDetectProjectTypePython verifies Python project detection
func TestDetectProjectTypePython(t *testing.T) {
	tmpdir := t.TempDir()
	// Create pyproject.toml
	if err := os.WriteFile(filepath.Join(tmpdir, "pyproject.toml"), []byte("[tool.poetry]"), 0644); err != nil {
		t.Fatal(err)
	}

	projectType := DetectProjectType(tmpdir)
	if projectType != ProjectTypePython {
		t.Errorf("Expected ProjectTypePython, got %s", projectType)
	}
}

// TestDetectProjectTypeNode verifies Node.js project detection
func TestDetectProjectTypeNode(t *testing.T) {
	tmpdir := t.TempDir()
	// Create package.json
	if err := os.WriteFile(filepath.Join(tmpdir, "package.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	projectType := DetectProjectType(tmpdir)
	if projectType != ProjectTypeNode {
		t.Errorf("Expected ProjectTypeNode, got %s", projectType)
	}
}

// TestDetectProjectTypeGo verifies Go project detection
func TestDetectProjectTypeGo(t *testing.T) {
	tmpdir := t.TempDir()
	// Create go.mod
	if err := os.WriteFile(filepath.Join(tmpdir, "go.mod"), []byte("module example.com"), 0644); err != nil {
		t.Fatal(err)
	}

	projectType := DetectProjectType(tmpdir)
	if projectType != ProjectTypeGo {
		t.Errorf("Expected ProjectTypeGo, got %s", projectType)
	}
}

// TestDetectProjectTypeJava verifies Java project detection
func TestDetectProjectTypeJava(t *testing.T) {
	tmpdir := t.TempDir()
	// Create pom.xml
	if err := os.WriteFile(filepath.Join(tmpdir, "pom.xml"), []byte("<?xml"), 0644); err != nil {
		t.Fatal(err)
	}

	projectType := DetectProjectType(tmpdir)
	if projectType != ProjectTypeJava {
		t.Errorf("Expected ProjectTypeJava, got %s", projectType)
	}
}

// TestDetectProjectTypeGeneric verifies generic project detection
func TestDetectProjectTypeGeneric(t *testing.T) {
	tmpdir := t.TempDir()
	// Empty directory - should be generic

	projectType := DetectProjectType(tmpdir)
	if projectType != ProjectTypeGeneric {
		t.Errorf("Expected ProjectTypeGeneric, got %s", projectType)
	}
}

// TestGetDefaultStagesForRust verifies Rust-specific stages
func TestGetDefaultStagesForRust(t *testing.T) {
	stages := GetDefaultStagesForType(ProjectTypeRust)

	expectedStages := []string{"fmt", "clippy", "test", "check"}
	for _, stageName := range expectedStages {
		if _, exists := stages[stageName]; !exists {
			t.Errorf("Expected stage %s not found for Rust", stageName)
		}
	}

	// Verify fmt is enabled by default
	if !stages["fmt"].Enabled {
		t.Error("fmt stage should be enabled by default for Rust")
	}
}

// TestGetDefaultStagesForPython verifies Python-specific stages
func TestGetDefaultStagesForPython(t *testing.T) {
	stages := GetDefaultStagesForType(ProjectTypePython)

	expectedStages := []string{"lint", "format", "test"}
	for _, stageName := range expectedStages {
		if _, exists := stages[stageName]; !exists {
			t.Errorf("Expected stage %s not found for Python", stageName)
		}
	}
}

// TestGetDefaultStagesForNode verifies Node.js-specific stages
func TestGetDefaultStagesForNode(t *testing.T) {
	stages := GetDefaultStagesForType(ProjectTypeNode)

	expectedStages := []string{"lint", "test", "build"}
	for _, stageName := range expectedStages {
		if _, exists := stages[stageName]; !exists {
			t.Errorf("Expected stage %s not found for Node", stageName)
		}
	}
}

// TestGetCachePatternForType verifies cache patterns are language-specific
func TestGetCachePatternForType(t *testing.T) {
	tests := []struct {
		projectType ProjectType
		shouldContain string
	}{
		{ProjectTypeRust, "*.rs"},
		{ProjectTypePython, "*.py"},
		{ProjectTypeNode, "*.js"},
		{ProjectTypeGo, "*.go"},
		{ProjectTypeJava, "*.java"},
	}

	for _, tt := range tests {
		patterns := GetCachePatternForType(tt.projectType)
		found := false
		for _, pattern := range patterns {
			if pattern == tt.shouldContain {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected pattern %s for %s, not found", tt.shouldContain, tt.projectType)
		}
	}
}

// TestGetSkipDirsForType verifies skip dirs are language-specific
func TestGetSkipDirsForType(t *testing.T) {
	tests := []struct {
		projectType ProjectType
		shouldContain string
	}{
		{ProjectTypeRust, "target"},
		{ProjectTypePython, "__pycache__"},
		{ProjectTypeNode, "node_modules"},
		{ProjectTypeGo, "vendor"},
		{ProjectTypeJava, "target"},
	}

	for _, tt := range tests {
		skipDirs := GetSkipDirsForType(tt.projectType)
		found := false
		for _, dir := range skipDirs {
			if dir == tt.shouldContain {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected skip dir %s for %s, not found", tt.shouldContain, tt.projectType)
		}
	}
}

// TestGetConfigTemplateForType verifies templates are non-empty
func TestGetConfigTemplateForType(t *testing.T) {
	projectTypes := []ProjectType{
		ProjectTypeRust,
		ProjectTypePython,
		ProjectTypeNode,
		ProjectTypeGo,
		ProjectTypeJava,
		ProjectTypeGeneric,
	}

	for _, projectType := range projectTypes {
		template := GetConfigTemplateForType(projectType)
		if len(template) == 0 {
			t.Errorf("Expected non-empty template for %s", projectType)
		}
		if !strings.Contains(template, "[cache]") {
			t.Errorf("Expected [cache] section in template for %s", projectType)
		}
		if !strings.Contains(template, "[stages") {
			t.Errorf("Expected [stages] section in template for %s", projectType)
		}
	}
}

// TestPriorityDetection verifies detection priority (Rust > Python > Node > Go > Java > Generic)
func TestPriorityDetection(t *testing.T) {
	tmpdir := t.TempDir()

	// Create both Cargo.toml and package.json (Rust should take priority)
	if err := os.WriteFile(filepath.Join(tmpdir, "Cargo.toml"), []byte("[package]"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpdir, "package.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	projectType := DetectProjectType(tmpdir)
	if projectType != ProjectTypeRust {
		t.Errorf("Expected Rust to have priority, got %s", projectType)
	}
}
