package main

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// RemoteExecutor handles execution of stages on a remote machine via SSH+tmux
type RemoteExecutor struct {
	Host    string        // SSH host (e.g., "aivcs@100.90.209.9")
	Session string        // tmux session name (e.g., "onion")
	WorkDir string        // Remote working directory
	Timeout time.Duration // SSH operation timeout
	Verbose bool          // Show detailed output
}

// NewRemoteExecutor creates a new remote executor
func NewRemoteExecutor(host, session, workDir string, timeout time.Duration, verbose bool) *RemoteExecutor {
	if session == "" {
		session = "onion"
	}
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	return &RemoteExecutor{
		Host:    host,
		Session: session,
		WorkDir: workDir,
		Timeout: timeout,
		Verbose: verbose,
	}
}

// ExecuteStage runs a single stage on the remote machine
func (re *RemoteExecutor) ExecuteStage(stage Stage) Result {
	start := time.Now()
	result := Result{
		Name:    stage.Name,
		Command: strings.Join(stage.Cmd, " "),
		Status:  "fail",
	}

	// Generate unique sentinel file for this stage
	sentinelFile := fmt.Sprintf("/tmp/kc_exit_%s_%d", stage.Name, time.Now().UnixNano())

	// Build remote command with exit code capture
	// Format: (cd /path && cmd); echo $? > /tmp/sentinel
	remoteCmd := fmt.Sprintf(
		"cd %s && %s; echo $? > %s",
		escapeShellArg(re.WorkDir),
		strings.Join(stage.Cmd, " "),
		sentinelFile,
	)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(stage.Timeout)*time.Second)
	defer cancel()

	// Execute command in tmux session
	output, err := re.runInSession(ctx, remoteCmd)
	if err != nil {
		result.Error = err
		result.Output = output
		result.Duration = time.Since(start)
		if re.Verbose {
			warnf("Remote execution failed: %v", err)
		}
		return result
	}

	// Poll for exit code from sentinel file
	exitCode, err := re.pollExitCode(ctx, sentinelFile)
	if err != nil {
		result.Error = fmt.Errorf("failed to get exit code: %w", err)
		result.Output = output
		result.Duration = time.Since(start)
		return result
	}

	// Cleanup sentinel file
	_ = re.cleanupSentinel(sentinelFile)

	// Update result based on exit code
	result.Output = output
	result.Duration = time.Since(start)

	if exitCode == 0 {
		result.Status = "pass"
	} else {
		result.Status = "fail"
		result.Error = fmt.Errorf("exit code %d", exitCode)
	}

	return result
}

// runInSession executes a command within a tmux session
func (re *RemoteExecutor) runInSession(ctx context.Context, cmd string) (string, error) {
	// First, ensure the session exists and we're in the right directory
	initCmd := fmt.Sprintf(
		"tmux new-session -d -s %s -c %s 'sleep 999999' 2>/dev/null; true",
		re.Session,
		escapeShellArg(re.WorkDir),
	)

	if err := re.sshExec(ctx, initCmd); err != nil {
		if re.Verbose {
			warnf("Warning: could not initialize session: %v (proceeding anyway)", err)
		}
	}

	// Send the actual command to the session
	sendCmd := fmt.Sprintf(
		"tmux send-keys -t %s '%s' Enter",
		re.Session,
		escapeForTmux(cmd),
	)

	if err := re.sshExec(ctx, sendCmd); err != nil {
		return "", fmt.Errorf("failed to send command to tmux: %w", err)
	}

	// Give command time to execute, then capture output
	time.Sleep(100 * time.Millisecond)

	// Capture pane output
	captureCmd := fmt.Sprintf("tmux capture-pane -t %s -p", re.Session)
	output, err := re.sshExecWithOutput(ctx, captureCmd)
	if err != nil {
		return "", fmt.Errorf("failed to capture output: %w", err)
	}

	return output, nil
}

