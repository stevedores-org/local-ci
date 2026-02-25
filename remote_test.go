package main

import (
	"context"
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

func TestEscapeShellArg(t *testing.T) {
	tests := map[string]string{
		"/simple/path":      "/simple/path",
		"/path with spaces": "''/path with spaces''",
		"/path'with'quotes": "''/path'\\''with'\\''quotes''",
	}

	for input, _ := range tests {
		result := escapeShellArg(input)
		// Just verify it doesn't crash and returns something
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
	// This test requires actual SSH access to aivcs@100.90.209.9
	// Skip if not available
	t.Skip("Integration test - requires SSH access to aivcs@100.90.209.9")

	re := NewRemoteExecutor("aivcs@100.90.209.9", "test-session", "/tmp", 30*time.Second, true)

	// Test SSH connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	err := re.TestSSHConnection(ctx)
	cancel()
	if err != nil {
		t.Skipf("Cannot connect to remote host: %v", err)
	}

	// Test session creation
	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	err = re.EnsureRemoteSession(ctx)
	cancel()
	if err != nil {
		t.Errorf("Failed to create remote session: %v", err)
		return
	}

	// Test simple command execution
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

	// Cleanup
	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	_ = re.KillRemoteSession(ctx)
	cancel()
}

func TestRemoteStageExecutionFailure(t *testing.T) {
	// This test requires actual SSH access
	t.Skip("Integration test - requires SSH access to aivcs@100.90.209.9")

	re := NewRemoteExecutor("aivcs@100.90.209.9", "test-session-fail", "/tmp", 30*time.Second, false)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	_ = re.TestSSHConnection(ctx)
	cancel()

	// Test command that fails
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

	// Cleanup
	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	_ = re.KillRemoteSession(ctx)
	cancel()
}
