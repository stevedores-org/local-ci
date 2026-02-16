package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// Workspace represents a Cargo workspace structure
type Workspace struct {
	Root     string
	Members  []string
	Excludes []string
	IsSingle bool // true if this is a single crate, not a workspace
}

// CargoToml represents the structure of Cargo.toml
type CargoToml struct {
	Workspace *struct {
		Members []string `toml:"members"`
		Exclude []string `toml:"exclude"`
	} `toml:"workspace"`
	Package *struct {
		Name string `toml:"name"`
	} `toml:"package"`
}

// DetectWorkspace detects the workspace structure from Cargo.toml
func DetectWorkspace(root string) (*Workspace, error) {
	cargoPath := filepath.Join(root, "Cargo.toml")

	data, err := os.ReadFile(cargoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read Cargo.toml: %w", err)
	}

	var cargo CargoToml
	if err := toml.Unmarshal(data, &cargo); err != nil {
		return nil, fmt.Errorf("failed to parse Cargo.toml: %w", err)
	}

	ws := &Workspace{
		Root: root,
	}

	// Check if this is a workspace or single crate
	if cargo.Workspace != nil {
		ws.Members = cargo.Workspace.Members
		ws.Excludes = cargo.Workspace.Exclude
		ws.IsSingle = false

		// Expand glob patterns in members
		expandedMembers, err := expandGlobPatterns(root, ws.Members)
		if err == nil {
			ws.Members = expandedMembers
		}

		// Expand glob patterns in excludes
		expandedExcludes, err := expandGlobPatterns(root, ws.Excludes)
		if err == nil {
			ws.Excludes = expandedExcludes
		}
	} else if cargo.Package != nil {
		// Single crate
		ws.IsSingle = true
		ws.Members = []string{"."}
	} else {
		return nil, fmt.Errorf("Cargo.toml is neither a workspace nor a package")
	}

	return ws, nil
}

// expandGlobPatterns expands glob patterns in paths
// e.g., "crates/*" -> ["crates/foo", "crates/bar"]
func expandGlobPatterns(root string, patterns []string) ([]string, error) {
	var result []string

	for _, pattern := range patterns {
		// Check if pattern contains glob characters
		if !strings.ContainsAny(pattern, "*?[") {
			// No glob, use as-is
			result = append(result, pattern)
			continue
		}

		// Expand glob pattern
		fullPattern := filepath.Join(root, pattern)
		matches, err := filepath.Glob(fullPattern)
		if err != nil {
			return nil, err
		}

		// Make relative to root
		for _, match := range matches {
			rel, err := filepath.Rel(root, match)
			if err != nil {
				continue
			}
			result = append(result, rel)
		}
	}

	return result, nil
}

// GetMembers returns the list of workspace members
// If this is a single crate, returns "."
func (w *Workspace) GetMembers() []string {
	if w.IsSingle {
		return []string{"."}
	}
	return w.Members
}

// IsExcluded checks if a crate path is excluded from the workspace
func (w *Workspace) IsExcluded(path string) bool {
	for _, exclude := range w.Excludes {
		if path == exclude {
			return true
		}
		// Check if path is a child of exclude pattern
		if strings.HasPrefix(path, exclude+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

// GetIncludedMembers returns members that are not excluded
func (w *Workspace) GetIncludedMembers() []string {
	var included []string
	for _, member := range w.Members {
		if !w.IsExcluded(member) {
			included = append(included, member)
		}
	}
	if len(included) == 0 {
		return []string{"."}
	}
	return included
}
