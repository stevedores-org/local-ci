package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- Project kind detection tests ---

func TestDetectProjectKindRust(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte("[package]\nname = \"test\"\n"), 0644)

	kind := DetectProjectKind(dir)
	if kind != ProjectKindRust {
		t.Errorf("expected %q, got %q", ProjectKindRust, kind)
	}
}

func TestDetectProjectKindTypeScript(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"test"}`), 0644)
	os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte(`{}`), 0644)

	kind := DetectProjectKind(dir)
	if kind != ProjectKindTypeScript {
		t.Errorf("expected %q, got %q", ProjectKindTypeScript, kind)
	}
}

func TestDetectProjectKindBunfig(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"test"}`), 0644)
	os.WriteFile(filepath.Join(dir, "bunfig.toml"), []byte(""), 0644)

	kind := DetectProjectKind(dir)
	if kind != ProjectKindTypeScript {
		t.Errorf("expected %q, got %q", ProjectKindTypeScript, kind)
	}
}

func TestDetectProjectKindBothPreferRust(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte("[package]\nname = \"test\"\n"), 0644)
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"test"}`), 0644)
	os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte(`{}`), 0644)

	kind := DetectProjectKind(dir)
	if kind != ProjectKindRust {
		t.Errorf("expected Rust to take priority, got %q", kind)
	}
}

func TestDetectProjectKindUnknown(t *testing.T) {
	dir := t.TempDir()

	kind := DetectProjectKind(dir)
	if kind != ProjectKindUnknown {
		t.Errorf("expected %q, got %q", ProjectKindUnknown, kind)
	}
}

func TestDetectProjectKindPackageJSONAlone(t *testing.T) {
	// package.json without tsconfig, bunfig, or bun.lock → unknown
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"test"}`), 0644)

	kind := DetectProjectKind(dir)
	if kind != ProjectKindUnknown {
		t.Errorf("expected %q for package.json without TS/Bun indicator, got %q", ProjectKindUnknown, kind)
	}
}

func TestDetectProjectKindBunLock(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"test"}`), 0644)
	os.WriteFile(filepath.Join(dir, "bun.lock"), []byte(""), 0644)

	kind := DetectProjectKind(dir)
	if kind != ProjectKindTypeScript {
		t.Errorf("expected %q for package.json + bun.lock, got %q", ProjectKindTypeScript, kind)
	}
}

func TestDetectProjectKindBunLockb(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"test"}`), 0644)
	os.WriteFile(filepath.Join(dir, "bun.lockb"), []byte(""), 0644)

	kind := DetectProjectKind(dir)
	if kind != ProjectKindTypeScript {
		t.Errorf("expected %q for package.json + bun.lockb, got %q", ProjectKindTypeScript, kind)
	}
}

// --- TypeScript default stages tests ---

func TestTypeScriptDefaultStages(t *testing.T) {
	stages := defaultTypeScriptStages()

	expected := []string{"typecheck", "lint", "test", "format"}
	for _, name := range expected {
		stage, ok := stages[name]
		if !ok {
			t.Errorf("missing expected stage %q", name)
			continue
		}
		if stage.Name != name {
			t.Errorf("stage %q has Name=%q", name, stage.Name)
		}
	}

	// Verify typecheck uses bun x tsc (auto-resolves typescript)
	tc := stages["typecheck"]
	if tc.Cmd[0] != "bun" || tc.Cmd[1] != "x" {
		t.Errorf("typecheck should use 'bun x', got %v", tc.Cmd)
	}
	if !sliceContains(tc.Cmd, "tsc") || !sliceContains(tc.Cmd, "--noEmit") {
		t.Errorf("typecheck cmd missing tsc --noEmit: %v", tc.Cmd)
	}

	// Verify test uses bun test
	ts := stages["test"]
	if ts.Cmd[0] != "bun" || !sliceContains(ts.Cmd, "test") {
		t.Errorf("test cmd unexpected: %v", ts.Cmd)
	}

	// Verify lint delegates to package.json script
	lint := stages["lint"]
	if lint.Cmd[0] != "bun" || lint.Cmd[1] != "run" || lint.Cmd[2] != "lint" {
		t.Errorf("lint should be 'bun run lint', got %v", lint.Cmd)
	}

	// Verify lint has fix command
	if lint.FixCmd == nil || !sliceContains(lint.FixCmd, "--fix") {
		t.Errorf("lint FixCmd should contain --fix: %v", lint.FixCmd)
	}

	// Verify format delegates to package.json script
	fmtStage := stages["format"]
	if fmtStage.Cmd[0] != "bun" || fmtStage.Cmd[1] != "run" || fmtStage.Cmd[2] != "format" {
		t.Errorf("format should be 'bun run format ...', got %v", fmtStage.Cmd)
	}

	// Verify format has fix command (without --check)
	if fmtStage.FixCmd == nil {
		t.Fatal("format stage should have FixCmd")
	}
	if sliceContains(fmtStage.FixCmd, "--check") {
		t.Error("format FixCmd should not contain --check")
	}
}

