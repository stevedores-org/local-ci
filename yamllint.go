package main

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
)

// findYAMLFiles recursively searches for all .yml and .yaml files, skipping excluded directories
func findYAMLFiles(root string, skipDirs []string) ([]string, error) {
	var files []string
	skipSet := make(map[string]bool)
	for _, dir := range skipDirs {
		skipSet[dir] = true
	}

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			if skipSet[d.Name()] || d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}

		ext := filepath.Ext(path)
		if ext == ".yml" || ext == ".yaml" {
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			files = append(files, rel)
		}
		return nil
	})

	return files, err
}

// cmdYamllint runs the yaml lint check using the python yamllint command and a temporary configuration file
func cmdYamllint(root string) error {
	// Check if yamllint is installed
	if _, err := exec.LookPath("yamllint"); err != nil {
		return fmt.Errorf("yamllint not found in PATH. Please install it using 'pip install yamllint' or 'brew install yamllint'")
	}

	// Load config to get skip_dirs
	config, err := LoadConfig(root, false)
	var skipDirs []string
	if err == nil && config != nil {
		skipDirs = config.Cache.SkipDirs
	} else {
		// Fallback defaults
		skipDirs = []string{".git", "node_modules", "target", "build", "dist", ".venv", "venv"}
	}

	yamlFiles, err := findYAMLFiles(root, skipDirs)
	if err != nil {
		return fmt.Errorf("failed to scan for YAML files: %w", err)
	}

	if len(yamlFiles) == 0 {
		fmt.Println("No YAML files found.")
		return nil
	}

	// Create temporary yamllint config file
	configContent := `extends: default
rules:
  comments:
    min-spaces-from-content: 1
  document-start: disable
  line-length:
    max: 160
  truthy: disable
`
	tmpFile, err := os.CreateTemp("", "yamllint-*.yml")
	if err != nil {
		return fmt.Errorf("failed to create temporary config file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(configContent); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write to temporary config file: %w", err)
	}
	tmpFile.Close()

	// Execute yamllint
	args := append([]string{"-c", tmpFile.Name()}, yamlFiles...)
	cmd := exec.Command("yamllint", args...)
	cmd.Dir = root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		// Exit code of the command is propagated
		if exitError, ok := err.(*exec.ExitError); ok {
			os.Exit(exitError.ExitCode())
		}
		return fmt.Errorf("failed to run yamllint: %w", err)
	}

	return nil
}
