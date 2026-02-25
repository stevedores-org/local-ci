package main

import (
	"testing"
)

func TestCheckToolInstalledFound(t *testing.T) {
	// "go" should be available in test environment
	tool := &Tool{
		Name:      "go",
		Command:   "go",
		CheckArgs: []string{"version"},
		ToolType:  "binary",
		Optional:  false,
	}

	check := CheckToolInstalled(tool)
	if !check.Found {
		t.Error("expected 'go' to be found")
	}
	if check.Error != "" {
		t.Errorf("expected no error, got %q", check.Error)
	}
}

func TestCheckToolInstalledNotFound(t *testing.T) {
	tool := &Tool{
		Name:      "nonexistent",
		Command:   "definitely-not-a-real-command-xyz",
		CheckArgs: []string{"--version"},
		ToolType:  "binary",
		Optional:  true,
	}

	check := CheckToolInstalled(tool)
	if check.Found {
		t.Error("expected nonexistent tool to not be found")
	}
	if check.Error == "" {
		t.Error("expected error message for missing tool")
	}
}

func TestCheckAllToolsReturnsMap(t *testing.T) {
	results := CheckAllTools()
	if results == nil {
		t.Fatal("CheckAllTools should return non-nil map")
	}

	// Should have entries for all cargo + system tools
	expectedMin := len(cargoTools) + len(systemTools)
	if len(results) < expectedMin {
		t.Errorf("expected at least %d tool checks, got %d", expectedMin, len(results))
	}
}

func TestToolIsAvailableGo(t *testing.T) {
	// "go" won't be in cargoTools or systemTools, so it should return false
	if ToolIsAvailable("go") {
		t.Error("'go' is not in the registered tool list, should return false")
	}
}

func TestToolIsAvailableUnknown(t *testing.T) {
	if ToolIsAvailable("completely-unknown-tool") {
		t.Error("unknown tool should not be available")
	}
}

func TestGetToolByNameCaseInsensitive(t *testing.T) {
	tool := getToolByName("PROTOC")
	if tool == nil {
		t.Fatal("expected to find protoc (case-insensitive)")
	}
	if tool.Name != "protoc" {
		t.Errorf("expected tool name 'protoc', got %q", tool.Name)
	}
}

func TestGetToolByNameNotFound(t *testing.T) {
	tool := getToolByName("nonexistent-tool-xyz")
	if tool != nil {
		t.Error("expected nil for unknown tool")
	}
}

func TestFormatMissingToolsMessageEmpty(t *testing.T) {
	msg := FormatMissingToolsMessage(map[string]string{})
	if msg != "" {
		t.Errorf("expected empty string for no missing tools, got %q", msg)
	}
}

func TestCargoToolsHaveRequiredFields(t *testing.T) {
	for _, tool := range cargoTools {
		if tool.Name == "" {
			t.Error("tool name should not be empty")
		}
		if tool.Command == "" {
			t.Errorf("tool %q command should not be empty", tool.Name)
		}
		if tool.InstallCmd == "" {
			t.Errorf("tool %q install command should not be empty", tool.Name)
		}
		if tool.ToolType == "" {
			t.Errorf("tool %q tool type should not be empty", tool.Name)
		}
	}
}

func TestSystemToolsHaveRequiredFields(t *testing.T) {
	for _, tool := range systemTools {
		if tool.Name == "" {
			t.Error("tool name should not be empty")
		}
		if tool.Command == "" {
			t.Errorf("tool %q command should not be empty", tool.Name)
		}
		if tool.InstallCmd == "" {
			t.Errorf("tool %q install command should not be empty", tool.Name)
		}
	}
}

func TestBunToolsHaveRequiredFields(t *testing.T) {
	for _, tool := range bunTools {
		if tool.Name == "" {
			t.Error("tool name should not be empty")
		}
		if tool.Command == "" {
			t.Errorf("tool %q command should not be empty", tool.Name)
		}
		if tool.InstallCmd == "" {
			t.Errorf("tool %q install command should not be empty", tool.Name)
		}
	}
}

func TestGetMissingToolsForKind(t *testing.T) {
	rustHints := GetMissingToolsWithHints(ProjectTypeRust)
	if rustHints == nil {
		t.Error("should return non-nil map for Rust")
	}

	tsHints := GetMissingToolsWithHints(ProjectTypeTypeScript)
	if tsHints == nil {
		t.Error("should return non-nil map for TypeScript")
	}
}
