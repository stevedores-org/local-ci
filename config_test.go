package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGetEnabledStages(t *testing.T) {
	config := &Config{
		Stages: map[string]Stage{
			"fmt":    {Name: "fmt", Enabled: true},
			"clippy": {Name: "clippy", Enabled: true},
			"test":   {Name: "test", Enabled: true},
			"check":  {Name: "check", Enabled: false},
			"deny":   {Name: "deny", Enabled: false},
		},
	}

	enabled := config.GetEnabledStages()
	if len(enabled) != 3 {
		t.Errorf("expected 3 enabled stages, got %d: %v", len(enabled), enabled)
	}

	// All enabled stages should be in the list
	enabledSet := make(map[string]bool)
	for _, name := range enabled {
		enabledSet[name] = true
	}
	for _, name := range []string{"fmt", "clippy", "test"} {
		if !enabledSet[name] {
			t.Errorf("expected %q in enabled stages", name)
		}
	}
	if enabledSet["check"] || enabledSet["deny"] {
		t.Error("disabled stages should not appear in enabled list")
	}
}

func TestGetTimeoutConfigured(t *testing.T) {
	config := &Config{
		Stages: map[string]Stage{
			"test": {Name: "test", Timeout: 600},
		},
	}

	timeout := config.GetTimeout("test")
	if timeout != 600*time.Second {
		t.Errorf("expected 600s timeout, got %v", timeout)
	}
}

func TestGetTimeoutDefault(t *testing.T) {
	config := &Config{
		Stages: map[string]Stage{
			"test": {Name: "test", Timeout: 0},
		},
	}

	timeout := config.GetTimeout("test")
	if timeout != 30*time.Second {
		t.Errorf("expected 30s default timeout, got %v", timeout)
	}
}

func TestGetTimeoutUnknownStage(t *testing.T) {
	config := &Config{
		Stages: map[string]Stage{},
	}

	timeout := config.GetTimeout("nonexistent")
	if timeout != 30*time.Second {
		t.Errorf("expected 30s default for unknown stage, got %v", timeout)
	}
}

func TestToStageConfigs(t *testing.T) {
	config := &Config{
		Stages: map[string]Stage{
			"fmt": {
				Name:    "fmt",
				Cmd:     []string{"cargo", "fmt", "--all", "--", "--check"},
				FixCmd:  []string{"cargo", "fmt", "--all"},
				Timeout: 120,
				Enabled: true,
			},
			"test": {
				Name:    "test",
				Cmd:     []string{"cargo", "test"},
				FixCmd:  nil,
				Timeout: 600,
				Enabled: true,
			},
		},
	}

	stageConfigs := config.ToStageConfigs()

	if len(stageConfigs) != 2 {
		t.Fatalf("expected 2 stage configs, got %d", len(stageConfigs))
	}

	fmtCfg := stageConfigs["fmt"]
	if fmtCfg.Timeout != 120 {
		t.Errorf("fmt timeout: expected 120, got %d", fmtCfg.Timeout)
	}
	if !fmtCfg.Enabled {
		t.Error("fmt should be enabled")
	}
	if len(fmtCfg.FixCommand) != 3 {
		t.Errorf("expected 3 fix command args, got %d", len(fmtCfg.FixCommand))
	}

	testCfg := stageConfigs["test"]
	if testCfg.FixCommand != nil {
		t.Error("test stage should have nil FixCommand")
	}
}

func TestLoadConfigMalformedTOML(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte("[package]\nname = \"x\"\n"), 0644)
	os.WriteFile(filepath.Join(dir, ".local-ci.toml"), []byte("this is not valid toml {{{}}}"), 0644)

	_, err := LoadConfig(dir)
	if err == nil {
		t.Error("expected error for malformed TOML config")
	}
}

func TestLoadConfigMergesDefaults(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte("[package]\nname = \"x\"\n"), 0644)

	// Partial config: only defines fmt, others should be filled from defaults
	configContent := `[stages.fmt]
command = ["cargo", "fmt"]
timeout = 99
enabled = true
`
	os.WriteFile(filepath.Join(dir, ".local-ci.toml"), []byte(configContent), 0644)

	config, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// fmt should have the custom timeout
	if config.Stages["fmt"].Timeout != 99 {
		t.Errorf("expected custom timeout 99, got %d", config.Stages["fmt"].Timeout)
	}

	// clippy should exist from defaults
	if _, ok := config.Stages["clippy"]; !ok {
		t.Error("clippy should be merged from defaults")
	}
}

func TestSaveDefaultConfigDoesNotOverwrite(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte("[package]\nname = \"x\"\n"), 0644)

	// Create existing config
	existing := "# my custom config\n"
	configPath := filepath.Join(dir, ".local-ci.toml")
	os.WriteFile(configPath, []byte(existing), 0644)

	ws := &Workspace{IsSingle: true, Members: []string{"."}}
	err := SaveDefaultConfig(dir, ws)
	if err == nil {
		t.Error("expected error when config already exists")
	}

	// Verify existing config was not overwritten
	data, _ := os.ReadFile(configPath)
	if string(data) != existing {
		t.Error("existing config should not be overwritten")
	}
}

func TestDefaultStagesHaveCorrectProperties(t *testing.T) {
	stages := GetDefaultStagesForType(ProjectTypeRust)

	// fmt should have Check=true and FixCmd
	fmt := stages["fmt"]
	if !fmt.Check {
		t.Error("fmt stage should have Check=true")
	}
	if fmt.FixCmd == nil {
		t.Error("fmt stage should have FixCmd")
	}
	if !fmt.Enabled {
		t.Error("fmt should be enabled by default")
	}

	// clippy should be enabled, no FixCmd
	clippy := stages["clippy"]
	if !clippy.Enabled {
		t.Error("clippy should be enabled by default")
	}
	if clippy.FixCmd != nil {
		t.Error("clippy should not have FixCmd")
	}

	// check should be disabled
	check := stages["check"]
	if check.Enabled {
		t.Error("check should be disabled by default")
	}

	// deny, audit, machete should be disabled
	for _, name := range []string{"deny", "audit", "machete", "taplo"} {
		if stages[name].Enabled {
			t.Errorf("%s should be disabled by default", name)
		}
	}
}
