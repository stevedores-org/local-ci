package main

import (
	"os"
	"path/filepath"
)

// ProjectKind identifies the type of project detected in the workspace root.
type ProjectKind string

const (
	ProjectKindRust       ProjectKind = "rust"
	ProjectKindTypeScript ProjectKind = "typescript"
	ProjectKindUnknown    ProjectKind = "unknown"
)

// DetectProjectKind inspects the root directory to determine the project type.
// Precedence: Cargo.toml → Rust, package.json + TS/Bun indicator → TypeScript.
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

	return ProjectKindUnknown
}

// fileExistsAt returns true if path exists and is not a directory.
func fileExistsAt(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