// pollExitCode polls the remote sentinel file for the exit code
func (re *RemoteExecutor) pollExitCode(ctx context.Context, sentinelFile string) (int, error) {
	maxRetries := 300 // 30 seconds with 100ms poll
	retries := 0

	for retries < maxRetries {
		select {
		case <-ctx.Done():
			return -1, ctx.Err()
		default:
		}

		// Try to read exit code
		catCmd := fmt.Sprintf("cat %s 2>/dev/null", sentinelFile)
		output, err := re.sshExecWithOutput(ctx, catCmd)
		if err == nil && output != "" {
			// Successfully read exit code
			code, err := strconv.Atoi(strings.TrimSpace(output))
			if err == nil {
				return code, nil
			}
		}

		// File doesn't exist yet or couldn't read, retry
		time.Sleep(100 * time.Millisecond)
		retries++
	}

	return -1, fmt.Errorf("timeout waiting for exit code from %s", sentinelFile)
}

// cleanupSentinel removes the sentinel file
func (re *RemoteExecutor) cleanupSentinel(sentinelFile string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cleanupCmd := fmt.Sprintf("rm -f %s", sentinelFile)
	return re.sshExec(ctx, cleanupCmd)
}

// sshExec executes a command on the remote host
func (re *RemoteExecutor) sshExec(ctx context.Context, cmd string) error {
	_, err := re.sshExecWithOutput(ctx, cmd)
	return err
}

// sshExecWithOutput executes a command on the remote host and returns output
func (re *RemoteExecutor) sshExecWithOutput(ctx context.Context, cmd string) (string, error) {
	sshCmd := exec.CommandContext(ctx, "ssh", "-o", "ConnectTimeout=10", re.Host, cmd)

	var stdout, stderr bytes.Buffer
	sshCmd.Stdout = &stdout
	sshCmd.Stderr = &stderr

	if err := sshCmd.Run(); err != nil {
		// For certain commands like 'cat', failure to find file is OK
		if strings.Contains(cmd, "cat") && strings.Contains(stderr.String(), "No such file") {
			return "", nil
		}
		// Other SSH errors should be reported
		if strings.Contains(stderr.String(), "") && stdout.String() == "" {
			return stdout.String(), nil // Command succeeded but produced no output
		}
		// Only return error if this is a real failure
		if !strings.Contains(cmd, "cat") || stderr.String() != "" {
			return stdout.String(), fmt.Errorf("SSH command failed: %w", err)
		}
	}

	return stdout.String(), nil
}

// escapeShellArg safely escapes a shell argument
func escapeShellArg(arg string) string {
	if !strings.ContainsAny(arg, " \t\n'\"\\$`") {
		return arg
	}
	return "'" + strings.ReplaceAll(arg, "'", "'\\''") + "'"
}

// escapeForTmux safely escapes a command for tmux send-keys
func escapeForTmux(cmd string) string {
	// For tmux send-keys, we need to escape single quotes
	return strings.ReplaceAll(cmd, "'", "'\\''")
}

// EnsureRemoteSession creates or attaches to a remote tmux session
func (re *RemoteExecutor) EnsureRemoteSession(ctx context.Context) error {
	cmd := fmt.Sprintf(
		"tmux new-session -d -s %s -c %s 'sleep 999999' 2>/dev/null || true",
		re.Session,
		escapeShellArg(re.WorkDir),
	)
	return re.sshExec(ctx, cmd)
}

// KillRemoteSession kills the remote tmux session
func (re *RemoteExecutor) KillRemoteSession(ctx context.Context) error {
	cmd := fmt.Sprintf("tmux kill-session -t %s 2>/dev/null || true", re.Session)
	return re.sshExec(ctx, cmd)
}

// TestSSHConnection tests if SSH connection works
func (re *RemoteExecutor) TestSSHConnection(ctx context.Context) error {
	return re.sshExec(ctx, "echo 'SSH connection OK'")
}
