package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRemoteExecutorCreation(t *testing.T) {
	re := NewRemoteExecutor("aivcs@100.90.209.9", "onion", "/tmp/project", 30*time.Second, false)
	if re.Host != "aivcs@100.90.209.9" {
		t.Errorf("Expected host aivcs@100.90.209.9, got %s", re.Host)
	}
	if re.Session != "onion" {
		t.Errorf("Expected session onion, got %s", re.Session)
	}
	if re.WorkDir != "/tmp/project" {
		t.Errorf("Expected workdir /tmp/project, got %s", re.WorkDir)
	}
}

func TestRemoteExecutorDefaultSession(t *testing.T) {
	re := NewRemoteExecutor("aivcs@100.90.209.9", "", "/tmp/project", 0, false)
	if re.Session != "onion" {
		t.Errorf("Expected default session onion, got %s", re.Session)
	}
	if re.Timeout != 30*time.Second {
		t.Errorf("Expected default timeout 30s, got %v", re.Timeout)
	}
}

func TestJoinShellCommand(t *testing.T) {
	tests := []struct {
		parts []string
		want  string
	}{
		{[]string{"cargo", "fmt"}, "cargo fmt"},
		{[]string{"cargo", "clippy", "-D", "warnings"}, "cargo clippy -D warnings"},
		{[]string{"path with spaces"}, "'path with spaces'"},
		{[]string{"it's fine"}, "'it'\\''s fine'"},
	}
	for _, tc := range tests {
		got := joinShellCommand(tc.parts)
		if got != tc.want {
			t.Errorf("joinShellCommand(%v): got %q want %q", tc.parts, got, tc.want)
		}
	}
}

func TestBuildRemoteStageCommand(t *testing.T) {
	cmd := buildRemoteStageCommand("/data/builds/local-ci", []string{"cargo", "test", "--workspace"}, "/tmp/kc_exit_test")
	if !strings.Contains(cmd, "cd /data/builds/local-ci") {
		t.Fatalf("missing cd: %q", cmd)
	}
	if !strings.Contains(cmd, "cargo test --workspace") {
		t.Fatalf("missing command: %q", cmd)
	}
	if !strings.Contains(cmd, "echo $? > /tmp/kc_exit_test") {
		t.Fatalf("missing sentinel: %q", cmd)
	}
}

type mockSSH struct {
	calls    []string
	exitCode string
	failOn   string
}

func (m *mockSSH) execWithOutput(_ context.Context, cmd string) (string, error) {
	m.calls = append(m.calls, cmd)
	if m.failOn != "" && strings.Contains(cmd, m.failOn) {
		return "", context.DeadlineExceeded
	}
	if strings.Contains(cmd, "capture-pane") {
		return "stage output\n", nil
	}
	if strings.Contains(cmd, "cat /tmp/kc_exit_") {
		if m.exitCode != "" {
			return m.exitCode, nil
		}
		return "", nil
	}
	return "", nil
}

func TestRemoteExecutorExecuteStageSuccess(t *testing.T) {
	mock := &mockSSH{exitCode: "0\n"}
	re := NewRemoteExecutor("aivcs@test", "onion", "/tmp/project", 30*time.Second, false)
	re.ssh = mock

	stage := Stage{
		Name:    "fmt",
		Cmd:     []string{"echo", "hello"},
		Timeout: 5,
		Enabled: true,
	}

	result := re.ExecuteStage(stage)
	if result.Status != "pass" {
		t.Fatalf("expected pass, got %s (%v)", result.Status, result.Error)
	}
	if !strings.Contains(result.Output, "stage output") {
		t.Errorf("expected captured output, got %q", result.Output)
	}
	if len(mock.calls) == 0 {
		t.Fatal("expected SSH calls")
	}
}

func TestRemoteExecutorExecuteStageFailure(t *testing.T) {
	mock := &mockSSH{exitCode: "42\n"}
	re := NewRemoteExecutor("aivcs@test", "onion", "/tmp/project", 30*time.Second, false)
	re.ssh = mock

	stage := Stage{
		Name:    "test",
		Cmd:     []string{"false"},
		Timeout: 5,
		Enabled: true,
	}

	result := re.ExecuteStage(stage)
	if result.Status != "fail" {
		t.Fatalf("expected fail, got %s", result.Status)
	}
	if result.Error == nil || !strings.Contains(result.Error.Error(), "42") {
		t.Fatalf("expected exit code 42 error, got %v", result.Error)
	}
}

