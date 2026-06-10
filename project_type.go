package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ProjectType represents the type of project being analyzed
type ProjectType string

const (
	ProjectTypeRust    ProjectType = "rust"
	ProjectTypePython  ProjectType = "python"
	ProjectTypeNode    ProjectType = "node"
	ProjectTypeGo      ProjectType = "go"
	ProjectTypeJava    ProjectType = "java"
	ProjectTypeSwift   ProjectType = "swift"
	ProjectTypeGeneric ProjectType = "generic"
)

// DetectProjectType analyzes the project root and determines its type
func DetectProjectType(root string) ProjectType {
	// Check for Rust project files
	if fileExists(filepath.Join(root, "Cargo.toml")) {
		return ProjectTypeRust
	}

	// Check for Node.js/TypeScript project files. A package.json is a
	// definitive Node marker and must take precedence over the Python/Swift
	// markers a Node repo may also carry (e.g. a JS frontend that ships
	// requirements.txt for tooling, or a Package.swift alongside a bundler).
	if fileExists(filepath.Join(root, "package.json")) {
		return ProjectTypeNode
	}

	// Check for Swift project files
	if fileExists(filepath.Join(root, "Package.swift")) {
		return ProjectTypeSwift
	}

	// Check for Xcode project/workspace directories
	entries, err := os.ReadDir(root)
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				name := entry.Name()
				if strings.HasSuffix(name, ".xcodeproj") || strings.HasSuffix(name, ".xcworkspace") {
					return ProjectTypeSwift
				}
			}
		}
	}

	// Check for Python project files
	if fileExists(filepath.Join(root, "pyproject.toml")) ||
		fileExists(filepath.Join(root, "setup.py")) ||
		fileExists(filepath.Join(root, "requirements.txt")) {
		return ProjectTypePython
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
	// For Swift, we need the root to check for Package.swift vs Xcode
	// Since GetDefaultStagesForType signature doesn't include root, we'll
	// use the current directory or handle it in the caller.
	// Actually, the stages can be generic enough or use detection logic inside.

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
	case ProjectTypeSwift:
		cwd, _ := os.Getwd()
		return defaultSwiftStages(cwd)
	default:
		return getGenericStages()
	}
}

// defaultTestCommand returns the appropriate test command for Rust
// Prefers cargo-nextest if available, falls back to cargo test
func defaultTestCommand() []string {
	if hasCargoNextest() {
		return []string{"cargo", "nextest", "run", "--workspace"}
	}
	return []string{"cargo", "test", "--workspace"}
}

// defaultRustTestCommand is an alias for defaultTestCommand (used in tests)
func defaultRustTestCommand() []string {
	return defaultTestCommand()
}

// hasCargoNextest checks if cargo-nextest is installed
func hasCargoNextest() bool {
	_, err := exec.LookPath("cargo-nextest")
	return err == nil
}

