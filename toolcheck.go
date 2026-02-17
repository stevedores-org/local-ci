package main

import (
	"fmt"
	"os/exec"
	"strings"
)

// Tool represents a cargo tool or system dependency
type Tool struct {
	Name        string // Display name
	Command     string // Command to check (e.g., "cargo-deny", "protoc")
	CheckArgs   []string // Args to use for version check (e.g., ["deny", "help"])
	InstallCmd  string // How to install
	ToolType    string // "cargo", "system", "binary"
	Optional    bool
}

var cargoTools = []Tool{
	{
		Name:       "cargo-deny",
		Command:    "cargo",
		CheckArgs:  []string{"deny", "help"},
		InstallCmd: "cargo install cargo-deny",
		ToolType:   "cargo",
		Optional:   true,
	},
	{
		Name:       "cargo-audit",
		Command:    "cargo",
		CheckArgs:  []string{"audit", "--help"},
		InstallCmd: "cargo install cargo-audit",
		ToolType:   "cargo",
		Optional:   true,
	},
	{
		Name:       "cargo-machete",
		Command:    "cargo",
		CheckArgs:  []string{"machete", "--help"},
		InstallCmd: "cargo install cargo-machete",
		ToolType:   "cargo",
		Optional:   true,
	},
	{
		Name:       "taplo",
		Command:    "taplo",
		CheckArgs:  []string{"--version"},
		InstallCmd: "cargo install taplo-cli",
		ToolType:   "binary",
		Optional:   true,
	},
}

var systemTools = []Tool{
	{
		Name:       "protoc",
		Command:    "protoc",
		CheckArgs:  []string{"--version"},
		InstallCmd: "brew install protobuf  # macOS\nsudo apt install protobuf-compiler  # Ubuntu",
		ToolType:   "system",
		Optional:   true,
	},
	{
		Name:       "clang",
		Command:    "clang",
		CheckArgs:  []string{"--version"},
		InstallCmd: "xcode-select --install  # macOS\nsudo apt install clang  # Ubuntu",
		ToolType:   "system",
		Optional:   true,
	},
}

// ToolCheck represents the result of checking for a tool
type ToolCheck struct {
	Tool      *Tool
	Found     bool
	Error     string
}

// CheckToolInstalled checks if a tool is installed
func CheckToolInstalled(tool *Tool) *ToolCheck {
	cmd := exec.Command(tool.Command, tool.CheckArgs...)
	err := cmd.Run()

	check := &ToolCheck{
		Tool:  tool,
		Found: err == nil,
	}

	if err != nil {
		check.Error = err.Error()
	}

	return check
}

// CheckAllTools checks all known tools
func CheckAllTools() map[string]*ToolCheck {
	results := make(map[string]*ToolCheck)

	// Check cargo tools
	for _, tool := range cargoTools {
		check := CheckToolInstalled(&tool)
		results[tool.Name] = check
	}

	// Check system tools
	for _, tool := range systemTools {
		check := CheckToolInstalled(&tool)
		results[tool.Name] = check
	}

	return results
}

// GetMissingTools returns a list of missing optional tools
func GetMissingTools() []string {
	var missing []string

	for _, tool := range cargoTools {
		if tool.Optional && !CheckToolInstalled(&tool).Found {
			missing = append(missing, tool.Name)
		}
	}

	return missing
}

// GetMissingToolsWithHints returns missing tools with installation hints
func GetMissingToolsWithHints() map[string]string {
	hints := make(map[string]string)

	allTools := make([]Tool, 0, len(cargoTools)+len(systemTools))
	allTools = append(allTools, cargoTools...)
	allTools = append(allTools, systemTools...)
	for _, tool := range allTools {
		if tool.Optional && !CheckToolInstalled(&tool).Found {
			hints[tool.Name] = tool.InstallCmd
		}
	}

	return hints
}

// ToolIsAvailable checks if a tool is available for running
func ToolIsAvailable(toolName string) bool {
	check := getToolByName(toolName)
	if check == nil {
		return false
	}
	return CheckToolInstalled(check).Found
}

// getToolByName finds a tool by name
func getToolByName(name string) *Tool {
	allTools := make([]Tool, 0, len(cargoTools)+len(systemTools))
	allTools = append(allTools, cargoTools...)
	allTools = append(allTools, systemTools...)
	for _, tool := range allTools {
		if strings.EqualFold(tool.Name, name) {
			return &tool
		}
	}
	return nil
}

// FormatMissingToolsMessage creates a helpful message about missing tools
func FormatMissingToolsMessage(missing map[string]string) string {
	if len(missing) == 0 {
		return ""
	}

	var msg strings.Builder
	msg.WriteString("\n💡 Optional tools missing:\n")

	for toolName, installCmd := range missing {
		msg.WriteString(fmt.Sprintf("\n  %s:\n", toolName))
		// Indent installation instructions
		for _, line := range strings.Split(installCmd, "\n") {
			msg.WriteString(fmt.Sprintf("    %s\n", line))
		}
	}

	return msg.String()
}