func TestRemoteExecutorSSHFailure(t *testing.T) {
	mock := &mockSSH{failOn: "send-keys"}
	re := NewRemoteExecutor("aivcs@test", "onion", "/tmp/project", 30*time.Second, false)
	re.ssh = mock

	stage := Stage{
		Name:    "fmt",
		Cmd:     []string{"echo", "hello"},
		Timeout: 5,
		Enabled: true,
	}

	result := re.ExecuteStage(stage)
	if result.Status != "fail" {
		t.Fatalf("expected fail, got %s", result.Status)
	}
	if result.Error == nil {
		t.Fatal("expected SSH error")
	}
}

func TestEscapeShellArg(t *testing.T) {
	tests := map[string]string{
		"/simple/path":      "/simple/path",
		"/path with spaces": "''/path with spaces''",
		"/path'with'quotes": "''/path'\\''with'\\''quotes''",
	}

	for input, _ := range tests {
		result := escapeShellArg(input)
		if len(result) == 0 {
			t.Errorf("escapeShellArg(%q) returned empty string", input)
		}
	}
}

func TestEscapeForTmux(t *testing.T) {
	tests := []string{
		"simple command",
		"command with 'single quotes'",
		"complex 'command' with 'multiple' quotes",
	}

	for _, cmd := range tests {
		result := escapeForTmux(cmd)
		if len(result) == 0 {
			t.Errorf("escapeForTmux(%q) returned empty string", cmd)
		}
	}
}

func TestRemoteStageExecution(t *testing.T) {
	t.Skip("Integration test - requires SSH access to aivcs@100.90.209.9")

	re := NewRemoteExecutor("aivcs@100.90.209.9", "test-session", "/tmp", 30*time.Second, true)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	err := re.TestSSHConnection(ctx)
	cancel()
	if err != nil {
		t.Skipf("Cannot connect to remote host: %v", err)
	}

	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	err = re.EnsureRemoteSession(ctx)
	cancel()
	if err != nil {
		t.Errorf("Failed to create remote session: %v", err)
		return
	}

	stage := Stage{
		Name:    "test",
		Cmd:     []string{"echo", "hello"},
		Timeout: 10,
		Enabled: true,
	}

	result := re.ExecuteStage(stage)
	if result.Status != "pass" {
		t.Errorf("Expected status pass, got %s", result.Status)
	}
	if !strings.Contains(result.Output, "hello") {
		t.Errorf("Expected output to contain 'hello', got %q", result.Output)
	}

	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	_ = re.KillRemoteSession(ctx)
	cancel()
}

func TestRemoteStageExecutionFailure(t *testing.T) {
	t.Skip("Integration test - requires SSH access to aivcs@100.90.209.9")

	re := NewRemoteExecutor("aivcs@100.90.209.9", "test-session-fail", "/tmp", 30*time.Second, false)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	_ = re.TestSSHConnection(ctx)
	cancel()

	stage := Stage{
		Name:    "failing-test",
		Cmd:     []string{"sh", "-c", "exit 42"},
		Timeout: 10,
		Enabled: true,
	}

	result := re.ExecuteStage(stage)
	if result.Status != "fail" {
		t.Errorf("Expected status fail, got %s", result.Status)
	}
	if result.Error == nil {
		t.Errorf("Expected error, got nil")
	}

	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	_ = re.KillRemoteSession(ctx)
	cancel()
}

func TestIssue61HostPresetsDiscoveryAndUranus(t *testing.T) {
	dir := t.TempDir()
	example := `[hosts.discovery]
host = "aivcs@discovery"

[hosts.uranus]
host = "aivcs@uranus"
description = "Tailscale macOS node"
`
	if err := os.WriteFile(filepath.Join(dir, ".local-ci-remote.toml"), []byte(example), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(dir, true)
	if err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"discovery", "uranus"} {
		h, err := cfg.GetRemoteHost(name)
		if err != nil {
			t.Fatalf("GetRemoteHost(%q): %v", name, err)
		}
		if h.Host == "" {
			t.Fatalf("preset %q has empty host", name)
		}
	}

	discovery, err := cfg.ResolveRemoteHost("discovery", "", "onion", "", false, false)
	if err != nil {
		t.Fatal(err)
	}
	if discovery.Host != "aivcs@discovery" {
		t.Errorf("discovery host: got %q", discovery.Host)
	}

	uranus, err := cfg.ResolveRemoteHost("uranus", "", "onion", "", false, false)
	if err != nil {
		t.Fatal(err)
	}
	if uranus.Host != "aivcs@uranus" {
		t.Errorf("uranus host: got %q", uranus.Host)
	}
}