// getRustStages returns Rust/Cargo specific stages
func getRustStages() map[string]Stage {
	return map[string]Stage{
		"fmt": {
			Name:      "fmt",
			Cmd:       []string{"cargo", "fmt", "--all", "--", "--check"},
			FixCmd:    []string{"cargo", "fmt", "--all"},
			Check:     true,
			Timeout:   120,
			Enabled:   true,
			DependsOn: []string{},
			Watch:     []string{"*.rs"},
		},
		"clippy": {
			Name:      "clippy",
			Cmd:       []string{"cargo", "clippy", "--workspace", "--all-targets", "--", "-D", "warnings"},
			FixCmd:    nil,
			Check:     false,
			Timeout:   600,
			Enabled:   true,
			DependsOn: []string{"fmt"},
			Watch:     []string{"*.rs", "Cargo.toml", "Cargo.lock"},
		},
		"test": {
			Name:      "test",
			Cmd:       defaultTestCommand(),
			FixCmd:    nil,
			Check:     false,
			Timeout:   1200,
			Enabled:   true,
			DependsOn: []string{"fmt"},
			Watch:     []string{"*.rs", "Cargo.toml", "Cargo.lock"},
		},
		"check": {
			Name:      "check",
			Cmd:       []string{"cargo", "check", "--workspace"},
			FixCmd:    nil,
			Check:     false,
			Timeout:   600,
			Enabled:   false,
			DependsOn: []string{},
			Watch:     []string{"*.rs", "Cargo.toml", "Cargo.lock"},
		},
		"deny": {
			Name:      "deny",
			Cmd:       []string{"cargo", "deny", "check"},
			FixCmd:    nil,
			Check:     false,
			Timeout:   300,
			Enabled:   false,
			DependsOn: []string{},
			Watch:     []string{"Cargo.toml", "Cargo.lock", "deny.toml"},
		},
		"audit": {
			Name:      "audit",
			Cmd:       []string{"cargo", "audit"},
			FixCmd:    nil,
			Check:     false,
			Timeout:   300,
			Enabled:   false,
			DependsOn: []string{},
			Watch:     []string{"Cargo.toml", "Cargo.lock"},
		},
		"machete": {
			Name:      "machete",
			Cmd:       []string{"cargo", "machete"},
			FixCmd:    nil,
			Check:     false,
			Timeout:   300,
			Enabled:   false,
			DependsOn: []string{},
			Watch:     []string{"*.rs", "Cargo.toml"},
		},
	}
}

// getPythonStages returns Python specific stages
func getPythonStages() map[string]Stage {
	return map[string]Stage{
		"lint": {
			Name:      "lint",
			Cmd:       []string{"pylint", ".", "--errors-only"},
			FixCmd:    nil,
			Check:     false,
			Timeout:   300,
			Enabled:   false,
			DependsOn: []string{},
			Watch:     []string{"*.py"},
		},
		"format": {
			Name:      "format",
			Cmd:       []string{"black", "--check", "."},
			FixCmd:    []string{"black", "."},
			Check:     true,
			Timeout:   120,
			Enabled:   false,
			DependsOn: []string{},
			Watch:     []string{"*.py"},
		},
		"test": {
			Name:      "test",
			Cmd:       []string{"pytest"},
			FixCmd:    nil,
			Check:     false,
			Timeout:   600,
			Enabled:   false,
			DependsOn: []string{},
			Watch:     []string{"*.py", "pyproject.toml"},
		},
	}
}

// getNodeStages returns Node.js specific stages
func getNodeStages() map[string]Stage {
	return map[string]Stage{
		"lint": {
			Name:      "lint",
			Cmd:       []string{"npm", "run", "lint"},
			FixCmd:    []string{"npm", "run", "lint", "--", "--fix"},
			Check:     false,
			Timeout:   300,
			Enabled:   false,
			DependsOn: []string{},
			Watch:     []string{"*.js", "*.ts", "*.json"},
		},
		"test": {
			Name:      "test",
			Cmd:       []string{"npm", "test"},
			FixCmd:    nil,
			Check:     false,
			Timeout:   600,
			Enabled:   false,
			DependsOn: []string{},
			Watch:     []string{"*.js", "*.ts", "*.json"},
		},
		"build": {
			Name:      "build",
			Cmd:       []string{"npm", "run", "build"},
			FixCmd:    nil,
			Check:     false,
			Timeout:   600,
			Enabled:   false,
			DependsOn: []string{},
			Watch:     []string{"*.js", "*.ts", "*.json"},
		},
	}
}