func TestTypeScriptDefaultConfig(t *testing.T) {
	cache := defaultTSCacheConfig()

	if !sliceContains(cache.SkipDirs, "node_modules") {
		t.Error("skip_dirs should include node_modules")
	}
	if !sliceContains(cache.SkipDirs, ".git") {
		t.Error("skip_dirs should include .git")
	}
	if !sliceContains(cache.SkipDirs, "dist") {
		t.Error("skip_dirs should include dist")
	}

	if !sliceContains(cache.IncludePatterns, "*.ts") {
		t.Error("include_patterns should include *.ts")
	}
	if !sliceContains(cache.IncludePatterns, "*.tsx") {
		t.Error("include_patterns should include *.tsx")
	}
	if !sliceContains(cache.IncludePatterns, "*.json") {
		t.Error("include_patterns should include *.json")
	}
}

func TestTypeScriptSourceHash(t *testing.T) {
	dir := t.TempDir()

	// Create some TS files
	os.WriteFile(filepath.Join(dir, "index.ts"), []byte("console.log('hello')"), 0644)
	os.WriteFile(filepath.Join(dir, "app.tsx"), []byte("<App/>"), 0644)
	os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"key":"val"}`), 0644)

	// Create node_modules that should be skipped
	nmDir := filepath.Join(dir, "node_modules", "pkg")
	os.MkdirAll(nmDir, 0755)
	os.WriteFile(filepath.Join(nmDir, "index.ts"), []byte("should be skipped"), 0644)

	config := defaultTypeScriptConfig()

	hash1, err := computeSourceHash(dir, config, nil)
	if err != nil {
		t.Fatal(err)
	}
	if hash1 == "" {
		t.Error("expected non-empty hash")
	}

	// Changing a file should change the hash
	os.WriteFile(filepath.Join(dir, "index.ts"), []byte("console.log('changed')"), 0644)
	hash2, err := computeSourceHash(dir, config, nil)
	if err != nil {
		t.Fatal(err)
	}
	if hash1 == hash2 {
		t.Error("hash should change when source changes")
	}

	// Adding a file in node_modules should NOT change the hash
	os.WriteFile(filepath.Join(nmDir, "new.ts"), []byte("ignored"), 0644)
	hash3, err := computeSourceHash(dir, config, nil)
	if err != nil {
		t.Fatal(err)
	}
	if hash2 != hash3 {
		t.Error("hash should not change from node_modules changes")
	}
}

// --- TypeScript workspace detection tests ---

func TestDetectTypeScriptWorkspace(t *testing.T) {
	dir := t.TempDir()
	pkgJSON := `{
		"name": "my-monorepo",
		"workspaces": ["packages/*", "apps/*"]
	}`
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkgJSON), 0644)
	os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte(`{}`), 0644)

	// Create some workspace packages
	for _, p := range []string{"packages/core", "packages/utils", "apps/web"} {
		os.MkdirAll(filepath.Join(dir, p), 0755)
		os.WriteFile(filepath.Join(dir, p, "package.json"), []byte(`{"name":"`+p+`"}`), 0644)
	}

	ws, err := DetectTypeScriptWorkspace(dir)
	if err != nil {
		t.Fatal(err)
	}
	if ws.IsSingle {
		t.Error("expected workspace, not single package")
	}
	if len(ws.Members) < 3 {
		t.Errorf("expected at least 3 workspace members, got %d: %v", len(ws.Members), ws.Members)
	}
}

func TestDetectTypeScriptSinglePackage(t *testing.T) {
	dir := t.TempDir()
	pkgJSON := `{
		"name": "my-app",
		"scripts": {"test": "bun test"}
	}`
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkgJSON), 0644)
	os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte(`{}`), 0644)

	ws, err := DetectTypeScriptWorkspace(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !ws.IsSingle {
		t.Error("expected single package, not workspace")
	}
}

// --- TypeScript fix command tests ---

func TestTypeScriptFixCmd(t *testing.T) {
	stages := defaultTypeScriptStages()

	// Lint should have a fix command with --fix
	lint := stages["lint"]
	if lint.FixCmd == nil {
		t.Fatal("lint stage should have FixCmd")
	}
	if !sliceContains(lint.FixCmd, "--fix") {
		t.Errorf("lint FixCmd should contain --fix: %v", lint.FixCmd)
	}

	// Format fix should not contain --check
	format := stages["format"]
	if format.FixCmd == nil {
		t.Fatal("format stage should have FixCmd")
	}
	if sliceContains(format.FixCmd, "--check") {
		t.Error("format FixCmd should not contain --check")
	}
}

// --- Config generation for TS ---

func TestSaveDefaultTypeScriptConfig(t *testing.T) {
	dir := t.TempDir()
	err := SaveDefaultTypeScriptConfig(dir)
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".local-ci.toml"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	if !strings.Contains(content, "typecheck") {
		t.Error("TS config should mention typecheck stage")
	}
	if !strings.Contains(content, "bun") {
		t.Error("TS config should mention bun")
	}
	if !strings.Contains(content, "node_modules") {
		t.Error("TS config should mention node_modules in skip_dirs")
	}
	if !strings.Contains(content, "*.ts") {
		t.Error("TS config should include *.ts pattern")
	}
}

// --- No project rejection ---

func TestMainRejectsNoProject(t *testing.T) {
	dir := t.TempDir()
	kind := DetectProjectKind(dir)
	if kind != ProjectKindUnknown {
		t.Errorf("empty dir should detect as unknown, got %q", kind)
	}
}

// --- Helpers ---

func sliceContains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}
