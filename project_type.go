package main

import (
	"os"
	"path/filepath"
)

// ProjectType represents the type of project being analyzed
type ProjectType string

const (
	ProjectTypeRust     ProjectType = "rust"
	ProjectTypePython   ProjectType = "python"
	ProjectTypeNode     ProjectType = "node"
	ProjectTypeGo       ProjectType = "go"
	ProjectTypeJava     ProjectType = "java"
	ProjectTypeGeneric  ProjectType = "generic"
)

// DetectProjectType analyzes the project root and determines its type
func DetectProjectType(root string) ProjectType {
	// Check for Rust project files
	if fileExists(filepath.Join(root, "Cargo.toml")) {
		return ProjectTypeRust
	}

	// Check for Python project files
	if fileExists(filepath.Join(root, "pyproject.toml")) ||
		fileExists(filepath.Join(root, "setup.py")) ||
		fileExists(filepath.Join(root, "requirements.txt")) {
		return ProjectTypePython
	}

	// Check for Node.js project files
	if fileExists(filepath.Join(root, "package.json")) {
		return ProjectTypeNode
	}

	// Check for Go project files
	if fileExists(filepath.Join(root, "go.mod")) {
		return ProjectTypeGo
	}

	// Check for Java project files
	if fileExists(filepath.Join(root, "pom.xml")) ||
		fileExists(filepath.Join(root, "build.gradle")) {
		return ProjectTypeJava
	}

	return ProjectTypeGeneric
}

// GetDefaultStagesForType returns language-specific default stages
func GetDefaultStagesForType(projectType ProjectType) map[string]Stage {
	switch projectType {
	case ProjectTypeRust:
		return getRustStages()
	case ProjectTypePython:
		return getPythonStages()
	case ProjectTypeNode:
		return getNodeStages()
	case ProjectTypeGo:
		return getGoStages()
	case ProjectTypeJava:
		return getJavaStages()
	default:
		return getGenericStages()
	}
}

// getRustStages returns Rust/Cargo specific stages
func getRustStages() map[string]Stage {
	return map[string]Stage{
		"fmt": {
			Name:    "fmt",
			Cmd:     []string{"cargo", "fmt", "--all", "--", "--check"},
			FixCmd:  []string{"cargo", "fmt", "--all"},
			Check:   true,
			Timeout: 120,
			Enabled: true,
		},
		"clippy": {
			Name:    "clippy",
			Cmd:     []string{"cargo", "clippy", "--workspace", "--all-targets", "--", "-D", "warnings"},
			FixCmd:  nil,
			Check:   false,
			Timeout: 600,
			Enabled: true,
		},
		"test": {
			Name:    "test",
			Cmd:     []string{"cargo", "test", "--workspace"},
			FixCmd:  nil,
			Check:   false,
			Timeout: 1200,
			Enabled: true,
		},
		"check": {
			Name:    "check",
			Cmd:     []string{"cargo", "check", "--workspace"},
			FixCmd:  nil,
			Check:   false,
			Timeout: 600,
			Enabled: false,
		},
	}
}

// getPythonStages returns Python specific stages
func getPythonStages() map[string]Stage {
	return map[string]Stage{
		"lint": {
			Name:    "lint",
			Cmd:     []string{"pylint", ".", "--errors-only"},
			FixCmd:  nil,
			Check:   false,
			Timeout: 300,
			Enabled: false,
		},
		"format": {
			Name:    "format",
			Cmd:     []string{"black", "--check", "."},
			FixCmd:  []string{"black", "."},
			Check:   true,
			Timeout: 120,
			Enabled: false,
		},
		"test": {
			Name:    "test",
			Cmd:     []string{"pytest"},
			FixCmd:  nil,
			Check:   false,
			Timeout: 600,
			Enabled: false,
		},
	}
}

// getNodeStages returns Node.js specific stages
func getNodeStages() map[string]Stage {
	return map[string]Stage{
		"lint": {
			Name:    "lint",
			Cmd:     []string{"npm", "run", "lint"},
			FixCmd:  []string{"npm", "run", "lint", "--", "--fix"},
			Check:   false,
			Timeout: 300,
			Enabled: false,
		},
		"test": {
			Name:    "test",
			Cmd:     []string{"npm", "test"},
			FixCmd:  nil,
			Check:   false,
			Timeout: 600,
			Enabled: false,
		},
		"build": {
			Name:    "build",
			Cmd:     []string{"npm", "run", "build"},
			FixCmd:  nil,
			Check:   false,
			Timeout: 600,
			Enabled: false,
		},
	}
}