// getGoStages returns Go specific stages
func getGoStages() map[string]Stage {
	return map[string]Stage{
		"fmt": {
			Name:      "fmt",
			Cmd:       []string{"go", "fmt", "./..."},
			FixCmd:    []string{"go", "fmt", "./..."},
			Check:     true,
			Timeout:   120,
			Enabled:   false,
			DependsOn: []string{},
			Watch:     []string{"*.go"},
		},
		"vet": {
			Name:      "vet",
			Cmd:       []string{"go", "vet", "./..."},
			FixCmd:    nil,
			Check:     false,
			Timeout:   300,
			Enabled:   false,
			DependsOn: []string{},
			Watch:     []string{"*.go", "go.mod", "go.sum"},
		},
		"test": {
			Name:      "test",
			Cmd:       []string{"go", "test", "./..."},
			FixCmd:    nil,
			Check:     false,
			Timeout:   600,
			Enabled:   false,
			DependsOn: []string{},
			Watch:     []string{"*.go", "go.mod", "go.sum"},
		},
	}
}

// getJavaStages returns Java specific stages
func getJavaStages() map[string]Stage {
	return map[string]Stage{
		"build": {
			Name:      "build",
			Cmd:       []string{"mvn", "clean", "compile"},
			FixCmd:    nil,
			Check:     false,
			Timeout:   600,
			Enabled:   false,
			DependsOn: []string{},
			Watch:     []string{"*.java", "pom.xml"},
		},
		"test": {
			Name:      "test",
			Cmd:       []string{"mvn", "test"},
			FixCmd:    nil,
			Check:     false,
			Timeout:   900,
			Enabled:   false,
			DependsOn: []string{},
			Watch:     []string{"*.java", "pom.xml"},
		},
	}
}

// getGenericStages returns a minimal set of generic stages
func getGenericStages() map[string]Stage {
	return map[string]Stage{
		"test": {
			Name:      "test",
			Cmd:       []string{"echo", "Please configure stages in .local-ci.toml"},
			FixCmd:    nil,
			Check:     false,
			Timeout:   60,
			Enabled:   false,
			DependsOn: []string{},
			Watch:     []string{},
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
	case ProjectTypeSwift:
		return []string{"*.swift", "Package.swift", "Package.resolved", "*.xcconfig", "project.pbxproj"}
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
	case ProjectTypeSwift:
		return append(baseSkip, ".build", ".swiftpm", "DerivedData", "Pods")
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
	case ProjectTypeSwift:
		return getSwiftConfigTemplate()
	default:
		return getGenericConfigTemplate()
	}
}

func getSwiftConfigTemplate() string {
	cwd, _ := os.Getwd()
	isSPM := fileExistsAt(filepath.Join(cwd, "Package.swift"))

	if isSPM {
		return `# local-ci configuration for Swift (SPM) project
# See: https://github.com/stevedores-org/local-ci

[cache]
skip_dirs = [".git", ".github", "scripts", ".claude", ".build", ".swiftpm"]
include_patterns = ["*.swift", "Package.swift", "Package.resolved"]

[stages.fmt]
command = ["swift-format", "lint", "--recursive", "."]
fix_command = ["swift-format", "format", "--in-place", "--recursive", "."]
timeout = 120
enabled = true

[stages.build]
command = ["swift", "build"]
timeout = 600
enabled = true

[stages.test]
command = ["swift", "test"]
timeout = 1200
enabled = true

[dependencies]
required = ["swift-format"]
optional = ["swiftlint"]

[workspace]
exclude = []
`
	}

	return `# local-ci configuration for Swift (Xcode) project
# See: https://github.com/stevedores-org/local-ci

[cache]
skip_dirs = [".git", ".github", "scripts", ".claude", "DerivedData", "Pods"]
include_patterns = ["*.swift", "*.xcconfig", "project.pbxproj"]

[stages.fmt]
command = ["swift-format", "lint", "--recursive", "."]
fix_command = ["swift-format", "format", "--in-place", "--recursive", "."]
timeout = 120
enabled = true

[stages.build]
command = ["xcodebuild", "-scheme", "Placeholder", "build"]
timeout = 600
enabled = true

[stages.test]
command = ["xcodebuild", "test", "-scheme", "Placeholder", "-destination", "platform=macOS"]
timeout = 1200
enabled = true

[dependencies]
required = ["swift-format"]
optional = ["swiftlint"]

[workspace]
exclude = []
`
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
