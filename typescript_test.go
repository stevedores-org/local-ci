package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Helper: create a minimal TypeScript project directory
func createTestTSProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	pkg := map[string]interface{}{
		"name":    "test-project",
		"version": "1.0.0",
		"scripts": map[string]string{
			"test":      "bun test",
			"lint":      "eslint .",
			"typecheck": "tsc --noEmit",
		},
	}
	data, _ := json.Marshal(pkg)
	os.WriteFile(filepath.Join(dir, "package.json"), data, 0644)
	os.WriteFile(filepath.Join(dir, "index.ts"), []byte("console.log('hello');\n"), 0644)

	return dir
}

// Helper: create a TS monorepo with workspaces
func createTestTSMonorepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	pkg := map[string]interface{}{
		"name":       "test-monorepo",
		"version":    "1.0.0",
		"workspaces": []string{"packages/*"},
	}
	data, _ := json.Marshal(pkg)
	os.WriteFile(filepath.Join(dir, "package.json"), data, 0644)

	// Create two workspace packages
	for _, name := range []string{"pkg-a", "pkg-b"} {
		pkgDir := filepath.Join(dir, "packages", name)
		os.MkdirAll(pkgDir, 0755)

		memberPkg := map[string]interface{}{
			"name":    name,
			"version": "1.0.0",
		}
		memberData, _ := json.Marshal(memberPkg)
		os.WriteFile(filepath.Join(pkgDir, "package.json"), memberData, 0644)
		os.WriteFile(filepath.Join(pkgDir, "index.ts"), []byte("export {};\n"), 0644)
	}

	return dir
}

// --- Project detection tests ---

func TestDetectProjectKindRust(t *testing.T) {
	dir := createTestWorkspace(t)
	defer os.RemoveAll(dir)

	kind := DetectProjectKind(dir)
	if kind != ProjectRust {
		t.Errorf("expected ProjectRust, got %s", kind)
	}
}

func TestDetectProjectKindTypeScript(t *testing.T) {
	dir := createTestTSProject(t)

	kind := DetectProjectKind(dir)
	if kind != ProjectTypeScript {
		t.Errorf("expected ProjectTypeScript, got %s", kind)
	}
}

func TestDetectProjectKindUnknown(t *testing.T) {
	dir := t.TempDir()
	kind := DetectProjectKind(dir)
	if kind != ProjectUnknown {
		t.Errorf("expected ProjectUnknown, got %s", kind)
	}
}

