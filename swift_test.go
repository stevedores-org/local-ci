package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectProjectKind_Swift(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "local-ci-swift-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Test SPM detection
	spmDir := filepath.Join(tempDir, "spm")
	os.MkdirAll(spmDir, 0755)
	os.WriteFile(filepath.Join(spmDir, "Package.swift"), []byte("// swift-tools-version: 5.9"), 0644)

	if kind := DetectProjectKind(spmDir); kind != ProjectKindSwift {
		t.Errorf("expected ProjectKindSwift for SPM, got %v", kind)
	}

	// Test Xcode detection
	xcodeDir := filepath.Join(tempDir, "xcode")
	os.MkdirAll(filepath.Join(xcodeDir, "MyApp.xcodeproj"), 0755)

	if kind := DetectProjectKind(xcodeDir); kind != ProjectKindSwift {
		t.Errorf("expected ProjectKindSwift for Xcode, got %v", kind)
	}
}

func TestGetDefaultStagesForType_Swift(t *testing.T) {
	stages := GetDefaultStagesForType(ProjectTypeSwift)

	if _, ok := stages["fmt"]; !ok {
		t.Error("expected 'fmt' stage for Swift")
	}
	if _, ok := stages["build"]; !ok {
		t.Error("expected 'build' stage for Swift")
	}
	if _, ok := stages["test"]; !ok {
		t.Error("expected 'test' stage for Swift")
	}
}

func TestGetCachePatternForType_Swift(t *testing.T) {
	patterns := GetCachePatternForType(ProjectTypeSwift)
	found := false
	for _, p := range patterns {
		if p == "*.swift" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected '*.swift' in Swift cache patterns")
	}
}

func TestGetSkipDirsForType_Swift(t *testing.T) {
	skipDirs := GetSkipDirsForType(ProjectTypeSwift)
	found := false
	for _, d := range skipDirs {
		if d == ".build" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected '.build' in Swift skip dirs")
	}
}
