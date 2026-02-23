package main

import (
	"os"
	"path/filepath"
)

// ProjectKind identifies the type of project
type ProjectKind string

const (
	ProjectRust       ProjectKind = "rust"
	ProjectTypeScript ProjectKind = "typescript"
	ProjectUnknown    ProjectKind = "unknown"
)

// DetectProjectKind auto-detects the project kind based on files in root.
// Priority: Cargo.toml (Rust) > package.json (TypeScript/JS).
func DetectProjectKind(root string) ProjectKind {
	if fileExists(filepath.Join(root, "Cargo.toml")) {
		return ProjectRust
	}
	if fileExists(filepath.Join(root, "package.json")) {
		return ProjectTypeScript
	}
	return ProjectUnknown
}

// fileExists returns true if path exists and is a regular file.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