// getGoStages returns Go specific stages
func getGoStages() map[string]Stage {
	return map[string]Stage{
		"fmt": {
			Name:    "fmt",
			Cmd:     []string{"go", "fmt", "./..."},
			FixCmd:  []string{"go", "fmt", "./..."},
			Check:   true,
			Timeout: 120,
			Enabled: false,
		},
		"vet": {
			Name:    "vet",
			Cmd:     []string{"go", "vet", "./..."},
			FixCmd:  nil,
			Check:   false,
			Timeout: 300,
			Enabled: false,
		},
		"test": {
			Name:    "test",
			Cmd:     []string{"go", "test", "./..."},
			FixCmd:  nil,
			Check:   false,
			Timeout: 600,
			Enabled: false,
		},
	}
}

// getJavaStages returns Java specific stages
func getJavaStages() map[string]Stage {
	return map[string]Stage{
		"build": {
			Name:    "build",
			Cmd:     []string{"mvn", "clean", "compile"},
			FixCmd:  nil,
			Check:   false,
			Timeout: 600,
			Enabled: false,
		},
		"test": {
			Name:    "test",
			Cmd:     []string{"mvn", "test"},
			FixCmd:  nil,
			Check:   false,
			Timeout: 900,
			Enabled: false,
		},
	}
}

// getGenericStages returns a minimal set of generic stages
func getGenericStages() map[string]Stage {
	return map[string]Stage{
		"test": {
			Name:    "test",
			Cmd:     []string{"echo", "Please configure stages in .local-ci.toml"},
			FixCmd:  nil,
			Check:   false,
			Timeout: 60,
			Enabled: false,
		},
	}
}

// GetCachePatternForType returns language-specific file patterns for caching
func GetCachePatternForType(projectType ProjectType) []string {
	switch projectType {
	case ProjectTypeRust:
		return []string{"*.rs", "*.toml", "*.lock"}
	case ProjectTypePython:
		return []string{"*.py", "*.toml", "*.txt", "*.yml", "*.yaml"}
	case ProjectTypeNode:
		return []string{"*.js", "*.ts", "*.json", "package.json", "tsconfig.json"}
	case ProjectTypeGo:
		return []string{"*.go", "go.mod", "go.sum"}
	case ProjectTypeJava:
		return []string{"*.java", "pom.xml", "build.gradle"}
	default:
		return []string{"*"}
	}
}

// GetSkipDirsForType returns language-specific directories to skip in hash calculation
func GetSkipDirsForType(projectType ProjectType) []string {
	baseSkip := []string{".git", ".github", "scripts", ".claude", ".venv", "venv"}

	switch projectType {
	case ProjectTypeRust:
		return append(baseSkip, "target")
	case ProjectTypePython:
		return append(baseSkip, ".pytest_cache", "__pycache__", ".mypy_cache")
	case ProjectTypeNode:
		return append(baseSkip, "node_modules", "dist", "build")
	case ProjectTypeGo:
		return append(baseSkip, "vendor")
	case ProjectTypeJava:
		return append(baseSkip, "target", "build")
	default:
		return append(baseSkip, "node_modules", "target", "build", "dist")
	}
}

// fileExists checks if a file exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// GetConfigTemplateForType returns a TOML config template for the project type
func GetConfigTemplateForType(projectType ProjectType) string {
	switch projectType {
	case ProjectTypeRust:
		return getRustConfigTemplate()
	case ProjectTypePython:
		return getPythonConfigTemplate()
	case ProjectTypeNode:
		return getNodeConfigTemplate()
	case ProjectTypeGo:
		return getGoConfigTemplate()
	case ProjectTypeJava:
		return getJavaConfigTemplate()
	default:
		return getGenericConfigTemplate()
	}
}

