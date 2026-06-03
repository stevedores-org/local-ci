package main

import (
	"os"
	"path/filepath"
	"strings"
)

// ProjectKind identifies the type of project detected in the workspace root.
type ProjectKind string

const (
	ProjectKindRust       ProjectKind = "rust"
	ProjectKindTypeScript ProjectKind = "typescript"
	ProjectKindSwift      ProjectKind = "swift"
	ProjectKindUnknown    ProjectKind = "unknown"
)

// DetectProjectKind inspects the root directory to determine the project type.
// Precedence: Cargo.toml → Rust, package.json + TS/Bun indicator → TypeScript, Package.swift or *.xcodeproj → Swift.
func DetectProjectKind(root string) ProjectKind {
	if fileExistsAt(filepath.Join(root, "Cargo.toml")) {
		return ProjectKindRust
	}

	if fileExistsAt(filepath.Join(root, "package.json")) {
		hasTSConfig := fileExistsAt(filepath.Join(root, "tsconfig.json"))
		hasBunfig := fileExistsAt(filepath.Join(root, "bunfig.toml"))
		hasBunLock := fileExistsAt(filepath.Join(root, "bun.lock")) || fileExistsAt(filepath.Join(root, "bun.lockb"))
		if hasTSConfig || hasBunfig || hasBunLock {
			return ProjectKindTypeScript
		}
	}

	// Check for Swift project files
	if fileExistsAt(filepath.Join(root, "Package.swift")) {
		return ProjectKindSwift
	}

	// Check for Xcode project/workspace directories
	entries, err := os.ReadDir(root)
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				name := entry.Name()
				if strings.HasSuffix(name, ".xcodeproj") || strings.HasSuffix(name, ".xcworkspace") {
					return ProjectKindSwift
				}
			}
		}
	}

	return ProjectKindUnknown
}

// fileExistsAt returns true if path exists and is not a directory.
func fileExistsAt(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