func TestDetectProjectKindRustTakesPriority(t *testing.T) {
	// If both Cargo.toml and package.json exist, Rust wins
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte("[package]\nname = \"test\"\n"), 0644)
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"test"}`), 0644)

	kind := DetectProjectKind(dir)
	if kind != ProjectRust {
		t.Errorf("expected ProjectRust when both exist, got %s", kind)
	}
}

// --- TS workspace detection tests ---

func TestDetectTSWorkspaceSingle(t *testing.T) {
	dir := createTestTSProject(t)

	ws, err := DetectTSWorkspace(dir)
	if err != nil {
		t.Fatalf("DetectTSWorkspace failed: %v", err)
	}

	if !ws.IsSingle {
		t.Error("expected single package")
	}
	if len(ws.Members) != 1 || ws.Members[0] != "." {
		t.Errorf("expected [.], got %v", ws.Members)
	}
}

func TestDetectTSWorkspaceMonorepo(t *testing.T) {
	dir := createTestTSMonorepo(t)

	ws, err := DetectTSWorkspace(dir)
	if err != nil {
		t.Fatalf("DetectTSWorkspace failed: %v", err)
	}

	if ws.IsSingle {
		t.Error("expected monorepo, got single")
	}
	if len(ws.Members) != 2 {
		t.Errorf("expected 2 members, got %d: %v", len(ws.Members), ws.Members)
	}

	members := strings.Join(ws.Members, ",")
	if !strings.Contains(members, "pkg-a") || !strings.Contains(members, "pkg-b") {
		t.Errorf("expected pkg-a and pkg-b in members, got %v", ws.Members)
	}
}

func TestDetectTSWorkspaceObjectFormat(t *testing.T) {
	dir := t.TempDir()

	// workspaces as object: {packages: [...]}
	pkg := `{"name": "test", "workspaces": {"packages": ["packages/*"]}}`
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkg), 0644)

	// Create a matching package
	pkgDir := filepath.Join(dir, "packages", "foo")
	os.MkdirAll(pkgDir, 0755)
	os.WriteFile(filepath.Join(pkgDir, "package.json"), []byte(`{"name":"foo"}`), 0644)

	ws, err := DetectTSWorkspace(dir)
	if err != nil {
		t.Fatalf("DetectTSWorkspace failed: %v", err)
	}

	if ws.IsSingle {
		t.Error("expected monorepo")
	}
	if len(ws.Members) != 1 || !strings.HasSuffix(ws.Members[0], "foo") {
		t.Errorf("expected [packages/foo], got %v", ws.Members)
	}
}

// --- Config tests for TS ---

func TestLoadConfigTSDefaults(t *testing.T) {
	dir := createTestTSProject(t)

	config, err := LoadConfig(dir, ProjectTypeScript)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Should have TS default stages
	expectedStages := []string{"install", "typecheck", "lint", "test"}
	for _, name := range expectedStages {
		if _, ok := config.Stages[name]; !ok {
			t.Errorf("missing expected TS stage: %s", name)
		}
	}

	// Should have TS cache config
	if !containsString(config.Cache.IncludePatterns, "*.ts") {
		t.Error("expected *.ts in include_patterns for TS project")
	}
	if !containsString(config.Cache.SkipDirs, "node_modules") {
		t.Error("expected node_modules in skip_dirs for TS project")
	}
}

func TestSaveDefaultConfigTS(t *testing.T) {
	dir := createTestTSProject(t)

	err := SaveDefaultConfig(dir, nil, ProjectTypeScript)
	if err != nil {
		t.Fatalf("SaveDefaultConfig failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".local-ci.toml"))
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "[stages.typecheck]") {
		t.Error("TS config should contain typecheck stage")
	}
	if !strings.Contains(content, "bun") {
		t.Error("TS config should reference bun")
	}
	if strings.Contains(content, "cargo") {
		t.Error("TS config should not reference cargo")
	}
}

// --- validateStageCommands tests ---

func TestValidateStageCommandsSuccess(t *testing.T) {
	// "go" should be available in test environment
	stages := []Stage{
		{Name: "test", Cmd: []string{"go", "version"}},
	}
	if err := validateStageCommands(stages); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestValidateStageCommandsMissing(t *testing.T) {
	stages := []Stage{
		{Name: "test", Cmd: []string{"nonexistent-binary-xyz", "--help"}},
	}
	err := validateStageCommands(stages)
	if err == nil {
		t.Fatal("expected error for missing command")
	}
	if !strings.Contains(err.Error(), "nonexistent-binary-xyz") {
		t.Errorf("error should mention the missing binary: %v", err)
	}
	if !strings.Contains(err.Error(), "test") {
		t.Errorf("error should mention the stage name: %v", err)
	}
}

func TestValidateStageCommandsEmpty(t *testing.T) {
	// Empty stages should not error
	if err := validateStageCommands(nil); err != nil {
		t.Fatalf("expected no error for empty stages, got: %v", err)
	}
}

// --- Fix flag tests ---

func TestFixFlagAppliesToAllStagesWithFixCmd(t *testing.T) {
	stages := []Stage{
		{Name: "fmt", Cmd: []string{"cargo", "fmt", "--check"}, FixCmd: []string{"cargo", "fmt"}, Check: true},
		{Name: "lint", Cmd: []string{"bun", "run", "lint"}, FixCmd: []string{"bun", "run", "lint", "--fix"}, Check: true},
		{Name: "test", Cmd: []string{"bun", "run", "test"}, Check: false},
	}

	// Simulate --fix
	for i := range stages {
		if len(stages[i].FixCmd) > 0 {
			stages[i].Cmd = stages[i].FixCmd
			stages[i].Check = false
		}
	}

	if stages[0].Cmd[1] != "fmt" || len(stages[0].Cmd) != 2 {
		t.Errorf("fmt stage should use fix command: %v", stages[0].Cmd)
	}
	if stages[1].Cmd[3] != "--fix" {
		t.Errorf("lint stage should use fix command: %v", stages[1].Cmd)
	}
	if stages[2].Cmd[2] != "test" {
		t.Errorf("test stage should be unchanged: %v", stages[2].Cmd)
	}
}

// --- Source hash test for TS ---

func TestComputeSourceHashTS(t *testing.T) {
	dir := createTestTSProject(t)

	config, _ := LoadConfig(dir, ProjectTypeScript)
	hash1, err := computeSourceHash(dir, config, nil)
	if err != nil {
		t.Fatalf("computeSourceHash failed: %v", err)
	}
	if hash1 == "" {
		t.Error("hash should not be empty")
	}

	// Modifying a .ts file should change the hash
	os.WriteFile(filepath.Join(dir, "index.ts"), []byte("console.log('changed');\n"), 0644)
	hash2, _ := computeSourceHash(dir, config, nil)
	if hash1 == hash2 {
		t.Error("hash should change when source changes")
	}
}

func TestComputeSourceHashTSSkipsNodeModules(t *testing.T) {
	dir := createTestTSProject(t)

	config, _ := LoadConfig(dir, ProjectTypeScript)
	hash1, _ := computeSourceHash(dir, config, nil)

	// Add file under node_modules - should not affect hash
	nmDir := filepath.Join(dir, "node_modules", "some-pkg")
	os.MkdirAll(nmDir, 0755)
	os.WriteFile(filepath.Join(nmDir, "index.js"), []byte("module.exports = {};"), 0644)

	hash2, _ := computeSourceHash(dir, config, nil)
	if hash1 != hash2 {
		t.Error("hash should not change when node_modules changes")
	}
}

// helpers

func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}
