package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// SwiftPackage represents the JSON output of 'swift package describe --type json'
type SwiftPackage struct {
	Name    string        `json:"name"`
	Path    string        `json:"path"`
	Targets []SwiftTarget `json:"targets"`
}

type SwiftTarget struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Type string `json:"type"`
}

// DetectSwiftWorkspace resolves workspace members for Swift projects.
func DetectSwiftWorkspace(root string) (*Workspace, error) {
	ws := &Workspace{
		Root: root,
	}

	// Try SPM first
	if fileExistsAt(filepath.Join(root, "Package.swift")) {
		spmWS, err := detectSPMWorkspace(root)
		if err == nil {
			return spmWS, nil
		}
		// Fallback to single member if SPM describe fails
		ws.IsSingle = true
		ws.Members = []string{"."}
		return ws, nil
	}

	// Try Xcode
	xcodeWS, err := detectXcodeWorkspace(root)
	if err == nil {
		return xcodeWS, nil
	}

	// Default fallback
	ws.IsSingle = true
	ws.Members = []string{"."}
	return ws, nil
}

func detectSPMWorkspace(root string) (*Workspace, error) {
	cmd := exec.Command("swift", "package", "describe", "--type", "json")
	cmd.Dir = root
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var pkg SwiftPackage
	if err := json.Unmarshal(output, &pkg); err != nil {
		return nil, err
	}

	ws := &Workspace{
		Root: root,
	}

	if len(pkg.Targets) == 0 {
		ws.IsSingle = true
		ws.Members = []string{"."}
		return ws, nil
	}

	// For SPM, we can treat targets as members or just the package as a whole.
	// Requirements say "enumerate targets".
	for _, target := range pkg.Targets {
		ws.Members = append(ws.Members, target.Name)
	}

	sort.Strings(ws.Members)
	return ws, nil
}

func detectXcodeWorkspace(root string) (*Workspace, error) {
	ws := &Workspace{
		Root: root,
	}

	// Look for .xcodeproj or .xcworkspace
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		name := entry.Name()
		if strings.HasSuffix(name, ".xcodeproj") || strings.HasSuffix(name, ".xcworkspace") {
			ws.Members = append(ws.Members, name)
		}
	}

	if len(ws.Members) == 0 {
		return nil, fmt.Errorf("no Xcode projects found")
	}

	sort.Strings(ws.Members)
	return ws, nil
}

// defaultSwiftStages returns the built-in Swift stage definitions.
func defaultSwiftStages(root string) map[string]Stage {
	isSPM := fileExistsAt(filepath.Join(root, "Package.swift"))

	stages := map[string]Stage{
		"fmt": {
			Name:    "fmt",
			Cmd:     []string{"swift-format", "lint", "--recursive", "."},
			FixCmd:  []string{"swift-format", "format", "--in-place", "--recursive", "."},
			Check:   true,
			Timeout: 120,
			Enabled: true,
		},
		"lint": {
			Name:    "lint",
			Cmd:     []string{"swiftlint", "--strict"},
			FixCmd:  nil,
			Check:   false,
			Timeout: 300,
			Enabled: false,
		},
	}

	if isSPM {
		stages["build"] = Stage{
			Name:    "build",
			Cmd:     []string{"swift", "build"},
			FixCmd:  nil,
			Check:   false,
			Timeout: 600,
			Enabled: true,
		}
		stages["test"] = Stage{
			Name:    "test",
			Cmd:     []string{"swift", "test"},
			FixCmd:  nil,
			Check:   false,
			Timeout: 1200,
			Enabled: true,
		}
	} else {
		// Xcode fallback - needs scheme, but we use a placeholder or try to detect
		scheme := "Placeholder"
		stages["build"] = Stage{
			Name:    "build",
			Cmd:     []string{"xcodebuild", "-scheme", scheme, "build"},
			FixCmd:  nil,
			Check:   false,
			Timeout: 600,
			Enabled: true,
		}
		stages["test"] = Stage{
			Name:    "test",
			Cmd:     []string{"xcodebuild", "test", "-scheme", scheme, "-destination", "platform=macOS"},
			FixCmd:  nil,
			Check:   false,
			Timeout: 1200,
			Enabled: true,
		}
	}

	return stages
}