func getRustConfigTemplate() string {
	return `# local-ci configuration for Rust project
# See: https://github.com/stevedores-org/local-ci

[cache]
skip_dirs = [".git", "target", ".github", "scripts", ".claude", "node_modules"]
include_patterns = ["*.rs", "*.toml", "*.lock"]

[stages.fmt]
command = ["cargo", "fmt", "--all", "--", "--check"]
fix_command = ["cargo", "fmt", "--all"]
timeout = 120
enabled = true

[stages.clippy]
command = ["cargo", "clippy", "--workspace", "--all-targets", "--", "-D", "warnings"]
timeout = 600
enabled = true

[stages.test]
command = ["cargo", "test", "--workspace"]
timeout = 1200
enabled = true

[stages.check]
command = ["cargo", "check", "--workspace"]
timeout = 600
enabled = false

[dependencies]
optional = []

[workspace]
exclude = []
`
}

func getPythonConfigTemplate() string {
	return `# local-ci configuration for Python project
# See: https://github.com/stevedores-org/local-ci

[cache]
skip_dirs = [".git", ".github", "scripts", ".claude", ".pytest_cache", "__pycache__", ".mypy_cache", ".venv", "venv"]
include_patterns = ["*.py", "*.toml", "*.txt", "*.yml", "*.yaml"]

[stages.lint]
command = ["pylint", ".", "--errors-only"]
timeout = 300
enabled = false

[stages.format]
command = ["black", "--check", "."]
fix_command = ["black", "."]
timeout = 120
enabled = false

[stages.test]
command = ["pytest"]
timeout = 600
enabled = false

[dependencies]
optional = []

[workspace]
exclude = []
`
}

func getNodeConfigTemplate() string {
	return `# local-ci configuration for Node.js project
# See: https://github.com/stevedores-org/local-ci

[cache]
skip_dirs = [".git", ".github", "scripts", ".claude", "node_modules", "dist", "build"]
include_patterns = ["*.js", "*.ts", "*.json", "package.json", "tsconfig.json"]

[stages.lint]
command = ["npm", "run", "lint"]
fix_command = ["npm", "run", "lint", "--", "--fix"]
timeout = 300
enabled = false

[stages.test]
command = ["npm", "test"]
timeout = 600
enabled = false

[stages.build]
command = ["npm", "run", "build"]
timeout = 600
enabled = false

[dependencies]
optional = []

[workspace]
exclude = []
`
}

func getGoConfigTemplate() string {
	return `# local-ci configuration for Go project
# See: https://github.com/stevedores-org/local-ci

[cache]
skip_dirs = [".git", ".github", "scripts", ".claude", "vendor"]
include_patterns = ["*.go", "go.mod", "go.sum"]

[stages.fmt]
command = ["go", "fmt", "./..."]
fix_command = ["go", "fmt", "./..."]
timeout = 120
enabled = false

[stages.vet]
command = ["go", "vet", "./..."]
timeout = 300
enabled = false

[stages.test]
command = ["go", "test", "./..."]
timeout = 600
enabled = false

[dependencies]
optional = []

[workspace]
exclude = []
`
}

func getJavaConfigTemplate() string {
	return `# local-ci configuration for Java project
# See: https://github.com/stevedores-org/local-ci

[cache]
skip_dirs = [".git", ".github", "scripts", ".claude", "target", "build"]
include_patterns = ["*.java", "pom.xml", "build.gradle"]

[stages.build]
command = ["mvn", "clean", "compile"]
timeout = 600
enabled = false

[stages.test]
command = ["mvn", "test"]
timeout = 900
enabled = false

[dependencies]
optional = []

[workspace]
exclude = []
`
}

func getGenericConfigTemplate() string {
	return `# local-ci configuration (Generic)
# See: https://github.com/stevedores-org/local-ci
#
# Define your custom stages below. Example:
#
# [stages.test]
# command = ["npm", "test"]
# timeout = 600
# enabled = true

[cache]
skip_dirs = [".git", ".github", "scripts", ".claude", "node_modules", "target", "build", "dist"]
include_patterns = ["*"]

[stages.placeholder]
command = ["echo", "Configure stages in .local-ci.toml"]
timeout = 60
enabled = false

[dependencies]
optional = []

[workspace]
exclude = []
`
}
